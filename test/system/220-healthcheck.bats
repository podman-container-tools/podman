#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman healthcheck
#
#

load helpers
load helpers.systemd

# bats file_tags=ci:parallel

# Helper function: run 'podman inspect' and check various given fields
function _check_health {
    local ctrname="$1"
    local testname="$2"
    local tests="$3"
    local since="$4"
    local hc_status="$5"

    # Loop-wait (up to a few seconds) for healthcheck event (#20342)
    # Allow a margin when running parallel, because of system load
    local timeout=5
    if [[ -n "$PARALLEL_JOBSLOT" ]]; then
        timeout=$((timeout + 3))
    fi

    while :; do
        run_podman events --filter container=$ctrname --filter event=health_status \
                   --since "$since" --stream=false --format "{{.HealthStatus}}"
        # Output may be empty or multiple lines.
        if [[ -n "$output" ]]; then
            if [[ "${lines[-1]}" = "$hc_status" ]]; then
                break
            fi
        fi

        timeout=$((timeout - 1))
        if [[ $timeout -eq 0 ]]; then
            die "$testname - timed out waiting for '$hc_status' in podman events"
        fi
        sleep 1
    done

    # Got the desired status. Now verify all the healthcheck fields
    run_podman inspect --format "{{json .State.Healthcheck}}" $ctrname

    defer-assertion-failures
    parse_table "$tests" | while read field expect;do
        actual=$(jq ".$field" <<<"$output")
        is "$actual" "$expect" "$testname - .State.Healthcheck.$field"
    done
    immediate-assertion-failures
}

@test "podman healthcheck" {
    local ctrname="c-h-$(safename)"
    run_podman run -d --name $ctrname                  \
               --health-cmd /home/podman/healthcheck   \
               --health-interval 1s                    \
               --health-retries 3                      \
               --health-on-failure=kill                \
               --health-startup-cmd /home/podman/healthcheck \
               --health-startup-interval 1s                  \
               $IMAGE /home/podman/pause
    cid="$output"

    run_podman inspect $ctrname --format "{{.Config.HealthcheckOnFailureAction}}"
    is "$output" "kill" "on-failure action is set to kill"

    run_podman inspect $ctrname --format "{{.Config.StartupHealthCheck.Test}}"
    is "$output" "[CMD-SHELL /home/podman/healthcheck]" ".Config.StartupHealthCheck.Test"

    current_time=$(date --iso-8601=ns)
    # We can't check for 'starting' because a 1-second interval is too
    # short; it could run healthcheck before we get to our first check.
    #
    # So, just force a healthcheck run, then confirm that it's running.
    run_podman healthcheck run $ctrname
    is "$output" "" "output from 'podman healthcheck run'"

    _check_health $ctrname "All healthy" "
Status           | \"healthy\"
FailingStreak    | 0
Log[-1].ExitCode | 0
Log[-1].Output   | \"Life is Good on stdout\\\nLife is Good on stderr\\\n\"
" "$current_time" "healthy"

    current_time=$(date --iso-8601=ns)
    # Force a failure
    run_podman exec $ctrname touch /uh-oh

    _check_health $ctrname "First failure" "
Status           | \"healthy\"
FailingStreak    | [123]
Log[-1].ExitCode | 1
Log[-1].Output   | \"Uh-oh on stdout!\\\nUh-oh on stderr!\\\n\"
" "$current_time" "healthy"

    # Capture time before systemctl checks so we don't miss the "unhealthy"
    # event that may fire during those checks (health-interval is only 1s).
    current_time=$(date --iso-8601=ns)

    # Check that we now we do have valid podman units with this
    # name so that the leak check below does not turn into a NOP without noticing.
    run -0 systemctl list-units
    cidmatch=$(grep "$cid" <<<"$output")
    echo "$cidmatch"
    assert "$cidmatch" =~ " $cid-[0-9a-f]+\.timer  *.*/podman .* healthcheck run --ignore-result $cid" \
           "Healthcheck systemd unit exists"

    # Check that the right service option is applied so we don't hit the systemd restart limit.
    # Even though the code sets StartLimitIntervalSec the systemd command prints StartLimitInterval*U*Sec
    # Use show --all otherwise the glob might not match the already inactive transient unit.
    run -0 systemctl show --all "$cid-*.service"
    assert "$output" =~ "StartLimitIntervalUSec=0" "The hc service has the right interval set"

    # After three successive failures, container should no longer be healthy
    _check_health $ctrname "Four or more failures" "
Status           | \"unhealthy\"
FailingStreak    | [3456]
Log[-1].ExitCode | 1
Log[-1].Output   | \"Uh-oh on stdout!\\\nUh-oh on stderr!\\\n\"
" "$current_time" "unhealthy"

    # now the on-failure should kick in and kill the container
    run_podman wait $ctrname

    # Clean up
    run_podman rm -t 0 -f $ctrname

    # Important check for https://github.com/containers/podman/issues/22884
    # We never should leak the unit files, healthcheck uses the cid in name so just grep that.
    # (Ignore .scope units, those are conmon and can linger for 5 minutes)
    # (Ignore .mount, too. They are created/removed by systemd based on the actual real mounts
    #  on the host and that is async and might be slow enough in CI to cause failures.)
    run -0 systemctl list-units --quiet "*$cid*"
    except_scope_mount=$(grep -vF ".scope " <<<"$output" | { grep -vF ".mount" || true; } )
    assert "$except_scope_mount" == "" "Healthcheck systemd unit cleanup: no units leaked"
}

