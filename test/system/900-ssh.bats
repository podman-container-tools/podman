#!/usr/bin/env bats
#
# Tests for podman system connection using native ssh
#

load helpers
load helpers.network

function setup() {
    is_remote || skip "only applicable on podman-remote"

    basic_setup
}

function teardown() {
    run_podman system connection rm testing

    basic_teardown
}

@test "podman --ssh test" {
    type -P ssh || skip "no ssh binary available on host"

    skip "FIXME: this is not an actual test of anything"

    # FIXME: original code used misleading variable name "notme", even though
    # the target user was always the same.
    test -n "$PODMAN_ROOTLESS_USER" || skip "\$PODMAN_ROOTLESS_USER is undefined"

    rootless_uid=$(id -u $PODMAN_ROOTLESS_USER)

    # FIXME FIXME FIXME: someone needs to add a comment here explaining what
    # this is supposed to test
    run_podman 125 --ssh=native system connection add testing \
               ssh://${PODMAN_ROOTLESS_USER}@localhost:22/run/user/${rootless_uid}/podman/podman.sock
    # FIXME FIXME FIXME: this is not an acceptable error message
    is "$output" "Error: exit status 255"

    # FIXME FIXME FIXME: it is not clear what the message below means
    # need to figure out how to podman remote test with the new ssh
}
