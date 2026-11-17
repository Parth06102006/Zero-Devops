#!/usr/bin/env python3
"""Produce a measured report only after all 57 pinned worker cases finish."""
import collections, csv, datetime, hashlib, json, pathlib, statistics, subprocess

REPO=pathlib.Path(__file__).resolve().parents[2]
OUT=REPO/'docs/worker-build-fixes-2026-11-16'

def main():
    records=[json.loads(p.read_text()) for p in (OUT/'results').glob('*.json') if not p.name.endswith('.worker-image.json')]
    if len(records)!=57 or not all(r.get('complete') for r in records):raise RuntimeError('report requires 57 completed cases')
    manifest=json.loads((REPO/'docs/worker-build-campaign-50/manifest.json').read_text())
    order={c['name']:i for i,c in enumerate(manifest)}
    records.sort(key=lambda r:order[r['case']['name']])
    if len({r['case']['name'] for r in records})!=57:raise RuntimeError('duplicate cases')
    with (REPO/'docs/worker-build-campaign-50/metrics.csv').open() as f:
        qualified={row['name'] for row in csv.DictReader(f) if row['reference_qualified'].lower()=='true'}
    if {r['case']['name'] for r in records}!=qualified:raise RuntimeError('replay differs from the 57 baseline-qualified cases')
    for r in records:
        case=manifest[order[r['case']['name']]]
        if r['case']['repo']!=case['repo'] or r['case']['sha']!=case['sha'] or r['case']['path']!=case['path']:raise RuntimeError('source drift')
    baseline=json.loads((OUT/'baseline-sha256.json').read_text())
    changed=[p for p,h in baseline.items() if hashlib.sha256((REPO/p).read_bytes()).hexdigest()!=h]
    if changed:raise RuntimeError('baseline evidence changed: '+repr(changed))
    source=json.loads((OUT/'execution-source.json').read_text())
    drift=[p for p,h in source['worker_source_sha256'].items() if hashlib.sha256((REPO/p).read_bytes()).hexdigest()!=h]
    if drift:raise RuntimeError('tested product source changed: '+repr(drift))
    resumes=[json.loads(p.read_text()) for p in OUT.glob('resume-*/reconciliation.json')]
    interrupted=[{'case':name,**details} for resume in resumes if resume.get('completed') for name,details in resume.get('interrupted_cases',{}).items()]
    builds=sum(r['worker']['image_built'] for r in records)
    runtimes=sum(bool(r['worker'].get('runtime',{}).get('ready')) for r in records)
    old_builds=sum(r['baseline_worker']['state']=='success' for r in records)
    old_runtimes=sum(bool(r['baseline_worker'].get('runtime',{}).get('ready')) for r in records)
    outcomes=collections.Counter(r['classification'] for r in records)
    targeted=[]
    for category in ['missing-lockfile/install-policy','base-image-resolution','workspace-context']:
        group=[r for r in records if r['baseline_classification']==category]
        targeted.append({'baseline_category':category,'cases':len(group),'baseline_images':sum(r['baseline_worker']['state']=='success' for r in group),
                         'images':sum(r['worker']['image_built'] for r in group),'http_smokes':sum(bool(r['worker'].get('runtime',{}).get('ready')) for r in group)})
    elapsed=[r['worker']['end_to_end_seconds'] for r in records]
    cold=json.loads((OUT/'cold-pulls.json').read_text())
    summary={'recorded_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'denominator':57,
             'baseline':{'images':old_builds,'runtime_smokes':old_runtimes},
             'replay':{'images':builds,'runtime_smokes':runtimes,'worker_success_states':sum(r['worker']['state']=='success' for r in records)},
             'deployment':'not attempted','reference_rebuild':'not performed; eligibility comes from preserved baseline',
             'outcomes':dict(outcomes),'baseline_unchanged':True,
             'product_source_unchanged':True,'user_interrupted_attempts_excluded':interrupted,
             'targeted_baseline_groups':targeted,
             'cold_pulls':{'attempted':len(cold['pulls']),'passed':sum(p['exit_code']==0 for p in cold['pulls'])},
             'end_to_end_seconds':{'median':statistics.median(elapsed),'total':sum(elapsed)},
             'excluded_reference_candidates':[c['name'] for c in manifest if c['name'] not in {r['case']['name'] for r in records}]}
    (OUT/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
    with (OUT/'metrics.csv').open('w',newline='') as f:
        writer=csv.writer(f);writer.writerow(['name','repo','path','sha','baseline_classification','classification','image_built','http_smoke_ready','worker_state','end_to_end_seconds','image_layer_bytes','mock_upload_bytes'])
        for r in records:
            c,w=r['case'],r['worker']
            writer.writerow([c['name'],c['repo'],c['path'],c['sha'],r['baseline_classification'],r['classification'],w['image_built'],w.get('runtime',{}).get('ready',False),w['state'],w['end_to_end_seconds'],w.get('image_layer_bytes',''),w.get('upload_bytes',0)])
    env=json.loads((OUT/'environment.json').read_text())
    lines=[
        '# Worker build fixes — measured replay',
        '',
        f'Tested product commit: `{env["source_commit"]}` on `fix-builds`; baseline `f01968cf3c453781fe74936ee51c2e000b327107`. See `execution-source.json` for binary and template hashes, and Git history for subsequent harness/test/report commits.',
        '',
        '| Outcome | Baseline | Fixed worker replay |',
        '|---|---:|---:|',
        f'| Image created | {old_builds}/57 ({old_builds/57:.2%}) | {builds}/57 ({builds/57:.2%}) |',
        f'| Local HTTP smoke passed | {old_runtimes}/57 ({old_runtimes/57:.2%}) | {runtimes}/57 ({runtimes/57:.2%}) |',
        '| Production deployment/publication | Not attempted | Not attempted |',
        '',
        'These are measured application-case outcomes, not independent repository probabilities. Several cases share upstream repositories. Image creation is determined by inspecting the final tagged image independently of database/finalization state. HTTP readiness is tested separately. The worker still reports dynamic success through its existing publication stub; this is not counted as deployment.',
        '',
        '## Changes',
        '',
        '- Retain frozen JS installs whenever a supported lockfile exists; recognize/copy npm shrinkwrap. Refuse missing npm locks early unless an operator sets `ALLOW_UNLOCKED_NPM_INSTALL=true`. Fallback keeps `--ignore-scripts`, emits a reproducibility warning, and does not write a checkout lockfile.',
        '- Fully qualify template base references. Derive the Go image version from the greater released `go`/`toolchain` requirement; disable implicit Go toolchain switching. Select Node 20/22/24 image families from package engine ranges and explicit pins, with a Node 24 default and actual-image engine checks before installation. Other language stacks retain their previous version policy.',
        '- Separate app Dockerfile location from Buildah context. Preserve the repository for Cargo workspaces/path dependencies, select one package and binary, and copy that exact binary. Support repository-configured Go package/context selection. Reject escaping paths, symlinks, and ambiguous native targets.',
        '',
        '## Validation and method',
        '',
        '- Worker-server suite: `GOCACHE=/tmp/zd-fix-builds-gocache go test -count=1 ./...` (see `worker-suite-with-native-tests.log`). The host skips live Buildah integration when Buildah is unavailable; separate container tests exercise it.',
        '- Live toolchain/native fixtures: `TestDockerBuild_WithDockerfile` and `TestNativeBuildIntegration` in a fresh isolated store. See `native-build-integration.log` for Node engine validation in both install stages, Go explicit package/context execution, Rust standalone exact-binary execution, and Rust workspace/path-dependency execution.',
        f'- Fresh-image verification: {summary["cold_pulls"]["passed"]}/{summary["cold_pulls"]["attempted"]} qualified image pulls passed from an initially empty separate store. The worker also began with empty storage; cold-pull images were not copied into it. See `cold-pulls.json` and individual logs.',
        '- Replay: `python3 scripts/worker-build-campaign/rerun.py --baseline /home/parth_garg/zero-devops-campaign-50`. The worker shallow-fetches each recorded SHA through the unchanged immutable-clone path. All 57 baseline-qualified app/path/SHA identities match. Reference builds were not repeated; qualification is the preserved earlier reference evidence.',
        '- First 17 cases ran sequentially; the user then authorized up to four concurrent cases / 6 CPUs / 8 GiB for builders. Docker exposed only 5.6 GiB total VM RAM, so the remainder ran on two isolated 1.5 CPU / 2 GiB / 512-PID workers, reserving 1.5 GiB for host/support services. Each has its own build store, network, AMQP vhost, and mock endpoint; only the run-owned test DB/broker are shared. The 15-minute build deadline remains. See parallel-state.json and lanes/lane2.json. No production credentials, deployment/publication, global prune, or unrelated-container cleanup.',
        f'- The user paused the campaign overnight. {len(interrupted)} incomplete Rust attempts were archived and excluded from measured failures, with their queues/DB rows reconciled before controlled retries. Completed case records were preserved byte-for-byte. The rebuilt worker matches the original binary SHA-256 exactly. Counts use the first completed, non-interrupted trial per case on the final implementation; completed failures were not replaced by later successes. See latest-resume.json and its reconciliation directory. Local cache snapshot images created during recovery do not enter the application denominator.',
        '- The locally compiled worker binary and committed templates were mounted read-only into the retained test runtime image; the worker Dockerfile was not rebuilt. See VALIDATION.md, build-toolchain.txt, running-worker-sha256.txt and environment.json. Implementation commits and their scope are listed in COMMITS.md.',
        '- Install fallback is enabled for this replay. Unlocked dependency resolution may differ from baseline references and may differ between build/runtime installation stages. Version tags and warmed worker layers are not immutable or uniformly cold.',
        f'- Worker end-to-end duration: median {statistics.median(elapsed):.2f}s, sum of case durations {sum(elapsed):.2f}s (not parallel wall time). Failures can stop early, so this is not a successful-build speed comparison. Per-case timings and sampled resource data are retained.',
        '- HTTP probes run in the Buildah parent network namespace, matching the baseline harness. They establish local endpoint response only; they do not establish external container reachability, companion APIs, auth, persistent data, or production routing.',
        '',
        '## Remaining outcomes',
        '',
    ]
    for outcome,count in sorted(outcomes.items()):lines.append(f'- {outcome}: {count}')
    lines += ['', '### Targeted baseline groups', '', '| Original failure group | Cases | Baseline images | Replay images | Replay local HTTP |', '|---|---:|---:|---:|---:|']
    for group in targeted:
        lines.append(f'| {group["baseline_category"]} | {group["cases"]} | {group["baseline_images"]} | {group["images"]} | {group["http_smokes"]} |')
    diagnosis=json.loads((OUT/'failure-diagnosis.json').read_text())
    diagnosed=[name for group in diagnosis['groups'] for name in group['cases']]
    failed={r['case']['name'] for r in records if r['classification']!='build-and-runtime-pass'}
    if len(diagnosed)!=len(set(diagnosed)) or set(diagnosed)!=failed:raise RuntimeError('failure diagnosis is incomplete or duplicated')
    lines += ['', '### Verified remaining blockers', '', '| Blocker | Cases | Stage |', '|---|---:|---|']
    for group in diagnosis['groups']:
        lines.append(f'| {group["cause"]} | {group["count"]} | {group["stage"]} |')
    lines += ['', 'Case names and supporting records for each blocker are in `failure-diagnosis.json`; detailed newly exposed issues are in `NEW_FAILURES.md`. Git deadline failures do not establish application compilation/runtime incompatibility.']
    control=OUT/'controlled-timeout-repeats'
    if (control/'summary.json').exists():
        repeated=json.loads((control/'summary.json').read_text())['trials']
        summary['controlled_timeout_repeats']=repeated
        lines += ['', '### Separate serial repeats of Git timeouts', '', 'These use the unchanged worker and per-builder limits after the primary replay. They are diagnostic controls, not replacements for primary failures; the headline remains the original 57 completed trials.', '', '| Case | Repeat classification | Image | Local HTTP |', '|---|---|---|---|']
        for repeat in repeated:
            lines.append(f'| {repeat["case"]} | {repeat["classification"]} | {"yes" if repeat["image_built"] else "no"} | {"pass" if repeat["http_ready"] else "not passed"} |')
        lines += ['', 'See `controlled-timeout-repeats/` for full logs, job IDs and the primary-evidence preservation check. Different scheduling/cache/network conditions prevent attributing repeat differences solely to concurrency.']
        (OUT/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
    lines += ['', 'Missing-manifest discovery, Laravel framework selection, publication, and hardcoded loopback binding were outside these fixes. Rust loopback listeners may pass the local probe while remaining unreachable from an external container. Prisma generation, static output paths, native dependency preparation, and production startup commands can remain as later blockers. See `NEW_FAILURES.md` and individual build/runtime logs for the actual failure in each case.', '', '## Exclusions and evidence', '',
              'The four original reference-inconclusive candidates remain excluded: '+', '.join(summary['excluded_reference_candidates'])+'. No qualified cases were dropped from the denominator. Original repository evidence hashes remain unchanged (`baseline-sha256.json`). Service setup repairs before application submission are documented in `harness-incidents.md`. The interim Node-20 pilot is preserved separately in `../worker-build-fixes-node20-pilot-2026-11-16/`; it exposed Astro’s declared Node engine requirement and was superseded by this complete fresh-checkout replay. Pilot outcomes are not mixed into these 57 completed trials on the final implementation.', '',
              'Each result JSON records its source SHA, baseline classification, final image inspection, worker database state, status messages, phase events, sampled resources, and runtime response. `metrics.csv` provides the comparison without replacing the preserved baseline. `cleanup.json` records cleanup of run-owned resources only.', '',
              '## Every qualified case', '', '| App | Baseline | Replay | Image | HTTP smoke |', '|---|---|---|---|---|']
    for r in records:
        w=r['worker'];lines.append(f'| {r["case"]["name"]} | {r["baseline_classification"]} | {r["classification"]} | {"yes" if w["image_built"] else "no"} | {"pass" if w.get("runtime",{}).get("ready") else "fail" if w["image_built"] else "not run"} |')
    (OUT/'REPORT.md').write_text('\n'.join(lines)+'\n')
    print(json.dumps(summary,indent=2))

if __name__=='__main__':main()
