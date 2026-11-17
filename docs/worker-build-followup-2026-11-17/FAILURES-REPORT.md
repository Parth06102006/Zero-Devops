# Previous-failure batch — measured report

Tested product: `d15eae067cf77a8dcb5d6d6aaf8a59b166c65106`. Full methods and scope: [REPORT.md](REPORT.md).

All **20/20** historical failure cases completed: **12/20 images**, **12/20 local HTTP passes**, **11/12 external HTTP passes**. Prior source: 2/20 images and 0/20 local HTTP passes.

## Requested categories (expanded scope)

| Category | Cases | Images | Local HTTP | External passed / attempted |
|---|---:|---:|---:|---:|
| 1. Optional Next.js public/ handling | 5 | 5 | 5 | 5/5 |
| 2. Framework detection and output selection | 5 | 5 | 5 | 5/5 |
| 3. Production startup and runtime assets | 2 | 2 | 2 | 1/2 |
| 4. Generation, dependency formats and discovery — excluded | 8 | 0 | 0 | 0/0 |
| 5. Checkout reliability — overlaps categories above | 4 | 3 | 3 | 3/3 |

Categories 1–4 partition twenty cases; category 5 overlaps and is not an additional denominator. Original primary-blocker groupings are preserved separately in the full report. Point 4 was intentionally excluded. All eight remaining build/detection failures are in that excluded scope. Actix remains loopback-only externally.

Next TypeScript retains its original canceled worker state after broker-channel closure; the final original image and later HTTP probes passed. Both recovered Next images were measured without rebuilding. See recovery-20261117/.

## Every case

| Case | Image | Local HTTP | External HTTP | Worker state | Outcome |
|---|---|---|---|---|---|
| actix-static-files | pass | pass | fail | success | build-and-runtime-pass |
| astro-framework-svelte | pass | pass | pass | success | build-and-runtime-pass |
| dotnet-insights-web | fail | not attempted | not attempted | canceled | detection-unsupported |
| fastapi-first-steps | fail | not attempted | not attempted | canceled | detection-unsupported |
| fastapi-path-params | fail | not attempted | not attempted | canceled | detection-unsupported |
| fastapi-query-params | fail | not attempted | not attempted | canceled | detection-unsupported |
| flask-tutorial | fail | not attempted | not attempted | canceled | other-build-failure |
| gatsby-starter | fail | not attempted | not attempted | canceled | native-dependency/install-scripts |
| laravel-skeleton | pass | pass | pass | success | build-and-runtime-pass |
| nestjs-starter | pass | pass | pass | success | build-and-runtime-pass |
| next-api-routes-rest | pass | pass | pass | success | build-and-runtime-pass |
| next-basic-css | pass | pass | pass | success | build-and-runtime-pass |
| next-hello-world | pass | pass | pass | success | build-and-runtime-pass |
| next-with-absolute-imports | pass | pass | pass | success | build-and-runtime-pass |
| next-with-typescript | pass | pass | pass | canceled | build-and-runtime-pass |
| node-prisma-realworld | fail | not attempted | not attempted | canceled | missing-code-generation |
| sinatra-simple | fail | not attempted | not attempted | canceled | detection-unsupported |
| vite-svelte | pass | pass | pass | success | build-and-runtime-pass |
| vite-svelte-ts | pass | pass | pass | success | build-and-runtime-pass |
| vue-docs | pass | pass | pass | success | build-and-runtime-pass |

Full raw logs, pinned repository/path/SHA identities, original worker/reference records, image inspections, phase timings and probe responses are retained. Neither batch establishes a new full 57-case rate. Nothing was deployed or published. See [EXECUTIVE-SUMMARY.md](EXECUTIVE-SUMMARY.md) for key conclusions and [REPORT.md](REPORT.md) for limitations.
