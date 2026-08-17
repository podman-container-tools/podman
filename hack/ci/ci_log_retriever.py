#!/usr/bin/env python3
"""
ci_log_retriever - Retrieve and normalize CI failure logs from GitHub Actions.

Downloads raw job logs via the GitHub Actions API, strips noise
(ANSI escapes, timestamps, GitHub Actions markup), extracts
failure-relevant sections, and produces structured failure records
suitable for downstream classification and flake detection.

Usage:
    # Retrieve logs for failed jobs from a specific run
    ./hack/ci/ci_log_retriever.py --run-id 31813098642

    # Pipe from ci_run_collector for batch processing
    ./hack/ci/ci_run_collector.py --branch main --status failure --limit 5 --json | \
        ./hack/ci/ci_log_retriever.py --stdin

Environment:
    GITHUB_TOKEN       Required for log downloads (API returns 302 redirects).
    GITHUB_REPOSITORY  Optional. Defaults to 'podman-container-tools/podman'.
"""

import argparse
import io
import json
import os
import re
import sys
import zipfile
from dataclasses import asdict, dataclass, field
from typing import Optional

# Import the API layer from PR 1.
sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import ci_run_collector

# Default repository.
DEFAULT_REPO = "podman-container-tools/podman"
# Maximum bytes of raw log to process per job (10 MB).
MAX_LOG_BYTES = 10 * 1024 * 1024
# Maximum bytes of extracted failure context to keep per record.
MAX_FAILURE_EXCERPT_BYTES = 64 * 1024


# --- ANSI / GHA noise stripping ---

# ANSI escape sequences (colors, cursor movement, etc.).
_ANSI_RE = re.compile(r"\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?\x07")
# GitHub Actions timestamp prefix: "2024-08-14T15:12:34.1234567Z "
_GHA_TIMESTAMP_RE = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z\s?")
# GitHub Actions group markers.
_GHA_GROUP_RE = re.compile(r"^##\[(?:group|endgroup|command|error|warning|notice)\]")
# Logformatter timestamp: "[+NNNs] "
_LOGFORMATTER_TS_RE = re.compile(r"^\[\+\d+s\]\s?")


def strip_ansi(text: str) -> str:
    """Remove ANSI escape sequences from text."""
    return _ANSI_RE.sub("", text)


def strip_gha_timestamps(text: str) -> str:
    """Remove GitHub Actions timestamp prefixes from each line."""
    return "\n".join(
        _GHA_TIMESTAMP_RE.sub("", line) for line in text.split("\n")
    )


def strip_gha_markup(text: str) -> str:
    """Remove GitHub Actions workflow command markup (##[group], etc.)."""
    return "\n".join(
        line for line in text.split("\n") if not _GHA_GROUP_RE.match(line)
    )


def strip_logformatter_timestamps(text: str) -> str:
    """Remove logformatter [+NNNs] timestamp prefixes."""
    return "\n".join(
        _LOGFORMATTER_TS_RE.sub("", line) for line in text.split("\n")
    )


# --- Secret redaction ---

def _redact_hex_token(match: re.Match) -> str:
    """
    Selectively redact long hex strings that look like tokens.

    Preserves git commit SHAs (exactly 40 hex chars in git context)
    and container image digests. Only redacts if the pattern is
    unlikely to be a legitimate identifier.
    """
    value = match.group(0)
    # Preserve 40-char strings that look like git SHAs.
    if len(value) == 40:
        return value
    # Preserve sha256 digest prefixes (64 chars).
    if len(value) == 64:
        return value
    # Redact everything else.
    return f"[REDACTED_TOKEN_{len(value)}chars]"


# Common secret patterns to redact from logs before analysis.
_SECRET_PATTERNS = [
    # GitHub tokens (classic and fine-grained).
    (re.compile(r"ghp_[A-Za-z0-9]{36,}"), "[REDACTED_GH_TOKEN]"),
    (re.compile(r"ghs_[A-Za-z0-9]{36,}"), "[REDACTED_GH_TOKEN]"),
    (re.compile(r"github_pat_[A-Za-z0-9_]{80,}"), "[REDACTED_GH_TOKEN]"),
    # Bearer/Basic auth headers.
    (re.compile(r"(?i)(Bearer|Basic)\s+[A-Za-z0-9+/=_-]{20,}"), "[REDACTED_AUTH]"),
    # URLs with embedded credentials.
    (re.compile(r"https?://[^@\s]+:[^@\s]+@"), "https://[REDACTED]@"),
    # AWS-style keys.
    (re.compile(r"(?:AKIA|ASIA)[A-Z0-9]{16}"), "[REDACTED_AWS_KEY]"),
    # Generic long hex tokens (40+ chars, like SHA tokens).
    (re.compile(r"(?<![a-fA-F0-9])[a-fA-F0-9]{40,}(?![a-fA-F0-9])"), _redact_hex_token),
]


