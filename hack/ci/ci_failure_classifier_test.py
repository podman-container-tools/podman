#!/usr/bin/env python3
"""
Unit tests for ci_failure_classifier.py

Run with:
    python3 -m pytest hack/ci/ci_failure_classifier_test.py -v
    python3 hack/ci/ci_failure_classifier_test.py
"""

import json
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))

import ci_failure_classifier


# --- Realistic log snippets for each failure category ---

LOG_GINKGO_ASSERTION = """\
-----
[FAIL] Podman run with volume
  Expected
      <string>: /tmp/testdir
  to equal
      <string>: /tmp/expected
-----
FAIL! -- 45 Passed | 1 Failed | 0 Pending | 3 Skipped
"""

LOG_BATS_ASSERTION = """\
1..50
ok 1 podman ps
ok 2 podman images
not ok 3 podman run with networking
# (in test file test/system/500-networking.bats, line 42)
#   `run_podman run --rm --net=host alpine wget -qO- http://localhost:8080' failed
#   expected: 0
#   actual:   7
ok 4 podman stop
"""

LOG_GO_PANIC = """\
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x1234]

goroutine 1 [running]:
main.(*Runtime).Start(0x0, 0xc000123456)
    /go/src/podman/libpod/runtime.go:123 +0x45
"""

LOG_SIGABRT = """\
signal SIGABRT received, stopping runtime
goroutine 42 [running]:
os/signal.signal_recv()
"""

LOG_TIMEOUT = """\
The job running on runner cncf-ubuntu-8-32-x86-abc has exceeded the maximum execution time of 30 minutes
Canceling job...
"""

LOG_CONTEXT_DEADLINE = """\
Error: context deadline exceeded: failed to connect to /run/podman/podman.sock
Error: timed out waiting for container
"""

LOG_OOM = """\
kernel: Out of memory: Killed process 12345 (podman) total-vm:8388608kB
oom-kill: constraint=CONSTRAINT_MEMCG
"""

LOG_DISK_SPACE = """\
Error: writing blob: copying layer sha256:abc123
No space left on device
ENOSPC
"""

LOG_FD_LIMIT = """\
Error: unable to create runtime: Too many open files
"""

LOG_LIMA_FAILURE = """\
FATA[0120] limactl: failed to start VM: error mounting volume
lima VM not running after setup, check host configuration
"""

LOG_RUNNER_FAILURE = """\
The self-hosted runner lost communication with the server.
Receiving request timed out.
"""

LOG_DNS_FAILURE = """\
Error: failed to pull image: could not resolve host: quay.io
Temporary failure in name resolution
"""

LOG_CONNECTION_FAILURE = """\
dial tcp 10.0.2.15:5000: connection refused
Get "https://registry.example.com/v2/": net/http: TLS handshake timeout
"""

LOG_IMAGE_PULL = """\
Error: initializing source docker://quay.io/libpod/testimage:latest:
failed to pull image: manifest unknown
"""

LOG_CRUN_ERROR = """\
crun: error creating container: OCI runtime error
error creating new namespace: operation not permitted
"""

LOG_BUILD_FAILURE = """\
# go.podman.io/podman/v6/cmd/podman
cmd/podman/main.go:42:15: undefined: newCommand
build failed
"""

LOG_DEPENDENCY_FAILURE = """\
dnf install -y golang
Error: No matching packages to list
"""

LOG_PERMISSION_ERROR = """\
Error: Permission denied while trying to bind mount /proc/sys
operation not permitted for rootless user
"""

LOG_CLEAN = """\
Running test suite
ok 1 podman ps
ok 2 podman images
ok 3 podman run
SUCCESS! -- 3 Passed | 0 Failed | 0 Skipped
"""


