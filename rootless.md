# Shortcomings of Rootless Podman

The following list categorizes the known issues and irregularities with running Podman as a non-root user. Many of these are kernel-level restrictions in place for security reasons, and are not reasonably solvable by Podman.

* Podman can not create containers that bind to ports < 1024.
  * The kernel does not allow processes without `CAP_NET_BIND_SERVICE` to bind to low ports.
  * You can modify the `net.ipv4.ip_unprivileged_port_start` sysctl to change the lowest port.  For example `sysctl net.ipv4.ip_unprivileged_port_start=443` allows rootless Podman containers to bind to ports >= 443.
  * A proxy server, kernel firewall rule, or redirection tool such as [redir](https://github.com/troglobit/redir) may be used to redirect traffic from a privileged port to an unprivileged one (where a podman pod is bound) in a server scenario - where a user has access to the root account (or setuid on the binary would be an acceptable risk), but wants to run the containers as an unprivileged user for enhanced security and for a limited number of pre-known ports.
* As of Podman 5.0, pasta is the default networking tool. Since pasta copies the IP address of the main interface, connections to that IP from containers do not work. This means that unless you have more than one interface, inter-container connections cannot be made without explicitly passing a pasta network configuration, either in `containers.conf` or at runtime.
  * If you previously had port forwards (ex. via `-p 80:80`) that other containers could access, you can use the solution (setting pasta options with `10.0.2.x` IPs) posted [here](https://blog.podman.io/2024/03/podman-5-0-breaking-changes-in-detail/).
* Rootless bridge networks use `rootlessport` for port forwarding by default, which is a userspace proxy that does not preserve client source IPs. Setting `rootless_port_forwarder="pasta"` in the `[network]` section of `containers.conf` switches to pasta's kernel-level forwarding (via `pesto`), preserving the original client IP. This option is experimental and its behavior is subject to change. It requires a version of passt (`>= passt-0^20260507.g1afd4ed`) that ships the `pesto` binary.
* If /etc/subuid and /etc/subgid are not set up for a user, then podman commands
can easily fail
  * Some identity providers (e.g. FreeIPA) have integrated subuid/subgid support, but many have not.
  * We are working to get support for NSSWITCH on the /etc/subuid and /etc/subgid files.
* No support for setting resource limits on systems using cgroups v1
* Some systemd unit configuration options do not work in the rootless container
  * Use of certain options will cause service startup failures (e.g. PrivateNetwork).  The systemd services requiring `PrivateNetwork` can be made to work by passing `--cap-add SYS_ADMIN`, but the security implications should be carefully evaluated.  In most cases, it's better to create an override.conf drop-in that sets `PrivateNetwork=no`.  This also applies to containers run by root.
* Container images cannot easily be shared with other users
* Difficult to use additional stores for sharing content
* Does not work on NFS or parallel filesystem homedirs (e.g. [GPFS](https://www.ibm.com/support/knowledgecenter/en/SSFKCN/gpfs_welcome.html))
  * NFS and parallel filesystems enforce file creation on different UIDs on the server side and does not understand User Namespace.
  * When a container root process like YUM attempts to create a file owned by a different UID, NFS Server/GPFS denies the creation.
* Requires a writable home directory that is not mounted with noexec or nodev
  * User can set up storage to point to other directories they can write to
* Support for using native overlayfs as an unprivileged user is only available for Podman version >= 3.1 on a Linux kernel version >= 5.12, otherwise the slower `fuse-overlayfs` will be used.
* Only supported storage drivers are overlay (optionally using `fuse-overlayfs`) and VFS
* A few commands do not work or have reduced functionality
  * Directories created by `podman mount` and `podman unmount` are only visible in the rootless user namespace, which can be accessed with `podman unshare`
  * `podman container checkpoint` and `podman container restore` (CRIU requires root)
* Issues with higher UIDs can cause builds to fail
  * A standard rootless configuration only gives containers access to 65536 UIDs and GIDs - images with higher UIDs and GIDs cannot be used.
  * If a build is attempting to use a UID that is not mapped into the user namespace mapping for a container, then builds will not be able to put the UID in an image.
* Making device nodes within a container fails, even when using privileged containers
  * The kernel does not allow non root user processes (processes without `CAP_MKNOD`) to create device nodes.  If a container needs to create device nodes, it must be run as root.
