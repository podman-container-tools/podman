#!/usr/bin/env bats

load helpers

# Very simple test
# bats test_tags=ci:parallel
@test "podman stop - basic test" {
    run_podman run -d $IMAGE sleep 60
    cid="$output"

    # Run 'stop'. Time how long it takes. If local, require a warning.
    local plusw="+w"
    if is_remote; then
        plusw=
    fi
    t0=$SECONDS
    run_podman 0$plusw stop $cid
    t1=$SECONDS
    if [[ -n "$plusw" ]]; then
        require_warning "StopSignal SIGTERM failed to stop container .*, resorting to SIGKILL"
    fi

    # Confirm that container is stopped. Podman-remote unfortunately
    # cannot tell the difference between "stopped" and "exited", and
    # spits them out interchangeably, so we need to recognize either.
    run_podman inspect --format '{{.State.Status}} {{.State.ExitCode}}' $cid
    is "$output" "\\(stopped\|exited\\) \+137" \
       "Status and exit code of stopped container"

    # The initial SIGTERM is ignored, so this operation should take
    # exactly 10 seconds. Give it some leeway.
    delta_t=$(( $t1 - $t0 ))
    assert $delta_t -gt  8 "podman stop: ran too quickly!"
    assert $delta_t -le 18 "podman stop: took too long"

    run_podman rm $cid
}

# #9051 : podman stop --all was not working with podman-remote
# DO NOT PARALLELIZE! (due to stop -a)
@test "podman stop --all" {
    # Start three containers, create (without running) a fourth
    run_podman run -d --name c1 $IMAGE sleep 20
    cid1="$output"
    run_podman run -d --name c2 $IMAGE sleep 40
    cid2="$output"
    run_podman run -d --name c3 $IMAGE sleep 60
    cid3="$output"
    run_podman create --name c4 $IMAGE sleep 80
    cid4="$output"

    # podman ps (without -a) should show the three running containers
    run_podman ps --sort names --format '{{.Names}}--{{.Status}}'
    is "${#lines[*]}" "3"        "podman ps shows exactly three containers"
    is "${lines[0]}" "c1--Up.*"  "podman ps shows running container (1)"
    is "${lines[1]}" "c2--Up.*"  "podman ps shows running container (2)"
    is "${lines[2]}" "c3--Up.*"  "podman ps shows running container (3)"

    # Stop -a. Local podman issues a warning, check for it.
    local plusw="+w"
    if is_remote; then
        plusw=
    fi
    run_podman 0$plusw stop -a -t 1
    if [[ -n "$plusw" ]]; then
        require_warning "StopSignal SIGTERM failed to stop container .*, resorting to SIGKILL"
    fi

    # Now podman ps (without -a) should show nothing.
    run_podman ps --format '{{.Names}}'
    is "$output" "" "podman ps, after stop -a, shows no running containers"

    # ...but with -a, containers are shown
    run_podman ps -a --sort names --format '{{.Names}}--{{.Status}}'
    is "${#lines[*]}" "4"        "podman ps -a shows exactly four containers"
    is "${lines[0]}" "c1--Exited.*"  "ps -a, first stopped container"
    is "${lines[1]}" "c2--Exited.*"  "ps -a, second stopped container"
    is "${lines[2]}" "c3--Exited.*"  "ps -a, third stopped container"
    is "${lines[3]}" "c4--Created.*" "ps -a, created container (unaffected)"

    run_podman rm $cid1 $cid2 $cid3 $cid4
}

# DO NOT PARALLELIZE! (due to stop -a)
@test "podman stop print IDs or raw input" {
    # stop -a must print the IDs
    run_podman run -d $IMAGE top
    ctrID="$output"
    run_podman stop -t0 --all
    is "$output" "$ctrID"

    # stop $input must print $input
    cname=$(random_string)
    run_podman run -d --name $cname $IMAGE top
    run_podman stop -t0 $cname
    is "$output" $cname

    run_podman rm -t 0 -f $ctrID $cname
}

# #9051 : podman stop --ignore was not working with podman-remote
# bats test_tags=ci:parallel
@test "podman stop --ignore" {
    name=thiscontainerdoesnotexist
    run_podman 125 stop $name
    is "$output" \
       "Error: no container with name or ID \"$name\" found: no such container" \
       "podman stop nonexistent container"

    run_podman stop --ignore $name
    is "$output" "" "podman stop nonexistent container, with --ignore"

    nosuchfile=$PODMAN_TMPDIR/no-such-file
    run_podman 125 stop --cidfile=$nosuchfile
    is "$output" "Error: reading CIDFile: open $nosuchfile: no such file or directory" "podman stop with missing cidfile, without --ignore"

    # create a container to reproduce (#23554)
    run_podman run -d $IMAGE sleep inf
    cid="$output"

    # Important for (#23554) that there is no output here
    run_podman stop --cidfile=$nosuchfile --ignore
    is "$output" "" "podman stop with missing cidfile, with --ignore (empty output)"

    run_podman rm -f -t0 $cid
}


