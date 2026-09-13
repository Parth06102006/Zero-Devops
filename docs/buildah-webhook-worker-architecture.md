# Buildah Worker, GitHub Webhooks, and Repository Sync Plan

## Recommendation

Replace the worker's Docker build execution with a Buildah-backed build adapter, but do not make Buildah responsible for orchestration, repository synchronization, or webhook delivery. Keep those responsibilities behind interfaces so the build executor and eventual registry can be replaced later.

For the current repository, keep RabbitMQ as the durable deployment command bus because both `server` and `worker-server` already use it. Use Redis for repository synchronization state, webhook deduplication, and optional coalescing—not as a second competing deployment queue in the first implementation.

Do not build a push on top of a mutable or previously published tag. Every build must be identified by repository, branch/ref, and exact commit SHA. A push event should enqueue a new immutable build request for that SHA. Buildah layer caching may accelerate the build, but the source of truth remains the checked-out commit and build configuration.

## What I found in the repository

- `worker-server/internal/deployments/deployments.go` currently clones with `git clone --depth 1`, detects a framework, writes one of the repository's Dockerfile templates, invokes Docker, saves an image tar through the Docker client, uploads the tar to S3-compatible storage, and removes the workspace with `defer os.RemoveAll`.
- `worker-server/internal/worker/worker.go` consumes RabbitMQ with manual acknowledgements and `Qos(1, 0, false)`, so the current worker processes only one unacknowledged build at a time.
- `worker-server/internal/domain/queue.go` and `server/internal/deployments/usecase/deployment_ucase.go` define the current deployment job/status contract over RabbitMQ.
- `server/internal/deployments/delivery/http/deployment_handler.go` exposes `POST /deploy` with only `repo_id`; `server/internal/deployments/usecase/deployment_ucase.go` obtains the GitHub installation token, resolves the clone URL, stores the deployment, and publishes `deploy.jobs`.
- The current deployment migration stores deployment status and a clone URL but has no repository/project configuration, source SHA, build commands, webhook delivery ID, build log reference, or image digest.
- The existing future webhook documents say GitHub webhook routing and lifecycle synchronization are not wired yet.
- No repository plan file named `PLAN` was found. The closest current design sources are `docs/future/github_integration_future.md`, `docs/future/webhook_implementation.md`, `docs/future/issues.md`, and `docs/revise.md`.
- The code currently uploads an image tar rather than pushing an OCI image to a registry. Since the registry is undecided, the registry must be an interface and no provider-specific assumptions should enter the domain model.

## Target architecture

```text
GitHub App webhook
  -> server webhook endpoint
  -> verify HMAC signature and delivery identity
  -> persist/dedupe delivery
  -> resolve repository/project
  -> coalesce or enqueue BuildRequest on RabbitMQ
  -> worker claims job
  -> clone exact commit SHA
  -> resolve explicit commands or scanner fallback
  -> run bounded Buildah build
  -> retain local image and metadata for now
  -> optionally export OCI archive through an artifact adapter
  -> publish status and build-log chunks
  -> atomically finalize deployment record
  -> remove workspace, temporary archive, and build container state
```

### Responsibility boundaries

1. **GitHub webhook adapter in `server`**
   - Accept only supported GitHub events, initially `push` and the required installation/repository lifecycle events.
   - Verify `X-Hub-Signature-256` against the raw request body before parsing.
   - Require and persist `X-GitHub-Delivery`; reject or ignore already-processed delivery IDs.
   - Return a fast 2xx after durable acceptance/queueing. Do not build synchronously in the HTTP request.
   - Extract repository identity, ref, before/after SHA, and changed/deleted state.

2. **Repository/project synchronization**
   - Add a durable repository/project model in PostgreSQL for selected repositories and deployment configuration.
   - Use Redis as a short-lived index/cache and delivery/build coalescing mechanism, not the sole source of truth.
   - On installation or repository events, upsert repository records and mark deleted/removed repositories inactive rather than silently losing deployment history.
   - Do not call GitHub repeatedly from the webhook request. Use the event payload for changes and reconcile asynchronously when the payload is insufficient.

