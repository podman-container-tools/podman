#!/bin/bash
#
# tests for pr-should-include-tests
#
# Runs against real historical commits in the podman repo, so it
# only works in a developer tree with full git history (not in a
# shallow clone).
#

ME=$(basename "$0")

###############################################################################
# BEGIN test cases
#
# Syntax is:
#    <exit status>  <merge-base sha>  <commit sha>  <pr>  # comments
#
# Where:
#    exit status       is the expected exit status of the script
#    merge-base sha    is the SHA of the branch point of the commit
#    commit sha        is the SHA of a real commit in the podman repo
#
# We need the actual sha of the merge base because once a branch is
# merged 'git merge-base' becomes useless.
#
tests="
0  68c9e02df  db71759b1   8821  multiple commits, includes tests
0  bb82c37b7  eeb4c129b   8832  single commit, w/tests, merge-base test
1  96eadb51a  2f17614d0  28488  source change with no tests
0  7592f8fbb  6bbe54f2b   8766  no tests, but CI:DOCS in commit message
0  355e38769  bfbd915d6   8884  a vendor bump
0  ffe2b1e95  e467400eb   8899  only .cirrus.yml
0  06a6fd9f2  3cc080151   8695  docs-only, without CI:DOCS
0  a47515008  ecedda63a   8816  unit tests only
0  caa84cd35  e55320efd   8565  hack/podman-socat only
0  c342583da  12f835d12   8523  version.go + podman.spec.in
0  8f75ed958  7b3ad6d89   8835  only a README.md change
0  b6db60e58  f06dd45e0   9420  a test rename
"

# The script we're testing
test_script=$(dirname "$0")/$(basename "$0" .t)

# END   test cases
###############################################################################
# BEGIN test-script runner and status checker

function run_test_script() {
    local expected_rc=$1
    local testname=$2

    testnum=$(( testnum + 1 ))

    # DO NOT COMBINE 'local output=...' INTO ONE LINE. If you do, you lose $?
    local output
    output=$( $test_script )
    local actual_rc=$?

    if [[ $actual_rc != $expected_rc ]]; then
        echo "not ok $testnum $testname"
        echo "#  expected rc $expected_rc"
        echo "#  actual rc   $actual_rc"
        if [[ -n "$output" ]]; then
            echo "# script output: $output"
        fi
        rc=1
    else
        if [[ $expected_rc == 1 ]]; then
            # Confirm we get an error message
            if [[ ! "$output" =~ "Please write a regression test" ]]; then
                echo "not ok $testnum $testname"
                echo "# Expected: ~ 'Please write a regression test'"
                echo "# Actual:   $output"
                rc=1
            else
                echo "ok $testnum $testname - rc=$expected_rc"
            fi
        else
            echo "ok $testnum $testname - rc=$expected_rc"
        fi
    fi

    # If we expect an error, confirm that we can override it. We only need
    # to do this once.
    if [[ $expected_rc == 1 ]]; then
        if [[ -z "$tested_override" ]]; then
            testnum=$(( testnum + 1 ))

            OVERRIDE=true $test_script &>/dev/null
            if [[ $? -ne 0 ]]; then
                echo "not ok $testnum $rest (override with OVERRIDE=true)"
                rc=1
            else
                echo "ok $testnum $rest (override with OVERRIDE=true)"
            fi

            tested_override=1
        fi
    fi
}

# END   test-script runner and status checker
###############################################################################
# BEGIN test-case parsing

rc=0
testnum=0
tested_override=

while read expected_rc parent_sha commit_sha pr rest; do
    # Skip blank lines
    test -z "$expected_rc" && continue

    export BASE_SHA=$parent_sha
    export HEAD_SHA=$commit_sha

    run_test_script $expected_rc "PR $pr - $rest"
done <<<"$tests"

echo "1..$testnum"
exit $rc

# END   Test-case parsing
###############################################################################
