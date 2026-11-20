#!/usr/bin/env python3
"""Isolated FastAPI backend pilot using the pinned upstream Dockerfile as reference."""
import sys
import datetime as dt
import secrets
import hashlib
import json
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parents[4] / "scripts/worker-build-campaign"))
import shutil
import subprocess as sp
import tempfile
import time

import rerun as common
import round2_run as previous

ROOT = Path(__file__).resolve().parents[4]
EVIDENCE = ROOT / 'docs/worker-build-compatibility-broad'
PRODUCT = 'd15eae067cf77a8dcb5d6d6aaf8a59b166c65106'
PREFIX = 'zdbroad-pilot3'
LABEL = 'zero-devops-broad-run'
NAMES = {k: f'{PREFIX}-{k}' for k in ['worker', 'mock', 'db', 'appdb', 'rabbit', 'net', 'storage', 'pgdata', 'appdata', 'rabbitdata']}
CASE = dict(name='fastapi-uv-sqlmodel', repo='fastapi/full-stack-fastapi-template', sha='77e550a73bb49dd2967622c5ed11bf7a49603c93', path='backend', kind='python', port=8000)
MANIFEST_HASH = 'a4fdc75ae9c47e77dd70c3460a4a3a83545def601650786df386976b49da892e'
LOCK_HASH = 'b6f25f6aa3090eb18843be1d0ac0d585dda8c472434f84f4d28cbc3d8dbdd85b'
EMAIL = 'pilot@example.com'


def stamp():
    return dt.datetime.now(dt.timezone.utc).isoformat()


def cmd(*args, timeout=60, check=True, **kw):
    p = sp.run(args, stdout=sp.PIPE, stderr=sp.STDOUT, text=True, timeout=timeout, **kw)
    if check and p.returncode:
        raise RuntimeError(f'{args[:4]!r} exited {p.returncode}: {p.stdout[-2000:]}')
    return p.stdout.strip() if check else p


def docker(*args, **kw):
    return cmd('docker', *args, **kw)


def dex(role, *args, **kw):
    return docker('exec', NAMES[role], *args, **kw)


def until(f, timeout=90):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        val = f()
        if val:
            return val
        time.sleep(1)
    raise TimeoutError('readiness deadline expired')


def logged(file, *args, timeout):
    with file.open('x') as f:
        p = sp.run(args, stdout=f, stderr=sp.STDOUT, timeout=timeout)
    return p.returncode


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()


def pin_image(image):
    found = docker('image', 'inspect', image, '--format', '{{.Id}}', check=False)
    if found.returncode:
        docker('pull', image, timeout=180)
    return docker('image', 'inspect', image, '--format', '{{.Id}}')


def service(role, image, cpus, memory, pids, *options, command=()):
    docker('run', '-d', '--name', NAMES[role], '--label', f'{LABEL}={PREFIX}', '--network', NAMES['net'], '--network-alias', role,
           '--cpus', cpus, '--memory', memory, '--pids-limit', pids, *options, image, *command)


def tcp(container_ip, port):
    return dex('worker', 'python3', '-c', 'import socket,sys;s=socket.socket();s.settimeout(.3);print(s.connect_ex((sys.argv[1],int(sys.argv[2])))==0)', container_ip, str(port), timeout=5) == 'True'


def http(origin, url, *args):
    p = dex(origin, 'curl', '-sS','--max-time','4','-w','\n%{http_code}',*args,url,timeout=7,check=False)
    body, _, status = p.stdout.strip().rpartition('\n')
    try:
        value = json.loads(body)
    except ValueError:
        value = None
    return status if p.returncode == 0 else None, value