@test "podman healthcheck - restart cleans up old state" {
    ctr="c-h-$(safename)"

    run_podman run -d --name $ctr                  \
           --health-cmd /home/podman/healthcheck   \
           --health-retries=3                      \
           --health-interval=disable               \
           $IMAGE /home/podman/pause

    run_podman container inspect $ctr --format "{{.State.Healthcheck.FailingStreak}}"
    is "$output" "0" "Failing streak of fresh container should be 0"

    # Get the healthcheck to fail
    run_podman exec $ctr touch /uh-oh-only-once
    run_podman 1 healthcheck run $ctr
    is "$output" "unhealthy" "output from 'podman healthcheck run'"
    run_podman container inspect $ctr --format "{{.State.Healthcheck.FailingStreak}}"
    is "$output" "1" "Failing streak after one failed healthcheck should be 1"

    run_podman container restart $ctr
    run_podman container inspect $ctr --format "{{.State.Healthcheck.FailingStreak}}"
    is "$output" "0" "Failing streak of restarted container should be 0 again"

    run_podman rm -f -t0 $ctr
}

@test "podman wait --condition={healthy,unhealthy}" {
    ctr="c-h-$(safename)"

    wait_file="$PODMAN_TMPDIR/$(random_string).wait_for_me"

    for condition in healthy unhealthy;do
        rm -f $wait_file
        run_podman run -d --name $ctr                  \
               --health-cmd /home/podman/healthcheck   \
               --health-retries=1                      \
               --health-interval=disable               \
               $IMAGE /home/podman/pause
        if [[ $condition == "unhealthy" ]];then
            # create the uh-oh file to let the health check fail
            run_podman exec $ctr touch /uh-oh
        fi

        # Wait for the container in the background and create the $wait_file to
        # signal the specified wait condition was met.
        (timeout --foreground -v --kill=5 10 "${PODMAN_CMD[@]}" wait --condition=$condition $ctr && touch $wait_file) &

        # Sleep 1 second to make sure above commands are running
        sleep 1
        if [[ -f $wait_file ]]; then
            die "the wait file should only be created after the container turned healthy"
        fi

        if [[ $condition == "healthy" ]];then
            run_podman healthcheck run $ctr
        else
            run_podman 1 healthcheck run $ctr
        fi
        wait_for_file $wait_file
        run_podman rm -f -t0 $ctr
    done
}

