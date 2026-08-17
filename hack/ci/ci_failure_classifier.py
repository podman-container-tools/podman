#!/usr/bin/env python3
"""
ci_failure_classifier - Deterministic classification of CI failures.

Categorizes CI failure logs using pattern-matching rules based on
real Podman CI failure patterns. Each rule has a category, a set of
regex patterns, a confidence score, and evidence extraction logic.

Designed to handle the most common and well-understood failure types
reliably and testably before any LLM-based analysis (PR 5).

Usage:
    # Classify failures from ci_log_retriever output
    ./hack/ci/ci_log_retriever.py --run-id 31813098642 | \
        ./hack/ci/ci_failure_classifier.py --stdin

    # Classify a single log snippet
    echo '{"normalized_log_excerpt": "panic: nil pointer"}' | \
        ./hack/ci/ci_failure_classifier.py --stdin

Environment:
    GITHUB_TOKEN       Required if using --run-id to fetch data.
    GITHUB_REPOSITORY  Optional. Defaults to 'podman-container-tools/podman'.
"""

import argparse
import json
import os
import re
import sys
from dataclasses import asdict, dataclass, field
from typing import Optional

# Import upstream modules from the pipeline.
sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import ci_run_collector
import ci_log_retriever


# --- Failure categories ---

# Category constants. Each maps to a human-readable description.
CATEGORIES = {
    "test_assertion_failure": (
        "A test assertion failed — the code under test produced an "
        "unexpected result."
    ),
    "panic": (
        "A Go panic, segfault, or fatal runtime error crashed the process."
    ),
    "timeout": (
        "The job or test exceeded its time limit."
    ),
    "network_error": (
        "A network-related failure: DNS resolution, connection refused, "
        "TLS handshake, or HTTP timeout."
    ),
    "infrastructure_failure": (
        "CI infrastructure problem: VM boot, lima setup, runner issue, "
        "or disk/mount error."
    ),
    "resource_exhaustion": (
        "System resource exhaustion: OOM, file descriptor limit, "
        "storage space, or process limit."
    ),
    "container_runtime_error": (
        "Container runtime failure: crun/runc error, namespace setup, "
        "cgroup, or seccomp problem."
    ),
    "image_pull_failure": (
        "Failed to pull a container image from a registry."
    ),
    "build_failure": (
        "Compilation or build error: Go build failure, linker error, "
        "or missing dependency."
    ),
    "dependency_failure": (
        "External dependency failure: package install, pip/go module "
        "download, or missing tool."
    ),
    "permission_error": (
        "Permission or access denied error in test execution."
    ),
    "unknown": (
        "Could not determine the failure category from the available "
        "log content."
    ),
}


# --- Classification rules ---

@dataclass
class ClassificationRule:
    """A single pattern-matching rule for failure classification."""

    category: str
    name: str
    patterns: list  # List of compiled regex patterns (any match triggers).
    confidence: float  # 0.0 to 1.0
    priority: int = 0  # Higher priority rules win ties.

    def match(self, text: str) -> Optional[dict]:
        """
        Test if this rule matches the given text.

        Returns a dict with match details if any pattern matches,
        or None if no match.
        """
        for pattern in self.patterns:
            m = pattern.search(text)
            if m:
                # Extract evidence: the matching line plus context.
                start = max(0, m.start() - 200)
                end = min(len(text), m.end() + 200)
                evidence = text[start:end].strip()
                return {
                    "rule_name": self.name,
                    "category": self.category,
                    "confidence": self.confidence,
                    "priority": self.priority,
                    "matched_pattern": pattern.pattern,
                    "matched_text": m.group(0)[:200],
                    "evidence": evidence[:500],
                }
        return None


