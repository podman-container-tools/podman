#!/usr/bin/env python3
"""
ci_agentic_analyzer - LLM-powered analysis of CI failures.

Provides deeper analysis of CI failure patterns using a large language
model (LLM). Builds on the deterministic pipeline (PRs 1-4) by adding
contextual reasoning about failure causes, suggested fixes, and
correlation with known issues.

The LLM layer is optional and gracefully degrades — the deterministic
pipeline produces fully usable results without it.

Usage:
    # Analyze classified failures with LLM enhancement
    ./hack/ci/ci_failure_classifier.py --stdin < records.json | \
        ./hack/ci/ci_agentic_analyzer.py --stdin

    # Analyze a specific run
    ./hack/ci/ci_agentic_analyzer.py --run-id 31813098642

    # Dry-run: show prompt without calling LLM
    ./hack/ci/ci_agentic_analyzer.py --stdin --dry-run < records.json

Environment:
    CI_LLM_API_KEY     Required. API key for the LLM service.
    CI_LLM_API_URL     LLM API endpoint URL.
                       Default: https://api.openai.com/v1/chat/completions
    CI_LLM_MODEL       Model identifier.
                       Default: gpt-4o-mini
    GITHUB_TOKEN       Required if using --run-id.
    GITHUB_REPOSITORY  Optional. Defaults to 'podman-container-tools/podman'.
"""

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.request
from dataclasses import asdict, dataclass, field
from typing import Optional

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import ci_failure_classifier
import ci_log_retriever
import ci_run_collector

# --- Configuration ---

DEFAULT_API_URL = "https://api.openai.com/v1/chat/completions"
DEFAULT_MODEL = "gpt-4o-mini"
# Maximum tokens to request from the LLM.
MAX_RESPONSE_TOKENS = 1024
# Maximum characters of failure context to include in prompt.
MAX_CONTEXT_CHARS = 8000
# Rate limit: minimum seconds between API calls.
MIN_CALL_INTERVAL = 1.0
# Maximum retries for LLM API calls.
LLM_RETRY_DELAYS = [2, 5]


# --- Prompt engineering ---

SYSTEM_PROMPT = """\
You are a CI failure analysis assistant for the Podman container engine project.
Your task is to analyze CI test failure logs and provide structured analysis.

You will receive:
1. Failure metadata (job name, branch, commit, classification)
2. Log excerpts showing the failure context
3. The deterministic classification from the rule-based system

Your analysis should:
- Confirm or refine the rule-based classification
- Identify the likely root cause
- Suggest whether this is a flake or a genuine bug
- Provide a concise actionable suggestion

IMPORTANT CONSTRAINTS:
- Be concise. Each field should be 1-2 sentences.
- Do not speculate beyond what the log evidence supports.
- If the evidence is insufficient, say so explicitly.
- Never fabricate file paths, function names, or line numbers.

You MUST respond with valid JSON matching this exact schema:
{
  "refined_category": "<category string>",
  "root_cause": "<1-2 sentence description>",
  "is_flake_assessment": "<likely_flake | likely_bug | uncertain>",
  "confidence": <0.0 to 1.0>,
  "suggested_action": "<1-2 sentence actionable suggestion>",
  "reasoning": "<brief reasoning for the assessment>"
}
"""


def build_analysis_prompt(record: dict) -> str:
    """
    Build a structured user prompt from a classified failure record.

    Includes metadata, classification, and bounded failure context.
    """
    clf = record.get("classification", {})
    sections = record.get("failure_sections", [])
    excerpt = record.get("normalized_log_excerpt", "")

    # Build context from failure sections (preferred) or excerpt.
    context_parts = []
    total_chars = 0
    for section in sections:
        text = section.get("text", "")
        if total_chars + len(text) > MAX_CONTEXT_CHARS:
            text = text[: MAX_CONTEXT_CHARS - total_chars]
        context_parts.append(f"[{section.get('type', 'unknown')}]\n{text}")
        total_chars += len(text)
        if total_chars >= MAX_CONTEXT_CHARS:
            break

    if not context_parts and excerpt:
        context_parts.append(excerpt[:MAX_CONTEXT_CHARS])

    context = "\n---\n".join(context_parts) if context_parts else "(no log content available)"

    prompt = f"""\
Analyze this CI failure:

## Metadata
- Job: {record.get('job_name', 'unknown')}
- Step: {record.get('step_name', 'unknown')}
- Branch: {record.get('branch', 'unknown')}
- Commit: {record.get('commit_sha', 'unknown')[:12]}
- Commit message: {record.get('commit_message', 'unknown')[:100]}
- Event: {record.get('event', 'unknown')}
- Date: {record.get('created_at', 'unknown')}

## Rule-Based Classification
- Category: {clf.get('category', 'unknown')}
- Confidence: {clf.get('confidence', 0):.0%}
- Rule: {clf.get('rule_name', 'unknown')}
- Matched: {clf.get('matched_text', '')[:100]}

## Failure Log Context
{context}

Respond with valid JSON only.
"""
    return prompt


# --- LLM API interaction ---

