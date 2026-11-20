#!/usr/bin/env python3
"""One separately labeled serial repeat of each primary Git timeout."""
import datetime, hashlib, importlib.util, json, pathlib

HERE=pathlib.Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('campaign',HERE/'rerun.py')
campaign=importlib.util.module_from_spec(spec);spec.loader.exec_module(campaign)
ROOT=campaign.ROOT

def main():
    primary=ROOT/'results'
    records=[json.loads(p.read_text()) for p in primary.glob('*.json') if not p.name.endswith('.worker-image.json')]
    if len(records)!=57 or not all(r.get('complete') for r in records):raise RuntimeError('complete the primary campaign first')
    output=ROOT/'controlled-timeout-repeats'
    output.mkdir(exist_ok=False)
    before={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in primary.glob('*') if p.is_file()}
    (output/'primary-files-before.json').write_text(json.dumps(before,indent=2))
    selected=[r for r in records if 'git checkout failed:' in r['worker'].get('database_row','') and 'context deadline exceeded' in r['worker']['database_row']]
    results=[]
    for original in sorted(selected,key=lambda r:r['case']['name']):
        name=original['case']['name'];directory=output/name;directory.mkdir()
        campaign.RESULTS=directory
        record={'case':original['case'],'reference_qualified':True,'record_type':'controlled-serial-repeat',
                'primary_deployment_id':original['worker']['deployment_id'],'primary_classification':original['classification'],
                'purpose':'unchanged worker and per-builder limits, serial execution after the primary replay; not substituted into the primary 57-case rate',
                'started_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'complete':False}
        print('REPEAT START',name,flush=True)
        try:
            campaign.worker(record['case'],record)
            record['classification']=campaign.classify(record);record['complete']=True
        except Exception as exc:
            record['harness_error']=repr(exc)
            (directory/'repeat.json').write_text(json.dumps(record,indent=2));raise
        (directory/'repeat.json').write_text(json.dumps(record,indent=2))
        results.append({'case':name,'classification':record['classification'],'image_built':record['worker']['image_built'],
                        'http_ready':record['worker'].get('runtime',{}).get('ready',False),'worker_row':record['worker']['database_row']})
        (output/'summary.json').write_text(json.dumps({'trials':results,'primary_rates_unchanged':True},indent=2))
        print('REPEAT RESULT',name,record['classification'],flush=True)
    changed=[name for name,checksum in before.items() if hashlib.sha256((primary/name).read_bytes()).hexdigest()!=checksum]
    if changed:raise RuntimeError('primary evidence changed: '+repr(changed))
    (output/'primary-preservation-check.json').write_text(json.dumps({'unchanged':True,'files_checked':len(before)},indent=2))
    print('Controlled repeats complete; primary evidence unchanged',flush=True)

if __name__=='__main__':main()
