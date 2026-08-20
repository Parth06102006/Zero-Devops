# Server, Client, and Worker Implementation Plan — 12-08

## Goal

Deliver the complete user flow across `client`, `server`, and `worker-server`: connect a GitHub App installation, list its repositories through a Redis-backed cache, select and configure a project/branch/build command, create immutable manual or webhook-triggered builds, display reliable build state, and produce a local worker Buildah image.

This document is intentionally end-to-end. `server` is the authority for authorization, persisted configuration, GitHub/webhook validation, idempotency, and build state; `client` is the authenticated configuration and observability UI; `worker-server` performs the isolated build.

## Recommendation

- **Use GitHub as the repository inventory.** `GET /integrations/github/repositories` should obtain an installation access token and paginate GitHub's installation-repositories API. Do not copy every available repository into PostgreSQL merely to populate the picker.
- **Use Redis as the repository-list cache, not as a job queue or authority.** Cache GitHub installation repository-list pages by installation ID/query/cursor for a short TTL (5–15 minutes); invalidate on installation and `installation_repositories` events. A cache miss fetches GitHub. PostgreSQL remains the source of truth for selected projects; RabbitMQ remains the only build-job bus.
- **Persist a selected project, not a repository cache.** A project needs durable ownership, GitHub repository ID, branch/ref policy, enabled state, and versioned build configuration. A webhook must be able to determine whether the repository is selected and allowed to build after the original HTTP session is gone.
- **Use PostgreSQL for durable state and RabbitMQ for delivery.** Do not HTTP-push builds from the webhook handler to the worker. The current RabbitMQ `deploy.jobs` channel is the correct integration seam.
- **Add a transactional outbox.** PostgreSQL and RabbitMQ cannot be committed atomically. Insert the deployment and an outbox event in one database transaction; a dispatcher publishes it with RabbitMQ publisher confirms and marks it sent only after confirmation.

This is the recommended production architecture. A direct `deploymentRepo.Store` followed by `Channel.Publish` can lose a build when PostgreSQL commits but RabbitMQ publishing fails.

## Current Code Reality

- `server/internal/integrations/scm/webhook/github/webhook.go` parses and HMAC-verifies `installation`, `installation_repositories`, and `push`, but no HTTP handler or route calls it.
- `server/cmd/server/main.go` applies auth middleware globally. The public webhook route must be explicitly excluded from cookie/JWT authentication; GitHub authenticity is established by HMAC instead.
- `POST /deploy` in `server/internal/deployments/delivery/http/deployment_handler.go` accepts only `repo_id`. Its use case fetches the clone URL from GitHub and then directly publishes a minimal RabbitMQ job.
- `server/internal/deployments/usecase/deployment_ucase.go` has no publisher confirms, outbox, SHA/ref, event ID, or terminal-state idempotency.
- `worker-server` accepts only `deployment_id`, `clone_url`, `retry_count`, and `request_id`; it shallow-clones the branch tip rather than the requested commit. The webhook feature is unsafe until the shared job contract adds a commit SHA/ref and the worker checks out that SHA.
- Server migrations use Goose annotations. Existing migrations must never be edited once applied to any shared environment; add a new forward-only migration.
- `client/app/page.tsx` is still the stock Next.js starter page. There is currently no authenticated app shell, API client, repository picker, project configuration UI, build view, or deployment-state display; all of these are in scope below.

## Decisions Confirmed Before Coding

