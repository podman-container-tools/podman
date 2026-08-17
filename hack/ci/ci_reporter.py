#!/usr/bin/env python3
"""
ci_reporter - Generate reports and integrate CI failure analysis with GitHub.

Formats failure classifications and flake analysis reports into:
- GitHub Actions Step Summaries ($GITHUB_STEP_SUMMARY)
- GitHub Issue bodies for tracking intermittent test flakes (with dedup)
- GitHub PR triage comments
- Periodic / weekly flake digests

Usage:
    # Generate GitHub step summary from classified records / flake reports
    ./hack/ci/ci_flake_detector.py --branch main --limit 30 --json | \
        ./hack/ci/ci_reporter.py --stdin --format step-summary

    # Write step summary directly to GITHUB_STEP_SUMMARY environment file
    ./hack/ci/ci_reporter.py --stdin --format step-summary --write-step-summary < analysis.json

    # Generate issue body for a specific flake
    ./hack/ci/ci_reporter.py --stdin --format issue-body < flake_report.json

    # Post or dry-run PR comment
    ./hack/ci/ci_reporter.py --stdin --format pr-comment --pr 29279 --dry-run < analysis.json

Environment:
    GITHUB_TOKEN       Required for creating issues or posting comments.
    GITHUB_REPOSITORY  Optional. Defaults to 'podman-container-tools/podman'.
    GITHUB_STEP_SUMMARY Optional. Path to GitHub Actions step summary file.
"""

import argparse
import hashlib
import json
import os
import sys
import time
import urllib.error
import urllib.request
from dataclasses import asdict, dataclass, field
from typing import Dict, List, Optional, Tuple

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import ci_failure_classifier
import ci_flake_detector
import ci_run_collector

# Default repository.
DEFAULT_REPO = "podman-container-tools/podman"
# Marker tag embedded in issue bodies for reliable deduplication.
FINGERPRINT_PREFIX = "<!-- ci-flake-fingerprint:"
# Marker tag embedded in PR comments for updating existing bot comments.
PR_COMMENT_MARKER = "<!-- podman-ci-failure-analysis -->"
# Maximum new issues created in a single batch to prevent runaway spam.
DEFAULT_MAX_ISSUES = 5
# GitHub issue label for automated flake tracking.
FLAKE_LABEL = "ci-flake"


def compute_fingerprint(job_name: str, primary_category: str = "") -> str:
    """
    Generate a deterministic SHA-256 fingerprint for a job/category combination.
    Used to deduplicate GitHub issues and comments across runs.
    """
    raw = f"{job_name.strip()}::{primary_category.strip()}".lower()
    return hashlib.sha256(raw.encode("utf-8")).hexdigest()[:16]


