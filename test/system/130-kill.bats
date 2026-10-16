#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman kill
#

load helpers

# bats test_tags=ci:parallel
@test "podman kill - test signal handling in containers" {
    local cname=c-$(safename)
    local fifo=${PODMAN_TMPDIR}/podman-kill-fifo.$(random_string 10)
    mkfifo $fifo

    # Start a container that will handle all signals by emitting 'got: N'
    local -a signals=(1 2 3 4 5 6 8 10 12 13 14 15 16 20 21 22 23 24 25 26 64)
    "${PODMAN_CMD[@]}" run --name $cname $IMAGE sh -c \
        "for i in ${signals[*]}; do trap \"echo got: \$i\" \$i; done;
        echo READY;
        while ! test -e /stop; do sleep 0.1; done;
        echo DONE" &>$fifo </dev/null &
    podman_run_pid=$!

    # Open the FIFO for reading, and keep it open. This prevents a race
    # condition in which the container can exit (e.g. if for some reason
    # it doesn't handle the signal) and we (this test) try to read from
    # the FIFO. Since there wouldn't be an active writer, the open()
    # would hang forever. With this exec we keep the FD open, allowing
    # 'read -t' to time out and report a useful error.
    exec 5<$fifo

    # First container emits READY when ready; wait for it.
    read -t 60 -u 5 ready
    is "$ready" "READY" "first log message from container"

    # Helper function: send the given signal, verify that it's received.
    kill_and_check() {
        local signal=$1
        local signum=${2:-$1}       # e.g. if signal=HUP, we expect to see '1'

        run_podman kill -s $signal $cname
        read -t 60 -u 5 actual || die "Timed out: no ACK for kill -s $signal"
        is "$actual" "got: $signum" "Signal $signal handled by container"
    }

    # Send signals in random order; make sure each one is received
    for s in $(fmt --width=2 <<< "${signals[*]}" | sort --random-sort);do
        kill_and_check $s
    done

    # Variations: with leading dash; by name, with/without dash or SIG
    kill_and_check -1        1
    kill_and_check -INT      2
    kill_and_check  FPE      8
    kill_and_check -SIGUSR1 10
    kill_and_check  SIGUSR2 12

    # Done. Tell the container to stop, and wait for final DONE.
    # The '-d' is because container exit is racy: the exec process itself
    # could get caught and killed by cleanup, causing this step to exit 137
    run_podman exec -d $cname touch /stop
    read -t 5 -u 5 done || die "Timed out waiting for DONE from container"
    is "$done" "DONE" "final log message from container"

    # Clean up
    run_podman rm -f -t0 $cname
    wait $podman_run_pid || die "wait for podman run failed"
}

# bats test_tags=ci:parallel
@test "podman kill - rejects invalid args" {
    # These errors are thrown by the imported docker/signal.ParseSignal()
    local -a bad_signal_names=(0 SIGBADSIG SIG BADSIG %% ! "''" '""' " ")
    for s in ${bad_signal_names[@]}; do
        # 'nosuchcontainer' is fine: podman should bail before it gets there
        run_podman 125 kill -s $s nosuchcontainer
        is "$output" "Error: invalid signal: $s" "Error from kill -s $s"

        run_podman 125 pod kill -s $s nosuchpod
        is "$output" "Error: invalid signal: $s" "Error from pod kill -s $s"
    done

    # Special case: these too are thrown by docker/signal.ParseSignal(),
    # but the dash sign is stripped by our wrapper in utils, so the
    # error message doesn't include the dash.
    local -a bad_dash_signals=(-0 -SIGBADSIG -SIG -BADSIG -)
    for s in ${bad_dash_signals[@]}; do
        run_podman 125 kill -s $s nosuchcontainer
        is "$output" "Error: invalid signal: ${s##-}" "Error from kill -s $s"
    done

    # This error (signal out of range) is thrown by our wrapper
    local -a bad_signal_nums=(65 -65 96 999 99999999)
    for s in ${bad_signal_nums[@]}; do
        run_podman 125 kill -s $s nosuchcontainer
        is "$output" "Error: valid signals are 1 through 64" \
           "Error from kill -s $s"
    done

    # 'podman create' uses the same parsing code
    run_podman 125 create --stop-signal=99 $IMAGE
    is "$output" "Error: valid signals are 1 through 64" "podman create"
}

