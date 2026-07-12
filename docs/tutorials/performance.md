# Podman performance guide

The performance of Podman may be influenced by a number of factors, such as,

* the specified Podman command-line options and configuration
* the OCI runtime
* the host file system
* the container image

Changing any of these may or may not have any noticeable effect on the performance of Podman on your system.

## Using a separate user account for benchmarking

Some performance tips, such as using a different storage driver would require the user to run `podman system reset`,
which erases all containers and container images for the user.
Instead of benchmarking different alternatives in your own home directory, you could create a new user
that afterwards can be removed.

### Example: Specify the storage driver _vfs_ and run `/bin/true` in an Alpine container

Interactively

```
sudo useradd testuser
sudo machinectl shell testuser@
podman pull docker.io/library/alpine
/usr/bin/time -v podman --storage-driver=vfs run --rm docker.io/library/alpine /bin/true
exit
```

Noninteractively

```
sudo useradd testuser
systemd-run --machine=testuser@ --quiet --user --collect --pipe --wait \
   podman --storage-driver=vfs pull docker.io/library/alpine
systemd-run --machine=testuser@ --quiet --user --collect --pipe --wait \
   /usr/bin/time -v podman --storage-driver=vfs run --rm docker.io/library/alpine /bin/true
```

The command `/usr/bin/time -v` measures and displays benchmarking information.

## Performance considerations

### Use a fast OCI runtime