def runtime_probe(out, name, command, label, app_secret, db_secret, password):
    port = 8000
    assert not tcp('127.0.0.1', port), 'stale listener before start'
    ip = docker('inspect', '-f', '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}', NAMES['worker'])
    env = {'DATABASE_URL':f'postgresql://pilot:{db_secret}@appdb:5432/{label}', 'PROJECT_NAME':'Broad compatibility pilot','SECRET_KEY':app_secret,'FIRST_SUPERUSER':EMAIL,'FIRST_SUPERUSER_PASSWORD':password}
    for k,v in env.items():
        dex('worker','buildah','config','--env',f'{k}={v}',name)
    result = {'port_closed_before_start':True,'ready':False,'external_pass':False,'checks':[]}
    result['fixture_exit_code'] = logged(out/f'{label}-fixture.log','docker','exec',NAMES['worker'],'timeout','--kill-after=5s','50s','buildah','run',name,'--','sh','scripts/prestart.sh',timeout=60)
    with (out/f'{label}-runtime.log').open('x') as stdout:
        p = sp.Popen(['docker','exec',NAMES['worker'],'timeout','--signal=TERM','--kill-after=5s','65s','buildah','run',name,'--','sh','-c','exec '+command], stdout=stdout, stderr=sp.STDOUT)
        try:
            for _ in range(20):
                if p.poll() is not None:
                    break
                if tcp('127.0.0.1',port):
                    result['listener_started'] = True
                    break
                time.sleep(.5)
            if result.get('listener_started'):
                for origin, address in [('worker','127.0.0.1'),('mock',ip)]:
                    base = f'http://{address}:{port}/api/v1'
                    status, body = http(origin,base+'/utils/health-check/')
                    checks = {'origin':origin,'http_status':status,'health_pass':status=='200' and body is True, 'item_round_trip':None}
                    if checks['health_pass'] and result['fixture_exit_code']==0:
                        login_status, response = http(origin,base+'/login/access-token','--data-urlencode','username='+EMAIL,'--data-urlencode','password='+password)
                        token = response.get('access_token') if isinstance(response,dict) else None
                        checks.update(login_status=login_status,item_round_trip=False)
                        if login_status=='200' and token:
                            auth = 'Authorization: Bearer '+token
                            title = 'Disposable product '+origin
                            create_status,item = http(origin,base+'/items/','-H',auth,'-H','Content-Type: application/json','--data',json.dumps({'title':title}))
                            item_id = item.get('id') if isinstance(item,dict) else None
                            if create_status in ('200','201') and item_id:
                                read_status,read = http(origin,base+'/items/'+item_id,'-H',auth)
                                checks['item_round_trip'] = read_status=='200' and isinstance(read,dict) and read.get('title')==title and read.get('id')==item_id
                            checks['create_status'] = create_status
                    result['checks'].append(checks)
                result['ready'],result['external_pass'] = (check['health_pass'] for check in result['checks'])
        finally:
            p.wait(timeout=75)
    result['port_closed_after_stop'] = not tcp('127.0.0.1',port)
    result['functional_pass'] = len(result['checks'])==2 and all(c['item_round_trip'] is True for c in result['checks']) and result['port_closed_after_stop']
    return result