@test "podman healthcheck --health-on-failure" {
    run_podman 125 create --health-on-failure=kill $IMAGE
    is "$output" "Error: cannot set on-failure action to kill without a health check"

    ctr="c-h-$(safename)"

    for policy in none kill restart stop;do
        uhoh=/uh-oh
        if [[ $policy != "none" ]];then
            # only fail the first run
            uhoh=/uh-oh-only-once
        fi

        # Run healthcheck image.
        run_podman run -d --name $ctr                 \
               --health-cmd /home/podman/healthcheck  \
               --health-retries=1                     \
               --health-on-failure=$policy            \
               --health-interval=disable              \
               $IMAGE /home/podman/pause

        # healthcheck should succeed
        run_podman healthcheck run $ctr

        # Now cause the healthcheck to fail
        run_podman exec $ctr touch $uhoh

        # healthcheck should now fail, with exit status 1 and 'unhealthy' output
        run_podman 1 healthcheck run $ctr
        is "$output" "unhealthy" "output from 'podman healthcheck run' (policy: $policy)"

        if [[ $policy == "restart" ]];then
           # Make sure the container transitions back to running
           run_podman wait --condition=running $ctr
           run_podman inspect $ctr --format "{{.RestartCount}}"
           assert "${#lines[@]}" != 0 "Container has been restarted at least once"
           run_podman container inspect $ctr --format "{{.State.Healthcheck.FailingStreak}}"
           is "$output" "0" "Failing streak of restarted container should be 0 again"
           run_podman healthcheck run $ctr
        elif [[ $policy == "none" ]];then
            run_podman inspect $ctr --format "{{.State.Status}} {{.Config.HealthcheckOnFailureAction}}"
            # Container is still running and health check still broken
            is "$output" "running $policy" "container continued running"
            run_podman 1 healthcheck run $ctr
            is "$output" "unhealthy" "output from 'podman healthcheck run' (policy: $policy)"
        else
            run_podman inspect $ctr --format "{{.State.Status}} {{.Config.HealthcheckOnFailureAction}}"
            # kill and stop yield the container into a non-running state
            is "$output" ".* $policy" "container was stopped/killed (policy: $policy)"
            assert "$output" != "running $policy"
            # also make sure that it's not stuck in the stopping state
            assert "$output" != "stopping $policy"
        fi

        run_podman rm -f -t0 $ctr
    done
}

@test "podman healthcheck --health-on-failure with interval" {
    ctr="c-h-$(safename)"

    for policy in stop kill restart ;do
        t0=$(date --iso-8601=seconds)
        run_podman run -d --name $ctr      \
               --health-cmd /bin/false     \
               --health-retries=1          \
               --health-on-failure=$policy \
               --health-interval=1s        \
               $IMAGE top

        if [[ $policy == "restart" ]];then
            # Sleeping for 2 seconds makes the test much faster than using
            # podman-wait which would compete with the container getting
            # restarted.
            sleep 2
            # Make sure the container transitions back to running
            run_podman wait --condition=running $ctr
            run_podman inspect $ctr --format "{{.RestartCount}}"
            assert "${#lines[@]}" != 0 "Container has been restarted at least once"
        else
            # kill and stop yield the container into a non-running state
            run_podman wait $ctr
            run_podman inspect $ctr --format "{{.State.Status}} {{.Config.HealthcheckOnFailureAction}}"
            is "$output" ".* $policy" "container was stopped/killed (policy: $policy)"
            assert "$output" != "running $policy"
            # also make sure that it's not stuck in the stopping state
            assert "$output" != "stopping $policy"
        fi

        run_podman rm -f -t0 $ctr
    done
}

function _create_container_with_health_log_settings {
    local ctrname="$1"
    local msg="$2"
    local format="$3"
    local flag="$4"
    local expect="$5"
    local expect_msg="$6"

    run_podman run -d --name $ctrname   \
               --health-cmd "echo $msg" \
               $flag                    \
               $IMAGE /home/podman/pause
    cid="$output"

    run_podman inspect $ctrname --format $format
    is "$output" "$expect" "$expect_msg"

    output=$cid
}

function _check_health_log {
    local ctrname="$1"
    local expect_msg="$2"
    local comparison=$3
    local expect_count="$4"

    run_podman inspect $ctrname --format "{{.State.Health.Log}}"
    count=$(grep -co "$expect_msg" <<< "$output")
    assert "$count" $comparison $expect_count "Number of matching health log messages"
}

@test "podman healthcheck --health-max-log-count values" {
    # flag                    | expected value | op   | log count
    test="
                              | 5              | -eq  | 5
    --health-max-log-count 0  | 0              | -ge  | 11
    --health-max-log-count=0  | 0              | -ge  | 11
    --health-max-log-count 10 | 10             | -eq  | 10
    --health-max-log-count=10 | 10             | -eq  | 10
    "

    while read flag value op logs_count ; do
        local msg="healthmsg-$(random_string)"
        local ctrname="c-h-$(safename)"
        _create_container_with_health_log_settings $ctrname $msg "{{.Config.HealthMaxLogCount}}" $flag $value "HealthMaxLogCount"

        for i in $(seq 1 $((logs_count + 5)));
        do
            run_podman healthcheck run $ctrname
            is "$output" "" "unexpected output from podman healthcheck run (pass $i)"
        done

        _check_health_log $ctrname $msg $op $logs_count

        run_podman rm -t 0 -f $ctrname
    done < <(parse_table "$tests")
}

