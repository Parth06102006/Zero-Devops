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
