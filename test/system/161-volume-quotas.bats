#!/usr/bin/env bats   -*- bats -*-
#
# podman volume XFS quota tests
#

load helpers

function setup() {
    basic_setup

    run_podman '?' volume rm -a
}

function teardown() {
    run_podman '?' rm -af -t 0
    run_podman '?' volume rm -a

    loop=$PODMAN_TMPDIR/disk.img
    vol_path=$PODMAN_TMPDIR/volpath
    if [ -f ${loop} ]; then
        if [ -d ${vol_path} ]; then
           if mountpoint ${vol_path}; then
              umount "$vol_path"
           fi
           rm -rf "$vol_path"
        fi

        while read path dev; do
            if [[ "$path" == "$loop" ]]; then
                losetup -d $dev
            fi
        done < <(losetup -l --noheadings --output BACK-FILE,NAME)
        rm -f $loop
    fi

    basic_teardown
}

@test "podman volumes with XFS quotas" {
    skip_if_rootless "Quotas are only possible with root"
    skip_if_remote "Requires --root flag, not possible w/ remote"

    OS_RELEASE_ID="${OS_RELEASE_ID:-$(source /etc/os-release; echo $ID)}"
    if [[ "$OS_RELEASE_ID" == "fedora" ]]; then
        skip "FIXME #27759: There is a selinux problem with this test"
    fi

    # Minimum XFS filesystem size is 300mb
    loop=$PODMAN_TMPDIR/disk.img
    fallocate -l 300m  ${loop}
    run -0 losetup -f --show $loop
    loop_dev="$output"
    mkfs.xfs $loop_dev

    safe_opts=$(podman_isolation_opts ${PODMAN_TMPDIR})
    vol_path=$PODMAN_TMPDIR/volpath
    mkdir -p $vol_path
    safe_opts="$safe_opts --volumepath=$vol_path"
    mount -t xfs -o defaults,pquota $loop_dev $vol_path

    vol_one="testvol1"
    run_podman $safe_opts volume create --opt o=size=2m $vol_one

    vol_two="testvol2"
    run_podman $safe_opts volume create --opt o=size=4m $vol_two

    # prefetch image to avoid registry pulls because this is using a
    # unique root which does not have the image already present.
    # _PODMAN_TEST_OPTS is used to overwrite the podman options to
    # make the function aware of the custom --root.
    _PODMAN_TEST_OPTS="$safe_opts --storage-driver $(podman_storage_driver)" _prefetch $IMAGE

    ctrname="testctr"
    # pull never to ensure the prefetch works correctly
    run_podman $safe_opts run -d --pull=never --name=$ctrname -i -v $vol_one:/one -v $vol_two:/two $IMAGE top

    run_podman $safe_opts exec $ctrname dd if=/dev/zero of=/one/oneMB bs=1M count=1
    run_podman 1 $safe_opts exec $ctrname dd if=/dev/zero of=/one/twoMB bs=1M count=1
    assert "$output" =~ "No space left on device"
    run_podman $safe_opts exec $ctrname dd if=/dev/zero of=/two/threeMB bs=1M count=3
    run_podman 1 $safe_opts exec $ctrname dd if=/dev/zero of=/two/oneMB bs=1M count=1
    assert "$output" =~ "No space left on device"

    run_podman $safe_opts rm -f -t 0 $ctrname
    run_podman $safe_opts volume rm -af
}

# vim: filetype=sh
