#!/usr/bin/env python3
"""Provision only follow-up campaign workers; retain prior evidence and stores."""
import argparse, datetime, hashlib, json, pathlib, shutil, subprocess, time

REPO=pathlib.Path(__file__).absolute().parents[2]
OUT=REPO/'docs/worker-build-followup-2026-11-17'
TEMP=pathlib.Path('/tmp/zd-round2-services')
LABEL='zero-devops-build-followup=20261117'
IMAGE='zero-devops-e2e-worker:local'
DB='zdfix-db';RABBIT='zdfix-rabbit'

def run(args,**kwargs):
 p=subprocess.run(args,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,**kwargs)
 if p.returncode:raise RuntimeError(repr(args)+'\n'+p.stdout[-5000:])
 return p.stdout.strip()
def docker(*args):return run(['docker',*args])
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def ready(args,attempts=90):
 for _ in range(attempts):
  if subprocess.run(args,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode==0:return
  time.sleep(1)
 raise RuntimeError('readiness failed: '+repr(args))
def exists(kind,name):return subprocess.run(['docker',kind,'inspect',name],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode==0
def owned(kind,name):
 field='{{index .Config.Labels "zero-devops-build-followup"}}' if kind=='container' else '{{index .Labels "zero-devops-build-followup"}}'
 if docker(kind,'inspect','--format',field,name)!='20261117':raise RuntimeError('refuse unrelated resource '+name)

def setup():
 OUT.mkdir(parents=True,exist_ok=True)
 if (OUT/'environment.json').exists():raise RuntimeError('environment evidence exists; do not overwrite')
 for lane in [1,2]:
  prefix='zdround2-'+str(lane)
  for kind,name in [('container',prefix+'-worker'),('container',prefix+'-mock'),('volume',prefix+'-storage'),('network',prefix+'-network')]:
   if exists(kind,name):raise RuntimeError('resource exists: '+name)
 capacity=json.loads(docker('info','--format','{"cpus":{{.NCPU}},"memory_bytes":{{.MemTotal}}}'))
 if capacity['memory_bytes']<5500*1024**2:raise RuntimeError('not enough VM RAM for two 2GiB workers plus services')
 if not pathlib.Path('/tmp/zd-round2-worker').is_file():raise RuntimeError('frozen worker binary missing')
 for c in [DB,RABBIT]:
  if docker('inspect',c,'--format','{{index .Config.Labels "zero-devops-build-fixes"}}')!='20261116':raise RuntimeError('unowned support service')
 docker('start',DB,RABBIT)
 ready(['docker','exec',DB,'pg_isready','-U','e2e','-d','e2e'])
 ready(['docker','exec',RABBIT,'rabbitmq-diagnostics','-q','check_running'])
 TEMP.mkdir(mode=0o700,exist_ok=True)
 if (TEMP/'worker-engine').exists():raise RuntimeError('temporary binary exists; inspect before reuse')
 shutil.copy2('/tmp/zd-round2-worker',TEMP/'worker-engine')
 if pathlib.Path('/tmp/zd-round2-deployment-tests').exists():shutil.copy2('/tmp/zd-round2-deployment-tests',TEMP/'deployment-tests')
 run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','4','-keyout',str(TEMP/'mock.key'),'-out',str(TEMP/'mock.crt'),'-subj','/CN=e2e.r2.cloudflarestorage.com','-addext','subjectAltName=DNS:e2e.r2.cloudflarestorage.com,DNS:testbucket.e2e.r2.cloudflarestorage.com'])
 (TEMP/'mock.key').chmod(0o600)
 old=json.loads((REPO/'docs/worker-build-fixes-2026-11-16/environment.json').read_text())
 lanes=[]
 for lane in [1,2]:
  prefix='zdround2-'+str(lane);net=prefix+'-network';worker=prefix+'-worker';mock=prefix+'-mock';store=prefix+'-storage';vhost='zdround2-'+str(lane)
  docker('network','create','--label',LABEL,net);docker('volume','create','--label',LABEL,store)
  docker('network','connect','--alias','postgres',net,DB);docker('network','connect','--alias','rabbit',net,RABBIT)
  docker('exec',RABBIT,'rabbitmqctl','add_vhost',vhost);docker('exec',RABBIT,'rabbitmqctl','set_permissions','-p',vhost,'e2e','.*','.*','.*')
  common=['run','-d','--label',LABEL,'--network',net]
  docker(*common,'--name',mock,'--network-alias','mock','--network-alias','e2e.r2.cloudflarestorage.com','--network-alias','testbucket.e2e.r2.cloudflarestorage.com','--cpus','0.25','--memory','128m','--pids-limit','64','-v',str(TEMP)+':/e2e:ro','-v',str(REPO/'scripts/worker-build-campaign')+':/harness:ro','--entrypoint','python3',IMAGE,'/harness/mock_services.py')
  ready(['docker','exec',mock,'curl','--fail','-sS','-u','e2e:e2e','http://rabbit:15672/api/overview'])
  env=dict(old['worker_environment']);env['RABBITMQ_CONNECTION_STRING']='amqp://e2e:e2e@rabbit:5672/'+vhost
  envargs=[v for k,value in env.items() for v in ['-e',k+'='+value]]
  docker(*common,'--name',worker,'--privileged','--cpus','1.5','--memory','2g','--pids-limit','512','-v',store+':/var/lib/containers','-v',str(TEMP)+':/e2e:ro','-v',str(REPO/'worker-server/templates')+':/app/templates:ro','--entrypoint','/e2e/worker-engine',*envargs,IMAGE)
  for _ in range(90):
   if 'Worker consumer registered' in docker('logs',worker):break
   time.sleep(1)
  else:raise RuntimeError('worker not ready: '+worker)
  initial=json.loads(docker('exec',worker,'buildah','images','--json'))
  if initial:raise RuntimeError('new worker storage is not empty')
  lanes.append({'lane':lane,'worker':worker,'mock':mock,'network':net,'volume':store,'vhost':vhost,'initial_images':initial,'limits':json.loads(docker('inspect',worker,'--format','{"nano_cpus":{{.HostConfig.NanoCpus}},"memory_bytes":{{.HostConfig.Memory}},"pids":{{.HostConfig.PidsLimit}}}')),'binary_sha256':docker('exec',worker,'sha256sum','/e2e/worker-engine').split()[0]})
 metadata={'created_at':now(),'product_commit':run(['git','rev-parse','HEAD']),'worker_binary_sha256':hashlib.sha256((TEMP/'worker-engine').read_bytes()).hexdigest(),'runtime_image_id':docker('image','inspect',IMAGE,'--format','{{.Id}}'),'worker_environment':env,'docker_capacity':capacity,'lanes':lanes,'support_services':[DB,RABBIT],'publication':'fake upload sink only; no deployment or publication','limits':'2 x 1.5 CPU / 2 GiB builders; supporting services retain previous limits','old_worker_storage':'preserved; never mounted in this campaign'}
 (OUT/'environment.json').write_text(json.dumps(metadata,indent=2)+'\n')
 print('follow-up isolated workers ready',flush=True)

def pause():
 stopped=[]
 for lane in [1,2]:
  for suffix in ['worker','mock']:
   c='zdround2-'+str(lane)+'-'+suffix
   if exists('container',c):owned('container',c);docker('stop','-t','10',c);stopped.append(c)
 for c in [DB,RABBIT]:
  if docker('inspect',c,'--format','{{index .Config.Labels "zero-devops-build-fixes"}}')=='20261116':docker('stop','-t','10',c);stopped.append(c)
 (OUT/('pause-'+datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')+'.json')).write_text(json.dumps({'at':now(),'stopped':stopped,'stores_preserved':True},indent=2))
 print('paused',flush=True)

def cleanup():
 removed=[]
 for lane in [1,2]:
  prefix='zdround2-'+str(lane);net=prefix+'-network'
  for suffix in ['worker','mock']:
   c=prefix+'-'+suffix
   if exists('container',c):owned('container',c);docker('rm','-f','-v',c);removed.append(c)
  volume=prefix+'-storage'
  if exists('volume',volume):owned('volume',volume);docker('volume','rm',volume)
  if exists('network',net):
   owned('network',net)
   for c in [DB,RABBIT]:docker('network','disconnect',net,c)
   docker('network','rm',net)
 # Shared prior support services and old stores are retained, stopped.
 for c in [DB,RABBIT]:docker('stop','-t','10',c)
 (OUT/'cleanup.json').write_text(json.dumps({'at':now(),'containers_removed':removed,'prior_resources_retained':True,'global_prune':False},indent=2)+'\n')
 print('follow-up resources cleaned',flush=True)

if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('action',choices=['setup','pause','cleanup']);a=p.parse_args();globals()[a.action]()
