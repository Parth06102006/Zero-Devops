# Zero DevOps Server

Go (Echo) API for GitHub OAuth, GitHub App integration, project configuration, and durable build orchestration.

The client talks only to this service (HTTPS + session cookies). The server owns PostgreSQL, Redis (repository-list cache), RabbitMQ (`deploy.jobs` / `deploy.status`), and GitHub. Build execution lives in `worker-server/`.

For the full system picture, see [`docs/architecture.md`](../docs/architecture.md). Shared job contract: [`schemas/deploy-jobs-v1.schema.json`](../schemas/deploy-jobs-v1.schema.json). Historical notes live under [`_reference/`](_reference/).

## Architecture

Clean-architecture style layout:

| Package | Responsibility |
| --- | --- |
| `internal/auth` | GitHub OAuth login/callback/refresh/logout/me, JWT cookies, auth middleware |
| `internal/integrations/scm` | GitHub App install lifecycle, installation tokens, repository listing, webhooks |
| `internal/project` | Project CRUD, branch/command config, command scanner/policy |
| `internal/deployments` | Builds, outbox dispatcher, `deploy.status` consumer, V1 contract |
| `internal/queue` | RabbitMQ exchange/queue/DLQ declaration |
| `internal/domain` | Shared entities, interfaces, error sentinels |
| `config`, `internal/logger`, `internal/middleware`, `internal/helper` | Viper config, Zap logging, CORS/request-ID, response envelopes |

```mermaid
flowchart LR
    Client[Client] -->|HTTPS + cookies| API[Server]
    API --> PG[(PostgreSQL)]
    API --> Redis[(Redis)]
    API --> GitHub[GitHub]
    API -->|outbox → deploy.jobs| MQ[(RabbitMQ)]
    MQ -->|deploy.status| API
    MQ --> Worker[worker-server]
```

**Build path:** manual `POST /projects/:id/builds` or webhook push → deployment + outbox row in one transaction → outbox dispatcher publishes `deploy.jobs` with publisher confirms → worker builds → `deploy.status` → server applies durable state transitions.

## Current capabilities

- **Auth** — GitHub OAuth, `access_token` / `refresh_token` cookies, rotation, logout, current user. Public paths: `/auth/github/login`, `/auth/github/login/callback`, `/auth/refresh`, `/webhooks/github`.
- **GitHub App** — install / get / delete, status (`active` / `suspended` / `uninstalled`), installation gate, paginated repository picker (Redis-cached, fail-open).
- **Projects** — full CRUD, branch + webhook flag, command scanning/policy, `configuration_version` bump on update.
- **Manual builds** — resolve ref → immutable SHA, config snapshot, `StoreProjectBuildWithOutbox`, idempotency-key dedup, per-project `build_number`.
- **Webhook builds** — `POST /webhooks/github` (HMAC + delivery dedup), installation/project gates, exact configured-branch match, `StoreWebhookBuildWithOutbox`.
- **Outbox dispatcher** — sole `deploy.jobs` producer; poll/claim, publisher confirms, stuck-row reconciliation, dead-letter state. Tunable via `OUTBOX_POLL_INTERVAL_MS` / `OUTBOX_BATCH_SIZE`.
- **Status consumer** — durable `deploy.status` with manual ack, legal-transition validation, generation-aware currentness.

Legacy `POST /deploy` has been removed. All builds go through project builds or the webhook.

## HTTP API

Base URL is `SERVER_ADDRESS` (example: `http://127.0.0.1:8750`). Authenticated routes require the `access_token` cookie unless noted.

### Auth

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `GET` | `/auth/github/login` | Public | Start OAuth (state cookie + redirect) |
| `GET` | `/auth/github/login/callback` | Public | Exchange code; set session cookies |
| `POST` | `/auth/refresh` | `refresh_token` cookie | Rotate session tokens |
| `POST` | `/auth/logout` | `access_token` cookie | Clear session + cookies |
| `GET` | `/auth/user/me` | Cookie | Current user |

