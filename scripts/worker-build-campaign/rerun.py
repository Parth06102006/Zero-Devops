#!/usr/bin/env python3
"""Worker-only replay of the preserved 57 reference-qualified pinned cases.
Probe and worker routines adapted from the original campaign harness, whose
baseline evidence is never modified. Eligibility is historical; reference
builds are not rerun. All services and storage belong to this replay.
"""
import argparse, base64, datetime, hashlib, json, os, pathlib, re, shlex, shutil, statistics, subprocess, sys, time, urllib.request, uuid
REPO=pathlib.Path(__file__).resolve().parents[2]
ROOT=REPO/'docs/worker-build-fixes-2026-11-16'
RESULTS=ROOT/'results'
WORKER='zdfix-worker'; DB='zdfix-db'; REF='zdfix-unused-reference'
def cmd(argv,timeout=30,check=False):
 p=subprocess.run(argv,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True,timeout=timeout)
 if check and p.returncode:raise RuntimeError('command failed '+repr(argv)+': '+p.stdout[-2000:])
 return p.returncode,p.stdout.strip()
def dex(container,args,timeout=30,check=False):return cmd(['docker','exec',container]+args,timeout,check)
def guard():
 if shutil.disk_usage(ROOT).free<20*1024**3:raise RuntimeError('disk guard: less than 20 GiB free')
 for c in [WORKER]:
  _,v=dex(c,['du','-sb','/var/lib/containers/storage'],60)
  try:n=int(v.split()[0])
  except:n=0
  if n>25*1024**3:raise RuntimeError('storage guard >25GiB: '+c)
def resource(container):
 _,s=cmd(['docker','stats','--no-stream','--format','{{.CPUPerc}}|{{.MemUsage}}|{{.PIDs}}',container],15)
 _,cg=dex(container,['sh','-c','for f in cpu.stat memory.peak memory.current; do echo "$f"; head -20 /sys/fs/cgroup/$f; done'],10)
 return {'stats':s,'cgroup':cg}
def execute(container,args,log,timeout=1200):
 start=time.monotonic();samples=[]
 _,baseline=dex(container,['sh','-c','echo 0 > /sys/fs/cgroup/memory.peak 2>/dev/null || true; head -1 /sys/fs/cgroup/memory.peak; head -15 /sys/fs/cgroup/cpu.stat'],10)
 with open(log,'w') as f:
  p=subprocess.Popen(['docker','exec',container]+args,stdout=f,stderr=subprocess.STDOUT)
  deadline=start+timeout; nxt=start; timed=False
  while p.poll() is None:
   now=time.monotonic()
   if now>=nxt:
    guard();samples.append({'seconds':round(now-start,2),**resource(container)});nxt=now+20
   if now>=deadline:
    # timeout wrapper runs INSIDE container, so host exec exit does not leave build running.
    timed=True;break
   time.sleep(1)
  if timed:
   p.wait(timeout=40)
  rc=p.returncode
 _,final_cgroup=dex(container,['sh','-c','head -1 /sys/fs/cgroup/memory.peak; head -15 /sys/fs/cgroup/cpu.stat; head -10 /sys/fs/cgroup/memory.events'],10)
 return {'seconds':round(time.monotonic()-start,3),'exit_code':rc,'timed_out':timed,'resource_samples':samples,'cgroup_before':baseline,'cgroup_after':final_cgroup}
def api(path,data=None):
 port=cmd(['docker','port','zdfix-rabbit','15672/tcp'],check=True)[1].rsplit(':',1)[1]
 headers={'Authorization':'Basic '+base64.b64encode(b'e2e:e2e').decode(),'Content-Type':'application/json'}
 req=urllib.request.Request('http://127.0.0.1:'+port+path,data=None if data is None else json.dumps(data).encode(),headers=headers,method='GET' if data is None else 'POST')
 with urllib.request.urlopen(req,timeout=20) as r:return json.load(r)
def base(c):
 if c.get('base_image'):return c['base_image']
 return {'node':'docker.io/library/node:24-alpine','go':'docker.io/library/golang:1.26-alpine','rust':'docker.io/library/rust:1-alpine','python':'docker.io/library/python:3.12-slim','java':'docker.io/library/maven:3-eclipse-temurin-21','php':'docker.io/library/composer:2','ruby':'docker.io/library/ruby:3.3','dotnet':'mcr.microsoft.com/dotnet/sdk:6.0'}[c['kind']]
