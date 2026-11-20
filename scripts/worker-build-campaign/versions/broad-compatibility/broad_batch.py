#!/usr/bin/env python3
"""Serial diagnostic runner for reviewed broad-campaign recipes and probes."""
import sys
import argparse
from html.parser import HTMLParser
import json
from pathlib import Path
import re
import secrets
import shutil
import subprocess as sp
import tempfile
import time
from urllib.parse import urlsplit

import broad_fastapi as h
import broad_remaining as remaining

CASES = {
    'react-router': {'slot':6,'name':'react-router-express','repo':'remix-run/react-router-templates','sha':'338a991a4c4d2474ea8d3699cfcd5a77a27c9424','path':'node-custom-server','kind':'node','port':3000,
                     'file_hashes':{'node-custom-server/package.json':'40fbc13ac2fae61b621179eb32e12a96ad790b69dc998a33da6c22333a8119db','node-custom-server/server.js':'9e840b24ca09a8150177d94b8a1989a0eb34e15506cf629dea7748a5cc2812bd'}},
    'gin': {'slot':7,'name':'gin-multipart','repo':'gin-gonic/examples','sha':'2661f002ee5de912f23b03ec293c8ab741170d5e','path':'upload-file/single','kind':'go','port':8080,
            'file_hashes':{'go.mod':'44d916674d2431c250f09a6e68e4a5f8252b045471e5e46a4e3938ceb157985d','go.sum':'ba85191ad340cb911bf16198040ac2f6d8b71b00684ea7c69b5eeeefde1124ef','upload-file/single/main.go':'9207f968a525b8c616ca9515ce9433e1aa38b4687503ab0a5b97dcbd2cea1cb8'}},
    'django-cms': {'slot':4,'name':'django-cms','repo':'django-cms/django-cms-quickstart','sha':'b89a0ee62b65b195d4f3db85456862b648dbafc3','path':'.','kind':'python','port':80,
                   'file_hashes':{'requirements.txt':'ccd9348d3ecd54bb65efe356a89809174d40d0cca1db34e4cfbf5e256add5e04','democontent/home.json':'29faf3d4c8a0a4388453f1a0a88a72358915bd8a1db77a00d7b0d1723c8ae19b'}},
    'angular': {'slot':5,'name':'angular-realworld','repo':'gothinkster/angular-realworld-example-app','sha':'d426ec804f7d035c374aa4a1b48ecdb05a03eb02','path':'.','kind':'node','port':4200,
                'file_hashes':{'package.json':'d822151cf71979c3946bfcb50fc3a0f4860401fa749603c8189c9c55888e27c3','bun.lock':'9e511e9f7b4f82e2b916a1a72bc14e8c1b5f0a51f620cc80088af02ae72730bd'},
                'submodule':['realworld','https://github.com/realworld-apps/realworld.git','25588846c314b15a5e427ffe816e6b4af7be68c7']},
}
CASES.update(remaining.CASES)


def checkout(root, url, sha, destination):
    root.mkdir()
    h.cmd('git','-C',str(root),'init','-q')
    h.cmd('git','-C',str(root),'fetch','--depth=1',url,sha,timeout=180)
    h.cmd('git','-C',str(root),'checkout','--detach',sha)
    if h.cmd('git','-C',str(root),'rev-parse','HEAD')!=sha:
        raise RuntimeError('incorrect pinned revision')
    archive = root.parent/(root.name+'.tar')
    h.cmd('git','-C',str(root),'archive','--output',str(archive),sha,timeout=120)
    destination.mkdir(exist_ok=True)
    h.cmd('tar','-xf',str(archive),'-C',str(destination),timeout=120)


def ref_image(image):
    ident = h.pin_image(image)
    digest = h.docker('image','inspect',ident,'--format','{{index .RepoDigests 0}}')
    name = digest.partition('@')[0]
    first = name.split('/')[0]
    if '/' not in name:
        digest = 'docker.io/library/'+digest
    elif '.' not in first and ':' not in first and first!='localhost':
        digest = 'docker.io/'+digest
    return digest, ident


