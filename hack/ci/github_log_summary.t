#!/usr/bin/env python3
#
# tests for github_log_summary.py
#

import importlib.util
import os
import tempfile
import unittest

TESTS_DIR = os.path.dirname(os.path.abspath(__file__))

# The tool is a script, not a module, so load it by path.
spec = importlib.util.spec_from_file_location(
    "github_log_summary", os.path.join(TESTS_DIR, "github_log_summary.py")
)
github_log_summary = importlib.util.module_from_spec(spec)
spec.loader.exec_module(github_log_summary)


# Trimmed-down versions of what logformatter emits. The ginkgo path wraps
# failures in a "log-failed" span inside the "tt" block; the bats path marks
# up each line with a "bats-*" class.
GINKGO_HTML = """<div class='tt'> <!-- begin processed output -->
<span class="timestamp">[+0298s] </span><span class="log-failed">[FAILED] podman pod correctly sets up PIDNS</span>
<span class="timestamp">[+0298s] </span>expected exit code 0, got 125
</div>
"""

GINKGO_PASSING_HTML = """<div class='tt'> <!-- begin processed output -->
<span class="timestamp">[+0271s] </span>ok, all tests passed
</div>
"""

BATS_HTML = """<div class='tt'> <!-- begin processed output -->
<span class='bats-failed'><a name='t--00001'>not ok 1 podman run</a></span>
<span class='bats-log'># expected 0, got 125</span>
</div>
"""


def summarize(html, name):
    """Write html to a file called name, then run it through the filter."""
    with tempfile.TemporaryDirectory() as tmpdir:
        path = os.path.join(tmpdir, name)
        with open(path, "w", encoding="utf-8") as f:
            f.write(html)
        return github_log_summary.filter_html_file(path)


class TestFormatDetection(unittest.TestCase):
    def test_ginkgo_int_suite(self):
        """The int suite is ginkgo and is detected as such."""
        out = "".join(summarize(GINKGO_HTML, "int-local-root-fedora.log.html"))
        self.assertIn("[FAILED] podman pod correctly sets up PIDNS", out)
        self.assertIn("expected exit code 0, got 125", out)

    def test_ginkgo_suite_not_named_int(self):
        """Ginkgo suites are detected by content, not by the file name.

        The bindings suite is ginkgo but is not called "int-". Keying off the
        name meant its failures were parsed with the bats parser, which found
        no bats markup and so reported nothing useful.
        """
        out = "".join(summarize(GINKGO_HTML, "bindings-root-fedora.log.html"))
        self.assertIn("[FAILED] podman pod correctly sets up PIDNS", out)
        self.assertIn("expected exit code 0, got 125", out)

    def test_bats_suite(self):
        """The bats parser still handles bats logs."""
        out = "".join(summarize(BATS_HTML, "sys-local-root-fedora.log.html"))
        self.assertIn("not ok 1 podman run", out)
        self.assertIn("expected 0, got 125", out)

    def test_ginkgo_without_failures(self):
        """A ginkgo log with no failures has nothing to report."""
        out = "".join(summarize(GINKGO_PASSING_HTML, "int-local-root-fedora.log.html"))
        self.assertEqual(out.strip(), "")


if __name__ == "__main__":
    unittest.main()
