#!/usr/bin/env python3
"""Focused regression checks for follow-up report calculations (no Docker)."""
import importlib.util
import pathlib
import unittest

PATH = pathlib.Path(__file__).with_name('round2_report.py')
SPEC = importlib.util.spec_from_file_location('round2_report', PATH)
REPORT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(REPORT)


class ReportTests(unittest.TestCase):
    def test_external_status_distinguishes_not_attempted(self):
        self.assertIsNone(REPORT.external({}))
        self.assertIs(REPORT.external({'external_container_http': [
            {'reachable_expected_class': False}]}), False)
        self.assertIs(REPORT.external({'external_container_http': [
            {'reachable_expected_class': False},
            {'reachable_expected_class': True}]}), True)

    def test_image_local_external_and_state_are_distinct(self):
        records = [{'worker': {
            'image_built': True, 'runtime': {'ready': False},
            'state': 'success', 'end_to_end_seconds': 2.0}}, {'worker': {
            'image_built': True, 'runtime': {'ready': True},
            'external_container_http': [{'reachable_expected_class': False}],
            'state': 'success', 'end_to_end_seconds': 4.0}}]
        result = REPORT.measures(records)
        self.assertEqual(result['image_builds'], 2)
        self.assertEqual(result['local_http_passes'], 1)
        self.assertEqual(result['external_container_http_passes'], 0)
        self.assertEqual(result['external_container_http_attempted'], 1)
        self.assertEqual(result['worker_success_states'], 2)
        self.assertEqual(result['sum_case_seconds'], 6.0)
        self.assertEqual(result['median_case_seconds'], 3.0)

    def test_historical_worker_duration_is_not_reference_duration(self):
        result = REPORT.measures([{'prior_worker': {
            'image_built': True, 'state': 'success',
            'end_to_end_seconds': 9.5}}], 'prior_worker')
        self.assertEqual(result['sum_case_seconds'], 9.5)
        self.assertEqual(result['worker_success_states'], 1)
        reference = REPORT.measures([{'reference': {
            'image_built': False, 'seconds': 7.5}}], 'reference')
        self.assertEqual(reference['sum_case_seconds'], 7.5)
        self.assertIsNone(reference['worker_success_states'])

    def test_requested_categories_partition_twenty_cases(self):
        manifest = REPORT.read(REPORT.OUT / 'failures-manifest.json')
        records = []
        for case in manifest:
            old = REPORT.read(REPORT.PRIOR / 'results' / (case['name'] + '.json'))
            records.append({'case': case, 'worker': old['worker'],
                            'prior_worker': old['worker']})
        groups = REPORT.requested_categories(records)
        self.assertEqual(len(groups), 5)
        names = [name for group in groups[:4] for name in group['cases']]
        self.assertEqual(len(names), 20)
        self.assertEqual(len(set(names)), 20)
        self.assertEqual([len(group['cases']) for group in groups], [5, 5, 2, 8, 4])
        original_names = [name for group in groups for name in group['original_blocker_cases']]
        self.assertEqual(len(original_names), 20)
        self.assertEqual(len(set(original_names)), 20)
        self.assertEqual([len(group['original_blocker_cases']) for group in groups], [2, 5, 2, 7, 4])
        self.assertEqual(groups[3]['scope'], 'excluded')
        self.assertTrue(groups[4]['overlaps_other_categories'])
        self.assertEqual(groups[2]['prior']['image_builds'], 2)
        self.assertEqual(groups[2]['prior']['local_http_passes'], 0)
        self.assertTrue(set(groups[4]['cases']).issubset(names))

    def test_concrete_diagnostics_are_not_generic_exit_status(self):
        for name, marker in [('flask-tutorial', 'requirements.txt'),
                             ('gatsby-starter', 'sharp-linuxmusl-x64.node')]:
            record = REPORT.read(REPORT.OUT / 'failures/results' / (name + '.json'))
            issue = REPORT.diagnosis(record, 'worker')
            self.assertEqual(issue['stage'], 'image build')
            self.assertIn(marker, issue['reason'])

    def test_categories_reject_missing_or_extra_cases(self):
        with self.assertRaises(RuntimeError):
            REPORT.requested_categories([])


if __name__ == '__main__':
    unittest.main(verbosity=2)