# Regression test for #2472
# bats test_tags=ci:parallel
@test "podman stop - can trap signal" {
    # Because the --time and --timeout options can be wonky, try three
    # different variations of this test.
    for t_opt in '' '--time=9' '--timeout=11'; do
        # Run a simple container that logs output on SIGTERM
        run_podman run -d $IMAGE sh -c \
                   "trap 'echo Received SIGTERM, finishing; exit' SIGTERM; echo READY; while :; do sleep 1; done"
        cid="$output"
        wait_for_ready $cid

        # Run 'stop' against it...
        t0=$SECONDS
        run_podman stop $t_opt $cid
        t1=$SECONDS

        # ...the container should trap the signal, log it, and exit.
        run_podman logs $cid
        is "$output" ".*READY.*Received SIGTERM, finishing" "podman stop $t_opt"

        # Exit code should be 0, because container did its own exit
        run_podman inspect --format '{{.State.ExitCode}}' $cid
        is "$output" "0" "Exit code of stopped container"

        # The 'stop' command should return almost instantaneously
        local howlong=2
        # ...unless running parallel, due to high system load.
        if [[ -n "$PARALLEL_JOBSLOT" ]]; then
            howlong=8
        fi
        delta_t=$(( $t1 - $t0 ))
        assert $delta_t -le $howlong "podman stop: took too long"

        run_podman rm $cid
    done
}

# Regression test for #8501
# bats test_tags=ci:parallel
@test "podman stop - unlock while waiting for timeout" {
    # Test that the container state transitions to "stopping" and that other
    # commands can get the container's lock.  To do that, run a container that
    # ignores SIGTERM such that the Podman would wait 20 seconds for the stop
    # to finish.  This gives us enough time to try some commands and inspect
    # the container's status.

    ctrname="c-stopme-$(safename)"
    run_podman run --name $ctrname -d $IMAGE sh -c \
        "trap 'echo Received SIGTERM, ignoring' SIGTERM; echo READY; while :; do sleep 0.2; done"

    wait_for_ready $ctrname

    local t0=$SECONDS
    # Stop the container, but do so in the background so we can inspect
    # the container status while it's stopping. Use $PODMAN because we
    # don't want the overhead and error checks of run_podman.
    "${PODMAN_CMD[@]}" stop -t 20 $ctrname &

    # Wait for container to acknowledge the signal. We can't use wait_for_output
    # because that aborts if .State.Running != true
    local timeout=10
    while [[ $timeout -gt 0 ]]; do
        run_podman logs $ctrname
        if [[ "$output" =~ "Received SIGTERM, ignoring" ]]; then
            break
        fi
        timeout=$((timeout - 1))
        assert $timeout -gt 0 "Timed out waiting for container to receive SIGTERM"
        sleep 0.5
    done

    # This command must be able to take the container lock.
    # The container state transitioned to "stopping"
    run_podman inspect --format '{{.State.Status}}' $ctrname
    is "$output" "stopping" "Status of container should be 'stopping'"

    # Time check: make sure we were able to run 'ps' before the container
    # exited. If this takes too long, it means ps had to wait for lock.
    local delta_t=$(( $SECONDS - t0 ))
    assert $delta_t -le 7 "Operations took too long"

    run_podman kill $ctrname
    run_podman wait $ctrname

    # Exit code should be 137 as it was killed
    run_podman inspect --format '{{.State.ExitCode}}' $ctrname
    is "$output" "137" "Exit code of killed container"

    run_podman rm $ctrname
}

# bats test_tags=ci:parallel
@test "podman stop -t 1 Generate warning" {
    skip_if_remote "warning only happens on server side"

    ctrname="c-stopme-$(safename)"
    run_podman run --rm --name $ctrname -d $IMAGE sleep 100

    local plusw="+w"
    if is_remote; then
        plusw=
    fi
    run_podman 0$plusw stop -t 1 $ctrname
    if [[ -n "$plusw" ]]; then
        require_warning ".*StopSignal SIGTERM failed to stop container $ctrname in 1 seconds, resorting to SIGKILL"
    fi
}

# bats test_tags=ci:parallel
@test "podman stop --noout" {
    ctrname="c-$(safename)"
    run_podman run --rm --name $ctrname -d $IMAGE top
    run_podman --noout stop -t 0 $ctrname
    is "$output" "" "output should be empty"
}

# bats test_tags=ci:parallel
@test "podman stop, with --rm container" {
    OCIDir=/run/$(podman_runtime)

    if is_rootless; then
        OCIDir=/run/user/$(id -u)/$(podman_runtime)
    fi

    ctrname="c-$(safename)"
    run_podman run --rm -d --name $ctrname $IMAGE sleep infinity
    local cid="$output"
    run_podman stop -t0 $ctrname

    # Check the OCI runtime directory has removed.
    is "$(ls $OCIDir | grep $cid)" "" "The OCI runtime directory should have been removed"
}
# vim: filetype=sh
