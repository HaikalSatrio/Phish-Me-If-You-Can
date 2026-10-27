"""Checks for measurement errors that can overstate collector consistency."""
import unittest
from report_consistency import jaccard, urlkey


class MeasurementTests(unittest.TestCase):
    def test_two_empty_runs_are_not_perfect_retrieval(self):
        self.assertIsNone(jaccard(set(), set()))
        self.assertEqual(jaccard({"a"}, set()), 0)

    def test_jaccard_uses_union_not_smaller_result_count(self):
        self.assertEqual(jaccard({"a", "b"}, {"a", "c"}), 1/3)

    def test_url_identity_does_not_collapse_different_people_or_pages(self):
        self.assertEqual(urlkey("https://www.github.com/Rich-Harris/"), urlkey("https://github.com/rich-harris"))
        self.assertNotEqual(urlkey("https://en.wikipedia.org/wiki/Rich_Harris"), urlkey("https://en.wikipedia.org/wiki/Judith_Rich_Harris"))
        self.assertNotEqual(urlkey("https://example.org/A"), urlkey("https://example.org/a"))


if __name__ == "__main__":
    unittest.main()
