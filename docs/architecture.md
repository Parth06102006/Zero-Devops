# Zero-DevOps Architecture

> Zero-DevOps removes the DevOps layer so developers can focus on building products.
> This document captures the **current** implementation and the **planned** target
> architecture, followed by a project completion status snapshot.

The codebase is split into four top-level workspaces:

| Workspace | Purpose |
| --- | --- |
| `client/` | Next.js 15 web UI (landing page + authenticated app shell). |
| `server/` | Go (Echo) API server — auth, GitHub integration, projects, deployments, queueing. |
| `worker-server/` | Go build worker — consumes deployment jobs, builds, uploads artifacts. |
| `schemas/` | Shared versioned contracts (currently the `deploy.jobs` V1 JSON Schema). |

The two Go services use clean-architecture-style package layout (`domain`, `usecase`,
`repository`, `delivery/http`) and share a RabbitMQ build-job bus and a PostgreSQL
database. The client never talks to GitHub, RabbitMQ, or PostgreSQL directly; it only
calls authenticated `server` APIs.

---

## 1. Current Setup

### 1.1 Runtime components

```
Client (Next.js)
   |
   | HTTPS + cookies (access_token / refresh_token)
   v
Server (Go/Echo)
   |-- PostgreSQL   (users, github_installations, projects, deployments, webhook_deliveries, deployment_outbox)
   |-- Redis        (repository-list cache only)
   |-- RabbitMQ     (deploy.jobs / deploy.status command bus)
   |-- GitHub       (OAuth + GitHub App installation + repositories)
   ^
   | deploy.jobs (RabbitMQ)
   |
Worker-server (Go)
   |-- PostgreSQL   (worker-side deployment records)
   |-- RabbitMQ     (deploy.jobs consumer / deploy.status publisher)
   `-- Cloudflare R2 (S3-compatible artifact upload)
```

```mermaid
flowchart LR
    Browser[Client\nNext.js] -->|HTTPS + secure cookies| API[Server\nGo / Echo]
    API --> PG[(PostgreSQL)]
    API --> Redis[(Redis\nrepository cache)]
    API --> GitHub[GitHub\nOAuth + App API]
    API -->|deploy.jobs| MQ[(RabbitMQ)]
    MQ --> Worker[Worker-server\nGo]
    Worker -->|deploy.status| MQ
    Worker --> WorkerPG[(Worker PostgreSQL)]
    Worker --> R2[(Cloudflare R2\nimage tar artifacts)]
```

**Current request flow**

```mermaid
sequenceDiagram
    participant C as Client
    participant S as Server
    participant G as GitHub
    participant P as PostgreSQL
    participant R as Redis
    participant Q as RabbitMQ
    participant W as Worker
    participant A as Cloudflare R2

    C->>S: OAuth, project, or manual-build request
    S->>G: OAuth, installation, repository, commit APIs
    S->>P: Store user, installation, project, deployment
    S->>R: Read/write repository-list cache
    S->>Q: Publish deploy.jobs V1
    Q->>W: Consume deployment job
    W->>W: Checkout exact SHA and detect framework
    W->>W: Docker build and save image tar
    W->>A: Upload image tar
    W->>Q: Publish deploy.status
    Q-->>S: Status message
    S->>P: Update deployment state
