#!/usr/bin/env bats   -*- bats -*-
#
# Tests #2730 - regular users are not able to read/write container storage
# Tests #6957 - /sys/dev (et al) are masked from unprivileged containers
#

load helpers

@test "podman container storage is not accessible by unprivileged users" {
    skip_if_rootless "test meaningless without suid"
    skip_if_remote

    run_podman run --name c_uidmap   --uidmap 0:10000:10000 $IMAGE true
    run_podman run --name c_uidmap_v --uidmap 0:10000:10000 -v foo:/foo $IMAGE true

    run_podman run --name c_mount $IMAGE \
               sh -c "echo hi > /myfile;mkdir -p /mydir/mysubdir; chmod 777 /myfile /mydir /mydir/mysubdir"

    run_podman mount c_mount
    mount_path=$output

    # Do all the work from within a test script. Since we'll be invoking it
    # as a user, the parent directory must be world-readable.
    test_script=$PODMAN_TMPDIR/fail-if-writable
    cat >$test_script <<"EOF"
#!/usr/bin/env bash

path="$1"

die() {
    echo "#/vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv"  >&2
    echo "#| FAIL: $*"                                           >&2
    echo "#\\^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^" >&2

    # Show permissions of directories from here on up
    while expr "$path" : "/var/lib/containers" >/dev/null; do
        echo "#|  $(ls -ld $path)"
        path=$(dirname $path)
    done

    exit 1
}

parent=$(dirname "$path")
if chmod +w $parent; then
    die "Able to chmod $parent"
fi
if chmod +w "$path"; then
    die "Able to chmod $path"
fi

EOF

    # Under overlay, and presumably any future storage drivers, we
    # should never be able to read or write $path.
    #
    # Under VFS, though, if podman has *ever* been run with --uidmap,
    # all images become world-accessible. So don't bother checking.
    if [[ $(podman_storage_driver) != "vfs" ]]; then
        cat >>$test_script <<EOF
if [ -d "$path" ]; then
    if ls "$path" >/dev/null; then
        die "Able to run 'ls $path' without error"
    fi
    if echo hi >"$path"/test; then
        die "Able to write to file under $path"
    fi
else
    # Plain file
    if cat "$path" >/dev/null; then
        die "Able to read $path"
    fi
    if echo hi >"$path"; then
        die "Able to write to $path"
    fi
fi

EOF
    fi
    echo "exit 0" >>$test_script
    chmod 755 $PODMAN_TMPDIR $test_script

    # get podman image and container storage directories
    run_podman info --format '{{.Store.GraphRoot}}'
    is "$output" "/var/lib/containers/storage" "GraphRoot in expected place"
    GRAPH_ROOT="$output"
    run_podman info --format '{{.Store.RunRoot}}'
    is "$output" ".*/run/containers/storage" "RunRoot in expected place"
    RUN_ROOT="$output"

    # The main test: find all world-writable files or directories underneath
    # container storage, run the test script as a nonroot user, and try to
    # access each path.
    find $GRAPH_ROOT $RUN_ROOT \! -type l -perm -o+w -print | while read i; do
        dprint " o+w: $i"

        # use chroot because su fails if uid/gid don't exist or have no shell
        # For development: test all this by removing the "--userspec x:x"
        chroot --userspec 1000:1000 / $test_script "$i"
    done

    # Done. Clean up.
    rm -f $test_script

    run_podman umount c_mount
    run_podman rm c_mount

    run_podman rm c_uidmap c_uidmap_v
    run_podman volume rm foo
}


# #6957 - mask out /proc/acpi, /sys/dev, and other sensitive system files
@test "sensitive mount points are masked without --privileged" {
    # FIXME: this should match the list in pkg/specgen/generate/config_linux.go
    local -a mps=(
        /proc/acpi
        /proc/kcore
        /proc/keys
        /proc/latency_stats
        /proc/timer_list
        /proc/timer_stats
        /proc/sched_debug
        /proc/scsi
        /sys/firmware
        /sys/fs/selinux
        /sys/dev/block
    )

    # Some of the above may not exist on our host. Find only the ones that do.
    local -a subset=()
    for mp in "${mps[@]}"; do
        if [ -e $mp ]; then
            subset+=($mp)
        fi
    done

    # If the image does not exists, the pull output will make the test below fail
    _prefetch $IMAGE

    # Run 'stat' on all the files, plus /dev/null. Get path, file type,
    # number of links, major, and minor (see below for why). Do it all
    # in one go, to avoid multiple podman-runs
    run_podman '?' run --rm $IMAGE stat -c'%n:%F:%h:%T:%t' /dev/null "${subset[@]}"
    assert $status -le 1 "stat exit status: expected 0 or 1"

    local devnull=
    for result in "${lines[@]}"; do
        # e.g. /proc/acpi:character special file:1:3:1
        local IFS=:
        read path type nlinks major minor <<<"$result"

        if [[ $path = "/dev/null" ]]; then
            # /dev/null is our reference point: masked *files* (not directories)
            # will be created as /dev/null clones.
            # This depends on 'stat' returning results in argv order,
            # so /dev/null is first, so we have a reference for others.
            # If that ever breaks, this test will have to be done in two passes.
            devnull="$major:$minor"
        elif [[ $type = "character special file" ]]; then
            # Container file is a character device: it must match /dev/null
            is "$major:$minor" "$devnull" "$path: major/minor matches /dev/null"
        elif [[ $type = "directory" ]]; then
            # Directories: must be empty (only two links).
            # FIXME: this is a horrible almost-worthless test! It does not
            # actually check for files in the directory (expect: zero),
            # merely for the nonexistence of any subdirectories! It relies
            # on the observed (by Ed) fact that all the masked directories
            # contain further subdirectories on the host. If there's ever
            # a new masked directory that contains only files, this test
            # will silently pass without any indication of error.
            # If you can think of a better way to do this check,
            # please feel free to fix it.
            is "$nlinks" "2" "$path: directory link count"
        elif [[ $result =~ stat:.*No.such.file.or.directory ]]; then
            # No matter what the path is, this is OK. It has to do with #8949
            # and RHEL8 and rootless and cgroups v1. Bottom line, what we care
            # about is that the path not be available inside the container.
            :
        else
            die "$path: Unknown file type '$type'"
        fi
    done
}

# vim: filetype=sh
