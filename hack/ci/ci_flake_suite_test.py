#!/usr/bin/env python3
"""
ci_flake_suite_test - Unified test runner for all CI failure & flake analysis modules.

Discovers and executes all unit test suites in hack/ci/ related to the CI flake
categorization, retrieval, classification, detection, agentic analysis, and reporting suite.

Usage:
    python3 hack/ci/ci_flake_suite_test.py
    python3 hack/ci/ci_flake_suite_test.py -v
"""

import os
import sys
import unittest

# Ensure hack/ci is in search path
SCRIPT_DIR = os.path.dirname(os.path.realpath(__file__))
sys.path.insert(0, SCRIPT_DIR)

# Import all module test suites
import ci_run_collector_test
import ci_log_retriever_test
import ci_failure_classifier_test
import ci_flake_detector_test
import ci_agentic_analyzer_test
import ci_reporter_test


def suite() -> unittest.TestSuite:
    """Build the consolidated test suite."""
    loader = unittest.TestLoader()
    test_suite = unittest.TestSuite()

    modules = [
        ci_run_collector_test,
        ci_log_retriever_test,
        ci_failure_classifier_test,
        ci_flake_detector_test,
        ci_agentic_analyzer_test,
        ci_reporter_test,
    ]

    for mod in modules:
        test_suite.addTests(loader.loadTestsFromModule(mod))

    return test_suite


def main():
    verbosity = 2 if "-v" in sys.argv or "--verbose" in sys.argv else 1
    runner = unittest.TextTestRunner(verbosity=verbosity)
    result = runner.run(suite())
    sys.exit(0 if result.wasSuccessful() else 1)


if __name__ == "__main__":
    main()