- [x] **Webhook trigger:** each project has exactly one configured branch, set or changed by its authenticated owner from the client. A push triggers a build only when its ref equals the branch configuration stored by the server at webhook-processing time. Branch patterns are out of scope for V1.
- [x] **Manual build:** the client must request a SHA or ref. The server authorizes it against the project repository, resolves any ref to a full immutable commit SHA at request time, and queues that SHA. The source value is retained as `requested_ref`; `commit_sha` is authoritative. Manual and webhook builds have distinct triggers and idempotency keys, and are never coalesced with one another.
- [x] **Deleted/force-pushed branches:** ignore deleted refs (`after` is all zeros). Build a force push only for its exact `after` SHA. Store a monotonic project desired-revision/generation with each trigger, and do not let an older queued/running result become the project's current/latest result after a newer accepted revision.
- [x] **Installation ownership:** V1 is single-user only: one GitHub installation belongs to one local user; teams and shared installations are out of scope. Enforce a unique GitHub installation ID and the intended one-installation-per-user rule in the migration and authorization layer.
- [x] **Image/build semantics:** this milestone produces a **local worker Buildah image**, not a portable deployment. It may be reported as `build_succeeded` with its local image reference/digest and worker identity, but it must not be labelled deployed or available to other workers. A future registry/artifact publication with immutable digest is required before deployment semantics exist.

### Command-selection and build safety policy

The worker may autoscan a repository to propose a framework/build command, and the user may select or provide a command. That command is untrusted build configuration, not a security boundary.

- The server must validate the selected command using a versioned policy/scanner before it accepts the build, persist the scanner result/policy version, and put the approved configuration snapshot in the job. Reject unknown, malformed, or policy-denied commands rather than letting the worker improvise.
- Prefer a structured command (`executable` plus argument array and declared working directory) over an arbitrary `sh -c` string. Apply an allowlist and deny dangerous host-oriented flags/paths where the product supports only known build flows.
- Command scanning cannot make building arbitrary repository code safe: project Dockerfiles, package scripts, and build tools execute code too. The worker must therefore enforce isolation, non-root execution where feasible, minimal mounted host access, restricted network/egress, CPU/memory/PID/time/disk limits, secret isolation, and workspace/image cleanup.
- Autoscan suggestions must be displayed as suggestions and require explicit user confirmation. Record the selected command/configuration snapshot on every deployment so rebuilds are reproducible and auditable.
- The scanner itself is a required worker/server prerequisite, not a reason to expose an unrestricted command runner before isolation and policy tests are complete.

## Implementation TODOs

### 0. Establish a safe baseline

- [x] Run `GOCACHE=$(pwd)/.gocache go test ./...` from `server/` and `worker-server/`; record current failures before changing contracts.
  - Baseline recorded 2026-08-12: both commands passed. `server` passed all tested packages (with `internal/domain` reporting no test files); `worker-server` passed all tested packages (with `internal/deployments/repository/pgsql` reporting no test files).
- [x] Validate Goose status safely before changing contracts. Do not alter `20260703000001_create_deployments_table.sql` or other already-applied migration files. The local application DSN and `server/bin/goose` were unavailable, so validation used an isolated disposable PostgreSQL 17 container and `go run github.com/pressly/goose/v3/cmd/goose@v3.11.0`; all migrations applied, both new migrations rolled back individually, then both reapplied successfully. Production/target-environment status remains a release-time check.
- [x] Write a versioned JSON schema and compatibility policy for `deploy.jobs`. This is not a new queue or a Redis replacement: it is the explicit RabbitMQ message contract between `server` and `worker-server`. Both services must accept the same version before rollout.
  - Canonical schema: `schemas/deploy-jobs-v1.schema.json` (JSON Schema draft 2020-12, repo root, immutable once V1 ships).
  - Policy: `docs/deploy-jobs-contract.md` (wire envelope, versioning rules, fail-closed/DLQ behavior, rollout order).
  - Conformance: `server/internal/deployments/schema/` and `worker-server/internal/deployments/schema/` validate the shared schema and `schemas/fixtures/` in both services; CI runs them explicitly.

#### Phase 1 — immutable build-job contract

Phase 1 is the minimum safe contract implementation that must land before enabling webhook-triggered builds. Define `BuildRequestV1` with mandatory `version: 1`, `event_id`, `deployment_id`, `project_id`, `installation_id`, `repository_id`, immutable clone source, full `commit_sha`, `requested_ref`, trigger (`manual` or `webhook_push`), approved configuration snapshot/scanner-policy version, retry metadata, and correlation/request ID.

