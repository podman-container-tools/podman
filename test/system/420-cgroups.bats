#!/usr/bin/env bats   -*- bats -*-
#
# cgroups-related tests
#

load helpers

# bats test_tags=ci:parallel
@test "podman run, preserves initial --cgroup-manager" {
    skip_if_remote "podman-remote does not support --cgroup-manager"

    # Find out our default cgroup manager, and from that, get the non-default
    run_podman info --format '{{.Host.CgroupManager}}'
    case "$output" in
        systemd)  other="cgroupfs" ;;
        cgroupfs) other="systemd"  ;;
        *)        die "Unknown CgroupManager '$output'" ;;
    esac

    run_podman --cgroup-manager=$other run --name myc $IMAGE true
    assert "$output" = "" "run true, with cgroup-manager=$other, is silent"

    run_podman container inspect --format '{{.HostConfig.CgroupManager}}' myc
    is "$output" "$other" "podman preserved .HostConfig.CgroupManager"

    if is_rootless && test $other = cgroupfs ; then
        run_podman container inspect --format '{{.HostConfig.CgroupParent}}' myc
        is "$output" "" "podman didn't set .HostConfig.CgroupParent for cgroupfs and rootless"
    fi

    # Restart the container, without --cgroup-manager option (ie use default)
    # Prior to #7970, this would fail with an OCI runtime error
    run_podman start -a myc
    assert "$output" = "" "restarted container emits no output"

    run_podman rm myc
}

# bats test_tags=ci:parallel
@test "podman run --cgroups=disabled keeps the current cgroup" {
    skip_if_remote "podman-remote does not support --cgroups=disabled"
    runtime=$(podman_runtime)
    if [[ $runtime != "crun" ]]; then
        skip "runtime is $runtime; --cgroups=disabled requires crun"
    fi

    current_cgroup=$(cat /proc/self/cgroup)

    # --cgroupns=host is required to have full visibility of the cgroup path inside the container
    run_podman run --cgroups=disabled --cgroupns=host --rm $IMAGE cat /proc/self/cgroup
    is "$output" $current_cgroup "--cgroups=disabled must not change the current cgroup"

    ctr1="c1-$(safename)"
    ctr2="c2-$(safename)"

    # verify that "podman stats --all" works when there is a container with --cgroups=disabled
    run_podman run --cgroups=disabled --name $ctr1 -d $IMAGE top
    run_podman run --name $ctr2 -d $IMAGE top

    run_podman stats -a --no-stream --no-reset
    assert "$output" !~ "$ctr1" "ctr1 not in stats output"
    assert "$output" =~ "$ctr2" "ctr2 in stats output"

    run_podman rm -f -t 0 $ctr1 $ctr2
}

# vim: filetype=sh
