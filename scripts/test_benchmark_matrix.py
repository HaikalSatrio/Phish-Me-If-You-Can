import argparse
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from benchmark_matrix import Runner, metrics, save, digest


CASE = {'id': 'Q1', 'name': 'Mira', 'split': 'development', 'reference_names': ['Mira'],
        'reference_article': 'https://en.wikipedia.org/wiki/Mira'}


class MatrixTests(unittest.TestCase):
    def test_failed_retrieval_is_not_a_semantic_abstention(self):
        got = metrics({'search': {'status': 'FAILED'}, 'candidates': []}, CASE)
        self.assertEqual(got['collection_status'], 'FAILED')
        self.assertEqual(got['candidates'], 0)
        self.assertEqual(got['model_proposals'], 0)

    def test_cross_candidate_and_nonliteral_claims_are_visible(self):
        got = metrics({'evidence': [{'id': 'e', 'candidate_id': 'other', 'value': 'Mira is a singer.'}],
                       'relations': [{'candidate_id': 'mira', 'evidence_id': 'e', 'quote': 'Mira is a doctor.'}]}, CASE)
        self.assertEqual(got['grounding_reference_errors'], 1)

    def test_resume_verifies_snapshots_without_reissuing_requests(self):
        with tempfile.TemporaryDirectory() as tmp:
            args = argparse.Namespace(out=Path(tmp))
            runner = Runner(args, [CASE])
            folder = args.out / 'development' / 'Q1'
            snapshot = folder / 'web-rules.json'
            save(snapshot, {'search': {'status': 'FAILED'}, 'candidates': []})
            with patch.object(runner, 'run_command', side_effect=AssertionError('unexpected subprocess')):
                first = runner.task(CASE, 'web')
                self.assertTrue(first['recovered'])
                self.assertEqual(first['snapshot_sha256'], digest(snapshot))
                self.assertEqual(runner.task(CASE, 'web'), first)
                save(snapshot, {'search': {'status': 'EMPTY'}})
                with self.assertRaisesRegex(ValueError, 'snapshot changed'):
                    runner.task(CASE, 'web')


if __name__ == '__main__':
    unittest.main()