def source(c):return ROOT/'sources'/c['repo'].replace('/','--')
def app_path(c):return source(c)/c['path']
def source_metrics(c):
 p=app_path(c);files=[f for f in p.rglob('*') if f.is_file()];return {'app_source_bytes':sum(f.stat().st_size for f in files),'app_source_files':len(files),'repo_source_bytes':sum(f.stat().st_size for f in source(c).rglob('*') if f.is_file())}
def probe_runtime(container,name,start_command,c,label):
 log=RESULTS/(c['name']+'.'+label+'-runtime.log')
 command="set -m; "+start_command+" & p=$!; trap 'kill -- -\"$p\" 2>/dev/null || kill \"$p\" 2>/dev/null || true; wait \"$p\" 2>/dev/null || true' EXIT TERM INT; while kill -0 \"$p\" 2>/dev/null; do sleep 1; done; wait \"$p\""
 began=time.monotonic()
 # runtime wrapper is capped internally and process group is terminated on stop
 with open(log,'w') as f:
  p=subprocess.Popen(['docker','exec',container,'timeout','--signal=TERM','--kill-after=5s','95s','buildah','run',name,'--','sh','-c',command],stdout=f,stderr=subprocess.STDOUT)
  samples=[];ready=False;last='';http=[]
  for i in range(45):
   request_opts=c.get('probe_args',[])
   rc,last=dex(container,['curl','--max-time','2','-sS','-o','/tmp/zdfix-probe-body','-w','%{http_code}|%{time_total}|%{content_type}']+request_opts+['http://127.0.0.1:'+str(c['port'])+c['probe']],8)
   fields=last.split('|')
   accepted=c.get('expected_status',list(range(200,400)))
   if rc==0 and fields and fields[0].isdigit() and int(fields[0]) in accepted:
    ready=True;break
   if p.poll() is not None:break
   time.sleep(1)
  startup=time.monotonic()-began
  if ready:
   _,body=dex(container,['head','-c','250','/tmp/zdfix-probe-body']);
   for _ in range(8):
    rc,v=dex(container,['curl','--max-time','3','-sS','-o','/dev/null','-w','%{http_code}|%{time_total}']+request_opts+['http://127.0.0.1:'+str(c['port'])+c['probe']],8)
    try:http.append({'status':int(v.split('|')[0]),'seconds':float(v.split('|')[1])})
    except:pass
  else:body=''
  if not ready:
   _,body=dex(container,['head','-c','250','/tmp/zdfix-probe-body']);
  else:body=body
  # stop only the explicit per-case timeout/runtime process, via buildah container host pid list
  killer="import os,signal; target="+repr('timeout --signal=TERM --kill-after=5s 95s buildah run '+name)+";\nfor item in os.listdir('/proc'):\n if not item.isdigit(): continue\n try:\n  data=open('/proc/'+item+'/cmdline','rb').read().replace(b'\\0',b' ').decode()\n  if data.startswith(target+' '): os.kill(int(item),signal.SIGTERM)\n except (OSError,UnicodeError): pass"
  dex(container,['python3','-c',killer],15)
  try:p.wait(timeout=12)
  except subprocess.TimeoutExpired:p.wait(timeout=95)
 return {'ready':ready,'startup_seconds':round(startup,3),'probe_url':'http://127.0.0.1:'+str(c['port'])+c['probe'],'first_response':last,'response_sample':body,'warm_http':http,'note':'Expected HTTP status confirms smoke endpoint only, not complete app behavior; status/body recorded'}
