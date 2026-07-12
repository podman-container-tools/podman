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

# vim: filetype=sh