```

### 1.2 Server layout (`server/`)

| Package | Responsibility |
| --- | --- |
| `internal/auth` | GitHub OAuth login/refresh/logout/current-user, JWT session cookies, auth middleware. |
| `internal/integrations/scm` | GitHub App installation lifecycle, installation-token provider, repository listing, webhook parser/delivery. |
| `internal/project` | Selected-project CRUD, branch/command configuration, command scanner/policy. |
| `internal/deployments` | Deployment records, manual project builds, V1 `deploy.jobs` contract, RabbitMQ queue setup. |
| `internal/queue` | RabbitMQ exchange/queue/DLQ declaration. |
| `internal/domain` | Shared entities, interfaces, error sentinels. |
| `config`, `internal/logger`, `internal/middleware`, `internal/helper` | Config (Viper), structured logging (Zap), CORS/request-ID/request-logger, response envelope helpers. |

Current server HTTP surface:

- Auth (cookie-based, GitHub OAuth only)
  - `GET /auth/github/login` — start OAuth, set state cookie, redirect to GitHub.
  - `GET /auth/github/login/callback` — exchange code, issue `access_token`/`refresh_token` cookies.
  - `POST /auth/refresh` — rotate session tokens.
  - `POST /auth/logout` — clear session + cookies.
  - `GET /auth/user/me` — current user.
- GitHub integration
  - `POST /integration/scm/github/install` — install GitHub App from OAuth code (legacy path).
  - `GET /integration/scm/github/` — stored installation (legacy path).
  - `DELETE /integration/scm/github/delete` — disconnect installation (legacy path).
  - `GET /integrations/github/installation` — installation/status (new path).
  - `GET /integrations/github/repositories` — paginated repository picker (Redis-cached).
- Projects
  - `GET /projects`, `POST /projects`
  - `GET /projects/:id`, `PATCH /projects/:id`, `DELETE /projects/:id`
- Builds
  - `POST /projects/:id/builds` — manual build (immutable SHA + idempotency key).
  - `GET /projects/:id/builds` — build history for a project.
  - `GET /builds/:id` — build detail.
  - `POST /deploy` — legacy endpoint, currently fails closed (HTTP 409).

### 1.3 Worker layout (`worker-server/`)

| Package | Responsibility |
| --- | --- |
| `internal/worker` | RabbitMQ consumer, deployment job orchestration. |
| `internal/deployments` | Cloning exact commit SHA, framework detection, Dockerfile templating, Docker build, image tar save. |
| `internal/queue` | RabbitMQ queue/status publisher. |
| `internal/upload` | Upload image tar to Cloudflare R2 (S3-compatible). |
| `internal/deployments/contract` | V1 `deploy.jobs` contract mirror (validated against shared schema). |
| `templates/` | Dockerfile templates for detected frameworks. |

Current worker flow: consume `deploy.jobs` → validate V1 → clone exact `commit_sha` →
detect framework/package manager → write Dockerfile from template (or use existing) →
`docker build` → `docker save` to tar → upload tar to R2 → publish status.

> Important: the worker still uses **Docker** for builds, not Buildah, and produces a
> node-local image tar rather than pushing an OCI image to a registry.

### 1.4 Data model (server PostgreSQL)

- `users` — OAuth user identity, refresh token.
- `github_installations` — installation ID, account, `status` (`active`/`suspended`/`uninstalled`), unique per `user_id` and per `installation_id`.
- `projects` — selected/configured repository, branch, webhook flag, build configuration, scanner result, generation counter.
- `deployments` — build runs, linked to project/installation, full V1 fields (SHA, ref, trigger, config snapshot, idempotency key).
- `webhook_deliveries` — GitHub delivery ID dedup/audit.
- `deployment_outbox` — durable outbox for build jobs (schema present; dispatcher not yet implemented).

### 1.5 Shared contract

`schemas/deploy-jobs-v1.schema.json` is the single source of truth for the RabbitMQ
`deploy.jobs` message. Both Go services compile against the same file and validate it
in conformance tests. V1 is immutable; `deploy.status` is a separate, looser contract.

---

## 2. Planned Setup

The target is a reliable, queue-driven build pipeline that accepts webhook-triggered
and manual builds, builds in an isolated worker, and eventually publishes immutable
OCI images.

### 2.1 Target runtime

```
GitHub App webhook
   -> server POST /webhooks/github (HMAC-verified, unauthenticated except signature)
   -> delivery dedup + ownership/ref validation
   -> transactional outbox (PostgreSQL)
   -> RabbitMQ deploy.jobs (publisher confirms)
   -> worker claims job
   -> checkout exact commit SHA
   -> resolve explicit command or scanner fallback
   -> isolated Buildah build (rootless, resource-limited)
   -> local image + digest (not yet portable)
   -> publish deploy.status + build logs
   -> finalize deployment record
```

```mermaid
flowchart LR
    GH[GitHub App\nwebhook] -->|POST /webhooks/github| Ingress[Webhook ingress\nraw body + HMAC validation]
    Ingress --> Dedup[Delivery deduplication\nX-GitHub-Delivery]
    Dedup --> State[PostgreSQL transaction\nlifecycle + project + deployment]
    State --> Outbox[Deployment outbox]
    Outbox -->|publisher confirms| MQ[(RabbitMQ\ndeploy.jobs)]
    MQ --> Claim[Isolated build worker\nmanual ack + bounded concurrency]
    Claim --> Checkout[Checkout exact\ncommit SHA]
    Checkout --> Config[Approved config\nor scanner fallback]
    Config --> Build[Buildah executor\nrootless + resource limits]
    Build --> Result[Image digest +\nbounded build logs]
    Result --> Status[Durable deploy.status\nstate transition]
    Status --> State
    Result --> Registry[(Future OCI registry\npublish by digest)]
    Client[Client UI] -->|authenticated API| State
    Client -->|poll/reconnect| Status
    Redis[(Redis cache)] -. repository pages .-> Ingress
