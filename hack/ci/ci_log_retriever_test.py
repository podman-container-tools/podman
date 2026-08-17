#!/usr/bin/env python3
"""
Unit tests for ci_log_retriever.py

Run with:
    python3 -m pytest hack/ci/ci_log_retriever_test.py -v
    python3 hack/ci/ci_log_retriever_test.py
"""

import io
import json
import os
import sys
import unittest
import zipfile
from unittest.mock import MagicMock, patch

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))

import ci_log_retriever


# --- Test fixtures: sample log content ---

SAMPLE_RAW_LOG_GINKGO = """\
2026-08-14T15:12:34.1234567Z ##[group]Run test on lima
2026-08-14T15:12:34.2345678Z ./hack/ci/ci.sh sys local root fedora-current
2026-08-14T15:12:34.3456789Z ##[endgroup]
2026-08-14T15:12:35.0000000Z \x1b[1m\x1b[32mRunner executing test as root user\x1b[0m
2026-08-14T15:12:36.0000000Z [+12s] Running integration test suite
2026-08-14T15:12:40.0000000Z [+16s] Podman run with --rm
2026-08-14T15:12:40.1000000Z [+16s]     Expected container to be removed
2026-08-14T15:12:41.0000000Z [+17s] -----
2026-08-14T15:12:42.0000000Z [+18s] [FAIL] Podman run with volume
2026-08-14T15:12:43.0000000Z [+19s] Expected
2026-08-14T15:12:44.0000000Z [+20s]     <string>: /tmp/testdir
2026-08-14T15:12:45.0000000Z [+21s] to equal
2026-08-14T15:12:46.0000000Z [+22s]     <string>: /tmp/expected
2026-08-14T15:12:47.0000000Z [+23s] -----
2026-08-14T15:12:50.0000000Z [+26s] FAIL! -- 45 Passed | 1 Failed | 0 Pending | 3 Skipped
"""

SAMPLE_RAW_LOG_BATS = """\
2026-08-14T16:00:00.0000000Z [+1s] 1..50
2026-08-14T16:00:01.0000000Z [+2s] ok 1 podman ps
2026-08-14T16:00:02.0000000Z [+3s] ok 2 podman images
2026-08-14T16:00:03.0000000Z [+4s] not ok 3 podman run with networking
2026-08-14T16:00:04.0000000Z [+5s] # (in test file test/system/500-networking.bats, line 42)
2026-08-14T16:00:05.0000000Z [+6s] #   `run_podman run --rm --net=host alpine wget -qO- http://localhost:8080' failed
2026-08-14T16:00:06.0000000Z [+7s] #   expected: 0
2026-08-14T16:00:07.0000000Z [+8s] #   actual:   7
2026-08-14T16:00:08.0000000Z [+9s] ok 4 podman stop
"""

SAMPLE_RAW_LOG_PANIC = """\
2026-08-14T17:00:00.0000000Z some normal output
2026-08-14T17:00:01.0000000Z panic: runtime error: invalid memory address or nil pointer dereference
2026-08-14T17:00:02.0000000Z [signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x1234]
2026-08-14T17:00:03.0000000Z
2026-08-14T17:00:04.0000000Z goroutine 1 [running]:
2026-08-14T17:00:05.0000000Z main.(*Runtime).Start(0x0, 0xc000123456)
2026-08-14T17:00:06.0000000Z     /go/src/podman/libpod/runtime.go:123 +0x45
"""

SAMPLE_LOG_WITH_SECRETS = """\
Authenticating with token ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklm
Pulling from https://user:p4ssw0rd@registry.example.com/image
Using key AKIAIOSFODNN7EXAMPLE to access bucket
Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abc123.xyz
"""

SAMPLE_RUN_DATA = {
    "run_id": 31813098642,
    "run_number": 1503,
    "workflow": "ci",
    "branch": "main",
    "commit_sha": "3a18c5e98f5ccdc5384ac998e6d9cb00648d3ff8",
    "commit_message": "Merge pull request #29279\n\nlibpod: fix prune race",
    "event": "push",
    "created_at": "2026-08-14T15:09:38Z",
    "jobs": [
        {
            "job_id": 90001,
            "name": "validate-source",
            "conclusion": "success",
            "html_url": "https://example.com/job/90001",
            "failed_steps": [],
        },
        {
            "job_id": 90002,
            "name": "sys local root fedora-current / lima",
            "conclusion": "failure",
            "html_url": "https://example.com/job/90002",
            "failed_steps": [
                {"name": "Run test on lima", "number": 4, "conclusion": "failure"},
            ],
        },
    ],
}


