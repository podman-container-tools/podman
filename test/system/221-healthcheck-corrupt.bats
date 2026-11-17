#!/usr/bin/env bats   -*- bats -*-
#
# Test for corrupted healthcheck log file handling.
#
# This test must NOT run in parallel (no ci:parallel tag) because it
# deliberately corrupts a healthcheck log which causes podman to emit
# warnings and collateral test failures as a result.
#

load helpers

@test "podman healthcheck - corrupted log file is handled gracefully" {
    skip_if_remote "Warning output is not forwarded to the remote client"
    local TMP_DIR_HEALTHCHECK="$PODMAN_TMPDIR/healthcheck"
    mkdir $TMP_DIR_HEALTHCHECK
    local ctrname="c-h-$(safename)"
    local msg="healthmsg-$(random_string)"
    run_podman run -d --name $ctrname   \
               --health-cmd "echo $msg" \
               --health-log-destination $TMP_DIR_HEALTHCHECK \
               $IMAGE /home/podman/pause
    cid="$output"

    run_podman inspect $ctrname --format "{{.Config.HealthLogDestination}}"
    is "$output" "$TMP_DIR_HEALTHCHECK" "HealthLogDestination"

    # First make sure there is an uncorrupted log.
    run_podman healthcheck run $ctrname
    assert "$output" == "" "output from 'podman healthcheck run'"

    healthcheck_log_path="${TMP_DIR_HEALTHCHECK}/${cid}-healthcheck.log"
    count=$(grep -co "$msg" $healthcheck_log_path)
    assert "$count" -ge 1 "Number of matching health log messages"

    # Corrupt the log file with invalid JSON.
    echo "{invalid json{" > $healthcheck_log_path

    # podman ps must not fail but should warn and report unhealthy.
    run_podman 0+w ps --format '{{.Names}} {{.Status}}'
    assert "$output" =~ "$ctrname" "podman ps must still list the container"
    assert "$output" =~ "unhealthy" "corrupted log should be reported as unhealthy"
    assert "$output" =~ "healthcheck log corrupted" "expected warning about corrupted log from ps"

    # Verify that healthcheck run recovers by overwriting the corrupted
    # log file with a new result while issuing a warning.
    run_podman 0+w healthcheck run $ctrname
    assert "$output" =~ "healthcheck log corrupted" "expected warning about corrupted log"

    count=$(grep -co "$msg" $healthcheck_log_path)
    assert "$count" -ge 1 "Number of matching health log messages after recovery"

    # Run again to verify the warning is gone after the log was rewritten.
    run_podman healthcheck run $ctrname
    assert "$output" == "" "no warnings after log recovery"

    run_podman rm -t 0 -f $ctrname
}
