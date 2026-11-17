# Fixed-worker replay protocol

Baseline: `../worker-build-campaign-50/`, with complete original per-case data
at `/home/parth_garg/zero-devops-campaign-50/`. Preserve both. The new evidence
directory is separate, and `baseline-sha256.json` verifies the repository copy.

The denominator remains the same 57 cases that passed the original reference
build and runtime checks. The four reference-inconclusive candidates stay
excluded. Reference builds are not repeated in this replay. Each worker job
restores the recorded upstream SHA through `cloneRepo` into a fresh checkout;
no generated reference lockfiles or patched upstream source enter the worker.

Use only `zdfix-*` containers, network, and volumes labeled
`zero-devops-build-fixes=20261116`. Each worker builder has 1.5 CPUs, 2 GiB,
512 PIDs, and a 15-minute build deadline. One application build runs at a time.
Keep 20 GiB free in the evidence filesystem; stop if build storage exceeds
25 GiB. Logs and results survive cleanup. Never run a global prune.

Worker policy for this replay: `ALLOW_UNLOCKED_NPM_INSTALL=true`. Supported
lockfiles retain frozen installs. Missing-lock npm resolution is explicitly
non-reproducible; lifecycle install scripts remain disabled. Versions resolved
today can differ from the baseline references and between image stages.

Before the campaign, verify affected image pulls in a distinct initially empty
Buildah store. Do not copy those images into the worker store, which also begins
empty. Run native Go target and Rust workspace/binary fixtures in another
isolated store. Native fixture execution is CLI validation, separate from the
application campaign's HTTP checks and denominator.

For each case, inspect the final tagged image independently of the worker's
database state. If an image exists, start its configured command and probe the
same local HTTP endpoint/status as the original harness. Record response body,
startup duration, and eight warm requests. The probe shares the parent Buildah
network namespace; it does not establish external reachability. In particular,
loopback-only apps can pass this local smoke check.

Fake clone credentials and a fake HTTPS upload endpoint replace production
credentials and storage. No production publication or deployment occurs.
Dynamic finalization remains the existing worker stub. Frontend homepages do
not validate companion APIs, auth, persistent data, or routing.

Primary results are first attempts. Keep incomplete records and failures;
inspect them before resuming. Never replace a first failure with a later success
without labeling a separate repeat. Generate the measured report only when all
57 case identities and SHAs are accounted for. Document any newly exposed
failures and all exclusions. The report must distinguish image creation,
local runtime smoke, worker status, fake upload, and production deployment.

Commands used (from the repository root unless stated):

```text
git fetch origin
git merge --ff-only origin/main          # while on main
git switch -c fix-builds

# From worker-server/:
GOCACHE=/tmp/zd-fix-builds-gocache go test -count=1 ./...
CGO_ENABLED=0 GOCACHE=/tmp/zd-fix-builds-gocache go build -buildvcs=false -trimpath -o /tmp/zd-fix-builds-worker ./cmd/worker-server
GOCACHE=/tmp/zd-fix-builds-gocache go test -c -o /tmp/zd-fix-builds-final-services/deployments.test ./internal/deployments

python3 scripts/worker-build-campaign/environment.py setup
python3 scripts/worker-build-campaign/environment.py cold-pulls
# Startup recovery and endpoint repair are documented in harness-incidents.md.
# Native test container uses RUN_NATIVE_BUILD_TESTS=true and runs:
deployments.test -test.v -test.run 'TestNativeBuildIntegration|TestDockerBuild_WithDockerfile'

python3 scripts/worker-build-campaign/rerun.py --baseline /home/parth_garg/zero-devops-campaign-50
python3 scripts/worker-build-campaign/report.py
python3 scripts/worker-build-campaign/environment.py stop
```

## User-authorized parallel amendment

After 17 completed cases on the same product commit, the user authorized up to four concurrent isolated workers with aggregate builder limits of 6 CPUs / 8 GiB, plus supporting services. Docker reports 6061568000 bytes total VM RAM, so the controller uses two 1.5 CPU / 2 GiB workers and reserves 1.5 GiB for host/supporting services. This supersedes the initial sequential rule for the remaining cases. Workers have separate build stores, private networks, broker vhosts, and mock endpoints; only the run-owned test DB and broker are shared. Longest builds are scheduled first. Cache state and concurrency differ from the baseline, so timings are descriptive rather than a performance claim. See parallel-state.json, parallel-transition.json, and lanes/lane2.json.

Continuation command:

```text
python3 scripts/worker-build-campaign/environment.py add-lane --lane 2
python3 scripts/worker-build-campaign/parallel.py --baseline /home/parth_garg/zero-devops-campaign-50 --workers 4
```
