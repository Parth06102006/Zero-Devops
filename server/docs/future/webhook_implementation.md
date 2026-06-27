# Webhook Implementation Review Plan

This note compares the current SCM/GitHub architecture with the intended Vercel-style GitHub integration flow and turns that into a concrete webhook plan for this app.

The main idea is:

- keep GitHub App installation separate from repository listing
- keep repository selection separate from deployment
- use webhooks to keep GitHub installation state and repo access in sync

---

## 1. Current Architecture Reference

Current relevant files:

- `server/integrations/scm/delivery/http/scm_handler.go`
- `server/integrations/scm/github/usecase/github_ucase.go`
- `server/domain/github.go`
- `server/docs/future/issues.md`
- `server/docs/future/github_integration_future.md`

Current shape:

- `SCMHandler` exposes install, get, and delete routes for GitHub App integration.
- `githubAppUsecase` exchanges the OAuth code, fetches GitHub installations, and stores the installation in PostgreSQL.
- `domain.GithubInstallation` currently stores user linkage and `installation_id`.
- `issues.md` already says the next step is repository access and deployment flow.
- `github_integration_future.md` already notes that webhook support will come later for installation lifecycle status.

---

## 2. What Vercel-Style Flow Usually Means

The likely flow is:

1. User connects GitHub to the platform.
2. User installs the GitHub App.
3. Backend stores the installation reference.
4. Dashboard loads repository choices for the installed app.
5. User selects a repository.
6. Backend stores the chosen repository.
7. User deploys the selected repository.
8. Webhooks keep install and repo state updated after the initial selection.

This means the repo list is not the same thing as the installation record.
The installation is the access gate.
The repo list is a runtime read from GitHub.
The webhook is the sync layer.

---

## 3. Comparison With The Current App

### What already matches

- GitHub App installation is already separate from GitHub login.
- The backend already stores installation data in the database.
- `user_id` is already being read from middleware context.
- The codebase already has a future-doc path for installation lifecycle work.

### What is still missing

- Repository listing endpoint.
- Repository selection storage.
- Installation webhook receiver.
- Webhook signature verification.
- Installation state sync for suspend and uninstall events.
- Repo sync after installation changes.
- Deployment trigger flow after repository selection.

---

## 4. Things We Need To Do

### A. Add a webhook endpoint

- Create a dedicated GitHub webhook handler.
- Accept installation lifecycle events from GitHub.
- Keep it separate from the existing install/get/delete handler.

### B. Verify webhook authenticity

- Validate GitHub webhook signatures.
- Reject unsigned or invalid requests.
- Use a shared webhook secret from config.

### C. Handle installation lifecycle events

- installation created
- installation deleted
- installation suspended
- installation unsuspended

### D. Keep database state in sync

- store installation status if needed
- mark uninstalled installations as inactive or removed
- keep user-installation mapping consistent

### E. Support repository listing

- use the stored `installation_id`
- create an installation access token
- call GitHub to list repositories available to that installation

### F. Support repository selection

- store the selected repository for a user or project
- verify the selected repo belongs to the installation
- prepare the deployment flow from that stored selection

### G. Prepare deployment hooks

- subscribe to push and installation-related events later if needed
- trigger build or redeploy from repository changes
- keep the deployment system aware of repo sync changes

---

## 5. Reason And Alternative

### A. Webhook endpoint

Reason:

- GitHub should notify us automatically when installation state changes.
- polling is wasteful and easy to miss.

Alternative:

- periodically poll GitHub for installation state.

Why the alternative is weaker:

- slower
- more API usage
- less reliable for real-time install updates

### B. Signature verification

Reason:

- webhooks are external input and must be trusted only after verification.
- this prevents spoofed installation events.

Alternative:

- trust the request payload without verification.

Why the alternative is weaker:

- unsafe
- makes the webhook endpoint vulnerable to fake events

### C. Separate installation and repository flows

Reason:

- installation is access setup.
- repository listing is a user choice step.
- deployment depends on the chosen repository, not only on installation.

Alternative:

- put repo listing inside the installation handler.

Why the alternative is weaker:

- mixes responsibilities
- makes the install flow harder to reason about
- will become difficult once deployment and repo refresh logic grow

### D. Store installation state in DB

Reason:

- the app needs a local source of truth for user linkage.
- status helps determine whether actions should be allowed.

Alternative:

- infer everything from live GitHub calls every time.

Why the alternative is weaker:

- slower
- harder to debug
- not stable if GitHub is temporarily unavailable

### E. Re-fetch repositories from GitHub on demand

Reason:

- repository access can change after installation.
- the repo picker should reflect current GitHub permissions.

Alternative:

- store a permanent cached repo list and never refresh it.

Why the alternative is weaker:

- stale results
- users may not see newly granted access
- revoked access may still appear selectable

---

## 6. Review Of The List

After reviewing the architecture and the future notes, the cleanest approach is:

- keep the current installation flow as-is for now
- add a separate repository listing flow next
- add a separate webhook flow for installation lifecycle sync
- use the webhook to maintain status, not to replace repo listing

The most important boundary is:

- **installation = access**
- **repository list = selection**
- **webhook = state synchronization**

This boundary keeps the code easier to extend and makes the deployment flow easier to build later.

---

## 7. Final Conclusion

The webhook implementation should be added as a separate future feature, not mixed into the current install/get/delete handler.

Final recommended direction:

1. Keep GitHub App installation in the current SCM GitHub area.
2. Add a dedicated webhook handler for GitHub installation events.
3. Add webhook signature verification before processing any event.
4. Store or update installation status locally when GitHub sends lifecycle events.
5. Add a repository listing usecase that reads the stored installation and queries GitHub on demand.
6. Add repository selection storage before deployment work begins.
7. Keep deployment logic dependent on the selected repository and valid installation state.

---

## 8. Suggested Implementation Order

1. Add webhook route and handler.
2. Add signature verification.
3. Add installation created / deleted / suspended / unsuspended handling.
4. Update database model for installation status if needed.
5. Add repository list API.
6. Add repository select API.
7. Add deployment flow.

---

## 9. Suggested Folder Split

If webhook support is added cleanly, the structure can evolve into:

```text
server/
  integrations/
    scm/
      delivery/
        http/
          scm_handler.go
          github_webhook_handler.go

      github/
        usecase/
          github_ucase.go
          github_repo_ucase.go
          github_webhook_ucase.go

        repository/
          pgsql/
            pgsql_github.go

      webhook/
        github/
          handler.go
          signature.go
          events.go
```

This keeps the webhook path separate while still staying inside the SCM integration area.

