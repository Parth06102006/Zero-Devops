#!/usr/bin/env python3
"""Provision/stop ONLY zdfix-* services. No production services or publication."""
import argparse, datetime, hashlib, json, os, pathlib, shutil, subprocess, time

REPO = pathlib.Path(__file__).resolve().parents[2]
OUT = REPO / 'docs/worker-build-fixes-2026-11-16'
TEMP = pathlib.Path('/tmp/zd-fix-builds-final-services')
IMAGE = 'zero-devops-e2e-worker:local'
LABEL = 'zero-devops-build-fixes=20261116'
CONTAINERS = ['zdfix-worker', 'zdfix-mock', 'zdfix-db', 'zdfix-rabbit', 'zdfix-cold', 'zdfix-native-tests']
VOLUMES = ['zdfix-worker-storage', 'zdfix-db-data', 'zdfix-rabbit-data', 'zdfix-cold-storage', 'zdfix-native-storage']

def run(args, **kwargs):
    result = subprocess.run(args, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, **kwargs)
    if result.returncode:
        raise RuntimeError(repr(args) + '\n' + result.stdout[-6000:])
    return result.stdout.strip()

def docker(*args):
    return run(['docker', *args])

def wait_for(command, attempts=90):
    for _ in range(attempts):
        result = subprocess.run(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        if result.returncode == 0:
            return
        time.sleep(1)
    raise RuntimeError('service readiness timed out: ' + repr(command))

def setup():
    OUT.mkdir(parents=True, exist_ok=True)
    if (OUT / 'environment.json').exists():
        raise RuntimeError('environment evidence already exists; inspect rather than overwrite')
    # Never adopt a container/volume/network from an unrelated run.
    for kind, names in [('container', CONTAINERS), ('volume', VOLUMES), ('network', ['zdfix-network'])]:
        for name in names:
            if subprocess.run(['docker', kind, 'inspect', name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0:
                raise RuntimeError('resource already exists: ' + name)
    TEMP.mkdir(mode=0o700, exist_ok=True)
    if (TEMP/'mock.key').exists():
        raise RuntimeError('temporary certificate already exists; inspect before reuse')
    run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '2',
         '-keyout', str(TEMP/'mock.key'), '-out', str(TEMP/'mock.crt'),
         '-subj', '/CN=e2e.r2.cloudflarestorage.com',
         '-addext', 'subjectAltName=DNS:e2e.r2.cloudflarestorage.com,DNS:testbucket.e2e.r2.cloudflarestorage.com'])
    os.chmod(TEMP/'mock.key', 0o600)
    docker('network', 'create', '--label', LABEL, 'zdfix-network')
    for volume in VOLUMES:
        docker('volume', 'create', '--label', LABEL, volume)
    common = ['run', '-d', '--label', LABEL, '--network', 'zdfix-network']
    docker(*common, '--name', 'zdfix-db', '--network-alias', 'postgres', '--cpus', '0.5', '--memory', '256m',
           '--pids-limit', '128', '-v', 'zdfix-db-data:/var/lib/postgresql/data',
           '-e', 'POSTGRES_USER=e2e', '-e', 'POSTGRES_PASSWORD=e2e', '-e', 'POSTGRES_DB=e2e', 'postgres:17-alpine')
    docker(*common, '--name', 'zdfix-rabbit', '--network-alias', 'rabbit', '--cpus', '0.5', '--memory', '384m',
           '--pids-limit', '256', '-v', 'zdfix-rabbit-data:/var/lib/rabbitmq', '-p', '127.0.0.1::15672',
           '-e', 'RABBITMQ_DEFAULT_USER=e2e', '-e', 'RABBITMQ_DEFAULT_PASS=e2e',
           '-e', 'RABBITMQ_SERVER_ADDITIONAL_ERL_ARGS=+S 1:1', 'rabbitmq:3-management-alpine')
    docker(*common, '--name', 'zdfix-mock', '--network-alias', 'mock', '--network-alias', 'e2e.r2.cloudflarestorage.com',
           '--network-alias','testbucket.e2e.r2.cloudflarestorage.com',
           '--cpus', '0.5', '--memory', '128m', '--pids-limit', '64',
           '-v', str(TEMP)+':/e2e:ro', '-v', str(REPO/'scripts/worker-build-campaign')+':/harness:ro',
           '--entrypoint', 'python3', IMAGE, '/harness/mock_services.py')
    wait_for(['docker','exec','zdfix-db','pg_isready','-U','e2e','-d','e2e'])
    wait_for(['docker','exec','zdfix-mock','python3','-c','import socket; socket.create_connection(("rabbit",5672),2).close()'])
    wait_for(['docker','exec','zdfix-mock','curl','--fail','-sS','-u','e2e:e2e','http://rabbit:15672/api/overview'])
    migration=(REPO/'worker-server/migrations/20260701203442_create_worker_deployments_table.sql').read_text().split('-- +goose Down')[0]
    run(['docker','exec','-i','zdfix-db','psql','-v','ON_ERROR_STOP=1','-U','e2e','-d','e2e'], input=migration)
    env = {
        'DATABASE_HOST':'postgres','DATABASE_PORT':'5432','DATABASE_USER':'e2e','DATABASE_PASS':'e2e','DATABASE_NAME':'e2e',
        'RABBITMQ_CONNECTION_STRING':'amqp://e2e:e2e@rabbit:5672/', 'SERVER_INTERNAL_URL':'http://mock:8000',
        'WORKER_INTERNAL_AUTH_TOKEN':'test-only','CLOUDFLARE_ACCOUNT_ID':'e2e','CLOUDFLARE_BUCKET_NAME':'testbucket',
        'CLOUDFLARE_ACCESS_KEY_ID':'test','CLOUDFLARE_ACCESS_KEY_SECRET':'test','SSL_CERT_FILE':'/e2e/mock.crt',
        'APP_ENV':'development','MAX_RETRIES_COUNT':'1','GOMAXPROCS':'1','BUILDAH_ISOLATION':'chroot',
        'ALLOW_UNLOCKED_NPM_INSTALL':'true',
    }
    envargs=[item for k,v in env.items() for item in ['-e',k+'='+v]]
    shutil.copy2('/tmp/zd-fix-builds-worker', TEMP/'worker-engine')
    docker(*common, '--name','zdfix-worker','--privileged','--cpus','1.5','--memory','2g','--pids-limit','512',
           '-v','zdfix-worker-storage:/var/lib/containers','-v',str(TEMP)+':/e2e:ro',
           '--entrypoint','/e2e/worker-engine',
           '-v',str(REPO/'worker-server/templates')+':/app/templates:ro', *envargs, IMAGE)
    write_metadata(env)
    print('isolated services ready',flush=True)

def write_metadata(env):
    (OUT/'initial-worker-images.json').write_text(docker('exec','zdfix-worker','buildah','images','--json')+'\n')
    metadata={'created_at':datetime.datetime.now(datetime.timezone.utc).isoformat(), 'branch':run(['git','branch','--show-current']),
              'source_commit':run(['git','rev-parse','HEAD']), 'binary_sha256':hashlib.sha256(pathlib.Path('/tmp/zd-fix-builds-worker').read_bytes()).hexdigest(),
              'image':docker('image','inspect',IMAGE,'--format','{{.Id}}'), 'containers':CONTAINERS,'volumes':VOLUMES,
              'limits':{'cpus':1.5,'memory_bytes':2*1024**3,'pids':512,'sequential':True},
              'worker_environment':env, 'qualification':'57 preserved baseline reference passes; worker builds and smoke probes rerun at immutable SHAs; no reference rebuild',
              'publication':'fake upload sink only; no real deployment or publication'}
    (OUT/'environment.json').write_text(json.dumps(metadata,indent=2))
    (OUT/'tested-source.diff').write_text(run(['git','diff','HEAD']))

def recover_startup():
    if (OUT/'environment.json').exists():raise RuntimeError('environment already initialized')
    label=docker('inspect','zdfix-worker','--format','{{index .Config.Labels "zero-devops-build-fixes"}}')
    if label!='20261116':raise RuntimeError('worker is not owned by this run')
    wait_for(['docker','exec','zdfix-mock','python3','-c','import socket; socket.create_connection(("rabbit",5672),2).close()'])
    env=dict(item.split('=',1) for item in json.loads(docker('inspect','zdfix-worker','--format','{{json .Config.Env}}')))
    if docker('inspect','zdfix-worker','--format','{{.State.Running}}')=='true':raise RuntimeError('worker is already running; inspect before replacing')
    # Docker Desktop retained the old inode for the initial single-file bind
    # after go build replaced it. Mount the directory and execute its copy.
    docker('rm','-v','zdfix-worker')
    shutil.copy2('/tmp/zd-fix-builds-worker',TEMP/'worker-engine')
    envargs=[item for k,v in env.items() for item in ['-e',k+'='+v]]
    docker('run','-d','--label',LABEL,'--network','zdfix-network','--name','zdfix-worker',
           '--privileged','--cpus','1.5','--memory','2g','--pids-limit','512',
           '-v','zdfix-worker-storage:/var/lib/containers','-v',str(TEMP)+':/e2e:ro',
           '--entrypoint','/e2e/worker-engine','-v',str(REPO/'worker-server/templates')+':/app/templates:ro',*envargs,IMAGE)
    write_metadata(env)
    print('startup recovered after RabbitMQ listener readiness',flush=True)

def cold_pulls():
    if (OUT/'cold-pulls.json').exists():
        raise RuntimeError('cold-pull evidence exists; do not overwrite')
    docker('run','-d','--label',LABEL,'--name','zdfix-cold','--privileged','--cpus','1.5','--memory','2g','--pids-limit','512',
           '-e','BUILDAH_ISOLATION=chroot','-v','zdfix-cold-storage:/var/lib/containers','--entrypoint','sleep',IMAGE,'infinity')
    initial=json.loads(docker('exec','zdfix-cold','buildah','images','--json'))
    if initial:raise RuntimeError('cold storage unexpectedly contains images')
    images=set()
    for template in (REPO/'worker-server/templates').rglob('*.tmpl'):
        for line in template.read_text().splitlines():
            if line.startswith('FROM '):
                image=line.split()[1]
                if '__GO_VERSION__' in image:continue
                # Relevant cold pulls, including both stages of each affected stack.
                if any('/'+x+':' in image for x in ['golang','eclipse-temurin','composer','php','rust','alpine','node']):images.add(image)
    # Exact requirements of the pinned Go cases, not a guessed moving default.
    images.add('docker.io/library/golang:1.26.0-alpine')
    records=[]
    for i,image in enumerate(sorted(images)):
        print('COLD PULL',image,flush=True)
        started=time.monotonic()
        result=subprocess.run(['docker','exec','zdfix-cold','timeout','--kill-after=10s','360s','buildah','pull',image],text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
        log=OUT/('cold-pull-'+str(i)+'.log');log.write_text(result.stdout)
        records.append({'image':image,'exit_code':result.returncode,'seconds':round(time.monotonic()-started,3),'log':log.name})
        (OUT/'cold-pulls.json').write_text(json.dumps({'initial_images':initial,'isolated_storage':'zdfix-cold-storage','pulls':records},indent=2))
    print('cold pulls complete',flush=True)

def fix_upload_endpoint():
    if (OUT/'results').exists() and any((OUT/'results').glob('*.json')):raise RuntimeError('application evidence already exists; inspect before service changes')
    for name in ['zdfix-mock','zdfix-worker']:
        if docker('inspect',name,'--format','{{index .Config.Labels "zero-devops-build-fixes"}}')!='20261116':raise RuntimeError('unowned service')
    run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','2',
         '-keyout',str(TEMP/'mock.key'),'-out',str(TEMP/'mock.crt'),'-subj','/CN=e2e.r2.cloudflarestorage.com',
         '-addext','subjectAltName=DNS:e2e.r2.cloudflarestorage.com,DNS:testbucket.e2e.r2.cloudflarestorage.com'])
    os.chmod(TEMP/'mock.key',0o600)
    docker('network','disconnect','zdfix-network','zdfix-mock')
    docker('network','connect','--alias','mock','--alias','e2e.r2.cloudflarestorage.com','--alias','testbucket.e2e.r2.cloudflarestorage.com','zdfix-network','zdfix-mock')
    docker('restart','zdfix-mock');docker('restart','zdfix-worker')
    print('fake upload endpoint aliases and certificate repaired before application jobs',flush=True)

