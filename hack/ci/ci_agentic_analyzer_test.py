#!/usr/bin/env python3
"""
Unit tests for ci_agentic_analyzer.py

Run with:
    python3 -m pytest hack/ci/ci_agentic_analyzer_test.py -v
    python3 hack/ci/ci_agentic_analyzer_test.py
"""

import json
import os
import sys
import unittest
from unittest.mock import MagicMock, patch

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))

import ci_agentic_analyzer


# --- Test fixtures ---

SAMPLE_CLASSIFIED_RECORD = {
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
        {
            "type": "ginkgo_failure",
            "text": "[FAIL] Podman run with volume\n  Expected\n    /tmp/testdir\n  to equal\n    /tmp/expected",
        },
    ],
    "normalized_log_excerpt": "[FAIL] test excerpt",
    "raw_log_bytes": 1024,
    "classification": {
        "category": "test_assertion_failure",
        "category_description": "A test assertion failed.",
        "confidence": 0.85,
        "rule_name": "ginkgo_assertion",
        "matched_text": "[FAIL]",
        "evidence": "[FAIL] Podman run with volume",
        "all_matches_count": 1,
    },
}

VALID_LLM_RESPONSE = json.dumps({
    "refined_category": "test_assertion_failure",
    "root_cause": "Volume mount path mismatch in test expectations.",
    "is_flake_assessment": "likely_bug",
    "confidence": 0.9,
    "suggested_action": "Update test fixture to use the correct expected path.",
    "reasoning": "The test expects /tmp/expected but gets /tmp/testdir, suggesting a hardcoded path issue.",
})

VALID_LLM_RESPONSE_WRAPPED = f"```json\n{VALID_LLM_RESPONSE}\n```"

INVALID_JSON_RESPONSE = "This is not JSON at all, just text analysis."

PARTIAL_JSON_RESPONSE = json.dumps({
    "refined_category": "test_assertion_failure",
    # Missing several required fields.
})

UNKNOWN_CATEGORY_RESPONSE = json.dumps({
    "refined_category": "made_up_category",
    "root_cause": "Something.",
    "is_flake_assessment": "invalid_value",
    "confidence": 5.0,
    "suggested_action": "Do something.",
    "reasoning": "Because.",
})


class TestBuildAnalysisPrompt(unittest.TestCase):
    """Tests for build_analysis_prompt."""

    def test_includes_metadata(self):
        prompt = ci_agentic_analyzer.build_analysis_prompt(SAMPLE_CLASSIFIED_RECORD)
        self.assertIn("sys local root fedora-current / lima", prompt)
        self.assertIn("Run test on lima", prompt)
        self.assertIn("main", prompt)
        self.assertIn("3a18c5e98f5c", prompt)
        self.assertIn("test_assertion_failure", prompt)

    def test_includes_failure_context(self):
        prompt = ci_agentic_analyzer.build_analysis_prompt(SAMPLE_CLASSIFIED_RECORD)
        self.assertIn("[FAIL] Podman run with volume", prompt)

    def test_includes_classification(self):
        prompt = ci_agentic_analyzer.build_analysis_prompt(SAMPLE_CLASSIFIED_RECORD)
        self.assertIn("ginkgo_assertion", prompt)
        self.assertIn("85%", prompt)

    def test_no_failure_sections_uses_excerpt(self):
        record = {
            **SAMPLE_CLASSIFIED_RECORD,
            "failure_sections": [],
        }
        prompt = ci_agentic_analyzer.build_analysis_prompt(record)
        self.assertIn("test excerpt", prompt)

    def test_no_content_shows_placeholder(self):
        record = {
            **SAMPLE_CLASSIFIED_RECORD,
            "failure_sections": [],
            "normalized_log_excerpt": "",
        }
        prompt = ci_agentic_analyzer.build_analysis_prompt(record)
        self.assertIn("no log content available", prompt)

    def test_context_bounded(self):
        long_text = "x" * 20000
        record = {
            **SAMPLE_CLASSIFIED_RECORD,
            "failure_sections": [{"type": "test", "text": long_text}],
        }
        prompt = ci_agentic_analyzer.build_analysis_prompt(record)
        # Prompt should not include the full 20K chars.
        self.assertLess(len(prompt), 15000)