# CANNOT BE PARALLELIZED: kill -a
@test "podman kill - print IDs or raw input" {
    # kill -a must print the IDs
    run_podman run --rm -d $IMAGE top
    ctrID="$output"
    run_podman kill -a
    is "$output" "$ctrID"

    # kill $input must print $input
    cname=c-$(safename)
    run_podman run --rm -d --name $cname $IMAGE top
    run_podman kill $cname
    is "$output" $cname
    # Wait for the container to get removed to avoid the leak check from triggering,
    # since it might have already been removed here ignore the exit code check.
    run_podman '?' wait --condition=removing $cname
}

# bats test_tags=ci:parallel
@test "podman kill - concurrent stop" {
    # 14761 - concurrent kill/stop must record the exit code
    cname=c-$(safename)
    run_podman run -d --replace --name=$cname $IMAGE sh -c "trap 'echo Received SIGTERM, ignoring' SIGTERM; echo READY; while :; do sleep 0.2; done"
    "${PODMAN_CMD[@]}" stop -t 5 $cname &
    run_podman kill $cname
    run_podman wait $cname
    run_podman rm -f $cname
}

# bats test_tags=ci:parallel
@test "podman wait - exit codes" {
    cname=c-$(safename)
    run_podman create --name=$cname $IMAGE /no/such/command
    run_podman container inspect  --format "{{.State.StoppedByUser}}" $cname
    is "$output" "false" "container not marked to be stopped by a user"
    # Container never ran -> default condition (=stopped) returns 0 immediately.
    # This matches Docker's "not-running" semantic: a container that has never
    # run is, by definition, not running. Callers that want to block until the
    # container has actually run and exited must use --condition=next-exit.
    run_podman wait $cname
    is "$output" "0" "wait on never-started container returns 0 (documented semantic)"
    # Container did not start successfully -> exit code != 0
    run_podman 125 start $cname
    # The container that failed to start has no recorded exit code, so wait
    # still returns 0 for the default condition.
    run_podman wait $cname
    is "$output" "0" "wait still returns 0 after a failed start"
    run_podman rm $cname
}

# bats test_tags=ci:parallel
@test "podman wait --condition=next-exit blocks until actual exit" {
    cname=c-$(safename)
    run_podman create --name=$cname $IMAGE sh -c "exit 7"

    # Launch wait in the background; it must NOT return immediately for a
    # never-started container when --condition=next-exit is used.
    timeout --foreground -v --kill=10 30 \
        "${PODMAN_CMD[@]}" wait --condition=next-exit $cname > $PODMAN_TMPDIR/wait-output 2>&1 &
    wait_pid=$!

    # Give the wait command time to subscribe to events.
    sleep 1

    # Trigger the exit.
    run_podman start $cname
    run_podman 0 wait $cname

    # Now the backgrounded wait should return with the actual exit code.
    wait $wait_pid
    assert "$(< $PODMAN_TMPDIR/wait-output)" = "7" \
        "--condition=next-exit returned the container's actual exit code"

    run_podman rm $cname
}

# bats test_tags=ci:parallel
@test "podman kill - no restart" {
    ctr=c-$(safename)
    run_podman run -d --restart=always --name=$ctr $IMAGE \
        sh -c "trap 'exit 42' SIGTERM; echo READY; while :; do sleep 0.2; done"
    run_podman container inspect  --format "{{.State.Status}}" $ctr
    is "$output" "running" "make sure container is running"
    # Send SIGTERM and make sure the container exits.
    run_podman kill -s=TERM $ctr
    run_podman wait $ctr
    is "$output" "42" "container exits with 42 on receiving SIGTERM"
    run_podman container inspect  --format "{{.State.StoppedByUser}}" $ctr
    is "$output" "true" "container is marked to be stopped by a user"
    run_podman rm $ctr
}

# vim: filetype=sh
