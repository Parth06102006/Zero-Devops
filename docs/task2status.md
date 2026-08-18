# Task 2 — Repository Listing & Project Selection API — Status

> Scope: "Repository listing and project selection API" from `docs/plan-server-12-08.md`.
> This document records what was implemented for Task 2, how and why it was done,
> and what remains for the future. It reflects the state of the **current session**
> (continue-task work) on top of the earlier Task 2 foundation.

---

## 1. Why this task exists

The legacy deploy flow accepted a bare `repo_id` and reached GitHub ad-hoc, with
JWT / installation-token creation duplicated in multiple handlers and **no**
server-side authorization or configuration snapshotting. Task 2 replaces that with:

- An **authenticated installation-scoped repository picker** so a caller can only
  see repositories that belong to their *active* GitHub App installation.
- A **Project** entity that captures the selected repository + a single configured
  branch + an approved build command, so webhooks and manual builds run against an
  explicit, server-authorized configuration rather than client-supplied input.
- Reuse of a single **installation-token provider** behind a domain interface so
  token/JWT logic lives in exactly one place.
- A **manual build** endpoint that resolves `sha_or_ref` to an immutable commit SHA
  server-side and publishes a complete V1 `deploy.jobs` build request, while the
  legacy `POST /deploy` stays **disabled (fails closed)** because it cannot supply
  the required immutable V1 inputs.

---

## 2. What was done

### 2.1 Domain contracts (`server/internal/domain/`)
- `project.go` — `Project`, `BuildConfiguration`, `CommandScanResult`,
  `CreateProjectParams`, `UpdateProjectParams`, `ProjectUsecase`,
  `ProjectRepository`, and error sentinels (`ErrNotFound`, `ErrConflict`,
  `ErrBadParamInput`, `ErrCommandDenied`).
- `github.go` — `GithubUsecase` gained `ListRepositories`,
  `GetRepositoryDetails`, and `InvalidateRepositoryCache`; `GithubRepositoryClient`
  gained `ResolveCommit`. Added `RepositoryList`, `RepositoryPicker`,
  `GithubInstallation` types used by the picker.
- `deployment.go` — `Deployment` extended with project/manual-build columns
  (`project_id`, `github_installation_id`, `commit_sha`, `requested_ref`,
  `trigger`, `desired_revision_generation`, `configuration_snapshot`,
  `configuration_version`, `command_policy_version`, `command_scan_result`,
  `manual_idempotency_key`); added `CreateProjectBuildParams`,
  `DeploymentUsecase.CreateProjectBuild`, `DeploymentRepository.StoreProjectBuild`.

### 2.2 Installation-token provider (`server/internal/integrations/scm/github/token/provider.go`)
- `NewInstallationTokenProvider(appID, privateKeyPath)` + `CreateInstallationToken(ctx, installationID)`
  signs a JWT (`iss` = static App ID) and exchanges it for a short-lived
  installation token. Constructed once in `main.go` and injected into the GitHub
  usecase **and** the deployment usecase — no token logic in handlers.

### 2.3 Repository listing (SCM)
- `usecase/github_ucase.go` — `ListRepositories`:
  - Verifies the caller has an **active** installation (else `ErrInvalidStatus`).
  - Normalizes the query (`fields.ToLower`, collapsed whitespace).
  - Checks the Redis cache by an **installation-isolated** key.
  - On miss/error, obtains a token, calls the repository client, and caches only
    the picker DTO with a TTL.
  - **Single-flight cache stampede protection** (`repoListCalls` keyed by cache key)
    so concurrent identical requests hit GitHub once.
  - Dependency nil-guards return `ErrInternalServerError` instead of panicking.
  - New `InvalidateRepositoryCache(ctx, installationID)` for webhook-driven eviction.
- `client/repositories.go`:
  - `ListRepositories` against `/installation/repositories` with opaque base64 page
    cursors, `per_page` clamped to 1..100, `io.LimitReader` 2 MiB cap, local
    filtering by normalized query (installations endpoint has no search), `NextCursor`
    set only when the page is full.
  - `GetRepositoryDetails` maps 404 → `ErrNotFound`.
  - `ResolveCommit` → `GET /repos/{owner}/{repo}/commits/{shaOrRef}`; validates the
    returned SHA is a 40-char lowercase hex string; 404 → `ErrNotFound`.
