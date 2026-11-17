#!/usr/bin/env python3
"""Recover measurements of existing images without rebuilding interrupted trials."""
import datetime
import importlib.util
import json
import pathlib
import re
import shlex
import subprocess

REPO = pathlib.Path(__file__).absolute().parents[2]
OUT = REPO / 'docs/worker-build-followup-2026-11-17'
ARCHIVE = OUT / 'recovery-20261117'
SPEC = importlib.util.spec_from_file_location('round2', REPO / 'scripts/worker-build-campaign/round2_run.py')
C = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(C)


def run(args, timeout=90):
    return subprocess.check_output(args, text=True, stderr=subprocess.STDOUT, timeout=timeout).strip()


def main():
    env = json.loads((OUT / 'environment.json').read_text())
    manifest = {case['name']: case for case in json.loads((OUT / 'failures-manifest.json').read_text())}
    recovered = []
    for lane, name, ident in [
            (1, 'next-with-absolute-imports', 'fdfb714d-defc-4663-894c-766f00a43afa'),
            (2, 'next-with-typescript', 'e9ca21a2-8a1c-4cfa-af0e-b3d8b44ddbe1')]:
        original = f'zdround2-{lane}-worker'
        helper = f'zdround2-recovery-{lane}'
        path = OUT / 'failures/results' / (name + '.json')
        record = json.loads(path.read_text())
        if record.get('complete'):
            raise RuntimeError('measurement already recovered: ' + name)
        if record != json.loads((ARCHIVE / (name + '.json')).read_text()):
            raise RuntimeError('incomplete trial no longer matches archive')
        if run(['docker', 'inspect', original, '--format', '{{.State.Running}}']) != 'false':
            raise RuntimeError('original worker must stay stopped for exclusive store access')
        label = run(['docker', 'inspect', original, '--format', '{{index .Config.Labels "zero-devops-build-followup"}}'])
        if label != '20261117':
            raise RuntimeError('unowned original worker')
        lane_env = next(x for x in env['lanes'] if x['lane'] == lane)
        args = ['docker', 'run', '-d', '--name', helper,
                '--label', 'zero-devops-build-followup=20261117',
                '--network', lane_env['network'], '--privileged', '--cpus', '1.5',
                '--memory', '2g', '--pids-limit', '512',
                '-v', lane_env['volume'] + ':/var/lib/containers',
                '-v', '/tmp/zd-round2-services:/e2e:ro',
                '-e', 'BUILDAH_ISOLATION=chroot', '--entrypoint', 'sleep',
                'zero-devops-e2e-worker:local', 'infinity']
        run(args)
        try:
            expected = json.loads((OUT / 'frozen-campaign.json').read_text())['binary_sha256']
            if run(['docker', 'exec', helper, 'sha256sum', '/e2e/worker-engine']).split()[0] != expected:
                raise RuntimeError('retained frozen binary mismatch')
            c = C.load_campaign(lane, 'failures')
            c.WORKER = helper
            case = manifest[name]
            since = record['started_at']
            log = run(['docker', 'logs', '--since', since, original])
            # Include Git diagnostics written just after dispatch; only one case
            # was submitted to this lane between dispatch and worker shutdown.
            (c.RESULTS / (name + '.worker.log')).write_text(log + '\n')
            mock = run(['docker', 'logs', '--since', since, c.MOCK])
            (c.RESULTS / (name + '.mock.log')).write_text(mock + '\n')
            row = run(['docker', 'exec', c.DB, 'psql', '-U', 'e2e', '-d', 'e2e',
                       '-At', '-F', '|', '-c',
                       f"SELECT status, COALESCE(error_message,''), COALESCE(output_url,'') FROM deployments WHERE id='{ident}'"])
            if row.split('|')[0] not in ['success', 'canceled']:
                raise RuntimeError('original deployment is not terminal')
            phase = [line for line in log.splitlines() if 'ZERODEVOPS_GIT' in line]
            if not any('verified_sha=' + case['sha'] in line for line in phase):
                raise RuntimeError('missing original exact-SHA checkout evidence')
            timestamps = [line.split('\t')[0] for line in log.splitlines()
                          if ident in line and ('deployment job completed' in line or 'deployment job failed' in line)]
            if not timestamps:
                raise RuntimeError('missing original terminal timestamp')
            began = datetime.datetime.fromisoformat(record['started_at'])
            ended = datetime.datetime.fromisoformat(timestamps[-1].replace('Z', '+00:00'))
            tag = 'localhost/deploy-' + ident + ':latest'
            details = json.loads(run(['docker', 'exec', helper, 'buildah', 'inspect', '--type', 'image', tag]))
            (c.RESULTS / (name + '.worker-image.json')).write_text(json.dumps(details, indent=2) + '\n')
            conf = json.loads(details['Config']).get('config', {}) if isinstance(details.get('Config'), str) else details.get('OCIv1', {}).get('config', {})
            image_manifest = json.loads(details.get('Manifest', '{}'))
            command = shlex.join((conf.get('Entrypoint') or []) + (conf.get('Cmd') or []))
            runtime_name = 'runtime-' + name
            c.dex(helper, ['buildah', 'from', '--name', runtime_name, tag], 60, True)
            checks = C.external_probe_hook(c)
            try:
                runtime = c.probe_runtime(helper, runtime_name, command, case, 'worker')
            finally:
                c.dex(helper, ['buildah', 'rm', runtime_name], 60, True)
            statuses = c.api('/api/queues/%2F/deploy.status/get', {
                'count': 100, 'ackmode': 'ack_requeue_false', 'encoding': 'auto', 'truncate': 50000})
            record['worker'] = {
                'lane': lane, 'worker_container': original, 'runtime_probe_container': helper,
                'broker_vhost': c.VHOST, 'commit_sha': case['sha'], 'deployment_id': ident,
                'state': row.split('|')[0], 'database_row': row,
                'end_to_end_seconds': round((ended - began).total_seconds(), 3),
                'duration_method': 'original dispatch timestamp to original worker terminal log; host polling failed',
                'resource_samples': [], 'resource_sample_note': 'host telemetry failed; no isolated trial samples recovered',
                'configuration_sent': {'executable': 'npm', 'args': ['run', 'build'],
                                       'working_dir': case['path'], 'scanner_policy_version': 'v1'},
                'framework': 'nextjs', 'git_phase_diagnostics': phase,
                'status_messages': statuses, 'image_built': True,
                'image_layer_bytes': sum(layer.get('size', 0) for layer in image_manifest.get('layers', [])),
                'runtime': runtime, 'external_container_http': checks, 'deployment': 'not attempted',
                'mock_upload_bytes': 0,
            }
            record['measurement_recovery'] = {
                'archive': 'recovery-20261117/' + name + '.json',
                'initial_harness_error': record.pop('harness_error'),
                'image_rebuilt': False, 'original_deployment_id': ident,
                'note': 'Retained original final image probed later in a helper sharing the original store/network; original database state retained.'}
            record['worker_classification'] = c.classify(record)
            record['classification'] = record['worker_classification']
            record['complete'] = True
            record['finished_at'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
            c.write_json(path, record)
            recovered.append({'case': name, 'deployment_id': ident, 'state': row.split('|')[0],
                              'image_rebuilt': False, 'local_http': runtime['ready'],
                              'helper': helper})
            print('RECOVERED', name, record['classification'], 'original_state=' + row.split('|')[0], flush=True)
        finally:
            run(['docker', 'stop', '-t', '10', helper])
    (ARCHIVE / 'measurement-recovery.json').write_text(json.dumps({
        'at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
        'recovered_original_images': recovered, 'no_rebuild_or_outcome_substitution': True}, indent=2) + '\n')


if __name__ == '__main__':
    main()