class TestStripAnsi(unittest.TestCase):
    """Tests for ANSI escape code stripping."""

    def test_removes_color_codes(self):
        text = "\x1b[1m\x1b[32mGreen bold\x1b[0m normal"
        self.assertEqual(ci_log_retriever.strip_ansi(text), "Green bold normal")

    def test_removes_osc_sequences(self):
        text = "before\x1b]0;title\x07after"
        self.assertEqual(ci_log_retriever.strip_ansi(text), "beforeafter")

    def test_preserves_plain_text(self):
        text = "No escapes here"
        self.assertEqual(ci_log_retriever.strip_ansi(text), text)

    def test_empty_string(self):
        self.assertEqual(ci_log_retriever.strip_ansi(""), "")


class TestStripGhaTimestamps(unittest.TestCase):
    """Tests for GitHub Actions timestamp removal."""

    def test_removes_timestamps(self):
        text = "2026-08-14T15:12:34.1234567Z Hello world"
        self.assertEqual(
            ci_log_retriever.strip_gha_timestamps(text), "Hello world"
        )

    def test_multiline(self):
        text = (
            "2026-08-14T15:12:34.1234567Z line 1\n"
            "2026-08-14T15:12:35.0000000Z line 2\n"
            "no timestamp line 3"
        )
        result = ci_log_retriever.strip_gha_timestamps(text)
        self.assertEqual(result, "line 1\nline 2\nno timestamp line 3")

    def test_preserves_non_timestamp_lines(self):
        text = "Just a normal line"
        self.assertEqual(ci_log_retriever.strip_gha_timestamps(text), text)


class TestStripGhaMarkup(unittest.TestCase):
    """Tests for GitHub Actions markup removal."""

    def test_removes_group_markers(self):
        text = "##[group]Test Setup\nactual output\n##[endgroup]"
        result = ci_log_retriever.strip_gha_markup(text)
        self.assertEqual(result, "actual output")

    def test_removes_error_notices(self):
        text = "##[error]Something failed\nDetails here"
        result = ci_log_retriever.strip_gha_markup(text)
        self.assertEqual(result, "Details here")

    def test_preserves_normal_hashes(self):
        text = "## This is markdown\n### Not GHA markup"
        self.assertEqual(ci_log_retriever.strip_gha_markup(text), text)


class TestStripLogformatterTimestamps(unittest.TestCase):
    """Tests for logformatter [+NNNs] timestamp removal."""

    def test_removes_timestamps(self):
        text = "[+12s] Running test\n[+100s] Done"
        result = ci_log_retriever.strip_logformatter_timestamps(text)
        self.assertEqual(result, "Running test\nDone")

    def test_preserves_non_timestamp_brackets(self):
        text = "[FAIL] test failed"
        result = ci_log_retriever.strip_logformatter_timestamps(text)
        self.assertEqual(result, text)


class TestRedactSecrets(unittest.TestCase):
    """Tests for secret redaction."""

    def test_redacts_github_token(self):
        text = "token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklm"
        result = ci_log_retriever.redact_secrets(text)
        self.assertNotIn("ghp_", result)
        self.assertIn("[REDACTED_GH_TOKEN]", result)

    def test_redacts_url_credentials(self):
        text = "pulling https://user:pass@registry.com/image"
        result = ci_log_retriever.redact_secrets(text)
        self.assertNotIn("user:pass", result)
        self.assertIn("[REDACTED]", result)

    def test_redacts_bearer_token(self):
        text = "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9abc"
        result = ci_log_retriever.redact_secrets(text)
        self.assertNotIn("eyJhbG", result)
        self.assertIn("[REDACTED_AUTH]", result)

    def test_redacts_aws_keys(self):
        text = "key: AKIAIOSFODNN7EXAMPLE"
        result = ci_log_retriever.redact_secrets(text)
        self.assertNotIn("AKIAIOSFODNN7EXAMPLE", result)
        self.assertIn("[REDACTED_AWS_KEY]", result)

    def test_preserves_git_shas(self):
        """40-char hex strings that look like git SHAs should be preserved."""
        sha = "3a18c5e98f5ccdc5384ac998e6d9cb00648d3ff8"
        text = f"commit {sha}"
        result = ci_log_retriever.redact_secrets(text)
        self.assertIn(sha, result)

    def test_preserves_normal_text(self):
        text = "No secrets here, just normal CI output."
        self.assertEqual(ci_log_retriever.redact_secrets(text), text)