```

**Planned end-to-end flow**

```mermaid
sequenceDiagram
    participant G as GitHub
    participant S as Server
    participant P as PostgreSQL
    participant R as Redis
    participant O as Outbox dispatcher
    participant Q as RabbitMQ
    participant W as Isolated worker
    participant I as Future registry
    participant C as Client

    G->>S: Signed push/install webhook
    S->>S: Limit body, verify HMAC, validate event
    S->>P: Deduplicate delivery and validate installation/project
    S->>P: Create immutable deployment + outbox row
    S-->>G: Return 2xx after durable acceptance
    O->>P: Claim pending outbox row
    O->>Q: Publish persistent BuildRequestV1 with confirm
    Q->>W: Deliver job
    W->>W: Validate contract and checkout exact after SHA
    W->>W: Run isolated Buildah build with limits
    W->>P: Persist terminal result and logs/status
    W->>Q: Acknowledge only after durable outcome
    W->>I: Future digest-addressed image publish
    C->>S: Poll authorized build endpoint
    S-->>C: Current durable build state
```

### 2.2 Planned boundaries

1. **Webhook ingress** — public `POST /webhooks/github`, HMAC `X-Hub-Signature-256`,
   `X-GitHub-Delivery` dedup, event allowlist (`push`, `installation`,
   `installation_repositories`), strict body size limit, no synchronous build.
2. **Lifecycle sync** — `installation.suspend` → `suspended`, `unsuspend` → `active`,
   `deleted` → `uninstalled`; preserve history and block new builds while inactive.
3. **Repository inventory** — GitHub API is the source of inventory; Redis caches
   picker pages (5–15 min TTL, invalidated by webhooks); PostgreSQL persists only
   *selected* projects.
4. **Reliable delivery** — outbox dispatcher with publisher confirms, idempotent
   status transitions, retry/backoff, DLQ.
5. **Immutable builds** — every build keyed by `(repository, ref, commit_sha,
   configuration_version, trigger)`; worker never builds the moving branch tip.
6. **Build executor** — Buildah behind a `BuildExecutor` interface, rootless, with
   per-job timeouts, CPU/memory/PID/disk limits, workspace isolation, and cleanup.
7. **Image/registry abstraction** — `ImageStore`/`ImagePublisher` interface; local
   Buildah storage first, registry push by digest later.
8. **Client UI** — authenticated app shell, connection screen, cached repository
   picker, project form, manual-build form, build-detail/status views (polling first,
   SSE/WebSocket later).

### 2.3 Planned migrations / contracts

- Forward-only Goose migrations (already applied):
  - `20260812000001` — unique installation constraints + `projects`.
  - `20260812000002` — deployment history/outbox + `webhook_deliveries`.
- No new `deploy.jobs` breaking change; V2 would ship alongside V1 for a bounded window.

---

## 3. Completion Status (brief)

### Done

- **Auth**: GitHub OAuth login/callback/refresh/logout/current-user, JWT session cookies, auth middleware with public-path skipping.
- **GitHub installation**: install/get/delete + `status` tracking (`active`/`suspended`/`uninstalled`).
- **Repository picker**: paginated, Redis-cached, single-flight, installation-scoped.
- **Projects**: full CRUD with branch config, webhook flag, command scanning/policy, unique `(user, repo)` selection.
- **Manual builds**: `POST /projects/:id/builds` resolves ref → immutable SHA, snapshots config, publishes V1 job.
- **V1 contract**: shared JSON schema + Go contract packages in both services, with conformance tests.
- **Worker**: consumes V1 jobs, checks out exact SHA, framework/package-manager detection, Docker build, R2 upload, status publish.
- **Migrations**: users, installations, deployments, projects, webhook deliveries, outbox schema.
- **CI**: path-filtered `server`/`worker`/`client` jobs (lint, vet, schema conformance, test, build).

### In progress / partial

- **Webhook ingress**: parser and a stub handler/route exist, and `/webhooks/github` is already public in middleware, but the handler is **not wired** in `main.go` and returns `nil`.
- **Client UI**: landing page, app shell, auth guard/session, dashboard/deployments/settings shells exist; project picker, project form, and build views are not yet built.

### Not done (planned)

- Webhook → lifecycle status wiring (`suspend`/`unsuspend`/`deleted`).
- Webhook → push → build creation with exact `after` SHA and stale-generation coalescing.
- Webhook-driven Redis cache invalidation (method exists, not called).
- Outbox dispatcher (table exists; publisher confirms/idempotent delivery not implemented).
- Reliable status consumption (current consumer is auto-ack based).
- Buildah executor + resource isolation (worker still uses Docker).
- Registry push by digest (currently local tar → R2 only).
- Build-log retrieval endpoint / signed log URL.
- Autoscan command suggestions surfaced + confirmed in the client.
- End-to-end rollout, staging ingress, and load/observability validation.

### Key known gaps

- Legacy `POST /deploy` fails closed; legacy SCM routes still coexist with new `/integrations/...` routes.
- Refresh tokens are stored raw (hash before production).
- `StoreInstallation` is a plain insert; reinstall should be an upsert.
- Worker ack/retry/DLQ policy is not yet the final reliable path.
