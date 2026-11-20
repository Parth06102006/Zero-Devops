#!/usr/bin/env python3
"""Report only the complete, frozen follow-up cohorts; never project a 57-case rate."""
import collections
import csv
import datetime
import hashlib
import io
import json
import pathlib
import re
import statistics

REPO = pathlib.Path(__file__).absolute().parents[2]
OUT = REPO / 'docs/worker-build-followup-2026-11-17'
PRIOR = REPO / 'docs/worker-build-fixes-2026-11-16'


def read(path):
    return json.loads(path.read_text())


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def verify(snapshot):
    changed = [name for name, value in snapshot.items()
               if not (REPO / name).is_file() or digest(REPO / name) != value]
    if changed:
        raise RuntimeError('frozen source/evidence changed: ' + repr(changed))


def identity(case):
    return tuple(case[key] for key in ['name', 'repo', 'path', 'sha'])


def external(runtime):
    checks = runtime.get('external_container_http', [])
    if not checks:
        return None
    return any(check.get('reachable_expected_class', False) for check in checks)


def measures(records, key='worker'):
    runs = [record[key] for record in records if key in record]
    durations = [run['seconds' if key == 'reference' else 'end_to_end_seconds']
                 for run in runs]
    return {
        'cases': len(records),
        'attempted': len(runs),
        'image_builds': sum(bool(run.get('image_built')) for run in runs),
        'local_http_passes': sum(bool(run.get('runtime', {}).get('ready')) for run in runs),
        'external_container_http_passes': sum(external(run) is True for run in runs),
        'external_container_http_attempted': sum(external(run) is not None for run in runs),
        'worker_success_states': sum(run.get('state') == 'success' for run in runs) if key != 'reference' else None,
        'sum_case_seconds': round(sum(durations), 3),
        'median_case_seconds': round(statistics.median(durations), 3) if durations else None,
    }


def requested_categories(records):
    """Primary categories partition the 20 failures; checkout is cross-cutting."""
    groups = [
        ('1', 'Optional Next.js public/ handling', [
            'next-api-routes-rest', 'next-basic-css', 'next-hello-world',
            'next-with-absolute-imports', 'next-with-typescript']),
        ('2', 'Framework detection and output selection', [
            'astro-framework-svelte', 'laravel-skeleton', 'vite-svelte',
            'vite-svelte-ts', 'vue-docs']),
        ('3', 'Production startup and runtime assets', [
            'actix-static-files', 'nestjs-starter']),
        ('4', 'Generation, dependency formats and discovery — excluded', [
            'dotnet-insights-web', 'fastapi-first-steps', 'fastapi-path-params',
            'fastapi-query-params', 'flask-tutorial', 'gatsby-starter',
            'node-prisma-realworld', 'sinatra-simple']),
        ('5', 'Checkout reliability — overlaps categories above', [
            'dotnet-insights-web', 'next-basic-css', 'next-hello-world',
            'next-with-typescript']),
    ]
    lookup = {record['case']['name']: record for record in records}
    primary = [name for number, _, names in groups if number != '5' for name in names]
    if len(primary) != len(set(primary)) or set(primary) != set(lookup):
        raise RuntimeError('requested primary categories do not partition the failure cohort')
    original_blockers = {
        '1': ['next-api-routes-rest', 'next-with-absolute-imports'],
        '2': groups[1][2], '3': groups[2][2],
        '4': [name for name in groups[3][2] if name != 'dotnet-insights-web'],
        '5': groups[4][2],
    }
    original_names = [name for names in original_blockers.values() for name in names]
    if len(original_names) != 20 or len(set(original_names)) != 20:
        raise RuntimeError('original blocker categories do not partition the failure cohort')
    result = []
    for number, title, names in groups:
        selected = [lookup[name] for name in names]
        checkout_passes = [record['case']['name'] for record in selected
                           if any('verified_sha=' + record['case']['sha'] in line
                                  for line in record['worker'].get('git_phase_diagnostics', []))]
        result.append({
            'category': number, 'title': title,
            'scope': 'excluded' if number == '4' else 'implemented',
            'overlaps_other_categories': number == '5',
            'cases': names, 'prior': measures(selected, 'prior_worker'),
            'original_blocker_cases': original_blockers[number],
            'original_blocker_prior': measures([lookup[name] for name in original_blockers[number]], 'prior_worker'),
            'original_blocker_followup': measures([lookup[name] for name in original_blockers[number]]),
            'followup': measures(selected), 'verified_checkout_cases': checkout_passes,
            'fetch_retry_cases': [record['case']['name'] for record in selected
                                 if any('phase=fetch attempt=2 ' in line
                                        for line in record['worker'].get('git_phase_diagnostics', []))],
        })
    return result


def clean(text):
    return re.sub(r'\x1b\[[0-9;]*m', '', text).strip()


