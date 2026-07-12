#!/usr/bin/env bats   -*- bats -*-
#
# 410-selinux - podman selinux tests
#

load helpers


function check_label() {
    skip_if_no_selinux

    local args="$1"; shift        # command-line args for run

    # FIXME: it'd be nice to specify the command to run, e.g. 'ls -dZ /',
    # but alpine ls (from busybox) doesn't support -Z
    run_podman run --rm $args $IMAGE cat -v /proc/self/attr/current

    # FIXME: on some CI systems, 'run --privileged' emits a spurious
    # warning line about dup devices. Ignore it.
    remove_same_dev_warning
    local context="$output"
    run id -Z
    user=$(secon -u $output)
    role=$(secon -r $output)

    case "$args" in
        # Containers that run automatically without SELinux transitions, run
        # with the current role.
        *--privileged*| *--pid=host* | *--ipc=host* | *"--security-opt label=disable"*)
                is "$context" "$user:$role:.*" "Non SELinux separated containers role should always be the current user and role"
                ;;
        # Containers that are confined or force the spc_t type default
        # to running with the system_r role.
        *)
                is "$context" ".*_u:system_r:.*" "SELinux separated containers role should always be system_r"
                ;;
    esac
    # e.g. system_u:system_r:container_t:s0:c45,c745 -> "container_t"
    type=$(cut -d: -f3 <<<"$context")
    is "$type" "$1" "SELinux type"

    if [ -n "$2" ]; then
        # e.g. from the above example -> "s0:c45,c745"
        range=$(cut -d: -f4,5 <<<"$context")
        is "$range" "$2^@" "SELinux range"
    fi
}


# bats test_tags=ci:parallel
@test "podman selinux: confined container" {
    check_label "" "container_t"
}

# bats test_tags=ci:parallel
@test "podman selinux: container with label=disable" {
    check_label "--security-opt label=disable" "spc_t"
}

# bats test_tags=ci:parallel
@test "podman selinux: privileged container" {
    check_label "--privileged" "spc_t"
}

# bats test_tags=ci:parallel
@test "podman selinux: privileged --userns=host container" {
    check_label "--privileged --userns=host" "spc_t"
}

# bats test_tags=ci:parallel
@test "podman selinux: --ipc=host container" {
    check_label "--ipc=host" "spc_t"
}

# bats test_tags=ci:parallel
@test "podman selinux: init container" {
    check_label "--systemd=always" "container_init_t"
}

# bats test_tags=ci:parallel
@test "podman selinux: init container with --security-opt type" {
    check_label "--systemd=always --security-opt=label=type:spc_t" "spc_t"
}

# bats test_tags=ci:parallel
@test "podman selinux: init container with --security-opt level&type" {
    check_label "--systemd=always --security-opt=label=level:s0:c1,c2 --security-opt=label=type:spc_t" "spc_t" "s0:c1,c2"
}

# bats test_tags=ci:parallel
@test "podman selinux: init container with --security-opt level" {
    check_label "--systemd=always --security-opt=label=level:s0:c1,c2" "container_init_t"  "s0:c1,c2"
}

# bats test_tags=ci:parallel
@test "podman selinux: pid=host" {
    # FIXME this test fails when run rootless with runc:
    #   Error: container_linux.go:367: starting container process caused: process_linux.go:495: container init caused: readonly path /proc/asound: operation not permitted: OCI permission denied
    if is_rootless; then
        runtime=$(podman_runtime)
        test "$runtime" == "crun" \
            || skip "runtime is $runtime; this test requires crun"
    fi

    check_label "--pid=host" "spc_t"
}

# bats test_tags=ci:parallel
@test "podman selinux: container with overridden range" {
    check_label "--security-opt label=level:s0:c1,c2" "container_t" "s0:c1,c2"
}

# bats test_tags=ci:parallel
@test "podman selinux: inspect kvm labels" {
    skip_if_no_selinux
    skip_if_remote "runtime flag is not passed over remote"

    ctrname=c-$(safename)
    tmpdir=$PODMAN_TMPDIR/kata-test
    mkdir -p $tmpdir
    KATA=${tmpdir}/kata-runtime
    ln -s /bin/true ${KATA}
    run_podman create --runtime=${KATA} --name $ctrname $IMAGE
    run_podman inspect --format='{{ .ProcessLabel }}' $ctrname
    is "$output" ".*container_kvm_t"

    run_podman rm $ctrname
}