def _compile_rules() -> list:
    """
    Build the ordered list of classification rules.

    Rules are ordered by specificity — more specific patterns first,
    with higher priority. When multiple rules match, the highest
    priority+confidence combination wins.
    """
    rules = []

    # --- Panic / crash (highest priority — always significant) ---
    rules.append(ClassificationRule(
        category="panic",
        name="go_panic",
        patterns=[
            re.compile(r"^panic:\s", re.MULTILINE),
            re.compile(r"^fatal error:\s", re.MULTILINE),
            re.compile(r"signal SIGSEGV", re.IGNORECASE),
            re.compile(r"signal SIGABRT", re.IGNORECASE),
            re.compile(r"^goroutine \d+ \[running\]", re.MULTILINE),
        ],
        confidence=0.95,
        priority=100,
    ))

    # --- Timeout ---
    rules.append(ClassificationRule(
        category="timeout",
        name="job_timeout",
        patterns=[
            re.compile(r"The job running on runner .+ has exceeded the maximum execution time", re.IGNORECASE),
            re.compile(r"Error:\s+Timed out after\s+\d+", re.IGNORECASE),
            re.compile(r"context deadline exceeded"),
            re.compile(r"timed?\s*out.*waiting", re.IGNORECASE),
            re.compile(r"test\s+timed?\s*out\s+after", re.IGNORECASE),
        ],
        confidence=0.90,
        priority=90,
    ))

    # --- Resource exhaustion ---
    rules.append(ClassificationRule(
        category="resource_exhaustion",
        name="oom",
        patterns=[
            re.compile(r"Out [Oo]f [Mm]emory", re.IGNORECASE),
            re.compile(r"OOM\s*kill", re.IGNORECASE),
            re.compile(r"Cannot allocate memory", re.IGNORECASE),
            re.compile(r"oom-kill:"),
            re.compile(r"Killed\s+process\s+\d+"),
        ],
        confidence=0.95,
        priority=85,
    ))

    rules.append(ClassificationRule(
        category="resource_exhaustion",
        name="disk_space",
        patterns=[
            re.compile(r"No space left on device", re.IGNORECASE),
            re.compile(r"ENOSPC"),
            re.compile(r"Disk quota exceeded", re.IGNORECASE),
        ],
        confidence=0.95,
        priority=85,
    ))

    rules.append(ClassificationRule(
        category="resource_exhaustion",
        name="fd_limit",
        patterns=[
            re.compile(r"Too many open files", re.IGNORECASE),
            re.compile(r"EMFILE"),
            re.compile(r"ENFILE"),
        ],
        confidence=0.90,
        priority=80,
    ))

    # --- Infrastructure failure ---
    rules.append(ClassificationRule(
        category="infrastructure_failure",
        name="lima_vm_failure",
        patterns=[
            re.compile(r"limactl.*(?:failed|error|timeout)", re.IGNORECASE),
            re.compile(r"lima.*VM.*(?:failed|not running|crashed)", re.IGNORECASE),
            re.compile(r"FATA\[.*\].*lima", re.IGNORECASE),
            re.compile(r"failed to start.*VM", re.IGNORECASE),
        ],
        confidence=0.90,
        priority=80,
    ))

    rules.append(ClassificationRule(
        category="infrastructure_failure",
        name="runner_failure",
        patterns=[
            re.compile(r"Runner\s+.*(?:lost|disconnected|offline)", re.IGNORECASE),
            re.compile(r"The self-hosted runner.*lost communication", re.IGNORECASE),
            re.compile(r"Receiving request timed out", re.IGNORECASE),
        ],
        confidence=0.85,
        priority=75,
    ))

    # --- Network errors ---
    rules.append(ClassificationRule(
        category="network_error",
        name="dns_failure",
        patterns=[
            re.compile(r"(?:Temporary failure|NXDOMAIN).*(?:name resolution|resolving)", re.IGNORECASE),
            re.compile(r"could not resolve host", re.IGNORECASE),
            re.compile(r"Name or service not known", re.IGNORECASE),
            re.compile(r"no such host", re.IGNORECASE),
        ],
        confidence=0.90,
        priority=70,
    ))

    rules.append(ClassificationRule(
        category="network_error",
        name="connection_failure",
        patterns=[
            re.compile(r"connection refused", re.IGNORECASE),
            re.compile(r"connection reset by peer", re.IGNORECASE),
            re.compile(r"connection timed out", re.IGNORECASE),
            re.compile(r"dial tcp.*: i/o timeout"),
            re.compile(r"TLS handshake timeout", re.IGNORECASE),
            re.compile(r"Client\.Timeout exceeded", re.IGNORECASE),
            re.compile(r"net/http:.*timeout", re.IGNORECASE),
        ],
        confidence=0.85,
        priority=70,
    ))

    # --- Image pull failures ---
    rules.append(ClassificationRule(
        category="image_pull_failure",
        name="registry_error",
        patterns=[
            re.compile(r"(?:failed to|error|unable to)\s+pull\s+(?:image|manifest)", re.IGNORECASE),
            re.compile(r"manifest unknown", re.IGNORECASE),
            re.compile(r"(?:401|403)\s+(?:Unauthorized|Forbidden).*registry", re.IGNORECASE),
            re.compile(r"(?:registry|quay\.io|docker\.io).*(?:unavailable|timeout|error)", re.IGNORECASE),
            re.compile(r"Error: initializing source.*registry", re.IGNORECASE),
        ],
        confidence=0.85,
        priority=65,
    ))

    # --- Container runtime errors ---
    rules.append(ClassificationRule(
        category="container_runtime_error",
        name="crun_runc_error",
        patterns=[
            re.compile(r"(?:crun|runc):\s+(?:error|fatal)", re.IGNORECASE),
            re.compile(r"OCI runtime error", re.IGNORECASE),
            re.compile(r"error creating.*(?:namespace|cgroup)", re.IGNORECASE),
            re.compile(r"rootfs_linux\.go.*mounting", re.IGNORECASE),
            re.compile(r"error setting up.*network", re.IGNORECASE),
        ],
        confidence=0.85,
        priority=60,
    ))

    # --- Build failure ---
    rules.append(ClassificationRule(
        category="build_failure",
        name="go_build_error",
        patterns=[
            re.compile(r"^#\s+\S+\n.*\.go:\d+:\d+:.*(?:undefined|cannot|undeclared)", re.MULTILINE),
            re.compile(r"cannot find package", re.IGNORECASE),
            re.compile(r"could not import", re.IGNORECASE),
            re.compile(r"build failed", re.IGNORECASE),
            re.compile(r"(?:linker|ld).*(?:undefined reference|symbol)", re.IGNORECASE),
        ],
        confidence=0.85,
        priority=55,
    ))

    # --- Dependency failure ---
    rules.append(ClassificationRule(
        category="dependency_failure",
        name="package_install_failure",
        patterns=[
            re.compile(r"(?:dnf|apt-get|yum|rpm).*(?:error|failed|unable)", re.IGNORECASE),
            re.compile(r"pip.*(?:error|failed).*install", re.IGNORECASE),
            re.compile(r"go: .*module.*not found", re.IGNORECASE),
            re.compile(r"No matching packages", re.IGNORECASE),
        ],
        confidence=0.80,
        priority=50,
    ))

    # --- Permission errors ---
    rules.append(ClassificationRule(
        category="permission_error",
        name="access_denied",
        patterns=[
            re.compile(r"(?:Permission|Access)\s+denied", re.IGNORECASE),
            re.compile(r"EACCES"),
            re.compile(r"operation not permitted", re.IGNORECASE),
            re.compile(r"(?:rootless|unprivileged).*not.*(?:allowed|supported)", re.IGNORECASE),
        ],
        confidence=0.80,
        priority=45,
    ))

    # --- Test assertion failures (lowest priority — most common, least specific) ---
    rules.append(ClassificationRule(
        category="test_assertion_failure",
        name="ginkgo_assertion",
        patterns=[
            re.compile(r"^\[FAIL\]", re.MULTILINE),
            re.compile(r"^Expected\s*$", re.MULTILINE),
            re.compile(r"Expected\s+\n\s+<.*>:", re.MULTILINE),
            re.compile(r"to equal\s*$", re.MULTILINE),
            re.compile(r"Unexpected error:", re.IGNORECASE),
            re.compile(r"FAIL!\s+--\s+\d+\s+Passed\s*\|\s*[1-9]\d*\s+Failed"),
        ],
        confidence=0.85,
        priority=40,
    ))

    rules.append(ClassificationRule(
        category="test_assertion_failure",
        name="bats_assertion",
        patterns=[
            re.compile(r"^not ok\s+\d+", re.MULTILINE),
            re.compile(r"expected:\s+\d+\s*$\s*actual:\s+\d+", re.MULTILINE),
            re.compile(r"#\s+\(in test file.*\.bats", re.IGNORECASE),
        ],
        confidence=0.85,
        priority=40,
    ))

    return rules


