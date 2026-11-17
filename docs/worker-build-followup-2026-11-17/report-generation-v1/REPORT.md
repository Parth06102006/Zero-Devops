# Follow-up build and runtime campaign

Tested product commit: `d15eae067cf77a8dcb5d6d6aaf8a59b166c65106`; binary SHA256 `67c5461cea822b90163359972090fc6e2e27cbf1b1e08fd6777ecaf023fc556f`.

| Cohort / trial | Cases | Images created | Local HTTP passed | External-container HTTP passed / attempted |
|---|---:|---:|---:|---:|
| Previous failures, prior source 20544b6 | 20 | 2 | 0 | 0 / 0 |
| Previous failures, follow-up source | 20 | 12 | 12 | 11 / 12 |
| Fresh cases, worker | 12 | 12 | 12 | 10 / 12 |
| Fresh cases, independent reference | 12 | 12 | 12 | 10 / 12 |
| Fresh reference-qualified cases, worker | 12 | 12 | 12 | 10 / 12 |

The twenty-case cohort contains every previous image/runtime failure at the same app/path/SHA. Its prior result was two images and zero local HTTP passes. Fresh cases are a separate compatibility measurement. The other 37 previously successful cases were not rerun on this source: no new overall 57-case rate is established. Image creation is inspected independently of worker success status. A local HTTP response, a request from a separate container, and a deployed application are distinct outcomes. Deployment and publication were not attempted.

## Changes and scope

- Next.js normalizes optional public/ inside the builder and validates .next before runtime packaging.
- Framework ownership is resolved before supporting tools: Astro/Svelte integrations, SvelteKit adapters, VitePress output, and Laravel frontend/backend packaging. Ambiguous or unsupported choices retain actionable errors. Explicit framework and static output overrides are supported.
- Nest production startup uses the validated compiled entry; explicit runtime command, assets, and working directory are supported. Rust conventional static/ assets are carried into the runtime image. Asset paths and symlinks are checked inside the selected app/repository context; packaged assets receive the final runtime user ownership while retaining restrictive source permissions.
- Git fetch and checkout have separate bounded budgets; transient fetch failures get one retry, permanent failures do not. Fetch and checkout remain pinned to the accepted SHA, and HEAD is verified before building. Phase durations are recorded in each worker result.
- Point 4 remains excluded: no Prisma/code-generation step, new Python dependency format, native install-script relaxation, or missing-manifest discovery expansion. Application loopback bindings are unchanged. Laravel retains the existing PHP built-in server policy for local compatibility; no production hosting stack is certified.

## Validation and execution

- Full worker-server suite passed before freeze: [worker-suite-asset-ownership.log](worker-suite-asset-ownership.log). Final acceptance suite also passed: [worker-suite-final-acceptance.log](worker-suite-final-acceptance.log). Host tests conditionally skip live Buildah checks; the separate container validation executes them.
- Report calculation regression tests passed: [report-regression-tests.log](report-regression-tests.log); they check category partitioning, historical-worker durations, and separate image/local/external/status measures.
- Live native integration passed (exit 0): [native-integration.log](native-integration.log). It includes real image execution and Rust workspace asset packaging.
- Fresh-store pulls: 10/10. See [cold-pulls.json](cold-pulls.json); unavailable images remain failures in the relevant reference/worker trials.
- Two isolated worker lanes each have 1.5 CPUs, 2 GiB RAM, 512 PIDs, a separate build store/network/broker vhost/mock endpoint. Only the run-owned DB and broker are shared. The VM exposes about 5.6 GiB RAM, so two builders fit within the approved maximum; four were not launched.
- Failure and fresh trials alternate in predeclared order while both cohorts have pending work. Every fresh case runs its independently declared reference compilation/start command before the worker trial. A failed reference does not suppress the worker trial or cause case replacement.
- Source, template, binary, manifests and prior-evidence fingerprints were frozen before dispatch and rechecked afterward. Fresh-source inventory metadata and dependency-requirements JSON were not in the original controller freeze; a separate final audit verifies exact file inventories, byte hashes, executable/symlink modes, pinned upstream tree/blob linkage and dependency metadata. That is an additional acceptance check, not retroactive freeze evidence. See [supplemental-evidence-audit.json](supplemental-evidence-audit.json), [frozen-campaign.json](frozen-campaign.json), [environment.json](environment.json), [parallel-state.json](parallel-state.json), and [preservation-check.json](preservation-check.json).
- The Go worker binary/test executable and templates are mounted into the retained test runtime image. This does not validate a rebuild of the worker Dockerfile. Reference-toolchain/dependency logs show actual versions in the reference images; worker-dependencies logs inspect final Node runtime images. Static worker images retain the static server rather than the app builder dependency tree, so an empty/error npm listing there does not establish builder versions. Reference resolutions must not be presented as exact worker-builder resolutions. Declared source requirements are in fresh-dependency-requirements.json.

