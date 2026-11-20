#!/usr/bin/env python3
"""Run separate failure/fresh batches with immutable evidence and isolated lanes."""
import argparse, collections, datetime, hashlib, importlib.util, json, os, pathlib, re, shlex, subprocess, time, uuid

REPO=pathlib.Path(__file__).absolute().parents[2]
OUT=REPO/'docs/worker-build-followup-2026-11-17'
PRIOR=REPO/'docs/worker-build-fixes-2026-11-16'

def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def load_campaign(lane,batch):
 spec=importlib.util.spec_from_file_location('prior_campaign',REPO/'scripts/worker-build-campaign/rerun.py')
 c=importlib.util.module_from_spec(spec);spec.loader.exec_module(c)
 c.ROOT=OUT;c.RESULTS=OUT/batch/'results';c.LANE=lane;c.PREFIX='zdround2-'+str(lane);c.WORKER=c.PREFIX+'-worker';c.MOCK=c.PREFIX+'-mock';c.VHOST='zdround2-'+str(lane)
 original_probe=c.probe_runtime
 def probe(container,name,start_command,case,label):
  c.dex(container,['rm','-f','/tmp/zdfix-probe-body'],check=True)
  result=original_probe(container,name,start_command,case,label)
  status=result.get('first_response','').split('|')[0]
  if not status.isdigit() or status=='000':result['response_sample']=''
  return result
 c.probe_runtime=probe
 c.RESULTS.mkdir(parents=True,exist_ok=True)
 if c.cmd(['docker','inspect','--format','{{index .Config.Labels "zero-devops-build-followup"}}',c.WORKER],check=True)[1]!='20261117':raise RuntimeError('refusing unowned worker')
 return c

def external_probe_hook(c):
 """Record an additional request from a different container while runtime is live."""
 original=c.dex;checks=[]
 worker_ip=c.cmd(['docker','inspect','--format','{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}',c.WORKER],check=True)[1]
 def dex(container,args,timeout=30,check=False):
  rc,out=original(container,args,timeout,check)
  if container==c.WORKER and args and args[0]=='curl' and '/tmp/zdfix-probe-body' in args and rc==0 and out.split('|')[0].isdigit() and 200<=int(out.split('|')[0])<400:
   url=args[-1];remote_url=url.replace('127.0.0.1',worker_ip)
   remote_args=['curl','--connect-timeout','2','--max-time','3','-sS','-o','/tmp/round2-external-body','-w','%{http_code}|%{time_total}|%{content_type}']
   # Preserve the probe method/body/headers supplied by each case.
   opts=args[args.index('%{http_code}|%{time_total}|%{content_type}')+1:-1]
   remote_args+=opts+[remote_url]
   erc,eout=original(c.MOCK,remote_args,8)
   checks.append({'from_container':c.MOCK,'target_worker_ip':worker_ip,'url':remote_url,'exit_code':erc,'response':eout,'reachable_expected_class':erc==0 and eout.split('|')[0].isdigit() and 200<=int(eout.split('|')[0])<400})
  return rc,out
 c.dex=dex
 return checks