- [x] The server validates and publishes only complete V1 jobs; the worker validates `version == 1` and required fields, then checks out `commit_sha`, never the moving branch head.
  - `contract.BuildRequestV1` (identical in `server/internal/deployments/contract/` and `worker-server/internal/deployments/contract/`) with `Validate`, `DecodeV1`, `Publishing`, `ValidateMetadata`, `VersionV1`, `HeaderVersion`.
  - Server's `publishBuildRequestV1` is the only producer and uses `contract.Publishing`; `POST /deploy` still fails closed.
  - Worker `cloneRepo` now fetches/checks out the exact SHA (`git init` + `fetch --depth 1 origin <sha>` + `checkout FETCH_HEAD`) instead of `git clone --depth 1` of the branch tip.
- [x] A missing/malformed/unsupported version must be rejected safely and routed through the established retry/DLQ policy, with an observable error; it must not silently fall back to the old shallow-clone behaviour.
  - `DecodeV1`/`ValidateMetadata` reject and the worker `Nack(false, false)`s to `deploy.jobs.dlq`; retries re-publish the same V1 body with an incremented `retry_count`.
- [x] Keep compatibility during rolling deployment: deploy worker V1 support before server V1-only publishing. A future breaking change creates V2 and supports both versions for a bounded migration window.
  - Policy documented in `docs/deploy-jobs-contract.md` (consumers first, producers second; V1 immutable; V2 ships alongside for a bounded window).
- [ ] The worker acknowledges only after its outcome/status is durably reported according to the queue reliability policy.

### 1. Add two forward migrations for project identity and deployment history

Do not modify any migration already applied to a shared environment. Use two new timestamped Goose migrations in `server/migrations/`, in this order. Final names/types should follow the project’s existing conventions.

#### Migration 1 — selected projects and installation identity

- [x] Enforce V1 installation ownership: add unique `github_installations(installation_id)` and a unique user/install association consistent with one installation per local user. Webhook lookup uses `installation_id`; authorization also requires its owner user ID.
- [x] Add `projects` (selected/configured repositories): stable `id`, `user_id`, `installation_id`/foreign key, GitHub numeric `github_repository_id`, owner/name/full name display data, configured branch, enabled flag, desired-revision generation, build configuration JSON/version, command-policy/scanner result, and timestamps.
- [x] Add a unique constraint preventing duplicate active selections for the same owner/repository. `projects.id` is the permanent identity of the configured deployable project; GitHub repository ID identifies the repository even if its name changes.

#### Migration 2 — deployment history, linkage, and delivery

- [x] Extend `deployments` with `project_id` foreign key, `installation_id`, `commit_sha`, `requested_ref`, `trigger`, `webhook_delivery_id`, desired-revision generation, configuration version/snapshot, scanner policy/result, worker/local-image/log fields, and a durable idempotency key.
- [x] Treat identities precisely: `deployment_id` identifies one build run; `project_id` groups that configured project's history; full `commit_sha` identifies exact source; branch/ref is useful context but is mutable and never identifies a deployment.
- [x] Add indexes for history and current-result lookup, at least `(project_id, created_at DESC)` and a status-aware/project query appropriate to PostgreSQL. Query history by `project_id`, not by branch. Do not add a global unique `(project_id, commit_sha)`: the same SHA may be intentionally manually rebuilt.
- [x] Make webhook delivery ID unique for webhook idempotency; require an explicit client idempotency/request key for manual builds. Never deduplicate a manual build with a webhook merely because the SHA matches.
- [x] Add `webhook_deliveries`: GitHub delivery UUID (unique), event name/action, installation ID, repository ID when present, received/processed timestamps, processing status/error, and a bounded payload/audit reference if needed. Do not store secrets.
- [x] Add `deployment_outbox`: event ID, aggregate/deployment ID, message version/payload, state, attempt count, available/sent timestamps, and last error. Index unsent rows.
- [x] Each migration has a down migration that reverses only its own new objects/columns in dependency order. Both migrations were applied to an isolated empty PostgreSQL 17 database, rolled back one at a time, and reapplied successfully. A second isolated PostgreSQL fixture containing an existing user, GitHub installation, and deployment also migrated successfully; its historic deployment remained readable with nullable `project_id` and default `manual` trigger.

