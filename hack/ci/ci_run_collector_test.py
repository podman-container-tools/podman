#!/usr/bin/env python3
"""
Unit tests for ci_run_collector.py

Run with:
    python3 -m pytest hack/ci/ci_run_collector_test.py -v
    python3 hack/ci/ci_run_collector_test.py
"""

import json
import os
import sys
import unittest
from io import BytesIO
from unittest.mock import MagicMock, patch
from urllib.error import HTTPError, URLError

# Ensure hack/ci is importable.
sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))

import ci_run_collector


# --- Test fixtures ---

SAMPLE_RUN = {
    "id": 31813098642,
    "name": "ci",
    "run_number": 1503,
    "status": "completed",
    "conclusion": "failure",
    "head_branch": "main",
    "head_sha": "3a18c5e98f5ccdc5384ac998e6d9cb00648d3ff8",
    "event": "push",
    "html_url": "https://github.com/podman-container-tools/podman/actions/runs/31813098642",
    "created_at": "2026-08-14T15:09:38Z",
    "updated_at": "2026-08-14T16:12:06Z",
    "run_attempt": 1,
    "head_commit": {
        "id": "3a18c5e98f5ccdc5384ac998e6d9cb00648d3ff8",
        "message": "Merge pull request #29279 from vtushar06/fix-prune-build-race\n\nlibpod: do not fail prune when build container is already gone",
        "timestamp": "2026-08-14T15:09:35Z",
        "author": {"name": "Matt Heon", "email": "mheon@redhat.com"},
        "committer": {"name": "GitHub", "email": "noreply@github.com"},
    },
}

SAMPLE_JOB_PASSED = {
    "id": 90001,
    "name": "validate-source",
    "conclusion": "success",
    "status": "completed",
    "html_url": "https://github.com/podman-container-tools/podman/actions/runs/31813098642/job/90001",
    "started_at": "2026-08-14T15:10:00Z",
    "completed_at": "2026-08-14T15:15:00Z",
    "runner_name": "cncf-ubuntu-8-32-x86-1",
    "steps": [
        {
            "name": "Set up job",
            "status": "completed",
            "conclusion": "success",
            "number": 1,
            "started_at": "2026-08-14T15:10:00Z",
            "completed_at": "2026-08-14T15:10:05Z",
        },
    ],
}

SAMPLE_JOB_FAILED = {
    "id": 90002,
    "name": "sys local root fedora-current / lima",
    "conclusion": "failure",
    "status": "completed",
    "html_url": "https://github.com/podman-container-tools/podman/actions/runs/31813098642/job/90002",
    "started_at": "2026-08-14T15:10:00Z",
    "completed_at": "2026-08-14T15:45:00Z",
    "runner_name": "cncf-ubuntu-8-32-x86-3",
    "steps": [
        {
            "name": "Set up job",
            "status": "completed",
            "conclusion": "success",
            "number": 1,
            "started_at": "2026-08-14T15:10:00Z",
            "completed_at": "2026-08-14T15:10:05Z",
        },
        {
            "name": "Run test on lima",
            "status": "completed",
            "conclusion": "failure",
            "number": 4,
            "started_at": "2026-08-14T15:12:00Z",
            "completed_at": "2026-08-14T15:45:00Z",
        },
        {
            "name": "Output failure log as GITHUB_STEP_SUMMARY",
            "status": "completed",
            "conclusion": "success",
            "number": 5,
            "started_at": "2026-08-14T15:45:01Z",
            "completed_at": "2026-08-14T15:45:02Z",
        },
    ],
}


def _mock_api_response(data: dict) -> MagicMock:
    """Create a mock urllib response with JSON data."""
    body = json.dumps(data).encode("utf-8")
    mock_resp = MagicMock()
    mock_resp.read.return_value = body
    mock_resp.__enter__ = lambda s: s
    mock_resp.__exit__ = MagicMock(return_value=False)
    return mock_resp


class TestBuildHeaders(unittest.TestCase):
    """Tests for _build_headers."""

    def test_without_token(self):
        headers = ci_run_collector._build_headers()
        self.assertIn("Accept", headers)
        self.assertNotIn("Authorization", headers)

    def test_with_token(self):
        headers = ci_run_collector._build_headers("ghp_test123")
        self.assertEqual(headers["Authorization"], "Bearer ghp_test123")
        self.assertEqual(headers["Accept"], "application/vnd.github+json")