Podman uses an OCI runtime when running containers.
The fastest OCI runtime is probably [__crun__](https://github.com/containers/crun).

Check that you are using crun

```
$ podman info --format={{.Host.OCIRuntime.Name}}
crun
```

To benchmark an OCI runtime, create a test user account and
specify the OCI runtime path with [__--runtime__](https://docs.podman.io/en/latest/markdown/podman.1.html#runtime-value).

### Choosing a storage driver

The following storage drivers are listed from fastest to slowest:

1. native overlayfs
2. fuse-overlayfs
3. vfs

There is one notable exception to this speed ranking.
Creating a container takes significantly longer with _native overlayfs_ than _fuse-overlayfs_
when these conditions are all met:

* rootless Podman is used
* a modified UID/GID mapping is used
* _native overlayfs_ is used
* no container has yet been created with the specified container image and UID/GID mapping

Runtime speed is not affected. Only __podman create__ and the container creation phases of
__podman run__ and __podman build__ are affected.
For more details, see [GitHub comment](https://github.com/containers/podman/issues/16541#issuecomment-1352790422).
Command-line options that modify the UID/GID mapping are for example __--userns__, __--uidmap__ and __--gidmap__.
The command-line option `--userns auto` is particularly affected by this performance penalty,
because different UID/GID mappings could potentially be used on each invocation. For other uses of
__--userns__, __--uidmap__ and __--gidmap__ the performance penalty is a one-time cost
that only occurs the first time the command is run.

Using native overlayfs as an unprivileged user is available for Podman version >= 3.1 on a Linux kernel version >= 5.13.
If SELinux is not used, then Linux kernel version 5.11 or later is sufficient.
Native overlayfs support is included in RHEL >= 8.5, see [release notes](https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/8/html-single/8.5_release_notes/index).

To show the current storage driver

```
$ podman info -f {{.Store.GraphDriverName}}
overlay
$ podman info -f '{{index .Store.GraphStatus "Native Overlay Diff"}}'
true
```

Storage driver   | Result of `podman info -f {{.Store.GraphDriverName}}` | Result of `podman info -f '{{index .Store.GraphStatus "Native Overlay Diff"}}`
----             | ------                                                | -----
native overlayfs | overlay                                               | true
fuse-overlayfs   | overlay                                               | false
VFS              | vfs                                                   | false

Before changing the storage driver, running `podman system reset` is required.
The command erases all containers and container images for the user.
See the example above "_Using a separate user account for benchmarking_" for how to benchmark a storage driver in a separate test account.

#### Specifying the storage driver with command-line options

Storage driver   | Podman command
----             | ------
native overlayfs | `podman --storage-driver=overlay run ...`
fuse-overlayfs   | `podman --storage-driver=overlay --storage-opt overlay.mount_program=/usr/bin/fuse-overlayfs run ...`
VFS              | `podman --storage-driver=vfs run ...`

#### Configuring the default storage driver

The default storage driver can be configured in
_/etc/containers/storage.conf_ and overridden by a user in
_~/.config/containers/storage.conf_

To configure native overlayfs:
```
[storage]
driver = "overlay"
```

To configure fuse-overlayfs:
```
[storage]
driver = "overlay"
[storage.options.overlay]
mount_program = "/usr/bin/fuse-overlayfs"
```

To configure VFS:
```
[storage]
driver = "vfs"
```

See storage.conf(5) for all available configuration settings.

### Network performance for rootless Podman

When using rootless Podman, network traffic is normally passed through the network driver
[pasta](https://passt.top/passt/about/#pasta). This comes with a performance penalty.

You can avoid using _pasta_ in the following ways:

* Use socket activation for listening network sockets. Communication over the activated socket does not pass through
  pasta, so it has the same performance characteristics as the normal network on the host.
  Socket-activated services can be started and stopped in different ways:
  + Let systemd start the service when the first client connects. Let the service terminate by itself after some time of inactivity.
    Using a service on demand, can free up compute resources.
  + Start the service explicitly (`systemctl --user enable foobar.service`). If the service is already
    running when the first client connects, there will be no delay due to container startup.
  The [socket activation tutorial](https://github.com/containers/podman/blob/main/docs/tutorials/socket_activation.md)
  provides more information about socket activation support in Podman.

* Set up the network manually as root. Create a bridge and virtual ethernet pair (VETH). Note: compared to other methods,
  this setup doesn't provide any network isolation. In containers granted CAP_NET_ADMIN or CAP_NET_RAW, processes can
  open packet or raw sockets directly facing the host, which allows them to send arbitrary frames, including
  crafted Ethernet and IP packets, as well as receiving packets that were not originally intended for the container,
  by means of ARP spoofing.
  For more information, see
  + An [example](https://lists.podman.io/archives/list/podman@lists.podman.io/thread/W6MCYO6RY5YFRTSUDAOEZA7SC2EFXRZE/) posted on the Podman mailing list
  + The section _DIY networking_ in [Podman-Rootless-Networking.pdf](https://containers.github.io/podman.io_old/old/community/meeting/notes/2021-10-05/Podman-Rootless-Networking.pdf)

* Use `--network=host`. No network namespace is created. The container will use the host’s network.
  Note: By using `--network=host`, the container is given full access to local system services such as D-bus and is therefore considered insecure.

Pasta is the default and only rootless network driver since Podman 6.0.0.

Since Podman 5.1.0 the default network driver can be shown with

```
$ podman info -f '{{.Host.RootlessNetworkCmd}}'
pasta
```

### Partial pulling of container images

Podman supports partial pulling for the following container image formats:

* __zstd:chunked__

* __eStargz__

__zstd:chunked__ has better performance than __eStargz__.

Partial pulling downloads only the layer contents that are missing locally, but the image is still fully pulled before its containers run. Lazy pulling, where the image is mounted before it is fully downloaded, is out of scope for Podman itself and requires an additional snapshotter such as the [stargz-snapshotter](https://github.com/containerd/stargz-snapshotter) configured as an additional layer store.

See the article [_Pull container images faster with partial pulls_](https://www.redhat.com/sysadmin/faster-container-image-pulls) by Giuseppe Scrivano and Dan Walsh.

### Choosing a host file system

Partial pulling of container images can run more efficiently when the file system has reflink support. The file systems XFS and BTRFS have reflink support.

### Option --log-driver

The `podman run` option [__--log-driver__](https://docs.podman.io/en/latest/markdown/podman-run.1.html#log-driver-driver) specifies the logging driver for the container.

If logging is not needed, consider using `--log-driver=none` to disable logging.

### Reuse the package repository cache when building container images

The first step of a container build is often to download metadata from
the package repositories and post-process that data.

To speed up container builds, you could prepare a directory on the host
that contains the package metadata and then make the directory available
to the container build by using an _overlay mount_.

#### Example : Speed up _podman build_ by reusing the DNF metadata cache

In this example the containers are based on Fedora 36.

First create an empty directory on the host, for example *$HOME/dnf_cache_f36*.

```
$ mkdir $HOME/dnf_cache_f36
```

Fill the directory with the most recent __dnf__ metadata cache.

```
$ podman run --rm -v $HOME/dnf_cache_f36:/var/cache/dnf:Z registry.fedoraproject.org/fedora:36 dnf makecache
```

Create a new directory, for example, _$HOME/ctr_ and a file _$HOME/ctr/Containerfile_ with these contents

```
FROM registry.fedoraproject.org/fedora:36
RUN dnf -y update && dnf -y install cowsay && dnf clean all
```

To build the Containerfile using the prepared metadata cache, provide the directory _$HOME/dnf_cache_f36_ as an _overlay mount_ (volume option `:O`)

```
$ podman build -v $HOME/dnf_cache_f36:/var/cache/dnf:O -t cowsay $HOME/ctr
```

The article [_Speeding up container image builds with Buildah_](https://www.redhat.com/sysadmin/speeding-container-buildah) by Dan Walsh provides more details.
