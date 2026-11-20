# Worker build campaign final report

## Campaign sequence

The initial campaign below evaluated 61 candidate applications, of which 57 passed reference qualification and entered worker measurement. Its findings drove the worker-server and template fixes. The broad compatibility campaign followed those changes. The latest measured results are the follow-up campaign on the completed fix-builds worker revision, summarized at the end of this report. The active worker-server branch is fix-builds; its completed campaign commit is 1212c81.

The 50-case results are retained here as historical context. The broad campaign results are the latest measured outcomes. Counts and coverage from separate campaigns are not combined.

## Historical baseline — 57 measured applications from 61 candidates

# Public worker build campaign — 61 applications

**57 apps first passed a reference build and runtime smoke check, then ran through the unchanged worker.** These came from 20 public repositories. Four other reference candidates were excluded. No `Parth06102006` repo was used. This is 61 application cases, **not 61 independent repositories**.

Tested worker source: main `f01968cf3c453781fe74936ee51c2e000b327107`. Real worker/Buildah, isolated RabbitMQ and PostgreSQL, fake public-repo credentials and fake S3 endpoint. No product fixes, real R2, production deploys, or cloud-platform comparisons in this campaign.

## Rates and limitations

- Image creation: **3/57 = 5.26%**, nominal Wilson 95% interval [1.81, 14.37].
- Runnable-image HTTP smoke: **2/57 = 3.51%**, nominal Wilson 95% interval [0.97, 11.92].
- Image creation failure: **54/57 = 94.74%**; runnable smoke failure 55/57.
- These are observed case rates, not universal per-repo probabilities. Shared starter repos and intentional edge cases heavily influence them. One trial cannot distinguish every intermittent failure; selected repeats below provide limited evidence.
- A successful jobs frontend homepage does not prove its companion API works. Nest image success did not mean runnable. No production website/runner deployment was validated; fake upload is not publication to real R2.

## Size, time and latency

- Reference output range: **0.000–47.539 MiB**; 57 measured outputs.
- Reference build seconds: median **47.32**, p95 **141.07**, range 4.33–442.55.
- Worker end-to-end seconds: median **8.78**, p95 **88.07**, range 4.04–231.68.
- Failed builds stop early: a fast failure is not a faster successful build. Base pulls and progressively warmed caches affect times. Reference Rust builds share a campaign-only target/registry cache and are marked in JSON.
- CSV records clone/directory-resolution, build/failure, save, upload/finalize phases where logs allow; eight serial warm HTTP requests give local smoke p50/p95, not load/production network latency. Missing metrics remain blank, never fabricated.
- Source size, output size and image layer/tar size are different measurements. Language-only references may measure application tree when no separate artifact is emitted. Vue docs size was backfilled by an identical extra reference build and marked.

| Output size bucket | Reference-qualified apps | Primary image builds | Runtime passes |
|---|---:|---:|---:|
| 100 KiB–1 MiB | 10 | 2 | 2 |
| 1–10 MiB | 9 | 0 | 0 |
| <100 KiB | 21 | 1 | 0 |
| >=10 MiB | 17 | 0 | 0 |

### Primary image measurements

| App | Image layers MiB | Uploaded tar MiB | Build s | Startup s | Local HTTP p50/p95 ms |
|---|---:|---:|---:|---:|---|
| react-jobs | 143.265 | 143.291 | 74.201 | 2.682 | 3.024 / 4.239 |
| vue-jobs | 143.773 | 143.799 | 29.265 | 1.355 | 3.802 / 4.152 |
| nestjs-starter | 163.750 | 0.000 | 56.899 | 2.467 | — / — |

Sampled worker memory max across primary cases: **1299.456 MiB**. Twenty-second sampling can miss true peaks; per-case cgroup telemetry is also retained. There were **76 worker jobs** including repeat and controlled trials.

## Actual root causes

- **30 install-policy failures:** lockfile-less public examples cannot satisfy template `npm ci`. This is reproducible incompatibility, not proof the app is broken. Laravel is included in this count but additionally has a genuine **wrong primary framework**: Vite wins over PHP/Laravel, omitting the server entirely.
- **10 base lookup failures:** Go, Java, PHP template short names fail without a TTY. Controlled pre-pull allows Spring and Slim to build/run; Go then reveals Go1.24 versus required1.26.
- **5 Cargo workspace failures:** references build selected members with the root present; worker copies member only. Examples also bind loopback, so external container access requires application configuration changes.
- **5 missing-manifest detections:** bare FastAPI samples, Sinatra sample, and csproj-only .NET app run in explicitly configured reference environments but worker does not support that discovery/setup. Bare script dependency inference is intentionally not assumed safe.
- **1 Prisma generation failure:** documented generation plus isolated DB/migrations gives a working reference; worker omits generation.
- **1 static-output mistake:** Vue docs builds static .vitepress/dist; worker expects dist and calls it SSR.
- **1 native preparation failure:** Gatsby reference succeeds on compatible Debian/glibc; worker --ignore-scripts leaves Sharp absent on Alpine.
- **1 Python manifest recipe failure:** Flask pyproject is detected, but template wrongly copies requirements.txt.
- **1 runtime failure after image build:** Nest template uses npm start -> nest CLI, removed with dev dependencies. Use compiled production command.

## Repeated versus hidden failures

Eight representative apps each received two additional unchanged worker attempts (16 repeat trials). All matched the original outcome category: Vite lockfile, React Jobs pass, Prisma generation, Vue docs output path, Actix workspace, Go/Java/PHP base resolution. This is evidence of repeatability in this environment, not a guarantee against occasional outages.

| Controlled experiment (excluded from primary rate) | Result |
|---|---|
| gs-rest-service: explicit fully-qualified base pre-pull | build-and-runtime-pass |
| slim-skeleton: explicit fully-qualified base pre-pull | build-and-runtime-pass |
| fiber-optional-parameter: explicit fully-qualified base pre-pull | toolchain-version |

