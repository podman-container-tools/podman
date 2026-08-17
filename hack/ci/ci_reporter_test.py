#!/usr/bin/env python3
"""
Unit tests for ci_reporter.py

Run with:
    python3 -m pytest hack/ci/ci_reporter_test.py -v
    python3 hack/ci/ci_reporter_test.py
"""

import json
import os
import sys
import unittest
from io import StringIO
from unittest.mock import MagicMock, patch

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))

import ci_reporter


# --- Test Fixtures ---

SAMPLE_RECORD = {
    "run_id": 31813098642,
    "run_number": 1503,
    "workflow": "ci",
    "job_name": "sys local root fedora-current / lima",
    "job_id": 90002,
    "step_name": "Run test on lima",
    "branch": "main",
    "commit_sha": "3a18c5e98f5ccdc5384ac998e6d9cb00648d3ff8",
    "commit_message": "Merge pull request #29279",
    "event": "push",
    "created_at": "2026-08-14T15:09:38Z",
    "html_url": "https://example.com/job/90002",
    "failure_sections": [
        {"type": "ginkgo_failure", "text": "[FAIL] Podman run with volume"},
    ],
    "classification": {
        "category": "test_assertion_failure",
        "confidence": 0.85,
        "rule_name": "ginkgo_assertion",
        "matched_text": "[FAIL] Podman run with volume",
        "evidence": "[FAIL] Podman run with volume",
    },
    "agentic_analysis": {
        "status": "success",
        "refined_category": "test_assertion_failure",
        "root_cause": "Volume mount path mismatch in expected test fixture.",
        "is_flake_assessment": "likely_bug",
        "confidence": 0.9,
        "suggested_action": "Fix hardcoded path in test.",
    },
}

SAMPLE_FLAKE_REPORT = {
    "job_name": "sys local root fedora-current / lima",
    "flake_score": 0.82,
    "is_flaky": True,
    "total_failures": 8,
    "unique_commits": 6,
    "unique_dates": 5,
    "categories": {
        "network_error": 5,
        "test_assertion_failure": 3,
    },
    "signals": {
        "commit_diversity": 1.0,
        "category_flake_correlation": 0.8,
        "temporal_spread": 0.7,
        "failure_rate": 0.6,
        "retry_signal": 0.5,
    },
    "sample_failures": [
        {
            "run_id": 31813098642,
            "run_number": 1503,
            "commit_sha": "3a18c5e98f5c",
            "created_at": "2026-08-14T15:09:38Z",
            "category": "network_error",
            "evidence": "dial tcp 10.0.2.15: connection refused",
        },
    ],
}


class TestComputeFingerprint(unittest.TestCase):
    """Tests for fingerprint computation."""

    def test_deterministic(self):
        fp1 = ci_reporter.compute_fingerprint("sys local root fedora", "network_error")
        fp2 = ci_reporter.compute_fingerprint("sys local root fedora", "network_error")
        self.assertEqual(fp1, fp2)
        self.assertEqual(len(fp1), 16)

    def test_case_and_whitespace_insensitive(self):
        fp1 = ci_reporter.compute_fingerprint("  sys local root fedora  ", "network_error")
        fp2 = ci_reporter.compute_fingerprint("SYS LOCAL ROOT FEDORA", "NETWORK_ERROR")
        self.assertEqual(fp1, fp2)

    def test_distinct_for_different_jobs(self):
        fp1 = ci_reporter.compute_fingerprint("job-a", "network_error")
        fp2 = ci_reporter.compute_fingerprint("job-b", "network_error")
        self.assertNotEqual(fp1, fp2)


class TestFormatStepSummary(unittest.TestCase):
    """Tests for format_step_summary."""

    def test_empty_input(self):
        summary = ci_reporter.format_step_summary([], [])
        self.assertIn("No CI failures or flakes detected", summary)

    def test_summary_with_records(self):
        summary = ci_reporter.format_step_summary([SAMPLE_RECORD], [])
        self.assertIn("CI Failure & Flake Analysis Dashboard", summary)
        self.assertIn("Total Failed Jobs Analyzed", summary)
        self.assertIn("`test_assertion_failure`", summary)
        self.assertIn("Volume mount path mismatch", summary)
        self.assertIn("<details><summary>", summary)

    def test_summary_with_flake_reports(self):
        summary = ci_reporter.format_step_summary([SAMPLE_RECORD], [SAMPLE_FLAKE_REPORT])
        self.assertIn("Flaky Test Warnings", summary)
        self.assertIn("82%", summary)
        self.assertIn("sys local root fedora-current / lima", summary)
        self.assertIn("Suspected Flaky Jobs", summary)


