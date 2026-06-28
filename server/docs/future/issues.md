# Future Issues And Priorities

This file lists known follow-up issues after the current auth and GitHub App installation APIs were manually tested.

Current decision:

- Do not change the current delete flow right now.
- Add installation `status` now so webhook lifecycle events can update it.
- Treat webhook handling as the next step after the status column is in place.
- Move next into repository access and deployment flow.

## Important

- Build the next product flow after GitHub App installation: list the repositories available to the installed GitHub App, let the user choose a repository, then prepare that repository for deployment.
- Use the stored GitHub `installation_id` to create a GitHub App installation access token before reading repository data.
- Add a repository listing usecase and API for the authenticated user.
- Confirm GitHub App permissions include the repository access needed for reading repository metadata and code.
- Verify the deployed environment end to end on the real domain after auth and installation are connected to repository selection.

## Strict

- Keep the GitHub OAuth login flow separate from the GitHub App installation flow.
- Keep using middleware context for `user_id`; do not store request-specific user state on handlers.
- Keep protected SCM APIs behind the access-token middleware.
- Before cloning or deploying a repository, verify that the selected repository belongs to the authenticated user's GitHub App installation.
- Do not assume installation means access to all repositories; use the installation token and GitHub's allowed repository list.

## Optional

- Rename routes later for cleaner API shape, for example `/integrations/github/installation` instead of `/integration/scm/github/`.
- Add `INSERT ... ON CONFLICT ... DO UPDATE` for GitHub installation storage if reinstalling should update the same user row.
- Improve error-to-status-code mapping in handlers so auth and SCM errors return more precise `400`, `401`, `404`, and `409` responses.
- Hash refresh tokens before production instead of storing raw refresh tokens.
- Replace or refresh older documentation sections in `revise.md` that describe now-completed compile issues.

## Mild

- Normalize spelling and naming in older docs where `installation` was previously misspelled.
- Consider renaming `GET /auth/user/me` to `GET /auth/me` later for a smaller public API.
- Clean up older historical notes in `revise.md` once the project has a dedicated changelog.
- Improve response messages for GitHub App install/delete to use consistent capitalization.

## Later: Status And Webhooks

These are deliberately not part of the repository-listing/deployment implementation, but the status column itself now exists and is ready for webhook-driven updates.

- Add GitHub webhook handling for installation lifecycle events.
- Listen for installation suspend, unsuspend, and uninstall events.
- Mark suspended installations as `suspended` in the database.
- Mark uninstalled installations as `uninstalled` in the database.
- Do not delete the user's projects, deployment history, build logs, or environment variables when installation state changes.
- Continue serving cached application data from the database even when GitHub access is no longer active.
- Block repository sync, deployments, and other GitHub API-dependent operations when installation status is not `active`.
- Treat reinstall events as reconnection events and update the existing record instead of blindly creating a duplicate row.

## Recommended Next Implementation

The next implementation should be repository access for deployments.

Planned flow:

1. User logs in with GitHub OAuth.
2. User installs the GitHub App.
3. Backend stores the GitHub App `installation_id`.
4. Backend creates an installation access token using that `installation_id`.
5. Backend lists repositories available to that installation.
6. User selects one repository.
7. Backend fetches repository metadata and prepares the deployment flow.

Suggested first API:

```text
GET /integration/scm/github/repos
```

Expected behavior:

- Requires `access_token` cookie.
- Reads `user_id` from middleware context.
- Loads the stored GitHub installation for that user.
- Creates a GitHub App installation token.
- Calls GitHub to list repositories accessible to the installation.
- Returns the repository list to the frontend.

## GitHub Installation Lifecycle Rules

These rules define how webhook-driven status updates should behave once the webhook handler is wired.

- When GitHub sends `installation_suspend`, mark the installation as `suspended`.
- When GitHub sends `installation.deleted`, mark the installation as `uninstalled`.
- When GitHub sends `installation_unsuspend`, mark the installation as `active`.
- Do not make GitHub API calls with a suspended or uninstalled installation.
- Keep showing data already stored in the application's database.
- Hidden or stale GitHub data should not erase local project or deployment records.
- The database should remain the source of truth for user/project history after uninstall.
- The GitHub installation row should be updated, not deleted, unless a future cleanup policy explicitly says otherwise.

Recommended statuses:

- `active`
- `suspended`
- `uninstalled`
- `error` optional, only for unexpected provider or authentication failures

Reinstall behavior:

- Search for the existing GitHub account or installation first.
- If the record exists, update the row and set status back to `active`.
- Refresh the installation ID, permissions, and repository access if GitHub changed them.
- Only create a new record for a truly new GitHub account that has never connected before.
- Do not rely only on the setup URL callback; treat the webhook as the authoritative lifecycle source.