3. **Deployment command bus**
   - Extend the existing RabbitMQ job envelope rather than introducing a Redis queue in parallel.
   - Add `event_id`, `repository_id`, `deployment_id`, `commit_sha`, `ref`, `trigger`, and a configuration/version reference.
   - Use publisher confirms on the server and manual acknowledgement on the worker.
   - Make jobs idempotent using a unique key such as `(repository_id, commit_sha, configuration_version, trigger)`.
   - Coalesce stale push jobs for the same repository/ref when a newer SHA is already queued, while never coalescing an explicitly requested `/deploy` unless product semantics say it is safe.

4. **Build executor**
   - Introduce a `BuildExecutor` interface in the worker domain and a `BuildahExecutor` adapter in infrastructure/deployments.
   - Keep source checkout, command policy, Buildah invocation, image inspection, cleanup, and future publishing separate.
   - Prefer `buildah bud` for Dockerfile-compatible builds initially. The current framework scanner should generate a build definition only when the repository does not provide an approved build definition.
   - Treat client-provided build/run commands as untrusted data. Store them as configuration, validate them against an allowlist/length policy, and execute only inside the build context with explicit timeouts and resource limits. Do not pass an arbitrary shell string to `sh -c` without a deliberate security boundary.
   - The `run` command is deployment/runtime metadata, not something the image builder should execute on the worker. A build job should build and inspect; runtime execution belongs to a later runtime/deployment component.

5. **Image and registry abstraction**
   - Define an `ImageStore`/`ImagePublisher` interface with operations such as build, inspect digest, export, and publish.
   - Implement local Buildah storage first. Keep the image tagged with a content-addressed deployment identity and record the resulting digest.
   - Because no registry is selected, do not make the first milestone depend on registry credentials or a public image URL.
   - If images must survive worker replacement before a registry exists, export an OCI archive to object storage with a short-lived, deployment-specific key. Make that an explicit fallback, not the permanent image distribution design.
   - When a registry is selected, add a publisher that pushes by digest and records the immutable reference. Never use `latest` as the deployment identity.

6. **Logs and status**
   - Capture Buildah stdout/stderr through an `io.Writer` that both emits structured status events and persists a bounded log stream.
   - Store logs in object storage or a dedicated log store, with PostgreSQL retaining status, byte count, truncation state, and log URL/reference.
   - Publish progress as best-effort events; status transitions must be durable and idempotent.
   - Add `queued`, `cloning`, `detecting`, `building`, `exporting`, `publishing`, `success`, `failed`, and `canceled` states only if the frontend needs them; otherwise retain a small durable state machine and expose phase metadata separately.

## Major decisions and trade-offs

### Buildah in `worker-server`

**Decision:** Yes, use Buildah through a narrow executor adapter, but run it in a dedicated build worker boundary rather than treating the general application process as a trusted build sandbox.

**Pros**
- OCI-native image construction and registry interoperability.
- No Docker daemon dependency in the build path.
- Rootless operation is available and reduces host exposure compared with a privileged Docker socket.
- The CLI/process boundary makes the executor replaceable with BuildKit, a remote builder, or a Kubernetes Job later.
- Build output can be streamed directly from stdout/stderr.

**Cons and risks**
- Buildah is not a security sandbox for hostile source code. A build can consume CPU, memory, disk, network, and process slots, and container/image tooling has a host-kernel attack surface.
- Rootless builds require correct user namespaces, subordinate UID/GID ranges, storage configuration, and writable temporary storage. Misconfiguration commonly appears as permission, overlay, or storage-driver failures.
- Building inside a container may require additional kernel capabilities, `/dev/fuse`, or a supported storage setup. Avoid privileged host mounts and Docker socket access.
- Buildah's local image store is node-local and is lost or unavailable after worker replacement unless exported or pushed.
- Layer caching improves speed but increases disk usage and can cause cross-tenant cache leakage if cache ownership is not controlled.

**Required controls**
- Run as a dedicated non-root user where supported; use rootless mode as the default.
- Put the build process in a dedicated worker pool/VM/pod with no access to the server's Docker socket, application secrets, or host filesystem.
- Apply per-job wall-clock timeout, CPU quota, memory limit, process/PID limit, network policy, and ephemeral-storage quota.
- Use a unique workspace and unique Buildah storage/run root per job or per isolated worker boundary.
- Clean up with cancellation-safe cleanup: kill the process group, remove Buildah working containers, remove workspace/archive, and record cleanup failures for operator metrics.
- Pin and regularly patch Buildah, the base images, and the worker OS.
- Disable or tightly restrict network access during build unless dependency installation requires it; never expose cloud or registry credentials to arbitrary build commands.