class TestClassifyText(unittest.TestCase):
    """Tests for classify_text with each failure category."""

    def test_ginkgo_assertion(self):
        result = ci_failure_classifier.classify_text(LOG_GINKGO_ASSERTION)
        self.assertEqual(result.category, "test_assertion_failure")
        self.assertGreater(result.confidence, 0.5)

    def test_bats_assertion(self):
        result = ci_failure_classifier.classify_text(LOG_BATS_ASSERTION)
        self.assertEqual(result.category, "test_assertion_failure")
        self.assertIn("bats", result.rule_name)

    def test_go_panic(self):
        result = ci_failure_classifier.classify_text(LOG_GO_PANIC)
        self.assertEqual(result.category, "panic")
        self.assertGreaterEqual(result.confidence, 0.90)

    def test_sigabrt(self):
        result = ci_failure_classifier.classify_text(LOG_SIGABRT)
        self.assertEqual(result.category, "panic")

    def test_timeout(self):
        result = ci_failure_classifier.classify_text(LOG_TIMEOUT)
        self.assertEqual(result.category, "timeout")
        self.assertGreaterEqual(result.confidence, 0.85)

    def test_context_deadline(self):
        result = ci_failure_classifier.classify_text(LOG_CONTEXT_DEADLINE)
        self.assertEqual(result.category, "timeout")

    def test_oom(self):
        result = ci_failure_classifier.classify_text(LOG_OOM)
        self.assertEqual(result.category, "resource_exhaustion")
        self.assertGreaterEqual(result.confidence, 0.90)

    def test_disk_space(self):
        result = ci_failure_classifier.classify_text(LOG_DISK_SPACE)
        self.assertEqual(result.category, "resource_exhaustion")

    def test_fd_limit(self):
        result = ci_failure_classifier.classify_text(LOG_FD_LIMIT)
        self.assertEqual(result.category, "resource_exhaustion")

    def test_lima_failure(self):
        result = ci_failure_classifier.classify_text(LOG_LIMA_FAILURE)
        self.assertEqual(result.category, "infrastructure_failure")

    def test_runner_failure(self):
        result = ci_failure_classifier.classify_text(LOG_RUNNER_FAILURE)
        self.assertEqual(result.category, "infrastructure_failure")

    def test_dns_failure(self):
        result = ci_failure_classifier.classify_text(LOG_DNS_FAILURE)
        self.assertEqual(result.category, "network_error")

    def test_connection_failure(self):
        result = ci_failure_classifier.classify_text(LOG_CONNECTION_FAILURE)
        self.assertEqual(result.category, "network_error")

    def test_image_pull(self):
        result = ci_failure_classifier.classify_text(LOG_IMAGE_PULL)
        self.assertEqual(result.category, "image_pull_failure")

    def test_crun_error(self):
        result = ci_failure_classifier.classify_text(LOG_CRUN_ERROR)
        self.assertEqual(result.category, "container_runtime_error")

    def test_build_failure(self):
        result = ci_failure_classifier.classify_text(LOG_BUILD_FAILURE)
        self.assertEqual(result.category, "build_failure")

    def test_dependency_failure(self):
        result = ci_failure_classifier.classify_text(LOG_DEPENDENCY_FAILURE)
        self.assertEqual(result.category, "dependency_failure")

    def test_permission_error(self):
        result = ci_failure_classifier.classify_text(LOG_PERMISSION_ERROR)
        self.assertEqual(result.category, "permission_error")

    def test_clean_log_returns_unknown(self):
        result = ci_failure_classifier.classify_text(LOG_CLEAN)
        self.assertEqual(result.category, "unknown")

    def test_empty_text_returns_unknown(self):
        result = ci_failure_classifier.classify_text("")
        self.assertEqual(result.category, "unknown")
        self.assertEqual(result.confidence, 0.0)


class TestPriorityResolution(unittest.TestCase):
    """Tests that higher-priority rules win when multiple rules match."""

    def test_panic_beats_assertion(self):
        """A log with both panic and assertion markers should classify as panic."""
        text = LOG_GO_PANIC + "\n" + LOG_GINKGO_ASSERTION
        result = ci_failure_classifier.classify_text(text)
        self.assertEqual(result.category, "panic")

    def test_timeout_beats_assertion(self):
        """Timeout is more specific than assertion failure."""
        text = LOG_TIMEOUT + "\n" + LOG_BATS_ASSERTION
        result = ci_failure_classifier.classify_text(text)
        self.assertEqual(result.category, "timeout")

    def test_oom_beats_assertion(self):
        """OOM is more specific than assertion failure."""
        text = LOG_OOM + "\n" + LOG_GINKGO_ASSERTION
        result = ci_failure_classifier.classify_text(text)
        self.assertEqual(result.category, "resource_exhaustion")

    def test_all_matches_tracked(self):
        """When multiple rules match, all are recorded."""
        text = LOG_GO_PANIC + "\n" + LOG_GINKGO_ASSERTION
        result = ci_failure_classifier.classify_text(text)
        self.assertGreater(len(result.all_matches), 1)


