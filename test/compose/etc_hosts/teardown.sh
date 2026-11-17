if ! is_rootless; then
    umount /etc/hosts
else
    $PODMAN_BIN unshare umount /etc/hosts
fi
