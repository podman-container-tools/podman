#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman system renumber
#

load helpers

function setup() {
    basic_setup

    skip_if_remote "podman system renumber is not available remote"
}

@test "podman system renumber - Basic test with a volume" {
    run_podman volume create test
    assert "$output" == "test" "podman volume create output"
    run_podman system renumber
    assert "$output" == "" "podman system renumber output"
    run_podman volume rm test
    assert "$output" == "test" "podman volume rm output"
}

# vim: filetype=sh
