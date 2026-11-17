#!/usr/bin/env python3
"""Freeze a fresh public sample cohort before builds; never modify sample sources."""
import concurrent.futures, hashlib, json, os, pathlib, re, time, tomllib, urllib.request, xml.etree.ElementTree as ET

REPO = pathlib.Path(__file__).resolve().parents[2]
OUT = REPO / 'docs/worker-build-followup-2026-11-17'
SOURCES = pathlib.Path('/tmp/zd-round2-fresh-sources')
PINS = {
    'withastro/astro': '387105be28cb3a3c062c3390b551cc9e51a2bfb1',
    'gofiber/recipes': 'e1f44175db9420bb612134d9bb84d51dcca661ed',
    'tokio-rs/axum': '36d38c4cf2766cd7313dd53d808f30546ee36b1e',
    'spring-guides/gs-rest-service-cors': '404d6abe86fd54800e092d712e339ba9c0c67217',
    'spring-guides/gs-handling-form-submission': '9fd7eb0e8ea1853bdca92ffc91054131832bcace',
    'vercel/next.js': 'd958b3ac19a7859838a2a08e8d7c0014baf56146',
}

def case(name, repo, path, kind, port, probe, build, start, **extra):
    return dict(name=name, repo=repo, sha=PINS[repo], path=path, kind=kind,
                port=port, probe=probe, reference_build=build, reference_start=start,
                context=path, env={}, source_dir=str(SOURCES / repo.replace('/', '--')), **extra)

def manifest():
    cases=[]
    for app in ['framework-preact','framework-solid','with-mdx','with-markdoc']:
        cases.append(case('fresh-astro-'+app,'withastro/astro','examples/'+app,'node',4321,'/',
                          'npm install --ignore-scripts --package-lock=false && npm run build',
                          'npm run preview -- --host 0.0.0.0 --port 4321',
                          base_image='docker.io/library/node:24-alpine',static_output='dist',
                          stack='Astro/'+app))
    for app,probe in [('hello-world','/'),('404-handler','/hello'),('advanced-caching','/health')]:
        c=case('fresh-fiber-'+app,'gofiber/recipes',app,'go',3000,probe,
               'GOTOOLCHAIN=local go build -o /tmp/reference-server .','/tmp/reference-server',
               base_image='docker.io/library/golang:1.27.0-alpine',stack='Fiber v3/'+app)
        if app=='advanced-caching':c['env']={'ADMIN_API_KEY':'campaign-test-only-key'}
        cases.append(c)
    for app,probe,port in [('hello-world','/',3000),('cors','/json',4000)]:
        target='example-'+app
        cases.append(case('fresh-axum-'+app,'tokio-rs/axum','examples/'+app,'rust',port,probe,
                          'apk add --no-cache musl-dev && CARGO_BUILD_JOBS=1 cargo build --locked --release --manifest-path examples/'+app+'/Cargo.toml --package '+target+' --bin '+target+' --target-dir /tmp/reference-target',
                          '/tmp/reference-target/release/'+target,
                          base_image='docker.io/library/rust:1-alpine',reference_workdir='.',
                          stack='Axum/'+app))
    for app,probe,artifact in [('gs-rest-service-cors','/greeting','rest-service-cors-complete'),('gs-handling-form-submission','/greeting','handling-form-submission-complete')]:
        cases.append(case('fresh-'+app,'spring-guides/'+app,'complete','java',8080,probe,
                          'mvn -q -DskipTests package','java -jar target/'+artifact+'-0.0.1-SNAPSHOT.jar',
                          base_image='docker.io/library/maven:3-eclipse-temurin-21',
                          stack='Spring/'+app))
    cases.append(case('fresh-next-with-sass','vercel/next.js','examples/with-sass','node',3000,'/',
                      'npm install --ignore-scripts --package-lock=false && npm run build',
                      'npm start -- --hostname 0.0.0.0 --port 3000',
                      base_image='docker.io/library/node:24-alpine',stack='Next/Sass'))
    return cases

def get(url):
    for attempt in range(3):
        try:
            request=urllib.request.Request(url,headers={'User-Agent':'zero-devops-compatibility-campaign'})
            return urllib.request.urlopen(request,timeout=90).read()
        except Exception:
            if attempt==2:raise
            time.sleep(attempt+1)