### GitHub integration

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `POST` | `/integration/scm/github/install` | Cookie | Install/store App (legacy path) |
| `GET` | `/integration/scm/github/` | Cookie | Stored installation (legacy path) |
| `DELETE` | `/integration/scm/github/delete` | Cookie | Local disconnect (legacy path) |
| `GET` | `/integrations/github/installation` | Cookie | Installation + status gate |
| `GET` | `/integrations/github/repositories` | Cookie | Paginated repository picker |

### Projects & builds

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `GET` / `POST` | `/projects` | Cookie | List / create projects |
| `GET` / `PATCH` / `DELETE` | `/projects/:id` | Cookie | Read / update / delete |
| `POST` | `/projects/:id/builds` | Cookie | Manual build (immutable SHA + idempotency key) |
| `GET` | `/projects/:id/builds` | Cookie | Build history |
| `GET` | `/builds/:id` | Cookie | Build detail (`build_number` included) |

### Webhooks

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `POST` | `/webhooks/github` | HMAC signature | GitHub App ingress (`push`, installation lifecycle, etc.) |

Resources are scoped to the authenticated user; cross-user access returns `404` (not `403`).

## Data model (PostgreSQL)

| Table | Role |
| --- | --- |
| `users` | OAuth identity + refresh token |
| `github_installations` | Installation ID, account, status; unique per user and installation |
| `projects` | Selected repo, branch, webhook flag, build config, generation / build counters |
| `deployments` | Build runs (V1 fields, `build_number`, trigger, config snapshot) |
| `webhook_deliveries` | GitHub delivery ID dedup/audit |
| `deployment_outbox` | Durable `deploy.jobs` outbox (`pending` / `sent` / `dead`) |

Migrations: Goose under `database/migrations/` (apply with `make migrate-up`).

## Configuration

Copy `.env.example` → `.env` in `server/`. Important keys:

```env
SERVER_ADDRESS=127.0.0.1:8750
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=
DATABASE_PASS=
DATABASE_NAME=
JWT_SECRET=
OAUTH_GITHUB_CLIENT_ID=
OAUTH_GITHUB_CLIENT_SECRET=
OAUTH_GITHUB_REDIRECT_URL=
GITHUB_APP_ID=
GITHUB_APP_CLIENT_ID=
GITHUB_APP_CLIENT_SECRET=
GITHUB_APP_PRIVATE_KEY_PATH=./keys/your-private-key.pem
GITHUB_APP_WEBHOOK_SECRET=
RABBITMQ_CONNECTION_STRING=amqp://guest:guest@localhost:5672/
REDIS_ADDR=localhost:6379
REDIS_REPOSITORY_CACHE_TTL_SECONDS=600
OUTBOX_POLL_INTERVAL_MS=250
OUTBOX_BATCH_SIZE=50
ALLOWED_ORIGINS=http://localhost:5173
FRONTEND_URL=http://localhost:5173
APP_ENV=development
```

## Local development

Run commands from `server/`.

```bash
# Dependencies + Postgres / RabbitMQ / Redis
make install-deps
make dev-env

# Apply migrations
make migrate-up

# Hot reload (Air) or one-shot run
make up
# or
make build && ./engine

# Tests / lint
make tests
make lint
```

Docker Compose services: `pgsql` (Postgres 17), `rabbitmq`, `redis` — see `docker-compose.yaml`.

Focused tests (examples):

```bash
go test ./internal/auth/... -v
go test ./internal/deployments/... -v
go test ./internal/integrations/scm/... -v
go test ./internal/project/... -v
```

## Known gaps

See [`docs/architecture.md`](../docs/architecture.md) §3 for the full list. Highlights that affect this service:

- Worker does not yet publish terminal `success` on `deploy.status` (successful builds can remain `building` server-side).
- Legacy `/integration/scm/github/...` routes still coexist with `/integrations/...`.
- Refresh tokens are stored raw (hash before production).
- Live broker/DB failure-recovery tests and migration `20260906000001` runtime validation against real Postgres are still pending.