class TestNormalizeLog(unittest.TestCase):
    """Tests for the full normalization pipeline."""

    def test_ginkgo_log_normalization(self):
        result = ci_log_retriever.normalize_log(SAMPLE_RAW_LOG_GINKGO)
        # ANSI codes should be gone.
        self.assertNotIn("\x1b[", result)
        # GHA timestamps should be gone.
        self.assertNotIn("2026-08-14T15:12", result)
        # GHA markup should be gone.
        self.assertNotIn("##[group]", result)
        # Logformatter timestamps should be gone.
        self.assertNotIn("[+12s]", result)
        # Actual content should remain.
        self.assertIn("[FAIL] Podman run with volume", result)
        self.assertIn("FAIL!", result)

    def test_bats_log_normalization(self):
        result = ci_log_retriever.normalize_log(SAMPLE_RAW_LOG_BATS)
        self.assertIn("not ok 3 podman run with networking", result)
        self.assertNotIn("2026-08-14T16:00", result)

    def test_collapses_excessive_blank_lines(self):
        text = "line1\n\n\n\n\n\n\nline2"
        result = ci_log_retriever.normalize_log(text)
        self.assertEqual(result.count("\n\n\n\n"), 0)
        self.assertIn("line1", result)
        self.assertIn("line2", result)


class TestExtractFailureSections(unittest.TestCase):
    """Tests for failure section extraction."""

    def test_extracts_ginkgo_failure(self):
        normalized = ci_log_retriever.normalize_log(SAMPLE_RAW_LOG_GINKGO)
        sections = ci_log_retriever.extract_failure_sections(normalized)
        self.assertTrue(len(sections) > 0)
        types = [t for t, _ in sections]
        self.assertIn("ginkgo_failure", types)

    def test_extracts_bats_failure(self):
        normalized = ci_log_retriever.normalize_log(SAMPLE_RAW_LOG_BATS)
        sections = ci_log_retriever.extract_failure_sections(normalized)
        self.assertTrue(len(sections) > 0)
        types = [t for t, _ in sections]
        self.assertIn("bats_failure", types)

    def test_extracts_panic(self):
        normalized = ci_log_retriever.normalize_log(SAMPLE_RAW_LOG_PANIC)
        sections = ci_log_retriever.extract_failure_sections(normalized)
        self.assertTrue(len(sections) > 0)
        types = [t for t, _ in sections]
        self.assertIn("panic", types)

    def test_extracts_ginkgo_summary(self):
        normalized = ci_log_retriever.normalize_log(SAMPLE_RAW_LOG_GINKGO)
        sections = ci_log_retriever.extract_failure_sections(normalized)
        types = [t for t, _ in sections]
        # Should find the FAIL! summary line.
        self.assertTrue(
            "ginkgo_summary" in types or "ginkgo_failure" in types,
            f"Expected ginkgo_summary or ginkgo_failure in {types}",
        )

    def test_no_false_positives_on_clean_log(self):
        clean_log = "All tests passed\nok 1 test\nok 2 test\nSUCCESS!"
        sections = ci_log_retriever.extract_failure_sections(clean_log)
        self.assertEqual(len(sections), 0)

    def test_deduplicates_overlapping_sections(self):
        """Adjacent failure markers should be merged, not duplicated."""
        log = "[FAIL] test1\nExpected foo\nto equal bar\n[FAIL] test2\nExpected baz"
        sections = ci_log_retriever.extract_failure_sections(log)
        # All failure content should be captured, possibly merged.
        all_text = "\n".join(text for _, text in sections)
        self.assertIn("test1", all_text)
        self.assertIn("test2", all_text)


class TestExtractZipLog(unittest.TestCase):
    """Tests for zip file extraction."""

    def test_extracts_from_valid_zip(self):
        buf = io.BytesIO()
        with zipfile.ZipFile(buf, "w") as zf:
            zf.writestr("step_1.txt", "Step 1 output")
            zf.writestr("step_2.txt", "Step 2 output")
        result = ci_log_retriever._extract_zip_log(buf.getvalue())
        self.assertIn("Step 1 output", result)
        self.assertIn("Step 2 output", result)

    def test_handles_invalid_zip(self):
        result = ci_log_retriever._extract_zip_log(b"not a zip file")
        self.assertIn("not a zip file", result)


