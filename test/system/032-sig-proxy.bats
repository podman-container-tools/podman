#!/usr/bin/env bats

load helpers
load helpers.sig-proxy

# Each of the tests below does some setup, then invokes the helper from helpers.sig-proxy.bash.
@test "podman sigproxy test: run" {
    # We're forced to use $PODMAN because run_podman cannot be backgrounded
    "${PODMAN_CMD[@]}" run -i --name c_run $IMAGE sh -c "$SLEEPLOOP" &
    local kidpid=$!

    _test_sigproxy c_run $kidpid
}

@test "podman sigproxy test: start" {
    run_podman create --name c_start $IMAGE sh -c "$SLEEPLOOP"

    # See above comments regarding $PODMAN and backgrounding
    "${PODMAN_CMD[@]}" start --attach c_start &
    local kidpid=$!

    _test_sigproxy c_start $kidpid
}

@test "podman sigproxy test: attach" {
    run_podman run -d --name c_attach $IMAGE sh -c "$SLEEPLOOP"

    # See above comments regarding $PODMAN and backgrounding
    "${PODMAN_CMD[@]}" attach c_attach &
    local kidpid=$!

    _test_sigproxy c_attach $kidpid
}

@test "podman sigproxy test: exec" {
    local cname=c-exec-$(safename)
    run_podman run -d --name $cname $IMAGE top

    # sleep 97 is spawned by the exec'd shell, so both share a process group
    # and signalling it must take down both.
    # See above comments regarding $PODMAN and backgrounding.
    # SIGTERM, not SIGINT: shells ignore SIGINT in background jobs.
    "${PODMAN_CMD[@]}" exec $cname sh -c 'sleep 97 & sleep 98' &
    local kidpid=$!

    # Wait for both to come up
    local timeout=10
    while :;do
          sleep 0.5
          run_podman top $cname args
          if [[ "$output" =~ "sleep 97" ]] && [[ "$output" =~ "sleep 98" ]]; then
              break
          fi
          timeout=$((timeout - 1))
          if [[ $timeout -eq 0 ]]; then
              die "Timed out waiting for exec'd processes to start"
          fi
    done

    kill -TERM $kidpid
    local exec_status=0
    wait $kidpid || exec_status=$?
    # 128 + SIGTERM(15): the exec'd shell died from the forwarded signal,
    # not from some other error.
    is "$exec_status" "143" "podman exec exit status reflects SIGTERM"

    # Neither may outlive the exec. A dead child lingers as "[sleep]", so
    # match full command lines.
    timeout=20
    while :;do
          sleep 0.5
          run_podman top $cname args
          if [[ ! "$output" =~ "sleep 9" ]]; then
              break
          fi
          timeout=$((timeout - 1))
          if [[ $timeout -eq 0 ]]; then
              die "Timed out waiting for exec'd processes to be signalled"
          fi
    done

    run_podman rm -f -t0 $cname
}

# vim: filetype=sh
