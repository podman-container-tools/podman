

# This script attempts to confirm functional github action scripts.
# It expects to be called from Cirrus-CI, in a special execution
# environment.  Any use outside this environment will probably fail.

set -eo pipefail

# Defined by setup_environment.sh
# shellcheck disable=SC2154
if ! ((PREBUILD)); then
    echo "Not operating under expected environment"
    exit 1
fi

expect_regex() {
    local expected_regex
    local input_file
    expected_regex="$1"
    input_file="$2"
    grep -E -q "$expected_regex" $input_file || \
        die "No match to '$expected_regex' in '$(<$input_file)'"
}

req_env_vars CIRRUS_CI CIRRUS_REPO_FULL_NAME CIRRUS_WORKING_DIR CIRRUS_BUILD_ID

# Defined by the CI system
# shellcheck disable=SC2154
cd $CIRRUS_WORKING_DIR || fail

header="Testing cirrus-cron github-action script:"
msg "$header cron_failures.sh"

base=$CIRRUS_WORKING_DIR/.github/actions/check_cirrus_cron
# Don't care about mktemp return value
# shellcheck disable=SC2155
export GITHUB_OUTPUT=$(mktemp -p '' cron_failures_output_XXXX)
# CIRRUS_REPO_FULL_NAME checked above in req_env_vars
# shellcheck disable=SC2154
export GITHUB_REPOSITORY="$CIRRUS_REPO_FULL_NAME"
# shellcheck disable=SC2155
export GITHUB_WORKSPACE=$(mktemp -d -p '' cron_failures_workspace_XXXX)
export GITHUB_WORKFLOW="testing"
# shellcheck disable=SC2155
export ID_NAME_FILEPATH=$(mktemp -p '' cron_failures_data_XXXX)
trap "rm -rf $GITHUB_OUTPUT $GITHUB_WORKSPACE $ID_NAME_FILEPATH" EXIT

#####

cd $GITHUB_WORKSPACE || fail
# Replace newlines and indentation to make grep easier
if ! $base/cron_failures.sh |& \
        tr -s '[:space:]' ' ' > $GITHUB_WORKSPACE/output; then
    die "Failed: $base/cron_failures.sh with output '$(<$GITHUB_WORKSPACE/output)'"
fi

expect_regex \
    'result.+data.+ownerRepository.+cronSettings.+endgroup' \
    "$GITHUB_WORKSPACE/output"

#####

msg "$header make_email_body.sh"
# It's possible no cirrus-cron jobs actually failed
echo -e '\n\n     \n\t\n' >> "$ID_NAME_FILEPATH"  # blank lines should be ignored
# Don't need to test stdout/stderr of this
if ! $base/make_email_body.sh; then
    die "make_email_body.sh failed"
fi

expect_regex \
    '^Detected.+Cirrus-CI.+failed.*' \
    "$GITHUB_WORKSPACE/artifacts/email_body.txt"

#####

msg "$header make_email_body.sh name and link"
# Job names may contain spaces, confirm lines are parsed properly
echo -e '1234567890 cirrus-cron test job' >> "$ID_NAME_FILEPATH"  # Append to blank lines
$base/make_email_body.sh
expected="Cron build 'cirrus-cron test job' Failed: https://cirrus-ci.com/build/1234567890"
if ! grep -q "$expected" $GITHUB_WORKSPACE/artifacts/email_body.txt; then
    die "Expecting to find string '$expected' in generated e-mail body:
$(<$GITHUB_WORKSPACE/artifacts/email_body.txt)"
fi

#####

msg "$header rerun_failed_tasks.sh"
export SECRET_CIRRUS_API_KEY=testing-nottherightkey
# test.sh is sensitive to the 'testing' name.  Var. defined by cirrus-ci
# shellcheck disable=SC2154
echo "$CIRRUS_BUILD_ID test cron job name" > "$ID_NAME_FILEPATH"
if ! $base/rerun_failed_tasks.sh |& \
        tr -s '[:space:]' ' ' > $GITHUB_WORKSPACE/rerun_output; then
    die "rerun_failed_tasks.sh failed"
fi

expect_regex \
    "Posting GraphQL Query.+$CIRRUS_BUILD_ID.+Selecting.+re-run" \
    "$GITHUB_WORKSPACE/rerun_output"