### Redis for repository/webhook scanning

**Decision:** Redis is a cache/deduplication/coalescing layer; PostgreSQL remains the durable repository/deployment state store; RabbitMQ remains the deployment command queue initially.

**Pros**
- Prevents repeated GitHub API calls and supports fast delivery-ID dedupe.
- Can coalesce many pushes to the same repository/ref into the newest desired SHA.
- Can later provide a Redis Streams-based ingestion path without changing the repository model.

**Cons**
- A Redis-only repository inventory loses state unless persistence and recovery are designed carefully.
- Running both Redis and RabbitMQ as queues increases operational complexity and creates ambiguous delivery semantics.
- Cache invalidation and webhook ordering are difficult; a push event can arrive before a repository installation update or arrive more than once.

**Required controls**
- PostgreSQL unique constraints and upserts for repository identity.
- Redis keys with TTL and explicit event IDs; never rely on TTL alone for correctness.
- A reconciliation job using GitHub API pagination to repair missed, deleted, or out-of-order events.
- A documented policy for force pushes, branch deletion, repository renames, and installation removal.

### Build from explicit commands versus scanner fallback

**Decision:** Use a versioned project configuration with optional `build` and `start` metadata, but keep the scanner as a conservative fallback.

**Pros**
- Explicit commands handle supported custom projects and match the intended Vercel-like UX.
- Scanner fallback preserves current zero-configuration behavior.
- Versioned configuration makes a build reproducible and allows safe cache invalidation.

**Cons**
- Arbitrary commands are code execution and must be treated as hostile.
- A scanner can select incorrect frameworks and produce surprising builds.
- Supporting multiple package managers and monorepos expands test and maintenance scope.

**Required controls**
- Define an explicit schema: build command, output directory, install command, runtime command, working directory, environment-variable references, and configuration version.
- Validate paths so commands cannot escape the checked-out repository.
- Do not persist plaintext secrets in build configuration.
- Record the exact resolved configuration in the deployment so a rebuild is auditable.
- Fail closed when the scanner is uncertain rather than silently generating a potentially unsafe template.

### Push webhook builds

**Decision:** Build the exact `after` SHA from the push payload and create an immutable deployment/build record. Do not mutate an existing image in place.

**Pros**
- Reproducible builds and reliable rollback.
- Duplicate webhook deliveries become harmless.
- Commit SHA is a stable correlation key across GitHub, RabbitMQ, logs, and images.
- Layer cache can still reuse previous layers without changing correctness.

**Cons**
- A new image is produced for every accepted SHA unless coalescing skips stale queued pushes.
- A force push or deleted branch requires explicit policy.
- The commit may disappear from a public ref; the worker must clone/fetch by SHA while GitHub still permits access.

**Required controls**
- Verify the repository belongs to the installed GitHub App/user before queueing.
- Check the event's repository ID and installation ID against local records.
- Reject unsupported ref types and ignored paths according to project configuration.
- Deduplicate by `X-GitHub-Delivery` and separately by repository/SHA/configuration to cover retries and semantically duplicate events.
- Enforce ordering or newest-SHA coalescing per repository/ref; never allow an older push to overwrite the desired deployment state.

### Local-only image storage before registry selection

**Decision:** Allow local builds for the first milestone, but explicitly label the image as non-portable and retain a digest/archive only if later retrieval is required.

**Pros**
- Unblocks Buildah integration without selecting a vendor.
- Avoids leaking registry assumptions into domain contracts.
- Makes local integration tests and executor development simpler.

**Cons**
- A local image cannot be pulled by another worker or runtime after rescheduling.
- Worker disk grows unless image retention and pruning are enforced.
- A successful build may be unusable after worker restart.

**Required controls**
- Record node/worker identity, Buildah image ID, manifest digest, and retention deadline.
- Add an explicit `image_not_published`/`local_only` result state rather than reporting a deployable success.
- Implement quota-aware pruning of completed local images and stale layers.
- Select a registry before introducing multiple workers or runtime deployment.

## Webhook-to-worker contract

The main server should not call the worker over an ad-hoc HTTP request for each push. The reliable path is:

