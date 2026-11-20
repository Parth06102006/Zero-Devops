#!/usr/bin/env python3
"""Single-lane broad campaign pilot; starts with the Nest Bull sample."""
import sys
import datetime as dt
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
PREFIX = 'zdbroad-pilot1'
LABEL = 'zero-devops-broad-run'
NAMES = {k: f'{PREFIX}-{k}' for k in ['worker', 'mock', 'db', 'rabbit', 'redis', 'net', 'storage', 'pgdata', 'rabbitdata', 'redisdata']}
CASE = dict(name='nest-bull-redis', repo='nestjs/nest', sha='bbbfaf1765308826a581a75424a11d0f3e59521a', path='sample/26-queues', kind='node', port=3000, env={'REDIS_HOST': 'redis', 'REDIS_PORT': '6379'})
MANIFEST_HASH = '8ff4661128ff712c1b5254f85c73401ac65bea85de3b976bb1966bb036d1cee7'


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


def queue_probe(out, name, command, label):
    """Readiness is a port check; exactly one mutating HTTP request per origin."""
    port = 3000
    assert not tcp('127.0.0.1', port), 'stale listener before start'
    ip = docker('inspect', '-f', '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}', NAMES['worker'])
    log = out / f'{label}-runtime.log'
    result = {'port_closed_before_start': True, 'ready': False, 'external_pass': False, 'checks': []}
    def completed():
        return int(dex('redis', 'redis-cli', 'ZCARD', 'bull:audio:completed'))
    before = completed()
    with log.open('x') as stdout:
        p = sp.Popen(['docker','exec',NAMES['worker'],'timeout','--signal=TERM','--kill-after=5s','38s','buildah','run',name,'--','sh','-c','exec '+command], stdout=stdout, stderr=sp.STDOUT)
        try:
            for _ in range(20):
                if p.poll() is not None:
                    break
                if tcp('127.0.0.1', port):
                    result['listener_started'] = True
                    break
                time.sleep(.5)
            if result.get('listener_started'):
                for origin, addr in [('worker', '127.0.0.1'), ('mock', ip)]:
                    prior = completed()
                    response = dex(origin, 'curl', '-sS', '--max-time', '3', '-X', 'POST', '-w', '\n%{http_code}', f'http://{addr}:{port}/audio/transcode', timeout=6, check=False)
                    status = response.stdout.strip().splitlines()[-1:] or ['']
                    http = response.returncode == 0 and status == ['201']
                    job = False
                    if http:
                        try:
                            until(lambda: completed() > prior, 5)
                            job = completed() == prior + 1
                        except TimeoutError:
                            pass
                    result['checks'].append({'origin': origin, 'http_201': http, 'status': status[0], 'queue_completed_one': job})
                result['ready'] = result['checks'][0]['http_201']
                result['external_pass'] = result['checks'][1]['http_201']
        finally:
            # In-container timeout bounds the child even if the host loses the docker exec connection.
            p.wait(timeout=50)
    result['port_closed_after_stop'] = not tcp('127.0.0.1', port)
    result['completed_before'] = before
    result['completed_after'] = completed()
    result['processor_log_observed'] = 'Transcoding completed' in log.read_text()
    result['functional_pass'] = len(result['checks']) == 2 and all(x['queue_completed_one'] for x in result['checks']) and result['processor_log_observed'] and result['port_closed_after_stop']
    return result