def worker(c,case,record):
 ident=str(uuid.uuid4());event=str(uuid.uuid4());corr='zdround2-'+ident
 req={'version':1,'event_id':event,'deployment_id':ident,'project_id':'isolated-zdround2','installation_id':1,'repository_id':1,'clone_url':'https://github.com/'+case['repo']+'.git','commit_sha':case['sha'],'requested_ref':'HEAD','trigger':'manual','generation':1,'retry_count':0,'correlation_id':corr,'configuration':{'executable':{'go':'go','rust':'cargo','python':'python','java':'mvn','php':'composer','ruby':'bundle','dotnet':'dotnet'}.get(case['kind'],'npm'),'args':['run','build'] if case['kind']=='node' else [],'working_dir':case['path'],'scanner_policy_version':'v1'}}
 began=time.time();since=str(int(began)-1)
 body={'properties':{'content_type':'application/json','message_id':event,'correlation_id':corr},'routing_key':'deploy.jobs','payload':json.dumps(req),'payload_encoding':'string'}
 if not c.api('/api/exchanges/%2F/amq.default/publish',body).get('routed'):raise RuntimeError('job not routed')
 row='';state='pending';samples=[];nxt=0
 while time.time()-began<1700:
  _,row=c.dex(c.DB,['psql','-U','e2e','-d','e2e','-A','-t','-F','|','-c',f"SELECT status, COALESCE(error_message,''), COALESCE(output_url,'') FROM deployments WHERE id='{ident}';"],20)
  state=row.split('|')[0] if row else 'pending'
  if time.time()>=nxt:c.guard();samples.append({'seconds':round(time.time()-began,2),**c.resource(c.WORKER)});nxt=time.time()+20
  if state in ['success','canceled']:break
  if c.cmd(['docker','inspect','--format','{{.State.Running}}',c.WORKER])[1]!='true':raise RuntimeError('worker stopped')
  time.sleep(2)
 if state not in ['success','canceled']:raise RuntimeError('worker did not terminate; stop dispatch and preserve evidence')
 _,logs=c.cmd(['docker','logs','--since',since,c.WORKER],60);_,mock=c.cmd(['docker','logs','--since',since,c.MOCK],60)
 (c.RESULTS/(case['name']+'.worker.log')).write_text(logs+'\n');(c.RESULTS/(case['name']+'.mock.log')).write_text(mock+'\n')
 w={'lane':c.LANE,'worker_container':c.WORKER,'broker_vhost':c.VHOST,'commit_sha':case['sha'],'deployment_id':ident,'state':state,'database_row':row,'end_to_end_seconds':round(time.time()-began,3),'resource_samples':samples,'configuration_sent':req['configuration'],'deployment':'not attempted'}
 record['worker']=w
 match=re.search(r'"framework": "([^"]+)"',logs);w['framework']=match[1] if match else None
 w['git_phase_diagnostics']=[line for line in logs.splitlines() if 'ZERODEVOPS_GIT' in line]
 uploads=re.findall(r'TEST_UPLOAD path=/images/'+ident+r'\.tar[^\n]*bytes=(\d+)',mock);w['mock_upload_bytes']=int(uploads[-1]) if uploads else 0
 try:w['status_messages']=c.api('/api/queues/%2F/deploy.status/get',{'count':100,'ackmode':'ack_requeue_false','encoding':'auto','truncate':50000})
 except Exception as exc:w['status_queue_error']=repr(exc)
 tag='localhost/deploy-'+ident+':latest';rc,info=c.dex(c.WORKER,['buildah','inspect','--type','image',tag],60)
 w['image_built']=rc==0
 if w['image_built']:
  details=json.loads(info);(c.RESULTS/(case['name']+'.worker-image.json')).write_text(json.dumps(details,indent=2))
  manifest=json.loads(details.get('Manifest','{}'));w['image_layer_bytes']=sum(layer.get('size',0) for layer in manifest.get('layers',[]))
  conf=details.get('OCIv1',{}).get('config') or details.get('Docker',{}).get('config') or {}
  if isinstance(details.get('Config'),str):
   try:conf=json.loads(details['Config']).get('config',{})
   except (ValueError,TypeError):pass
  command=shlex.join((conf.get('Entrypoint') or [])+(conf.get('Cmd') or []));name='runtime-'+case['name']
  c.dex(c.WORKER,['buildah','from','--name',name,tag],60,True)
  for k,v in case.get('env',{}).items():c.dex(c.WORKER,['buildah','config','--env',k+'='+v,name],check=True)
  if case['kind']=='node':
   _,versions=c.dex(c.WORKER,['buildah','run',name,'--','sh','-c','node --version; npm --version; npm ls -g --depth=0; npm ls --depth=0'],60)
   (c.RESULTS/(case['name']+'.worker-dependencies.log')).write_text(versions+'\n')
  runtime_case=dict(case)
  static_ports={'vite':4173,'vue':8080,'svelte':5000,'solid':3000,'astro':4321,'vitepress':4173,'sveltekit-static':5000}
  if w['framework'] in static_ports:runtime_case['port']=static_ports[w['framework']]
  if w['framework'] in ['react','node','nextjs','nestjs']:runtime_case['port']=3000
  checks=external_probe_hook(c)
  try:w['runtime']=c.probe_runtime(c.WORKER,name,command,runtime_case,'worker');w['external_container_http']=checks
  finally:c.dex(c.WORKER,['buildah','rm',name],60)
  c.dex(c.WORKER,['buildah','rmi',tag],120)
 c.dex(c.WORKER,['buildah','rm','--all'],60);c.dex(c.WORKER,['buildah','rmi','--prune'],120)