**Do not** add a table that mirrors every GitHub repository simply to serve the UI picker. Add a repository cache only later if offline search, authorization auditing, or very large-inventory performance demonstrably requires it.

### 2. Repository listing and project selection API

- [x] Add an installation-token provider behind a domain interface; reuse it for listing, manual deployment, and reconciliation rather than duplicating JWT/token code in handlers.
- [x] Add authenticated `GET /integrations/github/repositories?cursor=&per_page=&query=`. Verify the local installation is `active`, use Redis for a short-lived cache keyed by installation ID plus normalized pagination/query values, and on a miss create a GitHub App installation token, call the GitHub installation repositories endpoint, then cache only the response needed for the picker. Return a DTO and opaque next cursor required by the frontend.
- [x] Define Redis failure behaviour: Redis timeout/unavailability is a cache miss, not a repository-listing outage. Apply cache stampede protection (short lock/single-flight) and response-size limits; never cache installation tokens, webhook secrets, cookies, or user credentials.
- [ ] Invalidate the affected installation's repository-list keys on `installation_repositories`, installation suspend/delete, project access failure, and explicit reconnect. TTL is the fallback for missed events. Redis must never decide authorization: project creation/update performs a fresh GitHub verification when cache freshness cannot be proven. (`InvalidateRepositoryCache` method and tests landed with Task 2; webhook-driven wiring is Task 4.)
- [x] Never trust a frontend `repo_id` by itself. On project creation, verify it is visible to the caller’s active installation.
- [x] Add authenticated project endpoints such as `POST /projects`, `GET /projects`, `GET/PATCH/DELETE /projects/:id`. Persist the selected repository ID and configuration, not the returned repository list.
- [x] Require exactly one explicit configured branch and an `enabled` switch before webhooks can create builds. The client may update that branch only through the authenticated project API; webhook handling reads the persisted server value, never a client-supplied branch.
- [x] Update manual deploy to take a project ID plus a required `sha_or_ref` and client idempotency key. Resolve the repository/configuration server-side, verify the ref/SHA belongs to that repository, resolve it to a full SHA, scan/approve the command configuration, and snapshot all inputs before enqueueing. Retain `repo_id` only as a temporary compatibility path if a migration period is necessary.
- [x] Unit test pagination, inactive installations, GitHub authorization failures, repository ownership validation, duplicate project selection, manual-deploy authorization, Redis hit/miss/failure behaviour, key isolation by installation, and webhook-driven cache invalidation.

### 3. Client application and server API contract

The client is a Next.js application, but it must never contain GitHub App private keys, webhook secrets, RabbitMQ credentials, Redis credentials, or direct worker access. It calls only authenticated `server` APIs; the server alone talks to GitHub, Redis, PostgreSQL, RabbitMQ, and the worker status channel.

- [ ] Replace the starter page with an authenticated app shell: loading, signed-out, no-installation, suspended/uninstalled-installation, empty-project, and error states. Reuse the existing server auth/session model; do not create a second competing client-side identity system.
- [ ] Add a typed client API module that sends authenticated requests to `server`, centralizes base URL/CSRF/cookie handling, parses a standard server error envelope (`code`, safe `message`, field errors, request/correlation ID), and never exposes server secrets through `NEXT_PUBLIC_*` variables.
- [ ] Define and document the API DTOs before UI work: installation state; paginated repository picker item; project read/create/update payload; build-create request (`sha_or_ref`, idempotency key); build list/detail/status; and a cursor/page error contract. The server owns validation; the client validates only for prompt UX.
  - Server-side response DTOs and the standard `success`/`error` envelope are now concrete in `server/internal/domain` (`GithubInstallation`, `Deployment`, `ResponseSuccess`, `ErrorResponse`) and `server/internal/helper`. Full client-facing API documentation remains pending.