@dataclass
class LLMConfig:
    """Configuration for the LLM API."""

    api_url: str = DEFAULT_API_URL
    api_key: str = ""
    model: str = DEFAULT_MODEL
    max_tokens: int = MAX_RESPONSE_TOKENS
    temperature: float = 0.1  # Low temperature for consistent analysis.


def call_llm(
    prompt: str,
    config: LLMConfig,
    system_prompt: str = SYSTEM_PROMPT,
) -> Optional[str]:
    """
    Call the LLM API with the given prompt.

    Returns the raw response text, or None if the call fails.
    Uses an OpenAI-compatible chat completions API.
    """
    if not config.api_key:
        return None

    payload = {
        "model": config.model,
        "messages": [
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": prompt},
        ],
        "max_tokens": config.max_tokens,
        "temperature": config.temperature,
    }

    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {config.api_key}",
    }

    data = json.dumps(payload).encode("utf-8")
    last_error = None

    for attempt, delay in enumerate(LLM_RETRY_DELAYS + [0]):
        try:
            req = urllib.request.Request(
                config.api_url,
                data=data,
                headers=headers,
                method="POST",
            )
            with urllib.request.urlopen(req, timeout=60) as resp:
                result = json.loads(resp.read().decode("utf-8"))
                choices = result.get("choices", [])
                if choices:
                    return choices[0].get("message", {}).get("content", "")
                return None
        except (urllib.error.HTTPError, urllib.error.URLError, OSError) as e:
            last_error = e
            if isinstance(e, urllib.error.HTTPError) and e.code < 500 and e.code != 429:
                print(
                    f"Warning: LLM API error {e.code}: {e.reason}",
                    file=sys.stderr,
                )
                return None
            if delay:
                time.sleep(delay)

    print(
        f"Warning: LLM API failed after retries: {last_error}",
        file=sys.stderr,
    )
    return None


# --- Response validation ---

VALID_CATEGORIES = set(ci_failure_classifier.CATEGORIES.keys())
VALID_FLAKE_ASSESSMENTS = {"likely_flake", "likely_bug", "uncertain"}


@dataclass
class AgenticAnalysis:
    """Validated analysis result from the LLM."""

    refined_category: str = "unknown"
    root_cause: str = ""
    is_flake_assessment: str = "uncertain"
    confidence: float = 0.0
    suggested_action: str = ""
    reasoning: str = ""
    raw_response: str = ""
    parse_error: str = ""


def parse_llm_response(raw_response: str) -> AgenticAnalysis:
    """
    Parse and validate an LLM response into a structured analysis.

    Treats LLM output as untrusted: validates all fields, applies
    bounds checking, and falls back gracefully on parse errors.
    """
    analysis = AgenticAnalysis(raw_response=raw_response)

    if not raw_response:
        analysis.parse_error = "Empty response from LLM"
        return analysis

    # Strip markdown code fences if present.
    text = raw_response.strip()
    if text.startswith("```"):
        # Remove ```json ... ``` wrapper.
        lines = text.split("\n")
        if lines[0].startswith("```"):
            lines = lines[1:]
        if lines and lines[-1].strip() == "```":
            lines = lines[:-1]
        text = "\n".join(lines)

    try:
        data = json.loads(text)
    except json.JSONDecodeError as e:
        analysis.parse_error = f"Invalid JSON from LLM: {e}"
        return analysis

    if not isinstance(data, dict):
        analysis.parse_error = f"Expected JSON object, got {type(data).__name__}"
        return analysis

    # Validate and extract fields with bounds checking.

    # refined_category: must be a known category.
    cat = data.get("refined_category", "unknown")
    if isinstance(cat, str) and cat in VALID_CATEGORIES:
        analysis.refined_category = cat
    else:
        analysis.refined_category = "unknown"

    # root_cause: bounded string.
    rc = data.get("root_cause", "")
    if isinstance(rc, str):
        analysis.root_cause = rc[:500]

    # is_flake_assessment: must be one of the valid values.
    fa = data.get("is_flake_assessment", "uncertain")
    if isinstance(fa, str) and fa in VALID_FLAKE_ASSESSMENTS:
        analysis.is_flake_assessment = fa
    else:
        analysis.is_flake_assessment = "uncertain"

    # confidence: bounded float.
    conf = data.get("confidence", 0.0)
    if isinstance(conf, (int, float)):
        analysis.confidence = max(0.0, min(1.0, float(conf)))

    # suggested_action: bounded string.
    sa = data.get("suggested_action", "")
    if isinstance(sa, str):
        analysis.suggested_action = sa[:500]

    # reasoning: bounded string.
    reasoning = data.get("reasoning", "")
    if isinstance(reasoning, str):
        analysis.reasoning = reasoning[:500]

    return analysis


# --- Main analysis pipeline ---