def download_repo(repo,cases):
    sha=PINS[repo];root=SOURCES/repo.replace('/','--');root.mkdir(parents=True,exist_ok=True)
    tree=json.loads(get('https://api.github.com/repos/'+repo+'/git/trees/'+sha+'?recursive=1'))
    if tree.get('truncated'):raise RuntimeError('incomplete GitHub source tree '+repo)
    prefixes=[c['path'].rstrip('/')+'/' for c in cases]
    entries=[e for e in tree['tree'] if e['type']=='blob' and (repo=='tokio-rs/axum' or any(e['path'].startswith(p) for p in prefixes))]
    # Preserve immutable upstream blob hashes, not only filenames.
    def one(entry):
        path=root/entry['path']
        if not path.resolve().is_relative_to(root.resolve()):raise RuntimeError('source path escape')
        if entry.get('mode')=='120000':
            # The Git blob contains the link text; raw hosts may dereference links.
            blob_data=json.loads(get('https://api.github.com/repos/'+repo+'/git/blobs/'+entry['sha']))
            import base64
            raw=base64.b64decode(blob_data['content'])
        elif path.is_file() and not path.is_symlink():
            raw=path.read_bytes()
            existing_blob=hashlib.sha1(b'blob '+str(len(raw)).encode()+b'\0'+raw).hexdigest()
            if existing_blob!=entry['sha']:raise RuntimeError('existing source differs: '+str(path))
        else:raw=get('https://raw.githubusercontent.com/'+repo+'/'+sha+'/'+entry['path'])
        blob=hashlib.sha1(b'blob '+str(len(raw)).encode()+b'\0'+raw).hexdigest()
        if blob!=entry['sha']:raise RuntimeError('upstream Git blob mismatch: '+entry['path'])
        path.parent.mkdir(parents=True,exist_ok=True)
        if entry.get('mode')=='120000':
            target=raw.decode()
            if not (path.parent/target).resolve().is_relative_to(root.resolve()):raise RuntimeError('upstream symlink escapes source root')
            if path.is_symlink():
                if os.readlink(path)!=target:raise RuntimeError('existing symlink differs')
            elif path.exists():raise RuntimeError('expected upstream symlink, found file')
            else:path.symlink_to(target)
        else:
            if path.exists() and path.read_bytes()!=raw:raise RuntimeError('existing source differs: '+str(path))
            path.write_bytes(raw)
            if entry.get('mode')=='100755':path.chmod(0o755)
        return {'path':entry['path'],'mode':entry.get('mode'),'git_blob_sha':blob,'sha256':hashlib.sha256(raw).hexdigest(),'bytes':len(raw)}
    with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:records=list(pool.map(one,entries))
    metadata={'repo':repo,'sha':sha,'source_dir':str(root),'method':'immutable GitHub raw blobs, verified using Git blob SHA-1 against pinned recursive tree','files':records}
    (OUT/'fresh-source-evidence'/(repo.replace('/','--')+'.json')).write_text(json.dumps(metadata,indent=2)+'\n')
    print('SOURCES',repo,len(records),sum(x['bytes'] for x in records),flush=True)

def main():
    OUT.mkdir(parents=True,exist_ok=True);(OUT/'fresh-source-evidence').mkdir(exist_ok=True)
    cases=manifest()
    original=json.loads((REPO/'docs/worker-build-campaign-50/manifest.json').read_text())
    old={(c['repo'],c['path']) for c in original}
    assert len(cases)==12 and not any((c['repo'],c['path']) in old for c in cases)
    path=OUT/'fresh-manifest.json';serialized=json.dumps(cases,indent=2)+'\n'
    if path.exists() and path.read_text()!=serialized:raise RuntimeError('fresh cohort already frozen differently')
    path.write_text(serialized)
    for repo in PINS:download_repo(repo,[c for c in cases if c['repo']==repo])
    dependencies=[]
    for c in cases:
        app=pathlib.Path(c['source_dir'])/c['path']
        row={key:c[key] for key in ['name','repo','sha','path','kind','base_image']}
        if c['kind']=='node':
            pkg=json.loads((app/'package.json').read_text())
            row.update(engines=pkg.get('engines',{}),dependencies=pkg.get('dependencies',{}),devDependencies=pkg.get('devDependencies',{}),scripts=pkg.get('scripts',{}),lockfiles=[p.name for p in app.iterdir() if p.name in ['package-lock.json','npm-shrinkwrap.json','pnpm-lock.yaml','yarn.lock','bun.lock','bun.lockb']])
        elif c['kind']=='go':
            text=(app/'go.mod').read_text()
            row.update(go_mod=text,go_requirement=re.search(r'(?m)^go\s+(\S+)',text)[1],has_go_sum=(app/'go.sum').is_file())
        elif c['kind']=='rust':
            row.update(cargo_manifest=tomllib.loads((app/'Cargo.toml').read_text()),workspace_manifest=tomllib.loads((pathlib.Path(c['source_dir'])/'examples/Cargo.toml').read_text()),workspace_lock_sha256=hashlib.sha256((pathlib.Path(c['source_dir'])/'examples/Cargo.lock').read_bytes()).hexdigest())
        else:
            root=ET.fromstring((app/'pom.xml').read_text());ns={'m':'http://maven.apache.org/POM/4.0.0'}
            row.update(java_requirement=root.findtext('m:properties/m:java.version',namespaces=ns),spring_boot_parent=root.findtext('m:parent/m:version',namespaces=ns),dependencies=[{'group':e.findtext('m:groupId',namespaces=ns),'artifact':e.findtext('m:artifactId',namespaces=ns),'scope':e.findtext('m:scope',default='compile',namespaces=ns)} for e in root.findall('m:dependencies/m:dependency',ns)])
        dependencies.append(row)
    (OUT/'fresh-dependency-requirements.json').write_text(json.dumps(dependencies,indent=2)+'\n')
    print('FRESH SOURCES READY',len(cases),flush=True)

if __name__=='__main__':main()
