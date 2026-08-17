#!/usr/bin/env python3
"""
ci_run_collector - Collect GitHub Actions CI run data for failure analysis.

Retrieves workflow run and job data from the GitHub Actions API,
identifies failed runs, and outputs structured JSON suitable for
downstream analysis (classification, flake detection, etc.).

Usage:
    # List recent failed CI runs on main branch
    ./hack/ci/ci_run_collector.py --branch main --status failure --limit 10

    # Get detailed job information for a specific run
    ./hack/ci/ci_run_collector.py --run-id 31813098642

    # Output as JSON for piping to other tools
    ./hack/ci/ci_run_collector.py --branch main --status failure --json

Environment:
    GITHUB_TOKEN     Optional. GitHub API token for authentication.
                     Without it, requests are subject to stricter rate limits.
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


# Default repository; matches the upstream Podman project.
DEFAULT_REPO = "podman-container-tools/podman"
# Default workflow filename for the main CI pipeline.
DEFAULT_WORKFLOW = "ci.yml"
# GitHub API base URL.
API_BASE = "https://api.github.com"
# Maximum results per API page (GitHub caps at 100).
MAX_PER_PAGE = 100
# Default number of runs to retrieve.
DEFAULT_LIMIT = 20
# Maximum number of runs to retrieve in a single invocation to
# avoid accidentally downloading huge amounts of data.
MAX_LIMIT = 500
# Seconds to wait before retrying after a rate-limit or transient error.
RETRY_DELAYS = [1, 2, 4]


@dataclass
class FailedStep:
    """A single failed step within a job."""

    name: str
    conclusion: str
    number: int
    started_at: Optional[str] = None
    completed_at: Optional[str] = None


@dataclass
class JobResult:
    """Summary of a single job within a workflow run."""

    job_id: int
    name: str
    conclusion: str
    status: str
    html_url: str
    started_at: Optional[str] = None
    completed_at: Optional[str] = None
    runner_name: Optional[str] = None
    failed_steps: list = field(default_factory=list)


@dataclass
class RunResult:
    """Summary of a single workflow run."""

    run_id: int
    workflow: str
    run_number: int
    status: str
    conclusion: str
    branch: str
    commit_sha: str
    commit_message: str
    event: str
    html_url: str
    created_at: str
    updated_at: str
    run_attempt: int
    jobs: list = field(default_factory=list)


class GitHubAPIError(Exception):
    """Raised when a GitHub API request fails after retries."""


class RateLimitError(GitHubAPIError):
    """Raised when the GitHub API rate limit is exceeded."""


def _build_headers(token: Optional[str] = None) -> dict:
    """Build HTTP headers for GitHub API requests."""
    headers = {
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
    }
    if token:
        headers["Authorization"] = f"Bearer {token}"
    return headers


def api_request(url: str, token: Optional[str] = None) -> dict:
    """
    Make an authenticated GET request to the GitHub API with retry logic.

    Handles transient errors (5xx, network) and rate limiting with
    exponential backoff. Raises GitHubAPIError after exhausting retries.
    """
    headers = _build_headers(token)
    last_error = None

    for attempt, delay in enumerate(RETRY_DELAYS + [0]):
        try:
            req = urllib.request.Request(url, headers=headers, method="GET")
            with urllib.request.urlopen(req, timeout=30) as resp:
                return json.loads(resp.read().decode("utf-8"))
        except urllib.error.HTTPError as e:
            last_error = e
            if e.code == 403:
                # Check for rate limit.
                reset_time = e.headers.get("X-RateLimit-Reset")
                remaining = e.headers.get("X-RateLimit-Remaining")
                if remaining == "0" and reset_time:
                    wait = max(0, int(reset_time) - int(time.time())) + 1
                    raise RateLimitError(
                        f"GitHub API rate limit exceeded. "
                        f"Resets in {wait}s. "
                        f"Set GITHUB_TOKEN for higher limits."
                    ) from e
            if e.code == 404:
                raise GitHubAPIError(f"Resource not found: {url}") from e
            if e.code < 500 and e.code != 429:
                raise GitHubAPIError(
                    f"GitHub API error {e.code}: {e.reason}"
                ) from e
            # Retry on 5xx and 429.
            if delay:
                time.sleep(delay)
        except (urllib.error.URLError, TimeoutError, OSError) as e:
            last_error = e
            if delay:
                time.sleep(delay)

    raise GitHubAPIError(
        f"GitHub API request failed after {len(RETRY_DELAYS)} retries: "
        f"{last_error}"
    )


def fetch_workflow_runs(
    repo: str,
    branch: Optional[str] = None,
    status: Optional[str] = None,
    workflow: str = DEFAULT_WORKFLOW,
    limit: int = DEFAULT_LIMIT,
    token: Optional[str] = None,
) -> list:
    """
    Fetch workflow runs from the GitHub Actions API.

    Returns a list of raw run dicts, paginated up to `limit` results.
    """
    per_page = min(limit, MAX_PER_PAGE)
    params = [f"per_page={per_page}"]
    if branch:
        params.append(f"branch={branch}")
    if status:
        params.append(f"status={status}")

    url = (
        f"{API_BASE}/repos/{repo}/actions/workflows/{workflow}/runs"
        f"?{'&'.join(params)}"
    )

    all_runs = []
    pages_fetched = 0
    max_pages = (limit + per_page - 1) // per_page

    while url and pages_fetched < max_pages:
        data = api_request(url, token)
        runs = data.get("workflow_runs", [])
        all_runs.extend(runs)
        pages_fetched += 1

        if len(all_runs) >= limit:
            break

        # Follow pagination via Link header logic.
        # The API returns a 'next' link, but since we're using json
        # response, we manually construct the next page URL.
        if len(runs) < per_page:
            break
        page = pages_fetched + 1
        url = (
            f"{API_BASE}/repos/{repo}/actions/workflows/{workflow}/runs"
            f"?{'&'.join(params)}&page={page}"
        )

    return all_runs[:limit]


def fetch_jobs_for_run(
    repo: str, run_id: int, token: Optional[str] = None
) -> list:
    """
    Fetch all jobs for a given workflow run.

    Returns a list of raw job dicts. Handles pagination for runs
    with many jobs.
    """
    url = f"{API_BASE}/repos/{repo}/actions/runs/{run_id}/jobs?per_page=100"
    all_jobs = []

    while url:
        data = api_request(url, token)
        jobs = data.get("jobs", [])
        all_jobs.extend(jobs)

        if len(jobs) < 100:
            break
        # Simple page increment for additional jobs.
        page = (len(all_jobs) // 100) + 1
        url = (
            f"{API_BASE}/repos/{repo}/actions/runs/{run_id}/jobs"
            f"?per_page=100&page={page}"
        )

    return all_jobs


def parse_run(raw_run: dict) -> RunResult:
    """Parse a raw workflow run API response into a RunResult."""
    commit = raw_run.get("head_commit", {}) or {}
    return RunResult(
        run_id=raw_run["id"],
        workflow=raw_run.get("name", ""),
        run_number=raw_run.get("run_number", 0),
        status=raw_run.get("status", ""),
        conclusion=raw_run.get("conclusion", ""),
        branch=raw_run.get("head_branch", ""),
        commit_sha=raw_run.get("head_sha", ""),
        commit_message=commit.get("message", ""),
        event=raw_run.get("event", ""),
        html_url=raw_run.get("html_url", ""),
        created_at=raw_run.get("created_at", ""),
        updated_at=raw_run.get("updated_at", ""),
        run_attempt=raw_run.get("run_attempt", 1),
    )


def parse_job(raw_job: dict) -> JobResult:
    """Parse a raw job API response into a JobResult."""
    failed_steps = []
    for step in raw_job.get("steps", []):
        if step.get("conclusion") == "failure":
            failed_steps.append(
                FailedStep(
                    name=step.get("name", ""),
                    conclusion=step.get("conclusion", ""),
                    number=step.get("number", 0),
                    started_at=step.get("started_at"),
                    completed_at=step.get("completed_at"),
                )
            )

    return JobResult(
        job_id=raw_job["id"],
        name=raw_job.get("name", ""),
        conclusion=raw_job.get("conclusion", ""),
        status=raw_job.get("status", ""),
        html_url=raw_job.get("html_url", ""),
        started_at=raw_job.get("started_at"),
        completed_at=raw_job.get("completed_at"),
        runner_name=raw_job.get("runner_name"),
        failed_steps=[asdict(s) for s in failed_steps],
    )


def collect_failed_runs(
    repo: str = DEFAULT_REPO,
    branch: Optional[str] = None,
    status: str = "failure",
    workflow: str = DEFAULT_WORKFLOW,
    limit: int = DEFAULT_LIMIT,
    include_jobs: bool = True,
    token: Optional[str] = None,
) -> list:
    """
    Collect CI run data with optional job details.

    This is the main entry point for programmatic use. Returns a list
    of RunResult dicts.
    """
    raw_runs = fetch_workflow_runs(
        repo=repo,
        branch=branch,
        status=status,
        workflow=workflow,
        limit=limit,
        token=token,
    )

    results = []
    for raw_run in raw_runs:
        run = parse_run(raw_run)
        if include_jobs:
            raw_jobs = fetch_jobs_for_run(repo, run.run_id, token)
            run.jobs = [asdict(parse_job(j)) for j in raw_jobs]
        results.append(asdict(run))

    return results


def collect_single_run(
    repo: str, run_id: int, token: Optional[str] = None
) -> dict:
    """Collect detailed data for a single workflow run."""
    url = f"{API_BASE}/repos/{repo}/actions/runs/{run_id}"
    raw_run = api_request(url, token)
    run = parse_run(raw_run)

    raw_jobs = fetch_jobs_for_run(repo, run_id, token)
    run.jobs = [asdict(parse_job(j)) for j in raw_jobs]

    return asdict(run)


def format_summary(runs: list) -> str:
    """Format run data as a human-readable summary."""
    lines = []
    for run in runs:
        failed_jobs = [j for j in run.get("jobs", []) if j["conclusion"] == "failure"]
        lines.append(
            f"Run #{run['run_number']} ({run['run_id']}) "
            f"[{run['conclusion']}] "
            f"branch={run['branch']} "
            f"commit={run['commit_sha'][:12]}"
        )
        # Truncate long commit messages to first line.
        msg = run["commit_message"].split("\n")[0]
        lines.append(f"  {msg}")
        if failed_jobs:
            lines.append(f"  Failed jobs ({len(failed_jobs)}):")
            for job in failed_jobs:
                steps = job.get("failed_steps", [])
                step_names = ", ".join(s["name"] for s in steps) if steps else "unknown"
                lines.append(f"    - {job['name']}: {step_names}")
        lines.append("")
    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(
        description="Collect GitHub Actions CI run data for failure analysis.",
        epilog="Set GITHUB_TOKEN env var for authenticated API access.",
    )
    parser.add_argument(
        "--repo",
        default=os.environ.get("GITHUB_REPOSITORY", DEFAULT_REPO),
        help=f"GitHub repository (default: {DEFAULT_REPO})",
    )
    parser.add_argument(
        "--branch",
        default=None,
        help="Filter by branch name (e.g., 'main')",
    )
    parser.add_argument(
        "--status",
        default="failure",
        choices=["failure", "success", "completed", "cancelled"],
        help="Filter by run conclusion (default: failure)",
    )
    parser.add_argument(
        "--workflow",
        default=DEFAULT_WORKFLOW,
        help=f"Workflow filename (default: {DEFAULT_WORKFLOW})",
    )
    parser.add_argument(
        "--limit",
        type=int,
        default=DEFAULT_LIMIT,
        help=f"Maximum number of runs to retrieve (default: {DEFAULT_LIMIT}, max: {MAX_LIMIT})",
    )
    parser.add_argument(
        "--run-id",
        type=int,
        default=None,
        help="Retrieve detailed data for a specific run ID",
    )
    parser.add_argument(
        "--no-jobs",
        action="store_true",
        help="Skip fetching job details (faster, less data)",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        dest="json_output",
        help="Output as JSON instead of human-readable summary",
    )
    args = parser.parse_args()

    if args.limit > MAX_LIMIT:
        print(
            f"Error: --limit must be <= {MAX_LIMIT} to avoid excessive API usage.",
            file=sys.stderr,
        )
        sys.exit(1)

    token = os.environ.get("GITHUB_TOKEN")

    try:
        if args.run_id:
            results = [collect_single_run(args.repo, args.run_id, token)]
        else:
            results = collect_failed_runs(
                repo=args.repo,
                branch=args.branch,
                status=args.status,
                workflow=args.workflow,
                limit=args.limit,
                include_jobs=not args.no_jobs,
                token=token,
            )
    except RateLimitError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(2)
    except GitHubAPIError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)

    if args.json_output:
        json.dump(results, sys.stdout, indent=2)
        print()  # Trailing newline.
    else:
        if not results:
            print("No matching workflow runs found.")
        else:
            print(format_summary(results))


if __name__ == "__main__":
    main()