def format_step_summary(
    records: Optional[List[dict]] = None,
    flake_reports: Optional[List[dict]] = None,
) -> str:
    """
    Generate GitHub Actions Step Summary markdown.

    Produces a clean, structured dashboard showing failure counts,
    classification breakdown, flake alerts, and drill-down details.
    """
    records = records or []
    flake_reports = flake_reports or []

    lines = ["## 🧪 CI Failure & Flake Analysis Dashboard", ""]

    if not records and not flake_reports:
        lines.append("✅ No CI failures or flakes detected in the analyzed runs.")
        return "\n".join(lines)

    # --- Summary Metrics ---
    total_failures = len(records)
    flaky_jobs = [r for r in flake_reports if r.get("is_flaky")]
    non_flaky_jobs = [r for r in flake_reports if not r.get("is_flaky")]

    lines.append("| Metric | Count | Status |")
    lines.append("| :--- | :--- | :--- |")
    lines.append(f"| **Total Failed Jobs Analyzed** | `{total_failures}` | 🔍 |")
    if flake_reports:
        lines.append(f"| **Suspected Flaky Jobs** | `{len(flaky_jobs)}` | {'⚠️ High Attention' if flaky_jobs else '✅ None'} |")
        lines.append(f"| **Deterministic / Genuine Failures** | `{len(non_flaky_jobs)}` | ℹ️ |")
    lines.append("")

    # --- Flake Alerts ---
    if flaky_jobs:
        lines.append("### ⚠️ Flaky Test Warnings")
        lines.append("")
        lines.append("> [!WARNING]")
        lines.append("> The following jobs exhibit intermittent failure patterns across multiple commits or retries.")
        lines.append("")
        lines.append("| Job Name | Flake Score | Failures | Commits | Top Categories |")
        lines.append("| :--- | :--- | :--- | :--- | :--- |")
        for f in flaky_jobs:
            score = f.get("flake_score", 0.0)
            score_pct = f"{score:.0%}"
            cats = ", ".join(f"`{k}` ({v})" for k, v in sorted(f.get("categories", {}).items(), key=lambda x: x[1], reverse=True)[:2])
            lines.append(f"| **{f.get('job_name')}** | **`{score_pct}`** | {f.get('total_failures')} | {f.get('unique_commits')} | {cats or 'N/A'} |")
        lines.append("")

    # --- Category Breakdown ---
    if records:
        category_counts = {}
        for r in records:
            clf = r.get("classification", {})
            cat = clf.get("category", "unknown")
            category_counts[cat] = category_counts.get(cat, 0) + 1

        lines.append("### 📊 Failure Classification Breakdown")
        lines.append("")
        lines.append("| Category | Description | Count | % of Failures |")
        lines.append("| :--- | :--- | :--- | :--- |")
        for cat, cnt in sorted(category_counts.items(), key=lambda x: x[1], reverse=True):
            desc = ci_failure_classifier.CATEGORIES.get(cat, "No description")
            pct = f"{(cnt / total_failures):.1%}"
            lines.append(f"| `{cat}` | {desc} | **{cnt}** | {pct} |")
        lines.append("")

    # --- Detailed Failure Table ---
    if records:
        lines.append("<details><summary><b>🔍 View Individual Failure Details</b> (Click to expand)</summary>")
        lines.append("")
        lines.append("| Run # | Job | Step | Category | Confidence | Root Cause / LLM Refinement |")
        lines.append("| :--- | :--- | :--- | :--- | :--- | :--- |")
        for r in records:
            run_num = r.get("run_number", "?")
            job_name = r.get("job_name", "unknown")
            step_name = r.get("step_name", "unknown")
            clf = r.get("classification", {})
            cat = clf.get("category", "unknown")
            conf = clf.get("confidence", 0.0)

            aa = r.get("agentic_analysis", {})
            if aa and aa.get("status") == "success":
                llm_note = f"**LLM:** {aa.get('root_cause', '')} *(Action: {aa.get('suggested_action', '')})*"
            else:
                evidence = clf.get("matched_text", "")
                llm_note = f"`{evidence[:80]}`" if evidence else "Rule match"

            lines.append(f"| #{run_num} | `{job_name}` | `{step_name}` | `{cat}` | {conf:.0%} | {llm_note} |")
        lines.append("")
        lines.append("</details>")
        lines.append("")

    return "\n".join(lines)


def generate_flake_issue_body(
    flake_report: dict,
    repo: str = DEFAULT_REPO,
) -> Tuple[str, str]:
    """
    Generate issue title and markdown body for a flaky test.

    Includes machine-readable fingerprint comment for deduplication.
    """
    job_name = flake_report.get("job_name", "Unknown Job")
    score = flake_report.get("flake_score", 0.0)
    total_failures = flake_report.get("total_failures", 0)
    unique_commits = flake_report.get("unique_commits", 0)
    categories = flake_report.get("categories", {})
    signals = flake_report.get("signals", {})
    samples = flake_report.get("sample_failures", [])

    primary_category = max(categories, key=categories.get) if categories else "unknown"
    fingerprint = compute_fingerprint(job_name, primary_category)

    title = f"CI Flake: `{job_name}` failing intermittently ({primary_category})"

    lines = [
        f"{FINGERPRINT_PREFIX} {fingerprint} -->",
        f"## CI Flake Report: `{job_name}`",
        "",
        "> [!IMPORTANT]",
        f"> Automated CI flake detection identified intermittent failures in **`{job_name}`**.",
        "",
        "### 📈 Flake Summary",
        f"- **Flake Score:** `{score:.0%}` (Threshold: `{ci_flake_detector.FLAKE_THRESHOLD:.0%}`)",
        f"- **Total Recorded Failures:** `{total_failures}`",
        f"- **Unique Commits Affected:** `{unique_commits}`",
        f"- **Primary Category:** `{primary_category}`",
        "",
        "### 🔬 Signal Breakdown",
        f"- **Commit Diversity:** `{signals.get('commit_diversity', 0.0):.2f}`",
        f"- **Category Correlation:** `{signals.get('category_flake_correlation', 0.0):.2f}`",
        f"- **Temporal Spread:** `{signals.get('temporal_spread', 0.0):.2f}`",
        f"- **Failure Rate:** `{signals.get('failure_rate', 0.0):.2f}`",
        f"- **Retry Signal:** `{signals.get('retry_signal', 0.0):.2f}`",
        "",
        "### 🏷️ Failure Categories",
    ]
    for cat, count in sorted(categories.items(), key=lambda x: x[1], reverse=True):
        lines.append(f"- `{cat}`: {count} failure(s)")

    if samples:
        lines.append("")
        lines.append("### 📝 Recent Failure Samples")
        for s in samples[:3]:
            run_id = s.get("run_id")
            run_num = s.get("run_number", "?")
            commit = s.get("commit_sha", "")
            cat = s.get("category", "")
            evidence = s.get("evidence", "")
            url = f"https://github.com/{repo}/actions/runs/{run_id}" if run_id else ""

            lines.append(f"- **Run #{run_num}** ({f'[{commit}]({url})' if url else commit}) — `{cat}`")
            if evidence:
                lines.append(f"  ```\n  {evidence[:300].strip()}\n  ```")

    lines.extend([
        "",
        "### 🛠️ Recommended Actions for Maintainers",
        "- [ ] Check if the test relies on external network resources or timing constraints",
        "- [ ] Inspect lima VM / runner setup if category is `infrastructure_failure`",
        "- [ ] Check if temporary skip or test refactoring is needed",
        "",
        "---",
        "*Automated report generated by Podman CI Flake Categorization & Analysis system.*",
    ])

    return title, "\n".join(lines)


