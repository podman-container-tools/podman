#!/usr/bin/env python3
"""
ci_flake_detector - Detect flaky CI tests from classified failure records.

Analyzes classified failure records across multiple CI runs to identify
tests that fail intermittently (flakes) vs deterministically. Uses
multiple signals: failure frequency, commit diversity, category
correlation, and retry behavior.

Usage:
    # Analyze classified records from stdin
    ./hack/ci/ci_failure_classifier.py --stdin < records.json | \
        ./hack/ci/ci_flake_detector.py --stdin

    # Full pipeline from collector through flake detection
    ./hack/ci/ci_run_collector.py --branch main --status failure --limit 50 --json | \
        ./hack/ci/ci_log_retriever.py --stdin | \
        ./hack/ci/ci_failure_classifier.py --stdin | \
        ./hack/ci/ci_flake_detector.py --stdin

    # Analyze without log download (metadata-only mode)
    ./hack/ci/ci_flake_detector.py --branch main --limit 50 --no-download

Environment:
    GITHUB_TOKEN       Required if fetching data from the API.
    GITHUB_REPOSITORY  Optional. Defaults to 'podman-container-tools/podman'.
"""

import argparse
import json
import os
import sys
from collections import defaultdict
from dataclasses import asdict, dataclass, field
from typing import Optional

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import ci_failure_classifier
import ci_log_retriever
import ci_run_collector


# Categories that strongly correlate with flakiness rather than
# genuine code bugs. These failures are typically environmental.
FLAKE_CORRELATED_CATEGORIES = {
    "network_error",
    "infrastructure_failure",
    "image_pull_failure",
    "resource_exhaustion",
    "timeout",
}

# Categories that strongly suggest a genuine bug, not a flake.
BUG_CORRELATED_CATEGORIES = {
    "test_assertion_failure",
    "build_failure",
    "panic",
}

# Minimum number of failure records needed to compute meaningful
# flake statistics for a given job.
MIN_RECORDS_FOR_ANALYSIS = 2


@dataclass
class FlakeSignals:
    """Individual signals that contribute to the flake score."""

    # How many distinct commits triggered the same failure?
    # More diverse commits → more likely a flake.
    commit_diversity: float = 0.0
    # What fraction of runs for this job failed?
    # Very high or very low → less likely a flake.
    failure_rate: float = 0.0
    # Is the failure category correlated with flakiness?
    category_flake_correlation: float = 0.0
    # Was this failure retried and did the retry succeed?
    retry_signal: float = 0.0
    # How many distinct run dates had this failure?
    temporal_spread: float = 0.0


@dataclass
class FlakeReport:
    """Flake analysis report for a single job/test."""

    job_name: str
    flake_score: float  # 0.0 (definitely not flaky) to 1.0 (definitely flaky)
    is_flaky: bool
    total_failures: int
    unique_commits: int
    unique_dates: int
    categories: dict  # category → count
    signals: dict
    sample_failures: list = field(default_factory=list)


def _extract_date(timestamp: str) -> str:
    """Extract YYYY-MM-DD from an ISO timestamp."""
    if timestamp and len(timestamp) >= 10:
        return timestamp[:10]
    return ""