## Every case

All cases use an immutable SHA in `metrics.csv`, `manifest.json`, and per-case JSON. “Reference excluded” is not automatically “repository broken”. “Smoke pass” is only the recorded endpoint.

| # | App | Repo / app path | Reference output MiB | Ref build s | Worker s | Result |
|---:|---|---|---:|---:|---:|---|
| 1 | vite-vanilla | `vitejs/vite` / `packages/create-vite/template-vanilla` | 0.051 | 19.461 | 27.743 | missing-lockfile/install-policy |
| 2 | vite-vanilla-ts | `vitejs/vite` / `packages/create-vite/template-vanilla-ts` | 0.051 | 33.676 | 8.483 | missing-lockfile/install-policy |
| 3 | vite-react | `vitejs/vite` / `packages/create-vite/template-react` | 0.263 | 34.757 | 8.539 | missing-lockfile/install-policy |
| 4 | vite-react-ts | `vitejs/vite` / `packages/create-vite/template-react-ts` | 0.263 | 39.509 | 8.637 | missing-lockfile/install-policy |
| 5 | vite-vue | `vitejs/vite` / `packages/create-vite/template-vue` | 0.110 | 27.666 | 8.572 | missing-lockfile/install-policy |
| 6 | vite-vue-ts | `vitejs/vite` / `packages/create-vite/template-vue-ts` | 0.110 | 45.285 | 8.648 | missing-lockfile/install-policy |
| 7 | vite-svelte | `vitejs/vite` / `packages/create-vite/template-svelte` | 0.076 | 28.804 | 8.583 | missing-lockfile/install-policy |
| 8 | vite-svelte-ts | `vitejs/vite` / `packages/create-vite/template-svelte-ts` | 0.076 | 41.621 | 8.457 | missing-lockfile/install-policy |
| 9 | vite-solid | `vitejs/vite` / `packages/create-vite/template-solid` | 0.062 | 29.552 | 8.602 | missing-lockfile/install-policy |
| 10 | vite-solid-ts | `vitejs/vite` / `packages/create-vite/template-solid-ts` | 0.062 | 49.521 | 8.618 | missing-lockfile/install-policy |
| 11 | vite-preact | `vitejs/vite` / `packages/create-vite/template-preact` | 0.064 | 46.086 | 12.878 | missing-lockfile/install-policy |
| 12 | vite-preact-ts | `vitejs/vite` / `packages/create-vite/template-preact-ts` | 0.064 | 45.421 | 10.889 | missing-lockfile/install-policy |
| 13 | vite-lit | `vitejs/vite` / `packages/create-vite/template-lit` | 0.068 | 26.587 | 8.522 | missing-lockfile/install-policy |
| 14 | vite-lit-ts | `vitejs/vite` / `packages/create-vite/template-lit-ts` | 0.069 | 33.821 | 8.515 | missing-lockfile/install-policy |
| 15 | astro-minimal | `withastro/astro` / `examples/minimal` | 0.006 | 67.704 | 13.1 | missing-lockfile/install-policy |
| 16 | astro-basics | `withastro/astro` / `examples/basics` | 0.019 | 65.226 | 13.066 | missing-lockfile/install-policy |
| 17 | astro-blog | `withastro/astro` / `examples/blog` | 0.480 | 81.739 | 17.697 | missing-lockfile/install-policy |
| 18 | astro-portfolio | `withastro/astro` / `examples/portfolio` | 0.809 | 108.125 | 13.122 | missing-lockfile/install-policy |
| 19 | astro-framework-react | `withastro/astro` / `examples/framework-react` | 0.227 | 99.221 | 13.381 | missing-lockfile/install-policy |
| 20 | astro-framework-vue | `withastro/astro` / `examples/framework-vue` | 0.088 | 128.204 | 13.095 | missing-lockfile/install-policy |
| 21 | astro-framework-svelte | `withastro/astro` / `examples/framework-svelte` | 0.045 | 133.253 | 13.207 | missing-lockfile/install-policy |
| 22 | astro-with-tailwindcss | `withastro/astro` / `examples/with-tailwindcss` | 0.032 | 88.149 | 17.79 | missing-lockfile/install-policy |
| 23 | next-hello-world | `vercel/next.js` / `examples/hello-world` | 22.106 | 90.426 | 25.717 | missing-lockfile/install-policy |
| 24 | next-blog-starter | `vercel/next.js` / `examples/blog-starter` | 40.752 | 114.634 | 27.616 | missing-lockfile/install-policy |
| 25 | next-with-typescript | `vercel/next.js` / `examples/with-typescript` | 13.111 | 92.628 | 32.198 | missing-lockfile/install-policy |
| 26 | next-basic-css | `vercel/next.js` / `examples/basic-css` | 22.137 | 93.951 | 27.913 | missing-lockfile/install-policy |
| 27 | next-api-routes-rest | `vercel/next.js` / `examples/api-routes-rest` | 14.444 | 102.115 | 42.313 | missing-lockfile/install-policy |
| 28 | next-with-absolute-imports | `vercel/next.js` / `examples/with-absolute-imports` | 22.154 | 94.339 | 27.767 | missing-lockfile/install-policy |
| 29 | react-jobs | `bradtraversy/react-crash-2024` / `.` | 0.388 | 30.58 | 82.528 | build-and-runtime-pass |
| 30 | vue-jobs | `bradtraversy/vue-crash-2024` / `.` | 0.893 | 13.732 | 35.23 | build-and-runtime-pass |
| 31 | react-redux-realworld | `gothinkster/react-redux-realworld-example-app` / `.` | 1.868 | 212.633 | 4.041 | missing-lockfile/install-policy |
| 32 | node-prisma-realworld | `gothinkster/node-express-realworld-example-app` / `.` | 0.152 | 57.211 | 110.219 | missing-code-generation |
| 33 | nestjs-starter | `nestjs/typescript-starter` / `.` | 0.009 | 38.83 | 61.317 | image-build-pass/runtime-fail |
| 34 | sveltekit-realworld | `sveltejs/realworld` / `.` | — | 7.644 | — | reference-inconclusive |
| 35 | vue-docs | `vuejs/docs` / `.` | 27.847 | 65.948 | 70.742 | static-output-path/misleading-ssr |
| 36 | gatsby-starter | `gatsbyjs/gatsby-starter-default` / `.` | 1.842 | 172.346 | 231.684 | native-dependency/install-scripts |
| 37 | react-docs | `reactjs/react.dev` / `.` | — | — | — | reference-inconclusive |
| 38 | fiber-optional-parameter | `gofiber/recipes` / `optional-parameter` | 13.049 | 88.61 | 4.308 | base-image-resolution |
| 39 | fiber-dynamic-routing | `gofiber/recipes` / `dynamic-routing` | 14.306 | 90.705 | 6.406 | base-image-resolution |
| 40 | fiber-form-data | `gofiber/recipes` / `form-data` | 14.144 | 89.776 | 4.255 | base-image-resolution |
| 41 | fiber-multiple-ports | `gofiber/recipes` / `multiple-ports` | 13.049 | 87.779 | 4.191 | base-image-resolution |
| 42 | fiber-sse | `gofiber/recipes` / `sse` | 20.100 | 104.262 | 4.222 | base-image-resolution |
| 43 | fiber-stream-request-body | `gofiber/recipes` / `stream-request-body` | 13.049 | 100.348 | 6.452 | base-image-resolution |
| 44 | fiber-viper-config | `gofiber/recipes` / `viper-config` | 14.546 | 94.676 | 4.178 | base-image-resolution |
| 45 | actix-hello-world | `actix/examples` / `basics/hello-world` | 6.036 | 442.548 | 112.919 | workspace-context |
| 46 | actix-state | `actix/examples` / `basics/state` | 6.066 | 23.808 | 6.569 | workspace-context |
| 47 | actix-nested-routing | `actix/examples` / `basics/nested-routing` | 7.168 | 4.329 | 8.778 | workspace-context |
| 48 | actix-static-files | `actix/examples` / `basics/static-files` | 6.471 | 29.698 | 6.369 | workspace-context |
| 49 | actix-error-handling | `actix/examples` / `basics/error-handling` | 6.001 | 20.071 | 6.541 | workspace-context |
| 50 | gs-rest-service | `spring-guides/gs-rest-service` / `complete` | 19.138 | 47.324 | 4.211 | base-image-resolution |
| 51 | gs-serving-web-content | `spring-guides/gs-serving-web-content` / `complete` | 20.650 | 50.525 | 4.225 | base-image-resolution |
| 52 | flask-tutorial | `pallets/flask` / `examples/tutorial` | 0.057 | 8.229 | 20.013 | other-build-failure |
| 53 | fastapi-first-steps | `fastapi/fastapi` / `docs_src/first_steps` | 0.000 | 9.095 | 8.858 | detection-unsupported |
| 54 | fastapi-path-params | `fastapi/fastapi` / `docs_src/path_params` | 0.001 | 9.164 | 8.632 | detection-unsupported |
| 55 | fastapi-query-params | `fastapi/fastapi` / `docs_src/query_params` | 0.002 | 9.201 | 11.082 | detection-unsupported |
| 56 | django-realworld | `gothinkster/django-realworld-example-app` / `.` | — | 15.185 | — | reference-inconclusive |
| 57 | slim-skeleton | `slimphp/Slim-Skeleton` / `.` | 1.588 | 11.025 | 4.266 | base-image-resolution |
| 58 | laravel-skeleton | `laravel/laravel` / `.` | 47.539 | 40.679 | 4.318 | missing-lockfile/install-policy |
| 59 | rails-realworld | `gothinkster/rails-realworld-example-app` / `.` | — | 25.143 | — | reference-inconclusive |
| 60 | dotnet-insights-web | `dotnet/samples` / `azure/app-insights-aspnet-core-quickstart` | 7.770 | 32.252 | 50.727 | detection-unsupported |
| 61 | sinatra-simple | `sinatra/sinatra` / `examples` | 0.002 | 18.665 | 4.341 | detection-unsupported |

