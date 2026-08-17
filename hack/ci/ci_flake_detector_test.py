#!/usr/bin/env python3
"""
Unit tests for ci_flake_detector.py

Run with:
    python3 -m pytest hack/ci/ci_flake_detector_test.py -v
    python3 hack/ci/ci_flake_detector_test.py
"""

import json
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))

import ci_flake_detector


# --- Test fixtures ---

def _make_record(
    job_name="test-job",
    commit_sha="abc123",
    created_at="2026-08-01T10:00:00Z",
    category="test_assertion_failure",
    run_id=1,
    run_number=1,
    run_attempt=1,
    evidence="",
):
    """Create a minimal classified failure record for testing."""
    return {
        "run_id": run_id,
        "run_number": run_number,
        "run_attempt": run_attempt,
        "workflow": "ci",
        "job_name": job_name,
        "job_id": 90000 + run_id,
        "step_name": "Run test",
        "branch": "main",
        "commit_sha": commit_sha,
        "commit_message": f"commit {commit_sha}",
        "event": "push",
        "created_at": created_at,
        "html_url": f"https://example.com/run/{run_id}",
        "failure_sections": [],
        "normalized_log_excerpt": "",
        "raw_log_bytes": 0,
        "classification": {
            "category": category,
            "category_description": "",
            "confidence": 0.9,
            "rule_name": "test",
            "matched_text": "",
            "evidence": evidence,
            "all_matches_count": 1,
        },
    }


# Scenario 1: Classic flake — same job fails on many different commits
# and dates with infrastructure-correlated categories.
FLAKY_JOB_RECORDS = [
    _make_record(
        job_name="sys local root fedora / lima",
        commit_sha=f"commit_{i:040d}",
        created_at=f"2026-08-{i+1:02d}T10:00:00Z",
        category="network_error",
        run_id=i,
        run_number=100 + i,
    )
    for i in range(8)
]

# Scenario 2: Deterministic bug — same job fails on a single commit
# with assertion failure.
BUG_JOB_RECORDS = [
    _make_record(
        job_name="unit-test-fedora",
        commit_sha="deadbeef" * 5,
        created_at="2026-08-10T10:00:00Z",
        category="test_assertion_failure",
        run_id=100,
        run_number=200,
    ),
]

# Scenario 3: Retry-detected flake — run_attempt > 1.
RETRY_FLAKE_RECORDS = [
    _make_record(
        job_name="int local rootless ubuntu / lima",
        commit_sha=f"retry_{i:040d}",
        created_at=f"2026-08-{i+1:02d}T12:00:00Z",
        category="timeout",
        run_id=200 + i,
        run_number=300 + i,
        run_attempt=2,
    )
    for i in range(5)
]

# Scenario 4: Mixed categories across multiple commits.
MIXED_RECORDS = [
    _make_record(
        job_name="sys remote root fedora / lima",
        commit_sha=f"mixed_{i:040d}",
        created_at=f"2026-08-{i+1:02d}T14:00:00Z",
        category=["network_error", "test_assertion_failure", "timeout",
                  "infrastructure_failure", "image_pull_failure"][i % 5],
        run_id=300 + i,
        run_number=400 + i,
    )
    for i in range(10)
]


class TestExtractDate(unittest.TestCase):
    """Tests for _extract_date."""

    def test_iso_timestamp(self):
        self.assertEqual(
            ci_flake_detector._extract_date("2026-08-14T15:09:38Z"),
            "2026-08-14",
        )

    def test_short_string(self):
        self.assertEqual(ci_flake_detector._extract_date("2026"), "")

    def test_empty_string(self):
        self.assertEqual(ci_flake_detector._extract_date(""), "")