- `cache/redis.go` (new) — `RepositoryListCache` interface (`Get`/`Set`/`InvalidateInstallation`)
  with a `RedisRepositoryListCache` adapter. `InvalidateInstallation` deletes every
  key matching `github:installation:{id}:repositories:v1:*` via SCAN. Redis is a
  **cache only**: it never stores tokens/secrets and a backend failure is treated as
  a miss. Added `github.com/alicebob/miniredis/v2` for cache tests.

### 2.4 Project selection CRUD (`server/internal/project/...`)
- `usecase/project_ucase.go` — `CreateProject` verifies repository visibility via
  `GetRepositoryDetails`, loads the installation, scans the build command
  (denied → `ErrCommandDenied`), normalizes the branch to `refs/heads/*`, and
  stores with `ConfigurationVersion = 1`. `ListProjects` / `GetProject` / `UpdateProject`
  (re-normalizes branch, re-scans config, bumps policy version on change) / `DeleteProject`.
- `repository/pgsql/pgsql_project.go` — `Store`/`ListByUserID`/`GetByID`/`Update`/`Delete`;
  PostgreSQL unique-violation (`23505`) on the duplicate `(user_id, github_repository_id)`
  selection is mapped to `domain.ErrConflict`.
- `delivery/http/project_handler.go` — authenticated `GET /projects`,
  `POST /projects`, `GET /projects/:id`, `PATCH /projects/:id`,
  `DELETE /projects/:id`. Validates create/update bodies, scopes every call by the
  authenticated `user_id`, and maps `ErrNotFound`→404, `ErrConflict`→409,
  `ErrBadParamInput`→400, `ErrCommandDenied`→422.

### 2.5 Manual build (`server/internal/deployments/...`)
- `usecase/deployment_ucase.go` — `CreateProjectBuild`:
  - Validates `project_id`, `sha_or_ref`, and a UUID `idempotency_key`
    (`ErrBadParamInput` otherwise).
  - Loads the project; loads the caller's installation; **active** check
    (`ErrInvalidStatus`); **ownership** check (`project.InstallationID == installation.ID`,
    else `ErrNotFound` — Redis never decides authorization).
  - Resolves `sha_or_ref` → immutable commit SHA via `ResolveCommit`.
  - Snapshots `BuildConfiguration` + scan result, stores a `deployments` row, and
    publishes a complete V1 `deploy.jobs` `BuildRequestV1` (`TriggerManual`).
- `repository/pgsql/pgsql_deployment.go` — `StoreProjectBuild` writes the
  project/manual-build columns and maps duplicate `manual_idempotency_key`
  (`23505`) → `domain.ErrConflict`.
- `delivery/http/deployment_handler.go` — `POST /projects/:id/builds` (auth-scoped,
  `201` on success, `400`/`409` mapped). Legacy `POST /deploy` still fails closed.

### 2.6 Wiring (`server/cmd/server/main.go`)
- Builds Redis client, token provider, repository client, GitHub usecase (with cache),
  project repo/usecase/scanner, registers project routes, and passes `projectRepo` +
  `repositoryClient` into the deployment usecase for manual builds.

