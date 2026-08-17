# Podman CI Failure Categorization, Flake Detection, & Agentic Analysis

> Architecture, CLI reference, and operational guide for the automated CI failure triage and flake analysis suite in `hack/ci/`.

---

## 1. Overview & Architecture

This suite provides an end-to-end data pipeline for analyzing Podman GitHub Actions CI workflow runs, classifying failures into structured categories, calculating multi-signal flake probabilities, providing LLM-assisted root cause analysis, and integrating directly with GitHub (Step Summaries, Issues, and PR triage comments).

### Layered Pipeline Architecture

```
┌────────────────────────────────────────────────────────┐
│  PR 1: Data Ingestion                                  │
│  hack/ci/ci_run_collector.py                           │
│  • GitHub Actions REST API                             │
│  • Workflow runs, jobs, failed steps pagination        │
└──────────────────────────┬─────────────────────────────┘
                           │ RunResult / JobResult (JSON)
┌──────────────────────────▼─────────────────────────────┐
│  PR 2: Failure Log Retrieval & Normalization           │
│  hack/ci/ci_log_retriever.py                           │
│  • ANSI / GHA timestamp / markup stripping             │
│  • Secret / token / credential redaction               │
│  • Contextual Ginkgo / BATS / Panic section extraction │
└──────────────────────────┬─────────────────────────────┘
                           │ FailureRecord (Normalized JSON)
┌──────────────────────────▼─────────────────────────────┐
│  PR 3: Deterministic Failure Classification            │
│  hack/ci/ci_failure_classifier.py                      │
│  • 14 Extensible Regex Rules across 12 Categories      │
│  • Priority resolution & confidence scoring            │
└──────────────────────────┬─────────────────────────────┘
                           │ Classified FailureRecord
             ┌─────────────┴─────────────┐
             ▼                           ▼
┌─────────────────────────────┐ ┌─────────────────────────────┐
│  PR 4: Flake Detection      │ │  PR 5: Agentic Analysis     │
│  hack/ci/ci_flake_detector  │ │  hack/ci/ci_agentic_analyzer│
│  • 5-Signal Weighted Model  │ │  • OpenAI/Local LLM API     │
│  • Commit diversity/Retries │ │  • JSON Schema Validation   │
│  • Flake score (0.0 to 1.0) │ │  • Safe fallback on no key  │
└────────────┬────────────────┘ └─────────────┬───────────────┘
             │ FlakeReport                    │ AgenticAnalysis
             └─────────────┬──────────────────┘
                           │
┌──────────────────────────▼─────────────────────────────┐
│  PR 6: Reporting & GitHub Integration                  │
│  hack/ci/ci_reporter.py                                │
│  • GitHub Step Summary ($GITHUB_STEP_SUMMARY)          │
│  • Deduplicated GitHub Issues with SHA-256 fingerprint │
│  • Factual PR triage comments (LLM policy compliant)   │
└────────────────────────────────────────────────────────┘
```

---

## 2. CLI Tooling Reference

All scripts are standalone, use only Python standard library (`urllib`, `json`, `dataclasses`, `re`, `argparse`, `hashlib`), and compose seamlessly via Unix pipes (`|`).

### 1. `ci_run_collector.py`
Retrieves CI workflow runs and jobs from the GitHub Actions API.

```bash
# List recent failed CI runs on main branch
./hack/ci/ci_run_collector.py --branch main --status failure --limit 10

# Inspect a specific workflow run ID
./hack/ci/ci_run_collector.py --run-id 31813098642

# Output raw structured JSON for downstream piping
./hack/ci/ci_run_collector.py --branch main --status failure --limit 20 --json
```

### 2. `ci_log_retriever.py`
Downloads raw job logs, normalizes them, redacts secrets, and extracts failure sections.

```bash
# Retrieve and normalize logs for a single run
./hack/ci/ci_log_retriever.py --run-id 31813098642

# Process runs piped from ci_run_collector
./hack/ci/ci_run_collector.py --branch main --status failure --limit 5 --json | \
    ./hack/ci/ci_log_retriever.py --stdin
```

### 3. `ci_failure_classifier.py`
Classifies failures into deterministic categories using priority-ordered pattern rules.

```bash
# Classify failure records from stdin
./hack/ci/ci_log_retriever.py --run-id 31813098642 | \
    ./hack/ci/ci_failure_classifier.py --stdin

# Generate human-readable classification summary
./hack/ci/ci_failure_classifier.py --run-id 31813098642 --summary
```

### 4. `ci_flake_detector.py`
Performs multi-signal cross-run analysis to identify intermittent flakes.

```bash
# Analyze recent failures for flakiness (metadata mode)
./hack/ci/ci_flake_detector.py --branch main --limit 50 --no-download

# Full analysis pipeline with custom flake threshold
./hack/ci/ci_flake_detector.py --branch main --limit 30 --threshold 0.50
```

### 5. `ci_agentic_analyzer.py`
Optional LLM-assisted root cause analysis with strict JSON schema validation.

```bash
# Dry run (generate structured prompt without calling LLM)
./hack/ci/ci_agentic_analyzer.py --run-id 31813098642 --dry-run

# Run with LLM API key
export CI_LLM_API_KEY="your-api-key"
export CI_LLM_MODEL="gpt-4o-mini"
./hack/ci/ci_agentic_analyzer.py --run-id 31813098642
```

