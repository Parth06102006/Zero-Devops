# Future GitHub Integration Changes

Updated: 15 July 2026

This note tracks what remains for the GitHub App side after the current install/get/delete flow.

## Current State

- GitHub App installation records, installation status, and the install/get/delete flow are already in place.
- The remaining gap is webhook-driven lifecycle synchronization for suspend, unsuspend, and uninstall events.

## Still To Do

- Add a repository-listing flow that uses the stored `installation_id`.
- Return repository choices to the frontend after validating the authenticated user's installation.
- Decide the public route shape for the repo-listing and webhook APIs.
- Wire installation lifecycle webhook events into the status update path.
- Confirm how reinstall events should refresh or replace existing installation rows.

## Status Rules

The current status values are:

- `active`
- `suspended`
- `uninstalled`

Recommended behavior:

- keep the row when an installation is suspended
- mark it `suspended` instead of deleting it
- mark it `uninstalled` when GitHub says the app was removed
- keep deployment history and other local app data intact
- block GitHub API-dependent actions when the status is not `active`

## Suggested Follow-Up Files

- `server/domain/github.go`
- `server/integrations/scm/github/repository/pgsql/pgsql_github.go`
- `server/integrations/scm/github/usecase/github_ucase.go`
- `server/integrations/scm/delivery/http/scm_handler.go`
- `server/integrations/scm/webhook/github/webhook.go`
- `server/database/migrations/*.sql`