class TestClassifyFailureRecord(unittest.TestCase):
    """Tests for classify_failure_record with FailureRecord dicts."""

    def test_classifies_from_failure_sections(self):
        record = {
            "run_id": 12345,
            "run_number": 100,
            "workflow": "ci",
            "job_name": "sys local root fedora-current",
            "job_id": 90001,
            "step_name": "Run test on lima",
            "branch": "main",
            "commit_sha": "abc123",
            "commit_message": "test commit",
            "event": "push",
            "created_at": "2026-01-01T00:00:00Z",
            "html_url": "https://example.com",
            "failure_sections": [
                {"type": "ginkgo_failure", "text": LOG_GINKGO_ASSERTION},
            ],
            "normalized_log_excerpt": "",
            "raw_log_bytes": 1024,
        }
        result = ci_failure_classifier.classify_failure_record(record)
        clf = result["classification"]
        self.assertEqual(clf["category"], "test_assertion_failure")
        self.assertGreater(clf["confidence"], 0.0)
        self.assertIn("category_description", clf)

    def test_classifies_from_log_excerpt(self):
        """Falls back to normalized_log_excerpt if no failure_sections."""
        record = {
            "run_id": 12345,
            "run_number": 100,
            "workflow": "ci",
            "job_name": "test-job",
            "job_id": 90001,
            "step_name": "step",
            "branch": "main",
            "commit_sha": "abc",
            "commit_message": "msg",
            "event": "push",
            "created_at": "2026-01-01T00:00:00Z",
            "html_url": "https://example.com",
            "failure_sections": [],
            "normalized_log_excerpt": LOG_GO_PANIC,
            "raw_log_bytes": 512,
        }
        result = ci_failure_classifier.classify_failure_record(record)
        self.assertEqual(result["classification"]["category"], "panic")

    def test_unknown_when_no_content(self):
        record = {
            "run_id": 12345,
            "run_number": 100,
            "workflow": "ci",
            "job_name": "test-job",
            "job_id": 90001,
            "step_name": "step",
            "branch": "main",
            "commit_sha": "abc",
            "commit_message": "msg",
            "event": "push",
            "created_at": "2026-01-01T00:00:00Z",
            "html_url": "https://example.com",
            "failure_sections": [],
            "normalized_log_excerpt": "",
            "raw_log_bytes": 0,
        }
        result = ci_failure_classifier.classify_failure_record(record)
        self.assertEqual(result["classification"]["category"], "unknown")

    def test_preserves_original_fields(self):
        """Classification should augment, not replace, original fields."""
        record = {
            "run_id": 12345,
            "run_number": 100,
            "workflow": "ci",
            "job_name": "test-job",
            "job_id": 90001,
            "step_name": "step",
            "branch": "main",
            "commit_sha": "abc123",
            "commit_message": "test",
            "event": "push",
            "created_at": "2026-01-01T00:00:00Z",
            "html_url": "https://example.com",
            "failure_sections": [],
            "normalized_log_excerpt": LOG_DNS_FAILURE,
            "raw_log_bytes": 256,
        }
        result = ci_failure_classifier.classify_failure_record(record)
        # Original fields preserved.
        self.assertEqual(result["run_id"], 12345)
        self.assertEqual(result["job_name"], "test-job")
        self.assertEqual(result["commit_sha"], "abc123")
        # Classification added.
        self.assertIn("classification", result)
        self.assertEqual(result["classification"]["category"], "network_error")


class TestFormatSummary(unittest.TestCase):
    """Tests for format_classification_summary."""

    def test_groups_by_category(self):
        records = [
            {
                "run_number": 1,
                "job_name": "job-a",
                "classification": {
                    "category": "panic",
                    "confidence": 0.95,
                    "rule_name": "go_panic",
                },
            },
            {
                "run_number": 2,
                "job_name": "job-b",
                "classification": {
                    "category": "panic",
                    "confidence": 0.90,
                    "rule_name": "go_panic",
                },
            },
            {
                "run_number": 3,
                "job_name": "job-c",
                "classification": {
                    "category": "network_error",
                    "confidence": 0.85,
                    "rule_name": "dns_failure",
                },
            },
        ]
        output = ci_failure_classifier.format_classification_summary(records)
        self.assertIn("panic (2)", output)
        self.assertIn("network_error (1)", output)
        self.assertIn("3 failures", output)

    def test_empty_records(self):
        output = ci_failure_classifier.format_classification_summary([])
        self.assertIn("0 failures", output)


class TestClassificationRule(unittest.TestCase):
    """Tests for the ClassificationRule dataclass."""

    def test_match_returns_evidence(self):
        rule = ci_failure_classifier.ClassificationRule(
            category="test",
            name="test_rule",
            patterns=[ci_failure_classifier.re.compile(r"FAIL")],
            confidence=0.9,
            priority=50,
        )
        result = rule.match("This test will FAIL here")
        self.assertIsNotNone(result)
        self.assertEqual(result["category"], "test")
        self.assertEqual(result["matched_text"], "FAIL")
        self.assertIn("FAIL", result["evidence"])

    def test_no_match_returns_none(self):
        rule = ci_failure_classifier.ClassificationRule(
            category="test",
            name="test_rule",
            patterns=[ci_failure_classifier.re.compile(r"NONEXISTENT")],
            confidence=0.9,
            priority=50,
        )
        result = rule.match("Nothing matches here")
        self.assertIsNone(result)


class TestCategoriesComplete(unittest.TestCase):
    """Ensure all categories used in rules have descriptions."""

    def test_all_rule_categories_have_descriptions(self):
        for rule in ci_failure_classifier.RULES:
            self.assertIn(
                rule.category,
                ci_failure_classifier.CATEGORIES,
                f"Rule '{rule.name}' uses category '{rule.category}' "
                f"which has no description in CATEGORIES.",
            )

    def test_unknown_category_exists(self):
        self.assertIn("unknown", ci_failure_classifier.CATEGORIES)


if __name__ == "__main__":
    unittest.main()