1. GitHub sends the webhook to the main server.
2. The main server reads the raw body, validates the HMAC signature, validates event type, and extracts `X-GitHub-Delivery`.
3. The server transactionally records the delivery or uses a unique constraint to identify a duplicate.
4. The server resolves the local repository/project and creates an idempotent build/deployment record for the exact SHA.
5. The server publishes a persistent RabbitMQ message using publisher confirms.
6. The worker consumes with manual acknowledgement and bounded prefetch/concurrency.
7. The worker reports status/log events with the same deployment ID and event ID.
8. The worker acknowledges only after the durable terminal state is recorded. On crash, RabbitMQ redelivers; the worker must detect an already-terminal deployment and acknowledge without rebuilding.

Checks required before enqueueing:

- HMAC signature with constant-time comparison.
- Maximum request body size and strict JSON decoding.
- `X-GitHub-Delivery` presence, uniqueness, and replay window.
- GitHub event/action allowlist.
- Installation ID/repository ID ownership check.
- Repository active/not uninstalled and branch/ref policy.
- SHA format and `before`/`after` handling, including branch deletion and force-push policy.
- Idempotency key and unique database constraint.
- Publisher confirmation and a recovery path if database commit succeeds but message publish fails. Prefer an outbox table/dispatcher for this dual-write problem.
- Authentication on any internal worker HTTP endpoint; if RabbitMQ is used, the worker need not expose a public job endpoint.

## Capacity and memory assessment

### Is Buildah in the worker safe?

It can be made operationally safe with a dedicated, non-root, resource-limited build boundary. It is not safe to assume that simply replacing `docker build` with `buildah bud` makes arbitrary GitHub repositories safe. The build boundary must be treated similarly to CI infrastructure.

### Are there memory leaks?

The main risk is resource retention, not only a Go heap leak: child processes, Buildah working containers, image layers, temporary workspaces, tar/OCI archives, log buffers, and RabbitMQ deliveries can remain after cancellation or failure. Use context deadlines, process-group cancellation, bounded log writers, `defer` cleanup, explicit Buildah cleanup, disk quotas, and metrics for workspace/image-store bytes. Add tests for cancellation during clone, build, export, upload, and worker shutdown.

### Is this fine for 100 users at once?

Not as one unconstrained worker. One worker with prefetch 1 serializes builds; increasing concurrency to 100 would likely exhaust memory, CPU, disk, network, or PID capacity. The scalable design is a durable queue plus a bounded worker pool, admission control, per-tenant quotas, maximum concurrent builds, and eventually one isolated build pod/job per build. Validate the actual limit with representative repositories and resource measurements; do not promise 100 simultaneous builds until load tests pass.

### Can this move to another pod orchestrator later?

Yes, if the worker contract is kept stateless and queue-driven. Store all durable state in PostgreSQL/object storage/registry, treat local Buildah storage as a cache, and keep the executor behind an interface. Later, a Kubernetes Job, OpenShift Pipelines task, remote BuildKit service, or another builder can consume the same build request and report the same deployment ID. Avoid relying on host paths, Docker sockets, process-local queues, or mutable local image names.

## Proposed implementation stages

### Stage 1: contracts and persistence

- Add repository/project/deployment configuration entities and migrations.
- Extend deployment records with repository ID, commit SHA, ref, trigger, configuration version, image digest, worker ID, event ID, and log reference.
- Add an outbox or equivalent durable publish mechanism for deployment jobs.
- Define the versioned `BuildRequest`, `BuildResult`, `BuildLogEvent`, `BuildExecutor`, and `ImagePublisher` interfaces.
- Keep the current RabbitMQ exchange/queue names compatible while versioning the message schema.

### Stage 2: repository synchronization and webhook intake

- Add GitHub webhook route and raw-body signature middleware.
- Implement delivery-ID deduplication and repository/install ownership validation.
- Implement repository upsert/deactivation and asynchronous reconciliation.
- Implement push-to-build-request conversion with exact SHA and stale-push coalescing.
- Add tests for duplicate, invalid signature, unknown repository, branch deletion, force push, and out-of-order delivery.

### Stage 3: Buildah executor behind feature flag