def main():
    out = EVIDENCE / 'pilot-03'
    out.mkdir(exist_ok=False)
    state = {'case': CASE, 'product_commit': PRODUCT, 'started_at': stamp(), 'complete': False}
    def save():
        common.write_json(out / 'result.json', state)
    def stage(name):
        state['stage'] = name
        save()
        print(name, flush=True)
    created = []
    try:
        stage('freeze-source-and-inputs')
        cmd('git','-C',str(ROOT),'diff','--quiet',PRODUCT,'--','worker-server')
        capacity = json.loads(docker('info','--format','{"cpu":{{.NCPU}},"memory":{{.MemTotal}}}'))
        if capacity['memory'] < 5 * 1024**3 or shutil.disk_usage(ROOT).free < 20 * 1024**3:
            raise RuntimeError('insufficient engine capacity or checkout disk space')
        if any(docker('inspect', name, check=False).returncode == 0 for name in NAMES.values()):
            raise RuntimeError('pilot resource name collision')
        temp = Path(tempfile.mkdtemp(prefix=PREFIX+'-', dir=str(Path.home())))
        state['local_work_root'] = str(temp)
        bundle = temp / 'bundle'; bundle.mkdir(mode=0o700)
        source = temp / 'source'; source.mkdir()
        cmd('git','-C',str(source),'init','-q')
        git_url = f'https://github.com/{CASE["repo"]}.git'
        cmd('git','-C',str(source),'fetch','--depth=1',git_url,CASE['sha'], timeout=180)
        cmd('git','-C',str(source),'checkout','--detach',CASE['sha'])
        assert cmd('git','-C',str(source),'rev-parse','HEAD') == CASE['sha']
        app_path = source / CASE['path']
        assert sha(app_path/'pyproject.toml') == MANIFEST_HASH
        assert sha(source/'uv.lock') == LOCK_HASH
        context = bundle/'app'; context.mkdir()
        archive = temp/'source.tar'
        cmd('git','-C',str(source),'archive','--output',str(archive),CASE['sha'],timeout=120)
        cmd('tar','-xf',str(archive),'-C',str(context),timeout=120)
        cmd('env','CGO_ENABLED=0','go','build','-buildvcs=false','-trimpath','-o',str(bundle/'worker-engine'),'./cmd/worker-server',cwd=ROOT/'worker-server',timeout=240)
        cmd('openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','3','-keyout',str(bundle/'mock.key'),'-out',str(bundle/'mock.crt'),'-subj','/CN=e2e.r2.cloudflarestorage.com','-addext','subjectAltName=DNS:e2e.r2.cloudflarestorage.com,DNS:testbucket.e2e.r2.cloudflarestorage.com')
        stage('freeze-images-and-recipe')
        images = {role:pin_image(tag) for role,tag in {'worker':'zero-devops-e2e-worker:local','db':'postgres:17-alpine','rabbit':'rabbitmq:3-management-alpine'}.items()}
        images['appdb'] = images['db']
        reference_command = 'fastapi run --workers 4'
        recipe = app_path/'Dockerfile'
        assert sha(recipe)=='f6399822e16d9bfa11d2d1879db88741d60eef69c682f779e1b2aa101d76e7ef'
        shutil.copyfile(recipe,out/'reference.Dockerfile')
        app_secret,db_secret,password = (secrets.token_hex(20) for _ in range(3))
        state.update(images=images, capacity=capacity, worker_sha256=sha(bundle/'worker-engine'), harness_sha256={Path(m.__file__).name:sha(Path(m.__file__)) for m in [common,previous]}, pilot_sha256=sha(Path(__file__)), selected_manifest_sha256=MANIFEST_HASH, root_lock_sha256=LOCK_HASH, reference_recipe_sha256=sha(recipe), reference_scope='backend probes only; unmodified upstream image includes Bun frontend artifact build and moving base tags', lock_policy='reference uses root uv frozen workspace with upstream Dockerfile; worker receives unmodified backend app', publication='local stub only', limits={'builder_cpus':1.5,'builder_memory_bytes':2*1024**3,'builder_pids':512,'reference_build_seconds':1200})
        stage('provision-campaign-only-services')
        docker('network','create','--label',f'{LABEL}={PREFIX}',NAMES['net'])
        for name in ['storage','pgdata','appdata','rabbitdata']:
            docker('volume','create','--label',f'{LABEL}={PREFIX}',NAMES[name])
        def start(role, *args, **kw):
            created.append(role)
            service(role, images[role if role != 'mock' else 'worker'], *args, **kw)
        start('db','0.5','256m','128','-v',NAMES['pgdata']+':/var/lib/postgresql/data','-e','POSTGRES_USER=e2e','-e','POSTGRES_PASSWORD=e2e','-e','POSTGRES_DB=e2e')
        start('appdb','0.5','256m','128','-v',NAMES['appdata']+':/var/lib/postgresql/data','-e','POSTGRES_USER=pilot','-e','POSTGRES_PASSWORD='+db_secret,'-e','POSTGRES_DB=reference')
        start('rabbit','0.5','384m','256','-v',NAMES['rabbitdata']+':/var/lib/rabbitmq','-p','127.0.0.1::15672','-e','RABBITMQ_DEFAULT_USER=e2e','-e','RABBITMQ_DEFAULT_PASS=e2e','-e','RABBITMQ_SERVER_ADDITIONAL_ERL_ARGS=+S 1:1')
        start('mock','0.25','128m','64','--network-alias','e2e.r2.cloudflarestorage.com','--network-alias','testbucket.e2e.r2.cloudflarestorage.com','-v',str(bundle)+':/e2e:ro','-v',str(ROOT/'scripts/worker-build-campaign')+':/harness:ro','--entrypoint','python3',command=('/harness/mock_services.py',))
        until(lambda: dex('db','pg_isready','-h','127.0.0.1','-U','e2e',check=False).returncode == 0)
        until(lambda: dex('mock','curl','-fsS','--max-time','3','-u','e2e:e2e','http://rabbit:15672/api/overview',check=False).returncode == 0)
        until(lambda: dex('appdb','pg_isready','-h','127.0.0.1','-U','pilot','-d','reference',check=False).returncode == 0)
        dex('appdb','createdb','-U','pilot','worker')
        migration = (ROOT/'worker-server/migrations/20260701203442_create_worker_deployments_table.sql').read_text().split('-- +goose Down')[0]
        docker('exec','-i',NAMES['db'],'psql','-v','ON_ERROR_STOP=1','-U','e2e','-d','e2e',input=migration)
        env = json.loads((ROOT/'docs/worker-build-fixes-2026-11-16/environment.json').read_text())['worker_environment']
        env.update(DATABASE_HOST='db',RABBITMQ_CONNECTION_STRING='amqp://e2e:e2e@rabbit:5672/',ALLOW_UNLOCKED_NPM_INSTALL='true')
        if env['SERVER_INTERNAL_URL'] != 'http://mock:8000' or env['CLOUDFLARE_ACCOUNT_ID'] != 'e2e' or env['CLOUDFLARE_BUCKET_NAME'] != 'testbucket':
            raise RuntimeError('refuse non-stub configuration')
        start('worker','1.5','2g','512','--privileged','-v',NAMES['storage']+':/var/lib/containers','-v',str(bundle)+':/e2e:ro','-v',str(ROOT/'worker-server/templates')+':/app/templates:ro','--entrypoint','/e2e/worker-engine',*[arg for k,v in env.items() for arg in ('-e',f'{k}={v}')])
        until(lambda: 'Worker consumer registered' in docker('logs',NAMES['worker']))
        assert json.loads(dex('worker','buildah','images','--json')) == []
        stage('reference-build-and-runtime')
        tag = 'localhost/zdbroad-pilot3-fastapi-reference:trial'
        rc = logged(out/'reference-build.log','docker','exec',NAMES['worker'],'timeout','--kill-after=10s','1200s','buildah','bud','-f','/e2e/app/backend/Dockerfile','-t',tag,'/e2e/app',timeout=1230)
        state['reference'] = {'image_built':rc==0,'exit_code':rc}
        if rc==0:
            (out/'reference-image.json').write_text(dex('worker','buildah','inspect','--type','image',tag)+'\n')
            name='zdbroad-reference-runtime'; dex('worker','buildah','from','--name',name,tag)
            state['reference']['runtime'] = runtime_probe(out,name,reference_command,'reference',app_secret,db_secret,password)
            dex('worker','buildah','rm',name)
        stage('worker-build-and-runtime')
        common.WORKER,common.MOCK,common.DB,common.RABBIT = (NAMES[x] for x in ('worker','mock','db','rabbit'))
        common.RESULTS,common.ROOT,common.VHOST = out,out,'/'
        common.guard = lambda: None  # Disk guard before dispatch; no blocking storage scan during builds.
        common.resource = lambda _: {}  # Do not put optional telemetry on the dispatch path.
        common.probe_runtime = lambda container,name,command,case,label: runtime_probe(out,name,command,label,app_secret,db_secret,password)
        # The old function stores the generated job ID only after polling. Checkpoint it before publication.
        real_uuid = previous.uuid
        class CheckpointUUID:
            @staticmethod
            def uuid4():
                value = real_uuid.uuid4()
                if 'job_id' not in state:
                    state['job_id'] = str(value); save()
                return value
        previous.uuid = CheckpointUUID
        old_dex = common.dex
        def retain(container, args, timeout=30, check=False):
            if args[:2] in (['buildah','rmi'], ['buildah','rm']) and (args[1]=='rmi' or '--all' in args):
                return 0, ''  # preserve trial and intermediate images for evidence
            return old_dex(container,args,timeout,check)
        common.dex = retain
        previous.worker(common,CASE,state)
        state['complete'] = True
        stage('pilot-case-complete')
    except Exception as exc:
        state['harness_error'] = f'{type(exc).__name__}: {exc}'
        raise
    finally:
        state['finished_at'] = stamp()
        state['resource_stop_errors'] = []
        for role in reversed(created):
            name = NAMES[role]
            try:
                p = docker('inspect','--format','{{index .Config.Labels "'+LABEL+'"}}',name,check=False)
                if p.returncode or p.stdout.strip() != PREFIX:
                    continue
                (out/f'final-{role}.log').write_text(docker('logs',name,check=False).stdout)
                docker('stop','-t','10',name,timeout=30)
            except Exception as exc:
                state['resource_stop_errors'].append(f'{role}: {exc}')
        state['preservation'] = 'containers, images, network and volumes left in place; no global cleanup'
        save()


if __name__ == '__main__':
    main()