def main():
    out = EVIDENCE / 'pilot-01'
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
        temp = Path(tempfile.mkdtemp(prefix='zdbroad-pilot1-', dir=str(Path.home())))
        state['local_work_root'] = str(temp)
        bundle = temp / 'bundle'; bundle.mkdir(mode=0o700)
        source = temp / 'source'; source.mkdir()
        cmd('git','-C',str(source),'init','-q')
        git_url = f'https://github.com/{CASE["repo"]}.git'
        cmd('git','-C',str(source),'fetch','--depth=1',git_url,CASE['sha'], timeout=180)
        cmd('git','-C',str(source),'checkout','--detach',CASE['sha'])
        assert cmd('git','-C',str(source),'rev-parse','HEAD') == CASE['sha']
        app_path = source / CASE['path']
        assert sha(app_path / 'package.json') == MANIFEST_HASH
        assert not any((app_path / name).exists() for name in ['package-lock.json','npm-shrinkwrap.json','pnpm-lock.yaml','yarn.lock','bun.lock','bun.lockb'])
        cmd('cp','-a',str(app_path),str(bundle/'app'))
        cmd('env','CGO_ENABLED=0','go','build','-buildvcs=false','-trimpath','-o',str(bundle/'worker-engine'),'./cmd/worker-server',cwd=ROOT/'worker-server',timeout=240)
        cmd('openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','3','-keyout',str(bundle/'mock.key'),'-out',str(bundle/'mock.crt'),'-subj','/CN=e2e.r2.cloudflarestorage.com','-addext','subjectAltName=DNS:e2e.r2.cloudflarestorage.com,DNS:testbucket.e2e.r2.cloudflarestorage.com')
        stage('freeze-images-and-recipe')
        images = {role:pin_image(tag) for role,tag in {'worker':'zero-devops-e2e-worker:local','db':'postgres:17-alpine','rabbit':'rabbitmq:3-management-alpine','redis':'redis:7-alpine','node':'node:24-alpine'}.items()}
        node_img = docker('image','inspect',images['node'],'--format','{{index .RepoDigests 0}}')
        recipe = f'FROM {node_img}\nWORKDIR /app\nCOPY . .\nRUN node --version && npm --version && npm install --ignore-scripts --no-package-lock && npm ls --depth=0 && npm run build\nCMD ["node", "dist/main"]\n'
        (out/'reference.Dockerfile').write_text(recipe); (bundle/'reference.Dockerfile').write_text(recipe)
        state.update(images=images, node_reference_digest=node_img, capacity=capacity, worker_sha256=sha(bundle/'worker-engine'), harness_sha256={Path(m.__file__).name:sha(Path(m.__file__)) for m in [common,previous]}, pilot_sha256=sha(Path(__file__)), selected_manifest_sha256=MANIFEST_HASH, lock_policy='explicit unlocked npm in both lanes; dependency trees may differ; install scripts disabled', publication='local stub only', limits={'builder_cpus':1.5,'builder_memory_bytes':2*1024**3,'builder_pids':512,'reference_build_seconds':1200})
        stage('provision-campaign-only-services')
        docker('network','create','--label',f'{LABEL}={PREFIX}',NAMES['net'])
        for name in ['storage','pgdata','rabbitdata','redisdata']:
            docker('volume','create','--label',f'{LABEL}={PREFIX}',NAMES[name])
        def start(role, *args, **kw):
            created.append(role)
            service(role, images[role if role != 'mock' else 'worker'], *args, **kw)
        start('db','0.5','256m','128','-v',NAMES['pgdata']+':/var/lib/postgresql/data','-e','POSTGRES_USER=e2e','-e','POSTGRES_PASSWORD=e2e','-e','POSTGRES_DB=e2e')
        start('rabbit','0.5','384m','256','-v',NAMES['rabbitdata']+':/var/lib/rabbitmq','-p','127.0.0.1::15672','-e','RABBITMQ_DEFAULT_USER=e2e','-e','RABBITMQ_DEFAULT_PASS=e2e','-e','RABBITMQ_SERVER_ADDITIONAL_ERL_ARGS=+S 1:1')
        start('redis','0.25','128m','64','-v',NAMES['redisdata']+':/data')
        start('mock','0.25','128m','64','--network-alias','e2e.r2.cloudflarestorage.com','--network-alias','testbucket.e2e.r2.cloudflarestorage.com','-v',str(bundle)+':/e2e:ro','-v',str(ROOT/'scripts/worker-build-campaign')+':/harness:ro','--entrypoint','python3',command=('/harness/mock_services.py',))
        until(lambda: dex('db','pg_isready','-h','127.0.0.1','-U','e2e',check=False).returncode == 0)
        until(lambda: dex('mock','curl','-fsS','--max-time','3','-u','e2e:e2e','http://rabbit:15672/api/overview',check=False).returncode == 0)
        until(lambda: dex('redis','redis-cli','PING',check=False).stdout.strip() == 'PONG')
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
        tag = 'localhost/zdbroad-pilot1-nest-reference:trial'
        rc = logged(out/'reference-build.log','docker','exec',NAMES['worker'],'timeout','--kill-after=10s','1200s','buildah','bud','-f','/e2e/reference.Dockerfile','-t',tag,'/e2e/app',timeout=1230)
        state['reference'] = {'image_built':rc==0,'exit_code':rc}
        if rc==0:
            (out/'reference-image.json').write_text(dex('worker','buildah','inspect','--type','image',tag)+'\n')
            name='zdbroad-reference-runtime'; dex('worker','buildah','from','--name',name,tag)
            for k,v in CASE['env'].items(): dex('worker','buildah','config','--env',f'{k}={v}',name)
            state['reference']['runtime'] = queue_probe(out,name,'node dist/main','reference')
            dex('worker','buildah','rm',name)
        stage('worker-build-and-runtime')
        common.WORKER,common.MOCK,common.DB,common.RABBIT = (NAMES[x] for x in ('worker','mock','db','rabbit'))
        common.RESULTS,common.ROOT,common.VHOST = out,out,'/'
        common.guard = lambda: None  # Disk guard before dispatch; no blocking storage scan during builds.
        common.resource = lambda _: {}  # Do not put optional telemetry on the dispatch path.
        common.probe_runtime = lambda container,name,command,case,label: queue_probe(out,name,command,label)
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