def worker(c,record):
 ident=str(uuid.uuid4());event=str(uuid.uuid4());corr='zdfix-'+ident
 req={'version':1,'event_id':event,'deployment_id':ident,'project_id':'isolated-zdfix','installation_id':1,'repository_id':1,'clone_url':'https://github.com/'+c['repo']+'.git','commit_sha':c['sha'],'requested_ref':'HEAD','trigger':'manual','generation':1,'retry_count':0,'correlation_id':corr,'configuration':{'executable':{'go':'go','rust':'cargo','python':'python','java':'mvn','php':'composer','ruby':'bundle','dotnet':'dotnet'}.get(c['kind'],'npm'),'args':['run','build'] if c['kind']=='node' else [],'working_dir':c['path'],'scanner_policy_version':'v1'}}
 began=time.time();since=str(int(began)-1)
 body={'properties':{'content_type':'application/json','message_id':event,'correlation_id':corr},'routing_key':'deploy.jobs','payload':json.dumps(req),'payload_encoding':'string'}
 if not api('/api/exchanges/%2F/amq.default/publish',body).get('routed'):raise RuntimeError('job not routed')
 row='';state='pending';samples=[];nxt=0
 while time.time()-began<1100:
  _,row=dex(DB,['psql','-U','e2e','-d','e2e','-A','-t','-F','|','-c',f"SELECT status, COALESCE(error_message,''), COALESCE(output_url,'') FROM deployments WHERE id='{ident}';"],20)
  state=row.split('|')[0] if row else 'pending'
  if time.time()>=nxt:guard();samples.append({'seconds':round(time.time()-began,2),**resource(WORKER)});nxt=time.time()+20
  if state in ['success','canceled']:break
  if cmd(['docker','inspect','--format','{{.State.Running}}',WORKER])[1]!='true':raise RuntimeError('worker stopped')
  time.sleep(2)
 if state not in ['success','canceled']:raise RuntimeError('worker did not terminate within deadline; pause campaign')
 _,logs=cmd(['docker','logs','--since',since,WORKER],60)
 _,mock=cmd(['docker','logs','--since',since,'zdfix-mock'],60)
 (RESULTS/(c['name']+'.worker.log')).write_text(logs+'\n');(RESULTS/(c['name']+'.mock.log')).write_text(mock+'\n')
 wr={'commit_sha':c['sha'],'deployment_id':ident,'state':state,'database_row':row,'end_to_end_seconds':round(time.time()-began,3),'resource_samples':samples,'configuration_sent':req['configuration']}
 match=re.search(r'"framework": "([^"]+)"',logs);wr['framework']=match[1] if match else None
 phases=[]
 for line in logs.splitlines():
  if re.match(r'^\d{4}-',line):
   fields=line.split('\t');phases.append({'at':fields[0],'event':fields[3] if len(fields)>3 else line})
 wr['phase_events']=phases
 uploads=re.findall(r'TEST_UPLOAD path=/images/'+ident+r'\.tar[^\n]*bytes=(\d+)',mock)
 wr['upload_bytes']=int(uploads[-1]) if uploads else 0
 try:wr['status_messages']=api('/api/queues/%2F/deploy.status/get',{'count':100,'ackmode':'ack_requeue_false','encoding':'auto','truncate':50000})
 except Exception as e:wr['status_queue_error']=str(e)
 record['worker']=wr
 tag='localhost/deploy-'+ident+':latest';rc,info=dex(WORKER,['buildah','inspect','--type','image',tag],60)
 wr['image_built']=rc==0
 if wr['image_built']:
  info=json.loads(info)
  manifest=json.loads(info.get('Manifest','{}'))
  wr['image_layer_bytes']=sum(layer.get('size',0) for layer in manifest.get('layers',[]))
  wr['image_config_bytes']=manifest.get('config',{}).get('size')
  (RESULTS/(c['name']+'.worker-image.json')).write_text(json.dumps(info,indent=2))
  conf=info.get('OCIv1',{}).get('config') or info.get('Docker',{}).get('config') or {}
  command_cfg=info.get('Config')
  if isinstance(command_cfg,str):
   try:conf=json.loads(command_cfg).get('config',{})
   except (ValueError,TypeError):pass
  command=shlex.join((conf.get('Entrypoint') or [])+(conf.get('Cmd') or []))
  runtime_name='runtime-'+c['name'];dex(WORKER,['buildah','from','--name',runtime_name,tag],60,True)
  for k,v in c.get('env',{}).items():dex(WORKER,['buildah','config','--env',k+'='+v,runtime_name],check=True)
  runtime_case=dict(c)
  if wr['framework'] in ['vite','vue','svelte','solid','astro']:runtime_case['port']=4173 if wr['framework']!='astro' else 4321
  if wr['framework'] in ['react','node','nextjs']:runtime_case['port']=3000
  try:wr['runtime']=probe_runtime(WORKER,runtime_name,command,runtime_case,'worker')
  finally:dex(WORKER,['buildah','rm',runtime_name],60)
  dex(WORKER,['buildah','rmi',tag],120)
 # images from failed intermediate layers are campaign-only; base images can be cached temporarily
 dex(WORKER,['buildah','rm','--all'],60);dex(WORKER,['buildah','rmi','--prune'],120)