class TestApiRequest(unittest.TestCase):
    """Tests for api_request."""

    @patch("ci_run_collector.urllib.request.urlopen")
    def test_successful_request(self, mock_urlopen):
        expected = {"total_count": 1, "workflow_runs": []}
        mock_urlopen.return_value = _mock_api_response(expected)

        result = ci_run_collector.api_request("https://api.github.com/test")
        self.assertEqual(result, expected)

    @patch("ci_run_collector.urllib.request.urlopen")
    def test_404_raises_immediately(self, mock_urlopen):
        mock_urlopen.side_effect = HTTPError(
            "https://api.github.com/test", 404, "Not Found", {}, BytesIO(b"")
        )
        with self.assertRaises(ci_run_collector.GitHubAPIError) as ctx:
            ci_run_collector.api_request("https://api.github.com/test")
        self.assertIn("not found", str(ctx.exception).lower())

    @patch("ci_run_collector.urllib.request.urlopen")
    def test_rate_limit_raises_rate_limit_error(self, mock_urlopen):
        headers = MagicMock()
        headers.get = lambda key, default=None: {
            "X-RateLimit-Remaining": "0",
            "X-RateLimit-Reset": str(int(ci_run_collector.time.time()) + 60),
        }.get(key, default)
        err = HTTPError(
            "https://api.github.com/test", 403, "Forbidden", headers, BytesIO(b"")
        )
        mock_urlopen.side_effect = err
        with self.assertRaises(ci_run_collector.RateLimitError):
            ci_run_collector.api_request("https://api.github.com/test")

    @patch("ci_run_collector.time.sleep")
    @patch("ci_run_collector.urllib.request.urlopen")
    def test_retry_on_server_error(self, mock_urlopen, mock_sleep):
        # First two calls fail with 502, third succeeds.
        expected = {"ok": True}
        mock_urlopen.side_effect = [
            HTTPError(
                "https://api.github.com/test", 502, "Bad Gateway", {}, BytesIO(b"")
            ),
            HTTPError(
                "https://api.github.com/test", 502, "Bad Gateway", {}, BytesIO(b"")
            ),
            _mock_api_response(expected),
        ]
        result = ci_run_collector.api_request("https://api.github.com/test")
        self.assertEqual(result, expected)
        self.assertEqual(mock_sleep.call_count, 2)

    @patch("ci_run_collector.time.sleep")
    @patch("ci_run_collector.urllib.request.urlopen")
    def test_retry_exhaustion_raises(self, mock_urlopen, mock_sleep):
        mock_urlopen.side_effect = URLError("Connection refused")
        with self.assertRaises(ci_run_collector.GitHubAPIError):
            ci_run_collector.api_request("https://api.github.com/test")


class TestParseRun(unittest.TestCase):
    """Tests for parse_run."""

    def test_parse_complete_run(self):
        result = ci_run_collector.parse_run(SAMPLE_RUN)
        self.assertEqual(result.run_id, 31813098642)
        self.assertEqual(result.workflow, "ci")
        self.assertEqual(result.run_number, 1503)
        self.assertEqual(result.conclusion, "failure")
        self.assertEqual(result.branch, "main")
        self.assertEqual(result.commit_sha, "3a18c5e98f5ccdc5384ac998e6d9cb00648d3ff8")
        self.assertIn("fix-prune-build-race", result.commit_message)
        self.assertEqual(result.event, "push")
        self.assertEqual(result.run_attempt, 1)

    def test_parse_run_missing_commit(self):
        """Handles missing head_commit gracefully."""
        run_data = {**SAMPLE_RUN, "head_commit": None}
        result = ci_run_collector.parse_run(run_data)
        self.assertEqual(result.commit_message, "")

    def test_parse_run_minimal(self):
        """Handles minimal data without crashing."""
        minimal = {"id": 1, "head_commit": {}}
        result = ci_run_collector.parse_run(minimal)
        self.assertEqual(result.run_id, 1)
        self.assertEqual(result.workflow, "")


class TestParseJob(unittest.TestCase):
    """Tests for parse_job."""

    def test_parse_passed_job(self):
        result = ci_run_collector.parse_job(SAMPLE_JOB_PASSED)
        self.assertEqual(result.job_id, 90001)
        self.assertEqual(result.name, "validate-source")
        self.assertEqual(result.conclusion, "success")
        self.assertEqual(len(result.failed_steps), 0)

    def test_parse_failed_job(self):
        result = ci_run_collector.parse_job(SAMPLE_JOB_FAILED)
        self.assertEqual(result.job_id, 90002)
        self.assertEqual(result.name, "sys local root fedora-current / lima")
        self.assertEqual(result.conclusion, "failure")
        self.assertEqual(len(result.failed_steps), 1)
        self.assertEqual(result.failed_steps[0]["name"], "Run test on lima")
        self.assertEqual(result.failed_steps[0]["number"], 4)

    def test_parse_job_no_steps(self):
        """Handles job with no steps (e.g., cancelled)."""
        job = {**SAMPLE_JOB_FAILED, "steps": []}
        result = ci_run_collector.parse_job(job)
        self.assertEqual(len(result.failed_steps), 0)


