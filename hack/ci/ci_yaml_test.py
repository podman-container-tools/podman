#!/usr/bin/env python3

"""
Verify contents of .github/workflows/ci.yml meet specific expectations
"""

import sys
import os
import unittest
import yaml

# Assumes directory structure of this file relative to repo.
SCRIPT_DIRPATH = os.path.dirname(os.path.realpath(__file__))
REPO_ROOT = os.path.realpath(os.path.join(SCRIPT_DIRPATH, '../', '../'))


class TestCaseBase(unittest.TestCase):

    CI_YAML = None

    def setUp(self):
        with open(os.path.join(REPO_ROOT, '.github/workflows/ci.yml')) as ci_yaml:
            self.CI_YAML = yaml.safe_load(ci_yaml.read())


class TestDependsOn(TestCaseBase):

    ALL_TASK_NAMES = None

    def setUp(self):
        super().setUp()
        self.ALL_TASK_NAMES = list(self.CI_YAML['jobs'].keys())


    def test_success_deps(self):
        """Specific success task depends on all others"""
        all_tasks = self.ALL_TASK_NAMES.remove('success')
        needs = self.CI_YAML['jobs']['success']['needs']
        self.assertCountEqual(needs, self.ALL_TASK_NAMES)

if __name__ == "__main__":
    unittest.main()