class TestComputeFlakeSignals(unittest.TestCase):
    """Tests for compute_flake_signals."""

    def test_high_commit_diversity(self):
        signals = ci_flake_detector.compute_flake_signals(FLAKY_JOB_RECORDS)
        self.assertGreaterEqual(signals.commit_diversity, 0.7)

    def test_low_commit_diversity_single_commit(self):
        signals = ci_flake_detector.compute_flake_signals(BUG_JOB_RECORDS)
        self.assertEqual(signals.commit_diversity, 0.0)

    def test_category_correlation_network(self):
        signals = ci_flake_detector.compute_flake_signals(FLAKY_JOB_RECORDS)
        self.assertGreater(signals.category_flake_correlation, 0.5)

    def test_category_correlation_assertion(self):
        signals = ci_flake_detector.compute_flake_signals(BUG_JOB_RECORDS)
        # Bug-correlated categories should push correlation down.
        self.assertLess(signals.category_flake_correlation, 0.5)

    def test_retry_signal(self):
        signals = ci_flake_detector.compute_flake_signals(RETRY_FLAKE_RECORDS)
        self.assertGreater(signals.retry_signal, 0.0)

    def test_no_retry_signal(self):
        signals = ci_flake_detector.compute_flake_signals(FLAKY_JOB_RECORDS)
        self.assertEqual(signals.retry_signal, 0.0)

    def test_temporal_spread_many_days(self):
        signals = ci_flake_detector.compute_flake_signals(FLAKY_JOB_RECORDS)
        self.assertGreaterEqual(signals.temporal_spread, 0.7)

    def test_temporal_spread_single_day(self):
        signals = ci_flake_detector.compute_flake_signals(BUG_JOB_RECORDS)
        self.assertEqual(signals.temporal_spread, 0.0)

    def test_empty_records(self):
        signals = ci_flake_detector.compute_flake_signals([])
        self.assertEqual(signals.commit_diversity, 0.0)
        self.assertEqual(signals.failure_rate, 0.0)

    def test_mixed_categories(self):
        signals = ci_flake_detector.compute_flake_signals(MIXED_RECORDS)
        # Should have moderate correlation (mix of flake and bug categories).
        self.assertGreater(signals.category_flake_correlation, 0.0)


class TestComputeFlakeScore(unittest.TestCase):
    """Tests for compute_flake_score."""

    def test_high_score_for_flaky_signals(self):
        signals = ci_flake_detector.FlakeSignals(
            commit_diversity=1.0,
            failure_rate=0.8,
            category_flake_correlation=0.9,
            retry_signal=0.5,
            temporal_spread=1.0,
        )
        score = ci_flake_detector.compute_flake_score(signals)
        self.assertGreater(score, 0.7)

    def test_low_score_for_bug_signals(self):
        signals = ci_flake_detector.FlakeSignals(
            commit_diversity=0.0,
            failure_rate=0.1,
            category_flake_correlation=0.0,
            retry_signal=0.0,
            temporal_spread=0.0,
        )
        score = ci_flake_detector.compute_flake_score(signals)
        self.assertLess(score, 0.1)

    def test_score_bounded_0_1(self):
        signals = ci_flake_detector.FlakeSignals(
            commit_diversity=2.0,
            failure_rate=2.0,
            category_flake_correlation=2.0,
            retry_signal=2.0,
            temporal_spread=2.0,
        )
        score = ci_flake_detector.compute_flake_score(signals)
        self.assertLessEqual(score, 1.0)
        self.assertGreaterEqual(score, 0.0)

    def test_zero_signals_zero_score(self):
        signals = ci_flake_detector.FlakeSignals()
        score = ci_flake_detector.compute_flake_score(signals)
        self.assertEqual(score, 0.0)