# Module-level compiled rules (built once at import time).
RULES = _compile_rules()


# --- Classification result ---

@dataclass
class ClassificationResult:
    """The result of classifying a single failure record."""

    category: str
    category_description: str
    confidence: float
    rule_name: str
    matched_pattern: str
    matched_text: str
    evidence: str
    all_matches: list = field(default_factory=list)


def classify_text(text: str, rules: Optional[list] = None) -> ClassificationResult:
    """
    Classify a block of text against all rules.

    Returns the highest-priority match, with all matches stored
    in `all_matches` for transparency.
    """
    if rules is None:
        rules = RULES

    all_matches = []
    for rule in rules:
        match = rule.match(text)
        if match:
            all_matches.append(match)

    if not all_matches:
        return ClassificationResult(
            category="unknown",
            category_description=CATEGORIES["unknown"],
            confidence=0.0,
            rule_name="none",
            matched_pattern="",
            matched_text="",
            evidence="",
        )

    # Sort by priority (descending), then confidence (descending).
    all_matches.sort(key=lambda m: (m["priority"], m["confidence"]), reverse=True)
    best = all_matches[0]

    return ClassificationResult(
        category=best["category"],
        category_description=CATEGORIES.get(best["category"], ""),
        confidence=best["confidence"],
        rule_name=best["rule_name"],
        matched_pattern=best["matched_pattern"],
        matched_text=best["matched_text"],
        evidence=best["evidence"],
        all_matches=all_matches,
    )


