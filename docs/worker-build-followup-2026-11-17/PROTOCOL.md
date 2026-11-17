# Follow-up build and runtime campaign

Scope authorized on 2026-11-17: optional Next public directory; framework ownership and output selection; production startup and runtime asset packaging; separate Git reliability improvements. Explicit generation steps, new dependency formats, and discovery expansion are excluded. No deployment or publication.

The previous complete campaign remains 39/57 image builds and 37/57 local HTTP passes, on product commit 20544b6. Its original result files and controlled repeats must remain unchanged. Its measured report was generated before follow-up product changes.

Follow-up cohorts must be reported independently:

1. All twenty previous image/runtime failures, at identical repository/app/SHA identities. Eligibility is the preserved historical reference evidence. Do not replace previous outcomes with new results.
2. Twelve new public-source app identities frozen in fresh-manifest.json, rotating Astro integrations (Preact, Solid, MDX, Markdoc), Fiber examples, Axum/Tower HTTP, Spring/Thymeleaf and Next/Sass. Run independent declared reference compilation and HTTP probes before worker trials. Report reference-inconclusive cases explicitly and retain worker results even when reference qualification fails. Do not silently drop or replace difficult cases.

Alternate failure and fresh cases in the shared dispatch order while both remain available; each free lane takes the next case. Use two isolated lanes, each limited to 1.5 CPUs, 2 GiB RAM and 512 PIDs, with its own build store, network, broker vhost and mock endpoint. Reuse only the run-owned test DB/broker. Docker exposes about 5.6 GiB total RAM, so the approved maximum of four simultaneous builders cannot be used with these per-builder limits. Record initial empty image storage and cold-pull the required images, including the new Go 1.27.0 image, before interpreting toolchain compatibility.

Freeze one tested product commit, binary hash and template/source hashes before either cohort starts. Capture commands, build logs, image inspection, dependency/toolchain evidence, phase timings, resource samples, local HTTP and separate cross-container HTTP evidence. A local HTTP pass is not external reachability or deployment success. Loopback bindings remain unchanged. Shared upstream repositories and unlocked dependency resolution limit statistical interpretation.

Do not infer a full 57-case improvement from a failure-only rerun. A projected combination with prior successes, if shown, must be labeled unverified because those successful cases have not been retested on the follow-up source. Fresh-cohort success rates are a separate compatibility measurement.

Preserve previous containers and stores. Clean up only resources bearing the follow-up ownership label after all evidence is captured. Never prune globally.
