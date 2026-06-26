# Daily Tasks

Date: 11 June 2026
Source reviewed: `server/docs/revise.md`

This file converts the revision notes into an implementation workflow. It keeps the completed work visible, calls out shortcomings that still need attention, and lists the next tasks in the order they should be handled.

## Current Backend Direction

- Keep GitHub OAuth as the only login path for now.
- Do not add custom email/password signup yet.
- Keep GitHub OAuth login and GitHub App installation as separate flows connected by the local `user_id`.
- Keep clean architecture boundaries strict:
  - `domain` defines contracts and models.
  - `usecase` owns business flow.
  - `repository` and provider packages handle external systems.
  - `delivery/http` translates HTTP requests into usecase calls.
- App access and refresh tokens must be owned by the backend, not GitHub.

## Implemented Or Mostly Implemented

- `domain/auth.go` defines OAuth provider, token response, auth usecase, and safe user response contracts.
- `domain/user.go` defines the authorization user model and repository contract.
- `domain/github.go` uses the corrected `GithubInstallation`, `GithubRepository`, and `GithubUsecase` naming.
- User repository supports provider lookup, user creation, `GetByID`, and refresh-token update.
- Auth usecase supports GitHub OAuth callback, app JWT generation, refresh-token rotation, logout, and current-user lookup.
- Auth HTTP handler implements:
  - `POST /auth/github/login`
  - `POST /auth/refresh`
  - `POST /auth/logout`
  - `GET /auth/user/me`
- Auth middleware validates the `access_token` cookie, loads the user, and stores `user_id` in Echo context.
- `app/main.go` wires the user repository, auth middleware, GitHub OAuth provider, auth usecase, and auth HTTP handler.
- GitHub installation repository has store, get-by-user, and delete-by-user behavior drafted.
- Database migrations were added for the current auth and GitHub integration model.
- Auth APIs were manually exercised after migrations: login, refresh, logout, and current-user lookup.
- Root `README.md` was updated with project-specific backend/client documentation.

## Known Shortcomings From The Revision

### Auth Shortcomings

- `HandleOAuthCallback` must explicitly handle `domain.ErrNotFound` from provider lookup instead of deciding new-user flow by checking `existingUser.ID == 0`.
- New OAuth users must have the generated refresh token persisted immediately after `Store`; otherwise the first refresh after signup can fail.
- Refresh tokens are still stored raw. This is acceptable only for local development and should be replaced with hashed refresh-token storage before production.
- `getStatusCode` still needs better mappings:
  - invalid or missing token: `401`
  - bad OAuth/provider input: `400`
  - not found: `404`
  - conflict: `409`
  - missing secrets or server configuration: `500`
- `Logout` and `GetCurrentUser` duplicate access-token parsing. Extract a shared private helper.
- Current route is `GET /auth/user/me`; final route should be `GET /auth/me`.
- Middleware currently skips only `/auth/github/login` and `/auth/refresh`; future public routes must be added deliberately.

### GitHub Installation Shortcomings

- GitHub installation HTTP delivery handler is not done yet.
- `githubRepo` and `userRepo` still need to be wired into `NewGithubAppUsecase` from `app/main.go` once the handler exists.
- GitHub API calls need HTTP status-code checks after:
  - GitHub OAuth access-token request.
  - GitHub installations request.
- The error from `http.NewRequest` for the installations request must be checked before using the request.
- Add either a domain error for failed GitHub installation fetches or return a clear formatted HTTP-status error from the GitHub usecase.
- Confirm whether installation delete should be idempotent or should return `domain.ErrNotFound` when no row exists.
- Installation storage should use an upsert if only one GitHub App installation per user is allowed.

### Database And Migration Shortcomings

- Verify migrations match repository queries exactly.
- `github_installations.account_name` is being replaced by `account_type` and `account_login`.
- `created_at` and `updated_at` are being added to align with `domain.GithubInstallation`.
- Finalize rollback behavior before relying on the migration.
- Decide whether rollback should keep `account_name` as an empty string or rebuild it from `account_login`.

### Documentation And Verification Shortcomings

- `server/README.md` is still stale upstream template text and should be replaced with backend-specific documentation.
- Auth API test cases need to be documented, especially cookie behavior for login, refresh, logout, and current-user lookup.
- Re-run verification with repo-local Go cache.
- Re-run Goose status/up/down after exporting `GOOSE_DRIVER` and `GOOSE_DBSTRING`.

## Priority Workflow

### Day 1 - Make Auth Flow Correct