class TestGenerateFlakeIssueBody(unittest.TestCase):
    """Tests for generate_flake_issue_body."""

    def test_generates_fingerprint_and_title(self):
        title, body = ci_reporter.generate_flake_issue_body(SAMPLE_FLAKE_REPORT)
        self.assertIn("CI Flake: `sys local root fedora-current / lima`", title)
        self.assertIn(ci_reporter.FINGERPRINT_PREFIX, body)
        self.assertIn("Flake Score:** `82%`", body)
        self.assertIn("network_error", body)
        self.assertIn("dial tcp 10.0.2.15", body)
        self.assertIn("Recommended Actions for Maintainers", body)


class TestGeneratePrComment(unittest.TestCase):
    """Tests for generate_pr_comment."""

    def test_empty_records(self):
        comment = ci_reporter.generate_pr_comment([])
        self.assertIn(ci_reporter.PR_COMMENT_MARKER, comment)
        self.assertIn("All CI jobs passed", comment)

    def test_with_records_and_flake_match(self):
        comment = ci_reporter.generate_pr_comment([SAMPLE_RECORD], [SAMPLE_FLAKE_REPORT])
        self.assertIn(ci_reporter.PR_COMMENT_MARKER, comment)
        self.assertIn("Matches known historical flake pattern", comment)
        self.assertIn("Volume mount path mismatch", comment)
        self.assertIn("test_assertion_failure", comment)


class TestFindExistingFlakeIssue(unittest.TestCase):
    """Tests for finding existing flake issue by fingerprint."""

    @patch("ci_run_collector.api_request")
    def test_finds_matching_issue(self, mock_api):
        fp = ci_reporter.compute_fingerprint("test-job", "network_error")
        mock_api.return_value = [
            {"number": 101, "title": "Old Flake", "body": "some issue"},
            {"number": 102, "title": "Target Flake", "body": f"text {ci_reporter.FINGERPRINT_PREFIX} {fp} --> end"},
        ]
        res = ci_reporter.find_existing_flake_issue("podman/podman", fp, token="test-token")
        self.assertIsNotNone(res)
        self.assertEqual(res["number"], 102)

    @patch("ci_run_collector.api_request")
    def test_returns_none_when_not_found(self, mock_api):
        mock_api.return_value = [{"number": 101, "body": "unrelated"}]
        res = ci_reporter.find_existing_flake_issue("podman/podman", "missing_fp", token="test-token")
        self.assertIsNone(res)


class TestCreateOrUpdateFlakeIssue(unittest.TestCase):
    """Tests for create_or_update_flake_issue."""

    def test_dry_run_mode(self):
        action, res = ci_reporter.create_or_update_flake_issue(
            "podman/podman", SAMPLE_FLAKE_REPORT, token="test-token", dry_run=True
        )
        self.assertEqual(action, "dry_run")
        self.assertIn("title", res)
        self.assertIn("fingerprint", res)

    @patch("ci_reporter.find_existing_flake_issue")
    @patch("ci_run_collector.urllib.request.urlopen")
    def test_update_existing_issue(self, mock_urlopen, mock_find):
        mock_find.return_value = {"number": 555}
        mock_resp = MagicMock()
        mock_resp.read.return_value = json.dumps({"id": 12345}).encode("utf-8")
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = MagicMock(return_value=False)
        mock_urlopen.return_value = mock_resp

        action, res = ci_reporter.create_or_update_flake_issue(
            "podman/podman", SAMPLE_FLAKE_REPORT, token="test-token", dry_run=False
        )
        self.assertEqual(action, "updated")
        self.assertEqual(res["id"], 12345)

    @patch("ci_reporter.find_existing_flake_issue")
    @patch("ci_run_collector.urllib.request.urlopen")
    def test_create_new_issue(self, mock_urlopen, mock_find):
        mock_find.return_value = None
        mock_resp = MagicMock()
        mock_resp.read.return_value = json.dumps({"number": 600, "html_url": "https://github.com/issue/600"}).encode("utf-8")
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = MagicMock(return_value=False)
        mock_urlopen.return_value = mock_resp

        action, res = ci_reporter.create_or_update_flake_issue(
            "podman/podman", SAMPLE_FLAKE_REPORT, token="test-token", dry_run=False
        )
        self.assertEqual(action, "created")
        self.assertEqual(res["number"], 600)