- Host build toolchain: `go version go1.26.8 linux/amd64`. Exact compile commands and worker/test binary hashes are recorded in [build-provenance.json](build-provenance.json).

Commands used by the harness:

```text
GOCACHE=/tmp/zd-fix-builds-gocache go test -count=1 ./...
CGO_ENABLED=0 GOCACHE=/tmp/zd-fix-builds-gocache go test -c -o /tmp/zd-round2-deployment-tests ./internal/deployments
CGO_ENABLED=0 GOCACHE=/tmp/zd-fix-builds-gocache go build -buildvcs=false -trimpath -o /tmp/zd-round2-worker ./cmd/worker-server
python3 scripts/worker-build-campaign/round2_environment.py setup
python3 scripts/worker-build-campaign/round2_validate.py
python3 scripts/worker-build-campaign/round2_reverify.py --product-commit d15eae067cf77a8dcb5d6d6aaf8a59b166c65106
docker exec -e RUN_NATIVE_BUILD_TESTS=true -w /app zdround2-2-worker timeout --kill-after=10s 1200s /e2e/deployment-tests -test.v -test.run 'TestDockerBuild_WithDockerfile|TestNativeBuildIntegration'
python3 scripts/worker-build-campaign/round2_parallel.py --freeze
python3 scripts/worker-build-campaign/round2_parallel.py
python3 scripts/worker-build-campaign/round2_report.py
```

## Controller interruption and retained-image recovery

The first controller stopped after a 60-second storage-size telemetry timeout and a 20-second database poll timeout. Both affected Next applications had already created final images; the original workers subsequently stopped. Their initial incomplete records, controller state and full worker logs are retained under recovery-20261117/. The retained tagged images were inspected and HTTP-tested later in helpers with exclusive access to their original stores/networks; neither image was rebuilt and no completed outcome was replaced. The TypeScript job retains its original canceled database state after a broker channel closed, even though its image and HTTP probes passed. These are measured build/runtime outcomes, not successful finalization or deployment. The remaining five unsubmitted cases ran serially on lane 1; the recovered lane-2 queue item was left untouched with its worker stopped to avoid duplicate dispatch.

Recovered trials lack the failed host telemetry samples. Their durations use original dispatch-to-terminal-log timestamps, while runtime probes occurred later. This scheduling/infrastructure interruption limits timing comparisons further. See recovery-20261117/measurement-recovery.json and each record’s measurement_recovery field.

## Failure-cohort comparison

| Case | Prior image / local HTTP | Follow-up image / local HTTP | External HTTP | Worker outcome |
|---|---|---|---|---|
| actix-static-files | pass / fail | pass / pass | fail | build-and-runtime-pass |
| astro-framework-svelte | fail / not attempted | pass / pass | pass | build-and-runtime-pass |
| dotnet-insights-web | fail / not attempted | fail / not attempted | not attempted | detection-unsupported |
| fastapi-first-steps | fail / not attempted | fail / not attempted | not attempted | detection-unsupported |
| fastapi-path-params | fail / not attempted | fail / not attempted | not attempted | detection-unsupported |
| fastapi-query-params | fail / not attempted | fail / not attempted | not attempted | detection-unsupported |
| flask-tutorial | fail / not attempted | fail / not attempted | not attempted | other-build-failure |
| gatsby-starter | fail / not attempted | fail / not attempted | not attempted | native-dependency/install-scripts |
| laravel-skeleton | fail / not attempted | pass / pass | pass | build-and-runtime-pass |
| nestjs-starter | pass / fail | pass / pass | pass | build-and-runtime-pass |
| next-api-routes-rest | fail / not attempted | pass / pass | pass | build-and-runtime-pass |
| next-basic-css | fail / not attempted | pass / pass | pass | build-and-runtime-pass |
| next-hello-world | fail / not attempted | pass / pass | pass | build-and-runtime-pass |
| next-with-absolute-imports | fail / not attempted | pass / pass | pass | build-and-runtime-pass |
| next-with-typescript | fail / not attempted | pass / pass | pass | build-and-runtime-pass |
| node-prisma-realworld | fail / not attempted | fail / not attempted | not attempted | missing-code-generation |
| sinatra-simple | fail / not attempted | fail / not attempted | not attempted | detection-unsupported |
| vite-svelte | fail / not attempted | pass / pass | pass | build-and-runtime-pass |
| vite-svelte-ts | fail / not attempted | pass / pass | pass | build-and-runtime-pass |
| vue-docs | fail / not attempted | pass / pass | pass | build-and-runtime-pass |

