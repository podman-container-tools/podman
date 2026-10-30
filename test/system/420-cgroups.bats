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
@test "podman run rootless cgroupfs creates a cgroup for the container" {
    skip_if_remote "podman-remote does not support --cgroup-manager"
    skip_if_not_rootless "rootful cgroupfs always sets a cgroup parent"

    ctrname="c-$(safename)"
    run_podman --cgroup-manager=cgroupfs run -d --name $ctrname $IMAGE top
    cid="$output"

    run_podman inspect --format '{{.State.Pid}}' $ctrname
    cgroup=/sys/fs/cgroup$(sed -n 's/^0:://p' /proc/$output/cgroup)
    assert "$cgroup" =~ "/libpod-$cid\$" "container runs in the cgroup created for it"

    # The cgroup is deleted with the container, and created again on start
    run_podman stop -t 0 $ctrname
    test ! -e $cgroup || die "$cgroup still exists after stop"

    run_podman start $ctrname
    run_podman inspect --format '{{.State.Pid}}' $ctrname
    cgroup=/sys/fs/cgroup$(sed -n 's/^0:://p' /proc/$output/cgroup)
    assert "$cgroup" =~ "/libpod-$cid\$" "restarted container runs in the cgroup created for it"

    run_podman rm -f -t 0 $ctrname
    test ! -e $cgroup || die "$cgroup still exists after rm"
}

# bats test_tags=ci:parallel
@test "podman run rootless cgroupfs fails if the container cgroup cannot be created" {
    skip_if_remote "podman-remote does not support --cgroup-manager"
    skip_if_not_rootless "rootful cgroupfs always sets a cgroup parent"

    ctrname="c-$(safename)"
    # Make cgroupfs read-only in a private mount namespace, so that the
    # container cgroup cannot be created.  The container must not run without it.
    run_podman 126 unshare unshare --mount sh -c "
        mount --bind /sys/fs/cgroup /sys/fs/cgroup &&
        mount -o remount,bind,ro /sys/fs/cgroup &&
        $PODMAN $_PODMAN_TEST_OPTS --cgroup-manager=cgroupfs run --rm --name $ctrname $IMAGE echo $ctrname-started"
    assert "$output" =~ "creating cgroup" "error mentions the cgroup"
    assert "$output" !~ "$ctrname-started" "container did not run"

    run_podman 1 container exists $ctrname
}

# bats test_tags=ci:parallel
@test "podman run rootless cgroupfs removes the cgroup if the container cannot be created" {
    skip_if_remote "podman-remote does not support --runtime"
    skip_if_not_rootless "rootful cgroupfs always sets a cgroup parent"

    run_podman info --format '{{.Host.OCIRuntime.Path}}'
    runtime="$output"

    # OCI runtime that fails to create the container, after recording
    # the cgroup that podman created for it.
    marker=$PODMAN_TMPDIR/cgroup
    fake_runtime=$PODMAN_TMPDIR/fake-runtime
    cat >$fake_runtime <<EOF
#!/bin/sh
for arg; do id=\$arg; done
for arg; do
    if [ "\$arg" = create ]; then
        find /sys/fs/cgroup -type d -name "libpod-\$id" >$marker 2>/dev/null
        echo "fake runtime cannot create the container" >&2
        exit 1
    fi
done
exec $runtime "\$@"
EOF
    chmod +x $fake_runtime

    ctrname="c-$(safename)"
    run_podman 126 --cgroup-manager=cgroupfs run --runtime $fake_runtime --name $ctrname $IMAGE true
    assert "$output" =~ "fake runtime cannot create the container"

    cgroup=$(< $marker)
    assert "$cgroup" =~ "/libpod-" "the cgroup exists when the OCI runtime creates the container"
    test ! -e $cgroup || die "$cgroup was not removed"

    run_podman rm $ctrname
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