def compute_flake_signals(records: list) -> FlakeSignals:
    """
    Compute flake signals from a set of failure records for
    the same job name.
    """
    if not records:
        return FlakeSignals()

    signals = FlakeSignals()

    # --- Commit diversity ---
    # If failures span many different commits, it's less likely
    # that a single commit caused the failure → probably a flake.
    commits = set()
    for r in records:
        sha = r.get("commit_sha", "")
        if sha:
            commits.add(sha)

    n_commits = len(commits)
    if n_commits >= 5:
        signals.commit_diversity = 1.0
    elif n_commits >= 3:
        signals.commit_diversity = 0.7
    elif n_commits >= 2:
        signals.commit_diversity = 0.4
    else:
        signals.commit_diversity = 0.0

    # --- Failure rate ---
    # A failure rate between 10% and 80% is the "flake zone".
    # Always-failing or never-failing isn't flaky.
    # Note: We only have failed records, so we approximate using
    # the diversity of commits as a proxy.
    n_records = len(records)
    if n_records >= 10 and n_commits >= 5:
        # Many failures across many commits → persistent flake.
        signals.failure_rate = 0.8
    elif n_records >= 5 and n_commits >= 3:
        signals.failure_rate = 0.6
    elif n_records >= 2:
        signals.failure_rate = 0.3
    else:
        signals.failure_rate = 0.1

    # --- Category correlation ---
    # Aggregate categories across all records.
    categories = defaultdict(int)
    for r in records:
        cat = r.get("classification", {}).get("category", "unknown")
        categories[cat] += 1

    total = sum(categories.values())
    flake_count = sum(
        categories.get(c, 0) for c in FLAKE_CORRELATED_CATEGORIES
    )
    bug_count = sum(
        categories.get(c, 0) for c in BUG_CORRELATED_CATEGORIES
    )

    if total > 0:
        flake_ratio = flake_count / total
        bug_ratio = bug_count / total
        # Flake-correlated categories push the score up;
        # bug-correlated categories push it down.
        signals.category_flake_correlation = max(
            0.0, min(1.0, flake_ratio - bug_ratio * 0.5 + 0.3)
        )

    # --- Retry signal ---
    # If any record has run_attempt > 1, the CI system already
    # considered it worth retrying.
    retry_records = [
        r for r in records if r.get("run_attempt", 1) > 1
    ]
    if retry_records:
        signals.retry_signal = min(1.0, len(retry_records) / len(records) + 0.3)

    # --- Temporal spread ---
    # Failures spread across many days are more likely flakes.
    dates = set()
    for r in records:
        d = _extract_date(r.get("created_at", ""))
        if d:
            dates.add(d)

    n_dates = len(dates)
    if n_dates >= 7:
        signals.temporal_spread = 1.0
    elif n_dates >= 4:
        signals.temporal_spread = 0.7
    elif n_dates >= 2:
        signals.temporal_spread = 0.4
    else:
        signals.temporal_spread = 0.0

    return signals


def compute_flake_score(signals: FlakeSignals) -> float:
    """
    Combine individual signals into a single flake score.

    Uses weighted averaging with signal-specific weights that
    reflect their reliability as flake indicators.
    """
    weights = {
        "commit_diversity": 0.30,
        "category_flake_correlation": 0.25,
        "temporal_spread": 0.20,
        "failure_rate": 0.15,
        "retry_signal": 0.10,
    }

    score = (
        weights["commit_diversity"] * signals.commit_diversity
        + weights["category_flake_correlation"] * signals.category_flake_correlation
        + weights["temporal_spread"] * signals.temporal_spread
        + weights["failure_rate"] * signals.failure_rate
        + weights["retry_signal"] * signals.retry_signal
    )

    return round(min(1.0, max(0.0, score)), 3)


# Threshold above which a job is considered flaky.
FLAKE_THRESHOLD = 0.45


def analyze_flakes(
    records: list, threshold: float = FLAKE_THRESHOLD
) -> list:
    """
    Analyze a list of classified failure records for flakiness.

    Groups records by job name and computes flake signals and scores
    for each job. Returns a list of FlakeReport dicts, sorted by
    flake score descending.
    """
    # Group by job name.
    by_job = defaultdict(list)
    for r in records:
        job_name = r.get("job_name", "unknown")
        by_job[job_name].append(r)

    reports = []
    for job_name, job_records in by_job.items():
        signals = compute_flake_signals(job_records)
        score = compute_flake_score(signals)

        # Aggregate categories.
        categories = defaultdict(int)
        for r in job_records:
            cat = r.get("classification", {}).get("category", "unknown")
            categories[cat] += 1

        # Unique commits and dates.
        commits = set(
            r.get("commit_sha", "") for r in job_records if r.get("commit_sha")
        )
        dates = set(
            _extract_date(r.get("created_at", ""))
            for r in job_records
            if r.get("created_at")
        )

        # Sample failures for context (up to 3).
        samples = []
        for r in job_records[:3]:
            samples.append({
                "run_id": r.get("run_id"),
                "run_number": r.get("run_number"),
                "commit_sha": r.get("commit_sha", "")[:12],
                "created_at": r.get("created_at", ""),
                "category": r.get("classification", {}).get("category", "unknown"),
                "evidence": r.get("classification", {}).get("evidence", "")[:200],
            })

        report = FlakeReport(
            job_name=job_name,
            flake_score=score,
            is_flaky=score >= threshold,
            total_failures=len(job_records),
            unique_commits=len(commits),
            unique_dates=len(dates - {""}),
            categories=dict(categories),
            signals=asdict(signals),
            sample_failures=samples,
        )
        reports.append(asdict(report))

    # Sort by flake score descending, then by total failures.
    reports.sort(
        key=lambda r: (r["flake_score"], r["total_failures"]),
        reverse=True,
    )

    return reports