class TestParseLlmResponse(unittest.TestCase):
    """Tests for parse_llm_response."""

    def test_valid_response(self):
        result = ci_agentic_analyzer.parse_llm_response(VALID_LLM_RESPONSE)
        self.assertEqual(result.refined_category, "test_assertion_failure")
        self.assertEqual(result.is_flake_assessment, "likely_bug")
        self.assertAlmostEqual(result.confidence, 0.9)
        self.assertIn("Volume mount", result.root_cause)
        self.assertEqual(result.parse_error, "")

    def test_valid_response_with_code_fence(self):
        result = ci_agentic_analyzer.parse_llm_response(VALID_LLM_RESPONSE_WRAPPED)
        self.assertEqual(result.refined_category, "test_assertion_failure")
        self.assertEqual(result.parse_error, "")

    def test_invalid_json(self):
        result = ci_agentic_analyzer.parse_llm_response(INVALID_JSON_RESPONSE)
        self.assertNotEqual(result.parse_error, "")
        self.assertIn("Invalid JSON", result.parse_error)

    def test_empty_response(self):
        result = ci_agentic_analyzer.parse_llm_response("")
        self.assertNotEqual(result.parse_error, "")
        self.assertEqual(result.refined_category, "unknown")

    def test_partial_response(self):
        result = ci_agentic_analyzer.parse_llm_response(PARTIAL_JSON_RESPONSE)
        self.assertEqual(result.refined_category, "test_assertion_failure")
        self.assertEqual(result.root_cause, "")
        self.assertEqual(result.is_flake_assessment, "uncertain")
        self.assertEqual(result.parse_error, "")

    def test_unknown_category_sanitized(self):
        result = ci_agentic_analyzer.parse_llm_response(UNKNOWN_CATEGORY_RESPONSE)
        # Invalid category should fall back to "unknown".
        self.assertEqual(result.refined_category, "unknown")
        # Invalid flake assessment should fall back to "uncertain".
        self.assertEqual(result.is_flake_assessment, "uncertain")
        # Confidence should be capped at 1.0.
        self.assertLessEqual(result.confidence, 1.0)

    def test_bounds_on_long_strings(self):
        response = json.dumps({
            "refined_category": "panic",
            "root_cause": "x" * 2000,
            "is_flake_assessment": "likely_bug",
            "confidence": 0.8,
            "suggested_action": "y" * 2000,
            "reasoning": "z" * 2000,
        })
        result = ci_agentic_analyzer.parse_llm_response(response)
        self.assertLessEqual(len(result.root_cause), 500)
        self.assertLessEqual(len(result.suggested_action), 500)
        self.assertLessEqual(len(result.reasoning), 500)

    def test_non_dict_json(self):
        result = ci_agentic_analyzer.parse_llm_response("[1, 2, 3]")
        self.assertNotEqual(result.parse_error, "")
        self.assertIn("Expected JSON object", result.parse_error)


class TestAnalyzeRecord(unittest.TestCase):
    """Tests for analyze_record."""

    def test_dry_run(self):
        result = ci_agentic_analyzer.analyze_record(
            SAMPLE_CLASSIFIED_RECORD, dry_run=True
        )
        aa = result["agentic_analysis"]
        self.assertEqual(aa["status"], "dry_run")
        self.assertIn("sys local root fedora", aa["prompt"])

    def test_no_api_key(self):
        config = ci_agentic_analyzer.LLMConfig(api_key="")
        result = ci_agentic_analyzer.analyze_record(
            SAMPLE_CLASSIFIED_RECORD, config=config
        )
        self.assertEqual(result["agentic_analysis"]["status"], "no_api_key")

    def test_no_config(self):
        result = ci_agentic_analyzer.analyze_record(
            SAMPLE_CLASSIFIED_RECORD, config=None
        )
        self.assertEqual(result["agentic_analysis"]["status"], "no_api_key")

    @patch("ci_agentic_analyzer.call_llm")
    def test_successful_llm_call(self, mock_call):
        mock_call.return_value = VALID_LLM_RESPONSE
        config = ci_agentic_analyzer.LLMConfig(api_key="test-key")
        result = ci_agentic_analyzer.analyze_record(
            SAMPLE_CLASSIFIED_RECORD, config=config
        )
        aa = result["agentic_analysis"]
        self.assertEqual(aa["status"], "success")
        self.assertEqual(aa["refined_category"], "test_assertion_failure")
        self.assertEqual(aa["is_flake_assessment"], "likely_bug")

    @patch("ci_agentic_analyzer.call_llm")
    def test_llm_api_error(self, mock_call):
        mock_call.return_value = None
        config = ci_agentic_analyzer.LLMConfig(api_key="test-key")
        result = ci_agentic_analyzer.analyze_record(
            SAMPLE_CLASSIFIED_RECORD, config=config
        )
        self.assertEqual(result["agentic_analysis"]["status"], "api_error")

    @patch("ci_agentic_analyzer.call_llm")
    def test_llm_invalid_response(self, mock_call):
        mock_call.return_value = INVALID_JSON_RESPONSE
        config = ci_agentic_analyzer.LLMConfig(api_key="test-key")
        result = ci_agentic_analyzer.analyze_record(
            SAMPLE_CLASSIFIED_RECORD, config=config
        )
        self.assertIn("parse_error", result["agentic_analysis"]["status"])

    def test_preserves_original_fields(self):
        result = ci_agentic_analyzer.analyze_record(
            SAMPLE_CLASSIFIED_RECORD, dry_run=True
        )
        self.assertEqual(result["run_id"], 31813098642)
        self.assertEqual(result["job_name"], "sys local root fedora-current / lima")
        self.assertIn("classification", result)