# pr #6752
# bats test_tags=ci:parallel
@test "podman selinux: inspect multiple labels" {
    skip_if_no_selinux

    ctrname=c-$(safename)

    run_podman run -d --name $ctrname \
               --security-opt seccomp=unconfined \
               --security-opt label=type:spc_t \
               --security-opt label=level:s0 \
               $IMAGE top
    run_podman inspect --format='{{ .HostConfig.SecurityOpt }}' $ctrname
    is "$output" "[label=type:spc_t,label=level:s0 seccomp=unconfined]" \
      "'podman inspect' preserves all --security-opts"

    run_podman rm -t 0 -f $ctrname
}

# Sharing context between two containers not in a pod
# These tests were piggybacked in with #7902, but are not actually related
# bats test_tags=ci:parallel
@test "podman selinux: shared context in (some) namespaces" {
    skip_if_no_selinux

    if [[ $(podman_runtime) == "runc" ]]; then
        skip "some sort of runc bug, not worth fixing (issue 11784, wontfix)"
    fi

    ctrname1=c1-$(safename)
    ctrname2=c2-$(safename)
    run_podman run -d --name $ctrname1 $IMAGE top
    run_podman exec $ctrname1 cat -v /proc/self/attr/current
    context_c1="$output"

    # --ipc container
    run_podman run --name $ctrname2 --ipc container:$ctrname1 $IMAGE cat -v /proc/self/attr/current
    is "$output" "$context_c1" "new container, run with ipc of existing one "

    # --pid container
    run_podman run --rm --pid container:$ctrname1 $IMAGE cat -v /proc/self/attr/current
    is "$output" "$context_c1" "new container, run with --pid of existing one "

    # net NS: do not share context
    run_podman run --rm --net container:$ctrname1 $IMAGE cat -v /proc/self/attr/current
    assert "$output" != "$context_c1" \
           "run --net : context should != context of running container"

    # The 'ctrname2' above was not run with --rm, so it still exists, and
    # we can't remove the original container until this one is gone.
    run_podman stop -t 0 $ctrname1
    run_podman 125 rm $ctrname1
    is "$output" "Error: container .* has dependent containers"

    # We have to do this in two steps: even if ordered as '$ctrname2 $ctrname1',
    # podman will try the removes in random order, which fails if it
    # tries ctrname1 first.
    run_podman rm $ctrname2
    run_podman rm $ctrname1
}

# pr #7902 - containers in pods should all run under same context
# bats test_tags=ci:parallel
@test "podman selinux: containers in pods share full context" {
    skip_if_no_selinux

    # unique pod name helps when tracking down failure in journal
    local podname=p-$(safename)

    # We don't need a fullblown pause container; avoid pulling the k8s one
    run_podman pod create --name $podname \
               --infra-image $IMAGE \
               --infra-command /home/podman/pause

    # Get baseline
    run_podman run --rm --pod $podname $IMAGE cat -v /proc/self/attr/current
    context_c1="$output"

    # Prior to #7902, the labels (':c123,c456') would be different
    run_podman run --rm --pod $podname $IMAGE cat -v /proc/self/attr/current
    is "$output" "$context_c1" "SELinux context of 2nd container matches 1st"

    # What the heck. Try a third time just for extra confidence
    run_podman run --rm --pod $podname $IMAGE cat -v /proc/self/attr/current
    is "$output" "$context_c1" "SELinux context of 3rd container matches 1st"

    run_podman pod rm -f -t0 $podname
}