def format_flake_report(reports: list) -> str:
    """Format flake analysis as a human-readable report."""
    lines = []
    flaky = [r for r in reports if r["is_flaky"]]
    non_flaky = [r for r in reports if not r["is_flaky"]]

    lines.append(f"Flake Analysis Report")
    lines.append(f"{'=' * 60}")
    lines.append(
        f"Total jobs analyzed: {len(reports)}, "
        f"Flaky: {len(flaky)}, "
        f"Non-flaky: {len(non_flaky)}"
    )

    if flaky:
        lines.append(f"\n{'─' * 60}")
        lines.append("FLAKY JOBS (likely intermittent failures)")
        lines.append(f"{'─' * 60}")
        for r in flaky:
            lines.append(
                f"\n  {r['job_name']}"
            )
            lines.append(
                f"    Score: {r['flake_score']:.0%} | "
                f"Failures: {r['total_failures']} | "
                f"Commits: {r['unique_commits']} | "
                f"Days: {r['unique_dates']}"
            )
            cats = ", ".join(
                f"{k}({v})" for k, v in sorted(
                    r["categories"].items(),
                    key=lambda x: x[1],
                    reverse=True,
                )
            )
            lines.append(f"    Categories: {cats}")

            # Show signal breakdown.
            sigs = r["signals"]
            sig_parts = []
            for k in ["commit_diversity", "category_flake_correlation",
                       "temporal_spread", "failure_rate", "retry_signal"]:
                v = sigs.get(k, 0)
                if v > 0:
                    sig_parts.append(f"{k}={v:.1f}")
            if sig_parts:
                lines.append(f"    Signals: {', '.join(sig_parts)}")

    if non_flaky:
        lines.append(f"\n{'─' * 60}")
        lines.append("NON-FLAKY JOBS (likely deterministic failures)")
        lines.append(f"{'─' * 60}")
        for r in non_flaky:
            cats = ", ".join(
                f"{k}({v})" for k, v in sorted(
                    r["categories"].items(),
                    key=lambda x: x[1],
                    reverse=True,
                )
            )
            lines.append(
                f"  {r['job_name']}: "
                f"score={r['flake_score']:.0%}, "
                f"failures={r['total_failures']}, "
                f"cats=[{cats}]"
            )

    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(
        description="Detect flaky CI tests from classified failure records.",
    )
    parser.add_argument(
        "--repo",
        default=os.environ.get("GITHUB_REPOSITORY", ci_log_retriever.DEFAULT_REPO),
        help="GitHub repository",
    )
    parser.add_argument(
        "--branch",
        default="main",
        help="Branch to analyze (default: main)",
    )
    parser.add_argument(
        "--limit",
        type=int,
        default=50,
        help="Number of failed runs to analyze (default: 50)",
    )
    parser.add_argument(
        "--stdin",
        action="store_true",
        help="Read classified records from stdin (JSON)",
    )
    parser.add_argument(
        "--no-download",
        action="store_true",
        help="Skip log download (classify from metadata only)",
    )
    parser.add_argument(
        "--threshold",
        type=float,
        default=FLAKE_THRESHOLD,
        help=f"Flake score threshold (default: {FLAKE_THRESHOLD})",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        dest="json_output",
        help="Output as JSON",
    )
    args = parser.parse_args()

    token = os.environ.get("GITHUB_TOKEN")

    try:
        if args.stdin:
            records = json.load(sys.stdin)
            if isinstance(records, dict):
                records = [records]
        else:
            # Full pipeline: collect → retrieve → classify.
            raw_runs = ci_run_collector.collect_failed_runs(
                repo=args.repo,
                branch=args.branch,
                limit=args.limit,
                token=token,
            )
            records = []
            for run_data in raw_runs:
                failure_records = ci_log_retriever.build_failure_records(
                    run_data,
                    repo=args.repo,
                    token=token,
                    download_logs=not args.no_download,
                )
                for fr in failure_records:
                    classified = ci_failure_classifier.classify_failure_record(fr)
                    records.append(classified)

        reports = analyze_flakes(records, threshold=args.threshold)

        if args.json_output:
            json.dump(reports, sys.stdout, indent=2)
            print()
        else:
            if not reports:
                print("No failure records to analyze.")
            else:
                print(format_flake_report(reports))

    except ci_run_collector.GitHubAPIError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)
    except (json.JSONDecodeError, KeyError) as e:
        print(f"Error: Invalid input data: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