## Results by your requested categories

### Original primary blocker categories

| Category | Cases | Prior images / local HTTP | Follow-up images / local HTTP | External HTTP passes / attempted |
|---|---:|---:|---:|---:|
| 1. Optional Next.js public/ handling | 2 | 0 / 0 | 2 / 2 | 2 / 2 |
| 2. Framework detection and output selection | 5 | 0 / 0 | 5 / 5 | 5 / 5 |
| 3. Production startup and runtime assets | 2 | 2 / 0 | 2 / 2 | 1 / 2 |
| 4. Generation, dependency formats and discovery — excluded | 7 | 0 / 0 | 0 / 0 | 0 / 0 |
| 5. Checkout reliability | 4 | 0 / 0 | 3 / 3 | 3 / 3 |

These rows partition the twenty cases by their original primary blocker. Three original checkout failures later reached the optional Next public/ error in controlled repeats; .NET later reached excluded discovery. Current terminal diagnoses below remain separate from original blocker attribution.

### Expanded targeted scope and cross-cutting checkout checks

| Category | Scope | Cases | Prior images / local HTTP | Follow-up images / local HTTP | External HTTP passes / attempted |
|---|---|---:|---:|---:|---:|
| 1. Optional Next.js public/ handling | implemented | 5 | 0 / 0 | 5 / 5 | 5 / 5 |
| 2. Framework detection and output selection | implemented | 5 | 0 / 0 | 5 / 5 | 5 / 5 |
| 3. Production startup and runtime assets | implemented | 2 | 2 / 0 | 2 / 2 | 1 / 2 |
| 4. Generation, dependency formats and discovery — excluded | excluded | 8 | 0 / 0 | 0 / 0 | 0 / 0 |
| 5. Checkout reliability — overlaps categories above | implemented | 4 | 0 / 0 | 3 / 3 | 3 / 3 |

Categories 1–4 partition all twenty historical failures. Category 5 is cross-cutting: its four cases also appear in the other rows and must not be added again. The Next category includes the three controlled timeout repeats that previously reached the same optional-public packaging error; this is not a claim that all five primary trials originally reached that error.

- **1. Optional Next.js public/ handling:** `next-api-routes-rest`, `next-basic-css`, `next-hello-world`, `next-with-absolute-imports`, `next-with-typescript`.
- **2. Framework detection and output selection:** `astro-framework-svelte`, `laravel-skeleton`, `vite-svelte`, `vite-svelte-ts`, `vue-docs`.
- **3. Production startup and runtime assets:** `actix-static-files`, `nestjs-starter`.
- **4. Generation, dependency formats and discovery — excluded:** `dotnet-insights-web`, `fastapi-first-steps`, `fastapi-path-params`, `fastapi-query-params`, `flask-tutorial`, `gatsby-starter`, `node-prisma-realworld`, `sinatra-simple`.
- **5. Checkout reliability — overlaps categories above:** `dotnet-insights-web`, `next-basic-css`, `next-hello-world`, `next-with-typescript`.

Checkout verification reached the exact pinned HEAD for 4/4 original Git-timeout cases. Second fetch attempts were observed in 0 of these cases. Successful checkout is not image/runtime success and the later environment does not isolate the causal effect of the policy change.


## Fresh compatibility cohort