- [x] Implement and version the client-facing API surface: `GET /integrations/github/installation`, `GET /integrations/github/repositories`, `GET/POST /projects`, `GET/PATCH/DELETE /projects/:id`, `GET /projects/:id/builds`, `POST /projects/:id/builds`, and `GET /builds/:id`. Scope every resource to the authenticated user and return 404 rather than leaking another user's resource existence. Keep the webhook endpoint separate and unauthenticated except for signature validation.
  - Server side landed 2026-08-19: added `GET /integrations/github/installation`, `GET /projects/:id/builds`, and `GET /builds/:id`; confirmed/kept `GET /integrations/github/repositories`, `GET/POST /projects`, `GET/PATCH/DELETE /projects/:id`, and `POST /projects/:id/builds` stable. Build reads now return the full V1 fields (`project_id`, `commit_sha`, `requested_ref`, `configuration_snapshot`, `output_url`, `error_message`, etc.), and errors use the standard envelope with `ErrNotFound` mapped to 404. Client API module and UI remain in the following bullets.
- [ ] Build the GitHub connection screen: show connection/status, guide a user to install/reconnect the GitHub App, refresh state after return, and clearly explain suspended/uninstalled access. Do not represent the installation flow as complete until the server confirms it.
- [ ] Build the repository picker: query the paginated server endpoint, debounce search, show loading/empty/error/retry states, support pagination, and identify the GitHub owner/name/default branch. The client must not retain an unbounded repository inventory or call GitHub directly; the server's Redis cache makes repeated picker requests efficient.
- [ ] Build the project create/edit form: select one repository, configure exactly one branch, enable/disable webhook builds, review autoscan command suggestions, explicitly select or enter a command, and display server-side policy rejection. Warn that a branch update affects future webhook eligibility only.
- [ ] Build the manual-build form: require SHA/ref, generate one idempotency key per deliberate submit, disable duplicate submit while pending, and present the server-resolved SHA and configuration snapshot in the resulting build detail. A retry is a new deliberate action with a new key unless the original request outcome is unknown.
- [ ] Build project and build-detail views: show trigger (manual/webhook), requested ref, immutable SHA, generation/currentness, build state, timestamps, scanner/configuration version, logs/output URL when authorized, worker/local-image reference on success, and a clear **build succeeded, not deployed** label.
- [ ] Deliver build-status updates by first implementing authenticated polling with bounded backoff and visibility-aware pause/resume. Add SSE or WebSocket only after the server has a durable, authorized event/read model; either transport must reconnect using a cursor and reconcile with `GET /builds/:id`, not treat an in-memory event as authoritative.
- [ ] Add client accessibility and resilience: labelled controls, keyboard picker navigation, focus/error management, mobile layout, skeletons, no secret/error-payload leakage, cancellation on unmount, and handling for expired sessions/401/403/409/429/5xx responses.
- [ ] Add client tests for API error handling, repository pagination/search, branch and command validation UX, duplicate submission/idempotency, status rendering including stale generations, and no-installation/suspended states. Add an end-to-end flow against a test server for connect → picker → project → manual build → status.

### 4. Webhook ingress and lifecycle synchronization

- [ ] Add one public `POST /webhooks/github` Echo handler and exclude it from JWT middleware. Keep request-ID/log middleware.
- [ ] Read the raw body once with a strict size limit before parsing; verify `X-Hub-Signature-256` with the configured webhook secret using constant-time comparison; require `X-GitHub-Delivery` and an allowlisted `X-GitHub-Event`.
- [ ] Adapt or replace the parser so the handler can use the same raw bytes for signature validation, decoding, and delivery persistence. Reject malformed JSON and unsupported events safely.
- [ ] In one transaction, insert the delivery using its unique GitHub ID, update lifecycle/project state, create an idempotent deployment/outbox record for eligible pushes, and mark the delivery accepted. A duplicate delivery must return a fast successful response without another build.
- [ ] Handle `installation` actions: `suspend` -> suspended, `unsuspend` -> active, `deleted` -> uninstalled. Preserve projects and deployment history but prevent new GitHub API work/builds while inactive.
- [ ] Handle `installation_repositories`: only update selected-project availability/enabled state for added/removed repositories; do not build a full inventory table.
- [ ] Handle `push`: validate installation, selected enabled project, and exact match to that project's persisted configured branch. Ignore an all-zero `after` (deleted branch); otherwise create a build for the exact `after` SHA, including force pushes. Advance the project desired-revision generation transactionally, and prevent older generations from becoming the project's current/latest successful result.
- [ ] Return 2xx only after the transaction has durably accepted the delivery. Do not clone, build, or call the worker synchronously.
- [ ] Add handler/use-case tests for invalid/missing signature, missing delivery ID, duplicate delivery, unknown/inactive installation, unselected repo, disabled project, branch deletion, force push, and out-of-order events.