def generate_pr_comment(
    records: List[dict],
    flake_reports: Optional[List[dict]] = None,
) -> str:
    """
    Generate a factual, concise PR comment summarizing CI failures.
    Strictly follows LLM_POLICY.md guidelines: concise, helpful, objective.
    """
    flake_reports = flake_reports or []
    flaky_job_names = {f.get("job_name"): f for f in flake_reports if f.get("is_flaky")}

    lines = [
        PR_COMMENT_MARKER,
        "### 🤖 Podman CI Failure Triage",
        "",
    ]

    failed_jobs = records
    if not failed_jobs:
        lines.append("All CI jobs passed or no failures analyzed.")
        return "\n".join(lines)

    lines.append(f"Found **{len(failed_jobs)} failed job(s)** in recent CI run:")
    lines.append("")

    for r in failed_jobs:
        job_name = r.get("job_name", "unknown")
        step_name = r.get("step_name", "unknown")
        clf = r.get("classification", {})
        cat = clf.get("category", "unknown")
        url = r.get("html_url", "")

        is_known_flake = job_name in flaky_job_names
        flake_note = " ⚠️ *(Matches known historical flake pattern)*" if is_known_flake else ""

        link = f"[{job_name}]({url})" if url else f"`{job_name}`"
        lines.append(f"- **Job:** {link}{flake_note}")
        lines.append(f"  - **Step:** `{step_name}`")
        lines.append(f"  - **Category:** `{cat}` ({clf.get('confidence', 0):.0%})")

        aa = r.get("agentic_analysis", {})
        if aa and aa.get("status") == "success":
            lines.append(f"  - **Root Cause:** {aa.get('root_cause', '')}")
            if aa.get("suggested_action"):
                lines.append(f"  - **Action:** {aa.get('suggested_action')}")
        elif clf.get("matched_text"):
            lines.append(f"  - **Pattern:** `{clf.get('matched_text', '')[:100]}`")

    lines.append("")
    lines.append("---")
    lines.append("*Triage generated by `hack/ci` automated analysis tooling.*")

    return "\n".join(lines)


# --- GitHub API Helpers for Issues and Comments ---

def find_existing_flake_issue(
    repo: str,
    fingerprint: str,
    token: Optional[str] = None,
) -> Optional[dict]:
    """
    Search for an existing open issue with the given flake fingerprint.
    """
    url = f"{ci_run_collector.API_BASE}/repos/{repo}/issues?state=open&labels={FLAKE_LABEL}&per_page=100"
    try:
        data = ci_run_collector.api_request(url, token=token)
        if isinstance(data, list):
            target_str = f"{FINGERPRINT_PREFIX} {fingerprint}"
            for issue in data:
                body = issue.get("body", "") or ""
                if target_str in body:
                    return issue
    except ci_run_collector.GitHubAPIError as e:
        print(f"Warning: Failed searching issues: {e}", file=sys.stderr)
    return None