def classify(record):
 if not record.get('reference_qualified'):return 'reference-inconclusive'
 wr=record['worker']
 if wr.get('image_built'):
  if not wr.get('runtime',{}).get('ready'):return 'image-build-pass/runtime-fail'
  return 'build-and-runtime-pass'
 log=(RESULTS/(record['case']['name']+'.worker.log')).read_text()
 if 'short-name resolution' in log:return 'base-image-resolution'
 if 'npm ci' in log and ('existing package-lock.json' in log or 'no such file' in log):return 'missing-lockfile/install-policy'
 if 'requires go >=' in log or 'requires a minimum Node.js version' in log or 'Unsupported engine' in log:return 'toolchain-version'
 if 'workspace root' in log or 'workspace.package' in log:return 'workspace-context'
 if 'no framework detected' in log:return 'detection-unsupported'
 if 'sharp' in log and ('Cannot find module' in log or 'install' in log):return 'native-dependency/install-scripts'
 if 'ZERODEVOPS_SSR_MISMATCH' in log:return 'static-output-path/misleading-ssr'
 if 'PrismaClient' in log:return 'missing-code-generation'
 return 'other-build-failure'

def replay(baseline, names):
 if cmd(['docker','inspect','--format','{{index .Config.Labels "zero-devops-build-fixes"}}',WORKER],check=True)[1]!='20261116':raise RuntimeError('refusing unowned worker storage')
 RESULTS.mkdir(parents=True,exist_ok=True)
 cases=json.load(open(baseline/'manifest.json'))
 qualified=[]
 for c in cases:
  original=json.load(open(baseline/'results'/(c['name']+'.json')))
  if original.get('reference_qualified'):qualified.append((c,original))
 if len(qualified)!=57:raise RuntimeError('expected exactly 57 baseline-qualified cases')
 for c,original in qualified:
  name=c['name'];path=RESULTS/(name+'.json')
  if names and name not in names:continue
  if path.exists():
   previous=json.load(open(path))
   if previous.get('complete'):print('SKIP completed',name,flush=True);continue
   raise RuntimeError('incomplete evidence exists for '+name+'; inspect before resuming')
  print('START',name,flush=True)
  record={'case':c,'reference_qualified':True,'reference_eligibility':'preserved baseline; reference build/runtime not rerun','baseline_classification':original['classification'],'baseline_worker':original.get('worker',{}),'started_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'complete':False}
  try:
   worker(c,record)
   record['classification']=classify(record)
   record['complete']=True
  except Exception as exc:
   record['harness_error']=repr(exc)
   path.write_text(json.dumps(record,indent=2))
   raise
  path.write_text(json.dumps(record,indent=2))
  print('RESULT',name,record['classification'],flush=True)
  records=[json.load(open(p)) for p in RESULTS.glob('*.json') if not p.name.endswith('.worker-image.json')]
  finished=[r for r in records if r.get('complete')]
  (ROOT/'progress.json').write_text(json.dumps({'completed':len(finished),'denominator':57,'image_builds':sum(r['worker'].get('image_built',False) for r in finished),'runtime_passes':sum(r['worker'].get('runtime',{}).get('ready',False) for r in finished),'outcomes':{k:sum(r['classification']==k for r in finished) for k in sorted({r['classification'] for r in finished})}},indent=2))

if __name__=='__main__':
 parser=argparse.ArgumentParser()
 parser.add_argument('--baseline',type=pathlib.Path,required=True)
 parser.add_argument('--cases',nargs='*')
 args=parser.parse_args()
 replay(args.baseline,args.cases)