def redact_secrets(text: str) -> str:
    """Redact potential secrets and credentials from log text."""
    for pattern, replacement in _SECRET_PATTERNS:
        if callable(replacement):
            text = pattern.sub(replacement, text)
        else:
            text = pattern.sub(replacement, text)
    return text


# --- Log normalization pipeline ---

def normalize_log(raw_log: str) -> str:
    """
    Apply the full normalization pipeline to raw log content.

    Order matters: strip ANSI first (it can interfere with other
    patterns), then timestamps, then markup, then redact secrets.
    """
    text = strip_ansi(raw_log)
    text = strip_gha_timestamps(text)
    text = strip_gha_markup(text)
    text = strip_logformatter_timestamps(text)
    text = redact_secrets(text)
    # Collapse excessive blank lines.
    text = re.sub(r"\n{4,}", "\n\n\n", text)
    return text


# --- Failure section extraction ---

# Ginkgo failure block markers.
_GINKGO_FAIL_START_RE = re.compile(
    r"^(?:\[FAIL\]|Failure \[|•! Failure|FAIL!)"
    r"|^Unexpected error:|^Expected\s",
    re.MULTILINE,
)
_GINKGO_DIVIDER_RE = re.compile(r"^-{5,}$", re.MULTILINE)
# BATS failure markers.
_BATS_FAIL_RE = re.compile(r"^not ok\s+\d+", re.MULTILINE)
# Generic panic/fatal markers.
_PANIC_RE = re.compile(r"^(?:panic:|fatal error:|SIGSEGV|goroutine \d+)", re.MULTILINE)
# Ginkgo summary line with failure count.
_GINKGO_SUMMARY_RE = re.compile(
    r"^(?:FAIL!|SUCCESS!)\s+--\s+.*\d+\s+Passed", re.MULTILINE
)


def extract_failure_sections(log_text: str) -> list:
    """
    Extract failure-relevant sections from normalized log text.

    Returns a list of (section_type, text) tuples.
    Attempts to extract meaningful context around each failure
    rather than returning the entire (potentially huge) log.
    """
    sections = []
    lines = log_text.split("\n")

    # Strategy: scan for failure markers, then capture context window.
    for i, line in enumerate(lines):
        section_type = None

        if _GINKGO_FAIL_START_RE.search(line):
            section_type = "ginkgo_failure"
        elif _BATS_FAIL_RE.search(line):
            section_type = "bats_failure"
        elif _PANIC_RE.search(line):
            section_type = "panic"
        elif _GINKGO_SUMMARY_RE.search(line):
            section_type = "ginkgo_summary"

        if section_type:
            # Capture context: 5 lines before, the match, and 30 lines after.
            start = max(0, i - 5)
            end = min(len(lines), i + 31)
            context = "\n".join(lines[start:end])

            # Avoid duplicate captures: skip if this overlaps with
            # the previous section.
            if sections and sections[-1][2] >= start:
                # Extend the previous section instead.
                prev_type, prev_text, prev_end = sections[-1]
                if end > prev_end:
                    extended = "\n".join(lines[prev_end:end])
                    sections[-1] = (
                        prev_type,
                        prev_text + "\n" + extended,
                        end,
                    )
                continue

            sections.append((section_type, context, end))

    # Strip internal end-index tracking before returning.
    return [(t, text) for t, text, _ in sections]


# --- Structured failure record ---

@dataclass
class FailureRecord:
    """A structured record of a single CI failure."""

    run_id: int
    run_number: int
    workflow: str
    job_name: str
    job_id: int
    step_name: str
    branch: str
    commit_sha: str
    commit_message: str
    event: str
    created_at: str
    html_url: str
    failure_sections: list = field(default_factory=list)
    normalized_log_excerpt: str = ""
    raw_log_bytes: int = 0


# --- Log retrieval from GitHub Actions API ---

def download_job_log(
    repo: str, job_id: int, token: Optional[str] = None
) -> Optional[str]:
    """
    Download the raw log for a specific job via the GitHub Actions API.

    The API returns a 302 redirect to a time-limited URL for a zip
    file containing the log. We follow the redirect and extract the
    text content.

    Returns the log as a string, or None if retrieval fails.
    """
    import urllib.request
    import urllib.error

    url = f"{ci_run_collector.API_BASE}/repos/{repo}/actions/jobs/{job_id}/logs"
    headers = ci_run_collector._build_headers(token)

    try:
        req = urllib.request.Request(url, headers=headers, method="GET")
        with urllib.request.urlopen(req, timeout=60) as resp:
            content_type = resp.headers.get("Content-Type", "")
            raw_bytes = resp.read(MAX_LOG_BYTES)

            # The API may return a zip file or plain text depending
            # on the redirect target.
            if content_type == "application/zip" or raw_bytes[:4] == b"PK\x03\x04":
                return _extract_zip_log(raw_bytes)
            return raw_bytes.decode("utf-8", errors="replace")
    except (urllib.error.HTTPError, urllib.error.URLError, OSError) as e:
        print(
            f"Warning: Failed to download log for job {job_id}: {e}",
            file=sys.stderr,
        )
        return None