- Replace Docker client/CLI calls in `worker-server/internal/deployments/deployments.go` with the executor interface.
- Checkout the requested SHA, not only the default branch tip.
- Add explicit command resolution and scanner fallback.
- Invoke Buildah with an argument vector, not an interpolated shell command.
- Stream bounded stdout/stderr to structured logs and status events.
- Add timeout, cancellation, workspace isolation, per-job storage, cleanup, and image inspection.
- Keep local-only image result explicit; do not claim registry publication.

### Stage 4: worker reliability and resource controls

- Add publisher confirms, worker reconnection behavior, retry/backoff, dead-letter handling, and terminal-state idempotency.
- Replace the current manual re-publish retry path with a clear retry policy that avoids duplicate simultaneous processing.
- Add configurable bounded concurrency only after resource measurement; start conservatively.
- Add metrics for queue depth, build duration, CPU/memory/storage usage, cleanup failures, retries, and log truncation.
- Add integration tests using a controlled Buildah-capable Linux environment.

### Stage 5: log retrieval and registry adapter

- Persist logs in chunks and expose an authenticated deployment-log endpoint or signed object URL.
- Add a registry implementation only after vendor selection, using digest-addressed pushes and credential isolation.
- Make the runtime/deployment service consume the immutable image reference.
- Add garbage collection and retention policies for local layers, archives, logs, and registry images.

## Verification plan

- Unit-test webhook signature validation, delivery dedupe, event parsing, repository ownership, SHA/ref policy, configuration validation, image naming, and retry/idempotency decisions.
- Unit-test the executor with a fake command runner and fake image store; assert argument safety, timeout cancellation, bounded logs, and cleanup on every failure path.
- Run Buildah integration tests on Linux for rootless Dockerfile builds, generated build definitions, cache reuse, OCI export, failed builds, and cancellation.
- Test RabbitMQ behavior with manual ack, publisher confirms, worker crash before ack, duplicate job delivery, dead-letter routing, and bounded prefetch.
- Test PostgreSQL outbox recovery when publish fails after transaction commit.
- Load-test with representative small/medium/large repositories while measuring peak RSS, disk, inodes, PIDs, CPU, network, queue depth, and build latency. Test bursty 100-user traffic as queue admission, not as 100 unconstrained simultaneous Buildah processes.
- Verify that a worker restart does not lose durable deployment state and that a replacement worker can rebuild or retrieve the image through the selected artifact/registry path.

## Server implementation review — 12-08

The detailed executable server, client, and worker TODO is in [plan-server-12-08.md](plan-server-12-08.md). It adds these current-code-specific requirements to the staged plan above:

- The existing webhook parser is not routed; the new public Echo route must bypass the current global JWT middleware while retaining request logging and use HMAC authentication.
- Repository listing should be served from GitHub's installation-repositories API through a short-TTL Redis cache. PostgreSQL should persist a user-selected project and its configuration, not a copy of every repository available to an installation; the client must use only server APIs for the picker and build state.
- Add one forward-only Goose migration; do not modify the already-applied deployment migration. It must introduce selected-project state, webhook-delivery deduplication, deployment metadata/idempotency, and a transactional deployment outbox.
- The current server-to-worker job contains only a clone URL and worker cloning builds the moving branch tip. Version the job contract and require the worker to checkout the webhook's exact `after` SHA before enabling push-triggered builds.
- The current direct database-write then RabbitMQ-publish sequence and auto-ack status consumer are insufficient for reliable deployments. Use an outbox dispatcher with publisher confirms, idempotent status transitions, and recovery/DLQ handling.

## Sources consulted

- GitHub webhook best practices: https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks
- GitHub webhook signature validation: https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries
- RabbitMQ consumer acknowledgements and publisher confirms: https://www.rabbitmq.com/docs/confirms
- RabbitMQ consumers and prefetch: https://www.rabbitmq.com/docs/consumers
- Redis job queues, visibility timeout, and reclaiming work: https://redis.io/docs/latest/develop/use-cases/job-queue/
- Redis Streams consumer groups and retries: https://redis.io/tutorials/redis-backed-job-queue-for-background-workers/
- Buildah project: https://buildah.io/
- Buildah OCI image building and registry push documentation: https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/9/htmlsingle/building_running_and_managing_containers/assembly_building-container-images-with-buildah
- Rootless/unprivileged Buildah security guidance: https://docs.redhat.com/en/documentation_red_hat_openshift_pipelines/1.20/html/securing_openshift_pipelines/unprivileged-building-of-container-images-using-buildah
