#!/usr/bin/env python3
"""Complete unsubmitted cases serially after archived controller telemetry errors."""
import datetime
import hashlib
import importlib.util
import json
import pathlib
import subprocess

REPO = pathlib.Path(__file__).absolute().parents[2]
OUT = REPO / 'docs/worker-build-followup-2026-11-17'
ARCHIVE = OUT / 'recovery-20261117'
SPEC = importlib.util.spec_from_file_location('round2_parallel', REPO / 'scripts/worker-build-campaign/round2_parallel.py')
P = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(P)


def run(args, timeout=60):
    return subprocess.check_output(args, text=True, stderr=subprocess.STDOUT, timeout=timeout).strip()


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def main():
    frozen = json.loads((OUT / 'frozen-campaign.json').read_text())
    state = json.loads((ARCHIVE / 'parallel-state.json').read_text())
    if state['in_flight'] or not state['finished_at']:
        raise RuntimeError('original controller must be stopped')
    for key in ['product_source_sha256', 'prior_evidence_sha256', 'manifests_sha256']:
        P.verify(frozen[key])
    P.verify(state['harness_sha256'])
    recovery = json.loads((ARCHIVE / 'measurement-recovery.json').read_text())
    for recovered in recovery['recovered_original_images']:
        name = recovered['case']
        record = json.loads((OUT / 'failures/results' / (name + '.json')).read_text())
        if not record['complete'] or record.get('harness_error') or not record['measurement_recovery']['image_rebuilt'] is False:
            raise RuntimeError('unresolved retained-image recovery')
        state['completed'].append({'lane': record['worker']['lane'], 'batch': 'failures',
                                   'case': name, 'at': record['finished_at'],
                                   'measurement_recovered_without_rebuild': True})
    state['recovered_controller_errors'] = state['errors']
    state['errors'] = []
    state['finished_at'] = None
    state['resume_started_at'] = now()
    state['recovery_archive'] = 'recovery-20261117'
    state['scheduling'] += '; after telemetry failures, retained images measured without rebuild and remaining unsubmitted cases run serially on lane 1'
    P.write(OUT / 'parallel-state.json', state)
    # Lane 2 remains stopped with its ready/redelivered job preserved. Lane 1
    # queue was verified empty; starting it cannot duplicate either recovered job.
    original = 'zdround2-1-worker'
    if run(['docker', 'inspect', original, '--format', '{{index .Config.Labels "zero-devops-build-followup"}}']) != '20261117':
        raise RuntimeError('unowned worker')
    if run(['docker', 'inspect', 'zdround2-2-worker', '--format', '{{.State.Running}}']) != 'false':
        raise RuntimeError('lane 2 must remain stopped during serial continuation')
    run(['docker', 'start', original])
    if run(['docker', 'exec', original, 'sha256sum', '/e2e/worker-engine']).split()[0] != frozen['binary_sha256']:
        raise RuntimeError('frozen worker binary mismatch')
    remaining = []
    for batch in ['failures', 'fresh']:
        for case in json.loads((OUT / (batch + '-manifest.json')).read_text()):
            path = OUT / batch / 'results' / (case['name'] + '.json')
            if path.exists():
                if not json.loads(path.read_text()).get('complete'):
                    raise RuntimeError('unresolved incomplete record: ' + case['name'])
                continue
            remaining.append((batch, case['name']))
    for batch, name in remaining:
        for key in ['product_source_sha256', 'prior_evidence_sha256', 'manifests_sha256']:
            P.verify(frozen[key])
        P.verify(state['harness_sha256'])
        state['in_flight'] = {'1': {'batch': batch, 'case': name}}
        P.write(OUT / 'parallel-state.json', state)
        args = ['python3', str(REPO / 'scripts/worker-build-campaign/round2_run.py'),
                '--batch', batch, '--lane', '1', '--cases', name]
        if batch == 'fresh':
            args.append('--reference')
        print('START SERIAL', batch, name, flush=True)
        with (ARCHIVE / 'serial-controller.log').open('a') as log:
            result = subprocess.run(args, stdout=log, stderr=subprocess.STDOUT)
        if result.returncode:
            state['errors'].append({'lane': 1, 'batch': batch, 'case': name,
                                    'exit_code': result.returncode})
            P.write(OUT / 'parallel-state.json', state)
            raise RuntimeError('serial continuation stopped; inspect retained evidence')
        state['in_flight'] = {}
        state['completed'].append({'lane': 1, 'batch': batch, 'case': name, 'at': now()})
        P.write(OUT / 'parallel-state.json', state)
        print('DONE SERIAL', batch, name, flush=True)
    for key in ['product_source_sha256', 'prior_evidence_sha256', 'manifests_sha256']:
        P.verify(frozen[key])
    P.verify(state['harness_sha256'])
    if len(state['completed']) != 32 or len({(x['batch'], x['case']) for x in state['completed']}) != 32:
        raise RuntimeError('controller does not cover exactly 32 unique trials')
    state['finished_at'] = now()
    P.write(OUT / 'parallel-state.json', state)
    P.write(OUT / 'preservation-check.json', {'at': now(), 'product_source_unchanged': True,
            'prior_evidence_unchanged': True, 'manifests_unchanged': True,
            'prior_files': len(frozen['prior_evidence_sha256']),
            'initial_controller_errors_preserved': 'recovery-20261117',
            'retained_image_recovery_without_rebuild': True})
    print('FOLLOW-UP CAMPAIGN COMPLETE', flush=True)


if __name__ == '__main__':
    main()
