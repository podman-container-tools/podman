#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman healthcheck with --transient-store
#
#

load helpers
load helpers.systemd


function teardown() {
    run_podman '?' --transient-store rm -t 0 -a -f
    basic_teardown
}

# https://github.com/containers/podman/issues/28483
@test "podman healthcheck --transient-store" {
    skip_if_remote "transient-store is a local option"

    ctr="c-h-$(safename)"

    run_podman run -d --name $ctr --transient-store \
           --health-cmd /home/podman/healthcheck    \
           --health-interval 1s                     \
           --health-retries 3                       \
           $IMAGE /home/podman/pause
    cid="$output"

    # The systemd timer command line must include --transient-store=true so
    # the healthcheck subprocess opens the correct (volatile) store.
    run -0 systemctl list-units
    cidmatch=$(grep "$cid" <<<"$output")
    assert "$cidmatch" =~ "--transient-store=true .* healthcheck run --ignore-result $cid" \
           "Healthcheck systemd unit includes --transient-store=true"

    run_podman --transient-store wait --condition=healthy $ctr

    run_podman --transient-store inspect $ctr \
               --format "{{.State.Health.Status}} {{.State.Health.FailingStreak}}"
    assert "$output" == "healthy 0" "health status and failing streak"

    run_podman --transient-store rm -f -t0 $ctr
}

# vim: filetype=sh
