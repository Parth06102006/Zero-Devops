#!/usr/bin/env python3
"""Additional final evidence audit; does not mutate sources or active freeze."""
import datetime
import hashlib
import json
import os
import pathlib
import re
import stat
import tomllib
import urllib.request
import xml.etree.ElementTree as ET

REPO = pathlib.Path(__file__).absolute().parents[2]
OUT = REPO / 'docs/worker-build-followup-2026-11-17'


def read(path):
    return json.loads(path.read_text())


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def get(url):
    request = urllib.request.Request(url, headers={'User-Agent': 'zero-devops-final-evidence-audit'})
    with urllib.request.urlopen(request, timeout=90) as response:
        return json.load(response)


def declared_requirements(case):
    app = pathlib.Path(case['source_dir']) / case['path']
    row = {key: case[key] for key in ['name', 'repo', 'sha', 'path', 'kind', 'base_image']}
    if case['kind'] == 'node':
        package = read(app / 'package.json')
        row.update(engines=package.get('engines', {}),
                   dependencies=package.get('dependencies', {}),
                   devDependencies=package.get('devDependencies', {}),
                   scripts=package.get('scripts', {}),
                   lockfiles=[p.name for p in app.iterdir() if p.name in [
                       'package-lock.json', 'npm-shrinkwrap.json', 'pnpm-lock.yaml',
                       'yarn.lock', 'bun.lock', 'bun.lockb']])
    elif case['kind'] == 'go':
        text = (app / 'go.mod').read_text()
        row.update(go_mod=text, go_requirement=re.search(r'(?m)^go\s+(\S+)', text)[1],
                   has_go_sum=(app / 'go.sum').is_file())
    elif case['kind'] == 'rust':
        source = pathlib.Path(case['source_dir'])
        row.update(cargo_manifest=tomllib.loads((app / 'Cargo.toml').read_text()),
                   workspace_manifest=tomllib.loads((source / 'examples/Cargo.toml').read_text()),
                   workspace_lock_sha256=digest(source / 'examples/Cargo.lock'))
    else:
        root = ET.fromstring((app / 'pom.xml').read_text())
        ns = {'m': 'http://maven.apache.org/POM/4.0.0'}
        row.update(java_requirement=root.findtext('m:properties/m:java.version', namespaces=ns),
                   spring_boot_parent=root.findtext('m:parent/m:version', namespaces=ns),
                   dependencies=[{
                       'group': e.findtext('m:groupId', namespaces=ns),
                       'artifact': e.findtext('m:artifactId', namespaces=ns),
                       'scope': e.findtext('m:scope', default='compile', namespaces=ns)
                   } for e in root.findall('m:dependencies/m:dependency', ns)])
    return row


