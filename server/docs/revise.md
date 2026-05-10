# Authorization And GitHub App Progress

Updated: 10 May 2026

This document is a revision note for the current authorization work. It explains what has been built, what the intended flow is, what looks wrong right now, and what should be fixed before adding the final APIs.

## Overall Progress

The project is still following the clean architecture style:

- `domain` contains the core contracts and entities.
- `authorization/auth/usecase` contains OAuth login and token session logic.
- `authorization/auth/delivery/http` is intended to expose HTTP endpoints.
- `authorization/user/repository/pgsql` stores OAuth users and refresh tokens in Postgres.
- `authorization/github/repository/pgsql` stores GitHub App installation data in Postgres.
- `authorization/auth/usecase/auth_provider` contains the GitHub OAuth provider adapter.

The important domain objects already drafted are:

- `User`: stores provider user information, username, email, avatar, created time, and refresh token.
- `OAuthUser`: normalized user profile returned by an OAuth provider.
- `TokenResponse`: app-owned access token and refresh token.
- `GithubInstallation`: intended to store the GitHub App installation ID against the local user.

The important interfaces already drafted are:

- `OAuthProvider`: exchanges OAuth code and fetches provider user profile.
- `AuthUsecase`: handles OAuth callback, refresh token, and logout.
- `UserRepository`: loads, creates, and updates authorization users.
- `GithubUsecase`: should handle GitHub App installation, lookup, and deletion.
- `GithubRepository`: persists and deletes installation records.

## Current Intended Flow

1. Frontend sends the GitHub OAuth callback `code` to the backend.
2. `AuthUsecase.HandleOAuthCallback` selects the provider from `providers["github"]`.
3. GitHub provider exchanges the code for a GitHub OAuth access token.
4. GitHub provider calls `https://api.github.com/user`.
5. Usecase checks if this GitHub user already exists locally by provider ID.
6. If the user does not exist, create a local user.
7. If the user exists, rotate the stored refresh token.
8. Backend returns app-owned access and refresh tokens.
9. Future GitHub App API will store `user_id`, `installation_id`, and `account_name`.
10. Future deletion API will remove the installation record for a user.

This separation is good. The app session token is your token, not the GitHub token. That keeps GitHub OAuth and your own session management loosely coupled.

## Today's Todo

Primary task: add the authorization API after fixing the current compile and flow issues.

Recommended authorization endpoints:

- `GET /auth/github/login`: optional endpoint that returns or redirects to the GitHub OAuth URL.
- `GET /auth/github/callback` or `POST /auth/github/callback`: receives `code`, calls `HandleOAuthCallback`, returns app tokens.
- `POST /auth/refresh`: accepts refresh token and returns new tokens.
- `POST /auth/logout`: invalidates the stored refresh token.
- `GET /auth/me`: reads the access token and returns the current user.

Second task: add GitHub App installation APIs.

Recommended GitHub App endpoints:

- `POST /integrations/github/installations`: store `installation_id`, `account_name`, and current authenticated `user_id`.
- `GET /integrations/github/installation`: return the current user's stored installation.
- `DELETE /integrations/github/installation`: delete the current user's stored installation.

Keep the route name generic enough for future apps by using `integrations`, but keep the implementation package focused on GitHub for now. Later, `integrations/slack`, `integrations/vercel`, or other apps can follow the same pattern.

## Recommendations Before Adding APIs

Fix compile issues first. The API layer will be hard to reason about while names, imports, and interfaces do not match.

Use consistent names:

- Current code has `GithubInstalltion`, `GithubInstallation`, `InstalltionID`, and `InstallationID` mixed together.
- Current code has `GithubRepositry`, `GithubRepository`, `StoreInstalltion`, and `StoreInstallation` mixed together.
- Pick one spelling everywhere: `GithubInstallation`, `InstallationID`, `GithubRepository`, `StoreInstallation`.

Keep domain independent:

- Domain interfaces should not import Echo, SQL, OAuth libraries, Viper, or GitHub SDKs.
- Usecases should depend on domain interfaces.
- Repositories and providers should depend inward on domain contracts.
- HTTP handlers should translate HTTP requests into usecase calls.

Do not store GitHub OAuth tokens as your app session. The current idea is correct: generate your own access and refresh tokens after OAuth succeeds.

Store refresh tokens carefully:

- Prefer storing a hash of the refresh token, not the raw token.
- Rotate refresh tokens during refresh.
- On logout, clear the stored refresh token or mark the session revoked.

For GitHub App installation storage:

- Make `(user_id, provider)` or `user_id` unique for GitHub if only one GitHub App installation per user is allowed.
- Use `INSERT ... ON CONFLICT ... DO UPDATE` so reinstalling or changing account does not create duplicate rows.
- Do not ask the user to install again if a valid installation exists.
- Add delete behavior that removes the installation row when the user disconnects GitHub.

## Do You Need Custom Auth?

Not right now.

For the current product flow, GitHub OAuth is enough because the main product depends on GitHub identity and GitHub App installation. Custom email/password auth would add password hashing, verification, password reset, session security, and account linking before the core integration is stable.

Add custom auth later only if:

- Users need to log in without GitHub.
- You need organization admins who are not developers.
- You want multiple login providers linked to one account.
- You need enterprise login methods such as Google Workspace or SSO.

Current recommendation: finish GitHub OAuth and GitHub App installation first. Keep the domain flexible enough to add custom auth later, but do not implement it yet.

## Loose Coupling And Decentralized Structure