| Case | Stack | Reference image / local HTTP | Qualified | Worker image / local HTTP | Worker external HTTP |
|---|---|---|---|---|---|
| fresh-astro-framework-preact | Astro/framework-preact | pass / pass | yes | pass / pass | pass |
| fresh-astro-framework-solid | Astro/framework-solid | pass / pass | yes | pass / pass | pass |
| fresh-astro-with-mdx | Astro/with-mdx | pass / pass | yes | pass / pass | pass |
| fresh-astro-with-markdoc | Astro/with-markdoc | pass / pass | yes | pass / pass | pass |
| fresh-fiber-hello-world | Fiber v3/hello-world | pass / pass | yes | pass / pass | pass |
| fresh-fiber-404-handler | Fiber v3/404-handler | pass / pass | yes | pass / pass | pass |
| fresh-fiber-advanced-caching | Fiber v3/advanced-caching | pass / pass | yes | pass / pass | pass |
| fresh-axum-hello-world | Axum/hello-world | pass / pass | yes | pass / pass | fail |
| fresh-axum-cors | Axum/cors | pass / pass | yes | pass / pass | fail |
| fresh-gs-rest-service-cors | Spring/gs-rest-service-cors | pass / pass | yes | pass / pass | pass |
| fresh-gs-handling-form-submission | Spring/gs-handling-form-submission | pass / pass | yes | pass / pass | pass |
| fresh-next-with-sass | Next/Sass | pass / pass | yes | pass / pass | pass |

Reference-inconclusive cases are excluded only from the fresh reference-qualified subset; all twelve remain visible above and in the all-case totals. Full immutable upstream SHAs, commands, probes and source paths are in the two manifests. No app source was changed to force a pass.

### Dependency and toolchain rotation

These are declared project requirements, not a claim that every range resolved to the same version in reference and worker images.

| New case | Declared toolchain / dependency variation | Observed reference tools |
|---|---|---|
| fresh-astro-framework-preact | Node >=22.23.4; @astrojs/preact ^6.1.6; @preact/signals ^2.11.4; astro ^7.4.12; preact ^11.2.0 | v24.22.3; 11.19.1 [log](fresh/results/fresh-astro-framework-preact.reference-toolchain-dependencies.log) |
| fresh-astro-framework-solid | Node >=22.23.4; @astrojs/solid-js ^7.1.6; astro ^7.4.12; solid-js ^1.9.19 | v24.22.3; 11.19.1 [log](fresh/results/fresh-astro-framework-solid.reference-toolchain-dependencies.log) |
| fresh-astro-with-mdx | Node >=22.23.4; @astrojs/mdx ^8.1.6; @astrojs/preact ^6.1.6; astro ^7.4.12; preact ^11.2.0 | v24.22.3; 11.19.1 [log](fresh/results/fresh-astro-with-mdx.reference-toolchain-dependencies.log) |
| fresh-astro-with-markdoc | Node >=22.23.4; @astrojs/markdoc ^2.1.5; astro ^7.4.12 | v24.22.3; 11.19.1 [log](fresh/results/fresh-astro-with-markdoc.reference-toolchain-dependencies.log) |
| fresh-fiber-hello-world | Go 1.27.0; github.com/gofiber/fiber/v3 v3.7.0 | go version go1.27.0 linux/amd64 [log](fresh/results/fresh-fiber-hello-world.reference-toolchain-dependencies.log) |
| fresh-fiber-404-handler | Go 1.27.0; github.com/gofiber/fiber/v3 v3.7.0 | go version go1.27.0 linux/amd64 [log](fresh/results/fresh-fiber-404-handler.reference-toolchain-dependencies.log) |
| fresh-fiber-advanced-caching | Go 1.27.0; github.com/gofiber/fiber/v3 v3.7.0 | go version go1.27.0 linux/amd64 [log](fresh/results/fresh-fiber-advanced-caching.reference-toolchain-dependencies.log) |
| fresh-axum-hello-world | Rust workspace; axum local path ../../axum; tokio 1.0 | rustc 1.100.0 (0ef1bc386 2026-11-09); cargo 1.100.0 (3acc79c0c 2026-11-05) [log](fresh/results/fresh-axum-hello-world.reference-toolchain-dependencies.log) |
| fresh-axum-cors | Rust workspace; axum local path ../../axum; tokio 1.0; tower-http 0.6.1 | rustc 1.100.0 (0ef1bc386 2026-11-09); cargo 1.100.0 (3acc79c0c 2026-11-05) [log](fresh/results/fresh-axum-cors.reference-toolchain-dependencies.log) |
| fresh-gs-rest-service-cors | Java source 17; Spring Boot 4.0.9; spring-boot-starter-web | openjdk version "21.0.13" 2026-10-20 LTS; Apache Maven 3.10.0 (c43a36b8d67be7e0805a411bc0898af1a51f5472); Java version: 21.0.13, vendor: Eclipse Adoptium, runtime: /opt/java/openjdk [log](fresh/results/fresh-gs-rest-service-cors.reference-toolchain-dependencies.log) |
| fresh-gs-handling-form-submission | Java source 17; Spring Boot 4.1.2; spring-boot-starter-thymeleaf, spring-boot-starter-webmvc | openjdk version "21.0.13" 2026-10-20 LTS; Apache Maven 3.10.0 (c43a36b8d67be7e0805a411bc0898af1a51f5472); Java version: 21.0.13, vendor: Eclipse Adoptium, runtime: /opt/java/openjdk [log](fresh/results/fresh-gs-handling-form-submission.reference-toolchain-dependencies.log) |
| fresh-next-with-sass | Node (no engine declared); next latest; react ^19.0.0; react-dom ^19.0.0; sass ^1.82.0; typescript ^5.7.2 | v24.22.3; 11.19.1 [log](fresh/results/fresh-next-with-sass.reference-toolchain-dependencies.log) |

