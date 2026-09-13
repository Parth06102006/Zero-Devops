# Zero DevOps Worker Server

Go build worker that consumes V1 `deploy.jobs` from RabbitMQ, checks out an exact commit SHA, builds with Docker, uploads an image tar to Cloudflare R2, and publishes status updates.

It does not expose an HTTP API. Jobs are produced only by the outbox dispatcher in `server/`. Shared contract: [`schemas/deploy-jobs-v1.schema.json`](../schemas/deploy-jobs-v1.schema.json). System overview: [`docs/architecture.md`](../docs/architecture.md). Target Buildah design notes: [`docs/buildah-webhook-worker-architecture.md`](../docs/buildah-webhook-worker-architecture.md).

## Architecture

| Package | Responsibility |
| --- | --- |
| `internal/worker` | RabbitMQ consumer loop (manual ack, prefetch 1), retry / DLQ policy |
| `internal/deployments` | Clone exact SHA, framework/package-manager detection, Dockerfile templating, Docker build + `docker save` |
| `internal/deployments/contract` | V1 `deploy.jobs` Go types + validation (mirrors server) |
| `internal/queue` | Queue setup, job / status publishing |
| `internal/upload` | Image-tar upload to Cloudflare R2 (S3-compatible) |
| `internal/domain` | Shared entities and interfaces |
| `templates/` | Dockerfile templates for detected frameworks |
| `migrations/` | Worker-side `deployments` table (Goose) |

```mermaid
flowchart LR
    MQ[(RabbitMQ\ndeploy.jobs)] --> Worker[worker-server]
    Worker --> PG[(Worker PostgreSQL)]
    Worker --> Docker[Docker build]
    Worker --> R2[(Cloudflare R2)]
    Worker -->|deploy.status| MQ
```

## Current job flow

1. Consume `deploy.jobs` with **manual ack** and **QoS prefetch 1**.
2. Decode + validate V1 body and AMQP envelope (`content_type`, `message_id`/`event_id`, `correlation_id`, `x-contract-version`). Invalid messages are **nacked without requeue** → `deploy.jobs.dlq`. Legacy/shallow job shapes are never reinterpreted.
3. Insert/reset the worker’s own `deployments` row; publish `building` on `deploy.status`.
4. Shallow-fetch and check out the exact `commit_sha` (never the moving branch tip).
5. Detect framework / package manager; write a Dockerfile from `templates/` (or keep an existing one).
6. `docker build` → `docker save` to a local tar → upload tar to R2.
7. Update worker DB; publish terminal status (`failed` / `canceled`).
8. Ack on success path; on failure, re-publish the same V1 body with incremented `retry_count`, or mark canceled + DLQ when `MAX_RETRIES_COUNT` is exceeded.

> Builds still use **Docker** (not Buildah) and produce a **node-local image tar** on R2 — not a digest-addressed OCI registry push.

### Supported templates

| Template | Typical detection |
| --- | --- |
| `Dockerfile.nextjs.tmpl` | Next.js |
| `Dockerfile.vite.tmpl` | Vite |
| `Dockerfile.react.tmpl` | Create React App / React |
| `Dockerfile.astro.tmpl` | Astro |
| `Dockerfile.node.tmpl` | Generic Node |
| `Dockerfile.go.tmpl` | Go |
| `Dockerfile.python.tmpl` | Python |

## Message contract (consumer rules)

- Body is authoritative; `version` must be `1`.
- Fail closed: malformed JSON, unsupported version, schema failure, or envelope mismatch → DLQ.
- Retries re-publish the **same** V1 body with `retry_count++`; they never reshape the message.
- See [`server/_reference/deploy-jobs-contract.md`](../server/_reference/deploy-jobs-contract.md) for wire format and versioning policy.

Example body (abbreviated):

```json
{
  "version": 1,
  "event_id": "9c5b94b3-1f1c-4f3f-9c8e-0a1b2c3d4e5f",
  "deployment_id": "dep-01J5XYZABC123",
  "project_id": "proj-01J5XYZABC123",
  "installation_id": 12345678,
  "repository_id": 987654321,
  "clone_url": "https://github.com/acme/web.git",
  "commit_sha": "0123456789abcdef0123456789abcdef01234567",
  "requested_ref": "refs/heads/main",
  "trigger": "webhook_push",
  "generation": 7,
  "retry_count": 0,
  "correlation_id": "req-9c5b94b3-1f1c",
  "configuration": {
    "executable": "npm",
    "args": ["run", "build"],
    "working_dir": ".",
    "scanner_policy_version": "v1"
  }
}
```

## Configuration

Copy `.env.example` → `.env` in `worker-server/`.

```env
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=
DATABASE_PASS=
DATABASE_NAME=
RABBITMQ_CONNECTION_STRING=amqp://guest:guest@localhost:5672/
CLOUDFLARE_BUCKET_NAME=
CLOUDFLARE_ACCOUNT_ID=
CLOUDFLARE_ACCESS_KEY_ID=
CLOUDFLARE_ACCESS_KEY_SECRET=
# Optional: public URL base for uploaded artifacts
# CLOUDFLARE_PUBLIC_BASE_URL=
APP_ENV=development
# Optional; defaults to 3 when unset / 0
# MAX_RETRIES_COUNT=3
```

The worker needs a local Docker daemon reachable by the Docker client used in `internal/deployments`.

## Local development

Run commands from `worker-server/`.

```bash
# Tooling + Postgres / RabbitMQ
make install-deps
make dev-env

# Apply worker migrations
make migrate-up

# Hot reload (Air) or direct binary
make up
# or
make build && make dev-run

# Tests / lint
make tests
make lint
```

Docker Compose services: `postgres` (worker DB) and `rabbitmq` (shared container name `zero_devops_rabbitmq` with the API server when both compose files are used carefully) — see `docker-compose.yaml`.

Binary output: `./tmp/worker-engine` (from `make build`).

Focused tests (examples):

```bash
go test ./internal/worker/... -v
go test ./internal/deployments/... -v
go test ./internal/deployments/contract/... -v
```

## Known gaps

Aligned with [`docs/architecture.md`](../docs/architecture.md) §3:

- **No `success` on `deploy.status`** — finalize path updates the worker DB but does not publish terminal success; the API server can leave successful builds stuck in `building`. Top priority fix.
- **`Insert` resets terminal rows** — `ON CONFLICT … status='pending'` can resurrect finished builds on redelivery.
- **Retry path** re-publishes then acks; a crash between those steps can duplicate concurrent builds.
- **`Configuration` from V1** is not applied to the build (approved executable/args/working-dir unused).
- **Buildah isolation** not started — plain Docker, no CPU/mem/network limits or workspace sandbox.
- **No OCI registry** — tar on R2 only; no digest reported on status.

Planned direction (Buildah executor, isolation, registry publish) is sketched in [`docs/buildah-webhook-worker-architecture.md`](../docs/buildah-webhook-worker-architecture.md); that is not the current runtime path.
