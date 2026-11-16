# Zero DevOps Worker Server

Go build worker that consumes V1 `deploy.jobs` from RabbitMQ, checks out an exact commit SHA, builds images with Buildah, and publishes status updates. The static path uploads an image tar to Cloudflare R2; dynamic publication remains a stub.

It does not expose an HTTP API. Jobs are produced only by the outbox dispatcher in `server/`. Shared contract: [`schemas/deploy-jobs-v1.schema.json`](../schemas/deploy-jobs-v1.schema.json). System overview: [`docs/architecture.md`](../docs/architecture.md). Target Buildah design notes: [`docs/buildah-webhook-worker-architecture.md`](../docs/buildah-webhook-worker-architecture.md).

## Architecture

| Package | Responsibility |
| --- | --- |
| `internal/worker` | RabbitMQ consumer loop (manual ack, prefetch 1), retry / DLQ policy |
| `internal/deployments` | Clone exact SHA, framework/package-manager detection, Dockerfile templating, Buildah build + archive export |
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
    Worker --> Builder[Buildah build]
    Worker --> R2[(Cloudflare R2)]
    Worker -->|deploy.status| MQ
```

## Current job flow

1. Consume `deploy.jobs` with **manual ack** and **QoS prefetch 1**.
2. Decode + validate V1 body and AMQP envelope (`content_type`, `message_id`/`event_id`, `correlation_id`, `x-contract-version`). Invalid messages are **nacked without requeue** → `deploy.jobs.dlq`. Legacy/shallow job shapes are never reinterpreted.
3. Insert/reset the worker’s own `deployments` row; publish `building` on `deploy.status`.
4. Shallow-fetch and check out the exact `commit_sha` (never the moving branch tip).
5. Detect framework / package manager; write a Dockerfile from `templates/` (or keep an existing one).
6. `buildah bud` → `buildah push docker-archive:...` to a local tar. Static builds upload the tar; dynamic builds use the existing finalization stub.
7. Update worker DB; publish `success` on completed builds or `canceled` after retry exhaustion. Local attempt failures remain eligible for retry.
8. Ack on success path; on failure, re-publish the same V1 body with incremented `retry_count`, or mark canceled + DLQ when `MAX_RETRIES_COUNT` is exceeded.

> Image creation and worker `success` do not establish deployment. Static finalization uploads a container-image tar, and dynamic finalization has no registry/runner publication yet.

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

### Build dependency policy and native targets

Application builds use Buildah. Generated templates use fully qualified image
references. Go builder versions come from the greater of the `go` and released
`toolchain` versions in the application's `go.mod`; `GOTOOLCHAIN=local` prevents
an implicit toolchain download. Other templates retain their existing version
policy. Image tags are not immutable digests.

Supported JavaScript lockfiles keep frozen installs, including npm shrinkwrap.
Without a lockfile, npm builds fail early with an actionable diagnostic. An
operator can explicitly set `ALLOW_UNLOCKED_NPM_INSTALL=true` in the worker's
process environment to permit `npm install --ignore-scripts --package-lock=false`.
The worker logs a reproducibility warning. Resolution can change between builds
and between build/runtime installation stages; no lockfile is written to the
checkout. Lifecycle install scripts remain disabled in both stages. This option
applies to all jobs handled by that worker; use a separate worker when policies
must differ. It is intentionally not an application-provided switch.

For native targets, put `zero-devops.build.json` in the selected app directory:

```json
{
  "build_context": ".",
  "go_package": "./cmd/server"
}
```

`build_context` is relative to the cloned repository root, while `go_package`
is relative to the app directory. Go defaults to the app context and package
`.`. One explicit local package is supported; wildcard targets are rejected.
Use a larger context for repository-local Go replacements or a Go workspace.

Rust defaults to the repository context, preserving workspace manifests and
sibling path dependencies. It builds the selected manifest's package and one
binary, using `--locked` when its workspace/app lockfile exists. For multiple
binaries or a virtual workspace, select targets explicitly:

```json
{
  "cargo_package": "web-server",
  "cargo_binary": "web-server"
}
```

`default-run` or a single discovered binary can supply the binary selection.
The template copies that exact binary rather than searching for an arbitrary
executable. Paths and symlinks must remain within the checkout; Cargo path
dependencies must fit inside the chosen context. Native options are repository
data, not shell commands. Queue command propagation remains a separate concern.
These changes do not implement image publication, application routing, or
external health checks. A build or local HTTP smoke pass is not a deployment.

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

The worker needs Buildah with usable container storage and the isolation privileges required by its runtime environment.

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

- **Publication is incomplete** — static output is an image tar rather than published web assets; dynamic finalization marks success without a registry/runner or runtime health check.
- **`Insert` resets terminal rows** — `ON CONFLICT … status='pending'` can resurrect finished builds on redelivery.
- **Retry path** re-publishes then acks; a crash between those steps can duplicate concurrent builds.
- **V1 command propagation** — the approved executable/args are not applied. Working directory selects the app; repository-native target options are documented above.
- **Production build limits** — Buildah has a build timeout, but per-job CPU/memory/network isolation is not implemented by these fixes. The campaign applies limits through its separate test containers.
- **No OCI registry** — tar on R2 only; no digest reported on status.

Further isolation and registry publication work is sketched in [`docs/buildah-webhook-worker-architecture.md`](../docs/buildah-webhook-worker-architecture.md).