### 5. Reliable server-to-worker delivery

- [ ] Define `BuildRequestV1` shared by the two services: `version`, `event_id`, `deployment_id`, `project_id`, `installation_id`, `repository_id`, immutable clone source, `commit_sha`, `requested_ref`, trigger, desired-revision generation, approved configuration/command snapshot and scanner-policy version, retry metadata, and correlation/request ID. `commit_sha` is mandatory and authoritative.
- [ ] Add an outbox dispatcher with locking/claiming, retry/backoff, and observability. Enable RabbitMQ confirm mode and require confirmation before marking an outbox row sent.
- [ ] Keep durable queue messages persistent. Add message ID/content type/version headers and dead-letter handling.
- [ ] Make status consumption idempotent and validate legal deployment transitions. The current server consumer uses auto-ack and independently updates fields; replace it with a durable, retryable status-update path before relying on it for terminal state.
- [ ] Add a reconciliation/admin task for pending outbox rows and deployments stuck in non-terminal states.
- [ ] Test database-commit/publish-failure recovery, duplicate job delivery, worker crash before ack, duplicate status delivery, and poison-message DLQ routing.

### 6. Worker prerequisites (required before enabling push builds)

- [x] Update `worker-server` to decode `BuildRequestV1` and reject unsupported versions. (Landed with Phase 1.)
- [x] Fetch/checkout the exact `commit_sha`; never build the moving default-branch tip from `git clone --depth 1`. (Landed with Phase 1.)
- [ ] Detect an already-terminal deployment and acknowledge duplicate delivery without rebuilding.
- [ ] Acknowledge a RabbitMQ message only after the terminal server state is durable. Replace re-publish-on-retry with one clear retry/DLQ policy to avoid duplicate concurrent builds.
- [ ] Implement the autoscan/command-policy flow: return suggestions for explicit confirmation, validate the selected structured command before queueing, and version/persist the resulting configuration snapshot. Test denied commands and policy-version changes.
- [ ] Complete the separate Buildah executor/resource-isolation work from `buildah-webhook-worker-architecture.md` before accepting untrusted public repositories at scale. A scanner is defense in depth only; it does not replace sandboxing arbitrary repository build code.
- [ ] Publish `build_succeeded` only after the local Buildah image exists; include worker ID plus local image reference/digest. Do not emit `deployed` or treat the image as portable until a future immutable registry/artifact publication succeeds.

### 7. End-to-end rollout and verification

- [ ] Deploy migration before server code that requires its columns; deploy worker support before the server publishes V1-only jobs; use a compatibility window if server and worker cannot be deployed together.
- [ ] Configure the client API origin/session cookie/CORS and CSRF policy deliberately. Prefer same-site deployment or a narrowly allowlisted client origin; do not use wildcard credentialed CORS. Verify redirects, cookies, and auth expiry in staging.
- [ ] Deploy the server API and client contract compatibly: release additive server DTOs first, then client UI, then remove temporary endpoints only after active clients no longer use them.
- [ ] Configure the GitHub App webhook URL and secret only after staging ingress tests pass.
- [ ] Exercise the full user flow: signed-in user → GitHub connection → cached repository picker → project branch/command configuration → manual SHA/ref build → poll/reconnect → local-image result. Also exercise GitHub redelivery, a real push, duplicate delivery, installation suspension/removal, repository removal, Redis outage/cache invalidation, worker restart, and client session expiry.
- [ ] Observe API latency/error rate, Redis hit rate/evictions/failures, GitHub rate-limit use, queue depth, outbox lag, webhook verification failures, duplicate rate, deployment transition failures, worker retry/DLQ counts, build duration, and cleanup/storage metrics.

