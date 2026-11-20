#!/usr/bin/env python3
"""Preserve failed pre-campaign validation and verify a corrected frozen binary."""
import argparse, datetime, hashlib, json, pathlib, shutil, subprocess, time

REPO=pathlib.Path(__file__).absolute().parents[2]
OUT=REPO/'docs/worker-build-followup-2026-11-17'
TEMP=pathlib.Path('/tmp/zd-round2-services')

def run(args,**kwargs):
 p=subprocess.run(args,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,**kwargs)
 if p.returncode:raise RuntimeError(p.stdout[-5000:])
 return p.stdout.strip()
def verify(snapshot):
 for path,digest in snapshot.items():
  if hashlib.sha256((REPO/path).read_bytes()).hexdigest()!=digest:raise RuntimeError('preserved evidence changed: '+path)

def main():
 p=argparse.ArgumentParser();p.add_argument('--product-commit',required=True);a=p.parse_args()
 if any((OUT/'failures/results').glob('*.json')) or any((OUT/'fresh/results').glob('*.json')):raise RuntimeError('application trials exist; cannot replace their source')
 prior=json.loads((OUT/'frozen-campaign.json').read_text())
 verify(prior['prior_evidence_sha256']);verify(prior['manifests_sha256'])
 archive=OUT/('pre-campaign-verification-'+prior['product_commit'][:7])
 archive.mkdir(exist_ok=False)
 for name in ['frozen-campaign.json','environment.json','native-integration.json','native-integration.log','validation-controller.log','build-provenance.json']:
  shutil.copy2(OUT/name,archive/name)
 env=json.loads((OUT/'environment.json').read_text())
 for lane in env['lanes']:
  if run(['docker','inspect',lane['worker'],'--format','{{index .Config.Labels "zero-devops-build-followup"}}'])!='20261117':raise RuntimeError('unowned worker')
  if run(['docker','inspect',lane['worker'],'--format','{{.State.Running}}'])!='false':raise RuntimeError('stop idle workers before binary replacement')
 shutil.copy2('/tmp/zd-round2-worker',TEMP/'worker-engine');shutil.copy2('/tmp/zd-round2-deployment-tests',TEMP/'deployment-tests')
 for lane in env['lanes']:
  run(['docker','start',lane['worker']])
  for _ in range(90):
   logs=run(['docker','logs','--since','20s',lane['worker']])
   if 'Worker consumer registered' in logs:break
   time.sleep(1)
  else:raise RuntimeError('worker not ready after replacement')
  lane['binary_sha256']=run(['docker','exec',lane['worker'],'sha256sum','/e2e/worker-engine']).split()[0]
 env['prior_product_commit']=env['product_commit'];env['product_commit']=a.product_commit
 env['worker_binary_sha256']=hashlib.sha256((TEMP/'worker-engine').read_bytes()).hexdigest()
 env['pre_campaign_validation_replacement']={'preserved_evidence':str(archive.relative_to(OUT)),'reason':'runtime asset fixture revealed root-owned 0600 assets inaccessible to non-root runtime; corrected before application trials','replacement_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'cold_pulls_reused':'all ten base image references unchanged; prior fresh-store validation remains authoritative','build_store_state':'base layers warmed by cold pulls and pre-campaign fixtures; no application jobs before replacement'}
 (OUT/'environment.json').write_text(json.dumps(env,indent=2)+'\n')
 provenance=json.loads((OUT/'build-provenance.json').read_text())
 provenance['product_commit']=a.product_commit
 provenance['recorded_at']=datetime.datetime.now(datetime.timezone.utc).isoformat()
 provenance['commands']['git rev-parse HEAD']=a.product_commit
 provenance['binaries_sha256']={path:hashlib.sha256(pathlib.Path(path).read_bytes()).hexdigest() for path in ['/tmp/zd-round2-worker','/tmp/zd-round2-deployment-tests']}
 provenance['worker_binary_sha256']=env['worker_binary_sha256']
 provenance['deployment_tests_binary_sha256']=hashlib.sha256(pathlib.Path('/tmp/zd-round2-deployment-tests').read_bytes()).hexdigest()
 provenance['worker_suite_log']='worker-suite-asset-ownership.log'
 provenance['prior_failed_verification']=str(archive.relative_to(OUT))
 (OUT/'build-provenance.json').write_text(json.dumps(provenance,indent=2)+'\n')
 # The original freeze remains byte-for-byte in the archive. Re-freeze corrected
 # source only after preserving and validating the previous evidence identities.
 (OUT/'frozen-campaign.json').unlink()
 run(['python3',str(REPO/'scripts/worker-build-campaign/round2_parallel.py'),'--freeze'])
 start=time.monotonic()
 with (OUT/'native-integration.log').open('w') as f:
  p=subprocess.run(['docker','exec','-e','RUN_NATIVE_BUILD_TESTS=true','-w','/app','zdround2-2-worker','timeout','--kill-after=10s','1200s','/e2e/deployment-tests','-test.v','-test.run','TestDockerBuild_WithDockerfile|TestNativeBuildIntegration'],stdout=f,stderr=subprocess.STDOUT,timeout=1240)
 (OUT/'native-integration.json').write_text(json.dumps({'exit_code':p.returncode,'seconds':round(time.monotonic()-start,3),'completed_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'log':'native-integration.log','app_jobs_dispatched':False,'prior_failed_verification':str(archive.relative_to(OUT))},indent=2)+'\n')
 if p.returncode:raise RuntimeError('corrected native verification failed; stop before campaign')
 print('corrected frozen worker live verification passed',flush=True)

if __name__=='__main__':main()