def reference_plan(case, source):
    if case['name'] in remaining.CASES:
        if not (source/case['path']/case['manifest']).exists():
            raise RuntimeError('expected pinned manifest not found')
        if case['steps'] is None:
            path = 'docker/Dockerfile' if case['name']=='mealie' else 'Dockerfile'
            text = (source/path).read_text()
            def qualify(m):
                name = m[1]
                first = name.split('/')[0].partition(':')[0]
                if '/' not in name:
                    return 'FROM docker.io/library/'+name
                if '.' not in first and first != 'localhost':
                    return 'FROM docker.io/'+name
                return m[0]
            return re.sub(r'(?m)^FROM ([\w./:@${}-]+)(?=\s)',lambda m: m[0] if m[1] in {'scratch','python-base','venv-builder-base'} else qualify(m),text),None,{}
        if case['name']=='aspnet-min-api':
            projects = list((source/case['path']).glob('*.csproj'))
            if len(projects)!=1:
                raise RuntimeError('expected one pinned ASP.NET project')
            start = 'dotnet /out/'+projects[0].stem+'.dll'
        else:
            start = case['start']
        bases = [ref_image(tag) for tag in case['images']]
        stage = f'FROM {bases[1][0]} AS tool\n' if len(bases)>1 else ''
        recipe = stage+f'FROM {bases[0][0]}\nWORKDIR /app\n'+case['steps']+'\n'
        if case['name']=='sveltekit-workspace':
            recipe += f'WORKDIR /app/{case["path"]}\nENV HOST=0.0.0.0 PORT=3000\n'
        return recipe,start,dict(zip(case['images'],[t[1] for t in bases]))
    if case['name']=='react-router-express':
        base, ident = ref_image('node:24-alpine')
        app = source/case['path']
        locked = (app/'package-lock.json').exists() or (app/'npm-shrinkwrap.json').exists()
        install = 'npm ci --ignore-scripts' if locked else 'npm install --no-package-lock --ignore-scripts'
        case['reference_dependency_policy'] = {'command':install,'locked':locked}
        recipe = (f'FROM {base}\nWORKDIR /app\nCOPY node-custom-server/ ./\n'
                  f'RUN {install} && npm run build && test -f build/server/index.js && npm prune --omit=dev --ignore-scripts\n'
                  'ENV NODE_ENV=production\nCMD ["node","server.js"]\n')
        return recipe,'node server.js',{'node':ident}
    if case['name']=='gin-multipart':
        go, go_id = ref_image('golang:1.27.0-alpine')
        alp, alp_id = ref_image('alpine:3.23')
        recipe = (f'FROM {go} AS build\nWORKDIR /src\nCOPY . .\nENV GOTOOLCHAIN=local CGO_ENABLED=0\n'
                  'RUN go build -mod=readonly -o /out/gin-upload ./upload-file/single\n'
                  f'FROM {alp}\nWORKDIR /app\nCOPY --from=build /out/gin-upload ./gin-upload\n'
                  'COPY upload-file/single/public ./public\nCMD ["./gin-upload"]\n')
        return recipe,'./gin-upload',{'golang':go_id,'alpine':alp_id}
    if case['name']=='django-cms':
        base, ident = ref_image('python:3.11')
        source_recipe = (source/'Dockerfile').read_text()
        if not source_recipe.startswith('FROM python:3.11\n'):
            raise RuntimeError('unexpected upstream recipe')
        return source_recipe.replace('FROM python:3.11\n',f'FROM {base}\n',1), 'uwsgi --http=0.0.0.0:80 --module=backend.wsgi', {'python':ident}
    bun, bun_id = ref_image('oven/bun:1')
    node, node_id = ref_image('node:24-alpine')
    recipe = (f'FROM {bun} AS builder\nWORKDIR /app\nCOPY . .\n'
              'ENV CI=true\nRUN bun --version && bun install --frozen-lockfile --ignore-scripts && bun run build\n'
              'RUN test -f dist/angular-conduit/browser/index.html\n'
              f'FROM {node}\nWORKDIR /app\nRUN npm install --ignore-scripts -g serve@14.2.6\n'
              'COPY --from=builder /app/dist/angular-conduit/browser ./site\n'
              'CMD ["serve","-s","site","-l","4200"]\n')
    return recipe,'serve -s site -l 4200', {'bun':bun_id,'node':node_id}


def request(role, base, path, domain=None, json_body=None):
    cli = [] if domain is None else ['-H','Host: '+domain]
    if json_body is not None:
        cli.extend(['-H','Content-Type: application/json','--data-binary',json.dumps(json_body)])
    p = h.dex(role,'curl','-sS','--max-time','4','-i','-w','\n%{http_code}',*cli,base+path,timeout=7,check=False)
    payload, _, status = p.stdout.strip().rpartition('\n')
    headers, _, body = payload.replace('\r\n','\n').partition('\n\n')
    return (status if p.returncode==0 else None,headers.lower(),body)