# more pr #7902
# bats test_tags=ci:parallel
@test "podman selinux: containers in --no-infra pods do not share context" {
    skip_if_no_selinux

    # unique pod name helps when tracking down failure in journal
    local podname=p-$(safename)

    # We don't need a fullblown pause container; avoid pulling the k8s one
    run_podman pod create --name $podname --infra=false

    # Get baseline
    run_podman run --rm --pod $podname $IMAGE cat -v /proc/self/attr/current
    context_c1="$output"

    # Even after #7902, labels (':c123,c456') should be different
    run_podman run --rm --pod $podname $IMAGE cat -v /proc/self/attr/current
    assert "$output" != "$context_c1" \
           "context of two separate containers should be different"

    run_podman pod rm -f -t0 $podname
}

# #8946 - better diagnostics for nonexistent attributes
# bats test_tags=ci:parallel
@test "podman with nonexistent labels" {
    skip_if_no_selinux

    # runc and crun emit different diagnostics
    runtime=$(podman_runtime)
    case "$runtime" in
        # crun 0.20.1 changes the error message
        #   from /proc/thread-self/attr/exec`: .* unable to assign
        #   to   /proc/self/attr/keycreate`: .* unable to process
        crun) expect="\`/proc/.*\`: OCI runtime error: unable to \(assign\|process\) security attribute" ;;
        # runc 1.1 changed the error message because of new selinux pkg that uses standard os.PathError, see
        # https://github.com/opencontainers/selinux/pull/148/commits/a5dc47f74c56922d58ead05d1fdcc5f7f52d5f4e
        #   from failed to set /proc/self/attr/keycreate on procfs
        #   to   write /proc/self/attr/keycreate: invalid argument
        # runc 1.3.3 (temporarily?) changed the error message because of the fix to CVE-2025-52881, see
        # https://github.com/opencontainers/runc/commit/2c5356e73f15b246729d03198bfb2c6c33454099
        #   to   write fsmount:fscontext:proc/self/attr/keycreate: invalid argument
        runc) expect=".*: \(failed\|write\).*proc/self/attr/keycreate.*" ;;
        *)    skip "Unknown runtime '$runtime'";;
    esac

    # The '.*' in the error below is for dealing with podman-remote, which
    # includes "error preparing container <sha> for attach" in output.
    run_podman 126 run --rm --security-opt label=type:foo.bar $IMAGE true
    is "$output" "Error.*: $expect" "podman emits useful diagnostic on failure"
}

@test "podman selinux: check relabel" {
    skip_if_no_selinux

    LABEL="system_u:object_r:tmp_t:s0"
    RELABEL="system_u:object_r:container_file_t:s0"
    tmpdir=$PODMAN_TMPDIR/vol
    mkdir -p $tmpdir
    chcon -vR ${LABEL} $tmpdir
    ls -Z $tmpdir

    run_podman run --rm -v $tmpdir:/test $IMAGE cat /proc/self/attr/current
    run ls -dZ ${tmpdir}
    is "$output" "${LABEL} ${tmpdir}" "No Relabel Correctly"

    run_podman run --rm -v $tmpdir:/test:z --security-opt label=disable $IMAGE cat /proc/self/attr/current
    run ls -dZ $tmpdir
    is "$output" "${RELABEL} $tmpdir" "Privileged Relabel Correctly"

    run_podman run --rm -v $tmpdir:/test:z --privileged $IMAGE cat /proc/self/attr/current
    run ls -dZ $tmpdir
    is "$output" "${RELABEL} $tmpdir" "Privileged Relabel Correctly"

    ctrname=c-$(safename)
    run_podman run --name $ctrname -v $tmpdir:/test:Z $IMAGE cat /proc/self/attr/current
    level=$(secon -l $output)
    run ls -dZ $tmpdir
    is "$output" "system_u:object_r:container_file_t:$level $tmpdir" \
       "Confined Relabel Correctly"

    # podman-remote has no 'unshare'
    if is_rootless && ! is_remote; then
        run_podman unshare touch $tmpdir/test1
        # Relabel entire directory
        run_podman unshare chcon system_u:object_r:usr_t:s0 $tmpdir
        run_podman start --attach $ctrname
        newlevel=$(secon -l $output)
        is "$level" "$newlevel" "start should relabel with same SELinux labels"
        run ls -dZ $tmpdir
        is "$output" "system_u:object_r:container_file_t:$level $tmpdir" \
           "Confined Relabel Correctly"
        run ls -dZ $tmpdir/test1
        is "$output" "system_u:object_r:container_file_t:$level $tmpdir/test1" \
           "Start did not Relabel"

        # Relabel only file in subdir
        run_podman unshare chcon system_u:object_r:usr_t:s0 $tmpdir/test1
        run_podman start --attach $ctrname
        newlevel=$(secon -l $output)
        is "$level" "$newlevel" "start should use same SELinux labels"

        run ls -dZ $tmpdir/test1
        is "$output" "system_u:object_r:usr_t:s0 $tmpdir/test1" \
           "Start did not Relabel"
    fi
    run_podman rm $ctrname

    run_podman run --rm -v $tmpdir:/test:z $IMAGE cat /proc/self/attr/current
    run ls -dZ $tmpdir
    is "$output" "${RELABEL} $tmpdir" "Shared Relabel Correctly"
}

