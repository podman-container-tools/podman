if ! is_rootless; then
    mount --bind $TEST_ROOTDIR/etc_hosts/hosts /etc/hosts
else
    $PODMAN_BIN unshare mount --bind $TEST_ROOTDIR/etc_hosts/hosts /etc/hosts
fi
