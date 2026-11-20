#!/usr/bin/env python3
"""Reconcile the user-paused run before restarting its isolated workers."""
import base64, datetime, hashlib, json, os, pathlib, re, shutil, subprocess, time, urllib.parse, urllib.request, uuid
import environment as env

ROOT=env.OUT

def main():
    stamp=datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')
    out=ROOT/('resume-'+stamp);out.mkdir()
    source=json.loads((ROOT/'execution-source.json').read_text())
    binary=pathlib.Path('/tmp/zd-fix-builds-worker')
    if hashlib.sha256(binary.read_bytes()).hexdigest()!=source['worker_binary_sha256']:raise RuntimeError('rebuilt binary differs from tested source; inspect before restarting')
    for path,checksum in source['worker_source_sha256'].items():
        if hashlib.sha256((env.REPO/path).read_bytes()).hexdigest()!=checksum:raise RuntimeError('product source drift: '+path)
    for name in ['parallel-state.json','progress.json','parallel.log','pause-state.json']:
        if (ROOT/name).exists():shutil.copy2(ROOT/name,out/name)
    checksums={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in (ROOT/'results').glob('*.json')}
    (out/'preserved-results-sha256.json').write_text(json.dumps(checksums,indent=2))
    state={'at':stamp,'source_commit':source['product_commit'],'binary_sha256':source['worker_binary_sha256'],
           'archived_containers':[],'interrupted_cases':{},'queues':{},'snapshot_images':[], 'completed':False}
    def save():
        (out/'reconciliation.json').write_text(json.dumps(state,indent=2))
        (ROOT/'latest-resume.json').write_text(json.dumps({'directory':str(out.relative_to(ROOT)),'completed':state['completed']},indent=2))
    def owned(name):
        info=json.loads(env.docker('inspect',name))[0]
        if info['Config']['Labels'].get('zero-devops-build-fixes')!='20261116':raise RuntimeError('refusing unowned container '+name)
        return info
    def archive(name):
        info=owned(name)
        if info['State']['Running']:raise RuntimeError('expected paused container: '+name)
        (out/(name+'-inspect.json')).write_text(json.dumps(info,indent=2))
        (out/(name+'.log')).write_text(env.docker('logs',name))
        archived=name+'-paused-'+stamp.lower()
        env.docker('rename',name,archived);state['archived_containers'].append(archived);save()
        return info
    save()
    for name in ['zdfix-db','zdfix-rabbit']:
        owned(name)
        env.docker('start',name)
    env.wait_for(['docker','exec','zdfix-db','pg_isready','-U','e2e','-d','e2e'])
    env.TEMP.mkdir(mode=0o700,exist_ok=True)
    shutil.copy2(binary,env.TEMP/'worker-engine')
    if (env.TEMP/'mock.key').exists():raise RuntimeError('existing certificate requires inspection; do not overwrite')
    env.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','3',
             '-keyout',str(env.TEMP/'mock.key'),'-out',str(env.TEMP/'mock.crt'),
             '-subj','/CN=e2e.r2.cloudflarestorage.com',
             '-addext','subjectAltName=DNS:e2e.r2.cloudflarestorage.com,DNS:testbucket.e2e.r2.cloudflarestorage.com'])
    os.chmod(env.TEMP/'mock.key',0o600)
    workers={}
    for lane,prefix in [(1,'zdfix'),(2,'zdfix2')]:
        old=archive(prefix+'-mock')
        env.docker('run','-d','--label',env.LABEL,'--name',prefix+'-mock','--network',prefix+'-network',
                   '--network-alias','mock','--network-alias','e2e.r2.cloudflarestorage.com','--network-alias','testbucket.e2e.r2.cloudflarestorage.com',
                   '--cpus','0.5','--memory','128m','--pids-limit','64','-v',str(env.TEMP)+':/e2e:ro',
                   '-v',str(env.REPO/'scripts/worker-build-campaign')+':/harness:ro','--entrypoint','python3',old['Image'],'/harness/mock_services.py')
        old=archive(prefix+'-worker');workers[lane]=old
        if old['Image']!=json.loads((ROOT/'environment.json').read_text())['image']:raise RuntimeError('test runtime image changed')
    env.wait_for(['docker','exec','zdfix-mock','curl','--fail','-sS','-u','e2e:e2e','http://rabbit:15672/api/overview'])
    port=env.docker('port','zdfix-rabbit','15672/tcp').rsplit(':',1)[1]
    def api(path,data=None):
        headers={'Authorization':'Basic '+base64.b64encode(b'e2e:e2e').decode(),'Content-Type':'application/json'}
        req=urllib.request.Request('http://127.0.0.1:'+port+path,data=None if data is None else json.dumps(data).encode(),headers=headers,method='GET' if data is None else 'POST')
        with urllib.request.urlopen(req,timeout=20) as response:return json.load(response)
    interrupted=[]
    paused=json.loads((ROOT/'pause-state.json').read_text())
    for lane,prefix in [(1,'zdfix'),(2,'zdfix2')]:
        log=(ROOT/('paused-lane'+str(lane)+'-worker.log')).read_text()
        ident=str(uuid.UUID(re.findall(r'"deployment_id":\s*"([^"]+)"',log)[-1]))
        interrupted.append(ident)
        name=paused['interrupted_cases'][str(lane)]
        state['interrupted_cases'][name]={'deployment_id':ident,'reason':'interrupted by explicit user pause; excluded from measured failures'}
        vhost='/' if lane==1 else 'lane2';queue_path='/api/queues/'+urllib.parse.quote(vhost,safe='')+'/deploy.jobs'
        info=api(queue_path);state['queues'][vhost]={'before':info};save()
        if info['consumers'] or info['messages_unacknowledged']:raise RuntimeError('queue is not idle: '+vhost)
        preview=api(queue_path+'/get',{'count':100,'ackmode':'ack_requeue_true','encoding':'auto','truncate':100000})
        if any(json.loads(msg['payload'])['deployment_id']!=ident for msg in preview):raise RuntimeError('unexpected queued job; preserve it for inspection')
        state['queues'][vhost]['interrupted_messages']=preview;save()
        drained=api(queue_path+'/get',{'count':len(preview),'ackmode':'ack_requeue_false','encoding':'auto','truncate':100000}) if preview else []
        if len(drained)!=len(preview):raise RuntimeError('queue changed during reconciliation')
        state['queues'][vhost]['after']=api(queue_path);save()
    sql="SELECT id,status,error_message FROM deployments ORDER BY created_at;"
    (out/'rows-before.csv').write_text(env.docker('exec','zdfix-db','psql','-U','e2e','-d','e2e','-Ac',sql))
    ids=','.join("'"+ident+"'::uuid" for ident in interrupted)
    query="UPDATE deployments SET status='canceled',error_message='Campaign interrupted by explicit user pause; excluded from measured application failures',finished_at=now() WHERE status='building' AND id IN ("+ids+") RETURNING id,status;"
    (out/'pause-reconciliation.sql').write_text(query+'\n')
    (out/'rows-updated.csv').write_text(env.docker('exec','zdfix-db','psql','-v','ON_ERROR_STOP=1','-U','e2e','-d','e2e','-Ac',query))
    (out/'rows-after.csv').write_text(env.docker('exec','zdfix-db','psql','-U','e2e','-d','e2e','-Ac',sql))
    def start(lane,maintenance):
        prefix='zdfix' if lane==1 else 'zdfix2';old=workers[lane]
        args=['run','-d','--label',env.LABEL,'--name',prefix+'-worker','--network',prefix+'-network','--privileged',
              '--cpus','1.5','--memory','2g','--pids-limit','512','-v',prefix+'-worker-storage:/var/lib/containers',
              '-v',str(env.TEMP)+':/e2e:ro','-v',str(env.REPO/'worker-server/templates')+':/app/templates:ro']
        args += [item for value in old['Config']['Env'] for item in ['-e',value]]
        args += ['--entrypoint','sleep' if maintenance else '/e2e/worker-engine',old['Image']]
        if maintenance:args += ['infinity']
        env.docker(*args)
    for lane,prefix in [(1,'zdfix'),(2,'zdfix2')]:
        volume=json.loads(env.docker('volume','inspect',prefix+'-worker-storage'))[0]
        if volume['Labels'].get('zero-devops-build-fixes')!='20261116':raise RuntimeError('unowned build store')
        start(lane,True)
        containers=json.loads(env.docker('exec',prefix+'-worker','buildah','containers','--json'))
        (out/(prefix+'-build-containers.json')).write_text(json.dumps(containers,indent=2))
        for container in containers:
            ident=container.get('containerid',container.get('id',''))
            if not re.fullmatch('[a-f0-9]{12,64}',ident):raise RuntimeError('unexpected Buildah container ID')
            tag='localhost/zdfix-paused-lane'+str(lane)+'-'+ident[:12]+':snapshot'
            log=env.docker('exec',prefix+'-worker','buildah','commit',ident,tag)
            (out/(prefix+'-'+ident[:12]+'-snapshot.log')).write_text(log)
            state['snapshot_images'].append({'lane':lane,'container_id':ident,'image':tag});save()
            env.docker('exec',prefix+'-worker','buildah','rm',ident)
        (out/(prefix+'-images.json')).write_text(env.docker('exec',prefix+'-worker','buildah','images','--json'))
        env.docker('stop','--timeout','3',prefix+'-worker');env.docker('rm','-v',prefix+'-worker')
        start(lane,False)
        for _ in range(60):
            if 'Worker consumer registered' in env.docker('logs',prefix+'-worker'):break
            time.sleep(1)
        else:raise RuntimeError('worker did not become ready')
        if env.docker('exec',prefix+'-worker','sha256sum','/e2e/worker-engine').split()[0]!=source['worker_binary_sha256']:raise RuntimeError('running binary drift')
    state['completed']=True;save()
    print('Reconciled user-paused attempts; both source-identical workers ready',flush=True)

if __name__=='__main__':main()