### 2.7 Tests added / fixed (this session)
| File | Scenarios |
|---|---|
| `integrations/scm/github/usecase/github_ucase_list_test.go` | inactive installation → `ErrInvalidStatus`; cache miss fetches + caches; cache hit skips client; cache failure falls through; pagination param forwarding + next cursor; **key isolation by installation**; **single-flight**; token/authorization failure; missing deps fail closed; `InvalidateRepositoryCache` no-op without cache; **webhook-driven invalidation removes keys** |
| `integrations/scm/github/client/repositories_test.go` | parse + local query filter; invalid cursor → `ErrBadParamInput`; `GetRepositoryDetails` mapping + 404; `ResolveCommit` SHA normalization, empty ref, 404, invalid-SHA rejection (httptest mocks) |
| `integrations/scm/github/cache/redis_test.go` | miss returns nil (+`redis.Nil`); set→get round-trip; `InvalidateInstallation` evicts only the target installation (miniredis) |
| `project/usecase/project_ucase_test.go` | success + branch normalization + `ConfigurationVersion=1`; **duplicate selection → `ErrConflict`**; **repository not visible → `ErrNotFound`**; **command denied → `ErrCommandDenied`**; `toFullRef` table |
| `project/repository/pgsql/pgsql_project_test.go` | Store→GetByID→duplicate-selection `ErrConflict` round-trip (skips unless `DATABASE_TEST_URL` set) |
| `deployments/usecase/deployment_ucase_build_test.go` | **manual-deploy authorization**: missing idempotency key → `ErrBadParamInput`; project not found → `ErrNotFound`; inactive installation → `ErrInvalidStatus`; installation mismatch → `ErrNotFound` |

Also fixed stale mocks/tests broken by the new `InvalidateRepositoryCache` interface
method and removed several empty (0-byte) `*_test.go` files that were blocking
`go test ./...`.

---

## 3. How it was implemented (key decisions)

1. **Redis is a cache, never a source of truth.** A miss *or* any backend error is
   treated identically as "go to GitHub". No installation tokens, webhook secrets,
   cookies, or credentials are ever written to Redis. TTL is only a fallback;
   authorization is always re-verified against the DB + GitHub.
2. **Cache key is installation-isolated**:
   `github:installation:{installation_id}:repositories:v1:{sha256(cursor|query|perPage)}`.
   This guarantees one installation can never read another's picker data, and
   invalidation can target a single installation via SCAN.
3. **Single-flight** prevents cache-stampede when many requests arrive for the same
   (installation, cursor, query, perPage) at once.
4. **Repository ownership is enforced server-side**: before creating a project, the
   usecase confirms the `repo_id` is visible to the caller's active installation;
   before a manual build, it confirms the project belongs to that installation. The
   persisted `repo_id` is used only transiently; the project/branch/config are the
   durable inputs.
5. **Immutable inputs**: `sha_or_ref` is resolved to a concrete commit SHA before
   enqueueing, and the full resolved V1 `BuildRequestV1` (clone URL, SHA, config,
   policy version) is published — satisfying the "no migration period / complete V1
   inputs" requirement of the plan and `docs/deploy-jobs-contract.md`.
6. **Legacy path disabled**: `POST /deploy` returns HTTP 409 because it lacks the
   required V1 immutable inputs, keeping the surface safe during migration.
7. **Injected cache interface** (`RepositoryListCache`) lets Redis behavior be unit
   tested with an in-memory fake and a real Redis via `miniredis`, without a live
   server in CI for the usecase tests.

---

## 4. What is planned for the future (remaining work)

- **Webhook-driven cache invalidation wiring (Task 4/5).** The
  `InvalidateRepositoryCache` method exists and is unit-tested, but the webhook
  handler is not yet wired to call it on `installation_repositories`,
  installation `suspend`/`delete`, project-access failure, and explicit reconnect.
  Until then, TTL is the only eviction path.
- **Outbox dispatcher.** `deployment_outbox` exists; `CreateProjectBuild` currently
  publishes the V1 job inline. A durable outbox worker (Phase 5) should drain the
  outbox so a publish failure can be retried without losing the build request.
- **End-to-end / integration coverage.** `pgsql_project_test.go` is skip-based
  (needs `DATABASE_TEST_URL` + applied migrations). Add a CI job that runs the pgsql
  and deployment-outbox integration tests against a real Postgres.
- **Observability.** Add cache hit/miss and stampede metrics so the caching layer
  can be tuned in production.
- **Legacy cleanup.** Once all callers use the project-build API, remove the legacy
  `CreateDeployment` repo-only path and its now-unreachable code.

---

## 5. Verification

- `GOCACHE=$(pwd)/.gocache go test ./...` — **passes** (exit 0, no failures).
- `go build ./...` — clean.
- New dependencies: `github.com/alicebob/miniredis/v2` (test-only), already present
  `github.com/redis/go-redis/v9`.