class TestFetchWorkflowRuns(unittest.TestCase):
    """Tests for fetch_workflow_runs."""

    @patch("ci_run_collector.api_request")
    def test_basic_fetch(self, mock_api):
        mock_api.return_value = {
            "total_count": 1,
            "workflow_runs": [SAMPLE_RUN],
        }
        result = ci_run_collector.fetch_workflow_runs(
            "podman-container-tools/podman", branch="main", limit=5
        )
        self.assertEqual(len(result), 1)
        self.assertEqual(result[0]["id"], 31813098642)

    @patch("ci_run_collector.api_request")
    def test_limit_respected(self, mock_api):
        """Results are capped at the requested limit."""
        runs = [
            {**SAMPLE_RUN, "id": i} for i in range(10)
        ]
        mock_api.return_value = {
            "total_count": 10,
            "workflow_runs": runs,
        }
        result = ci_run_collector.fetch_workflow_runs(
            "podman-container-tools/podman", limit=3
        )
        self.assertEqual(len(result), 3)

    @patch("ci_run_collector.api_request")
    def test_pagination(self, mock_api):
        """Multiple pages are fetched when needed."""
        page1 = [
            {**SAMPLE_RUN, "id": i} for i in range(100)
        ]
        page2 = [
            {**SAMPLE_RUN, "id": i} for i in range(100, 150)
        ]
        mock_api.side_effect = [
            {"total_count": 150, "workflow_runs": page1},
            {"total_count": 150, "workflow_runs": page2},
        ]
        result = ci_run_collector.fetch_workflow_runs(
            "podman-container-tools/podman", limit=150
        )
        self.assertEqual(len(result), 150)
        self.assertEqual(mock_api.call_count, 2)

    @patch("ci_run_collector.api_request")
    def test_url_includes_filters(self, mock_api):
        mock_api.return_value = {"total_count": 0, "workflow_runs": []}
        ci_run_collector.fetch_workflow_runs(
            "podman-container-tools/podman",
            branch="main",
            status="failure",
            workflow="ci.yml",
            limit=5,
        )
        call_url = mock_api.call_args[0][0]
        self.assertIn("branch=main", call_url)
        self.assertIn("status=failure", call_url)
        self.assertIn("ci.yml", call_url)


class TestCollectFailedRuns(unittest.TestCase):
    """Tests for collect_failed_runs (integration of fetch + parse)."""

    @patch("ci_run_collector.fetch_jobs_for_run")
    @patch("ci_run_collector.fetch_workflow_runs")
    def test_collect_with_jobs(self, mock_fetch_runs, mock_fetch_jobs):
        mock_fetch_runs.return_value = [SAMPLE_RUN]
        mock_fetch_jobs.return_value = [SAMPLE_JOB_PASSED, SAMPLE_JOB_FAILED]

        results = ci_run_collector.collect_failed_runs(limit=1)
        self.assertEqual(len(results), 1)

        run = results[0]
        self.assertEqual(run["run_id"], 31813098642)
        self.assertEqual(len(run["jobs"]), 2)

        failed_jobs = [j for j in run["jobs"] if j["conclusion"] == "failure"]
        self.assertEqual(len(failed_jobs), 1)
        self.assertEqual(failed_jobs[0]["name"], "sys local root fedora-current / lima")

    @patch("ci_run_collector.fetch_workflow_runs")
    def test_collect_without_jobs(self, mock_fetch_runs):
        mock_fetch_runs.return_value = [SAMPLE_RUN]

        results = ci_run_collector.collect_failed_runs(
            limit=1, include_jobs=False
        )
        self.assertEqual(len(results), 1)
        self.assertEqual(results[0]["jobs"], [])


class TestFormatSummary(unittest.TestCase):
    """Tests for format_summary."""

    def test_format_with_failures(self):
        from dataclasses import asdict

        run = ci_run_collector.parse_run(SAMPLE_RUN)
        job = ci_run_collector.parse_job(SAMPLE_JOB_FAILED)
        run.jobs = [asdict(job)]
        run_dict = asdict(run)

        output = ci_run_collector.format_summary([run_dict])
        self.assertIn("Run #1503", output)
        self.assertIn("failure", output)
        self.assertIn("sys local root fedora-current", output)
        self.assertIn("Run test on lima", output)

    def test_format_empty(self):
        output = ci_run_collector.format_summary([])
        self.assertEqual(output, "")


if __name__ == "__main__":
    unittest.main()