def _extract_zip_log(zip_bytes: bytes) -> str:
    """Extract text content from a zip file containing log data."""
    try:
        with zipfile.ZipFile(io.BytesIO(zip_bytes)) as zf:
            # Concatenate all text files in the zip.
            parts = []
            for name in sorted(zf.namelist()):
                try:
                    content = zf.read(name).decode("utf-8", errors="replace")
                    parts.append(f"=== {name} ===\n{content}")
                except Exception:
                    continue
            return "\n".join(parts)
    except zipfile.BadZipFile:
        return zip_bytes.decode("utf-8", errors="replace")


def build_failure_records(
    run_data: dict,
    repo: str = DEFAULT_REPO,
    token: Optional[str] = None,
    download_logs: bool = True,
) -> list:
    """
    Build structured failure records from a single run's data.

    Takes a run dict (as output by ci_run_collector) and produces
    a list of FailureRecord dicts, one per failed job.
    """
    records = []
    failed_jobs = [
        j for j in run_data.get("jobs", []) if j.get("conclusion") == "failure"
    ]

    for job in failed_jobs:
        failed_steps = job.get("failed_steps", [])
        step_name = failed_steps[0]["name"] if failed_steps else "unknown"

        record = FailureRecord(
            run_id=run_data["run_id"],
            run_number=run_data.get("run_number", 0),
            workflow=run_data.get("workflow", ""),
            job_name=job["name"],
            job_id=job["job_id"],
            step_name=step_name,
            branch=run_data.get("branch", ""),
            commit_sha=run_data.get("commit_sha", ""),
            commit_message=run_data.get("commit_message", "").split("\n")[0],
            event=run_data.get("event", ""),
            created_at=run_data.get("created_at", ""),
            html_url=job.get("html_url", ""),
        )

        if download_logs:
            raw_log = download_job_log(repo, job["job_id"], token)
            if raw_log:
                record.raw_log_bytes = len(raw_log.encode("utf-8"))
                normalized = normalize_log(raw_log)
                # Extract failure sections.
                sections = extract_failure_sections(normalized)
                record.failure_sections = [
                    {"type": t, "text": text[:MAX_FAILURE_EXCERPT_BYTES]}
                    for t, text in sections
                ]
                # Keep a bounded excerpt of the full normalized log.
                record.normalized_log_excerpt = normalized[
                    :MAX_FAILURE_EXCERPT_BYTES
                ]

        records.append(asdict(record))

    return records


def main():
    parser = argparse.ArgumentParser(
        description="Retrieve and normalize CI failure logs.",
        epilog="Requires GITHUB_TOKEN for log downloads.",
    )
    parser.add_argument(
        "--repo",
        default=os.environ.get("GITHUB_REPOSITORY", DEFAULT_REPO),
        help=f"GitHub repository (default: {DEFAULT_REPO})",
    )
    parser.add_argument(
        "--run-id",
        type=int,
        help="Retrieve logs for a specific run ID",
    )
    parser.add_argument(
        "--stdin",
        action="store_true",
        help="Read run data from stdin (JSON from ci_run_collector.py --json)",
    )
    parser.add_argument(
        "--no-download",
        action="store_true",
        help="Skip log download (produce records from metadata only)",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        dest="json_output",
        help="Output as JSON (default)",
    )
    args = parser.parse_args()

    token = os.environ.get("GITHUB_TOKEN")

    try:
        if args.stdin:
            run_data_list = json.load(sys.stdin)
            if isinstance(run_data_list, dict):
                run_data_list = [run_data_list]
        elif args.run_id:
            run_data = ci_run_collector.collect_single_run(
                args.repo, args.run_id, token
            )
            run_data_list = [run_data]
        else:
            parser.error("Specify --run-id or --stdin")
            return

        all_records = []
        for run_data in run_data_list:
            records = build_failure_records(
                run_data,
                repo=args.repo,
                token=token,
                download_logs=not args.no_download,
            )
            all_records.extend(records)

        json.dump(all_records, sys.stdout, indent=2)
        print()

    except ci_run_collector.RateLimitError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(2)
    except ci_run_collector.GitHubAPIError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)
    except (json.JSONDecodeError, KeyError) as e:
        print(f"Error: Invalid input data: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
