# Worker build fixes — measured replay

Tested product commit: `20544b6c95b9cb1536250f2d8a7fef24560ceb04` on `fix-builds`; baseline `f01968cf3c453781fe74936ee51c2e000b327107`. See `execution-source.json` for binary and template hashes, and Git history for subsequent harness/test/report commits.

| Outcome | Baseline | Fixed worker replay |
|---|---:|---:|
| Image created | 3/57 (5.26%) | 39/57 (68.42%) |
| Local HTTP smoke passed | 2/57 (3.51%) | 37/57 (64.91%) |
| Production deployment/publication | Not attempted | Not attempted |

These are measured application-case outcomes, not independent repository probabilities. Several cases share upstream repositories. Image creation is determined by inspecting the final tagged image independently of database/finalization state. HTTP readiness is tested separately. The worker still reports dynamic success through its existing publication stub; this is not counted as deployment.

## Changes

- Retain frozen JS installs whenever a supported lockfile exists; recognize/copy npm shrinkwrap. Refuse missing npm locks early unless an operator sets `ALLOW_UNLOCKED_NPM_INSTALL=true`. Fallback keeps `--ignore-scripts`, emits a reproducibility warning, and does not write a checkout lockfile.
- Fully qualify template base references. Derive the Go image version from the greater released `go`/`toolchain` requirement; disable implicit Go toolchain switching. Select Node 20/22/24 image families from package engine ranges and explicit pins, with a Node 24 default and actual-image engine checks before installation. Other language stacks retain their previous version policy.
- Separate app Dockerfile location from Buildah context. Preserve the repository for Cargo workspaces/path dependencies, select one package and binary, and copy that exact binary. Support repository-configured Go package/context selection. Reject escaping paths, symlinks, and ambiguous native targets.

## Validation and method

- Worker-server suite: `GOCACHE=/tmp/zd-fix-builds-gocache go test -count=1 ./...` (see `worker-suite-with-native-tests.log`). The host skips live Buildah integration when Buildah is unavailable; separate container tests exercise it.
- Live toolchain/native fixtures: `TestDockerBuild_WithDockerfile` and `TestNativeBuildIntegration` in a fresh isolated store. See `native-build-integration.log` for Node engine validation in both install stages, Go explicit package/context execution, Rust standalone exact-binary execution, and Rust workspace/path-dependency execution.
- Fresh-image verification: 8/8 qualified image pulls passed from an initially empty separate store. The worker also began with empty storage; cold-pull images were not copied into it. See `cold-pulls.json` and individual logs.
- Replay: `python3 scripts/worker-build-campaign/rerun.py --baseline /home/parth_garg/zero-devops-campaign-50`. The worker shallow-fetches each recorded SHA through the unchanged immutable-clone path. All 57 baseline-qualified app/path/SHA identities match. Reference builds were not repeated; qualification is the preserved earlier reference evidence.
- First 17 cases ran sequentially; the user then authorized up to four concurrent cases / 6 CPUs / 8 GiB for builders. Docker exposed only 5.6 GiB total VM RAM, so the remainder ran on two isolated 1.5 CPU / 2 GiB / 512-PID workers, reserving 1.5 GiB for host/support services. Each has its own build store, network, AMQP vhost, and mock endpoint; only the run-owned test DB/broker are shared. The 15-minute build deadline remains. See parallel-state.json and lanes/lane2.json. No production credentials, deployment/publication, global prune, or unrelated-container cleanup.
- The user paused the campaign overnight. 2 incomplete Rust attempts were archived and excluded from measured failures, with their queues/DB rows reconciled before controlled retries. Completed case records were preserved byte-for-byte. The rebuilt worker matches the original binary SHA-256 exactly. Counts use the first completed, non-interrupted trial per case on the final implementation; completed failures were not replaced by later successes. See latest-resume.json and its reconciliation directory. Local cache snapshot images created during recovery do not enter the application denominator.
- The locally compiled worker binary and committed templates were mounted read-only into the retained test runtime image; the worker Dockerfile was not rebuilt. See VALIDATION.md, build-toolchain.txt, running-worker-sha256.txt and environment.json. Implementation commits and their scope are listed in COMMITS.md.
- Install fallback is enabled for this replay. Unlocked dependency resolution may differ from baseline references and may differ between build/runtime installation stages. Version tags and warmed worker layers are not immutable or uniformly cold.
- Worker end-to-end duration: median 89.65s, sum of case durations 8935.33s (not parallel wall time). Failures can stop early, so this is not a successful-build speed comparison. Per-case timings and sampled resource data are retained.
- HTTP probes run in the Buildah parent network namespace, matching the baseline harness. They establish local endpoint response only; they do not establish external container reachability, companion APIs, auth, persistent data, or production routing.