def reference(c,case):
 name=case['name'];source=pathlib.Path(case['source_dir'])
 if not source.is_dir():raise RuntimeError('fresh pinned source directory missing: '+str(source))
 evidence_path=OUT/'fresh-source-evidence'/(case['repo'].replace('/','--')+'.json')
 evidence=json.loads(evidence_path.read_text())
 if evidence['repo']!=case['repo'] or evidence['sha']!=case['sha']:raise RuntimeError('fresh source SHA identity mismatch')
 for entry in evidence['files']:
  path=source/entry['path']
  data=os.readlink(path).encode() if entry['mode']=='120000' else path.read_bytes()
  if hashlib.sha256(data).hexdigest()!=entry['sha256']:raise RuntimeError('fresh source changed: '+entry['path'])
 destination='/tmp/round2-reference/'+name
 c.dex(c.WORKER,['mkdir','-p',destination],check=True)
 c.cmd(['docker','cp',str(source)+'/.',c.WORKER+':'+destination],timeout=120,check=True)
 # Cargo reference commands address the workspace from its repository root.
 # Keep the frozen app identity intact; this is the reference build context.
 context='.' if case.get('reference_workdir')=='.' else case.get('context','.')
 if context.startswith('/') or '..' in pathlib.PurePosixPath(context).parts:raise RuntimeError('unsafe reference context')
 # Sources are verified immutable Git blobs. Recipe/probe is independent of worker detection.
 reference_workdir=case.get('reference_workdir',case['path'])
 workdir='/app/'+str(pathlib.PurePosixPath(reference_workdir).relative_to(pathlib.PurePosixPath(context))) if context!='.' else '/app/'+reference_workdir
 recipe='FROM '+c.base(case)+'\nWORKDIR /app\nCOPY . .\nWORKDIR '+workdir+'\nRUN '+case['reference_build']+'\nCMD '+json.dumps(['sh','-c',case['reference_start']])+'\n'
 dockerfile='/tmp/round2-reference-'+name+'.Dockerfile'
 p=subprocess.run(['docker','exec','-i',c.WORKER,'sh','-c','cat > "$1"','write-reference',dockerfile],input=recipe,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
 if p.returncode:raise RuntimeError(p.stdout)
 (c.RESULTS/(name+'.reference.Dockerfile')).write_text(recipe)
 tag='localhost/zdround2-reference-'+name+':test';began=time.monotonic()
 result=c.execute(c.WORKER,['timeout','--signal=TERM','--kill-after=10s','1200s','buildah','bud','-t',tag,'-f',dockerfile,destination+'/'+context],c.RESULTS/(name+'.reference-build.log'),1240)
 ref={'build':result,'base_image':c.base(case),'source_dir':str(source),'source_sha_verified':True,'source_evidence':str(evidence_path.relative_to(OUT)),'source_files_verified':len(evidence['files']),'recipe':recipe,'image_built':result['exit_code']==0}
 if ref['image_built']:
  runtime='reference-'+name;c.dex(c.WORKER,['buildah','from','--name',runtime,tag],60,True)
  for k,v in case.get('env',{}).items():c.dex(c.WORKER,['buildah','config','--env',k+'='+v,runtime],check=True)
  diagnostic={'node':'node --version; npm --version; npm ls --depth=0','go':'go version','rust':'rustc --version; cargo --version','java':'java -version; mvn --version'}.get(case['kind'])
  if diagnostic:
   _,versions=c.dex(c.WORKER,['buildah','run',runtime,'--','sh','-c',diagnostic],60)
   (c.RESULTS/(name+'.reference-toolchain-dependencies.log')).write_text(versions+'\n')
  checks=external_probe_hook(c)
  try:ref['runtime']=c.probe_runtime(c.WORKER,runtime,shlex.join(['sh','-c',case['reference_start']]),case,'reference');ref['external_container_http']=checks
  finally:c.dex(c.WORKER,['buildah','rm',runtime],60)
  c.dex(c.WORKER,['buildah','rmi',tag],120)
 c.dex(c.WORKER,['rm','-rf',destination,dockerfile],60,True)
 c.dex(c.WORKER,['buildah','rm','--all'],60);c.dex(c.WORKER,['buildah','rmi','--prune'],120)
 ref['seconds']=round(time.monotonic()-began,3)
 return ref

def prepare_failures():
 if (OUT/'failures-manifest.json').exists():return
 records=[json.loads(p.read_text()) for p in (PRIOR/'results').glob('*.json') if not p.name.endswith('.worker-image.json')]
 failures=[r['case'] for r in records if r.get('complete') and r['classification']!='build-and-runtime-pass']
 if len(failures)!=20:raise RuntimeError('expected twenty previous failures')
 (OUT/'failures-manifest.json').write_text(json.dumps(sorted(failures,key=lambda c:c['name']),indent=2)+'\n')

def main():
 p=argparse.ArgumentParser();p.add_argument('--batch',choices=['failures','fresh'],required=True);p.add_argument('--lane',type=int,choices=[1,2],required=True);p.add_argument('--cases',nargs='*');p.add_argument('--reference',action='store_true');a=p.parse_args()
 OUT.mkdir(parents=True,exist_ok=True);prepare_failures()
 manifest=json.loads((OUT/(a.batch+'-manifest.json')).read_text())
 if a.batch=='fresh' and len({(x['repo'],x['path']) for x in manifest})!=len(manifest):raise RuntimeError('duplicate fresh sources')
 for i,case in enumerate(manifest):
  if a.cases and case['name'] not in a.cases:continue
  if not a.cases and i%2!=a.lane-1:continue
  if not re.fullmatch(r'[a-z0-9][a-z0-9-]+',case['name']):raise RuntimeError('invalid case name')
  c=load_campaign(a.lane,a.batch);path=c.RESULTS/(case['name']+'.json')
  if path.exists():
   r=json.loads(path.read_text())
   if r.get('complete'):print('SKIP',case['name'],flush=True);continue
   raise RuntimeError('incomplete evidence exists: '+case['name'])
  record={'case':case,'started_at':now(),'complete':False,'batch':a.batch,'reference_qualified':a.batch=='failures','qualification':'historical qualification from primary57' if a.batch=='failures' else 'fresh reference trial recorded separately'}
  if a.batch=='failures':
   original=json.loads((PRIOR/'results'/(case['name']+'.json')).read_text());record['prior_worker']=original['worker'];record['prior_classification']=original['classification']
  print('START',a.batch,'lane='+str(a.lane),case['name'],flush=True)
  try:
   if a.reference:
    record['reference']=reference(c,case)
    record['reference_qualified']=record['reference']['image_built'] and record['reference'].get('runtime',{}).get('ready',False)
    # Reference probe hooks must not capture worker requests in their list.
    c=load_campaign(a.lane,a.batch)
   worker(c,case,record)
   classification_record=dict(record);classification_record['reference_qualified']=True
   record['worker_classification']=c.classify(classification_record)
   record['classification']=record['worker_classification'] if record['reference_qualified'] else 'reference-inconclusive'
   record['complete']=True;record['finished_at']=now()
  except Exception as exc:
   record['harness_error']=repr(exc);c.write_json(path,record);raise
  c.write_json(path,record)
  records=[json.loads(x.read_text()) for x in c.RESULTS.glob('*.json') if not x.name.endswith('.worker-image.json')];done=[x for x in records if x.get('complete')]
  c.write_json(OUT/a.batch/('progress-lane'+str(a.lane)+'.json'),{'completed_all_lanes':len(done),'denominator':len(manifest),'images':sum(x['worker']['image_built'] for x in done),'local_http':sum(bool(x['worker'].get('runtime',{}).get('ready')) for x in done),'reference_qualified':sum(x['reference_qualified'] for x in done),'outcomes':dict(collections.Counter(x['classification'] for x in done))})
  print('RESULT',case['name'],record['classification'],flush=True)

if __name__=='__main__':main()
