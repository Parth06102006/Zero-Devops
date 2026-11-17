Execution paused at the user's request on 2026-11-17.

All six active run-owned containers were stopped: zdfix-worker, zdfix2-worker, zdfix-mock, zdfix2-mock, zdfix-rabbit, zdfix-db. Containers, build stores, volumes, source changes, and evidence are retained. No deployment or publication occurred.

Product changes are committed on fix-builds through 20544b6c95b9cb1536250f2d8a7fef24560ceb04. Harness changes and evidence remain uncommitted; preserve unrelated untracked baseline evidence.

The primary campaign completed all 57 pinned cases: 39 images built, 37 local HTTP smoke passes. These are separate measurements, not deployment success. The complete worker suite, native integration checks, and eight fresh-store image pulls previously passed.

All four separately labeled timeout repeats have result entries in controlled-timeout-repeats/summary.json. They do not replace primary results. The .NET repeat reached unsupported discovery; all three Next repeats reached build failures. Review their detailed logs and the primary-preservation check before reporting diagnoses.

Remaining work when resumed: inspect the latest report.py changes and failure-diagnosis.json; verify controlled-repeat completion and evidence preservation; finish the measured report and limitations; audit source and baseline fingerprints; commit scoped harness/evidence changes; clean up only run-owned resources after evidence is captured. Do not rerun the completed primary campaign or discard existing work. Services are stopped, so any further live checks require a careful resume using retained resources and current temporary mount paths.
