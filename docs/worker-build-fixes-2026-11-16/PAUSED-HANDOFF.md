# Paused worker build fixes handoff

Paused explicitly by the user at 2026-11-16T21:14:06.928909+00:00. Do not resume without a user request.

## Current state

- Branch: `fix-builds`; HEAD `20544b6c95b9cb1536250f2d8a7fef24560ceb04`. Main was fetched and safely checked for fast-forward before branch creation; it was already at baseline `f01968cf3c453781fe74936ee51c2e000b327107`.
- Product fixes committed in `ae784de`, `7d853dc`, and `20544b6`. Final tested product source is `20544b6`: opt-in unlocked npm installs with restrictions/warnings; qualified images plus Go and Node version selection; separate native build contexts and explicit Go/Cargo targets with containment checks.
- Full worker suite, all eight fresh-store image pulls, and live Node/Go/Rust fixtures passed. Evidence is in this directory; `execution-source.json` records the source and binary hashes.
- Final campaign: **17/57 complete**, **15 images**, **15 local HTTP smoke passes**. Two Svelte variants fail because detection selects SvelteKit and copies `build/`, while compilation produces `dist/`. This is outside the three targeted fixes. Astro minimal, basics and blog now pass after the in-scope Node engine fix.
- No production deployment/publication, push, or PR. Preserve all original baseline evidence and the separate `../worker-build-fixes-node20-pilot-2026-11-16/` archive.

## Execution is stopped

All containers labeled `zero-devops-build-fixes=20261116` are stopped. Containers, named stores, broker/database state and intermediate layers are retained, not removed. See `paused-containers.txt`, `retained-volumes.txt`, `pause-state.json`.

Campaign controller/case harness PIDs were no longer present at shutdown; the prior tool session was missing. A scoped process audit found no remaining campaign Python processes. Both active Docker workers and all run-owned supporting services were then stopped explicitly. Unrelated services were not stopped.

Interrupted cases: `{'1': 'actix-hello-world', '2': 'actix-state'}`. Their partial worker logs are `paused-lane1-worker.log` and `paused-lane2-worker.log`; DB rows are in `paused-worker-rows.csv`. They were interrupted by the user's pause, not measured application failures. Their final per-case results are not yet complete.

**Do not blindly restart workers.** The durable RabbitMQ broker may have requeued those unacknowledged jobs, while their DB rows remain `building`. First start only the owned broker/database/mock services, inspect and reconcile the interrupted jobs and Buildah intermediate state, and preserve the paused attempt evidence before choosing controlled retries. Do not purge unrelated queues, containers or volumes. Do not overwrite any completed case JSON.

## Parallel execution authorization and capacity

The latest user authorized up to four concurrent cases, isolated workers/stores, and up to 6 CPUs / 8 GiB for builders plus supporting services. Docker actually exposes 12 CPUs and only 6061568000 bytes total VM RAM. The new controller therefore uses **two** 1.5 CPU / 2 GiB builders, reserving 1.5 GiB for host/supporting services; it must not overcommit unrelated workloads to reach four.

Lane 1 uses `zdfix-worker` / `zdfix-worker-storage`, default broker vhost `/`. Lane 2 uses `zdfix2-worker` / `zdfix2-worker-storage`, private `zdfix2-network`, vhost `lane2`, and `zdfix2-mock`. Only the run-owned `zdfix-db` and `zdfix-rabbit` are shared. See `lanes/lane2.json` for empty initial lane-2 image storage, limits and matching binary hash. Private networks keep each runtime probe isolated.

The original sequential harness stopped at a completed-case boundary after Astro blog using a clearly labeled control marker, now preserved in `controls/sequential-stop-marker.json`. No measured result was discarded. The 17 completed records predate the parallel continuation and remain intact.

## Uncommitted work and next steps

Parallel harness changes are **uncommitted** in `scripts/worker-build-campaign/environment.py`, `rerun.py`, `report.py`, and new `parallel.py`. They compile with `py_compile`; both workers received distinct Rust jobs before the pause. Worker Go source/templates remain frozen at `20544b6`. Keep these changes and all untracked evidence; do not reset, clean, or overwrite them.

On an explicit resume: inspect branch/status, retained containers/queues/DB rows and incomplete jobs first; reconcile pause interruptions safely; continue the remaining pinned cases without rerunning completed ones or mixing the Node-20 pilot into final rates. The controller command is:

```text
python3 scripts/worker-build-campaign/parallel.py --baseline /home/parth_garg/zero-devops-campaign-50 --workers 4
```

Its physical-memory check currently selects two workers. Existing final results are skipped. The first 17 cases were sequential; the remaining schedule prioritizes long Rust builds. Update the report's concurrency/caching limitations accordingly.

After all 57 qualified cases finish, inspect failures and exclusions, run `report.py`, verify baseline hashes and frozen product hashes, commit the new harness/evidence/report, and clean up **only** recorded run-owned resources. The final measured report has not been generated because the full campaign is incomplete. Keep image creation, local HTTP smoke, worker status, fake upload and real deployment distinct. Do not deploy or publish.
