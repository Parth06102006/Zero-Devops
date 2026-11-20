#!/usr/bin/env python3
"""Separate reference repeat: normalize only registry names; no worker job."""
import sys
import argparse
import json
from pathlib import Path
import secrets

import broad_fastapi as f

BASE = f.EVIDENCE/'pilot-03'
OUT = f.EVIDENCE/'pilot-03-reference-registry-repeat'
RECIPE = 'backend/Dockerfile'
REGISTRIES = {'oven/bun:1':'docker.io/oven/bun:1', 'python:3.14':'docker.io/library/python:3.14'}


def main():
    global OUT
    parser = argparse.ArgumentParser()
    parser.add_argument('--retry-dns', action='store_true')
    retry = parser.parse_args().retry_dns
    if retry:
        assert 'DNSResolveFailed' in (OUT/'reference-build.log').read_text()
        previous_attempt = json.loads((OUT/'result.json').read_text())
        assert previous_attempt['complete'] and not previous_attempt['reference']['image_built']
        OUT = f.EVIDENCE/'pilot-03-reference-dns-retry'
    baseline = json.loads((BASE/'result.json').read_text())
    assert baseline['complete'] and not baseline['reference']['image_built'] and not baseline['worker']['image_built']
    assert 'short-name resolution enforced' in (BASE/'reference-build.log').read_text()
    original = f.NAMES['worker']
    for role in ('worker','appdb','mock'):
        info = json.loads(f.docker('inspect',f.NAMES[role]))[0]
        assert info['Config']['Labels'][f.LABEL]==f.PREFIX and not info['State']['Running']
    for kind,role in [('network','net'),('volume','storage')]:
        assert json.loads(f.docker(kind,'inspect',f.NAMES[role]))[0]['Labels'][f.LABEL]==f.PREFIX
    OUT.mkdir(exist_ok=False)
    bundle = Path(baseline['local_work_root'])/'bundle'
    dockerfile = bundle/'app'/RECIPE
    assert f.sha(dockerfile)==baseline['reference_recipe_sha256']
    copied = OUT/'reference.Dockerfile'
    recipe = dockerfile.read_text()
    for source,target in REGISTRIES.items():
        assert f'FROM {source}' in recipe
        recipe = recipe.replace(f'FROM {source}',f'FROM {target}',1)
    copied.write_text(recipe)
    alternate = bundle/'registry-normalized.Dockerfile'
    if retry:
        assert alternate.read_text() == recipe
    else:
        with alternate.open('x') as o:
            o.write(recipe)
    assert not f.docker('ps','--filter','volume='+f.NAMES['storage'],'--format','{{.ID}}')
    f.NAMES['worker'] = f.PREFIX+('-reference-dns-retry' if retry else '-reference-repeat')
    state = {'case':f.CASE,'product_commit':f.PRODUCT,'started_at':f.stamp(),'complete':False,'reference_only':True,'prior_attempt':'../pilot-03/result.json','original_worker_ref':original,'changes':REGISTRIES,'source_recipe_sha256':f.sha(dockerfile),'normalized_recipe_sha256':f.sha(copied),'build_limit_seconds':1200,'reference_scope':baseline['reference_scope']}
    def save():
        f.common.write_json(OUT/'result.json',state)
    save()
    started = []
    try:
        for role in ('mock','appdb'):
            started.append(role)
            f.docker('start',f.NAMES[role])
        f.until(lambda: f.dex('appdb','pg_isready','-h','127.0.0.1','-U','pilot','-d','reference',check=False).returncode==0)
        params = json.loads(f.docker('inspect',f.NAMES['appdb'],'--format','{{json .Config.Env}}'))
        db_secret = next(v.partition('=')[2] for v in params if v.startswith('POSTGRES_PASSWORD='))
        started.append('worker')
        f.service('worker',baseline['images']['worker'],'1.5','2g','512','--privileged','-e','BUILDAH_ISOLATION=chroot','-v',f.NAMES['storage']+':/var/lib/containers','-v',str(bundle)+':/e2e:ro','--entrypoint','sleep',command=('infinity',))
        tag = 'localhost/zdbroad-pilot3-fastapi-reference:'+('dns-retry' if retry else 'registry-repeat')
        print('FastAPI reference-only registry-normalized build',flush=True)
        rc = f.logged(OUT/'reference-build.log','docker','exec',f.NAMES['worker'],'timeout','--kill-after=10s','1200s','buildah','bud','-f','/e2e/registry-normalized.Dockerfile','-t',tag,'/e2e/app',timeout=1230)
        state['reference'] = {'exit_code':rc,'image_built':rc==0}; save()
        if rc==0:
            (OUT/'reference-image.json').write_text(f.dex('worker','buildah','inspect','--type','image',tag)+'\n')
            name = 'zdbroad-fastapi-reference-repeat'; f.dex('worker','buildah','from','--name',name,tag)
            state['reference']['runtime'] = f.runtime_probe(OUT,name,'fastapi run --workers 4','reference',secrets.token_hex(20),db_secret,secrets.token_hex(20))
            f.dex('worker','buildah','rm',name)
        state['complete'] = True
    except Exception as exc:
        state['harness_error'] = type(exc).__name__  # Command arguments may contain disposable credentials.
        raise
    finally:
        state['resource_stop_errors'] = []
        for role in reversed(started):
            try:
                name = f.NAMES[role]
                info = f.docker('inspect',name,'--format','{{index .Config.Labels "'+f.LABEL+'"}}',check=False)
                if info.returncode or info.stdout.strip()!=f.PREFIX:
                    continue
                (OUT/f'final-{role}.log').write_text(f.docker('logs',name,check=False).stdout)
                f.docker('stop','-t','10',name,timeout=30)
            except Exception as exc:
                state['resource_stop_errors'].append({'role':role,'error':type(exc).__name__})
        state['finished_at']=f.stamp(); save()


if __name__=='__main__':
    main()