class Scripts(HTMLParser):
    def __init__(self):
        super().__init__(); self.src = []
    def handle_starttag(self, tag, attrs):
        src = dict(attrs).get('src')
        parsed = urlsplit(src or '')
        if tag=='script' and src and not parsed.netloc and not parsed.scheme and not src.startswith('//') and '..' not in parsed.path.split('/'):
            self.src.append('/'+src.lstrip('/'))


def probe(out, case, name, command, label):
    django = case['name']=='django-cms'
    gin = case['name']=='gin-multipart'
    router = case['name']=='react-router-express'
    port = case['port'] if label=='reference' or not django else 8000
    domain = 'cms.test' if django else None
    ip = h.docker('inspect','-f','{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}',h.NAMES['worker'])
    if h.tcp('127.0.0.1',port):
        raise RuntimeError('listener already present')
    result = {'port_closed_before_start':True,'ready':False,'external_pass':False,'checks':[]}
    if django:
        for key,value in {'DOMAIN':domain,'SECURE_SSL_REDIRECT':'False','SECRET_KEY':secrets.token_hex(24),'DATABASE_URL':'sqlite:////tmp/zdbroad-cms.sqlite3','DEFAULT_STORAGE_DSN':'/tmp/zdbroad-media/'}.items():
            h.dex('worker','buildah','config','--env',f'{key}={value}',name)
        fixture = ('python manage.py migrate --noinput && python manage.py democontent democontent/home.json && '
                   'python manage.py shell -c "from cms.models.contentmodels import PageContent; assert PageContent.admin_manager.filter(title=\'Home\',language=\'en\').exists(); print(\'CMS_ORM_READBACK_OK\')"')
        result['fixture_exit_code'] = h.logged(out/f'{label}-fixture.log','docker','exec',h.NAMES['worker'],'timeout','--kill-after=5s','100s','buildah','run',name,'--','sh','-c',fixture,timeout=110)
    with (out/f'{label}-runtime.log').open('x') as log:
        proc = sp.Popen(['docker','exec',h.NAMES['worker'],'timeout','--signal=TERM','--kill-after=5s','52s','buildah','run',name,'--','sh','-c','exec '+command],stdout=log,stderr=sp.STDOUT)
        try:
            for _ in range(22):
                if proc.poll() is not None:
                    break
                if h.tcp('127.0.0.1',port):
                    result['listener_started'] = True; break
                time.sleep(.5)
            if result.get('listener_started'):
                for role,address in [('worker','127.0.0.1'),('mock',ip)]:
                    base = f'http://{address}:{port}'
                    if django:
                        status, _, body = request(role,base,'/admin/login/',domain)
                        match = status=='200' and 'csrfmiddlewaretoken' in body
                        asset_code, headers, data = request(role,base,'/static/admin/css/base.css',domain)
                        specific = {'admin_css_pass':asset_code=='200' and 'text/css' in headers and bool(data)}
                    elif gin:
                        name_on_disk = 'zdbroad-'+role+'.txt'
                        payload = 'campaign multipart fixture '+role
                        status, _, body = request(role,base,'/')
                        match = status=='200' and '<form' in body.lower()
                        upload = h.docker('exec','-i',h.NAMES[role],'curl','-sS','--max-time','4','-w','\n%{http_code}',
                                       '-F','name=Campaign','-F','email=campaign@example.invalid',
                                       '-F','file=@-;filename='+name_on_disk,base+'/upload',input=payload,check=False,timeout=7)
                        saved = h.dex('worker','buildah','run',name,'--','sh','-c','test -f "$1" && cat "$1"','probe',name_on_disk,check=False,timeout=10)
                        specific = {'upload_status':upload.stdout.rpartition('\n')[2], 'saved_bytes_match':saved.returncode==0 and saved.stdout==payload,
                                    'upload_response_pass':upload.returncode==0 and upload.stdout.endswith('\n200') and f'File {name_on_disk} uploaded successfully' in upload.stdout}
                    elif router:
                        status, _, body = request(role,base,'/')
                        match = status=='200' and 'Hello from Express' in body
                        assets = sorted(set(re.findall(r'["\'](/assets/[A-Za-z0-9_.-]+\.(?:js|css))["\']',body)))
                        first_asset = request(role,base,assets[0]) if assets else (None,'','')
                        specific = {'ssr_loader_pass':match,'static_asset_pass':first_asset[0]=='200' and bool(first_asset[2]) and ('javascript' in first_asset[1] or 'text/css' in first_asset[1])}
                    elif case['name'] in remaining.CASES:
                        endpoint = case['endpoint']
                        mode = case['expected']
                        status, _, body = request(role,base,endpoint,json_body={'query':'{ __typename }'} if mode=='graphql' else None)
                        match = status=='200'
                        specific = {}
                        if mode=='graphql':
                            try: specific['graphql_pass'] = json.loads(body).get('data',{}).get('__typename')=='Query'
                            except (ValueError,AttributeError): specific['graphql_pass'] = False
                        elif mode=='todo':
                            title='disposable campaign '+role
                            created, _, _ = request(role,base,endpoint,json_body={'name':title,'isComplete':False})
                            _, _, listed = request(role,base,endpoint)
                            try: specific['ef_readback_pass'] = created=='201' and any(v.get('name')==title for v in json.loads(listed))
                            except (ValueError,TypeError,AttributeError): specific['ef_readback_pass'] = False
                    else:
                        status, _, body = request(role,base,'/')
                        match = status=='200' and '<title>Conduit</title>' in body and '<app-root' in body
                        links = Scripts(); links.feed(body)
                        asset = [request(role,base,path) for path in links.src[:4]]
                        route_code, _, route = request(role,base,'/login')
                        specific = {'javascript_pass':any(code=='200' and 'javascript' in header and bool(data) for code,header,data in asset),'static_route_fallback':route_code=='200' and '<app-root' in route,'browser_or_api_not_attempted':True}
                    result['checks'].append({'origin':role,'status':status,'http_pass':match,**specific})
                result['ready'],result['external_pass'] = (r['http_pass'] for r in result['checks'])
        finally:
            proc.wait(timeout=65)
    result['port_closed_after_stop'] = not h.tcp('127.0.0.1',port)
    if case['name'] in remaining.CASES:
        key = {'graphql':'graphql_pass','todo':'ef_readback_pass'}.get(case['expected'])
        result['functional_pass'] = (len(result['checks'])==2 and all(r['http_pass'] and r[key] for r in result['checks']) and result['port_closed_after_stop']) if key else None
        return result
    keys = ('admin_css_pass',) if django else ('saved_bytes_match','upload_response_pass') if gin else ('ssr_loader_pass','static_asset_pass') if router else ('javascript_pass','static_route_fallback')
    result['functional_pass'] = len(result['checks'])==2 and all(r['http_pass'] and all(r[key] for key in keys) for r in result['checks']) and result['port_closed_after_stop'] and (not django or result.get('fixture_exit_code')==0)
    return result