## Recommended fixes / exclusions

- **vite-vanilla:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **vite-vanilla-ts:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **vite-react:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **vite-react-ts:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **vite-vue:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **vite-vue-ts:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **vite-svelte:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **vite-svelte-ts:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **vite-solid:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **vite-solid-ts:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **vite-preact:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **vite-preact-ts:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **vite-lit:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **vite-lit-ts:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **astro-minimal:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **astro-basics:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **astro-blog:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **astro-portfolio:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **astro-framework-react:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **astro-framework-vue:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **astro-framework-svelte:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **astro-with-tailwindcss:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **next-hello-world:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **next-blog-starter:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **next-with-typescript:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **next-basic-css:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **next-api-routes-rest:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **next-with-absolute-imports:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **react-jobs:** Initial homepage smoke passed; not full app/backend/production deployment. **Possible solution:** Jobs API must be separately provisioned; upload to fake S3 is not deployment.
- **vue-jobs:** Initial homepage smoke passed; not full app/backend/production deployment. **Possible solution:** Jobs API must be separately provisioned; upload to fake S3 is not deployment.
- **react-redux-realworld:** Upstream starter has no lockfile; worker forces npm ci. **Possible solution:** Commit/use compatible lockfile or approved no-lock install mode with reproducibility warning; do not generate lock silently.
- **node-prisma-realworld:** Prisma client generation omitted; documented reference generation succeeds. **Possible solution:** Add approved generation step and preserve generated runtime client; use production entrypoint not dev Nx.
- **nestjs-starter:** NestJS start invokes nest CLI removed by production dependency install. **Possible solution:** Use node dist/main.js or start:prod; preserve artifact, runtime HTTP check before success.
- **sveltekit-realworld:** Pinned source has duplicate pnpm lockfile mapping; reference installation fails. **Possible solution:** Do not count as worker failure; repair upstream lockfile or use supported framework/toolchain, or rerun large app with deliberately approved resources.
- **vue-docs:** Vue docs build completes but output .vitepress/dist is mistaken for missing dist/SSR. **Possible solution:** Recognize VitePress/static output path or explicit output setting; never infer SSR solely from missing default output.
- **gatsby-starter:** Sharp native binary absent with --ignore-scripts; Alpine compatibility also matters. **Possible solution:** Allow explicitly approved native preparation, select compatible glibc/musl image, verify native libs before build.
- **react-docs:** Large docs reference hits 2GiB OOM; telemetry timeout/leftover processes, no worker trial. **Possible solution:** Do not count as worker failure; repair upstream lockfile or use supported framework/toolchain, or rerun large app with deliberately approved resources.
- **fiber-optional-parameter:** Noninteractive Buildah cannot resolve unqualified template base name. **Possible solution:** Use fully qualified pinned base refs; fresh-storage smoke test; then address toolchain compatibility.
- **fiber-dynamic-routing:** Noninteractive Buildah cannot resolve unqualified template base name. **Possible solution:** Use fully qualified pinned base refs; fresh-storage smoke test; then address toolchain compatibility.
- **fiber-form-data:** Noninteractive Buildah cannot resolve unqualified template base name. **Possible solution:** Use fully qualified pinned base refs; fresh-storage smoke test; then address toolchain compatibility.
- **fiber-multiple-ports:** Noninteractive Buildah cannot resolve unqualified template base name. **Possible solution:** Use fully qualified pinned base refs; fresh-storage smoke test; then address toolchain compatibility.
- **fiber-sse:** Noninteractive Buildah cannot resolve unqualified template base name. **Possible solution:** Use fully qualified pinned base refs; fresh-storage smoke test; then address toolchain compatibility.
- **fiber-stream-request-body:** Noninteractive Buildah cannot resolve unqualified template base name. **Possible solution:** Use fully qualified pinned base refs; fresh-storage smoke test; then address toolchain compatibility.
- **fiber-viper-config:** Noninteractive Buildah cannot resolve unqualified template base name. **Possible solution:** Use fully qualified pinned base refs; fresh-storage smoke test; then address toolchain compatibility.
- **actix-hello-world:** Cargo member copied without parent workspace manifests/dependencies. **Possible solution:** Separate build context from app target; preserve workspace, cargo build --locked -p selected-package; configure external bind.
- **actix-state:** Cargo member copied without parent workspace manifests/dependencies. **Possible solution:** Separate build context from app target; preserve workspace, cargo build --locked -p selected-package; configure external bind.
- **actix-nested-routing:** Cargo member copied without parent workspace manifests/dependencies. **Possible solution:** Separate build context from app target; preserve workspace, cargo build --locked -p selected-package; configure external bind.
- **actix-static-files:** Cargo member copied without parent workspace manifests/dependencies. **Possible solution:** Separate build context from app target; preserve workspace, cargo build --locked -p selected-package; configure external bind.
- **actix-error-handling:** Cargo member copied without parent workspace manifests/dependencies. **Possible solution:** Separate build context from app target; preserve workspace, cargo build --locked -p selected-package; configure external bind.
- **gs-rest-service:** Noninteractive Buildah cannot resolve unqualified template base name. **Possible solution:** Use fully qualified pinned base refs; fresh-storage smoke test; then address toolchain compatibility.
- **gs-serving-web-content:** Noninteractive Buildah cannot resolve unqualified template base name. **Possible solution:** Use fully qualified pinned base refs; fresh-storage smoke test; then address toolchain compatibility.
- **flask-tutorial:** pyproject.toml detected but npm/default PM falls into requirements.txt template; file absent. **Possible solution:** Support pip install of pyproject project and Flask module entrypoint; preserve required repo context.
- **fastapi-first-steps:** No recognized app manifest: bare Python/Ruby sample or csproj-only .NET app is rejected. **Possible solution:** Require/declare dependency manifest for bare scripts; detect csproj/fsproj for .NET; explicit start and dependency configuration.
- **fastapi-path-params:** No recognized app manifest: bare Python/Ruby sample or csproj-only .NET app is rejected. **Possible solution:** Require/declare dependency manifest for bare scripts; detect csproj/fsproj for .NET; explicit start and dependency configuration.
- **fastapi-query-params:** No recognized app manifest: bare Python/Ruby sample or csproj-only .NET app is rejected. **Possible solution:** Require/declare dependency manifest for bare scripts; detect csproj/fsproj for .NET; explicit start and dependency configuration.
- **django-realworld:** Legacy Django cannot import under Python3.12 reference. **Possible solution:** Do not count as worker failure; repair upstream lockfile or use supported framework/toolchain, or rerun large app with deliberately approved resources.
- **slim-skeleton:** Noninteractive Buildah cannot resolve unqualified template base name. **Possible solution:** Use fully qualified pinned base refs; fresh-storage smoke test; then address toolchain compatibility.
- **laravel-skeleton:** Wrong primary framework: Vite frontend selected instead of Laravel/PHP; then missing npm lockfile. **Possible solution:** Prioritize explicit full-stack/PHP recipe or split services; run PHP and asset build steps, provision DB/env; add lockfile policy.
- **rails-realworld:** Bundler1.10.6/Ruby3.3 incompatibility in reference. **Possible solution:** Do not count as worker failure; repair upstream lockfile or use supported framework/toolchain, or rerun large app with deliberately approved resources.
- **dotnet-insights-web:** No recognized app manifest: bare Python/Ruby sample or csproj-only .NET app is rejected. **Possible solution:** Require/declare dependency manifest for bare scripts; detect csproj/fsproj for .NET; explicit start and dependency configuration.
- **sinatra-simple:** No recognized app manifest: bare Python/Ruby sample or csproj-only .NET app is rejected. **Possible solution:** Require/declare dependency manifest for bare scripts; detect csproj/fsproj for .NET; explicit start and dependency configuration.

