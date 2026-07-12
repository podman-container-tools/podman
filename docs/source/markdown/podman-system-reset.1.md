% podman-system-reset 1

## NAME
podman\-system\-reset - Reset storage back to initial state

## SYNOPSIS
**podman system reset** [*options*]

## DESCRIPTION
**podman system reset** removes all pods, containers, images, networks and volumes, and machines.
It also removes the configured graphRoot and runRoot directories. Make sure these are not set to
some important directory.

This command must be run **before** changing any of the following fields in the
`containers.conf` or `storage.conf` files: `driver`, `static_dir`, `tmp_dir`
or `volume_path`.

`podman system reset` reads the current configuration and attempts to remove all
of the relevant configurations. If the administrator modified the configuration files first,
`podman system reset` might not be able to clean up the previous storage.

`podman system reset` does not restart podman.service and podman.socket systemd units. You may need to manually restart it after running this command.

## OPTIONS
#### **--force**, **-f**

Do not prompt for confirmation

#### **--help**, **-h**

Print usage statement

## EXAMPLES

Reset all storage back to a clean initialized state.
```
$ podman system reset
WARNING! This will remove:
        - all containers
        - all pods
        - all images
        - all networks
        - all build cache
        - all machines
        - all volumes
        - the graphRoot directory: /var/lib/containers/storage
        - the runRoot directory: /run/containers/storage
Are you sure you want to continue? [y/N] y
```

Force reset all storage back to a clean initialized state.
```
$ podman system reset --force
```

### Switching rootless user from VFS driver to overlay with fuse-overlayfs

If the user ran rootless containers without having the `fuse-overlayfs` program
installed, podman defaults to the `vfs` storage in their home directory. If they
want to switch to use fuse-overlay, they must install the fuse-overlayfs
package. The user needs to reset the storage to use overlayfs by default.
Execute `podman system reset` as the user first to remove the VFS storage. Now
the user can edit the `/etc/containers/storage.conf` to make any changes if
necessary. If the system's default was already `overlay`, then no changes are
necessary to switch to fuse-overlayfs. Podman looks for the existence of
fuse-overlayfs to use it when set in the `overlay` driver, only falling back to vfs
if the program does not exist. Users can run `podman info` to ensure Podman is
using fuse-overlayfs and the overlay driver.

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system(1)](podman-system.1.md)**,  **[fuse-overlayfs(1)](https://github.com/containers/fuse-overlayfs/blob/main/fuse-overlayfs.1.md)**, **[containers-storage.conf(5)](https://github.com/containers/storage/blob/main/docs/containers-storage.conf.5.md)**

## HISTORY
November 2019, Originally compiled by Dan Walsh (dwalsh at redhat dot com)