def diagnosis(record, kind):
    run = record.get(kind, {})
    batch, name = record['batch'], record['case']['name']
    result_dir = OUT / batch / 'results'
    image = bool(run.get('image_built'))
    ready = bool(run.get('runtime', {}).get('ready'))
    if ready:
        if external(run) is False:
            return {'stage': 'external HTTP', 'reason': 'Local endpoint passed; the separate container request failed.',
                    'evidence': f'{batch}/results/{name}.json'}
        return None
    suffix = '.worker.log' if kind == 'worker' else '.reference-build.log'
    if image:
        suffix = '.' + kind + '-runtime.log'
    path = result_dir / (name + suffix)
    text = clean(path.read_text(errors='replace')) if path.exists() else ''
    markers = ['Error:', 'error:', 'ERROR', 'Cannot find', 'not found', 'No such file',
               'no such file', 'requires', 'unsupported', 'failed', 'Failed', 'Exception',
               'ZERODEVOPS_SSR_MISMATCH', 'not detected', 'no framework detected', 'could not']
    lines = [line.strip() for line in text.splitlines()
             if any(marker in line for marker in markers)]
    # Worker finalization often repeats a generic exit status; use the concrete
    # build/runtime error instead, retaining the raw log as the authority.
    concrete = [line for line in lines if not line.startswith('{') and '\tERROR\t' not in line]
    specific = [line for line in concrete
                if not re.fullmatch(r'Error: (?:building at STEP ".+": )?exit status \d+', line)
                and 'deployment job failed' not in line
                and not line.startswith('Error: error building at STEP')]
    if kind == 'worker' and record.get('worker_classification') == 'missing-code-generation':
        prisma_errors = [line for line in concrete if 'PrismaClient' in line and 'error TS' in line]
        if prisma_errors:
            specific = prisma_errors
    reason = (specific[-1] if specific else concrete[-1] if concrete else lines[-1] if lines else
              run.get('database_row') or run.get('runtime', {}).get('first_response') or
              'See raw trial log for the terminal result.')
    if '{' in reason:
        try:
            details = json.loads(reason[reason.index('{'):])
            reason = details.get('error', reason)
        except (ValueError, TypeError):
            pass
    stage = 'runtime HTTP' if image else 'image build'
    if not image and ('git checkout failed' in text or 'git fetch failed' in text):
        stage = 'source checkout'
    elif not image and ('no framework detected' in text or 'clone/prepare failed' in text):
        stage = 'detection / preparation'
    return {'stage': stage, 'reason': reason[:700],
            'evidence': str(path.relative_to(OUT)) if path.exists() else f'{batch}/results/{name}.json'}


def table_cell(value):
    return str(value).replace('|', '\\|').replace('\n', ' ')


def status(value):
    return 'pass' if value is True else 'fail' if value is False else 'not attempted'


def local_status(run):
    return status(bool(run.get('runtime', {}).get('ready'))) if run.get('image_built') else 'not attempted'


def observed_reference_tools(name):
    path = OUT / 'fresh/results' / (name + '.reference-toolchain-dependencies.log')
    if not path.exists():
        return {'tools': [], 'log': None}
    lines = [clean(line) for line in path.read_text(errors='replace').splitlines()]
    tools = [line for line in lines if re.match(r'^(v\d+\.\d+\.\d+$|\d+\.\d+\.\d+$|go version |rustc |cargo |openjdk version |java version |Apache Maven |Java version:)', line)]
    return {'tools': tools[:5], 'log': str(path.relative_to(OUT))}


def load_records(batch, count):
    manifest = read(OUT / (batch + '-manifest.json'))
    paths = [path for path in (OUT / batch / 'results').glob('*.json')
             if not path.name.endswith('.worker-image.json')]
    records = [read(path) for path in paths]
    if len(manifest) != count or len(records) != count or not all(record.get('complete') for record in records):
        raise RuntimeError(f'{batch}: requires exactly {count} completed trials')
    if len({record['case']['name'] for record in records}) != count:
        raise RuntimeError(batch + ': duplicate result names')
    if {identity(record['case']) for record in records} != {identity(case) for case in manifest}:
        raise RuntimeError(batch + ': result / manifest identity mismatch')
    expected = {case['name']: case for case in manifest}
    for record in records:
        case, run = record['case'], record['worker']
        if record['batch'] != batch or case != expected[case['name']]:
            raise RuntimeError('trial manifest content drift: ' + case['name'])
        if not re.fullmatch(r'[0-9a-f]{40}', case['sha']) or run['commit_sha'] != case['sha']:
            raise RuntimeError('unpinned trial: ' + case['name'])
        if record.get('harness_error'):
            raise RuntimeError('unresolved harness error: ' + case['name'])
        if run.get('runtime', {}).get('ready') and not run.get('image_built'):
            raise RuntimeError('HTTP pass without image: ' + case['name'])
        if run.get('image_built') and not any('verified_sha=' + case['sha'] in line for line in run.get('git_phase_diagnostics', [])):
            raise RuntimeError('built image lacks verified worker checkout evidence: ' + case['name'])
        for kind in ['worker', 'reference']:
            trial = record.get(kind, {})
            if trial.get('image_built'):
                runtime = trial.get('runtime', {})
                if not all(runtime.get(field) is True for field in ['port_closed_before_start', 'port_closed_after_stop']):
                    raise RuntimeError(kind + ' runtime isolation failed: ' + case['name'])
        if batch == 'fresh':
            if 'reference' not in record:
                raise RuntimeError('missing independent reference: ' + case['name'])
            reference = record['reference']
            qualified = bool(reference['image_built'] and reference.get('runtime', {}).get('ready'))
            if bool(record['reference_qualified']) != qualified:
                raise RuntimeError('reference qualification mismatch: ' + case['name'])
            if not reference.get('source_sha_verified'):
                raise RuntimeError('unverified reference source: ' + case['name'])
    order = {case['name']: index for index, case in enumerate(manifest)}
    return sorted(records, key=lambda record: order[record['case']['name']])