See fresh-dependency-requirements.json for the full manifests/requirements, and each *.reference-toolchain-dependencies.log for observed reference tools and installed Node packages. The worker builder dependency tree is not reconstructed from static final images.

## Remaining failures and external reachability gaps

| Batch / case | Trial | Stage | Recorded diagnostic | Evidence |
|---|---|---|---|---|
| failures / actix-static-files | worker | external HTTP | Local endpoint passed; the separate container request failed. | [log](failures/results/actix-static-files.json) |
| failures / dotnet-insights-web | worker | detection / preparation | clone/prepare failed: no framework detected | [log](failures/results/dotnet-insights-web.worker.log) |
| failures / fastapi-first-steps | worker | detection / preparation | clone/prepare failed: no framework detected | [log](failures/results/fastapi-first-steps.worker.log) |
| failures / fastapi-path-params | worker | detection / preparation | clone/prepare failed: no framework detected | [log](failures/results/fastapi-path-params.worker.log) |
| failures / fastapi-query-params | worker | detection / preparation | clone/prepare failed: no framework detected | [log](failures/results/fastapi-query-params.worker.log) |
| failures / flask-tutorial | worker | image build | Error: building at STEP "COPY requirements.txt ./": checking on sources under "/tmp/zerodevops-build/3ed134e4-174c-4a1a-afa1-06fba0b03c72/examples/tutorial": copier: stat: "/requirements.txt": no such file or directory | [log](failures/results/flask-tutorial.worker.log) |
| failures / gatsby-starter | worker | image build | Cannot find module '../build/Release/sharp-linuxmusl-x64.node' | [log](failures/results/gatsby-starter.worker.log) |
| failures / node-prisma-realworld | worker | image build | Failed tasks: | [log](failures/results/node-prisma-realworld.worker.log) |
| failures / sinatra-simple | worker | detection / preparation | clone/prepare failed: no framework detected | [log](failures/results/sinatra-simple.worker.log) |
| fresh / fresh-axum-hello-world | worker | external HTTP | Local endpoint passed; the separate container request failed. | [log](fresh/results/fresh-axum-hello-world.json) |
| fresh / fresh-axum-hello-world | reference | external HTTP | Local endpoint passed; the separate container request failed. | [log](fresh/results/fresh-axum-hello-world.json) |
| fresh / fresh-axum-cors | worker | external HTTP | Local endpoint passed; the separate container request failed. | [log](fresh/results/fresh-axum-cors.json) |
| fresh / fresh-axum-cors | reference | external HTTP | Local endpoint passed; the separate container request failed. | [log](fresh/results/fresh-axum-cors.json) |