Yes, you are allowed to keep it decentralized and loosely coupled. That is the point of the clean architecture direction.

The useful rule is this:

- Domain defines what the system needs.
- Usecase defines business flow.
- Repositories and providers handle external details.
- Delivery/http only handles transport.

Debate:

- A decentralized package structure gives flexibility and testability.
- Too much decentralization too early can create naming drift and half-connected interfaces.

Balanced recommendation:

- Keep auth, user, and GitHub integration as separate packages.
- Keep interface names in `domain`.
- Keep only one public constructor per concrete adapter.
- Wire everything in `app/main.go`.
- Do not let controllers call repositories directly.

## Current Issues Found In The Flow

These are important before adding controllers.

1. The code currently has compile errors.

- `domain/github.go` defines `GithubInstalltion`, but other files use `GithubInstallation`.
- `domain/github.go` defines `GithubRepositry`, but repositories return `domain.GithubRepository`.
- `UserRepository` defines `GetProviderById`, but usecase calls `GetByProviderId`.
- Repository methods return `nil` where the interface expects `domain.User`.
- Some repository files are missing `package` declarations.
- Some imports are invalid, for example `import ("context", "database/sql")` uses a comma.
- `github_provider.go` imports `domain` instead of the module path.
- `main.go` references `_authUcase`, `AuthProvider`, and `domain` without matching imports.
- `auth_handler.go` registers methods that are not implemented.

2. Token generation happens before a new user's database ID is known.

In `HandleOAuthCallback`, tokens are generated from `oauthUser`, but `generateTokens` expects a `*domain.User` and should include the local user ID. For a new user, save the user first, receive the generated ID, then create tokens.

3. Refresh-token update query does not match the current model.

The `User` model has `RefreshToken`, but `Update` tries to set `AccessToken` and `RefreshToken` while only passing two arguments. Decide whether access tokens are stateless JWTs or stored sessions. Current recommendation: do not store access tokens; only store hashed refresh tokens.

4. GitHub App installation is not the same as GitHub OAuth login.

OAuth login proves the user identity. GitHub App installation gives your app permission to work on repositories. Keep these as two separate flows connected by the local `user_id`.

5. GitHub email may be empty.

GitHub `/user` can return an empty email depending on privacy settings. If email is required, call `/user/emails` with `user:email`; otherwise make email nullable.

6. Repository result handling needs cleanup.

`QueryRowContext` does not return a closeable row. Do not call `row.Close()`. Handle `sql.ErrNoRows` and map it to `domain.ErrNotFound`.

7. PostgreSQL insert behavior needs cleanup.

`LastInsertId()` is not supported by `lib/pq`. Use `RETURNING id` and `Scan(&inst.ID)`.

## Verification Proof

`go test ./...` was run from the `server` folder on 10 May 2026. It currently fails before tests can run fully because the authorization work is not compiling yet.

Confirmed blockers from the command output:

- `app/main.go` imports `server/...` packages, but the module path is `github.com/bxcodec/go-clean-arch`.
- `authorization/auth/usecase/auth_provider/github_provider.go` imports `domain` instead of the project module path.
- `authorization/github/repository/pgsql/pgsql_github.go` is missing a `package` declaration.
- `authorization/user/repository/pgsql/pgsql_user.go` is missing a `package` declaration.
- `domain/github.go` refers to `GithubInstallation`, but the struct is currently misspelled as `GithubInstalltion`.
- `authorization/github/usercase/github_ucase.go` imports `context` but does not use it yet.
- `authorization/auth/delivery/http/auth_handler.go` registers routes but the handler methods are unfinished.

## Suggested Implementation Order

1. Normalize domain names and repository interface method names.
2. Make user repository compile and return `domain.ErrNotFound` when no row exists.
3. Fix auth usecase so it creates or loads a local user before generating tokens.
4. Implement refresh token rotation and logout.
5. Implement auth HTTP handler routes.
6. Add an access-token middleware that puts `user_id` into request context.
7. Implement GitHub installation repository with upsert, get, and delete.
8. Implement GitHub installation usecase.
9. Implement GitHub installation HTTP handler.
10. Add focused tests for auth usecase and GitHub installation usecase.

## Revision Notes

The mental model to remember:

- OAuth provider token: temporary external token used to ask GitHub who the user is.
- App access token: short-lived JWT used by your frontend to call your backend.
- App refresh token: long-lived secret used to get a new access token.
- GitHub App installation ID: permission handle for your installed GitHub App.
- Local user ID: your main stable identity inside your backend.

The clean flow should be:

```text
GitHub OAuth code
  -> OAuthProvider.ExchangeCode
  -> OAuthProvider.GetUser
  -> UserRepository.GetByProviderID
  -> UserRepository.Store or Update
  -> Generate app tokens from local user
  -> Return TokenResponse
```

GitHub App installation flow should be:

```text
Authenticated backend user
  -> GitHub App installation callback or request body
  -> GithubUsecase.StoreInstallation
  -> GithubRepository.UpsertInstallation
  -> Do not ask user to install again while record exists
```

Disconnect flow should be:

```text
Authenticated backend user
  -> GithubUsecase.DeleteInstallation
  -> GithubRepository.DeleteInstallationByUserID
  -> User can reconnect later
```

## Final Recommendation

Do not add custom auth yet. First make GitHub OAuth stable, then add the GitHub App installation API as a separate integration flow. Keep the structure decentralized, but make the names and interfaces strict. Loose coupling works best when the contracts are boring, consistent, and easy to test.
