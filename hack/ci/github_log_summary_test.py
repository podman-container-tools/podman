#!/usr/bin/env python3

"""
Verify github_log_summary.py picks the right parser and reports every log
"""

import os
import subprocess
import sys
import tempfile
import unittest

from github_log_summary import GINKGO_CLASS, GINKGO_CLASS_PREFIX

# Assumes directory structure of this file relative to repo.
SCRIPT_DIRPATH = os.path.dirname(os.path.realpath(__file__))
SCRIPT = os.path.join(SCRIPT_DIRPATH, 'github_log_summary.py')
LOGFORMATTER = os.path.join(SCRIPT_DIRPATH, 'logformatter')

# The stylesheet logformatter writes into every log names all the classes, so a
# summary that looked for them in the raw html would call any log a ginkgo one.
STYLE = """<style type="text/css">
.log-failed   { color: #F00; font-weight: bold; font-size: 150%; }
.ginkgo-timeline   { margin-top: 1ex; margin-bottom: 1ex; }
.bats-failed    { color: #F00; font-weight: bold; }
.bats-log-failblock { color: #b00; background-color: #fee; }
</style>"""

GINKGO_FAILURE = "Expected error to be nil, got: exit status 125"
GINKGO_LOG = f"""<html><head>{STYLE}</head><body>
<div class='tt'>
<span class="timestamp">[+0298s] </span><span class="log-failed">[FAILED]</span>
<span class="timestamp">         </span><a name='t--Podman-pod-create--1' /><h2 class="log-failed">  Podman pod create</h2>
<span class="timestamp">         </span>  {GINKGO_FAILURE}
</div>
</body></html>"""

BATS_FAILURE = "# #| FAIL: exit code is 1; expected 0"
BATS_LOG = f"""<html><head>{STYLE}</head><body>
<div class='tt'>
<span class="timestamp">[+0018s] </span><span class='bats-failed'><a name='t--00002'>not ok 2 podman images</a></span>
<span class="timestamp">         </span><span class='bats-log-failblock'>{BATS_FAILURE}</span>
</div>
</body></html>"""

# A job that died before any test ran carries neither layout's markers.
UNMARKED_LOG = f"""<html><head>{STYLE}</head><body>
<div class='tt'>
<span class="timestamp">[+0004s] </span>make: *** [Makefile:1: binaries] Error 2
</div>
</body></html>"""


class TestCase(unittest.TestCase):

    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmpdir.cleanup)

    def write_log(self, name, content):
        path = os.path.join(self.tmpdir.name, name)
        with open(path, 'w') as log:
            log.write(content)
        return path

    def summarize(self, *args):
        return subprocess.run([sys.executable, SCRIPT, *args],
                              capture_output=True, text=True)

    def test_ginkgo_log_not_named_int(self):
        """bindings is a ginkgo suite too, its failures must be reported"""
        log = self.write_log('bindings-root-fedora-current.log.html', GINKGO_LOG)
        result = self.summarize(log)
        self.assertEqual(result.returncode, 0, msg=result.stderr)
        self.assertIn(GINKGO_FAILURE, result.stdout)

    def test_bats_log(self):
        log = self.write_log('sys-local-root-fedora-current.log.html', BATS_LOG)
        result = self.summarize(log)
        self.assertEqual(result.returncode, 0, msg=result.stderr)
        self.assertIn(BATS_FAILURE, result.stdout)

    def test_all_given_logs_are_summarized(self):
        """the workflow passes a glob, none of the matches may be dropped"""
        result = self.summarize(
            self.write_log('int-local-root-fedora-current.log.html', GINKGO_LOG),
            self.write_log('sys-local-root-fedora-current.log.html', BATS_LOG))
        self.assertEqual(result.returncode, 0, msg=result.stderr)
        self.assertIn(GINKGO_FAILURE, result.stdout)
        self.assertIn(BATS_FAILURE, result.stdout)
        self.assertLess(result.stdout.index(GINKGO_FAILURE),
                        result.stdout.index(BATS_FAILURE),
                        msg="logs must be reported in the order given")

    def test_logs_are_named_only_when_there_is_more_than_one(self):
        """a single log needs no heading, several would run into each other"""
        one = self.write_log('int-local-root-fedora-current.log.html', GINKGO_LOG)
        self.assertNotIn('###', self.summarize(one).stdout)

        two = self.write_log('sys-local-root-fedora-current.log.html', BATS_LOG)
        stdout = self.summarize(one, two).stdout
        self.assertIn(f"### {os.path.basename(one)}", stdout)
        self.assertIn(f"### {os.path.basename(two)}", stdout)

    def test_every_failure_block_is_reported(self):
        """ginkgo logs hold one block per failing test, not just one in total"""
        second = GINKGO_FAILURE.replace('125', '126')
        log = self.write_log('int-local-root-fedora-current.log.html',
                             GINKGO_LOG + GINKGO_LOG.replace(GINKGO_FAILURE, second))
        result = self.summarize(log)
        self.assertEqual(result.returncode, 0, msg=result.stderr)
        self.assertIn(GINKGO_FAILURE, result.stdout)
        self.assertIn(second, result.stdout)

    def test_log_without_failures_says_nothing(self):
        """an empty code fence is noise, the step summary is better off blank"""
        log = self.write_log('build-fedora-current.log.html', UNMARKED_LOG)
        result = self.summarize(log)
        self.assertEqual(result.returncode, 0, msg=result.stderr)
        self.assertEqual(result.stdout, '')

    def test_log_holding_invalid_utf8(self):
        """logformatter reads through ':utf8', which lets raw bytes past"""
        path = os.path.join(self.tmpdir.name, 'int-local-root-fedora-current.log.html')
        with open(path, 'wb') as log:
            log.write(GINKGO_LOG.encode().replace(b'exit status 125',
                                                  b'exit status \xff\xfe125'))
        result = self.summarize(path)
        self.assertEqual(result.returncode, 0, msg=result.stderr)
        self.assertIn('Expected error to be nil', result.stdout)

    def test_logformatter_still_emits_the_classes_we_look_for(self):
        """the layout is detected by classes logformatter has to keep emitting"""
        with open(LOGFORMATTER) as source:
            body = source.read()
        self.assertIn(GINKGO_CLASS, body)
        self.assertIn(GINKGO_CLASS_PREFIX, body)

    def test_unreadable_log_is_skipped(self):
        """jobs without any html log hand over an unexpanded glob"""
        result = self.summarize(
            os.path.join(self.tmpdir.name, '*.html'),
            self.write_log('sys-local-root-fedora-current.log.html', BATS_LOG))
        self.assertEqual(result.returncode, 0, msg=result.stderr)
        self.assertIn(BATS_FAILURE, result.stdout)
        self.assertIn('skipping', result.stderr)

    def test_no_logs_given(self):
        result = self.summarize()
        self.assertEqual(result.returncode, 2)
        self.assertIn('usage', result.stderr)


if __name__ == "__main__":
    unittest.main()