@test "podman healthcheck --health-max-log-size values" {
    local s=$(printf "healthmsg-%1000s")
    local long_msg=${s// /$(random_string)}

    # flag                    | expected value | exp_msg
    test="
                              | 500            | ${long_msg:0:500}}]\$
    --health-max-log-size 0   | 0              | $long_msg}]\$
    --health-max-log-size=0   | 0              | $long_msg}]\$
    --health-max-log-size 10  | 10             | ${long_msg:0:10}}]\$
    --health-max-log-size=10  | 10             | ${long_msg:0:10}}]\$
    "

    while read flag value exp_msg ; do
        local ctrname="c-h-$(safename)"
        _create_container_with_health_log_settings $ctrname $long_msg "{{.Config.HealthMaxLogSize}}" $flag $value "HealthMaxLogSize"

        run_podman healthcheck run $ctrname
        is "$output" "" "output from 'podman healthcheck run'"

        _check_health_log $ctrname $exp_msg -eq 1

        run_podman rm -t 0 -f $ctrname
    done < <(parse_table "$tests")
}

@test "podman healthcheck --health-log-destination file" {
    local TMP_DIR_HEALTHCHECK="$PODMAN_TMPDIR/healthcheck"
    mkdir $TMP_DIR_HEALTHCHECK
    local ctrname="c-h-$(safename)"
    local msg="healthmsg-$(random_string)"
    _create_container_with_health_log_settings $ctrname $msg "{{.Config.HealthLogDestination}}" "--health-log-destination $TMP_DIR_HEALTHCHECK" "$TMP_DIR_HEALTHCHECK" "HealthLogDestination"
    cid="$output"

    run_podman healthcheck run $ctrname
    is "$output" "" "output from 'podman healthcheck run'"

    healthcheck_log_path="${TMP_DIR_HEALTHCHECK}/${cid}-healthcheck.log"
    # The healthcheck is triggered by the podman when the container is started, but its execution depends on systemd.
    # And since `run_podman healthcheck run` is also run manually, it will result in two runs.
    count=$(grep -co "$msg" $healthcheck_log_path)
    assert "$count" -ge 1 "Number of matching health log messages"

    run_podman rm -t 0 -f $ctrname
}


@test "podman healthcheck - corrupted log file is handled gracefully" {
    local TMP_DIR_HEALTHCHECK="$PODMAN_TMPDIR/healthcheck"
    mkdir $TMP_DIR_HEALTHCHECK
    local ctrname="c-h-$(safename)"
    local msg="healthmsg-$(random_string)"
    _create_container_with_health_log_settings $ctrname $msg "{{.Config.HealthLogDestination}}" "--health-log-destination $TMP_DIR_HEALTHCHECK" "$TMP_DIR_HEALTHCHECK" "HealthLogDestination"
    cid="$output"

    # First make sure there is an uncorrupted log.
    run_podman healthcheck run $ctrname
    is "$output" "" "output from 'podman healthcheck run'"

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
    is "$output" "" "no warnings after log recovery"

    run_podman rm -t 0 -f $ctrname
}

@test "podman healthcheck --health-log-destination journal" {
    skip_if_remote "We cannot read journalctl over remote."

    # We can't use journald on RHEL as rootless, either: rhbz#1895105
    skip_if_journald_unavailable

    local ctrname="c-h-$(safename)"
    local msg="healthmsg-$(random_string)"
    _create_container_with_health_log_settings $ctrname $msg "{{.Config.HealthLogDestination}}" "--health-log-destination events_logger" "events_logger" "HealthLogDestination"
    cid="$output"

    run_podman healthcheck run $ctrname
    is "$output" "" "output from 'podman healthcheck run'"

    cmd="journalctl --output cat --output-fields=PODMAN_HEALTH_LOG PODMAN_ID=$cid"
    echo "$_LOG_PROMPT $cmd"
    run $cmd
    echo "$output"
    assert "$status" -eq 0 "exit status of journalctl"

    # The healthcheck is triggered by the podman when the container is started, but its execution depends on systemd.
    # And since `run_podman healthcheck run` is also run manually, it will result in two runs.
    count=$(grep -co "$msg" <<< "$output")
    assert "$count" -ge 1 "Number of matching health log messages"

    run_podman rm -t 0 -f $ctrname
}

