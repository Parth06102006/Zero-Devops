# Fresh compatibility cohort

Twelve application paths were selected and pinned before build trials. None appears in the original 61-candidate manifest (including its 57 qualified cases). Their outcomes are reported separately from the failure-only comparison cohort. Selection was based on source manifests and explicit HTTP samples, not on worker results.

| Cases | Dependency/tool variation | Reference HTTP check |
|---|---|---|
| Astro Preact, Solid, MDX, Markdoc | Astro 7.4 ranges, Preact 11, Solid 1.9, MDX/Markdoc integrations; Node >=22.23.4, Node24 image | Generated static homepage via Astro preview |
| Fiber hello-world, 404-handler, advanced-caching | Fiber v3.7.0, Go1.27.0 requirement; middleware/cache/keyauth | `/`, `/hello`, `/health` respectively |
| Axum hello-world, CORS | Local Axum path dependency, Tokio, Tower HTTP; nested workspace and locked dependency graph | `/` on3000 and `/json` on4000 |
| Spring REST CORS, form submission | Spring Boot4, Java17 source compatibility, JDK21 builder, Thymeleaf | `/greeting` |
| Next Sass | Next latest range, React19, Sass1.82 range, TypeScript5.7 range | Homepage |

Full immutable SHAs and exact commands are in `fresh-manifest.json`. Declared dependency ranges and toolchain requirements are in `fresh-dependency-requirements.json`; ranges do not establish exact resolved dependency versions. Per-repository source metadata under `fresh-source-evidence/` records Git blob hashes and SHA256 for downloaded files. Selected Node/Go/Java paths are restored at their repository-relative locations; the entire Axum repository is restored to preserve workspace and local library dependencies. No application files or manifests are changed to force compatibility.

Each fresh case gets a reference image build and HTTP smoke trial from its pinned source, independently of worker detection, followed by the worker trial even if the reference is inconclusive. Qualification requires both reference image creation and local HTTP readiness. Unavailable required images, dependency resolution failures, or failed reference probes are reported rather than silently removed or downgraded to older toolchains.

Install scripts remain disabled for Node reference installs, matching the worker restriction. Lockless Node dependency resolution is explicitly non-reproducible. Go and Java use their declared manifests; Axum uses its committed workspace lockfile. The advanced-caching sample receives only a test API key needed for startup; its health check does not validate cache invalidation or authentication behavior.

Axum samples intentionally bind loopback. Local HTTP success is kept separate from the additional external-container probe and from deployment success. No application binding is rewritten. CORS checks hit the backend endpoint; browser CORS behavior and companion frontend integration are not established. Static framework preview and PHP/Java development samples are compatibility smoke checks, not production readiness certification. No deployment or publication is performed.