### 6. `ci_reporter.py`
Formats dashboards, creates deduplicated issues, and posts PR comments.

```bash
# Generate GitHub Actions Step Summary markdown
./hack/ci/ci_flake_detector.py --branch main --limit 20 --json | \
    ./hack/ci/ci_reporter.py --stdin --format step-summary

# Write directly to GitHub Actions Step Summary environment
./hack/ci/ci_reporter.py --stdin --format step-summary --write-step-summary < records.json

# Post PR triage comment (dry-run mode)
./hack/ci/ci_reporter.py --stdin --format pr-comment --pr 29279 --dry-run < records.json
```

---

## 3. End-to-End Pipeline Example

```bash
# Complete automated pipeline: ingest -> normalize -> classify -> detect flakes -> format step summary
./hack/ci/ci_run_collector.py --branch main --status failure --limit 30 --json | \
    ./hack/ci/ci_log_retriever.py --stdin | \
    ./hack/ci/ci_failure_classifier.py --stdin --json | \
    ./hack/ci/ci_flake_detector.py --stdin --json | \
    ./hack/ci/ci_reporter.py --stdin --format step-summary
```

---

## 4. Failure Categories & Classification Rules

The deterministic classifier uses 14 priority-ordered regex rules mapped across 12 distinct categories:

| Priority | Category | Description | Sample Triggers |
|---|---|---|---|
| 100 | `panic` | Go panics, SIGSEGV, SIGABRT crashes | `panic:`, `signal SIGSEGV`, `goroutine N [running]` |
| 90 | `timeout` | Test or job execution timeout | `exceeded the maximum execution time`, `context deadline exceeded` |
| 85 | `resource_exhaustion` | OOM, disk space, file descriptors | `Out of Memory`, `No space left on device`, `EMFILE` |
| 80 | `infrastructure_failure` | Lima VM, runner disconnect | `limactl: failed to start VM`, `runner lost communication` |
| 70 | `network_error` | DNS, connection refused, TLS handshake | `Temporary failure in name resolution`, `TLS handshake timeout` |
| 65 | `image_pull_failure` | Registry download, manifest error | `manifest unknown`, `failed to pull image`, `401 Unauthorized` |
| 60 | `container_runtime_error` | crun/runc namespace/cgroup error | `crun: error creating container`, `OCI runtime error` |
| 55 | `build_failure` | Go compilation or linker error | `undefined: symbol`, `build failed`, `could not import` |
| 50 | `dependency_failure` | Package or module download | `dnf: No matching packages`, `go: module not found` |
| 45 | `permission_error` | Permission or rootless capability | `Permission denied`, `EACCES`, `operation not permitted` |
| 40 | `test_assertion_failure` | Ginkgo or BATS expectation | `[FAIL]`, `Expected ... to equal`, `not ok N` |
| 0 | `unknown` | Unmatched pattern | Fallback for unclassified logs |

---

## 5. Multi-Signal Flake Scoring Model

The flake detector computes an overall probability score between `0.0` (deterministic bug) and `1.0` (certain flake):

$$\text{Flake Score} = 0.30 \cdot S_{\text{commits}} + 0.25 \cdot S_{\text{category}} + 0.20 \cdot S_{\text{dates}} + 0.15 \cdot S_{\text{rate}} + 0.10 \cdot S_{\text{retry}}$$

- **Commit Diversity ($S_{\text{commits}}$)**: Same failure appearing across $\ge 5$ distinct commits indicates environmental flakiness.
- **Category Correlation ($S_{\text{category}}$)**: Network, infra, and timeout failures push score up; pure test assertions push score down.
- **Temporal Spread ($S_{\text{dates}}$)**: Failures spanning multiple days indicate recurring background flakiness.
- **Retry Signal ($S_{\text{retry}}$)**: Failures occurring on `run_attempt > 1` where previous attempts were retried by CI.
- **Default Threshold**: Jobs with score $\ge 0.45$ are flagged as flaky.

---

## 6. Testing & CI Integration

### Running Unit Tests

To run the complete test suite (176 unit tests):

```bash
# Run unified test suite
python3 hack/ci/ci_flake_suite_test.py -v

# Or run individual component test suites
python3 hack/ci/ci_run_collector_test.py -v
python3 hack/ci/ci_log_retriever_test.py -v
python3 hack/ci/ci_failure_classifier_test.py -v
python3 hack/ci/ci_flake_detector_test.py -v
python3 hack/ci/ci_agentic_analyzer_test.py -v
python3 hack/ci/ci_reporter_test.py -v
```

### GitHub Actions Workflow Integration Example

An automated scheduled workflow (`.github/workflows/ci-flake-analyzer.yml`) can run periodically on `main`:

```yaml
name: CI Flake Analyzer
on:
  schedule:
    - cron: '0 6 * * 1' # Weekly on Monday at 06:00 UTC
  workflow_dispatch:

jobs:
  analyze-flakes:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-python@v5
        with:
          python-version: '3.x'
      - name: Run Flake Analysis & Update Step Summary
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          ./hack/ci/ci_run_collector.py --branch main --status failure --limit 50 --json | \
            ./hack/ci/ci_log_retriever.py --stdin | \
            ./hack/ci/ci_failure_classifier.py --stdin --json | \
            ./hack/ci/ci_flake_detector.py --stdin --json | \
            ./hack/ci/ci_reporter.py --stdin --format step-summary --write-step-summary
```