- [ ] Fix `HandleOAuthCallback` to branch on `domain.ErrNotFound`.
- [ ] Return unexpected repository errors instead of silently treating them as new users.
- [ ] Persist refresh token immediately after creating a new OAuth user.
- [ ] Re-test first-time GitHub OAuth signup followed by `POST /auth/refresh`.
- [ ] Improve auth error-to-status-code mapping in `auth_handler.go`.
- [ ] Extract a shared helper for access-token parsing used by logout and current-user lookup.
- [ ] Rename `GET /auth/user/me` to `GET /auth/me`, or temporarily support both routes during transition.

### Day 2 - Finish GitHub Installation Usecase

- [ ] Add a clear error path for failed GitHub installation fetches.
- [ ] Add HTTP status-code checks for GitHub token and installation API calls.
- [ ] Check `http.NewRequest` errors before using the installation request.
- [ ] Confirm `NewGithubAppUsecase` accepts `domain.GithubRepository` and `domain.UserRepository`.
- [ ] Ensure the usecase returns `domain.GithubUsecase`, not `*domain.GithubUsecase`.
- [ ] Decide and document delete behavior for missing installations.

### Day 3 - Finish GitHub Installation Persistence

- [ ] Compare GitHub installation migration columns against `domain.GithubInstallation`.
- [ ] Compare repository SQL against the migration column names and constraints.
- [ ] Replace insert-only behavior with `INSERT ... ON CONFLICT ... DO UPDATE` if one installation per user is allowed.
- [ ] Ensure Postgres inserts use `RETURNING id` with `Scan(&inst.ID)`.
- [ ] Finalize the down migration for restoring `account_name`.
- [ ] Run Goose migration status/up/down locally.

Recommended down migration shape from `revise.md`:

```sql
-- +goose Down
ALTER TABLE github_installations
    DROP COLUMN account_type,
    DROP COLUMN account_login,
    DROP COLUMN created_at,
    DROP COLUMN updated_at,
    ADD COLUMN account_name TEXT NOT NULL DEFAULT '';
```

### Day 4 - Add GitHub Installation HTTP APIs

- [ ] Create the GitHub installation delivery handler.
- [ ] Add protected routes:
  - `POST /integrations/github/installations`
  - `GET /integrations/github/installation`
  - `DELETE /integrations/github/installation`
- [ ] Read `user_id` from Echo context set by auth middleware.
- [ ] Validate request body fields for install/store.
- [ ] Call the GitHub installation usecase instead of calling repositories from handlers.
- [ ] Wire `githubRepo`, `userRepo`, GitHub usecase, and handler in `app/main.go`.
- [ ] Remove temporary `_ = githubRepo` parking once the usecase is wired.

### Day 5 - Verification And Cleanup

- [ ] Run backend tests from `server` with repo-local Go cache:

```powershell
$env:GOCACHE = (Resolve-Path .gocache).Path
go test ./...
```

- [ ] Run Goose status from `server` after exporting database settings:

```powershell
$env:GOOSE_DRIVER = "postgres"
$env:GOOSE_DBSTRING = "<postgres-connection-string>"
goose -dir migrations status
```

- [ ] Manually verify auth cookies:
  - login writes `access_token` and `refresh_token`
  - refresh rotates both cookies
  - logout clears both cookies and stored refresh token
  - `/auth/me` returns safe user data only
- [ ] Manually verify GitHub installation flow:
  - store installation for authenticated user
  - fetch current user's installation
  - delete current user's installation
  - reconnect after deletion
- [ ] Update `server/README.md` with current backend setup, routes, env vars, and migration commands.

## Final Target Route Set

Auth:

```text
POST /auth/github/login
POST /auth/refresh
POST /auth/logout
GET  /auth/me
```

GitHub integration:

```text
POST   /integrations/github/installations
GET    /integrations/github/installation
DELETE /integrations/github/installation
```

## Production Hardening Backlog

- [ ] Hash refresh tokens before storing them.
- [ ] Confirm cookie `Secure` is true in production behind HTTPS.
- [ ] Add focused tests for auth usecase.
- [ ] Add focused tests for GitHub installation usecase.
- [ ] Add handler tests for auth and GitHub integration routes.
- [ ] Decide whether GitHub email is required; if yes, call `/user/emails` with the correct scope.
- [ ] Keep custom auth deferred until GitHub OAuth and GitHub App installation are stable.

## Readiness Definition

This work is ready when:

- `go test ./...` passes with the repo-local Go cache.
- Goose migrations apply and roll back cleanly.
- First-time OAuth signup can immediately refresh without failing.
- Auth routes return correct status codes for missing, invalid, and expired tokens.
- `GET /auth/me` returns safe user data and no refresh token.
- GitHub installation routes are protected, wired through usecases, and verified against the migrated schema.
- `app/main.go` has no parked GitHub repository and no direct handler-to-repository shortcuts.