class TestBuildFailureRecords(unittest.TestCase):
    """Tests for build_failure_records (no log download)."""

    def test_creates_record_for_failed_job(self):
        records = ci_log_retriever.build_failure_records(
            SAMPLE_RUN_DATA, download_logs=False
        )
        self.assertEqual(len(records), 1)

        rec = records[0]
        self.assertEqual(rec["run_id"], 31813098642)
        self.assertEqual(rec["job_name"], "sys local root fedora-current / lima")
        self.assertEqual(rec["step_name"], "Run test on lima")
        self.assertEqual(rec["branch"], "main")
        self.assertEqual(rec["commit_message"], "Merge pull request #29279")

    def test_skips_successful_jobs(self):
        run_data = {
            **SAMPLE_RUN_DATA,
            "jobs": [SAMPLE_RUN_DATA["jobs"][0]],  # Only the passing job.
        }
        records = ci_log_retriever.build_failure_records(
            run_data, download_logs=False
        )
        self.assertEqual(len(records), 0)

    def test_handles_no_failed_steps(self):
        """A failed job with no identified failed steps."""
        job = {
            "job_id": 99999,
            "name": "cancelled-job",
            "conclusion": "failure",
            "html_url": "https://example.com",
            "failed_steps": [],
        }
        run_data = {**SAMPLE_RUN_DATA, "jobs": [job]}
        records = ci_log_retriever.build_failure_records(
            run_data, download_logs=False
        )
        self.assertEqual(len(records), 1)
        self.assertEqual(records[0]["step_name"], "unknown")

    @patch("ci_log_retriever.download_job_log")
    def test_with_log_download(self, mock_download):
        mock_download.return_value = SAMPLE_RAW_LOG_GINKGO
        records = ci_log_retriever.build_failure_records(
            SAMPLE_RUN_DATA, download_logs=True
        )
        self.assertEqual(len(records), 1)
        rec = records[0]
        self.assertGreater(rec["raw_log_bytes"], 0)
        self.assertGreater(len(rec["normalized_log_excerpt"]), 0)
        self.assertGreater(len(rec["failure_sections"]), 0)
        # Verify failure sections have correct structure.
        for section in rec["failure_sections"]:
            self.assertIn("type", section)
            self.assertIn("text", section)

    @patch("ci_log_retriever.download_job_log")
    def test_handles_log_download_failure(self, mock_download):
        mock_download.return_value = None
        records = ci_log_retriever.build_failure_records(
            SAMPLE_RUN_DATA, download_logs=True
        )
        self.assertEqual(len(records), 1)
        rec = records[0]
        self.assertEqual(rec["raw_log_bytes"], 0)
        self.assertEqual(rec["normalized_log_excerpt"], "")
        self.assertEqual(rec["failure_sections"], [])


class TestEndToEnd(unittest.TestCase):
    """Integration tests for the full pipeline (without network)."""

    @patch("ci_log_retriever.download_job_log")
    def test_full_pipeline_ginkgo(self, mock_download):
        mock_download.return_value = SAMPLE_RAW_LOG_GINKGO
        records = ci_log_retriever.build_failure_records(SAMPLE_RUN_DATA)

        self.assertEqual(len(records), 1)
        rec = records[0]
        # Metadata is correct.
        self.assertEqual(rec["workflow"], "ci")
        self.assertEqual(rec["branch"], "main")
        # Log was normalized (no ANSI, no GHA timestamps).
        self.assertNotIn("\x1b[", rec["normalized_log_excerpt"])
        self.assertNotIn("2026-08-14T15:12", rec["normalized_log_excerpt"])
        # Failure sections were extracted.
        self.assertGreater(len(rec["failure_sections"]), 0)

    @patch("ci_log_retriever.download_job_log")
    def test_full_pipeline_with_secrets(self, mock_download):
        mock_download.return_value = SAMPLE_LOG_WITH_SECRETS
        records = ci_log_retriever.build_failure_records(SAMPLE_RUN_DATA)

        rec = records[0]
        excerpt = rec["normalized_log_excerpt"]
        # Secrets should be redacted.
        self.assertNotIn("ghp_", excerpt)
        self.assertNotIn("p4ssw0rd", excerpt)
        self.assertNotIn("AKIAIOSFODNN7EXAMPLE", excerpt)


if __name__ == "__main__":
    unittest.main()