def create_or_update_flake_issue(
    repo: str,
    flake_report: dict,
    token: Optional[str] = None,
    dry_run: bool = True,
) -> Tuple[str, dict]:
    """
    Create a new flake issue or update an existing one if a matching
    fingerprint is found. Returns ('created'|'updated'|'dry_run', issue_dict).
    """
    job_name = flake_report.get("job_name", "Unknown Job")
    categories = flake_report.get("categories", {})
    primary_category = max(categories, key=categories.get) if categories else "unknown"
    fingerprint = compute_fingerprint(job_name, primary_category)

    title, body = generate_flake_issue_body(flake_report, repo=repo)

    if dry_run or not token:
        return "dry_run", {"title": title, "body": body, "fingerprint": fingerprint}

    existing = find_existing_flake_issue(repo, fingerprint, token=token)
    if existing:
        issue_number = existing.get("number")
        url = f"{ci_run_collector.API_BASE}/repos/{repo}/issues/{issue_number}"
        # Post a comment to update the existing issue rather than spamming a new issue
        comment_url = f"{url}/comments"
        comment_body = f"🔁 **Flake recurrence update**: `{job_name}` failed again.\n\nLatest flake score: `{flake_report.get('flake_score', 0):.0%}`."
        try:
            req = urllib.request.Request(
                comment_url,
                data=json.dumps({"body": comment_body}).encode("utf-8"),
                headers=ci_run_collector._build_headers(token),
                method="POST",
            )
            with urllib.request.urlopen(req, timeout=30) as resp:
                res = json.loads(resp.read().decode("utf-8"))
                return "updated", res
        except Exception as e:
            print(f"Warning: Failed updating issue #{issue_number}: {e}", file=sys.stderr)
            return "error", {"error": str(e)}

    # Create new issue
    create_url = f"{ci_run_collector.API_BASE}/repos/{repo}/issues"
    payload = {
        "title": title,
        "body": body,
        "labels": [FLAKE_LABEL],
    }
    try:
        req = urllib.request.Request(
            create_url,
            data=json.dumps(payload).encode("utf-8"),
            headers=ci_run_collector._build_headers(token),
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=30) as resp:
            res = json.loads(resp.read().decode("utf-8"))
            return "created", res
    except Exception as e:
        print(f"Warning: Failed creating issue for {job_name}: {e}", file=sys.stderr)
        return "error", {"error": str(e)}


def post_or_update_pr_comment(
    repo: str,
    pr_number: int,
    comment_body: str,
    token: Optional[str] = None,
    dry_run: bool = True,
) -> Tuple[str, dict]:
    """
    Post or update an existing analysis comment on a GitHub PR.
    """
    if dry_run or not token:
        return "dry_run", {"pr_number": pr_number, "body": comment_body}

    # Check for existing comment with marker
    list_url = f"{ci_run_collector.API_BASE}/repos/{repo}/issues/{pr_number}/comments?per_page=100"
    try:
        existing_comments = ci_run_collector.api_request(list_url, token=token)
        if isinstance(existing_comments, list):
            for c in existing_comments:
                if PR_COMMENT_MARKER in (c.get("body") or ""):
                    comment_id = c.get("id")
                    patch_url = f"{ci_run_collector.API_BASE}/repos/{repo}/issues/comments/{comment_id}"
                    req = urllib.request.Request(
                        patch_url,
                        data=json.dumps({"body": comment_body}).encode("utf-8"),
                        headers=ci_run_collector._build_headers(token),
                        method="PATCH",
                    )
                    with urllib.request.urlopen(req, timeout=30) as resp:
                        res = json.loads(resp.read().decode("utf-8"))
                        return "updated", res
    except Exception as e:
        print(f"Warning: Failed checking existing PR comments: {e}", file=sys.stderr)

    # Post new comment
    post_url = f"{ci_run_collector.API_BASE}/repos/{repo}/issues/{pr_number}/comments"
    try:
        req = urllib.request.Request(
            post_url,
            data=json.dumps({"body": comment_body}).encode("utf-8"),
            headers=ci_run_collector._build_headers(token),
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=30) as resp:
            res = json.loads(resp.read().decode("utf-8"))
            return "created", res
    except Exception as e:
        print(f"Warning: Failed posting PR comment: {e}", file=sys.stderr)
        return "error", {"error": str(e)}


# --- CLI Interface ---