## Best-Practice Assessment and Alternatives

| Choice | Assessment | Why / alternative |
| --- | --- | --- |
| Direct webhook HTTP call to worker | **Not recommended** | It couples GitHub availability to worker availability and loses work during failures. Use PostgreSQL outbox + RabbitMQ. |
| Redis-only repository/project state | **Not recommended** | Redis TTL/eviction makes selected-project authorization and audit history unreliable. Use GitHub API for list data and PostgreSQL for selected-project configuration. |
| Redis repository-list cache | **Recommended** | Cache GitHub installation-list pages for 5–15 minutes, invalidate on relevant webhooks, and fall back to GitHub on cache failure. It improves picker latency/rate-limit use without becoming the authorization or persistence authority. |
| Persist every GitHub repository | **Usually unnecessary** | It creates synchronization and deletion/rename issues. List live from the GitHub installation API, then persist selection. Add a sync cache only for measured UX/performance needs. |
| Store no selected project in PostgreSQL | **Not viable for webhook builds** | A webhook must know durable user intent and configuration. Persist a project keyed by GitHub repository ID. |
| Deployment row then RabbitMQ publish | **Not sufficient** | This has an unavoidable dual-write gap. Use an outbox; publisher confirms alone do not repair a crash between DB commit and publish. |
| Build from branch/tag name | **Not recommended as the worker input** | Refs move. Resolve a manual ref to a full SHA server-side; put the webhook `after` SHA in the job and check out that commit. |
| Arbitrary user command as a shell string | **Not recommended** | Prefer a policy-checked executable/argument/working-directory structure. This improves validation and logging, but sandboxing remains mandatory because source-controlled build files/scripts are executable too. |
| Autoscan as the security control | **Not sufficient** | Autoscan is useful to propose a build configuration and reject known-dangerous input. It cannot safely classify arbitrary source code; enforce worker isolation and resource/network/secret boundaries. |
| Local Buildah image as a deployment | **Incorrect terminology** | It is a node-local build result. Record its worker and local reference/digest; reserve `deployed` for a later successful immutable registry/artifact publication. |
| RabbitMQ versus Redis queue | **Keep RabbitMQ now** | It is already the deployment bus. Introducing Redis Streams now creates a second queue and duplicate operational semantics. |

## Done Criteria

- [ ] An authenticated client has a complete connection, picker, project configuration, manual-build, build-detail, and resilient status UI, and it calls only authenticated server APIs.
- [ ] A user can list only repositories granted to their active installation; Redis speeds that picker but a Redis failure falls back safely and never grants access.
- [ ] Selecting a repository creates a durable project; the inventory itself is not persisted.
- [ ] GitHub webhook signatures, event types, delivery IDs, repository/install ownership, and push refs are validated.
- [ ] Duplicate/redelivered webhook deliveries and jobs do not create duplicate builds.
- [ ] A push to a project's enabled configured branch creates an outbox-backed job for the exact commit SHA; deleted branches are ignored and stale generations cannot become the current project result.
- [ ] A manual build has an explicit SHA/ref and idempotency key, is resolved to an immutable SHA server-side, and is not coalesced with a webhook build.
- [ ] Autoscan output is only a confirmed, policy-validated configuration snapshot; the worker enforces isolation even for approved commands.
- [ ] A successful local image is recorded as a build result with its worker/local reference or digest, never as a portable deployment.
- [ ] A worker restart or RabbitMQ publish failure cannot silently lose the accepted build request.
- [ ] Suspended/uninstalled installations and removed repositories cannot create new builds while historic data remains available.
- [ ] Tests cover the critical ingress, idempotency, migration, queue, and worker-contract paths.

## Related Documents

- [Buildah, webhook, and worker architecture](buildah-webhook-worker-architecture.md)
- [Existing webhook note](future/webhook_implementation.md)
- [GitHub integration future work](future/github_integration_future.md)