## Resource safety and harness incidents

- Sequential 1.5CPU/2GiB build containers, 512 PID cap, worker retries=1. Disk guard required >=20GiB free; per-builder storage guard capped25GiB. No concurrent app builds. Per-case build containers/images removed and dangling layers pruned only inside campaign-specific stores.
- Original reference PID1 did not reap orphaned children: ~459 zombie processes caused thread exhaustion. Recreated only reference container with `--init`; affected references were retried. Incorrect reference preview args/routes/test secrets were corrected from repo evidence. These were harness failures, not worker/app bugs.
- react.dev (825 generated pages) hit a confirmed 2GiB reference OOM; no worker trial was counted. It remains resource-inconclusive. SvelteKit source lockfile has duplicate mapping; legacy Django/Rails could not pass current toolchain references and are excluded.
- Detailed telemetry is preserved in per-case JSON: sampled CPU/memory/PIDs, cgroup before/after counters where available. Memory peaks may be process/container-lifetime when reset is not supported. No worker cgroup OOM event was seen.
- Cleanup details and exact removed resources are in `cleanup.json` alongside the preserved evidence archive. No global Docker prune and no unrelated images/containers were removed.

## Evidence files

- `metrics.csv`: all 61 cases, metrics, causes and possible solutions.
- `summary.json`: aggregate rates, nominal confidence intervals, stack/size distributions, repeat outcomes.
- `manifest.json`, `PROTOCOL.md`, `campaign.py`: selection and reproducible harness.
- `results/`, `repeats/`, `source-evidence/`: logs, raw outcomes, pinned manifests/configs, image descriptions.
- `harness-incidents.md`: reference environment mistakes and recoveries.
- Sources/caches removed after preservation; archived evidence location is reported with completion.