def main():
    parser = argparse.ArgumentParser(
        description="Format CI failure analysis and integrate with GitHub Actions / Issues / PRs.",
    )
    parser.add_argument(
        "--repo",
        default=os.environ.get("GITHUB_REPOSITORY", DEFAULT_REPO),
        help=f"GitHub repository (default: {DEFAULT_REPO})",
    )
    parser.add_argument(
        "--stdin",
        action="store_true",
        help="Read input JSON records or flake reports from stdin",
    )
    parser.add_argument(
        "--format",
        choices=["step-summary", "issue-body", "pr-comment", "json"],
        default="step-summary",
        help="Output format (default: step-summary)",
    )
    parser.add_argument(
        "--write-step-summary",
        action="store_true",
        help="Append formatted output directly to $GITHUB_STEP_SUMMARY file",
    )
    parser.add_argument(
        "--create-issues",
        action="store_true",
        help="Create/update GitHub issues for detected flakes",
    )
    parser.add_argument(
        "--max-issues",
        type=int,
        default=DEFAULT_MAX_ISSUES,
        help=f"Maximum issues to create in one run (default: {DEFAULT_MAX_ISSUES})",
    )
    parser.add_argument(
        "--pr",
        type=int,
        help="Post PR comment triage to specified PR number",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        default=False,
        help="Simulate GitHub API write operations without creating issues/comments",
    )
    args = parser.parse_args()

    token = os.environ.get("GITHUB_TOKEN")

    # Load input data
    input_data = []
    if args.stdin:
        try:
            raw = sys.stdin.read().strip()
            if raw:
                input_data = json.loads(raw)
                if isinstance(input_data, dict):
                    input_data = [input_data]
        except Exception as e:
            print(f"Error reading JSON from stdin: {e}", file=sys.stderr)
            sys.exit(1)

    # Determine data shape (FailureRecord list vs FlakeReport list)
    records = []
    flake_reports = []

    if input_data:
        # Check if items are flake reports (contain 'flake_score') or failure records (contain 'job_name' + 'classification')
        if any("flake_score" in item for item in input_data):
            flake_reports = input_data
        else:
            records = input_data
            flake_reports = ci_flake_detector.analyze_flakes(records)

    # Format output
    output_text = ""
    if args.format == "step-summary":
        output_text = format_step_summary(records, flake_reports)
    elif args.format == "issue-body":
        if flake_reports:
            for rep in flake_reports:
                title, body = generate_flake_issue_body(rep, repo=args.repo)
                output_text += f"# {title}\n\n{body}\n\n{'='*60}\n\n"
        else:
            output_text = "No flake reports available to generate issue body."
    elif args.format == "pr-comment":
        output_text = generate_pr_comment(records, flake_reports)
    elif args.format == "json":
        output_text = json.dumps({"records": records, "flake_reports": flake_reports}, indent=2)

    # Print to stdout
    if output_text:
        print(output_text)

    # Write to GITHUB_STEP_SUMMARY if requested
    if args.write_step_summary:
        summary_path = os.environ.get("GITHUB_STEP_SUMMARY")
        if summary_path:
            try:
                with open(summary_path, "a", encoding="utf-8") as f:
                    f.write("\n" + output_text + "\n")
                print(f"Wrote summary to {summary_path}", file=sys.stderr)
            except OSError as e:
                print(f"Error writing to GITHUB_STEP_SUMMARY: {e}", file=sys.stderr)
        else:
            print("Warning: GITHUB_STEP_SUMMARY environment variable not set.", file=sys.stderr)

    # Create issues if requested
    if args.create_issues and flake_reports:
        flaky_to_create = [f for f in flake_reports if f.get("is_flaky")][:args.max_issues]
        print(f"Processing {len(flaky_to_create)} flaky issue(s)...", file=sys.stderr)
        for f in flaky_to_create:
            action, res = create_or_update_flake_issue(
                args.repo, f, token=token, dry_run=args.dry_run
            )
            print(f"  [{action}] {f.get('job_name')}: {res.get('html_url') or res.get('title')}", file=sys.stderr)

    # Post PR comment if requested
    if args.pr:
        pr_comment_body = generate_pr_comment(records, flake_reports)
        action, res = post_or_update_pr_comment(
            args.repo, args.pr, pr_comment_body, token=token, dry_run=args.dry_run
        )
        print(f"PR #{args.pr} comment action: [{action}]", file=sys.stderr)


if __name__ == "__main__":
    main()
