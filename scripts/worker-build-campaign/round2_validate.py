#!/usr/bin/env python3
"""Cold pulls then live regression fixtures before any follow-up app jobs."""
import datetime, json, pathlib, subprocess, time

REPO=pathlib.Path(__file__).absolute().parents[2]
OUT=REPO/'docs/worker-build-followup-2026-11-17'
WORKER='zdround2-2-worker'

def command(args,timeout=30):return subprocess.run(args,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=timeout)
def docker(*args,timeout=30):
 p=command(['docker',*args],timeout)
 if p.returncode:raise RuntimeError(p.stdout[-5000:])
 return p.stdout.strip()

def main():
 if (OUT/'cold-pulls.json').exists() or (OUT/'native-integration.json').exists():raise RuntimeError('validation evidence exists; preserve it')
 if docker('inspect',WORKER,'--format','{{index .Config.Labels "zero-devops-build-followup"}}')!='20261117':raise RuntimeError('unowned validation worker')
 initial=json.loads(docker('exec',WORKER,'buildah','images','--json'))
 if initial:raise RuntimeError('cold validation store is not empty')
 images=['docker.io/library/alpine:3.20','docker.io/library/node:24-alpine','docker.io/library/golang:1.26.0-alpine','docker.io/library/golang:1.27.0-alpine','docker.io/library/rust:1-alpine','docker.io/library/eclipse-temurin:21-jdk-alpine','docker.io/library/eclipse-temurin:21-jre-alpine','docker.io/library/composer:2','docker.io/library/php:8.3-cli-alpine','docker.io/library/maven:3-eclipse-temurin-21']
 records=[]
 for i,image in enumerate(images):
  print('COLD PULL',image,flush=True);start=time.monotonic()
  p=command(['docker','exec',WORKER,'timeout','--kill-after=10s','360s','buildah','pull',image],390)
  log='cold-pull-'+str(i)+'.log';(OUT/log).write_text(p.stdout)
  item={'image':image,'exit_code':p.returncode,'seconds':round(time.monotonic()-start,3),'log':log}
  if p.returncode==0:
   inspection=docker('exec',WORKER,'buildah','inspect','--type','image',image)
   (OUT/('cold-image-'+str(i)+'.json')).write_text(inspection+'\n')
  records.append(item)
  (OUT/'cold-pulls.json').write_text(json.dumps({'initial_images':initial,'worker':WORKER,'isolated_storage':'zdround2-2-storage','pulls':records},indent=2)+'\n')
 print('LIVE NATIVE FIXTURES',flush=True)
 start=time.monotonic()
 with (OUT/'native-integration.log').open('w') as f:
  p=subprocess.run(['docker','exec','-e','RUN_NATIVE_BUILD_TESTS=true','-w','/app',WORKER,'timeout','--kill-after=10s','1200s','/e2e/deployment-tests','-test.v','-test.run','TestDockerBuild_WithDockerfile|TestNativeBuildIntegration'],stdout=f,stderr=subprocess.STDOUT,timeout=1240)
 (OUT/'native-integration.json').write_text(json.dumps({'exit_code':p.returncode,'seconds':round(time.monotonic()-start,3),'completed_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'log':'native-integration.log','app_jobs_dispatched':False},indent=2)+'\n')
 if p.returncode:raise RuntimeError('live regression fixtures failed; inspect log before campaign')
 print('validation complete',flush=True)

if __name__=='__main__':main()