## Remaining outcomes

- build-and-runtime-pass: 37
- detection-unsupported: 4
- image-build-pass/runtime-fail: 2
- missing-code-generation: 1
- native-dependency/install-scripts: 1
- other-build-failure: 10
- static-output-path/misleading-ssr: 2

### Targeted baseline groups

| Original failure group | Cases | Baseline images | Replay images | Replay local HTTP |
|---|---:|---:|---:|---:|
| missing-lockfile/install-policy | 30 | 0 | 21 | 21 |
| base-image-resolution | 10 | 0 | 10 | 10 |
| workspace-context | 5 | 0 | 5 | 4 |

### Verified remaining blockers

| Blocker | Cases | Stage |
|---|---:|---|
| Svelte or Astro misclassified as SvelteKit; dist produced, build copied | 3 | build/prepare |
| Optional Next public directory copied unconditionally | 2 | build/prepare |
| Git fetch or checkout deadline; application build not reached | 4 | checkout |
| Missing manifest discovery | 4 | build/prepare |
| Restricted native dependency preparation | 1 | build/prepare |
| Prisma client generation missing | 1 | build/prepare |
| VitePress output selection | 1 | build/prepare |
| Laravel detected as Vite frontend | 1 | build/prepare |
| Python pyproject unsupported by selected dependency recipe | 1 | build/prepare |
| Development Nest CLI used for production startup | 1 | runtime |
| Rust runtime asset directories absent | 1 | runtime |

Case names and supporting records for each blocker are in `failure-diagnosis.json`; detailed newly exposed issues are in `NEW_FAILURES.md`. Git deadline failures do not establish application compilation/runtime incompatibility.

### Separate serial repeats of Git timeouts

These use the unchanged worker and per-builder limits after the primary replay. They are diagnostic controls, not replacements for primary failures; the headline remains the original 57 completed trials.

| Case | Repeat classification | Image | Local HTTP |
|---|---|---|---|
| dotnet-insights-web | detection-unsupported | no | not passed |
| next-basic-css | other-build-failure | no | not passed |
| next-hello-world | other-build-failure | no | not passed |
| next-with-typescript | other-build-failure | no | not passed |

See `controlled-timeout-repeats/` for full logs, job IDs and the primary-evidence preservation check. Different scheduling/cache/network conditions prevent attributing repeat differences solely to concurrency.

Missing-manifest discovery, Laravel framework selection, publication, and hardcoded loopback binding were outside these fixes. Rust loopback listeners may pass the local probe while remaining unreachable from an external container. Prisma generation, static output paths, native dependency preparation, and production startup commands can remain as later blockers. See `NEW_FAILURES.md` and individual build/runtime logs for the actual failure in each case.

## Exclusions and evidence

The four original reference-inconclusive candidates remain excluded: sveltekit-realworld, react-docs, django-realworld, rails-realworld. No qualified cases were dropped from the denominator. Original repository evidence hashes remain unchanged (`baseline-sha256.json`). Service setup repairs before application submission are documented in `harness-incidents.md`. The interim Node-20 pilot is preserved separately in `../worker-build-fixes-node20-pilot-2026-11-16/`; it exposed Astro’s declared Node engine requirement and was superseded by this complete fresh-checkout replay. Pilot outcomes are not mixed into these 57 completed trials on the final implementation.