## Latest results — broad compatibility campaign

# Broad worker compatibility campaign — final queue report

Tested worker source: `d15eae067cf77a8dcb5d6d6aaf8a59b166c65106`. No deployment or publication was attempted; the worker was configured to use an isolated local stub. Earlier measured worker trials were not rerun.

**Queue exhausted: 23/23 identities accounted for; 19 complete harness records and 19 measured worker image outcomes.** An unmeasured case is not a worker failure.

| Metric | Independent reference | Unchanged worker |
|---|---:|---:|
| Image built | 7/19 attempted | 1/19 attempted |
| Local HTTP | 3/3 attempted | 0/0 attempted |
| Cross-container HTTP | 3/3 attempted | 0/0 attempted |
| Case-specific functional check | 1/3 attempted | 0/0 attempted |

The image metric counts diagnostic reference stages where an image exists; a build-only or typecheck reference is not full application qualification. HTTP readiness is not dependency behavior. The functional row counts explicit runtime probes only; build-time diagnostics (if any) are described in each case scope. A dash means unmeasured or not applicable.

| Case | Reference image / local / external / functional | Worker image / local / external / functional | First relevant failure or coverage limit |
|---|---|---|---|
| [nest-bull-redis](pilot-01/result.json) | fail / — / — / — | fail / — / — / — | Ref: npm error code ERESOLVE; Worker: npm error code ERESOLVE |
| [tanstack-start-query](pilot-02/result.json) | fail / — / — / — | fail / — / — / — | Ref: [WARN] GET https://registry.npmjs.org/ipaddr.js/-/ipaddr.js-2.2.0.tgz error (Failed to fetch https://registry.npmjs.org/ipaddr.js/-/ipaddr.js-2.2.0.tgz: error decoding response body: request or response body error: operation timed out) — 1; Worker: Error: building at STEP "COPY --from=builder /app/.output ./.output": checking on sources under "/var/lib/containers/storage/overlay/5ee1bc445e7d0192e86f768983a984aeb115977ee824937585d7e667d5340b4d/merged": copier: stat: "/app/.output": no such file or directory |
| [fastapi-uv-sqlmodel](pilot-03/result.json) | fail / — / — / — | fail / — / — / — | Ref: Error: creating build container: short-name resolution enforced but cannot prompt without a TTY; Worker: Error: building at STEP "COPY requirements.txt ./": checking on sources under "/tmp/zerodevops-build/b285d057-96cc-4133-be53-fb157f74b311/backend": copier: stat: "/requirements.txt": no such file or directory |
| [django-cms](case-04-django-cms-setup-retry/result.json) | pass / pass / pass / fail | fail / — / — / — | Ref: FileNotFoundError: [Errno 2] No such file or directory: '/app/democontent/home.json'; Worker: Error: pg_config executable not found. |
| [angular-realworld](case-05-angular-realworld/result.json) | fail / — / — / — | fail / — / — / — | Ref: Error: creating build container: short-name resolution enforced but cannot prompt without a TTY; Worker: ✘ [ERROR] Could not resolve "realworld/assets/theme/styles.css" |
| [react-router-express](case-06-react-router-express/result.json) | fail / — / — / — | fail / — / — / — | Ref: Error: building at STEP "RUN unlocked npm install --no-package-lock --ignore-scripts && npm run build && test -f build/server/index.js && npm prune --omit=dev --ignore-scripts": exit status 127; Worker: Error: building at STEP "COPY --from=builder /app/dist ./dist": checking on sources under "/var/lib/containers/storage/overlay/cc19456b83dd217176ce2fca5c25005e50c3a94830ce82ddb547a5e6eedfe2ed/merged": copier: stat: "/app/dist": no such file or directory |
| [gin-multipart](case-07-gin-multipart/result.json) | pass / pass / pass / fail | fail / — / — / — | Ref: Error: exit status 1; Worker: 2026-11-19T14:25:12.359Z	ERROR	worker/worker.go:109	deployment job failed	{"request_id": "zdround2-55fe619d-6d10-4f44-8c23-b767bdd6d0d1", "deployment_id": "55fe619d-6d10-4f44-8c23-b767bdd6d0d1", "error": "clone/prepare failed: no framework detected"} |
| [nuxt-content](case-08-nuxt-content/result.json) | fail / — / — / — | fail / — / — / — | Ref: Error: building at STEP "RUN npm install --no-package-lock --ignore-scripts && npm run build && test -f .output/server/index.mjs": exit status 1; Worker: Error: building at STEP "RUN npm run build": exit status 1; Scope: uses standalone published Nuxt Content rather than root workspace override; unlocked |
| [sveltekit-workspace](case-09-sveltekit-workspace/result.json) | fail / — / — / — | fail / — / — / — | Ref: Error: ERR_PNPM_RECURSIVE_RUN_FIRST_FAIL; Worker: 2026-11-19T14:46:33.710Z	ERROR	worker/worker.go:109	deployment job failed	{"request_id": "zdround2-b638f7fe-2e58-4ea0-ad2e-2cb9c783d184", "deployment_id": "b638f7fe-2e58-4ea0-ad2e-2cb9c783d184", "error": "clone/prepare failed: SvelteKit adapter is ambiguous or unsupported; select a; Scope: root lock plus explicit upstream Kit prepublish preparation; local apps and catalog |
| [trpc-fastify](case-10-trpc-fastify/result.json) | pass / — / — / — | fail / — / — / — | Worker: 2026-11-19T15:08:24.829Z	ERROR	worker/worker.go:109	deployment job failed	{"request_id": "zdround2-bca2f3a9-8c89-4926-b154-ea2d308d7c18", "deployment_id": "bca2f3a9-8c89-4926-b154-ea2d308d7c18", "error": "build failed: signal: killed"}; Scope: build only; RPC client/session probe not qualified |
| [drizzle-next](case-11-drizzle-next/result.json) | pass / — / — / — | fail / — / — / — | Worker: Error: POSTGRES_URL environment variable is not set; Scope: typecheck only; production build/pricing needs Stripe setup not approved for live use; no migrations, payment or functional pass |
| [nest-typeorm](case-12-nest-typeorm/result.json) | fail / — / — / — | fail / — / — / — | Ref: npm error code ERESOLVE; Worker: npm error code ERESOLVE; Scope: unlocked build; runtime requires MySQL on loopback per source; no DB qualification |
| [nest-graphql](case-13-nest-graphql/result.json) | fail / — / — / — | fail / — / — / — | Ref: npm error code ERESOLVE; Worker: npm error code ERESOLVE; Scope: unlocked build; GraphQL schema check via HTTP POST |
| [actual-budget](case-14-actual-budget/result.json) | fail / — / — / — | fail / — / — / — | Ref: originalError: undefined,; Worker: npm error code EUNSUPPORTEDPROTOCOL; Scope: reference uses upstream Yarn scripts for native install; standalone native SQLite query, not server persistence or bank integrations |
| [netbox](case-15-netbox/result.json) | — / — / — / — | — / — / — / — | Harness: RuntimeError: ('docker', 'network', 'create', '--label') exited 1: Error response from daemon: all predefined address pools have been fully subnetted ; Ref: no image result; Scope: dependency imports/image construction only; no PostgreSQL/Redis application runtime |
| [flask-admin](case-16-flask-admin/result.json) | — / — / — / — | fail / — / — / — | Ref: ('docker', 'pull', 'ghcr.io/astral-sh/uv:0.9.26') exited 1: <3>WSL (53280 - ) ERROR: UtilAcceptVsock:281: accept4 failed 110 error getting credentials - err: exit status 1, out: '' ; Worker: Error: building at STEP "COPY requirements.txt ./": checking on sources under "/tmp/zerodevops-build/58481aee-f70c-4a86-927f-a83a9c64efb7/examples/forms_files_images": copier: stat: "/requirements.txt": no such file or directory; Scope: dependency smoke only; documented example uses development server; upload not tested |
| [mealie](case-17-mealie/result.json) | — / — / — / — | — / — / — / — | Harness: RuntimeError: ('docker', 'network', 'create', '--label') exited 1: Error response from daemon: all predefined address pools have been fully subnetted ; Ref: no image result; Scope: upstream docker/Dockerfile; complete image build only; recipe create/read not qualified |
| [spring-jpa](case-18-spring-jpa/result.json) | fail / — / — / — | pass / — / — / — | Ref: > Could not resolve all dependencies for configuration ':testRuntimeClasspath'.; Scope: Gradle tests and CLI JPA customer output; no HTTP expected |
| [quarkus-panache](case-19-quarkus-panache/result.json) | pass / — / — / — | fail / — / — / — | Worker: time="2026-11-19T15:16:09Z" level=warning msg="Failed, retrying in 1s ... (1/3). Error: initializing source docker://eclipse-temurin:21-jdk-alpine: pinging container registry registry-1.docker.io: Get \"https://registry-1.docker.io/v2/\": net/http: TLS handshake timeout"; Scope: fast-JAR build; no PostgreSQL-backed runtime test |
| [aspnet-min-api](case-20-aspnet-min-api/result.json) | pass / pass / pass / pass | fail / — / — / — | Worker: 2026-11-19T15:31:00.156Z	ERROR	worker/worker.go:109	deployment job failed	{"request_id": "zdround2-bf2961c4-a732-4626-b318-901caff1d7b0", "deployment_id": "bf2961c4-a732-4626-b318-901caff1d7b0", "error": "clone/prepare failed: no framework detected"}; Scope: in-memory EF API create/list; project and endpoint contract checked independently |
| [symfony-demo](case-21-symfony-demo/result.json) | pass / — / — / — | fail / — / — / — | Worker: Error: building at STEP "RUN composer install --no-dev --no-interaction --no-progress --optimize-autoloader": exit status 1; Scope: Composer and console boot only; no seeded Doctrine runtime |
| [hexpm](case-22-hexpm/result.json) | — / — / — / — | — / — / — / — | Harness: RuntimeError: ('docker', 'network', 'create', '--label') exited 1: Error response from daemon: all predefined address pools have been fully subnetted ; Ref: no image result; Scope: upstream release image build only; production release not started without reviewed local service configuration |
| [sqlx-axum](case-23-sqlx-axum/result.json) | fail / — / — / — | — / — / — / — | Harness: TimeoutExpired: Command '['docker', 'exec', 'zdbroad-case23-db', 'psql', '-U', 'e2e', '-d', 'e2e', '-A', '-t', '-F', '/', '-c', "SELECT status, COALESCE(error_message,''), COALESCE(output_url,'') FROM deployments WHERE id='c4006b37-08ae-4e1d-b3b3-4552128ff8b6';"]' timed out after ; Ref: Error: building at STEP "RUN SQLX_OFFLINE=true cargo build --locked --manifest-path examples/postgres/axum-social-with-tests/Cargo.toml": exit status 1; Scope: workspace build only; no live PostgreSQL/CRUD test |

## Raw-log clarifications

- **Nuxt Content:** Both build logs explicitly report that `better-sqlite3` is required. This trial does not establish a worker-only native-module failure.
- **Actual Budget:** The reference reported `fatal: not a git repository` when building from an exported source tree. The worker stopped earlier: its nested npm recipe rejected a `workspace:*` dependency. Native SQLite was not exercised.
- **Quarkus Panache:** The worker log also contains a transient registry warning, but the terminal build error was `Directory '/app' does not contain a Gradle build`. The independent Maven fast-JAR reference built. PostgreSQL behavior was not tested.
- **Symfony Demo:** Composer installed packages, then `cache:clear` failed with `Could not open input file: ./bin/console`. The independent reference copied the app first, disabled Composer scripts and booted the console; it did not test Doctrine runtime behavior.
- **SQLx/Axum:** A read-only inspection of the retained worker log confirmed exact pinned checkout and entry into the Buildah build. The status-poll timeout still leaves the worker image outcome unmeasured.

## Interpretation

- Shared installation failures are not automatically worker-specific. An inconclusive reference does not erase a directly observed worker recipe or discovery mismatch.
- The FastAPI reference-only repeats and the initial Django setup error are retained as separate attempts. They are not additional cases. See [interim evidence and repeat links](PROGRESS.md).
- A reduced-memory parallel lane and a serial heavyweight lane are not comparable performance benchmarks. Unlocked resolutions are not reproducible.
- The Drizzle build-only check does not qualify production Next/Stripe setup, Drizzle migrations or CRUD. External payment, cloud and production credentials were not supplied.
- Runtime scopes and raw logs are linked above. No rate here establishes an updated outcome for the earlier cohorts.

Campaign containers still running in final check: `[]`. No global pruning was performed.

Queue evidence: [final controller state](queue-state.json).



## Latest campaign — worker follow-up on fix-builds

# Final campaign summary

## Measured results

Tested product: `d15eae067cf77a8dcb5d6d6aaf8a59b166c65106` on `fix-builds`.

| Batch | Cases | Images built | Local HTTP passed | External HTTP passed / attempted |
|---|---:|---:|---:|---:|
| Previous failures, prior worker | 20 | 2 (10%) | 0 (0%) | Not measured |
| Previous failures, latest worker | 20 | 12 (60%) | 12 (60%) | 11/12 |
| Fresh applications, latest worker | 12 | 12 (100%) | 12 (100%) | 10/12 |
| Fresh applications, independent reference | 12 | 12 (100%) | 12 (100%) | 10/12 |

The historical failure cohort gained ten image builds and twelve local HTTP passes. All 12 fresh cases qualified through independent reference builds and local probes; none was dropped or replaced.

The earlier complete 57-case measurement remains **39 images and 37 local HTTP passes** on `20544b6`. The other 37 previously passing applications were not retested on the latest source; combining old successes with new recoveries would be an unverified projection, not a measured full-cohort result.

## Results in your requested categories

| Category | Latest measured result |
|---|---|
| **1. Optional Next.js public/** | All **5/5** affected Next applications built and passed local/external HTTP: API routes, basic CSS, hello-world, absolute imports, TypeScript. Two originally failed on public packaging; three first hit checkout timeouts and then the same packaging error during separate controlled repeats. |
| **2. Framework detection/output selection** | **5/5** built and passed local/external HTTP: Astro/Svelte, Vite/Svelte, Vite/Svelte TypeScript, Laravel, Vue/VitePress docs. |
| **3. Production startup/runtime assets** | **2/2** built and passed local HTTP: Nest and Actix static files. Nest passed external HTTP. Actix still binds loopback and failed external HTTP; application binding was not changed. |
| **4. Generation/dependency formats/discovery** | **Excluded as requested.** Eight terminal failures remain: Prisma, Gatsby, Flask, three FastAPI examples, Sinatra, and .NET. |
| **5. Checkout reliability** | All **4/4** original checkout-timeout cases reached their exact pinned HEAD. Three Next cases built/served; .NET reached unsupported discovery. All four fetches succeeded on their first attempt, so these trials do not establish that retries caused recovery. This category overlaps the expanded scope above and must not be added again. |

## Fresh dependency/toolchain rotation

- Astro with Preact, Solid, MDX and Markdoc, using observed reference Node **24.22.3** / npm **11.19.1** and declared Astro **7.4** integration ranges.
- Three Fiber v3 samples using Go **1.27.0** and Fiber **v3.7.0** declarations.
- Two Axum workspace/path-dependency samples using observed reference Rust/Cargo **1.100.0**, Tokio and Tower HTTP.
- Spring REST CORS and Thymeleaf form samples using observed reference JDK **21.0.13** / Maven **3.10.0**, with declared Spring Boot **4.0.9** and **4.1.2**.
- Next/Sass with declared React 19, Sass 1.82 and TypeScript 5.7 ranges.

Observed reference versions are not automatically exact worker-builder dependency versions. Unlocked dependency resolution and image tags may change. Coverage is **Linux amd64 container smoke compatibility**, not Windows/macOS-native/ARM or universal hosting-platform support.

## Remaining build/detection failures

| Cause | Applications | Concrete result |
|---|---|---|
| Missing generated Prisma client | node-prisma-realworld | TypeScript cannot import exported `PrismaClient` from `@prisma/client`. |
| Restricted native dependency preparation | gatsby-starter | Missing `sharp-linuxmusl-x64.node`. Install-script restrictions were preserved. |
| Unsupported Python dependency format | flask-tutorial | Python recipe tries to copy absent `requirements.txt`; pinned project uses a different format. |
| Unsupported discovery | dotnet-insights-web, three FastAPI samples, sinatra-simple | No framework detected at the selected app path. |

## External reachability gaps

Actix static files and both fresh Axum samples pass local HTTP but remain unreachable from another container because of their original loopback bindings. Both Axum references show the same result. This is not a failed image build, and a cross-container pass is still not a production deployment.

## Verification and recovery

- Full worker suite passed before freeze and at final acceptance.
- Live native/asset execution fixtures passed; **10/10** fresh-storage image pulls passed.
- Supplemental audit verified **622 source entries**, pinned upstream tree/blob linkage and modes, all twelve dependency declarations, both reference/worker probe isolation, and **36 successful trial warm-request sets** with no recorded missing requests or unexpected statuses.
- **63 product hashes, 460 prior evidence files, both manifests and active harness fingerprints remained unchanged.**
- Controller telemetry timeouts affected two Next measurements after final images were already built. The original images were inspected and probed later **without rebuilding**. Original logs/incomplete records are archived; the TypeScript job's canceled broker-finalization state remains visible despite image/HTTP success. Five unsubmitted cases then completed serially.
- All campaign services are stopped; evidence, containers and build stores are retained. No unrelated resources were removed.

**Nothing was deployed or published.** Worker images were exercised through mounted compiled binaries/templates in the retained test runtime; the worker Dockerfile was not rebuilt.

## Deliverables

- [Full report](REPORT.md): complete per-case comparison, original blocker categories, commands, dependency rotation, limitations.
- [Failure-batch report](FAILURES-REPORT.md).
- [Fresh-batch report](FRESH-REPORT.md).
- [Machine-readable summary](summary.json).
- [Every case as CSV](metrics.csv).
- [Completed handoff](COMPLETED-HANDOFF.md).


# Campaign completed — final handoff

The earlier `RESUME-HANDOFF.md` is retained as historical evidence and is superseded by this document.

## Final state

- Branch: `fix-builds`.
- Frozen tested product: `d15eae067cf77a8dcb5d6d6aaf8a59b166c65106`.
- Original complete 57-case campaign remains 39 image builds and 37 local HTTP passes on `20544b6`.
- Follow-up completed all 20 historical failures and 12 fresh cases.
- Historical failures: 12 images, 12 local HTTP passes, 11 cross-container HTTP passes out of 12 attempted. Eleven worker states are success; Next TypeScript retains its original canceled broker-finalization state despite a passing retained image.
- Fresh worker and independent references: all 12 images and local HTTP checks pass; each has 10 cross-container passes out of 12 attempts. Both Axum samples bind loopback.
- Eight remaining historical failures belong to intentionally excluded generation, native dependency preparation, dependency formats or discovery support.
- No complete 57-case retest on the follow-up source, so no new full-cohort rate is established.
- No production deployment/publication or worker Dockerfile rebuild.

## Verification and evidence

- `REPORT.md`, `summary.json`, `metrics.csv`: final report, machine-readable summary and every case.
- `INDEPENDENT-REVIEW.md`: read-only evidence review findings and addressed acceptance gaps.
- `supplemental-evidence-audit.json`: 622 source files, six pinned upstream tree/blob inventories/modes, twelve dependency declarations, reference/worker isolation and warm HTTP records checked.
- `worker-suite-final-acceptance.log` and `report-regression-tests.log`: full worker suite and reporting regression tests passed.
- `native-integration.json` and `cold-pulls.json`: live fixtures passed, ten of ten fresh-store image pulls passed.
- `preservation-check.json`: all 63 product hashes, 460 prior evidence files and both manifests unchanged.
- `recovery-20261117/`: original telemetry/controller failure evidence, measurement recovery of two already-built Next images without rebuilding, and serial completion of five unsubmitted cases.
- `pre-campaign-verification-942664f/`: failed runtime-ownership fixture preserved separately from application counts; correction tested before app dispatch.
- `final-services/` and `cleanup.json`: final logs/inspect/rows captured; all campaign services stopped. Containers, volumes/build stores/networks and prior evidence retained. No global prune or unrelated-resource changes.

## Local product commits

- `ae784de`: npm lockfile policy, image qualification and native workspace context/targets.
- `7d853dc`: native execution checks and campaign tooling.
- `20544b6`: Node engine-aware selection.
- `942664f`: framework/output ownership, optional Next public, startup/assets and checkout policy.
- `d15eae0`: runtime asset ownership correction.
- `4c07e71`: prior campaign evidence/harness archive (not a product change).

Do not restart jobs or repeat completed trials solely to improve the headline. Any further feature work, broader platform certification or full-cohort retest should use a separate evidence set and explicit scope. Point 4 remains excluded from these product fixes.