def main():
    destination = OUT / 'supplemental-evidence-audit.json'
    if destination.exists():
        raise RuntimeError('supplemental audit exists; preserve rather than overwrite')
    cases = read(OUT / 'fresh-manifest.json')
    artifacts = [OUT / 'fresh-dependency-requirements.json',
                 *sorted((OUT / 'fresh-source-evidence').glob('*.json'))]
    repositories = []
    for evidence_path in artifacts[1:]:
        evidence = read(evidence_path)
        selected = [case for case in cases if case['repo'] == evidence['repo']]
        if not selected or any(case['sha'] != evidence['sha'] or
                               case['source_dir'] != evidence['source_dir'] for case in selected):
            raise RuntimeError('source evidence / case identity mismatch')
        root = pathlib.Path(evidence['source_dir'])
        expected = {entry['path']: entry for entry in evidence['files']}
        if len(expected) != len(evidence['files']):
            raise RuntimeError('duplicate source inventory entries')
        actual = {str(path.relative_to(root)): path for path in root.rglob('*')
                  if path.is_file() or path.is_symlink()}
        if set(actual) != set(expected):
            raise RuntimeError('source inventory mismatch: ' + evidence['repo'] +
                               ' extras=' + repr(set(actual) - set(expected)) +
                               ' missing=' + repr(set(expected) - set(actual)))
        for name, entry in expected.items():
            path = actual[name]
            mode = path.lstat().st_mode
            if entry['mode'] == '120000':
                if not stat.S_ISLNK(mode):
                    raise RuntimeError('expected symlink: ' + name)
                data = os.readlink(path).encode()
                if not path.resolve().is_relative_to(root.resolve()):
                    raise RuntimeError('source symlink escapes repository: ' + name)
            else:
                if not stat.S_ISREG(mode):
                    raise RuntimeError('expected regular file: ' + name)
                executable = bool(mode & 0o111)
                if executable != (entry['mode'] == '100755'):
                    raise RuntimeError('executable-bit mismatch: ' + name)
                data = path.read_bytes()
            blob = hashlib.sha1(b'blob ' + str(len(data)).encode() + b'\0' + data).hexdigest()
            if blob != entry['git_blob_sha'] or hashlib.sha256(data).hexdigest() != entry['sha256'] or len(data) != entry['bytes']:
                raise RuntimeError('source hash/size mismatch: ' + name)
        base = 'https://api.github.com/repos/' + evidence['repo']
        commit = get(base + '/git/commits/' + evidence['sha'])
        if commit['sha'] != evidence['sha']:
            raise RuntimeError('upstream commit identity mismatch')
        tree_sha = commit['tree']['sha']
        tree = get(base + '/git/trees/' + tree_sha + '?recursive=1')
        if tree.get('truncated') or tree['sha'] != tree_sha:
            raise RuntimeError('upstream tree is incomplete or mismatched')
        prefixes = [case['path'].rstrip('/') + '/' for case in selected]
        upstream = {entry['path']: entry for entry in tree['tree']
                    if entry['type'] == 'blob' and (evidence['repo'] == 'tokio-rs/axum' or
                       any(entry['path'].startswith(prefix) for prefix in prefixes))}
        if set(upstream) != set(expected):
            raise RuntimeError('pinned upstream tree / evidence inventory mismatch')
        for name, entry in expected.items():
            original = upstream[name]
            if original['sha'] != entry['git_blob_sha'] or original['mode'] != entry['mode']:
                raise RuntimeError('pinned upstream blob/mode mismatch: ' + name)
        repositories.append({'repo': evidence['repo'], 'commit_sha': evidence['sha'],
                             'upstream_tree_sha': tree_sha, 'files_verified': len(expected),
                             'exact_inventory': True, 'git_blob_hashes': True,
                             'sha256_and_sizes': True, 'git_modes': True,
                             'pinned_upstream_linkage': True})
        print('SOURCE AUDIT', evidence['repo'], len(expected), flush=True)
    declarations = [declared_requirements(case) for case in cases]
    if declarations != read(OUT / 'fresh-dependency-requirements.json'):
        raise RuntimeError('dependency metadata differs from pinned source declarations')
    warm_checks = []
    pending = []
    for batch in ['failures', 'fresh']:
        manifest = read(OUT / (batch + '-manifest.json'))
        for case in manifest:
            path = OUT / batch / 'results' / (case['name'] + '.json')
            if not path.exists() or not (record := read(path)).get('complete'):
                pending.append(case['name'])
                continue
            for kind in ['worker', 'reference']:
                trial = record.get(kind, {})
                if trial.get('image_built'):
                    runtime = trial.get('runtime', {})
                    if not all(runtime.get(field) is True for field in [
                            'port_closed_before_start', 'port_closed_after_stop']):
                        raise RuntimeError('probe isolation failed: ' + case['name'] + '/' + kind)
                if not trial.get('runtime', {}).get('ready'):
                    continue
                accepted = case.get('expected_status', list(range(200, 400)))
                samples = trial['runtime'].get('warm_http', [])
                warm_checks.append({'batch': batch, 'case': case['name'], 'trial': kind,
                                    'requests_recorded': len(samples),
                                    'unexpected_statuses': [item for item in samples if item['status'] not in accepted],
                                    'missing_of_eight': max(0, 8 - len(samples))})
    if pending:
        raise RuntimeError('supplemental final audit requires completed cohorts: ' + repr(pending))
    result = {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'passed': True,
              'note': 'Additional final check, not part of or modification to the original active freeze.',
              'repositories': repositories, 'source_files_verified': sum(item['files_verified'] for item in repositories),
              'dependency_metadata_verified': len(declarations),
              'reference_and_worker_isolation_verified': True, 'warm_http_review': warm_checks,
              'warm_http_instability_observed': any(item['unexpected_statuses'] or item['missing_of_eight'] for item in warm_checks),
              'audited_artifact_sha256': {str(path.relative_to(REPO)): digest(path) for path in artifacts},
              'audit_script_sha256': digest(pathlib.Path(__file__))}
    destination.write_text(json.dumps(result, indent=2) + '\n')
    print('SUPPLEMENTAL EVIDENCE AUDIT PASSED', flush=True)


if __name__ == '__main__':
    main()