class TestAnalyzeFlakes(unittest.TestCase):
    """Tests for analyze_flakes."""

    def test_identifies_flaky_job(self):
        reports = ci_flake_detector.analyze_flakes(FLAKY_JOB_RECORDS)
        self.assertEqual(len(reports), 1)
        self.assertTrue(reports[0]["is_flaky"])
        self.assertGreater(reports[0]["flake_score"], 0.45)

    def test_identifies_non_flaky_bug(self):
        reports = ci_flake_detector.analyze_flakes(BUG_JOB_RECORDS)
        self.assertEqual(len(reports), 1)
        self.assertFalse(reports[0]["is_flaky"])

    def test_groups_by_job_name(self):
        all_records = FLAKY_JOB_RECORDS + BUG_JOB_RECORDS
        reports = ci_flake_detector.analyze_flakes(all_records)
        self.assertEqual(len(reports), 2)
        job_names = {r["job_name"] for r in reports}
        self.assertIn("sys local root fedora / lima", job_names)
        self.assertIn("unit-test-fedora", job_names)

    def test_sorted_by_flake_score(self):
        all_records = FLAKY_JOB_RECORDS + BUG_JOB_RECORDS + RETRY_FLAKE_RECORDS
        reports = ci_flake_detector.analyze_flakes(all_records)
        scores = [r["flake_score"] for r in reports]
        self.assertEqual(scores, sorted(scores, reverse=True))

    def test_includes_sample_failures(self):
        reports = ci_flake_detector.analyze_flakes(FLAKY_JOB_RECORDS)
        self.assertGreater(len(reports[0]["sample_failures"]), 0)
        self.assertLessEqual(len(reports[0]["sample_failures"]), 3)

    def test_includes_categories(self):
        reports = ci_flake_detector.analyze_flakes(FLAKY_JOB_RECORDS)
        self.assertIn("network_error", reports[0]["categories"])

    def test_tracks_unique_commits(self):
        reports = ci_flake_detector.analyze_flakes(FLAKY_JOB_RECORDS)
        self.assertEqual(reports[0]["unique_commits"], 8)

    def test_tracks_unique_dates(self):
        reports = ci_flake_detector.analyze_flakes(FLAKY_JOB_RECORDS)
        self.assertEqual(reports[0]["unique_dates"], 8)

    def test_retry_records_detected_as_flaky(self):
        reports = ci_flake_detector.analyze_flakes(RETRY_FLAKE_RECORDS)
        self.assertEqual(len(reports), 1)
        self.assertTrue(reports[0]["is_flaky"])

    def test_empty_records(self):
        reports = ci_flake_detector.analyze_flakes([])
        self.assertEqual(len(reports), 0)

    def test_custom_threshold(self):
        # With a very high threshold, nothing should be flaky.
        reports = ci_flake_detector.analyze_flakes(
            FLAKY_JOB_RECORDS, threshold=0.99
        )
        self.assertFalse(reports[0]["is_flaky"])

    def test_report_has_signals(self):
        reports = ci_flake_detector.analyze_flakes(FLAKY_JOB_RECORDS)
        signals = reports[0]["signals"]
        self.assertIn("commit_diversity", signals)
        self.assertIn("category_flake_correlation", signals)
        self.assertIn("temporal_spread", signals)


class TestFormatFlakeReport(unittest.TestCase):
    """Tests for format_flake_report."""

    def test_format_with_flaky_and_non_flaky(self):
        all_records = FLAKY_JOB_RECORDS + BUG_JOB_RECORDS
        reports = ci_flake_detector.analyze_flakes(all_records)
        output = ci_flake_detector.format_flake_report(reports)
        self.assertIn("Flake Analysis Report", output)
        self.assertIn("FLAKY JOBS", output)
        self.assertIn("NON-FLAKY JOBS", output)
        self.assertIn("sys local root fedora / lima", output)

    def test_format_no_flaky_jobs(self):
        reports = ci_flake_detector.analyze_flakes(BUG_JOB_RECORDS)
        output = ci_flake_detector.format_flake_report(reports)
        self.assertIn("Flaky: 0", output)
        # The "FLAKY JOBS (likely intermittent" header should not appear.
        self.assertNotIn("FLAKY JOBS (likely intermittent", output)

    def test_format_empty(self):
        output = ci_flake_detector.format_flake_report([])
        self.assertIn("0", output)


class TestFlakeCorrelatedCategories(unittest.TestCase):
    """Ensure category sets are consistent."""

    def test_no_overlap_between_flake_and_bug(self):
        overlap = (
            ci_flake_detector.FLAKE_CORRELATED_CATEGORIES
            & ci_flake_detector.BUG_CORRELATED_CATEGORIES
        )
        self.assertEqual(
            len(overlap),
            0,
            f"Categories overlap: {overlap}",
        )

    def test_categories_exist_in_classifier(self):
        import ci_failure_classifier

        all_categories = set(ci_failure_classifier.CATEGORIES.keys())
        for cat in ci_flake_detector.FLAKE_CORRELATED_CATEGORIES:
            self.assertIn(
                cat, all_categories,
                f"Flake category '{cat}' not defined in classifier",
            )
        for cat in ci_flake_detector.BUG_CORRELATED_CATEGORIES:
            self.assertIn(
                cat, all_categories,
                f"Bug category '{cat}' not defined in classifier",
            )


if __name__ == "__main__":
    unittest.main()