@test "podman healthcheck - stop container when healthcheck runs" {
    ctr="c-h-$(safename)"
    msg="hc-msg-$(random_string)"
    hcStatus=$PODMAN_TMPDIR/hcStatus

    # Disable systemd healthcheck in the background as they mess up the timings of the manual commands.
    export DISABLE_HC_SYSTEMD="true"
    run_podman run -d --name $ctr             \
           --health-cmd "touch /tmp/abc; sleep 20; echo $msg" \
           $IMAGE /home/podman/pause

    timeout --foreground -v --kill=10 60 \
        "${PODMAN_CMD[@]}" healthcheck run $ctr &> $hcStatus &
    hc_pid=$!

    run_podman inspect $ctr --format "{{.State.Status}}"
    assert "$output" == "running" "Container is running"

    ### Flake, sometimes it is possible that the background healthcheck runs so slow that
    # it starts after the podman stop below and then fails with
    # "can only create exec sessions on running containers: container state improper".
    # To fix this we wait for a file th healthcheck creates right away to know it is running.
    timeout=5
    while :; do
        run_podman '?' exec $ctr cat /tmp/abc
        if [[ "$status" -eq 0 ]]; then
                break
        fi

        timeout=$((timeout - 1))
        if [[ $timeout -eq 0 ]]; then
            die "timed out waiting for healthcheck to run and create test file"
        fi
        sleep 1
    done

    run_podman stop $ctr

    # Wait for background healthcheck to finish and make sure the exit status is 1
    rc=0
    wait -n $hc_pid || rc=$?
    cat $hcStatus # just as debug in case the exit code check fails
    assert $rc -eq 1 "exit status check of healthcheck command"
    assert $(< $hcStatus) == "stopped" "Health status"

    run_podman inspect $ctr --format "{{.State.Status}}--{{.State.Health.Status}}--{{.State.Health.FailingStreak}}"
    assert "$output" == "exited--stopped--0" "Container is stopped -- Health status -- failing streak"

    run_podman inspect $ctr --format "{{.State.Health.Log}}"
    assert "$output" !~ "$msg" "Health log message not found"

    run_podman rm -f -t0 $ctr
}

@test "podman healthcheck --ignore-result" {
    ctr="c-h-$(safename)"

    run_podman run -d --name $ctr                  \
           --health-cmd /home/podman/healthcheck   \
           --health-retries=1                      \
           --health-interval=disable               \
           $IMAGE /home/podman/pause

    # Healthcheck should succeed normally
    run_podman healthcheck run $ctr
    is "$output" "" "output from 'podman healthcheck run'"

    # Now make the healthcheck fail
    run_podman exec $ctr touch /uh-oh
    run_podman 1 healthcheck run $ctr
    is "$output" "unhealthy" "output from 'podman healthcheck run' without --ignore-result"

    # With --ignore-result, we should still get "unhealthy" but exit 0
    run_podman healthcheck run --ignore-result $ctr
    is "$output" "unhealthy" "output from 'podman healthcheck run --ignore-result'"

    # It should NOT suppress fatal errors however (e.g., nonexistent container)
    run_podman 125 healthcheck run --ignore-result "nonexistent-$(safename)"
    assert "$output" =~ "no such container"

    run_podman rm -f -t0 $ctr
}

# https://github.com/containers/podman/issues/25034
@test "podman healthcheck - start errors" {
    skip_if_remote '$PATH overwrite not working via remote'
    ctr1="c1-h-$(safename)"
    ctr2="c2-h-$(safename)"

    local systemd_run="$PODMAN_TMPDIR/systemd-run"
    touch $systemd_run
    chmod +x $systemd_run

    # Set custom PATH to force our stub to be called instead of the real systemd-run.
    PATH="$PODMAN_TMPDIR:$PATH" run_podman 126 run -d --name $ctr1 \
           --health-cmd "true" $IMAGE /home/podman/pause
    assert "$output" =~ "create healthcheck: failed to execute systemd-run: fork/exec $systemd_run: exec format error" "error on invalid systemd-run"

    local systemd_run="$PODMAN_TMPDIR/systemd-run"
    cat > $systemd_run <<EOF
#!/bin/bash
echo stdout
echo stderr >&2
exit 2
EOF
    PATH="$PODMAN_TMPDIR:$PATH" run_podman 126 run -d --name $ctr2 \
           --health-cmd "true" $IMAGE /home/podman/pause
    assert "$output" =~ "create healthcheck: systemd-run failed: exit status 2: output: stdout
stderr" "systemd-run error message"

    run_podman rm -f -t0 $ctr1 $ctr2
}

# vim: filetype=sh