def classify_failure_record(record: dict) -> dict:
    """
    Classify a FailureRecord (from ci_log_retriever) and return
    the record augmented with classification data.
    """
    # Combine all text sources for classification.
    text_parts = []

    # Prioritize failure sections (most relevant).
    for section in record.get("failure_sections", []):
        text_parts.append(section.get("text", ""))

    # Fall back to the full normalized log excerpt.
    excerpt = record.get("normalized_log_excerpt", "")
    if excerpt:
        text_parts.append(excerpt)

    combined_text = "\n".join(text_parts)

    if not combined_text.strip():
        result = ClassificationResult(
            category="unknown",
            category_description=CATEGORIES["unknown"],
            confidence=0.0,
            rule_name="no_log_content",
            matched_pattern="",
            matched_text="",
            evidence="No log content available for classification.",
        )
    else:
        result = classify_text(combined_text)

    # Augment the record with classification.
    classified = dict(record)
    classified["classification"] = {
        "category": result.category,
        "category_description": result.category_description,
        "confidence": result.confidence,
        "rule_name": result.rule_name,
        "matched_text": result.matched_text,
        "evidence": result.evidence,
        "all_matches_count": len(result.all_matches),
    }

    return classified


def format_classification_summary(records: list) -> str:
    """Format classified records as a human-readable summary."""
    lines = []
    # Group by category.
    by_category = {}
    for rec in records:
        cat = rec.get("classification", {}).get("category", "unknown")
        by_category.setdefault(cat, []).append(rec)

    lines.append(f"Classification Summary ({len(records)} failures)")
    lines.append("=" * 50)

    for cat in sorted(by_category, key=lambda c: len(by_category[c]), reverse=True):
        recs = by_category[cat]
        lines.append(f"\n{cat} ({len(recs)}):")
        lines.append(f"  {CATEGORIES.get(cat, '')}")
        for rec in recs:
            clf = rec.get("classification", {})
            lines.append(
                f"  - Run #{rec.get('run_number', '?')} "
                f"job={rec.get('job_name', '?')!r} "
                f"conf={clf.get('confidence', 0):.0%} "
                f"rule={clf.get('rule_name', '?')}"
            )

    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(
        description="Classify CI failure logs into categories.",
    )
    parser.add_argument(
        "--repo",
        default=os.environ.get("GITHUB_REPOSITORY", ci_log_retriever.DEFAULT_REPO),
        help="GitHub repository",
    )
    parser.add_argument(
        "--run-id",
        type=int,
        help="Fetch and classify failures for a specific run ID",
    )
    parser.add_argument(
        "--stdin",
        action="store_true",
        help="Read failure records from stdin (JSON from ci_log_retriever.py)",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        dest="json_output",
        help="Output as JSON",
    )
    parser.add_argument(
        "--summary",
        action="store_true",
        help="Output human-readable summary (default)",
    )
    args = parser.parse_args()

    token = os.environ.get("GITHUB_TOKEN")

    try:
        if args.stdin:
            records = json.load(sys.stdin)
            if isinstance(records, dict):
                records = [records]
        elif args.run_id:
            # Full pipeline: collect → retrieve logs → classify.
            run_data = ci_run_collector.collect_single_run(
                args.repo, args.run_id, token
            )
            records = ci_log_retriever.build_failure_records(
                run_data, repo=args.repo, token=token
            )
        else:
            parser.error("Specify --run-id or --stdin")
            return

        classified = [classify_failure_record(r) for r in records]

        if args.json_output:
            json.dump(classified, sys.stdout, indent=2)
            print()
        else:
            if not classified:
                print("No failure records to classify.")
            else:
                print(format_classification_summary(classified))

    except ci_run_collector.GitHubAPIError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)
    except (json.JSONDecodeError, KeyError) as e:
        print(f"Error: Invalid input data: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
