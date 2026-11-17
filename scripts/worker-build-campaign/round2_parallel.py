#!/usr/bin/env python3
"""Alternate frozen failure and fresh-source cases across two isolated lanes."""
import argparse, concurrent.futures, datetime, hashlib, json, pathlib, queue, subprocess, threading, time

REPO=pathlib.Path(__file__).absolute().parents[2]
OUT=REPO/'docs/worker-build-followup-2026-11-17'
SCRIPT=REPO/'scripts/worker-build-campaign/round2_run.py'

def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def fingerprints(paths):
 return {str(path.relative_to(REPO)):hashlib.sha256(path.read_bytes()).hexdigest() for path in paths if path.is_file()}
def source_paths():return list((REPO/'worker-server').rglob('*.go'))+list((REPO/'worker-server/templates').rglob('*.tmpl'))+[REPO/'worker-server/go.mod',REPO/'worker-server/go.sum']
def verify(snapshot):
 drift=[name for name,digest in snapshot.items() if not (REPO/name).is_file() or hashlib.sha256((REPO/name).read_bytes()).hexdigest()!=digest]
 if drift:raise RuntimeError('frozen evidence/source changed: '+repr(drift))
def write(path,data):
 temp=path.with_name(path.name+'.'+str(threading.get_ident())+'.tmp');temp.write_text(json.dumps(data,indent=2)+'\n');temp.replace(path)

def main():
 p=argparse.ArgumentParser();p.add_argument('--freeze',action='store_true');a=p.parse_args()
 OUT.mkdir(parents=True,exist_ok=True)
 frozen=OUT/'frozen-campaign.json'
 if a.freeze:
  if frozen.exists():raise RuntimeError('freeze exists; do not overwrite')
  prior=[]
  for name in ['worker-build-campaign-50','worker-build-fixes-2026-11-16','worker-build-fixes-node20-pilot-2026-11-16']:
   prior+=list((REPO/'docs'/name).rglob('*'))
  manifests=[OUT/'failures-manifest.json',OUT/'fresh-manifest.json']
  record={'at':now(),'product_commit':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),'product_source_sha256':fingerprints(source_paths()),'prior_evidence_sha256':fingerprints(prior),'manifests_sha256':fingerprints(manifests),'binary_sha256':hashlib.sha256(pathlib.Path('/tmp/zd-round2-worker').read_bytes()).hexdigest(),'publication':'not attempted'}
  write(frozen,record);print('frozen',flush=True);return
 record=json.loads(frozen.read_text())
 for key in ['product_source_sha256','prior_evidence_sha256','manifests_sha256']:verify(record[key])
 environment=json.loads((OUT/'environment.json').read_text())
 for lane in environment['lanes']:
  if lane['binary_sha256']!=record['binary_sha256']:raise RuntimeError('worker binary mismatch')
  actual=subprocess.check_output(['docker','exec',lane['worker'],'sha256sum','/e2e/worker-engine'],text=True).split()[0]
  if actual!=record['binary_sha256']:raise RuntimeError('mounted worker binary changed')
 manifests={batch:json.loads((OUT/(batch+'-manifest.json')).read_text()) for batch in ['failures','fresh']}
 jobs=queue.Queue()
 # Alternate cohorts. Within each cohort preserve predeclared manifest order.
 for i in range(max(map(len,manifests.values()))):
  for batch in ['failures','fresh']:
   if i>=len(manifests[batch]):continue
   case=manifests[batch][i];path=OUT/batch/'results'/(case['name']+'.json')
   if path.exists():
    if json.loads(path.read_text()).get('complete'):continue
    raise RuntimeError('incomplete case exists: '+case['name'])
   jobs.put((batch,case['name']))
 harness=fingerprints([REPO/'scripts/worker-build-campaign'/name for name in ['round2_environment.py','round2_run.py','round2_parallel.py','round2_validate.py','rerun.py','mock_services.py']])
 state={'started_at':now(),'finished_at':None,'workers':2,'scheduling':'alternate failure/fresh cohorts; reference before worker for fresh; no same-lane concurrent jobs','harness_sha256':harness,'in_flight':{},'completed':[],'errors':[]}
 lock=threading.Lock();stop=threading.Event()
 def save():write(OUT/'parallel-state.json',state)
 def loop(lane):
  while not stop.is_set():
   try:batch,name=jobs.get_nowait()
   except queue.Empty:return
   verify(record['product_source_sha256']);verify(record['manifests_sha256']);verify(harness)
   with lock:state['in_flight'][str(lane)]={'batch':batch,'case':name};save();print('START',lane,batch,name,flush=True)
   argv=['python3',str(SCRIPT),'--batch',batch,'--lane',str(lane),'--cases',name]
   if batch=='fresh':argv.append('--reference')
   with (OUT/('lane'+str(lane)+'-controller.log')).open('a') as log:
    result=subprocess.run(argv,stdout=log,stderr=subprocess.STDOUT)
   with lock:
    state['in_flight'].pop(str(lane),None)
    if result.returncode:
     state['errors'].append({'lane':lane,'batch':batch,'case':name,'exit_code':result.returncode});stop.set();save();print('ERROR',lane,batch,name,flush=True);return
    state['completed'].append({'lane':lane,'batch':batch,'case':name,'at':now()});save();print('DONE',lane,batch,name,flush=True)
 with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
  futures=[pool.submit(loop,lane) for lane in [1,2]]
  for future in futures:future.result()
 state['finished_at']=now();save()
 for key in ['product_source_sha256','prior_evidence_sha256','manifests_sha256']:verify(record[key])
 verify(harness)
 write(OUT/'preservation-check.json',{'at':now(),'product_source_unchanged':True,'prior_evidence_unchanged':True,'manifests_unchanged':True,'prior_files':len(record['prior_evidence_sha256'])})
 if state['errors']:raise RuntimeError('dispatch stopped; inspect retained evidence')
 print('follow-up campaign complete',flush=True)

if __name__=='__main__':main()