def main():
    outputs = [OUT / name for name in ['REPORT.md', 'summary.json', 'metrics.csv']]
    if any(path.exists() for path in outputs):
        raise RuntimeError('final report artifacts exist; preserve rather than overwrite')
    frozen = read(OUT / 'frozen-campaign.json')
    for key in ['product_source_sha256', 'prior_evidence_sha256', 'manifests_sha256']:
        verify(frozen[key])
    current_source = list((REPO / 'worker-server').rglob('*.go')) + list((REPO / 'worker-server/templates').rglob('*.tmpl')) + [REPO / 'worker-server/go.mod', REPO / 'worker-server/go.sum']
    if {str(path.relative_to(REPO)) for path in current_source if path.is_file()} != set(frozen['product_source_sha256']):
        raise RuntimeError('worker source file inventory changed after freeze')
    environment = read(OUT / 'environment.json')
    provenance = read(OUT / 'build-provenance.json')
    if environment['product_commit'] != frozen['product_commit']:
        raise RuntimeError('environment / source commit mismatch')
    if provenance['commands']['git rev-parse HEAD'] != frozen['product_commit'] or provenance['binaries_sha256']['/tmp/zd-round2-worker'] != frozen['binary_sha256']:
        raise RuntimeError('build provenance / frozen product mismatch')
    if not re.fullmatch(r'[0-9a-f]{40}', frozen['product_commit']):
        raise RuntimeError('product source commit is not pinned')
    for lane in environment['lanes']:
        if lane['initial_images'] or lane['binary_sha256'] != frozen['binary_sha256']:
            raise RuntimeError('worker initial storage / binary mismatch')
    state = read(OUT / 'parallel-state.json')
    if 'harness_sha256' in state:
        verify(state['harness_sha256'])
    preservation = read(OUT / 'preservation-check.json')
    if not state.get('finished_at') or state.get('in_flight') or state.get('errors'):
        raise RuntimeError('campaign controller is unfinished or has unresolved errors')
    if not all(preservation.get(key) for key in ['product_source_unchanged', 'prior_evidence_unchanged', 'manifests_unchanged']):
        raise RuntimeError('campaign preservation check did not pass')
    supplemental_path = OUT / 'supplemental-evidence-audit.json'
    supplemental = read(supplemental_path)
    if not supplemental.get('passed'):
        raise RuntimeError('supplemental source/metadata audit did not pass')
    verify(supplemental['audited_artifact_sha256'])
    failures, fresh = load_records('failures', 20), load_records('fresh', 12)
    old = [read(PRIOR / 'results' / (record['case']['name'] + '.json')) for record in failures]
    if {identity(record['case']) for record in old} != {identity(record['case']) for record in failures}:
        raise RuntimeError('failure cohort differs from prior exact identities')
    all_old = [read(path) for path in (PRIOR / 'results').glob('*.json') if not path.name.endswith('.worker-image.json')]
    previous_failures = {identity(record['case']) for record in all_old
                         if record.get('complete') and record['classification'] != 'build-and-runtime-pass'}
    if previous_failures != {identity(record['case']) for record in failures}:
        raise RuntimeError('failure cohort omits or adds previous failures')
    for record, prior in zip(failures, old):
        if not record['reference_qualified'] or record['prior_worker'] != prior['worker'] or record['prior_classification'] != prior['classification']:
            raise RuntimeError('failure comparison provenance differs from prior record: ' + record['case']['name'])
    if {(record['case']['repo'], record['case']['path']) for record in fresh} & {
            (case['repo'], case['path']) for case in read(REPO / 'docs/worker-build-campaign-50/manifest.json')}:
        raise RuntimeError('fresh case appeared in original candidate set')
    cold = read(OUT / 'cold-pulls.json')
    native = read(OUT / 'native-integration.json')
    if cold['initial_images'] or native['exit_code'] != 0:
        raise RuntimeError('fresh-store or live regression validation failed')
    suite_path = OUT / provenance.get('worker_suite_log', 'worker-suite-final.log')
    suite = suite_path.read_text()
    if not re.search(r'^ok\s+.*internal/deployments\s', suite, re.MULTILINE) or re.search(r'^FAIL(?:\s|$)', suite, re.MULTILINE):
        raise RuntimeError('complete worker suite evidence did not pass')
    acceptance_path = OUT / 'worker-suite-final-acceptance.log'
    acceptance = acceptance_path.read_text()
    if not re.search(r'^ok\s+.*internal/deployments\s', acceptance, re.MULTILINE) or re.search(r'^FAIL(?:\s|$)', acceptance, re.MULTILINE):
        raise RuntimeError('final acceptance worker suite did not pass')
    report_tests_path = OUT / 'report-regression-tests.log'
    if not re.search(r'^OK$', report_tests_path.read_text(), re.MULTILINE):
        raise RuntimeError('report calculation regression tests did not pass')
    previous_validations = []
    for path in sorted(OUT.glob('pre-campaign-verification-*/native-integration.json')):
        attempt = read(path)
        previous_validations.append({'directory': str(path.parent.relative_to(OUT)), 'native_integration': attempt,
                                     'product_commit': read(path.parent / 'frozen-campaign.json')['product_commit']})
    qualified = [record for record in fresh if record['reference_qualified']]
    requirements = read(OUT / 'fresh-dependency-requirements.json')
    if {identity(item) for item in requirements} != {identity(record['case']) for record in fresh}:
        raise RuntimeError('fresh dependency requirements differ from frozen case identities')
    exclusions = [{'case': record['case']['name'], 'reference': diagnosis(record, 'reference')}
                  for record in fresh if not record['reference_qualified']]
    diagnoses = [{'batch': record['batch'], 'case': record['case']['name'], 'trial': key,
                  **issue}
                 for record in failures + fresh for key in ['worker', 'reference']
                 if key in record and (issue := diagnosis(record, key))]
    recovered = [record['case']['name'] for record in failures
                 if record['worker']['image_built'] and record['worker'].get('runtime', {}).get('ready')]
    summary = {
        'recorded_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
        'product_commit': frozen['product_commit'], 'binary_sha256': frozen['binary_sha256'],
        'failure_cohort': {'prior': measures(old), 'followup': measures(failures),
                           'recovered_local_http_cases': recovered,
                           'outcomes': dict(collections.Counter(record['worker_classification'] for record in failures))},
        'fresh_cohort': {'all_worker_trials': measures(fresh), 'independent_reference_trials': measures(fresh, 'reference'),
                         'reference_qualified_worker_trials': measures(qualified), 'reference_inconclusive': exclusions,
                         'observed_reference_tools': {record['case']['name']: observed_reference_tools(record['case']['name']) for record in fresh},
                         'all_worker_outcomes': dict(collections.Counter(record['worker_classification'] for record in fresh))},
        'measurement_recoveries': [{'case': record['case']['name'],
                                    'original_worker_state': record['worker']['state'],
                                    **record['measurement_recovery']}
                                   for record in failures + fresh if 'measurement_recovery' in record],
        'worker_terminal_state_gaps': [{'case': record['case']['name'],
                                       'state': record['worker']['state'],
                                       'database_row': record['worker']['database_row']}
                                      for record in failures + fresh
                                      if record['worker'].get('image_built') and record['worker']['state'] != 'success'],
        'requested_categories': requested_categories(failures),
        'full_57_case_retest': 'not performed on follow-up source; no overall improvement rate claimed',
        'deployment': 'not attempted', 'publication': 'not attempted; fake upload sink only',
        'validation': {'worker_suite': suite_path.name, 'worker_suite_passed': True,
                       'final_acceptance_worker_suite': acceptance_path.name,
                       'report_regression_tests': report_tests_path.name,
                       'supplemental_evidence_audit': supplemental_path.name,
                       'native_integration': native, 'cold_image_pulls_attempted': len(cold['pulls']),
                       'cold_image_pulls_passed': sum(pull['exit_code'] == 0 for pull in cold['pulls']),
                       'previous_pre_campaign_attempts': previous_validations},
        'preservation': preservation, 'diagnoses': diagnoses,
        'build_provenance': provenance,
        'prior_evidence_archive_commit': '4c07e71',
        'resources': {'lanes': environment['lanes'], 'docker_capacity': environment['docker_capacity']},
        'harness_sha256': state.get('harness_sha256', {}),
        'commands': provenance['build_commands'] + ['python3 scripts/worker-build-campaign/round2_environment.py setup',
                     'python3 scripts/worker-build-campaign/round2_validate.py',
                     'python3 scripts/worker-build-campaign/round2_reverify.py --product-commit ' + frozen['product_commit'],
                     "docker exec -e RUN_NATIVE_BUILD_TESTS=true -w /app zdround2-2-worker timeout --kill-after=10s 1200s /e2e/deployment-tests -test.v -test.run 'TestDockerBuild_WithDockerfile|TestNativeBuildIntegration'",
                     'python3 scripts/worker-build-campaign/round2_parallel.py --freeze',
                     'python3 scripts/worker-build-campaign/round2_parallel.py',
                     'python3 scripts/worker-build-campaign/round2_recover_measurements.py',
                     'python3 scripts/worker-build-campaign/round2_finish.py',
                     'GOCACHE=/tmp/zd-fix-builds-gocache go test -count=1 ./...  # final acceptance',
                     'python3 scripts/worker-build-campaign/round2_audit.py',
                     'python3 scripts/worker-build-campaign/round2_report_test.py',
                     'python3 scripts/worker-build-campaign/round2_report.py'],
    }
    before, after = summary['failure_cohort']['prior'], summary['failure_cohort']['followup']
    fresh_all, ref, fresh_qualified = (summary['fresh_cohort'][key] for key in
                                     ['all_worker_trials', 'independent_reference_trials', 'reference_qualified_worker_trials'])
    lines = ['# Follow-up build and runtime campaign', '',
             f'Tested product commit: `{frozen["product_commit"]}`; binary SHA256 `{frozen["binary_sha256"]}`.', '',
             '| Cohort / trial | Cases | Images created | Local HTTP passed | External-container HTTP passed / attempted |',
             '|---|---:|---:|---:|---:|']
    for label, measured in [('Previous failures, prior source 20544b6', before),
                            ('Previous failures, follow-up source', after), ('Fresh cases, worker', fresh_all),
                            ('Fresh cases, independent reference', ref), ('Fresh reference-qualified cases, worker', fresh_qualified)]:
        lines.append(f'| {label} | {measured["cases"]} | {measured["image_builds"]} | {measured["local_http_passes"]} | '
                     f'{measured["external_container_http_passes"]} / {measured["external_container_http_attempted"]} |')
    lines += ['', 'The twenty-case cohort contains every previous image/runtime failure at the same app/path/SHA. Its prior result was two images and zero local HTTP passes. Fresh cases are a separate compatibility measurement. The other 37 previously successful cases were not rerun on this source: no new overall 57-case rate is established. Image creation is inspected independently of worker success status. A local HTTP response, a request from a separate container, and a deployed application are distinct outcomes. Deployment and publication were not attempted.', '',
              '## Changes and scope', '',
              '- Next.js normalizes optional public/ inside the builder and validates .next before runtime packaging.',
              '- Framework ownership is resolved before supporting tools: Astro/Svelte integrations, SvelteKit adapters, VitePress output, and Laravel frontend/backend packaging. Ambiguous or unsupported choices retain actionable errors. Explicit framework and static output overrides are supported.',
              '- Nest production startup uses the validated compiled entry; explicit runtime command, assets, and working directory are supported. Rust conventional static/ assets are carried into the runtime image. Asset paths and symlinks are checked inside the selected app/repository context; packaged assets receive the final runtime user ownership while retaining restrictive source permissions.',
              '- Git fetch and checkout have separate bounded budgets; transient fetch failures get one retry, permanent failures do not. Fetch and checkout remain pinned to the accepted SHA, and HEAD is verified before building. Phase durations are recorded in each worker result.',
              '- Point 4 remains excluded: no Prisma/code-generation step, new Python dependency format, native install-script relaxation, or missing-manifest discovery expansion. Application loopback bindings are unchanged. Laravel retains the existing PHP built-in server policy for local compatibility; no production hosting stack is certified.', '',
              '## Validation and execution', '',
              f'- Full worker-server suite passed before freeze: [{suite_path.name}]({suite_path.name}). Final acceptance suite also passed: [{acceptance_path.name}]({acceptance_path.name}). Host tests conditionally skip live Buildah checks; the separate container validation executes them.',
              f'- Report calculation regression tests passed: [{report_tests_path.name}]({report_tests_path.name}); they check category partitioning, historical-worker durations, and separate image/local/external/status measures.',
              f'- Live native integration passed (exit {native["exit_code"]}): [native-integration.log](native-integration.log). It includes real image execution and Rust workspace asset packaging.',
              f'- Fresh-store pulls: {summary["validation"]["cold_image_pulls_passed"]}/{summary["validation"]["cold_image_pulls_attempted"]}. See [cold-pulls.json](cold-pulls.json); unavailable images remain failures in the relevant reference/worker trials.',
              '- Two isolated worker lanes each have 1.5 CPUs, 2 GiB RAM, 512 PIDs, a separate build store/network/broker vhost/mock endpoint. Only the run-owned DB and broker are shared. The VM exposes about 5.6 GiB RAM, so two builders fit within the approved maximum; four were not launched.',
              '- Failure and fresh trials alternate in predeclared order while both cohorts have pending work. Every fresh case runs its independently declared reference compilation/start command before the worker trial. A failed reference does not suppress the worker trial or cause case replacement.',
              '- Source, template, binary, manifests and prior-evidence fingerprints were frozen before dispatch and rechecked afterward. Fresh-source inventory metadata and dependency-requirements JSON were not in the original controller freeze; a separate final audit verifies exact file inventories, byte hashes, executable/symlink modes, pinned upstream tree/blob linkage and dependency metadata. That is an additional acceptance check, not retroactive freeze evidence. See [supplemental-evidence-audit.json](supplemental-evidence-audit.json), [frozen-campaign.json](frozen-campaign.json), [environment.json](environment.json), [parallel-state.json](parallel-state.json), and [preservation-check.json](preservation-check.json).',
              '- The Go worker binary/test executable and templates are mounted into the retained test runtime image. This does not validate a rebuild of the worker Dockerfile. Reference-toolchain/dependency logs show actual versions in the reference images; worker-dependencies logs inspect final Node runtime images. Static worker images retain the static server rather than the app builder dependency tree, so an empty/error npm listing there does not establish builder versions. Reference resolutions must not be presented as exact worker-builder resolutions. Declared source requirements are in fresh-dependency-requirements.json.', '',
              f'- Host build toolchain: `{provenance["commands"]["go version"]}`. Exact compile commands and worker/test binary hashes are recorded in [build-provenance.json](build-provenance.json).', '',
              'Commands used by the harness:', '', '```text', *summary['commands'], '```', '',
              '## Controller interruption and retained-image recovery', '',
              'The first controller stopped after a 60-second storage-size telemetry timeout and a 20-second database poll timeout. Both affected Next applications had already created final images; the original workers subsequently stopped. Their initial incomplete records, controller state and full worker logs are retained under recovery-20261117/. The retained tagged images were inspected and HTTP-tested later in helpers with exclusive access to their original stores/networks; neither image was rebuilt and no completed outcome was replaced. The TypeScript job retains its original canceled database state after a broker channel closed, even though its image and HTTP probes passed. These are measured build/runtime outcomes, not successful finalization or deployment. The remaining five unsubmitted cases ran serially on lane 1; the recovered lane-2 queue item was left untouched with its worker stopped to avoid duplicate dispatch.', '',
              'Recovered trials lack the failed host telemetry samples. Their durations use original dispatch-to-terminal-log timestamps, while runtime probes occurred later. This scheduling/infrastructure interruption limits timing comparisons further. See recovery-20261117/measurement-recovery.json and each record’s measurement_recovery field.', '',
              '## Failure-cohort comparison', '',
              '| Case | Prior image / local HTTP | Follow-up image / local HTTP | External HTTP | Worker outcome |',
              '|---|---|---|---|---|']
    for record, prior in zip(failures, old):
        w, p = record['worker'], prior['worker']
        lines.append('| ' + ' | '.join([record['case']['name'],
                     f'{status(p.get("image_built"))} / {local_status(p)}',
                     f'{status(w.get("image_built"))} / {local_status(w)}',
                     status(external(w)), record['worker_classification']]) + ' |')
    lines += ['', '## Results by your requested categories', '',
              '### Original primary blocker categories', '',
              '| Category | Cases | Prior images / local HTTP | Follow-up images / local HTTP | External HTTP passes / attempted |',
              '|---|---:|---:|---:|---:|']
    for group in summary['requested_categories']:
        p, w = group['original_blocker_prior'], group['original_blocker_followup']
        lines.append(f'| {group["category"]}. {group["title"].replace(" — overlaps categories above", "")} | {len(group["original_blocker_cases"])} | '
                     f'{p["image_builds"]} / {p["local_http_passes"]} | '
                     f'{w["image_builds"]} / {w["local_http_passes"]} | '
                     f'{w["external_container_http_passes"]} / {w["external_container_http_attempted"]} |')
    lines += ['', 'These rows partition the twenty cases by their original primary blocker. Three original checkout failures later reached the optional Next public/ error in controlled repeats; .NET later reached excluded discovery. Current terminal diagnoses below remain separate from original blocker attribution.', '',
              '### Expanded targeted scope and cross-cutting checkout checks', '',
              '| Category | Scope | Cases | Prior images / local HTTP | Follow-up images / local HTTP | External HTTP passes / attempted |',
              '|---|---|---:|---:|---:|---:|']
    for group in summary['requested_categories']:
        p, w = group['prior'], group['followup']
        lines.append(f'| {group["category"]}. {group["title"]} | {group["scope"]} | {len(group["cases"])} | '
                     f'{p["image_builds"]} / {p["local_http_passes"]} | '
                     f'{w["image_builds"]} / {w["local_http_passes"]} | '
                     f'{w["external_container_http_passes"]} / {w["external_container_http_attempted"]} |')
    lines += ['', 'Categories 1–4 partition all twenty historical failures. Category 5 is cross-cutting: its four cases also appear in the other rows and must not be added again. The Next category includes the three controlled timeout repeats that previously reached the same optional-public packaging error; this is not a claim that all five primary trials originally reached that error.', '']
    for group in summary['requested_categories']:
        lines.append(f'- **{group["category"]}. {group["title"]}:** ' + ', '.join('`' + name + '`' for name in group['cases']) + '.')
    checkout = summary['requested_categories'][-1]
    lines += ['', f'Checkout verification reached the exact pinned HEAD for {len(checkout["verified_checkout_cases"])}/4 original Git-timeout cases. '
              f'Second fetch attempts were observed in {len(checkout["fetch_retry_cases"])} of these cases. '
              'Successful checkout is not image/runtime success and the later environment does not isolate the causal effect of the policy change.', '']
    lines += ['', '## Fresh compatibility cohort', '',

              '| Case | Stack | Reference image / local HTTP | Qualified | Worker image / local HTTP | Worker external HTTP |',
              '|---|---|---|---|---|---|']
    for record in fresh:
        w, r, c = record['worker'], record['reference'], record['case']
        lines.append('| ' + ' | '.join([c['name'], table_cell(c.get('stack', c['kind'])),
                     f'{status(r.get("image_built"))} / {local_status(r)}',
                     'yes' if record['reference_qualified'] else 'no',
                     f'{status(w.get("image_built"))} / {local_status(w)}',
                     status(external(w))]) + ' |')
    lines += ['', 'Reference-inconclusive cases are excluded only from the fresh reference-qualified subset; all twelve remain visible above and in the all-case totals. Full immutable upstream SHAs, commands, probes and source paths are in the two manifests. No app source was changed to force a pass.', '',
              '### Dependency and toolchain rotation', '',
              'These are declared project requirements, not a claim that every range resolved to the same version in reference and worker images.', '',
              '| New case | Declared toolchain / dependency variation | Observed reference tools |', '|---|---|---|']
    for item in requirements:
        if item['kind'] == 'node':
            declared = '; '.join(name + ' ' + version for name, version in item.get('dependencies', {}).items())
            tools = '; '.join(name + ' ' + item['devDependencies'][name] for name in ['typescript', 'vite'] if name in item.get('devDependencies', {}))
            if tools:
                declared += '; ' + tools
            declared = 'Node ' + item.get('engines', {}).get('node', '(no engine declared)') + '; ' + declared
        elif item['kind'] == 'go':
            direct = re.findall(r'(github\.com/gofiber/fiber/v3)\s+(v\S+)', item['go_mod'])
            declared = 'Go ' + item['go_requirement'] + '; ' + '; '.join(name + ' ' + version for name, version in direct)
        elif item['kind'] == 'java':
            declared = 'Java source ' + item['java_requirement'] + '; Spring Boot ' + item['spring_boot_parent'] + '; ' + ', '.join(
                dependency['artifact'] for dependency in item['dependencies'] if dependency.get('scope') == 'compile')
        else:
            dependencies = item['cargo_manifest'].get('dependencies', {})
            declared = 'Rust workspace; ' + '; '.join(name + ' ' + (value if isinstance(value, str) else value.get('version', 'local path ' + value.get('path', '')))
                                                      for name, value in dependencies.items())
        observed = summary['fresh_cohort']['observed_reference_tools'][item['name']]
        tools = '; '.join(observed['tools']) or 'not recorded'
        if observed['log']:
            tools += ' [log](' + observed['log'] + ')'
        lines.append('| ' + item['name'] + ' | ' + table_cell(declared) + ' | ' + table_cell(tools) + ' |')
    lines += ['', 'See fresh-dependency-requirements.json for the full manifests/requirements, and each *.reference-toolchain-dependencies.log for observed reference tools and installed Node packages. The worker builder dependency tree is not reconstructed from static final images.', '',
              '## Remaining failures and external reachability gaps', '',
              '| Batch / case | Trial | Stage | Recorded diagnostic | Evidence |',
              '|---|---|---|---|---|']
    for item in diagnoses:
        lines.append('| ' + ' | '.join([item['batch'] + ' / ' + item['case'], item['trial'], item['stage'],
                     table_cell(item['reason']), f'[log]({item["evidence"]})']) + ' |')
    if previous_validations:
        lines += ['', '## Pre-campaign verification correction', '',
                  'Before application jobs were dispatched, the first live Rust workspace asset fixture built its image but failed at runtime with PermissionDenied reading a 0600 asset through 0750 directories. The image had copied root ownership before switching to the app user. The worker was corrected to preserve asset modes and assign the final runtime user ownership, then rebuilt, tested and frozen again before the application cohorts. This failed verification is retained separately and is not counted as an application-case failure.']
        for attempt in previous_validations:
            directory = attempt['directory']
            lines.append(f'- Prior source `{attempt["product_commit"]}`: native validation exit {attempt["native_integration"]["exit_code"]}; [retained log]({directory}/native-integration.log), with its original freeze/environment/provenance records in the same directory.')
        lines += ['- The ten cold pulls were retained because this correction did not change the selected base-image set. Application results and final live validation use the replacement frozen product shown above.', '']
    lines += ['', 'Diagnostics are excerpts from the terminal trial logs, not promises that removing one blocker will make the application pass. Full raw logs and structured worker/reference records remain authoritative.', '',
              '## Interpretation and limitations', '',
              '- Several cases share upstream repositories, framework implementations and caches; these are correlated case outcomes, not independent platform success probabilities. The fresh batch adds different app paths, integrations and manifest/toolchain requirements, rather than randomly sampling all applications.',
              '- Node installs without committed locks are explicitly opted in and keep install scripts disabled. Resolution can differ across dates, between reference and worker trials, and between builder/runtime installs. Image tags also move. Source SHAs are pinned; all dependency trees and base digests are not globally frozen.',
              '- Checkout recovery occurs under later network/cache/scheduling conditions. Before/after outcomes do not isolate the causal effect of the timeout/retry policy; per-phase fetch/checkout durations and attempts are retained for inspection.',
              '- Cold pulls prove image availability at validation time from initially empty lane-2 storage. Lane 2 was then warmed by pulls/live fixtures, and fresh reference trials precede worker trials in each lane/store. Base-image reuse is supported; exact intermediate-layer/dependency-cache reuse is not established merely by sharing storage. Timings are not uniformly cold or a controlled speed benchmark. Summed case durations are not parallel wall time.',
              '- Worker end-to-end timings include checkout, compilation, image archiving and finalization, and stop before the separately recorded smoke probe. Reference durations include their build/probe/cleanup sequence. These duration fields are not directly comparable. CPU counters and memory.peak can be cumulative over a worker lifetime; their samples are retained as telemetry, not presented as isolated per-app CPU cost or memory peaks. The configured per-worker resource ceilings are the authoritative limits.',
              '- HTTP qualification uses one accepted local status; eight warm requests are recorded separately and reviewed for instability without redefining the frozen readiness outcome. External probes are conditional on local readiness and check one 2xx/3xx response; absent probes are not attempted, not failures, and external response bodies are not preserved or validated. Successful runtime logs may end with killed/exit-status messages from intentional probe shutdown, not a startup failure.',
              '- References use a 1,200-second build budget and builder-style images retaining source/tools; worker build/save uses a 15-minute budget and its selected runtime packaging. These are not equivalent production configurations.',
              '- HTTP probes check only the declared endpoint/status. They do not exercise full application features, databases, auth flows, companion frontends, browser CORS, persistent state, production ingress, TLS, or publication. The separate-container probe reaches the Buildah worker network address; it is stronger reachability evidence than the parent-namespace local probe, but still is not a deployed service.',
              '- The campaign exercises Linux containers on the current Docker amd64 environment. It does not establish Windows, macOS-native, ARM, other CPU architectures, or every hosting-platform compatibility.',
              '- Existing loopback-only application bindings may pass local HTTP and fail the separate-container probe. This distinction is preserved rather than changing application source.',
              '- Detection/configuration support remains bounded: inherited or dynamically computed compiler/adapter settings and custom runtime output layouts are not universally resolved. Supported explicit framework/output/runtime overrides cover the documented forms; they do not establish compatibility with every layout.',
              '- Previous stopped workers/build stores and all prior evidence were retained. Cleanup, if performed after this report, is scoped to follow-up-owned resources and is recorded separately in cleanup.json. No global prune or unrelated-resource cleanup is authorized.', '']
    lines += [f'The prior campaign evidence/harness archive was committed separately as `4c07e71`. Evidence-only commits do not change the frozen tested product identity `{frozen["product_commit"]}`; later harness/report commits are likewise not application source retests.', '']
    csv_file = io.StringIO(newline='')
    writer = csv.writer(csv_file)
    writer.writerow(['batch', 'name', 'repo', 'path', 'sha', 'reference_qualified', 'reference_image', 'reference_local_http',
                     'reference_external_http', 'prior_image', 'prior_local_http', 'worker_image', 'worker_local_http',
                     'worker_external_http', 'worker_state', 'worker_outcome', 'worker_seconds', 'image_layer_bytes'])
    for record in failures + fresh:
        c, w, r, p = record['case'], record['worker'], record.get('reference', {}), record.get('prior_worker', {})
        writer.writerow([record['batch'], c['name'], c['repo'], c['path'], c['sha'], record['reference_qualified'],
                         r.get('image_built', ''), r.get('runtime', {}).get('ready', ''), external(r),
                         p.get('image_built', ''), p.get('runtime', {}).get('ready', '') if p else '',
                         w['image_built'], w.get('runtime', {}).get('ready', False), external(w), w['state'],
                         record['worker_classification'], w['end_to_end_seconds'], w.get('image_layer_bytes', '')])
    for path, content in zip(outputs, ['\n'.join(lines), json.dumps(summary, indent=2) + '\n', csv_file.getvalue()]):
        path.write_text(content)
    print(json.dumps({key: summary[key] for key in ['product_commit', 'failure_cohort', 'fresh_cohort', 'deployment']}, indent=2))


if __name__ == '__main__':
    main()
