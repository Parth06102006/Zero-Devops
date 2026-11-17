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
