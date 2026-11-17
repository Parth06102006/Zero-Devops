# Independent campaign evidence review

Read-only audit completed by the evidence-auditor subagent on 2026-11-17, using model `agentrouter-openai/gpt-6.1-sol`. The initial attempted audit could not launch because its configured `openai-codex/gpt-5.5` model was unavailable; it performed no work. The explicit-model retry completed successfully. The auditor did not change source, campaign processes, Docker resources, or evidence.

## Findings accepted

- Exact identities matched all twenty historical failures and twelve fresh cases.
- The auditor independently matched 63 product-file hashes, 460 prior-evidence hashes and both manifests, and recounted the prior completed 57-case run as 39 images and 37 local HTTP passes.
- The original campaign still needed completion at review time; provisional counts were not final rates.
- Image creation, worker terminal state, local HTTP, external-container HTTP, and deployment are distinct. Loopback-only Actix/Axum endpoints cannot be called externally despite successful local smoke checks.
- Reference and worker probes must both prove port closure before and after execution. Final report acceptance now enforces this for every image trial.
- Eight warm requests are telemetry, not the frozen readiness criterion. A separate final audit checks their recorded statuses without redefining results.
- Fresh source metadata and dependency-requirements JSON were outside the active controller freeze. The additional final audit verifies exact inventories, SHA256/Git blob hashes, executable/symlink modes, pinned upstream tree linkage and regenerated dependency declarations, and identifies this as a supplemental check rather than retroactive freezing.
- Original blocker counts by requested category are 2 Next, 5 detection/output, 2 startup/assets, 7 excluded dependency/discovery, 4 checkout. The expanded Next scope includes three controlled timeout repeats; checkout overlaps that expanded scope. The report presents both groupings without double-counting.
- Initially empty stores became lane-specifically warmed. References precede worker trials in the same lane/store. Exact intermediate-layer/dependency-cache reuse is not established just by storage sharing; timing is not a controlled cold-build benchmark.
- Declared dependencies, observed reference dependencies, and worker builder dependencies must not be conflated. Static final-image npm inventories cannot recover app builder packages.
- Qualification is build plus declared endpoint/status smoke compatibility, not full feature behavior, browser CORS, cache/auth correctness, complete runtime asset coverage, or production readiness.
- References have a 1,200-second budget and builder-style runtime; worker build/save has a 15-minute budget and its chosen runtime recipe.
- Intentional runtime-probe shutdown can produce trailing killed/exit-status messages after a successful readiness check.
- No worker Dockerfile rebuild, production publication, or deployment was performed.

## Final independent acceptance

The same auditor independently recounted all 32 completed records and accepted the final report with no conclusion-changing must-fix findings. It verified original and expanded category arithmetic; 25 originally completed + two original-image measurements recovered without rebuild + five serially completed cases; recovery archive hashes and image IDs in original logs; preserved canceled TypeScript state; 36 successful trial lists containing 288 warm requests with no missing/unexpected statuses; and unchanged 460/63/2 fingerprints. It independently rechecked all 622 local source entries and upstream-sampled 517 Next/Axum entries; the other upstream linkage was reviewed through the supplemental audit. It did not inspect live shutdown itself; the parent captured final ownership/log evidence and verified all campaign services stopped in cleanup.json. Its non-blocking Prisma diagnostic suggestion was applied to the final report, using the concrete TS2305 error.

## Additional event after the audit

The controller later stopped on storage/DB telemetry timeouts. Two already-built Next images were retained and measured without rebuilding; the TypeScript job's canceled broker-finalization state remains visible. Original incomplete/controller evidence is archived in `recovery-20261117/`. Five unsubmitted cases continued serially. Final reporting must require all 32 complete records, no unresolved controller errors, preservation checks, and supplemental acceptance checks.