class TestPostOrUpdatePrComment(unittest.TestCase):
    """Tests for post_or_update_pr_comment."""

    def test_dry_run_mode(self):
        action, res = ci_reporter.post_or_update_pr_comment(
            "podman/podman", 123, "test comment", token="test-token", dry_run=True
        )
        self.assertEqual(action, "dry_run")
        self.assertEqual(res["pr_number"], 123)

    @patch("ci_run_collector.api_request")
    @patch("ci_run_collector.urllib.request.urlopen")
    def test_updates_existing_comment(self, mock_urlopen, mock_api):
        mock_api.return_value = [
            {"id": 999, "body": f"{ci_reporter.PR_COMMENT_MARKER}\nOld summary"}
        ]
        mock_resp = MagicMock()
        mock_resp.read.return_value = json.dumps({"id": 999, "body": "updated"}).encode("utf-8")
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = MagicMock(return_value=False)
        mock_urlopen.return_value = mock_resp

        action, res = ci_reporter.post_or_update_pr_comment(
            "podman/podman", 123, "new body", token="test-token", dry_run=False
        )
        self.assertEqual(action, "updated")
        self.assertEqual(res["id"], 999)

    @patch("ci_run_collector.api_request")
    @patch("ci_run_collector.urllib.request.urlopen")
    def test_creates_new_comment(self, mock_urlopen, mock_api):
        mock_api.return_value = [{"id": 888, "body": "unrelated bot comment"}]
        mock_resp = MagicMock()
        mock_resp.read.return_value = json.dumps({"id": 1001, "body": "created"}).encode("utf-8")
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = MagicMock(return_value=False)
        mock_urlopen.return_value = mock_resp

        action, res = ci_reporter.post_or_update_pr_comment(
            "podman/podman", 123, "new body", token="test-token", dry_run=False
        )
        self.assertEqual(action, "created")
        self.assertEqual(res["id"], 1001)


class TestMainCLI(unittest.TestCase):
    """Tests for main CLI execution."""

    @patch("sys.stdin", StringIO(json.dumps([SAMPLE_RECORD])))
    @patch("sys.stdout", new_callable=StringIO)
    def test_cli_step_summary(self, mock_stdout):
        with patch.object(sys, "argv", ["ci_reporter.py", "--stdin", "--format", "step-summary"]):
            ci_reporter.main()
        output = mock_stdout.getvalue()
        self.assertIn("CI Failure & Flake Analysis Dashboard", output)

    @patch("sys.stdin", StringIO(json.dumps([SAMPLE_FLAKE_REPORT])))
    @patch("sys.stdout", new_callable=StringIO)
    def test_cli_issue_body(self, mock_stdout):
        with patch.object(sys, "argv", ["ci_reporter.py", "--stdin", "--format", "issue-body"]):
            ci_reporter.main()
        output = mock_stdout.getvalue()
        self.assertIn("CI Flake Report", output)

    @patch("sys.stdin", StringIO(json.dumps([SAMPLE_RECORD])))
    @patch("sys.stdout", new_callable=StringIO)
    def test_cli_pr_comment(self, mock_stdout):
        with patch.object(sys, "argv", ["ci_reporter.py", "--stdin", "--format", "pr-comment"]):
            ci_reporter.main()
        output = mock_stdout.getvalue()
        self.assertIn("Podman CI Failure Triage", output)


if __name__ == "__main__":
    unittest.main()
