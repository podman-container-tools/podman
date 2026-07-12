#!/usr/bin/env bats   -*- bats -*-
#
# podman blkio-related tests
#

load helpers

function teardown() {
    lofile=${PODMAN_TMPDIR}/disk.img
    if [ -f ${lofile} ]; then
        run_podman '?' rm -t 0 --all --force --ignore

        while read path dev; do
            if [[ "$path" == "$lofile" ]]; then
                losetup -d $dev
            fi
        done < <(losetup -l --noheadings --output BACK-FILE,NAME)

        rm ${lofile}
    fi
    basic_teardown
}

@test "podman run --blkio-weight-device" {
    skip_if_rootless "cannot create devices in rootless mode"

    # create loopback device
    lofile=${PODMAN_TMPDIR}/disk.img
    fallocate -l 1k  ${lofile}
    losetup -f ${lofile}

    run losetup -l --noheadings --output BACK-FILE,NAME,MAJ:MIN
    assert "$status" -eq 0 "losetup: status"
    assert "$output" != "" "losetup: output"

    lodevice=$(awk "\$1 == \"$lofile\" { print \$2 }" <<<"$output")
    lomajmin=$(awk "\$1 == \"$lofile\" { print \$3 }" <<<"$output")

    is "$lodevice" ".\+" "Could not determine device for $lofile"
    is "$lomajmin" ".\+" "Could not determine major/minor for $lofile"

    # use bfq io scheduler
    run grep -w bfq /sys/block/$(basename ${lodevice})/queue/scheduler
    if [ $status -ne 0 ]; then
        skip "BFQ scheduler is not supported on the system"
    fi
    echo bfq > /sys/block/$(basename ${lodevice})/queue/scheduler

    # run podman
    if is_cgroupsv2; then
        if [ ! -f /sys/fs/cgroup/system.slice/io.bfq.weight ]; then
            skip "Kernel does not support BFQ IO scheduler"
        fi
        run_podman run --device ${lodevice}:${lodevice} --blkio-weight-device ${lodevice}:123 --rm $IMAGE \
            /bin/sh -c "cat /sys/fs/cgroup/\$(sed -e 's/0:://' < /proc/self/cgroup)/io.bfq.weight"
        is "${lines[1]}" "${lomajmin}\s\+123"
    else
        if [ ! -f /sys/fs/cgroup/blkio/system.slice/blkio.bfq.weight_device ]; then
            skip "Kernel does not support BFQ IO scheduler"
        fi
        if [ $(podman_runtime) = "crun" ]; then
            # As of crun 1.2, crun doesn't support blkio.bfq.weight_device
            skip "crun doesn't support blkio.bfq.weight_device"
        fi
        run_podman run --device ${lodevice}:${lodevice} --blkio-weight-device ${lodevice}:123 --rm $IMAGE \
            /bin/sh -c "cat /sys/fs/cgroup/blkio/blkio.bfq.weight_device"
        is "${lines[1]}" "${lomajmin}\s\+123"
    fi
}