def analyze_record(
    record: dict,
    config: Optional[LLMConfig] = None,
    dry_run: bool = False,
) -> dict:
    """
    Analyze a single classified failure record with optional LLM.

    If the LLM is unavailable or dry_run is True, the record is
    returned with the deterministic classification only.
    """
    prompt = build_analysis_prompt(record)

    result = dict(record)
    result["agentic_analysis"] = {
        "prompt": prompt if dry_run else "",
        "status": "skipped",
        "refined_category": "",
        "root_cause": "",
        "is_flake_assessment": "",
        "confidence": 0.0,
        "suggested_action": "",
        "reasoning": "",
    }

    if dry_run:
        result["agentic_analysis"]["status"] = "dry_run"
        return result

    if config is None or not config.api_key:
        result["agentic_analysis"]["status"] = "no_api_key"
        return result

    raw_response = call_llm(prompt, config)
    if raw_response is None:
        result["agentic_analysis"]["status"] = "api_error"
        return result

    analysis = parse_llm_response(raw_response)

    if analysis.parse_error:
        result["agentic_analysis"]["status"] = f"parse_error: {analysis.parse_error}"
        result["agentic_analysis"]["raw_response"] = raw_response[:1000]
        return result

    result["agentic_analysis"] = {
        "status": "success",
        "refined_category": analysis.refined_category,
        "root_cause": analysis.root_cause,
        "is_flake_assessment": analysis.is_flake_assessment,
        "confidence": analysis.confidence,
        "suggested_action": analysis.suggested_action,
        "reasoning": analysis.reasoning,
    }

    return result


def analyze_records(
    records: list,
    config: Optional[LLMConfig] = None,
    dry_run: bool = False,
) -> list:
    """
    Analyze multiple records with rate limiting between LLM calls.
    """
    results = []
    for i, record in enumerate(records):
        result = analyze_record(record, config, dry_run)
        results.append(result)

        # Rate limit between calls.
        if (
            not dry_run
            and config
            and config.api_key
            and i < len(records) - 1
        ):
            time.sleep(MIN_CALL_INTERVAL)

    return results


def format_analysis_summary(records: list) -> str:
    """Format agentic analysis results as a human-readable report."""
    lines = []
    lines.append("Agentic Analysis Report")
    lines.append("=" * 60)

    for rec in records:
        aa = rec.get("agentic_analysis", {})
        clf = rec.get("classification", {})

        lines.append(f"\nRun #{rec.get('run_number', '?')} | {rec.get('job_name', '?')}")
        lines.append(f"  Deterministic: {clf.get('category', '?')} ({clf.get('confidence', 0):.0%})")

        status = aa.get("status", "unknown")
        if status == "success":
            lines.append(f"  LLM Refined:   {aa.get('refined_category', '?')} ({aa.get('confidence', 0):.0%})")
            lines.append(f"  Root Cause:    {aa.get('root_cause', 'N/A')}")
            lines.append(f"  Flake?:        {aa.get('is_flake_assessment', '?')}")
            lines.append(f"  Action:        {aa.get('suggested_action', 'N/A')}")
        elif status == "dry_run":
            lines.append("  LLM: [DRY RUN — prompt generated but not sent]")
        elif status == "no_api_key":
            lines.append("  LLM: [SKIPPED — no CI_LLM_API_KEY set]")
        else:
            lines.append(f"  LLM: [{status}]")

    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(
        description="LLM-powered CI failure analysis.",
        epilog="Set CI_LLM_API_KEY for LLM analysis. Without it, "
        "only deterministic analysis is performed.",
    )
    parser.add_argument(
        "--repo",
        default=os.environ.get("GITHUB_REPOSITORY", ci_log_retriever.DEFAULT_REPO),
        help="GitHub repository",
    )
    parser.add_argument(
        "--run-id",
        type=int,
        help="Analyze failures for a specific run ID",
    )
    parser.add_argument(
        "--stdin",
        action="store_true",
        help="Read classified records from stdin (JSON)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Generate prompts without calling LLM",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        dest="json_output",
        help="Output as JSON",
    )
    args = parser.parse_args()

    token = os.environ.get("GITHUB_TOKEN")
    config = LLMConfig(
        api_url=os.environ.get("CI_LLM_API_URL", DEFAULT_API_URL),
        api_key=os.environ.get("CI_LLM_API_KEY", ""),
        model=os.environ.get("CI_LLM_MODEL", DEFAULT_MODEL),
    )

    try:
        if args.stdin:
            records = json.load(sys.stdin)
            if isinstance(records, dict):
                records = [records]
        elif args.run_id:
            run_data = ci_run_collector.collect_single_run(
                args.repo, args.run_id, token
            )
            failure_records = ci_log_retriever.build_failure_records(
                run_data, repo=args.repo, token=token
            )
            records = [
                ci_failure_classifier.classify_failure_record(r)
                for r in failure_records
            ]
        else:
            parser.error("Specify --run-id or --stdin")
            return

        results = analyze_records(records, config, dry_run=args.dry_run)

        if args.json_output:
            json.dump(results, sys.stdout, indent=2)
            print()
        else:
            print(format_analysis_summary(results))

    except ci_run_collector.GitHubAPIError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)
    except (json.JSONDecodeError, KeyError) as e:
        print(f"Error: Invalid input data: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