class TestAnalyzeRecords(unittest.TestCase):
    """Tests for analyze_records (batch processing)."""

    def test_dry_run_batch(self):
        records = [SAMPLE_CLASSIFIED_RECORD, SAMPLE_CLASSIFIED_RECORD]
        results = ci_agentic_analyzer.analyze_records(records, dry_run=True)
        self.assertEqual(len(results), 2)
        for r in results:
            self.assertEqual(r["agentic_analysis"]["status"], "dry_run")

    @patch("ci_agentic_analyzer.call_llm")
    @patch("ci_agentic_analyzer.time.sleep")
    def test_rate_limiting(self, mock_sleep, mock_call):
        mock_call.return_value = VALID_LLM_RESPONSE
        config = ci_agentic_analyzer.LLMConfig(api_key="test-key")
        records = [SAMPLE_CLASSIFIED_RECORD] * 3
        results = ci_agentic_analyzer.analyze_records(records, config=config)
        self.assertEqual(len(results), 3)
        # Should have rate-limited between calls (n-1 sleeps).
        self.assertEqual(mock_sleep.call_count, 2)


class TestFormatAnalysisSummary(unittest.TestCase):
    """Tests for format_analysis_summary."""

    def test_format_dry_run(self):
        records = ci_agentic_analyzer.analyze_records(
            [SAMPLE_CLASSIFIED_RECORD], dry_run=True
        )
        output = ci_agentic_analyzer.format_analysis_summary(records)
        self.assertIn("DRY RUN", output)
        self.assertIn("Run #1503", output)

    @patch("ci_agentic_analyzer.call_llm")
    def test_format_success(self, mock_call):
        mock_call.return_value = VALID_LLM_RESPONSE
        config = ci_agentic_analyzer.LLMConfig(api_key="test-key")
        records = ci_agentic_analyzer.analyze_records(
            [SAMPLE_CLASSIFIED_RECORD], config=config
        )
        output = ci_agentic_analyzer.format_analysis_summary(records)
        self.assertIn("LLM Refined", output)
        self.assertIn("Root Cause", output)
        self.assertIn("Volume mount", output)

    def test_format_no_api_key(self):
        config = ci_agentic_analyzer.LLMConfig(api_key="")
        records = ci_agentic_analyzer.analyze_records(
            [SAMPLE_CLASSIFIED_RECORD], config=config
        )
        output = ci_agentic_analyzer.format_analysis_summary(records)
        self.assertIn("no CI_LLM_API_KEY", output)


class TestLLMConfig(unittest.TestCase):
    """Tests for LLMConfig defaults."""

    def test_defaults(self):
        config = ci_agentic_analyzer.LLMConfig()
        self.assertEqual(config.api_url, ci_agentic_analyzer.DEFAULT_API_URL)
        self.assertEqual(config.model, ci_agentic_analyzer.DEFAULT_MODEL)
        self.assertEqual(config.api_key, "")
        self.assertLess(config.temperature, 0.5)

    def test_custom_config(self):
        config = ci_agentic_analyzer.LLMConfig(
            api_url="https://custom.api/v1/chat",
            api_key="custom-key",
            model="custom-model",
        )
        self.assertEqual(config.api_url, "https://custom.api/v1/chat")
        self.assertEqual(config.api_key, "custom-key")


class TestSystemPrompt(unittest.TestCase):
    """Tests for the system prompt content."""

    def test_prompt_requests_json(self):
        self.assertIn("JSON", ci_agentic_analyzer.SYSTEM_PROMPT)

    def test_prompt_includes_schema(self):
        self.assertIn("refined_category", ci_agentic_analyzer.SYSTEM_PROMPT)
        self.assertIn("root_cause", ci_agentic_analyzer.SYSTEM_PROMPT)
        self.assertIn("is_flake_assessment", ci_agentic_analyzer.SYSTEM_PROMPT)

    def test_prompt_includes_constraints(self):
        self.assertIn("Do not speculate", ci_agentic_analyzer.SYSTEM_PROMPT)
        self.assertIn("Never fabricate", ci_agentic_analyzer.SYSTEM_PROMPT)


if __name__ == "__main__":
    unittest.main()