def stop():
    removed=[]
    anonymous=[]
    for name in CONTAINERS:
        result=subprocess.run(['docker','inspect','--format','{{index .Config.Labels "zero-devops-build-fixes"}}',name],text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
        if result.returncode:continue
        if result.stdout.strip()!='20261116':raise RuntimeError('refusing unrelated resource '+name)
        mounts=json.loads(docker('inspect',name,'--format','{{json .Mounts}}'))
        anonymous.extend(m['Name'] for m in mounts if m['Type']=='volume' and m['Name'] not in VOLUMES)
        docker('rm','-f','-v',name);removed.append(name)
    for name in VOLUMES:
        result=subprocess.run(['docker','volume','inspect','--format','{{index .Labels "zero-devops-build-fixes"}}',name],text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
        if result.returncode:continue
        if result.stdout.strip()!='20261116':raise RuntimeError('refusing unrelated volume '+name)
        docker('volume','rm',name)
    result=subprocess.run(['docker','network','inspect','--format','{{index .Labels "zero-devops-build-fixes"}}','zdfix-network'],text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    if result.returncode==0 and result.stdout.strip()=='20261116':docker('network','rm','zdfix-network')
    (OUT/'cleanup.json').write_text(json.dumps({'containers_removed':removed,'named_volumes_removed':VOLUMES,'anonymous_volumes_removed':anonymous,'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'no_global_prune':True},indent=2))

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('action',choices=['setup','recover-startup','fix-upload-endpoint','cold-pulls','stop']);args=parser.parse_args()
    {'setup':setup,'recover-startup':recover_startup,'fix-upload-endpoint':fix_upload_endpoint,'cold-pulls':cold_pulls,'stop':stop}[args.action]()
