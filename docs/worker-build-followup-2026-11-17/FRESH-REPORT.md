# Fresh compatibility batch — measured report

Tested product: `d15eae067cf77a8dcb5d6d6aaf8a59b166c65106`. Full methods and scope: [REPORT.md](REPORT.md).

All **12/12** fresh applications independently reference-qualified. Worker and reference trials each built **12/12** images and passed **12/12** local HTTP probes; each passed **10/12** external probes. No reference-inconclusive cases or replacements.

Dependency/toolchain rotation: Astro Preact/Solid/MDX/Markdoc; Fiber v3/Go1.27; Axum/Tokio/TowerHTTP/Rust workspaces; Spring/JDK21/Thymeleaf; Next/React19/Sass/TypeScript. Exact declared requirements and observed reference tools are in [REPORT.md](REPORT.md) and fresh-dependency-requirements.json.

Both Axum applications bind loopback and fail external probes in worker and reference trials. Coverage is declared endpoint/status smoke compatibility in Linux amd64 containers, not complete CORS/cache/auth/form/asset behavior or cross-platform certification.

## Every case

| Case | Image | Local HTTP | External HTTP | Worker state | Outcome |
|---|---|---|---|---|---|
| fresh-astro-framework-preact | pass | pass | pass | success | build-and-runtime-pass |
| fresh-astro-framework-solid | pass | pass | pass | success | build-and-runtime-pass |
| fresh-astro-with-mdx | pass | pass | pass | success | build-and-runtime-pass |
| fresh-astro-with-markdoc | pass | pass | pass | success | build-and-runtime-pass |
| fresh-fiber-hello-world | pass | pass | pass | success | build-and-runtime-pass |
| fresh-fiber-404-handler | pass | pass | pass | success | build-and-runtime-pass |
| fresh-fiber-advanced-caching | pass | pass | pass | success | build-and-runtime-pass |
| fresh-axum-hello-world | pass | pass | fail | success | build-and-runtime-pass |
| fresh-axum-cors | pass | pass | fail | success | build-and-runtime-pass |
| fresh-gs-rest-service-cors | pass | pass | pass | success | build-and-runtime-pass |
| fresh-gs-handling-form-submission | pass | pass | pass | success | build-and-runtime-pass |
| fresh-next-with-sass | pass | pass | pass | success | build-and-runtime-pass |

Full raw logs, pinned repository/path/SHA identities, original worker/reference records, image inspections, phase timings and probe responses are retained. Neither batch establishes a new full 57-case rate. Nothing was deployed or published. See [EXECUTIVE-SUMMARY.md](EXECUTIVE-SUMMARY.md) for key conclusions and [REPORT.md](REPORT.md) for limitations.
