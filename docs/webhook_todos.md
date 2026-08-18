# Webhook TODOs

Reference notes for the not-yet-implemented GitHub webhook processing path.
The webhook layer currently only *parses* events
(`server/internal/integrations/scm/webhook/github/webhook.go`); nothing consumes
them to trigger builds yet.

## TODO: Global installation-level webhook availability check (blocking)

When the webhook build path is implemented, a push event must **not** be able to
create a build unless the GitHub App installation is actually able to deliver
webhooks. This is a **global (installation-wide) gate** — it applies to every
project of the user, independent of any per-project setting.

### Rule

```
webhook build allowed (push event → build)
    = installation.Status == "active"
      AND project.WebhookEnabled == true
      AND pushed ref == project.ConfiguredBranch
```

- `installation.Status` is the single source of truth for "the user approved /
  still has the GitHub App webhook access":
  - `active` → webhooks are being delivered
  - `suspended` / `uninstalled` → GitHub is NOT delivering webhooks; no project
    of this user may build from a webhook event, regardless of
    `project.WebhookEnabled`.
- This check is **not** a per-project flag — it must be evaluated from
  `github_installations.status` at webhook-processing time (or at build-creation
  time). Do not persist/duplicate it on the project row.
- On denial, drop the event (or record it) but never enqueue a build.

### Where it lives

- Location to add: the webhook event consumer / build-creation path (e.g. a
  `webhook` usecase that handles `PushEventP`).
- Data to load: `GithubRepository.GetInstallationByUserID(userID)` and check
  `.Status == domain.GithubInstallationStatusActive`.

### Relationship to per-project `WebhookEnabled`

`projects.project_webhook_enabled` is the per-project opt-in (a project may
decline webhook builds even when the installation is active). The global
installation check is orthogonal: installation inactive → webhook builds blocked
for ALL projects; project flag false → webhook builds blocked for THAT project.

## Related known gaps (from earlier review)

- Uninstall webhooks are not wired: `UpdateInstallationStatus(userID,
  "uninstalled")` exists in the repository but has no caller. A user uninstalling
  from GitHub's side leaves a stale `active` row, so the global check above would
  pass incorrectly. Wire the `installation` webhook event (`action: deleted`) to
  mark the installation `uninstalled` (or delete the row) before relying on the
  status gate.
- `StoreInstallation` is a plain `INSERT`, so reinstalling after an external
  uninstall can violate the unique `user_id` index. Consider an upsert
  (`ON CONFLICT (user_id) DO UPDATE`).