def run(case, parallel_with_django=False, next_pair=False, campaign_loop=False):
    case = dict(case)
    if next_pair and case['slot'] not in (6,7):
        raise RuntimeError('next pair mode is limited to the reviewed two-case batch')
    if parallel_with_django and case['name'] != 'angular-realworld':
        raise RuntimeError('second-lane admission is scoped to Angular alongside Django')
    if campaign_loop and case['name'] not in remaining.CASES:
        raise RuntimeError('loop mode is scoped to the remaining approved cases')
    heavy = case['name'] in {'netbox','mealie','actual-budget','hexpm'}
    memory = '1280m' if parallel_with_django or next_pair or (campaign_loop and not heavy) else '2g'
    memory_bytes = int(memory[:-1])*1024**2 if memory.endswith('m') else 2*1024**3
    out = h.EVIDENCE/f'case-{case["slot"]:02d}-{case["name"]}'
    if out.exists():
        prior = json.loads((out/'result.json').read_text())
        if prior['complete']:
            print(case['name'], 'already measured; skipping', flush=True)
            return
        if prior.get('stage') != 'freeze' or prior.get('job_id') or prior.get('reference'):
            raise RuntimeError('incomplete attempt requires reconciliation')
        out = out.with_name(out.name+'-setup-retry')
    out.mkdir(exist_ok=False)
    h.PREFIX = f'zdbroad-case{case["slot"]:02d}'
    h.NAMES = {k:f'{h.PREFIX}-{k}' for k in ['worker','mock','db','rabbit','net','storage','pgdata','rabbitdata']}
    state = {'case':case,'product_commit':h.PRODUCT,'started_at':h.stamp(),'complete':False}
    def save(): h.common.write_json(out/'result.json',state)
    def stage(value): state['stage']=value; save(); print(case['name'],value,flush=True)
    own = []
    try:
        stage('freeze')
        h.cmd('git','-C',str(h.ROOT),'diff','--quiet',h.PRODUCT,'--','worker-server')
        capacity = json.loads(h.docker('info','--format','{"cpu":{{.NCPU}},"memory":{{.MemTotal}}}'))
        active = set(h.docker('ps','--filter','label='+h.LABEL,'--format','{{.Names}}').splitlines())
        django = {f'zdbroad-case04-{role}' for role in ('worker','mock','db','rabbit')}
        peer = f'zdbroad-case{13-case["slot"]:02d}' if next_pair else 'zdbroad-case04'
        allowed = {peer+'-'+role for role in ('worker','mock','db','rabbit')} if next_pair else django if parallel_with_django else set()
        if campaign_loop and not heavy:
            for name in active:
                match = re.fullmatch(r'zdbroad-case(\d\d)-(worker|mock|db|rabbit)',name)
                if match and 8<=int(match[1])<=23 and int(match[1])!=case['slot']:
                    allowed.add(name)
        if capacity['memory'] < 5*1024**3 or capacity['cpu'] < 6 or shutil.disk_usage(h.ROOT).free < 20*1024**3 or active - allowed:
            raise RuntimeError('insufficient capacity or unexpected active broad lane')
        if (next_pair or (campaign_loop and not heavy)) and 2*(memory_bytes+768*1024**2)+768*1024**2 > capacity['memory']:
            raise RuntimeError('insufficient memory to admit both next-pair lanes')
        if active:
            limits = [json.loads(h.docker('inspect',name))[0] for name in sorted(active)]
            prefixes = {item['Name'].lstrip('/').rsplit('-',1)[0] for item in limits}
            if len(prefixes)!=1 or any(item['Config']['Labels'].get(h.LABEL)!=item['Name'].lstrip('/').rsplit('-',1)[0] or item['HostConfig']['Memory']<=0 for item in limits):
                raise RuntimeError('cannot verify first-lane ownership or memory caps')
            projected = sum(item['HostConfig']['Memory'] for item in limits) + memory_bytes + 768*1024**2
            if projected+768*1024**2 > capacity['memory']:
                raise RuntimeError('second lane does not leave 768 MiB for existing services and host overhead')
            state['parallel_admission'] = {'first_lane':sorted(active),'projected_broad_memory_bytes':projected}
        if any(h.docker('inspect',v,check=False).returncode==0 for v in h.NAMES.values()):
            raise RuntimeError('resource collision')
        temp = Path(tempfile.mkdtemp(prefix=h.PREFIX+'-',dir=Path.home()))
        state['local_work_root'] = str(temp)
        bundle = temp/'bundle'; bundle.mkdir(mode=0o700)
        source = bundle/'app'; tree = temp/'checkout'
        checkout(tree,'https://github.com/'+case['repo']+'.git',case['sha'],source)
        if 'submodule' in case:
            path,url,sha = case['submodule']
            fields = h.cmd('git','-C',str(tree),'ls-tree',case['sha'],path).split()
            assert fields[:3]==['160000','commit',sha]
            checkout(temp/'submodule',url,sha,source/path)
        assert all(h.sha(source/path)==sha for path,sha in case['file_hashes'].items())
        manifest_names = {'package.json','package-lock.json','npm-shrinkwrap.json','pnpm-lock.yaml','pnpm-workspace.yaml','yarn.lock','bun.lock','Cargo.toml','Cargo.lock','go.mod','go.sum','pyproject.toml','uv.lock','requirements.txt','base_requirements.txt','composer.json','composer.lock','mix.exs','mix.lock','pom.xml','settings.gradle','build.gradle','gradle-wrapper.properties','global.json','rust-toolchain.toml'}
        files = set()
        for root in {source, source/case['path'], source/case['path']/'gradle/wrapper'}:
            if root.is_dir(): files.update(p for p in root.iterdir() if p.is_file() and (p.name in manifest_names or p.suffix=='.csproj'))
        state['manifest_sha256'] = {str(p.relative_to(source)):h.sha(p) for p in sorted(files)}
        inputs = out/'source-manifests'; inputs.mkdir()
        for p in files:
            dst = inputs/p.relative_to(source); dst.parent.mkdir(parents=True,exist_ok=True); shutil.copyfile(p,dst)
        try:
            recipe,command,ref_images = reference_plan(case,source)
        except (RuntimeError,OSError,sp.TimeoutExpired) as err:
            if not campaign_loop:
                raise
            recipe,command,ref_images = None,None,{}
            state['reference'] = {'image_built':None,'blocked':str(err)[:1400]}
        if recipe:
            (out/'reference.Dockerfile').write_text(recipe); (bundle/'reference.Dockerfile').write_text(recipe)
        h.cmd('env','CGO_ENABLED=0','go','build','-buildvcs=false','-trimpath','-o',str(bundle/'worker-engine'),'./cmd/worker-server',cwd=h.ROOT/'worker-server',timeout=240)
        h.cmd('openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','4','-keyout',str(bundle/'mock.key'),'-out',str(bundle/'mock.crt'),'-subj','/CN=e2e.r2.cloudflarestorage.com','-addext','subjectAltName=DNS:e2e.r2.cloudflarestorage.com,DNS:testbucket.e2e.r2.cloudflarestorage.com')
        images = {role:h.pin_image(image) for role,image in {'worker':'zero-devops-e2e-worker:local','db':'postgres:17-alpine','rabbit':'rabbitmq:3-management-alpine'}.items()}
        state.update(worker_binary_sha256=h.sha(bundle/'worker-engine'), support_image_ids=images, reference_image_ids=ref_images, recipe_sha256=h.sha(bundle/'reference.Dockerfile') if recipe else None, harness_sha256={Path(path).name:h.sha(Path(path)) for path in [__file__,remaining.__file__,h.__file__,h.common.__file__,h.previous.__file__]}, capacity=capacity, budget={'cpu':1.5,'memory_bytes':memory_bytes,'pids':512,'reference_limit_seconds':1200})
        stage('provision')
        h.docker('network','create','--label',h.LABEL+'='+h.PREFIX,h.NAMES['net'])
        for role in ['storage','pgdata','rabbitdata']:
            h.docker('volume','create','--label',h.LABEL+'='+h.PREFIX,h.NAMES[role])
        def start(role,*options,**kw):
            own.append(role); h.service(role,images['worker' if role=='mock' else role],*options,**kw)
        start('db','0.5','256m','128','-v',h.NAMES['pgdata']+':/var/lib/postgresql/data','-e','POSTGRES_USER=e2e','-e','POSTGRES_PASSWORD=e2e','-e','POSTGRES_DB=e2e')
        start('rabbit','0.5','384m','256','-v',h.NAMES['rabbitdata']+':/var/lib/rabbitmq','-p','127.0.0.1::15672','-e','RABBITMQ_DEFAULT_USER=e2e','-e','RABBITMQ_DEFAULT_PASS=e2e','-e','RABBITMQ_SERVER_ADDITIONAL_ERL_ARGS=+S 1:1')
        start('mock','0.25','128m','64','--network-alias','e2e.r2.cloudflarestorage.com','--network-alias','testbucket.e2e.r2.cloudflarestorage.com','-v',str(bundle)+':/e2e:ro','-v',str(h.ROOT/'scripts/worker-build-campaign')+':/harness:ro','--entrypoint','python3',command=('/harness/mock_services.py',))
        h.until(lambda:h.dex('db','pg_isready','-h','127.0.0.1','-U','e2e',check=False).returncode==0)
        h.until(lambda:h.dex('mock','curl','-fsS','--max-time','3','-u','e2e:e2e','http://rabbit:15672/api/overview',check=False).returncode==0)
        migration=(h.ROOT/'worker-server/migrations/20260701203442_create_worker_deployments_table.sql').read_text().split('-- +goose Down')[0]
        h.docker('exec','-i',h.NAMES['db'],'psql','-v','ON_ERROR_STOP=1','-U','e2e','-d','e2e',input=migration)
        env=json.loads((h.ROOT/'docs/worker-build-fixes-2026-11-16/environment.json').read_text())['worker_environment']
        env.update(DATABASE_HOST='db',RABBITMQ_CONNECTION_STRING='amqp://e2e:e2e@rabbit:5672/',ALLOW_UNLOCKED_NPM_INSTALL='true')
        assert env['SERVER_INTERNAL_URL']=='http://mock:8000' and env['CLOUDFLARE_ACCOUNT_ID']=='e2e' and env['CLOUDFLARE_BUCKET_NAME']=='testbucket'
        start('worker','1.5',memory,'512','--privileged','-v',h.NAMES['storage']+':/var/lib/containers','-v',str(bundle)+':/e2e:ro','-v',str(h.ROOT/'worker-server/templates')+':/app/templates:ro','--entrypoint','/e2e/worker-engine',*[arg for k,v in env.items() for arg in ('-e',f'{k}={v}')])
        h.until(lambda:'Worker consumer registered' in h.docker('logs',h.NAMES['worker']))
        assert json.loads(h.dex('worker','buildah','images','--json'))==[]
        stage('reference')
        tag=f'localhost/{h.PREFIX}-reference:trial'
        if recipe:
            try:
                rc=h.logged(out/'reference-build.log','docker','exec',h.NAMES['worker'],'timeout','--kill-after=10s','1200s','buildah','bud','-f','/e2e/reference.Dockerfile','-t',tag,'/e2e/app',timeout=1230)
                state['reference']={'image_built':rc==0,'exit_code':rc,'scope':case.get('reference_scope')}; save()
                if rc==0:
                    (out/'reference-image.json').write_text(h.dex('worker','buildah','inspect','--type','image',tag)+'\n')
                    if command:
                        runtime=h.PREFIX+'-reference'; h.dex('worker','buildah','from','--name',runtime,tag)
                        state['reference']['runtime']=probe(out,case,runtime,command,'reference'); save()
                        h.dex('worker','buildah','rm',runtime)
                    else:
                        state['reference']['runtime']={'attempted':False,'reason':case.get('reference_scope')}
            except (RuntimeError,OSError,sp.TimeoutExpired) as err:
                if not campaign_loop:
                    raise
                state.setdefault('reference',{})['measurement_error']=str(err)[:1400]
            save()
        stage('worker')
        c=h.common; c.WORKER,c.MOCK,c.DB,c.RABBIT=(h.NAMES[k] for k in ('worker','mock','db','rabbit'))
        c.RESULTS,c.ROOT,c.VHOST=out,out,'/'
        c.guard=lambda:None; c.resource=lambda _:{}
        def worker_probe(container,name,cmd,runtime_case,label):
            if campaign_loop and not case['endpoint']:
                return {'attempted':False,'reason':case['reference_scope']}
            return probe(out,{**case,'port':runtime_case['port']},name,cmd,label)
        c.probe_runtime=worker_probe
        orig_uuid=h.previous.uuid
        class Checkpoint:
            @staticmethod
            def uuid4():
                value=orig_uuid.uuid4()
                if 'job_id' not in state:
                    state['job_id']=str(value); save()
                return value
        h.previous.uuid=Checkpoint
        orig_dex=c.dex
        def no_prune(container,args,timeout=30,check=False):
            if args[:2]==['buildah','rmi'] or args[:3]==['buildah','rm','--all']:
                return (0,'')
            return orig_dex(container,args,timeout,check)
        c.dex=no_prune
        try:
            h.previous.worker(c,case,state)
        finally:
            h.previous.uuid=orig_uuid; c.dex=orig_dex
        state['complete']=True; stage('completed')
    except Exception as ex:
        state['harness_error']=f'{type(ex).__name__}: {str(ex)[:2000]}'
        raise
    finally:
        state['resource_stop_errors']=[]
        for role in reversed(own):
            name=h.NAMES[role]
            try:
                exists=h.docker('inspect','--format','{{index .Config.Labels "'+h.LABEL+'"}}',name,check=False)
                if exists.returncode or exists.stdout.strip()!=h.PREFIX:
                    continue
                (out/f'final-{role}.log').write_text(h.docker('logs',name,check=False).stdout)
                h.docker('stop','-t','10',name,timeout=30)
            except Exception as ex:
                state['resource_stop_errors'].append({'role':role,'error':type(ex).__name__})
        state['finished_at']=h.stamp(); save()


if __name__=='__main__':
    p=argparse.ArgumentParser(); p.add_argument('cases',nargs='+',choices=CASES)
    p.add_argument('--parallel-with-django',action='store_true')
    p.add_argument('--next-pair',action='store_true')
    p.add_argument('--campaign-loop',action='store_true')
    args=p.parse_args()
    for key in args.cases:
        run(CASES[key],args.parallel_with_django,args.next_pair,args.campaign_loop)
