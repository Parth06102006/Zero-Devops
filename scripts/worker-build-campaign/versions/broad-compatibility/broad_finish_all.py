#!/usr/bin/env python3
"""Run all outstanding campaign identities; keep failed and blocked attempts in evidence."""
import sys
import concurrent.futures as cf
import fcntl
import json
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parents[4] / "scripts/worker-build-campaign"))
import re
import subprocess as sp
import sys

import broad_batch as b

ROOT = b.h.EVIDENCE
HEAVY = {'netbox','mealie','actual-budget','hexpm'}
PILOTS = ['pilot-01','pilot-02','pilot-03','case-04-django-cms-setup-retry','case-05-angular-realworld','case-06-react-router-express','case-07-gin-multipart']
FILES = [Path(__file__),Path(b.__file__),Path(b.remaining.__file__),Path(b.h.__file__),Path(b.h.common.__file__),Path(b.h.previous.__file__)]


def location(case):
    return ROOT/f'case-{case["slot"]:02d}-{case["name"]}'


def load(path):
    return json.loads(path.read_text())


def one(case):
    name = case['name']; path = location(case)/'result.json'
    if path.exists():
        return {'case':name,'preserved':True,'complete':load(path)['complete']}
    log = ROOT/f'queue-{case["slot"]:02d}-{name}.log'
    print('QUEUE START',name,flush=True)
    with log.open('x') as stream:
        cmd = [sys.executable,'-B',str(FILES[1]),'--campaign-loop',name]
        p = sp.run(cmd,stdout=stream,stderr=sp.STDOUT)
    state = load(path) if path.exists() else {}
    print('QUEUE END',name,'controller_exit',p.returncode,'complete',state.get('complete',False),flush=True)
    return {'case':name,'controller_exit':p.returncode,'complete':state.get('complete',False),'log':log.name}


def guarded(case):
    try:
        return one(case)
    except Exception as err:
        print('QUEUE EXCEPTION',case['name'],type(err).__name__,str(err),flush=True)
        return {'case':case['name'],'queue_error':f'{type(err).__name__}: {err}','complete':False}


def yes(value):
    return 'pass' if value is True else 'fail' if value is False else '—'


def clean(text):
    return str(text).replace('|','/').replace('`',"'").replace('\n',' ')[:290]


SIGNALS = re.compile(r'(ERESOLVE|pg_config executable not found|Could not resolve|no framework detected|no Dockerfile|no such file or directory|unsupported|error:|error [A-Z]+\d+|ERR_PNPM|requires.*version|failed to|Killed|cannot prompt|timed out|No matching distribution)',re.I)


def diagnosis(paths):
    for path in paths:
        if not path.is_file():
            continue
        rows = [line.strip() for line in path.read_text(errors='replace').splitlines() if SIGNALS.search(line) and len(line)<900]
        if rows:
            ranked = sorted(enumerate(rows), key=lambda row: (0 if re.search(r'pg_config|ERESOLVE|Could not resolve|no framework detected|ERR_PNPM|cannot prompt|no such file',row[1],re.I) else 1,row[0]))
            return clean(ranked[0][1])
    return ''