## Pre-campaign verification correction

Before application jobs were dispatched, the first live Rust workspace asset fixture built its image but failed at runtime with PermissionDenied reading a 0600 asset through 0750 directories. The image had copied root ownership before switching to the app user. The worker was corrected to preserve asset modes and assign the final runtime user ownership, then rebuilt, tested and frozen again before the application cohorts. This failed verification is retained separately and is not counted as an application-case failure.
- Prior source `942664f3ccd5b8b6a4c3ff347d1c27552eb741fb`: native validation exit 1; [retained log](pre-campaign-verification-942664f/native-integration.log), with its original freeze/environment/provenance records in the same directory.
- The ten cold pulls were retained because this correction did not change the selected base-image set. Application results and final live validation use the replacement frozen product shown above.


Diagnostics are excerpts from the terminal trial logs, not promises that removing one blocker will make the application pass. Full raw logs and structured worker/reference records remain authoritative.

## Interpretation and limitations

- Several cases share upstream repositories, framework implementations and caches; these are correlated case outcomes, not independent platform success probabilities. The fresh batch adds different app paths, integrations and manifest/toolchain requirements, rather than randomly sampling all applications.
- Node installs without committed locks are explicitly opted in and keep install scripts disabled. Resolution can differ across dates, between reference and worker trials, and between builder/runtime installs. Image tags also move. Source SHAs are pinned; all dependency trees and base digests are not globally frozen.
- Checkout recovery occurs under later network/cache/scheduling conditions. Before/after outcomes do not isolate the causal effect of the timeout/retry policy; per-phase fetch/checkout durations and attempts are retained for inspection.
- Cold pulls prove image availability at validation time from initially empty lane-2 storage. Lane 2 was then warmed by pulls/live fixtures, and fresh reference trials precede worker trials in each lane/store. Base-image reuse is supported; exact intermediate-layer/dependency-cache reuse is not established merely by sharing storage. Timings are not uniformly cold or a controlled speed benchmark. Summed case durations are not parallel wall time.
- Worker end-to-end timings include checkout, compilation, image archiving and finalization, and stop before the separately recorded smoke probe. Reference durations include their build/probe/cleanup sequence. These duration fields are not directly comparable. CPU counters and memory.peak can be cumulative over a worker lifetime; their samples are retained as telemetry, not presented as isolated per-app CPU cost or memory peaks. The configured per-worker resource ceilings are the authoritative limits.
- HTTP qualification uses one accepted local status; eight warm requests are recorded separately and reviewed for instability without redefining the frozen readiness outcome. External probes are conditional on local readiness and check one 2xx/3xx response; absent probes are not attempted, not failures, and external response bodies are not preserved or validated. Successful runtime logs may end with killed/exit-status messages from intentional probe shutdown, not a startup failure.
- References use a 1,200-second build budget and builder-style images retaining source/tools; worker build/save uses a 15-minute budget and its selected runtime packaging. These are not equivalent production configurations.
- HTTP probes check only the declared endpoint/status. They do not exercise full application features, databases, auth flows, companion frontends, browser CORS, persistent state, production ingress, TLS, or publication. The separate-container probe reaches the Buildah worker network address; it is stronger reachability evidence than the parent-namespace local probe, but still is not a deployed service.
- The campaign exercises Linux containers on the current Docker amd64 environment. It does not establish Windows, macOS-native, ARM, other CPU architectures, or every hosting-platform compatibility.
- Existing loopback-only application bindings may pass local HTTP and fail the separate-container probe. This distinction is preserved rather than changing application source.
- Detection/configuration support remains bounded: inherited or dynamically computed compiler/adapter settings and custom runtime output layouts are not universally resolved. Supported explicit framework/output/runtime overrides cover the documented forms; they do not establish compatibility with every layout.
- Previous stopped workers/build stores and all prior evidence were retained. Cleanup, if performed after this report, is scoped to follow-up-owned resources and is recorded separately in cleanup.json. No global prune or unrelated-resource cleanup is authorized.

The prior campaign evidence/harness archive was committed separately as `4c07e71`. Evidence-only commits do not change the frozen tested product identity `d15eae067cf77a8dcb5d6d6aaf8a59b166c65106`; later harness/report commits are likewise not application source retests.