# bats test_tags=ci:parallel
@test "podman selinux nested" {
    skip_if_no_selinux

    ROOTCONTEXT='rw,rootcontext="system_u:object_r:container_file_t:s0:c1,c2"'
    SELINUXMNT="selinuxfs.*(rw,nosuid,noexec,relatime)"

    SELINUXMNT="tmpfs.*selinux.*\(ro"
    run_podman run --rm --security-opt label=level:s0:c1,c2 $IMAGE mount
    assert "$output" !~ "${ROOTCONTEXT}" "Don't use rootcontext"
    assert "$output" =~ "${SELINUXMNT}" "Mount SELinux file system readwrite"

    run_podman run --rm --security-opt label=nested --security-opt label=level:s0:c1,c2 $IMAGE mount
    assert "$output" =~ "${ROOTCONTEXT}" "Uses rootcontext"
    assert "$output" =~ "${SELINUXMNT}" "Mount SELinux file system readwrite"
}

# bats test_tags=ci:parallel
@test "podman EnableLabeledUsers" {
    skip_if_no_selinux

    overrideConf=$PODMAN_TMPDIR/containers.conf
    cat >$overrideConf <<EOF
[Containers]
label_users=true
EOF

    run id -Z
    user=$(secon -u $output)
    role=$(secon -r $output)
    CONTAINERS_CONF_OVERRIDE=$overrideConf run_podman run --rm $IMAGE cat /proc/self/attr/current
    level=$(secon -l $output)
    id -Z
    is "$output" "$user:$role:container_t:$level"  "Confined label Correctly"

    ctrname=c-$(safename)
    CONTAINERS_CONF_OVERRIDE=$overrideConf run_podman run --rm --name $ctrname --security-opt label=role:system_r $IMAGE cat /proc/self/attr/current
    level=$(secon -l $output)
    is "$output" "$user:system_r:container_t:$level" "Confined with role override label Correctly"
}

# bats test_tags=ci:parallel
@test "podman selinux: check unsupported relabel" {
    skip_if_no_selinux
    skip_if_rootless

    LABEL="system_u:object_r:tmp_t:s0"
    RELABEL="system_u:object_r:container_file_t:s0"
    tmpdir=$PODMAN_TMPDIR/vol
    mkdir -p $tmpdir

    mount --type tmpfs -o "context=\"$LABEL\"" tmpfs $tmpdir

    run getfattr --only-values --absolute-names -n security.selinux ${tmpdir}
    is "$output" "${LABEL}" "No Relabel Correctly"

    run_podman run --rm -v $tmpdir:/test:z --privileged $IMAGE true
    run getfattr --only-values --absolute-names -n security.selinux ${tmpdir}
    is "$output" "${LABEL}" "Ignored shared relabel Correctly"

    run_podman run --rm -v $tmpdir:/test:Z --privileged $IMAGE true
    run getfattr --only-values --absolute-names -n security.selinux ${tmpdir}
    is "$output" "${LABEL}" "Ignored private relabel Correctly"
    umount $tmpdir

    run_podman run --rm -v $tmpdir:/test:z --privileged $IMAGE true
    run getfattr --only-values --absolute-names -n security.selinux ${tmpdir}
    is "$output" "${RELABEL}" "Ignored private relabel Correctly"
}

# vim: filetype=sh