Each result JSON records its source SHA, baseline classification, final image inspection, worker database state, status messages, phase events, sampled resources, and runtime response. `metrics.csv` provides the comparison without replacing the preserved baseline. `cleanup.json` records cleanup of run-owned resources only.

## Every qualified case

| App | Baseline | Replay | Image | HTTP smoke |
|---|---|---|---|---|
| vite-vanilla | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| vite-vanilla-ts | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| vite-react | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| vite-react-ts | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| vite-vue | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| vite-vue-ts | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| vite-svelte | missing-lockfile/install-policy | other-build-failure | no | not run |
| vite-svelte-ts | missing-lockfile/install-policy | other-build-failure | no | not run |
| vite-solid | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| vite-solid-ts | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| vite-preact | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| vite-preact-ts | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| vite-lit | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| vite-lit-ts | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| astro-minimal | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| astro-basics | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| astro-blog | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| astro-portfolio | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| astro-framework-react | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| astro-framework-vue | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| astro-framework-svelte | missing-lockfile/install-policy | other-build-failure | no | not run |
| astro-with-tailwindcss | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| next-hello-world | missing-lockfile/install-policy | other-build-failure | no | not run |
| next-blog-starter | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| next-with-typescript | missing-lockfile/install-policy | other-build-failure | no | not run |
| next-basic-css | missing-lockfile/install-policy | other-build-failure | no | not run |
| next-api-routes-rest | missing-lockfile/install-policy | other-build-failure | no | not run |
| next-with-absolute-imports | missing-lockfile/install-policy | other-build-failure | no | not run |
| react-jobs | build-and-runtime-pass | build-and-runtime-pass | yes | pass |
| vue-jobs | build-and-runtime-pass | build-and-runtime-pass | yes | pass |
| react-redux-realworld | missing-lockfile/install-policy | build-and-runtime-pass | yes | pass |
| node-prisma-realworld | missing-code-generation | missing-code-generation | no | not run |
| nestjs-starter | image-build-pass/runtime-fail | image-build-pass/runtime-fail | yes | fail |
| vue-docs | static-output-path/misleading-ssr | static-output-path/misleading-ssr | no | not run |
| gatsby-starter | native-dependency/install-scripts | native-dependency/install-scripts | no | not run |
| fiber-optional-parameter | base-image-resolution | build-and-runtime-pass | yes | pass |
| fiber-dynamic-routing | base-image-resolution | build-and-runtime-pass | yes | pass |
| fiber-form-data | base-image-resolution | build-and-runtime-pass | yes | pass |
| fiber-multiple-ports | base-image-resolution | build-and-runtime-pass | yes | pass |
| fiber-sse | base-image-resolution | build-and-runtime-pass | yes | pass |
| fiber-stream-request-body | base-image-resolution | build-and-runtime-pass | yes | pass |
| fiber-viper-config | base-image-resolution | build-and-runtime-pass | yes | pass |
| actix-hello-world | workspace-context | build-and-runtime-pass | yes | pass |
| actix-state | workspace-context | build-and-runtime-pass | yes | pass |
| actix-nested-routing | workspace-context | build-and-runtime-pass | yes | pass |
| actix-static-files | workspace-context | image-build-pass/runtime-fail | yes | fail |
| actix-error-handling | workspace-context | build-and-runtime-pass | yes | pass |
| gs-rest-service | base-image-resolution | build-and-runtime-pass | yes | pass |
| gs-serving-web-content | base-image-resolution | build-and-runtime-pass | yes | pass |
| flask-tutorial | other-build-failure | other-build-failure | no | not run |
| fastapi-first-steps | detection-unsupported | detection-unsupported | no | not run |
| fastapi-path-params | detection-unsupported | detection-unsupported | no | not run |
| fastapi-query-params | detection-unsupported | detection-unsupported | no | not run |
| slim-skeleton | base-image-resolution | build-and-runtime-pass | yes | pass |
| laravel-skeleton | missing-lockfile/install-policy | static-output-path/misleading-ssr | no | not run |
| dotnet-insights-web | detection-unsupported | other-build-failure | no | not run |
| sinatra-simple | detection-unsupported | detection-unsupported | no | not run |
