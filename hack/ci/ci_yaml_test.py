#!/usr/bin/env python3

"""
Verify contents of .github/workflows/ci.yml meets some basic expectations
"""

import sys
import os
import unittest
import yaml
import re

# Assumes directory structure of this file relative to repo.
SCRIPT_DIRPATH = os.path.dirname(os.path.realpath(__file__))
REPO_ROOT = os.path.realpath(os.path.join(SCRIPT_DIRPATH, '..', '..'))


class TestCase(unittest.TestCase):

    CI_YAML = None

    def setUp(self):
        with open(os.path.join(REPO_ROOT, '.github/workflows/ci.yml')) as ci_yaml:
            self.CI_YAML = yaml.safe_load(ci_yaml.read())

    # Critical for the merge protection to work as we only block on this task.
    def test_success_deps(self):
        """success task depends on all others"""
        all_tasks = list(self.CI_YAML['jobs'].keys())
        # need to remove success from the list as it cannot depend on itself
        all_tasks.remove('success')
        needs = self.CI_YAML['jobs']['success']['needs']
        self.assertCountEqual(needs, all_tasks)

    def test_gh_actions_are_pinned_by(self):
        """ensure all actions are pinned by digest and have version comment"""
        # Note local paths are allowed, i.e. uses: ./.github/workflows/lima.yml
        pattern = re.compile(r"uses:\s+(?:(\./[\w./-]+)(?:\s+#.*)?|([\w.-]+/[\w./-]+)@([a-f0-9]{40})\s+#\s*(.+))$")
        dir = os.path.join(REPO_ROOT, '.github/workflows')
        for name in os.listdir(dir):
            with open(os.path.join(dir, name)) as file:
                for i, line in enumerate(file, 1):
                    if 'uses:' in line:
                        self.assertRegex(line, pattern, msg=f"Action must be pinned with a version number comment, file: .github/workflows/{name}:{i}")

    def test_job_timeout(self):
        """ensure all the ci jobs have a timeout set"""
        for job, item in self.CI_YAML['jobs'].items():
            # Note some jobs just start another workflow via uses, in this case no runs-on is set and no timeout can be set so skip it.
            if item.get('runs-on') is not None:
                timeout = item.get('timeout-minutes')
                self.assertIsNotNone(timeout, msg=f"job '{job}' has no timeout-minutes set")
                self.assertLessEqual(timeout, 70, msg=f"job '{job}' should never take longer than 70 minutes")

if __name__ == "__main__":
    unittest.main()