def report(queue):
    out = ROOT/'FINAL-REPORT.md'
    dirs = [ROOT/d for d in PILOTS]+[location(c) for c in b.remaining.CASES.values()]
    entries=[]
    for d in dirs:
        p=d/'result.json'
        entries.append((d, load(p) if p.is_file() else {}))
    def count(f): return sum(f(x) for _,x in entries)
    def trial(r,side): return r.get(side) or {}
    def rt(r,side): return trial(r,side).get('runtime') or {}
    assert len(entries)==23
    completion = count(lambda r:r.get('complete') is True)
    worker_measured = count(lambda r:trial(r,'worker').get('image_built') is not None)
    support = queue.get('end_state',{})
    lines = ['# Broad worker compatibility campaign — final queue report','',
             f'Tested worker source: `{b.h.PRODUCT}`. No deployment or publication was attempted; the worker was configured to use an isolated local stub. Earlier measured worker trials were not rerun.', '',
             f'**Queue exhausted: {len(entries)}/23 identities accounted for; {completion} complete harness records and {worker_measured} measured worker image outcomes.** An unmeasured case is not a worker failure.', '',
             '| Metric | Independent reference | Unchanged worker |','|---|---:|---:|']
    for label,f in [('Image built',lambda r,s:trial(r,s).get('image_built')),('Local HTTP',lambda r,s:rt(r,s).get('ready')),('Cross-container HTTP',lambda r,s:rt(r,s).get('external_pass')),('Case-specific functional check',lambda r,s:rt(r,s).get('functional_pass'))]:
        a=[]
        for s in ['reference','worker']:
            a.append(f'{count(lambda r:f(r,s) is True)}/{count(lambda r:isinstance(f(r,s),bool))} attempted')
        lines.append(f'| {label} | {a[0]} | {a[1]} |')
    lines += ['', 'The image metric counts diagnostic reference stages where an image exists; a build-only or typecheck reference is not full application qualification. HTTP readiness is not dependency behavior. The functional row counts explicit runtime probes only; build-time diagnostics (if any) are described in each case scope. A dash means unmeasured or not applicable.', '',
              '| Case | Reference image / local / external / functional | Worker image / local / external / functional | First relevant failure or coverage limit |', '|---|---|---|---|']
    for d,r in entries:
        name=(r.get('case') or {}).get('name',d.name)
        case=r.get('case') or {}; ref=trial(r,'reference'); wk=trial(r,'worker')
        detail=[]
        if not r.get('complete'):
            detail.append('Harness: '+(r.get('harness_error') or 'no completed case record'))
        if ref.get('image_built') is not True:
            detail.append('Ref: '+(ref.get('blocked') or diagnosis([d/'reference-build.log']) or 'no image result'))
        elif ref.get('measurement_error') or rt(r,'reference').get('functional_pass') is False:
            detail.append('Ref: '+(ref.get('measurement_error') or diagnosis([d/'reference-fixture.log',d/'reference-runtime.log']) or 'functional check did not pass'))
        if wk.get('image_built') is False:
            detail.append('Worker: '+(diagnosis([d/(name+'.worker.log'), d/'final-worker.log']) or (wk.get('database_row',{}).get('error_message') if isinstance(wk.get('database_row'),dict) else None) or 'no image'))
        elif rt(r,'worker').get('functional_pass') is False:
            detail.append('Worker: runtime functional check did not pass')
        if case.get('reference_scope'):
            detail.append('Scope: '+case['reference_scope'])
        def cells(side):
            t=rt(r,side)
            return ' / '.join(yes(x) for x in (trial(r,side).get('image_built'),t.get('ready'),t.get('external_pass'),t.get('functional_pass')))
        link=f'[{name}]({d.name}/result.json)' if r else name
        lines.append(f'| {link} | {cells("reference")} | {cells("worker")} | '+('; '.join(clean(s) for s in detail) or 'See retained checks.')+' |')
    lines += ['', '## Interpretation', '',
              '- Shared installation failures are not automatically worker-specific. An inconclusive reference does not erase a directly observed worker recipe or discovery mismatch.',
              '- The FastAPI reference-only repeats and the initial Django setup error are retained as separate attempts. They are not additional cases. See [interim evidence and repeat links](PROGRESS.md).',
              '- A reduced-memory parallel lane and a serial heavyweight lane are not comparable performance benchmarks. Unlocked resolutions are not reproducible.',
              '- The Drizzle build-only check does not qualify production Next/Stripe setup, Drizzle migrations or CRUD. External payment, cloud and production credentials were not supplied.',
              '- Runtime scopes and raw logs are linked above. No rate here establishes an updated outcome for the earlier cohorts.', '',
              f"Campaign containers still running in final check: `{clean(support.get('active_campaign_containers',[]))}`. No global pruning was performed.",
              '', 'Queue evidence: [final controller state](queue-state.json).']
    with out.open('x') as f: f.write('\n'.join(lines)+'\n')
    print('FINAL REPORT',out,flush=True)


def main():
    gate = Path.home()/'.zero-devops-broad-campaign.lock'
    with gate.open('a') as lock:
        fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
        statefile=ROOT/'queue-state.json'
        if statefile.exists() or (ROOT/'FINAL-REPORT.md').exists():
            raise RuntimeError('existing queue evidence must be reconciled before restarting')
        if b.h.docker('ps','--filter','label='+b.h.LABEL,'--format','{{.Names}}'):
            raise RuntimeError('another broad run is active')
        b.h.cmd('git','-C',str(b.h.ROOT),'diff','--quiet',b.h.PRODUCT,'--','worker-server')
        assert all(load(ROOT/d/'result.json')['complete'] for d in PILOTS)
        all_cases=list(b.remaining.CASES.values())
        assert len(all_cases)==16 and {c['slot'] for c in all_cases}==set(range(8,24))
        all_rows=[load(ROOT/d/'result.json')['case'] for d in PILOTS]+all_cases
        assert len({(c['repo'].lower(),c['path']) for c in all_rows})==23
        design=b.h.ROOT/'docs/superpowers/specs/2026-11-19-worker-compatibility-campaign-design.md'
        frozen=set((r.lower(),s,p) for r,s,p in re.findall(r'https://github.com/([^/]+/[^/]+)/tree/([0-9a-f]{40})[^)]*\) \| `([^`]+)`',design.read_text().split('**Deferred')[0]))
        assert frozen=={(c['repo'].lower(),c['sha'],c['path']) for c in all_rows}
        state={'started_at':b.h.stamp(),'product':b.h.PRODUCT,'design_sha256':b.h.sha(design),
               'harness_sha256':{p.name:b.h.sha(p) for p in FILES},'planned':all_cases,'outcomes':[]}
        def save(): b.h.common.write_json(statefile,state)
        save()
        with cf.ThreadPoolExecutor(max_workers=2) as pool:
            futures=[pool.submit(guarded,c) for c in all_cases if c['name'] not in HEAVY]
            for f in cf.as_completed(futures):
                state['outcomes'].append(f.result()); save()
        for case in all_cases:
            if case['name'] in HEAVY:
                state['outcomes'].append(guarded(case)); save()
        state['finished_at']=b.h.stamp()
        try:
            state['end_state']={'active_campaign_containers':b.h.docker('ps','--filter','label='+b.h.LABEL,'--format','{{.Names}}').splitlines()}
        except Exception as err:
            state['end_state']={'verification_error':str(err)}
        save()
        report(state)


if __name__=='__main__':
    main()
