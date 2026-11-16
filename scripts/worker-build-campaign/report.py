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
    for r in records:
        case=manifest[order[r['case']['name']]]
        if r['case']['sha']!=case['sha'] or r['case']['path']!=case['path']:raise RuntimeError('source drift')
    baseline=json.loads((OUT/'baseline-sha256.json').read_text())
    changed=[p for p,h in baseline.items() if hashlib.sha256((REPO/p).read_bytes()).hexdigest()!=h]
    if changed:raise RuntimeError('baseline evidence changed: '+repr(changed))
    builds=sum(r['worker']['image_built'] for r in records)
    runtimes=sum(bool(r['worker'].get('runtime',{}).get('ready')) for r in records)
    old_builds=sum(r['baseline_worker']['state']=='success' for r in records)
    old_runtimes=sum(bool(r['baseline_worker'].get('runtime',{}).get('ready')) for r in records)
    outcomes=collections.Counter(r['classification'] for r in records)
    elapsed=[r['worker']['end_to_end_seconds'] for r in records]
    cold=json.loads((OUT/'cold-pulls.json').read_text())
    summary={'recorded_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'denominator':57,
             'baseline':{'images':old_builds,'runtime_smokes':old_runtimes},
             'replay':{'images':builds,'runtime_smokes':runtimes,'worker_success_states':sum(r['worker']['state']=='success' for r in records)},
             'deployment':'not attempted','reference_rebuild':'not performed; eligibility comes from preserved baseline',
             'outcomes':dict(outcomes),'baseline_unchanged':True,
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
        '- Sequential jobs, one attempt per case, 1.5 CPUs / 2 GiB / 512 PIDs per worker builder, 15-minute worker build deadline. Separate labeled broker/database/mock endpoints and named build volumes. No production credentials, deployments, publication, global prune, or unrelated-container cleanup.',
        '- Install fallback is enabled for this replay. Unlocked dependency resolution may differ from baseline references and may differ between build/runtime installation stages. Version tags and warmed worker layers are not immutable or uniformly cold.',
        f'- Worker end-to-end duration: median {statistics.median(elapsed):.2f}s, sum {sum(elapsed):.2f}s. Failures can stop early, so this is not a successful-build speed comparison. Per-case timings and sampled resource data are retained.',
        '- HTTP probes run in the Buildah parent network namespace, matching the baseline harness. They establish local endpoint response only; they do not establish external container reachability, companion APIs, auth, persistent data, or production routing.',
        '',
        '## Remaining outcomes',
        '',
    ]
    for outcome,count in sorted(outcomes.items()):lines.append(f'- {outcome}: {count}')
    lines += ['', 'Missing-manifest discovery, Laravel framework selection, publication, and hardcoded loopback binding were outside these fixes. Rust loopback listeners may pass the local probe while remaining unreachable from an external container. Prisma generation, static output paths, native dependency preparation, and production startup commands can remain as later blockers. See `NEW_FAILURES.md` and individual build/runtime logs for the actual failure in each case.', '', '## Exclusions and evidence', '',
              'The four original reference-inconclusive candidates remain excluded: '+', '.join(summary['excluded_reference_candidates'])+'. No qualified cases were dropped from the denominator. Original repository evidence hashes remain unchanged (`baseline-sha256.json`). Service setup repairs before application submission are documented in `harness-incidents.md`. The interim Node-20 pilot is preserved separately in `../worker-build-fixes-node20-pilot-2026-11-16/`; it exposed Astro’s declared Node engine requirement and was superseded by this complete fresh-source replay. Pilot outcomes are not mixed into these 57 first attempts on the final implementation.', '',
              'Each result JSON records its source SHA, baseline classification, final image inspection, worker database state, status messages, phase events, sampled resources, and runtime response. `metrics.csv` provides the comparison without replacing the preserved baseline. `cleanup.json` records cleanup of run-owned resources only.', '',
              '## Every qualified case', '', '| App | Baseline | Replay | Image | HTTP smoke |', '|---|---|---|---|---|']
    for r in records:
        w=r['worker'];lines.append(f'| {r["case"]["name"]} | {r["baseline_classification"]} | {r["classification"]} | {"yes" if w["image_built"] else "no"} | {"pass" if w.get("runtime",{}).get("ready") else "fail" if w["image_built"] else "not run"} |')
    (OUT/'REPORT.md').write_text('\n'.join(lines)+'\n')
    print(json.dumps(summary,indent=2))

if __name__=='__main__':main()
