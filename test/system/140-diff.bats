#!/usr/bin/env bats   -*- bats -*-
#
# Tests for podman diff
#

load helpers

@test "podman diff" {
    n=$(random_string 10)          # container name
    rand_file=$(random_string 10)
    run_podman run --name $n $IMAGE sh -c "touch /$rand_file;rm /etc/services"

    # If running local, test `-l` (latest) option. This can't work with remote.
    if ! is_remote; then
        n=-l
    fi

    run_podman diff --format json $n

    # Expected results for each type of diff
    declare -A expect=(
        [added]="/$rand_file"
        [changed]="/etc"
        [deleted]="/etc/services"
    )

    for field in ${!expect[@]}; do
        # ARGH! The /sys/fs kludgery is for RHEL8 rootless, which mumble mumble
        # does some sort of magic muckery with /sys - I think the relevant
        # PR is https://github.com/containers/podman/pull/8561
        # Anyhow, without the egrep below, this test fails about 50% of the
        # time on rootless RHEL8. (No, I don't know why it's not 100%).
        result=$(jq -r -c ".${field}[]" <<<"$output" | grep -E -v '^/sys/fs')
        is "$result" "${expect[$field]}" "$field"
    done

    run_podman rm $n
}

@test "podman diff with buildah container " {
    rand_file=$(random_string 10)
    buildah from --name buildahctr $IMAGE
    buildah run buildahctr sh -c "touch /$rand_file;rm /etc/services"

    run_podman diff --format json buildahctr

    # Expected results for each type of diff
    declare -A expect=(
        [added]="/$rand_file"
        [changed]="/etc"
        [deleted]="/etc/services"
    )

    for field in ${!expect[@]}; do
        result=$(jq -r -c ".${field}[]" <<<"$output")
        is "$result" "${expect[$field]}" "$field"
    done

    buildah rm buildahctr
}

# vim: filetype=sh
