![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

# Troubleshooting

## A list of common issues and solutions for Podman

---
### 1) Variety of issues - Validate Version

A large number of issues reported against Podman are often found to already be fixed
in more current versions of the project.  Before reporting an issue, please verify the
version you are running with `podman version` and compare it to the latest release on our [GitHub Releases page](https://github.com/podman-container-tools/podman/releases/latest).

If they differ, please update your version of PODMAN to the latest possible
and retry your command before reporting the issue.

---
### 2) Can't use volume mount, get permission denied

```console
$ podman run -v ~/mycontent:/content fedora touch /content/file
touch: cannot touch '/content/file': Permission denied
```

#### Solution

This is sometimes caused by SELinux, and sometimes by user namespaces.

Labeling systems like SELinux require that proper labels are placed on volume
content mounted into a container. Without a label, the security system might
prevent the processes running inside the container from using the content. By
default, Podman does not change the labels set by the OS.

To change a label in the container context, you can add either of two suffixes
**:z** or **:Z** to the volume mount. These suffixes tell Podman to relabel file
objects on the shared volumes. The **z** option tells Podman that two containers
share the volume content. As a result, Podman labels the content with a shared
content label. Shared volume labels allow all containers to read/write content.
The **Z** option tells Podman to label the content with a private unshared label.
Only the current container can use a private volume.

```console
$ podman run -v ~/mycontent:/content:Z fedora touch /content/file
```

Make sure the content is private for the container.  Do not relabel system directories and content.
Relabeling system content might cause other confined services on your machine to fail.  For these
types of containers we recommend having SELinux separation disabled.  The option `--security-opt label=disable`
will disable SELinux separation for the container.

```console
$ podman run --security-opt label=disable -v ~:/home/user fedora touch /home/user/file
```

In cases where the container image runs as a specific, non-root user, though, the
solution is to fix the user namespace.  This would include container images such as
the Jupyter Notebook image (which runs as "jovyan") and the Postgres image (which runs
as "postgres").  In either case, use the `--userns` switch to map user namespaces,
most of the time by using the **keep-id** option.

```console
$ podman run -v "$PWD":/home/jovyan/work --userns=keep-id jupyter/scipy-notebook
```

---
### 3) No such image or Bare keys cannot contain ':'

When doing a `podman pull` or `podman build` command and a "common" image cannot be pulled,
it is likely that the `/etc/containers/registries.conf` file is either not installed or possibly
misconfigured.

#### Symptom

```console
$ sudo podman build -f Dockerfile
STEP 1: FROM alpine
error building: error creating build container: no such image "alpine" in registry: image not known
```

or

```console
$ sudo podman pull fedora
error pulling image "fedora": unable to pull fedora: error getting default registries to try: Near line 9 (last key parsed ''): Bare keys cannot contain ':'.
```

#### Solution

  * Verify that the `/etc/containers/registries.conf` file exists.  If not, verify that the containers-common package is installed.
  * Verify that the entries in the `unqualified-search-registries` list of the `/etc/containers/registries.conf` file are valid and reachable.
    * i.e. `unqualified-search-registries = ["registry.fedoraproject.org", "quay.io", "registry.access.redhat.com"]`

---
### 4) http: server gave HTTP response to HTTPS client

When doing a Podman command such as `build`, `commit`, `pull`, or `push` to a registry,
TLS verification is turned on by default.  If encryption is not used with
those commands, this error can occur.

#### Symptom

```console
$ sudo podman push alpine docker://localhost:5000/myalpine:latest
Getting image source signatures
Get https://localhost:5000/v2/: http: server gave HTTP response to HTTPS client
```

#### Solution

By default TLS verification is turned on when communicating to registries from
Podman.  If the registry does not require encryption the Podman commands
such as `build`, `commit`, `pull` and `push` will fail unless TLS verification is turned
off using the `--tls-verify` option.  **NOTE:** It is not at all recommended to
communicate with a registry and not use TLS verification.

  * Turn off TLS verification by passing false to the tls-verify option.
  * I.e. `podman push --tls-verify=false alpine docker://localhost:5000/myalpine:latest`


For a global workaround, users[1] can create the file `/etc/containers/registries.conf.d/registry-NAME.conf`
(replacing NAME with the name of this registry) with the following content (replacing FULLY.QUALIFIED.NAME.OF.REGISTRY with the address of this registry):

```
[[registry]]
location = "FULLY.QUALIFIED.NAME.OF.REGISTRY"
insecure = true
```

[1] If you are using a Mac / Windows, you should execute `podman machine ssh` to login into podman machine before adding the insecure entry to the registry—conf file.

**This is an insecure method and should be used cautiously.**

---
### 5) rootless containers cannot ping hosts

When using the ping command from a non-root container, the command may
fail because of a lack of privileges.

#### Symptom

```console
$ podman run --rm fedora ping -W10 -c1 redhat.com
PING redhat.com (209.132.183.105): 56 data bytes

--- redhat.com ping statistics ---
1 packets transmitted, 0 packets received, 100% packet loss
```

#### Solution

It is most likely necessary to enable unprivileged pings on the host.
Be sure the UID of the user is part of the range in the
`/proc/sys/net/ipv4/ping_group_range` file.

To change its value you can use something like:

```console
# sysctl -w "net.ipv4.ping_group_range=0 2000000"
```

To make the change persistent, you'll need to add a file in
`/etc/sysctl.d` that contains `net.ipv4.ping_group_range=0 $MAX_UID`.

---
### 6) Build hangs when the Dockerfile contains the useradd command

When the Dockerfile contains a command like `RUN useradd -u 99999000 -g users newuser` the build can hang.

#### Symptom

If you are using a useradd command within a Dockerfile with a large UID/GID, it will create a large sparse file `/var/log/lastlog`.  This can cause the build to hang forever.  Go language does not support sparse files correctly, which can lead to some huge files being created in your container image.

#### Solution

If the entry in the Dockerfile looked like: RUN useradd -u 99999000 -g users newuser then add the `--no-log-init` parameter to change it to: `RUN useradd --no-log-init -u 99999000 -g users newuser`. This option tells useradd to stop creating the lastlog file.

### 7) Permission denied when running Podman commands

When rootless Podman attempts to execute a container on a non exec home directory a permission error will be raised.

#### Symptom

If you are running Podman or Buildah on a home directory that is mounted noexec,
then they will fail with a message like:

```console
$ podman run centos:7
standard_init_linux.go:203: exec user process caused "permission denied"
```

#### Solution

Since the administrator of the system set up your home directory to be noexec, you will not be allowed to execute containers from storage in your home directory. It is possible to work around this by manually specifying a container storage path that is not on a noexec mount. Simply copy the file /etc/containers/storage.conf to ~/.config/containers/ (creating the directory if necessary). Specify a graphroot directory which is not on a noexec mount point and to which you have read/write privileges.  You will need to modify other fields to writable directories as well.

For example

```console
$ cat ~/.config/containers/storage.conf
[storage]
  driver = "overlay"
  runroot = "/run/user/1000"
  graphroot = "/execdir/myuser/storage"
  [storage.options]
    mount_program = "/bin/fuse-overlayfs"
```

### 8) Permission denied when running systemd within a Podman container

When running systemd as PID 1 inside of a container on an SELinux
separated machine, it needs to write to the cgroup file system.

#### Symptom

Systemd gets permission denied when attempting to write to the cgroup file
system, and AVC messages start to show up in the audit.log file or journal on
the system.

#### Solution

Newer versions of Podman (2.0 or greater) support running init based containers
with a different SELinux labels, which allow the container process access to the
cgroup file system. This feature requires container-selinux-2.132 or newer
versions.

Prior to Podman 2.0, the SELinux boolean `container_manage_cgroup` allows
container processes to write to the cgroup file system. Turn on this boolean,
on SELinux separated systems, to allow systemd to run properly in the container.
Only do this on systems running older versions of Podman.

```console
# setsebool -P container_manage_cgroup true
```

### 9) Newuidmap missing when running rootless Podman commands

Rootless Podman requires the newuidmap and newgidmap programs to be installed.

#### Symptom

If you are running Podman or Buildah as a rootless user, you get an error complaining about
a missing newuidmap executable.

```console
$ podman run -ti fedora sh
command required for rootless mode with multiple IDs: exec: "newuidmap": executable file not found in $PATH
```

#### Solution

Install a version of shadow-utils that includes these executables.  Note that for RHEL and CentOS 7, at least the 7.7 release must be installed for support to be available.

### 10) rootless setup user: invalid argument

Rootless Podman requires the user running it to have a range of UIDs listed in /etc/subuid and /etc/subgid.

#### Symptom

A user, either via --user or through the default configured for the image, is not mapped inside the namespace.

```console
$ podman run --rm -ti --user 1000000 alpine echo hi
Error: container create failed: container_linux.go:344: starting container process caused "setup user: invalid argument"
```

#### Solution

Update the /etc/subuid and /etc/subgid with fields for users that look like:

```console
$ cat /etc/subuid
johndoe:100000:65536
test:165536:65536
```

The format of this file is `USERNAME:UID:RANGE`

* username as listed in `/etc/passwd` or `getpwent`.
* The initial uid allocated for the user.
* The size of the range of UIDs allocated for the user.

This means johndoe is allocated UIDs 100000-165535 as well as his standard UID in the
`/etc/passwd` file.

You should ensure that each user has a unique range of UIDs, because overlapping UIDs,
would potentially allow one user to attack another user. In addition, make sure
that the range of UIDs you allocate can cover all UIDs that the container
requires. For example, if the container has a user with UID 10000, ensure you
have at least 10000 subuids, and if the container needs to be run as a user with
UID 1000000, ensure you have at least 1000000 subuids.

You could also use the `usermod` program to assign UIDs to a user.

If you update either the `/etc/subuid` or `/etc/subgid` file, you need to
stop all running containers and kill the pause process.  This is done
automatically by the `system migrate` command, which can also be used
to stop all the containers and kill the pause process.

```console
# usermod --add-subuids 200000-201000 --add-subgids 200000-201000 johndoe
# grep johndoe /etc/subuid /etc/subgid
/etc/subuid:johndoe:200000:1001
/etc/subgid:johndoe:200000:1001
```

### 11) Changing the location of the Graphroot leads to permission denied

When I change the graphroot storage location in storage.conf, the next time I
run Podman, I get an error like:

```console
# podman run -p 5000:5000 -it centos bash

bash: error while loading shared libraries: /lib64/libc.so.6: cannot apply additional memory protection after relocation: Permission denied
```

For example, the admin sets up a spare disk to be mounted at `/src/containers`,
and points storage.conf at this directory.


#### Symptom

SELinux blocks containers from using arbitrary locations for overlay storage.
These directories need to be labeled with the same labels as if the content was
under `/var/lib/containers/storage`.

#### Solution

Tell SELinux about the new containers storage by setting up an equivalence record.
This tells SELinux to label content under the new path, as if it was stored
under `/var/lib/containers/storage`.

```console
# semanage fcontext -a -e /var/lib/containers /srv/containers
# restorecon -R -v /srv/containers
```

The semanage command above tells SELinux to set up the default labeling of
`/srv/containers` to match `/var/lib/containers`.  The `restorecon` command
tells SELinux to apply the labels to the actual content.

Now all new content created in these directories will automatically be created
with the correct label.

### 12) Anonymous image pull fails with 'invalid username/password'

Pulling an anonymous image that doesn't require authentication can result in an
`invalid username/password` error.

#### Symptom

If you pull an anonymous image, one that should not require credentials, you can receive
an `invalid username/password` error if you have credentials established in the
authentication file for the target container registry that are no longer valid.

```console
$ podman run -it --rm docker://docker.io/library/alpine:latest ls
Trying to pull docker://docker.io/library/alpine:latest...ERRO[0000] Error pulling image ref //alpine:latest: Error determining manifest MIME type for docker://alpine:latest: unable to retrieve auth token: invalid username/password
Failed
Error: unable to pull docker://docker.io/library/alpine:latest: unable to pull image: Error determining manifest MIME type for docker://alpine:latest: unable to retrieve auth token: invalid username/password
```

This can happen if the authentication file is modified 'by hand' or if the credentials
are established locally and then the password is updated later in the container registry.

#### Solution

Depending upon which container tool was used to establish the credentials, use `podman logout`
or `docker logout` to remove the credentials from the authentication file.

### 13) Running Podman inside a container causes container crashes and inconsistent states

Running Podman in a container and forwarding some, but not all, of the required host directories can cause inconsistent container behavior.

#### Symptom

After creating a container with Podman's storage directories mounted in from the host and running Podman inside a container, all containers show their state as "configured" or "created", even if they were running or stopped.

#### Solution

When running Podman inside a container, it is recommended to mount at a minimum `/var/lib/containers/storage/` as a volume.
Typically, you will not mount in the host version of the directory, but if you wish to share containers with the host, you can do so.
If you do mount in the host's `/var/lib/containers/storage`, however, you must also mount in the host's `/run/libpod` and `/run/containers/storage` directories.
Not doing this will cause Podman in the container to detect that temporary files have been cleared, leading it to assume a system restart has taken place.
This can cause Podman to reset container states and lose track of running containers.

For running containers on the host from inside a container, we also recommend the [Podman remote client](docs/tutorials/remote_client.md), which only requires a single socket to be mounted into the container.

### 14) Rootless 'podman build' fails EPERM on NFS:

NFS enforces file creation on different UIDs on the server side and does not understand user namespace, which rootless Podman requires.
When a container root process like YUM attempts to create a file owned by a different UID, NFS Server denies the creation.
NFS is also a problem for the file locks when the storage is on it.  Other distributed file systems (for example: Lustre, Spectrum Scale, the General Parallel File System (GPFS)) are also not supported when running in rootless mode as these file systems do not understand user namespace.

#### Symptom
```console
$ podman build .
ERRO[0014] Error while applying layer: ApplyLayer exit status 1 stdout:  stderr: open /root/.bash_logout: permission denied
error creating build container: Error committing the finished image: error adding layer with blob "sha256:a02a4930cb5d36f3290eb84f4bfa30668ef2e9fe3a1fb73ec015fc58b9958b17": ApplyLayer exit status 1 stdout:  stderr: open /root/.bash_logout: permission denied
```

#### Solution
Choose one of the following:
  * Set up containers/storage in a different directory, not on an NFS share.
    * Create a directory on a local file system.
    * Edit `~/.config/containers/containers.conf` and point the `volume_path` option to that local directory. (Copy `/usr/share/containers/containers.conf` if `~/.config/containers/containers.conf` does not exist)
  * Otherwise just run Podman as root, via `sudo podman`

### 15) Rootless 'podman build' fails when using OverlayFS:

The Overlay file system (OverlayFS) requires the ability to call the `mknod` command when creating whiteout files
when extracting an image.  However, a rootless user does not have the privileges to use `mknod` in this capacity.

#### Symptom
```console
$ podman build --storage-driver overlay .
STEP 1: FROM docker.io/ubuntu:xenial
Getting image source signatures
Copying blob edf72af6d627 done
Copying blob 3e4f86211d23 done
Copying blob 8d3eac894db4 done
Copying blob f7277927d38a done
Copying config 5e13f8dd4c done
Writing manifest to image destination
Storing signatures
Error: creating build container: Error committing the finished image: error adding layer with blob "sha256:8d3eac894db4dc4154377ad28643dfe6625ff0e54bcfa63e0d04921f1a8ef7f8": Error processing tar file(exit status 1): operation not permitted
$ podman build .
ERRO[0014] Error while applying layer: ApplyLayer exit status 1 stdout:  stderr: open /root/.bash_logout: permission denied
error creating build container: Error committing the finished image: error adding layer with blob "sha256:a02a4930cb5d36f3290eb84f4bfa30668ef2e9fe3a1fb73ec015fc58b9958b17": ApplyLayer exit status 1 stdout:  stderr: open /root/.bash_logout: permission denied
```

#### Solution
Choose one of the following:
  * Complete the build operation as a privileged user.
  * Install and configure fuse-overlayfs.
    * Install the fuse-overlayfs package for your Linux Distribution.
    * Add `mount_program = "/usr/bin/fuse-overlayfs"` under `[storage.options]` in your `~/.config/containers/storage.conf` file.

### 16) RHEL 7 and CentOS 7 based `init` images don't work with cgroup v2

The systemd version shipped in RHEL 7 and CentOS 7 doesn't have support for cgroup v2.  Support for cgroup v2 requires version 230 of systemd or newer, which
was never shipped or supported on RHEL 7 or CentOS 7.

#### Symptom
```console
# podman run --name test -d registry.access.redhat.com/rhel7-init:latest && sleep 10 && podman exec test systemctl status
c8567461948439bce72fad3076a91ececfb7b14d469bfa5fbc32c6403185beff
Failed to get D-Bus connection: Operation not permitted
Error: non zero exit code: 1: OCI runtime error
```

#### Solution
You'll need to either:

* configure the host to use cgroup v1. On Fedora you can do:

```console
# dnf install -y grubby
# grubby --update-kernel=ALL --args=”systemd.unified_cgroup_hierarchy=0"
# reboot
```

* update the image to use an updated version of systemd.

### 17) rootless containers exit once the user session exits

You need to set lingering mode through loginctl to prevent user processes to be killed once
the user session completed.

#### Symptom

Once the user logs out all the containers exit.

#### Solution

```console
# loginctl enable-linger $UID
```

### 18) `podman run` fails with "bpf create: permission denied error"

The Kernel Lockdown patches deny eBPF programs when Secure Boot is enabled in the BIOS. [Matthew Garrett's post](https://mjg59.dreamwidth.org/50577.html) describes the relationship between Lockdown and Secure Boot and [Jan-Philip Gehrcke's](https://gehrcke.de/2019/09/running-an-ebpf-program-may-require-lifting-the-kernel-lockdown/) connects this with eBPF. [RH bug 1768125](https://bugzilla.redhat.com/show_bug.cgi?id=1768125) contains some additional details.

#### Symptom

Attempts to run podman result in

```Error: bpf create : Operation not permitted: OCI runtime permission denied error```

#### Solution

One workaround is to disable Secure Boot in your BIOS.

### 19) error creating libpod runtime: there might not be enough IDs available in the namespace

Unable to pull images

#### Symptom

```console
$ podman unshare cat /proc/self/uid_map
	 0       1000          1
```

#### Solution

```console
$ podman system migrate
```

Original command now returns

```console
$ podman unshare cat /proc/self/uid_map
	 0       1000          1
	 1     100000      65536
```

Reference [subuid](https://man7.org/linux/man-pages/man5/subuid.5.html) and [subgid](https://man7.org/linux/man-pages/man5/subgid.5.html) man pages for more detail.

### 20) Passed-in devices or files can't be accessed in rootless container

As a non-root user you have group access rights to a device or files that you
want to pass into a rootless container with `--device=...` or `--volume=...`

#### Symptom

Any access inside the container is rejected with "Permission denied".

#### Solution

The runtime uses `setgroups(2)` hence the process loses all additional groups
the non-root user has. Use the `--group-add keep-groups` flag to pass the
user's supplementary group access into the container. For it to work,
the container itself must not call `setgroups()`.
Currently only available with the `crun` OCI runtime.

### 21) A rootless container running in detached mode is closed at logout
<!-- This is the same as section 17 above and should be deleted -->

When running a container with a command like `podman run --detach httpd` as
a rootless user, the container is closed upon logout and is not kept running.

#### Symptom

When logging out of a rootless user session, all containers that were started
in detached mode are stopped and are not kept running.  As the root user, these
same containers would survive the logout and continue running.

#### Solution

When systemd notes that a session that started a Podman container has exited,
it will also stop any containers that have been associated with it.  To avoid
this, use the following command before logging out: `loginctl enable-linger`.
To later revert the linger functionality, use `loginctl disable-linger`.

LOGINCTL(1), SYSTEMD(1)

### 22) Containers default detach keys conflict with shell history navigation

Podman defaults to `ctrl-p,ctrl-q` to detach from a running containers. The
bash and zsh shells default to `ctrl-p` for the displaying of the previous
command.  This causes issues when running a shell inside of a container.

#### Symptom

With the default detach key combo ctrl-p,ctrl-q, shell history navigation
(tested in bash and zsh) using ctrl-p to access the previous command will not
display this previous command, or anything else.  Conmon is waiting for an
additional character to see if the user wants to detach from the container.
Adding additional characters to the command will cause it to be displayed along
with the additional character. If the user types ctrl-p a second time the shell
display the 2nd to last command.

#### Solution

The solution to this is to change the default detach_keys. For example in order
to change the defaults to `ctrl-q,ctrl-q` use the `--detach-keys` option.

```console
$ podman run -ti --detach-keys ctrl-q,ctrl-q fedora sh
```

To make this change the default for all containers, users can modify the
containers.conf file. This can be done simply in your home directory, but adding the
following lines to users containers.conf

```console
$ cat >> ~/.config/containers/containers.conf << _eof
[engine]
detach_keys="ctrl-q,ctrl-q"
_eof
```

In order to effect root running containers and all users, modify the system
wide defaults in `/etc/containers/containers.conf`.


### 23) Container with exposed ports won't run in a pod

A container with ports that have been published with the `--publish` or `-p` option
can not be run within a pod.

#### Symptom

```console
$ podman pod create --name srcview -p 127.0.0.1:3434:3434 -p 127.0.0.1:7080:7080 -p 127.0.0.1:3370:3370                        4b2f4611fa2cbd60b3899b936368c2b3f4f0f68bc8e6593416e0ab8ecb0a3f1d

$ podman run --pod srcview --name src-expose -p 3434:3434 -v "${PWD}:/var/opt/localrepo":Z,ro sourcegraph/src-expose:latest serve /var/opt/localrepo
Error: cannot set port bindings on an existing container network namespace
```

#### Solution

This is a known limitation.  If a container will be run within a pod, it is not necessary
to publish the port for the containers in the pod. The port must only be published by the
pod itself.  Pod network stacks act like the network stack on the host - you have a
variety of containers in the pod, and programs in the container, all sharing a single
interface and IP address, and associated ports. If one container binds to a port, no other
container can use that port within the pod while it is in use. Containers in the pod can
also communicate over localhost by having one container bind to localhost in the pod, and
another connect to that port.

In the example from the symptom section, dropping the `-p 3434:3434` would allow the
`podman run` command to complete, and the container as part of the pod would still have
access to that port.  For example:

```console
$ podman run --pod srcview --name src-expose -v "${PWD}:/var/opt/localrepo":Z,ro sourcegraph/src-expose:latest serve /var/opt/localrepo
```

### 24) Podman container images fail with `fuse: device not found` when run

Some container images require that the fuse kernel module is loaded in the kernel
before they will run with the fuse filesystem in play.

#### Symptom

When trying to run the container images found at quay.io/podman, quay.io/containers
registry.access.redhat.com/ubi8 or other locations, an error will sometimes be returned:

<!-- this would be better if it showed the command being run, and use ```console markup -->
```
ERRO error unmounting /var/lib/containers/storage/overlay/30c058cdadc888177361dd14a7ed7edab441c58525b341df321f07bc11440e68/merged: invalid argument
error mounting container "1ae176ca72b3da7c70af31db7434bcf6f94b07dbc0328bc7e4e8fc9579d0dc2e": error mounting build container "1ae176ca72b3da7c70af31db7434bcf6f94b07dbc0328bc7e4e8fc9579d0dc2e": error creating overlay mount to /var/lib/containers/storage/overlay/30c058cdadc888177361dd14a7ed7edab441c58525b341df321f07bc11440e68/merged: using mount program /usr/bin/fuse-overlayfs: fuse: device not found, try 'modprobe fuse' first
fuse-overlayfs: cannot mount: No such device
: exit status 1
ERRO exit status 1
```

#### Solution

If you encounter a `fuse: device not found` error when running the container image, it is likely that
the fuse kernel module has not been loaded on your host system.  Use the command `modprobe fuse` to load the
module and then run the container image afterwards.  To enable this automatically at boot time, you can add a configuration
file to `/etc/modules.load.d`.  See `man modules-load.d` for more details.

### 25) podman run --rootfs link/to//read/only/dir does not work

An error such as "OCI runtime error" on a read-only filesystem or the error "{image} is not an absolute path or is a symlink" are often times indicators for this issue.  For more details, review this [issue](
https://github.com/containers/podman/issues/5895).

#### Symptom

Rootless Podman requires certain files to exist in a file system in order to run.
Podman will create /etc/resolv.conf, /etc/hosts and other file descriptors on the rootfs in order
to mount volumes on them.

#### Solution

Run the container once in read/write mode, Podman will generate all of the FDs on the rootfs, and
from that point forward you can run with a read-only rootfs.

```console
$ podman run --rm --rootfs /path/to/rootfs true
```

The command above will create all the missing directories needed to run the container.

After that, it can be used in read-only mode, by multiple containers at the same time:

```console
$ podman run --read-only --rootfs /path/to/rootfs ....
```

Another option is to use an Overlay Rootfs Mount:

```console
$ podman run --rootfs /path/to/rootfs:O ....
```

Modifications to the mount point are destroyed when the container
finishes executing, similar to a tmpfs mount point being unmounted.

### 26) Running containers with resource limits fails with a permissions error

On some systemd-based systems, non-root users do not have resource limit delegation
permissions. This causes setting resource limits to fail.

#### Symptom

Running a container with a resource limit options will fail with an error similar to the following:

`--cpus`, `--cpu-period`, `--cpu-quota`, `--cpu-shares`:

    Error: OCI runtime error: crun: the requested cgroup controller `cpu` is not available

`--cpuset-cpus`, `--cpuset-mems`:

    Error: OCI runtime error: crun: the requested cgroup controller `cpuset` is not available

This means that resource limit delegation is not enabled for the current user.

#### Solution

You can verify whether resource limit delegation is enabled by running the following command:

```console
$ cat "/sys/fs/cgroup/user.slice/user-$(id -u).slice/user@$(id -u).service/cgroup.controllers"
```

Example output might be:

    memory pids

In the above example, `cpu` and `cpuset` are not listed, which means the current user does
not have permission to set CPU or CPUSET limits.

If you want to enable CPU or CPUSET limit delegation for all users, you can create the
file `/etc/systemd/system/user@.service.d/delegate.conf` with the contents:

```ini
[Service]
Delegate=memory pids cpu cpuset
```

After logging out and logging back in, you should have permission to set
CPU and CPUSET limits.

### 27) `exec container process '/bin/sh': Exec format error` (or another binary than `bin/sh`)

This can happen when running a container from an image for another architecture than the one you are running on.

For example, if a remote repository only has, and thus send you, a `linux/arm64` _OS/ARCH_ but you run on `linux/amd64` (as happened in https://github.com/openMF/community-app/issues/3323 due to https://github.com/timbru31/docker-ruby-node/issues/564).

### 28) `Error: failed to create sshClient: Connection to bastion host (ssh://user@host:22/run/user/.../podman/podman.sock) failed.: ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain`

In some situations where the client is not on the same machine as where the podman daemon is running the client key could be using a cipher not supported by the host. This indicates an issue with one's SSH config. Until remedied using podman over ssh
with a pre-shared key will be impossible.

#### Symptom

The accepted ciphers per `/etc/crypto-policies/back-ends/openssh.config` are not one that was used to create the public/private key pair that was transferred over to the host for ssh authentication.

You can confirm this is the case by attempting to connect to the host via `podman-remote info` from the client and simultaneously on the host running `journalctl -f` and watching for the error `userauth_pubkey: key type ssh-rsa not in PubkeyAcceptedAlgorithms [preauth]`.

#### Solution

Create a new key using a supported algorithm e.g. ecdsa:

```console
$ ssh-keygen -t ecdsa -f ~/.ssh/podman
```

Then copy the new id over:

```console
$ ssh-copy-id -i ~/.ssh/podman.pub user@host
```

And then re-add the connection (removing the old one if necessary):

```console
$ podman-remote system connection add myuser --identity ~/.ssh/podman ssh://user@host/run/user/1000/podman/podman.sock
```

And now this should work:

```console
$ podman-remote info
```

### 30) Container related firewall rules are lost after reloading firewalld
Container network can't be reached after `firewall-cmd --reload` and `systemctl restart firewalld` Running `podman network reload` will fix it but it has to be done manually.

#### Symptom
The firewall rules created by podman are lost when the firewall is reloaded.

#### Solution
[@ranjithrajaram](https://github.com/containers/podman/issues/5431#issuecomment-847758377) has created a systemd-hook to fix this issue

1) For "firewall-cmd --reload", create a systemd unit file with the following
```ini
[Unit]
Description=firewalld reload hook - run a hook script on firewalld reload
Wants=dbus.service
After=dbus.service

[Service]
Type=simple
ExecStart=/bin/bash -c '/bin/busctl monitor --system --match "interface=org.fedoraproject.FirewallD1,member=Reloaded" --match "interface=org.fedoraproject.FirewallD1,member=PropertiesChanged" | while read -r line ; do podman network reload --all ; done'

[Install]
WantedBy=default.target
```

2) For "systemctl restart firewalld", create a systemd unit file with the following
```ini
[Unit]
Description=podman network reload
Wants=firewalld.service
After=firewalld.service
PartOf=firewalld.service

[Service]
Type=simple
RemainAfterExit=yes
ExecStart=/usr/bin/podman network reload --all

[Install]
WantedBy=default.target
```

However, If you use busctl monitor then you can't get machine-readable output on RHEL 8.
Since it doesn't have `busctl -j` as mentioned here by [@yrro](https://github.com/containers/podman/issues/5431#issuecomment-896943018).

For RHEL 8, you can use the following one-liner bash script.
```ini
[Unit]
Description=Redo podman NAT rules after firewalld starts or reloads
Wants=dbus.service
After=dbus.service
Requires=firewalld.service

[Service]
Type=simple
ExecStart=/bin/bash -c "dbus-monitor --profile --system 'type=signal,sender=org.freedesktop.DBus,path=/org/freedesktop/DBus,interface=org.freedesktop.DBus,member=NameAcquired,arg0=org.fedoraproject.FirewallD1' 'type=signal,path=/org/fedoraproject/FirewallD1,interface=org.fedoraproject.FirewallD1,member=Reloaded' | sed -u '/^#/d' | while read -r type timestamp serial sender destination path interface member _junk; do if [[ $type = '#'* ]]; then continue; elif [[ $interface = org.freedesktop.DBus && $member = NameAcquired ]]; then echo 'firewalld started'; podman network reload --all; elif [[ $interface = org.fedoraproject.FirewallD1 && $member = Reloaded ]]; then echo 'firewalld reloaded'; podman network reload --all; fi; done"
Restart=Always

[Install]
WantedBy=default.target
```
`busctl-monitor` is almost usable in RHEL 8, except that it always outputs two bogus events when it starts up,
one of which is (in its only machine-readable format) indistinguishable from the `NameOwnerChanged` that you get when firewalld starts up.
This means you would get an extra `podman network reload --all` when this unit starts.

Apart from this, you can use the following systemd service with the python3 code.

```ini
[Unit]
Description=Redo podman NAT rules after firewalld starts or reloads
Wants=dbus.service
Requires=firewalld.service
After=dbus.service

[Service]
Type=simple
ExecStart=/usr/bin/python  /path/to/python/code/podman-redo-nat.py
Restart=always

[Install]
WantedBy=default.target
```
The code reloads podman network twice when you use `systemctl restart firewalld`.
```python3
import dbus
from gi.repository import GLib
from dbus.mainloop.glib import DBusGMainLoop
import subprocess
import sys

# I'm a bit confused on the return values in the code
# Not sure if they are needed.

def reload_podman_network():
    try:
        subprocess.run(["podman","network","reload","--all"],timeout=90)
        # I'm not sure about this part
        sys.stdout.write("podman network reload done\n")
        sys.stdout.flush()
    except subprocess.TimeoutExpired as t:
        sys.stderr.write(f"Podman reload failed due to Timeout {t}")
    except subprocess.CalledProcessError as e:
        sys.stderr.write(f"Podman reload failed due to {e}")
    except Exception as e:
        sys.stderr.write(f"Podman reload failed with an Unhandled Exception {e}")

    return False

def signal_handler(*args, **kwargs):
    if kwargs.get('member') == "Reloaded":
        reload_podman_network()
    elif kwargs.get('member') == "NameOwnerChanged":
        reload_podman_network()
    else:
        return None
    return None

def signal_listener():
    try:
        DBusGMainLoop(set_as_default=True)# Define the loop.
        loop = GLib.MainLoop()
        system_bus = dbus.SystemBus()
        # Listens to systemctl restart firewalld with a filter added, will cause podman network to be reloaded twice
        system_bus.add_signal_receiver(signal_handler,dbus_interface='org.freedesktop.DBus',arg0='org.fedoraproject.FirewallD1',member_keyword='member')
        # Listens to firewall-cmd --reload
        system_bus.add_signal_receiver(signal_handler,dbus_interface='org.fedoraproject.FirewallD1',signal_name='Reloaded',member_keyword='member')
        loop.run()
    except KeyboardInterrupt:
        loop.quit()
        sys.exit(0)
    except Exception as e:
        loop.quit()
        sys.stderr.write(f"Error occurred {e}")
        sys.exit(1)

if __name__ == "__main__":
    signal_listener()
```

### 31) Podman run fails with `ERRO[0000] XDG_RUNTIME_DIR directory "/run/user/0" is not owned by the current user` or `Error: creating tmpdir: mkdir /run/user/1000: permission denied`.

A failure is encountered when performing `podman run` with a warning `XDG_RUNTIME_DIR is pointing to a path which is not writable. Most likely podman will fail.`

#### Symptom

A rootless container is being invoked with cgroup configuration as `cgroupv2` for user with missing or invalid **systemd session**.

Example cases
```console
# su user1 -c 'podman images'
ERRO[0000] XDG_RUNTIME_DIR directory "/run/user/0" is not owned by the current user
```
```console
# su - user1 -c 'podman images'
Error: creating tmpdir: mkdir /run/user/1000: permission denied
```

#### Solution

Podman expects a valid login session for the `rootless+cgroupv2` use-case. Podman execution is expected to fail if the login session is not present. In most cases, podman will figure out a solution on its own but if `XDG_RUNTIME_DIR` is pointing to a path that is not writable execution will most likely fail. Typical scenarios of such cases are seen when users are trying to use Podman with `su - <user> -c '<podman-command>'`, or `sudo -l` and badly configured systemd session.

Alternatives:

* Execute Podman via __systemd-run__ that will first start a systemd login session:

  ```console
  $ sudo systemd-run --machine=username@ --quiet --user --collect --pipe --wait podman run --rm docker.io/library/alpine echo hello
  ```
* Start an interactive shell in a systemd login session with the command `machinectl shell <username>@`
  and then run Podman

  ```console
  $ sudo -i
  # machinectl shell username@
  Connected to the local host. Press ^] three times within 1s to exit session.
  $ podman run --rm docker.io/library/alpine echo hello
  ```
* Start a new systemd login session by logging in with `ssh` i.e. `ssh <username>@localhost` and then run Podman.

* Before invoking Podman command create a valid login session for your rootless user using `loginctl enable-linger <username>`

### 32) 127.0.0.1:7777 port already bound

After deleting a VM on macOS, the initialization of subsequent VMs fails.

#### Symptom

After deleting a client VM on macOS via `podman machine stop` && `podman machine rm`, attempting to `podman machine init` a new client VM leads to an error with the 127.0.0.1:7777 port already bound.

#### Solution

You will need to remove the hanging gv-proxy process bound to the port in question. For example, if the port mentioned in the error message is 127.0.0.1:7777, you can use the command `kill -9 $(lsof -i:7777)` in order to identify and remove the hanging process which prevents you from starting a new VM on that default port.

### 33) The sshd process fails to run inside of the container.

#### Symptom

The sshd process running inside the container fails with the error
"Error writing /proc/self/loginuid".

### Solution

If the `/proc/self/loginuid` file is already initialized then the
`CAP_AUDIT_CONTROL` capability is required to override it.

This happens when running Podman from a user session since the
`/proc/self/loginuid` file is already initialized.  The solution is to
run Podman from a system service, either using the Podman service, and
then using podman -remote to start the container or simply by running
something like `systemd-run podman run ...`.  In this case the
container will only need `CAP_AUDIT_WRITE`.

### 34) Container creates a file that is not owned by the user's regular UID

After running a container with rootless Podman, the non-root user sees a numerical UID and GID instead of a username and groupname.

#### Symptom

When listing file permissions with `ls -l` on the host in a directory that was passed as `--volume /some/dir` to `podman run`,
the UID and GID are displayed rather than the corresponding username and groupname. The UID and GID numbers displayed are
from the user's subordinate UID and GID ranges on the host system.

An example

```console
$ mkdir dir1
$ chmod 777 dir1
$ podman run --rm -v ./dir1:/dir1:Z \
             --user 2003:2003 \
             docker.io/library/ubuntu bash -c "touch /dir1/a; chmod 600 /dir1/a"
$ ls -l dir1/a
-rw-------. 1 102002 102002 0 Jan 19 19:35 dir1/a
$ less dir1/a
less: dir1/a: Permission denied
```

#### Solution

If you want to read, chown, or remove such a file, enter a user
namespace. Instead of running commands such as `less dir1/a` or `rm dir1/a`, you
need to prepend the command-line with `podman unshare`, i.e.,
`podman unshare less dir1/a` or `podman unshare rm dir1/a`. To change the ownership
of the file `dir1/a` to your regular user's UID and GID, run `podman unshare chown 0:0 dir1/a`.
A file having the ownership `0:0` in the user namespace is owned by the regular
user on the host. To use Bash features, such as variable expansion and
globbing, you need to wrap the command with `bash -c`, e.g.
`podman unshare bash -c 'ls $HOME/dir1/a*'`.

Would it have been possible to run Podman in another way so that your regular
user would have become the owner of the file? Yes, you can use the option
`--userns keep-id:uid=$uid,gid=$gid` to change how UIDs and GIDs are mapped
between the container and the host. Let's try it out.

In the example above `ls -l` shows the UID 102002 and GID 102002. Set shell variables

```console
$ uid_from_ls=102002
$ gid_from_ls=102002
```

Set shell variables to the lowest subordinate UID and GID

```console
$ lowest_subuid=$(podman info --format "{{ (index .Host.IDMappings.UIDMap 1).HostID }}")
$ lowest_subgid=$(podman info --format "{{ (index .Host.IDMappings.GIDMap 1).HostID }}")
```

Compute the UID and GID inside the container that map to the owner of the created file on the host.

```console
$ uid=$(( $uid_from_ls - $lowest_subuid + 1))
$ gid=$(( $gid_from_ls - $lowest_subgid + 1))
```
(In the computation it was assumed that there is only one subuid range and one subgid range)

```console
$ echo $uid
2003
$ echo $gid
2003
```

The computation shows that the UID is `2003` and the GID is `2003` inside the container.
This comes as no surprise as this is what was specified before with `--user=2003:2003`,
but the same computation could be used whenever a username is specified
or the `--user` option is not used.

Run the container again but now with UIDs and GIDs mapped

```console
$ mkdir dir1
$ chmod 777 dir1
$ podman run --rm
  -v ./dir1:/dir1:Z \
  --user $uid:$gid \
  --userns keep-id:uid=$uid,gid=$gid \
     docker.io/library/ubuntu bash -c "touch /dir1/a; chmod 600 /dir1/a"
$ id -u
tester
$ id -g
tester
$ ls -l dir1/a
-rw-------. 1 tester tester 0 Jan 19 20:31 dir1/a
$
```

In this example the `--user` option specified a rootless user in the container.
As the rootless user could also have been specified in the container image, e.g.

```console
$ podman image inspect --format "user: {{.User}}" IMAGE
user: hpc
```
the same problem could also occur even without specifying `--user`.

Another variant of the same problem could occur when using
`--user=root:root` (the default), but where the root user creates non-root owned files
in some way (e.g by creating them themselves, or switching the effective UID to
a rootless user and then creates files).

See also the troubleshooting tip:

[_Podman run fails with "Error: unrecognized namespace mode keep-id:uid=1000,gid=1000 passed"_](#39-podman-run-fails-with-error-unrecognized-namespace-mode-keep-iduid1000gid1000-passed)

### 35) Passed-in devices or files can't be accessed in rootless container (UID/GID mapping problem)

As a non-root user you have access rights to devices, files and directories that you
want to pass into a rootless container with `--device=...`, `--volume=...` or `--mount=..`.

Podman by default maps a non-root user inside a container to one of the user's
subordinate UIDs and subordinate GIDs on the host. When the container user tries to access a
file, a "Permission denied" error could occur because the container user does not have the
permissions of the regular user of the host.

#### Symptom

* Any access inside the container is rejected with "Permission denied"
for files, directories or devices passed in to the container
with `--device=..`,`--volume=..` or `--mount=..`, e.g.

```console
$ mkdir dir1
$ chmod 700 dir1
$ podman run --rm -v ./dir1:/dir1:Z \
             --user 2003:2003 \
             docker.io/library/ubuntu ls /dir1
ls: cannot open directory '/dir1': Permission denied
```

#### Solution

We follow essentially the same solution as in the previous
troubleshooting tip:

[_Container creates a file that is not owned by the user's regular UID_](#34-container-creates-a-file-that-is-not-owned-by-the-users-regular-uid)

but for this problem the container UID and GID can't be as
easily computed by mere addition and subtraction.

In other words, it might be more challenging to find out the UID and
the GID inside the container that we want to map to the regular
user on the host.

If the `--user` option is used together with a numerical UID and GID
to specify a rootless user, we already know the answer.

If the `--user` option is used together with a username and groupname,
we could look up the UID and GID in the file `/etc/passwd` of the container.

If the container user is not set via `--user` but instead from the
container image, we could inspect the container image

```console
$ podman image inspect --format "user: {{.User}}" IMAGE
user: hpc
```

and then look it up in `/etc/passwd` of the container.

If the problem occurs in a container that is started to run as root but later
switches to an effictive UID of a rootless user, it might be less
straightforward to find out the UID and the GID. Reading the
`Containerfile`, `Dockerfile` or the `/etc/passwd` could give a clue.

To run the container with the rootless container UID and GID mapped to the
user's regular UID and GID on the host follow these steps:

Set the `uid` and `gid` shell variables in a Bash shell to the UID and GID
of the user that will be running inside the container, e.g.

```console
$ uid=2003
$ gid=2003
```

and run

```console
$ mkdir dir1
$ echo hello > dir1/file.txt
$ chmod 700 dir1/file.txt
$ podman run --rm \
  -v ./dir1:/dir1:Z \
  --user $uid:$gid \
  --userns keep-id:uid=$uid,gid=$gid \
  docker.io/library/alpine cat /dir1/file.txt
hello
```

See also the troubleshooting tip:

[_Podman run fails with "Error: unrecognized namespace mode keep-id:uid=1000,gid=1000 passed"_](#39-podman-run-fails-with-error-unrecognized-namespace-mode-keep-iduid1000gid1000-passed)

### 36) Images in the additional stores can be deleted even if there are containers using them

When an image in an additional store is used, it is not locked thus it
can be deleted even if there are containers using it.

#### Symptom

WARN[0000] Can't stat lower layer "/var/lib/containers/storage/overlay/l/7HS76F2P5N73FDUKUQAOJA3WI5" because it does not exist. Going through storage to recreate the missing symlinks.

#### Solution

It is the user responsibility to make sure images in an additional
store are not deleted while being used by containers in another
store.

### 37) Syncing bugfixes for podman-remote or setups using Podman API

After upgrading Podman to a newer version an issue with the earlier version of Podman still presents itself while using podman-remote.

#### Symptom

While running podman remote commands with the most updated Podman, issues that were fixed in a prior version of Podman can arise either on the Podman client side or the Podman server side.

#### Solution

When upgrading Podman to a particular version for the required fixes, users often make the mistake of only upgrading the Podman client. However, suppose a setup uses `podman-remote` or uses a client that communicates with the Podman server on a remote machine via the REST API. In that case, it is required to upgrade both the Podman client and the Podman server running on the remote machine. Both the Podman client and server must be upgraded to the same version.

Example: If a particular bug was fixed in `v4.1.0` then the Podman client must have version `v4.1.0` as well the Podman server must have version `v4.1.0`.

### 38) Unexpected carriage returns are outputted on the terminal

When using the __--tty__ (__-t__) flag, unexpected carriage returns are outputted on the terminal.

#### Symptom

The container program prints a newline (`\n`) but the terminal outputs a carriage return and a newline (`\r\n`).

```
$ podman run --rm -t fedora echo abc | od -c
0000000   a   b   c  \r  \n
0000005
```

When run directly on the host, the result is as expected.

```
$ echo abc | od -c
0000000   a   b   c  \n
0000004
```

Extra carriage returns can also shift the prompt to the right.

```
$ podman run --rm -t fedora sh -c "echo 1; echo 2; echo 3" | cat -A
1^M$
    2^M$
        3^M$
            $
```

#### Solution

Run Podman without the __--tty__ (__-t__) flag.

```
$ podman run --rm fedora echo abc | od -c
0000000   a   b   c  \n
0000004
```

The __--tty__ (__-t__) flag should only be used when the program requires user interaction in the termainal, for instance expecting
the user to type an answer to a question.

Where does the extra carriage return `\r` come from?

The extra `\r` is not outputted by Podman but by the terminal. In fact, a reconfiguration of the terminal can make the extra `\r` go away.

```
$ podman run --rm -t fedora /bin/sh -c "stty -onlcr && echo abc" | od -c
0000000   a   b   c  \n
0000004
```

### 39) Podman run fails with "Error: unrecognized namespace mode keep-id:uid=1000,gid=1000 passed"

Podman 4.3.0 introduced the options _uid_ and _gid_ that can be given to `--userns keep-id` which are not recognized by older versions of Podman.

#### Symptom

When using a Podman version older than 4.3.0, the options _uid_ and _gid_ are not recognized, and an "unrecognized namespace mode" error is raised.

```
$ uid=1000
$ gid=1000
$ podman run --rm
  --user $uid:$gid \
  --userns keep-id:uid=$uid,gid=$gid \
     docker.io/library/ubuntu /bin/cat /proc/self/uid_map
Error: unrecognized namespace mode keep-id:uid=1000,gid=1000 passed
$
```

#### Solution

Use __--uidmap__ and __--gidmap__ options to describe the same UID and GID mapping.

Run

```
$ uid=1000
$ gid=1000
$ subuidSize=$(( $(podman info --format "{{ range \
   .Host.IDMappings.UIDMap }}+{{.Size }}{{end }}" ) - 1 ))
$ subgidSize=$(( $(podman info --format "{{ range \
   .Host.IDMappings.GIDMap }}+{{.Size }}{{end }}" ) - 1 ))
$ podman run --rm \
  --user $uid:$gid \
  --uidmap 0:1:$uid \
  --uidmap $uid:0:1 \
  --uidmap $(($uid+1)):$(($uid+1)):$(($subuidSize-$uid)) \
  --gidmap 0:1:$gid \
  --gidmap $gid:0:1 \
  --gidmap $(($gid+1)):$(($gid+1)):$(($subgidSize-$gid)) \
     docker.io/library/ubuntu /bin/cat /proc/self/uid_map
         0          1       1000
      1000          0          1
      1001       1001      64536
```

which uses the same UID and GID mapping as when specifying
`--userns keep-id:uid=$uid,gid=$gid` with Podman 4.3.0 (or greater)

```
$ uid=1000
$ gid=1000
$ podman run --rm \
  --user $uid:$gid \
  --userns keep-id:uid=$uid,gid=$gid \
     docker.io/library/ubuntu /bin/cat /proc/self/uid_map
         0          1       1000
      1000          0          1
      1001       1001      64536
```

Replace `/bin/cat /proc/self/uid_map` with
`/bin/cat /proc/self/gid_map` to show the GID mapping.

### 40) Podman fails to find expected image with "error locating pulled image", "image not known"

When trying to do a Podman command that pulls an image from local storage or a remote repository,
an error is raised saying "image not known" or "error locating pulled image".  Even though the image
had been verified before the Podman command was invoked.

#### Symptom

After verifying that an image is in place either locally or on a remote repository, a Podman command
referencing that image will fail in a manner like:
```
# ls Containerfile
FROM registry.access.redhat.com/ubi8-minimal:latest
MAINTAINER Podman Community
USER root

# podman build .
STEP 1/2: FROM registry.access.redhat.com/ubi8-minimal
Trying to pull registry.access.redhat.com/ubi8-minimal:latest...
Getting image source signatures
Checking if image destination supports signatures
Copying blob a6577091999b done
Copying config abb1ba1bce done
Writing manifest to image destination
Storing signatures
Error: error creating build container: error locating pulled image "registry.access.redhat.com/ubi8-minimal:latest" name in containers storage: registry.access.redhat.com/ubi8-minimal:latest: image not known
```

#### Solution
The general cause for this is a timing issue.  To make Podman commands as
efficient as possible, read and write locks are only established for critical
sections within the code.  When pulling an image from a repository, a copy of
that image is first written to local storage using a write lock.  This lock is
released before the image is then acquired/read.  If another process does a
harmful command such as `podman system prune --all` or `podman system reset`
or `podman rmi --all`, between the time the image is written and before the
first process can acquire it, this type of `image not known` error can arise.

The maintainers of Podman have considered heavier-duty locks to close this
timing window. However, the slowdown that all Podman commands would encounter
was not considered worth the cost of completely closing this small timing window.

### 41) A podman build step with `--mount=type=secret` fails with "operation not permitted"

Executing a step in a `Dockerfile`/`Containerfile` which mounts secrets using `--mount=type=secret` fails with "operation not permitted" when running on a host filesystem mounted with `nosuid` and when using the `runc` runtime.

#### Symptom

A `RUN` line in the `Dockerfile`/`Containerfile` contains a [secret mount](https://github.com/containers/container-libs/blob/main/common/docs/Containerfile.5.md) such as `--mount=type=secret,id=MY_USER,target=/etc/dnf/vars/MY_USER`.
When running `podman build` the process fails with an error message like:

```
STEP 3/13: RUN --mount=type=secret,id=MY_USER,target=/etc/dnf/vars/MY_USER     --mount=type=secret,id=MY_USER,target=/etc/dnf/vars/MY_USER     ...: time="2023-06-13T18:04:59+02:00" level=error msg="runc create failed: unable to start container process: error during container init: error mounting \"/var/tmp/buildah2251989386/mnt/buildah-bind-target-11\" to rootfs at \"/etc/dnf/vars/MY_USER\": mount /var/tmp/buildah2251989386/mnt/buildah-bind-target-11:/etc/dnf/vars/MY_USER (via /proc/self/fd/7), flags: 0x1021: operation not permitted"
: exit status 1
ERRO[0002] did not get container create message from subprocess: EOF
```

#### Solution

* Install `crun`, e.g. with `dnf install crun`.
* Use the `crun` runtime by passing `--runtime /usr/bin/crun` to `podman build`.

See also [Buildah issue 4228](https://github.com/containers/buildah/issues/4228) for a full discussion of the problem.

### 42) podman-in-podman builds that are file I/0 intensive are very slow

When using the `overlay` storage driver to do a nested `podman build` inside a running container, file I/O operations such as `COPY` of a large amount of data is very slow or can hang completely.

#### Symptom

Using the default `overlay` storage driver, a `COPY`, `ADD`, or an I/O intensive `RUN` line in a `Containerfile` that is run inside another container is very slow or hangs completely when running a `podman build` inside the running parent container.

#### Solution

This could be caused by the child container using `fuse-overlayfs` for writing to `/var/lib/containers/storage`. Writes can be slow with `fuse-overlayfs`. The solution is to use the native `overlay` filesystem by using a local directory on the host system as a volume to `/var/lib/containers/storage` like so: `podman run --privileged --rm -it -v ./nested_storage:/var/lib/containers/storage parent:latest`. Ensure that the base image of `parent:latest` in this example has no contents in `/var/lib/containers/storage` in the image itself for this to work. Once using the native volume, the nested container should not fall back to `fuse-overlayfs` to write files and the nested build will complete much faster.

If you don't have access to the parent run process, such as in a CI environment, then the second option is to change the storage driver to `vfs` in the parent image by changing changing this line in your `storage.conf` file: `driver = "vfs"`. You may have to run `podman system reset` for this to take effect. You know it's changed when `podman info |grep graphDriverName` outputs `graphDriverName: vfs`. This method is slower performance than using the volume method above but is significantly faster than `fuse-overlayfs`

### 43) `podman run --userns=auto` fails with "Error: creating container storage: not enough unused IDs in user namespace"

Using `--userns=auto` when creating new containers does not work as long as any containers exist that were created with `--userns=keep-id` or `--userns=nomap`

#### Symptom

1. Run with `--userns=auto`
   ```
   $ podman run --rm -d --userns=auto alpine sleep 3600
   ```
   The command succeeds.
2. Run with `--userns=auto`
   ```
   $ podman run --rm -d --userns=auto alpine sleep 3600
   ```
   The command succeeds.
3. Run with `--userns=keep-id`
   ```
   $ podman run --rm -d --userns=keep-id alpine sleep 3600
   ```
   The command succeeds.
4. Run with `--userns=auto`
   ```
   $ podman run --rm -d --userns=auto alpine sleep 3600
   ```
   The command fails with the error message
   ```
   Error: creating container storage: not enough unused IDs in user namespace
   ```

#### Solution

Any existing containers that were created using `--userns=keep-id` or `--userns=nomap` must first be deleted before any new container can be created with `--userns=auto`.
By default, the option `--userns=keep-id` allocates all unused IDs in the user namespace.
To avoid this, specify the `size` option, for example `--userns=keep-id:size=2000`

### 44) `sudo podman run --userns=auto` fails with `Cannot find mappings for user "containers"`

When rootful podman is invoked with `--userns=auto`, podman needs to
pick subranges of subuids and subgids for the user namespace of the container.
Rootful podman ensures that the subuid and subgid ranges for such containers
do not overlap, but how can rootful podman prevent other tools
from accidentally using these IDs?

It's not possible to block other tools that are running as root from using these IDs,
but such tools would normally not use subuids and subgids that have already
been assigned to a user in _/etc/subuid_ and _/etc/subgid_.

The username _containers_ on the host has a special function for rootful Podman.
Rootful podman uses the subuids and subgids of the user _containers_ when
running `--userns=auto` containers. The user _containers_ has no need for these
subuids and subgids because no processes should be started as the user _containers_.
In other words, the user _containers_ is a special user that only exists on the system
to reserve _subuids_ and _subgids_ for rootful podman.

_containers_ is the default username but it can be changed by setting the
option `root-auto-userns-user` in the file _/etc/containers/storage.conf_

#### Symptom

Run rootful podman with `--userns=auto`
```
sudo podman run --rm --userns=auto alpine echo hello
```
The command fails with the error message:
```
ERRO[0000] Cannot find mappings for user "containers": no subuid ranges found for user "containers" in /etc/subuid
Error: creating container storage: not enough unused IDs in user namespace
```

The files _/etc/subuid_ and _/etc/subgid_ do not have any lines that start with `containers:`

#### Solution

Add subuid and subgid ranges for the user _containers_ in _/etc/subuid_ and _/etc/subgid_
or provide such ranges with _/etc/nsswitch.conf_.
For details, see [subid(5)](https://man7.org/linux/man-pages/man5/subuid.5.html).

The following steps create the user _containers_ and assigns big subuid and subgid ranges to it:

1. Create the user _containers_
   ```
   sudo useradd --comment "Helper user to reserve subuids and subgids for Podman" \
                --no-create-home \
                --shell /usr/sbin/nologin \
                containers
   ```
2. Check the subuid and subgid ranges of the user _containers_
   ```
   $ grep ^containers: /etc/subuid
   containers:720896:65536
   $ grep ^containers: /etc/subgid
   containers:720896:65536
   ```
   By default __useradd__ assigns 65536 subuids and 65536 subgids to a new user.
   Typically you would like the reserved pool to be bigger than that. The bigger
   the size, the more containers could be started with `sudo podman run --userns=auto ...`
3. Edit the line for the user _containers_ in the files  _/etc/subuid_ and _/etc/subgid_
   to make the ranges bigger. Ensure that the subuid range of the user _containers_ do
   not overlap with any other subuid ranges in the files _/etc/subuid_. Ensure that the
   subgid range of the user _containers_ do not overlap with any other subgid ranges in
   the files _/etc/subgid_.

Test the echo command again

```
sudo podman run --rm --userns=auto alpine echo hello
```

The command succeeds and prints `hello`

### 45) Podman fails with `lsetxattr(label=system_u:object_r:container_file_t:s0) /dir: operation not permitted`

Strict file permissions prevent Podman from modifying SELinux context file labels
in a bind-mounted directory. The error might also occur when using bind-mounted
directories that reside on file systems that do not support XATTR (such as
old versions of NFS).

#### Symptom

1. Create a directory owned by root
   ```
   sudo mkdir /var/test
   ```
2. Give everyone read access to the directory
   ```
   sudo chmod 755 /var/test
   ```
3. Run `ls` directly on the host
   ```
   ls /var/test
   ```
   The command succeeds.
4. Run rootless podman with the directory bind-mounted. Set `z` as volume option.
   ```
   podman run --rm -v /var/test:/dir:z fedora ls /dir
   ```
   The command fails with the following error message:
   ```
   Error: lsetxattr(label=system_u:object_r:container_file_t:s0) /var/test: operation not permitted
   ```
   The error is expected because the unprivileged user does not have the privileges to modify the SELinux
   context of the directory `/var/test`
5. Repeat the previous command but remove the volume option
   ```
   podman run --rm -v /var/test:/dir fedora ls /dir
   ```
   The command is blocked by SELinux and fails with the
   following error message:
   ```
   ls: cannot open directory '/dir': Permission denied
   ```

Note, access to the bind-mounted directory depends on SELinux rules.
For example, bind-mounting the directory `/usr/share` succeeds even though
the user does not have write permissions to the directory. The following
command succeeds:

```
podman run --rm -v /usr/share:/dir fedora ls /dir
```

#### Solution

Alternative 1

Use the `O` flag to mount the directory from the host
as a temporary storage using the overlay file system.

```
podman run --rm -v /var/test:/dir:O fedora ls /dir
```

Modifications to the mount point are destroyed when the container
finishes executing, similar to a tmpfs mount point being unmounted.

The following command succeeds:

```
podman run --rm -v /var/test:/dir:O fedora touch /dir/file
```

The command `touch` creates the file but the file is discarded when the container finishes.

Alternative 2

If the directory `/var/test` resides on a file system that supports XATTR,
set a directory SELinux context that corresponds to the podman volume flag `:z`

```
sudo chcon -R system_u:object_r:container_file_t:s0 /var/test
```

The following command now succeeds

```
podman run --rm -v /var/test:/dir ls /dir
```

Alternative 3

Modify the default SELinux context configuration for the directory `/var/test`
with the command

```
sudo semanage fcontext -a -s system_u -t container_file_t "/var/test(/.*)?"
```

Change the SElinux contexts recursively for files and directories under `/var/test`
to match the new configuration

```
sudo restorecon -R -F -v /var/test
```

Alternative 4

Add the option `--security-opt label=disable` to disable SELinux for the Podman command.

```
podman run --rm --security-opt label=disable -v /var/test:/dir fedora ls /dir
```

### 46) Creating container fails with `Error: cannot specify a new uid/gid map when entering a pod with an infra container: invalid argument`

Specifying `podman run` command-line options related to UID/GID mapping (such as `--userns`, `--uidmap` or `--gidmap`) fails when running in a pod.

#### Symptom

Example 1

```
$ podman pod create pod1
2d99a6c3d736b3b5e9f105e6c89c91ae719f954b8486d54bccb7b94dde697cde
$ podman run --rm --pod pod1 --uidmap 0:0:2000 --name ctr1 alpine sleep 100
Error: cannot specify a new uid/gid map when entering a pod with an infra container: invalid argument
```

Example 2

```
$ podman pod create --uidmap 0:0:2000 pod1
aa445fb0163c43ca5c6400d36557914f36194b9b9917e1d51b7f57719802dc46
$ podman run --rm --pod pod1 --uidmap 0:0:2000 --name ctr1 alpine sleep 100
Error: cannot specify a new uid/gid map when entering a pod with an infra container: invalid argument
```

#### Solution

Alternative 1

When running containers in a pod, any UID/GID mapping should be specified
when creating the pod and not when creating the container.

In other words, options such as `--userns`, `--uidmap`, `--gidmap`, should only be passed
to `podman pod create`.

```
podman pod create --uidmap 0:0:2000 pod1
podman run --rm -d --pod pod1 --name ctr1 alpine sleep 100
podman run --rm -d --pod pod1 --name ctr2 alpine sleep 100
```

Container ctr1 and container ctr2 run with the same UID/GID mapping.

Alternative 2

If the containers can't run with the same UID/GID mapping, run the containers in a custom network
instead of a pod.

Containers in a pod can communicate directly over 127.0.0.1. When using a custom network, the containers
have different IP addresses. Podman provides an internal DNS server that resolves hostnames that are set with
`--name` (quadlet directive `ContainerName=`) or `--network-alias` (quadlet directive `NetworkAlias=`).

Example using pod without specifying UID/GID mapping:

```
podman pod create pod1
podman run --pod pod1 --rm -d docker.io/library/python python3 -m http.server 8080 --bind 127.0.0.1
podman run --pod pod1 --rm docker.io/library/fedora curl -s -S http://127.0.0.1:8080
```

Example using a custom network without specifying UID/GID mapping:

```
podman network create --opt=isolate=strict mynet
podman run --network mynet --rm -d --name ctr1 --network-alias ctr1alias docker.io/library/python python3 -m http.server 8080 --bind 0.0.0.0
podman run --network mynet --rm docker.io/library/fedora curl -s -S http://ctr1:8080
podman run --network mynet --rm docker.io/library/fedora curl -s -S http://ctr1alias:8080
```

As the containers are not running in a pod, it's possible to run them with different UID/GID mappings.
Add `--uidmap 0:0:2000`, `--uidmap 0:0:5000` and  `--uidmap 0:0:3000` to demonstrate
that different UID/GID mappings are possible.

```
podman network create --opt=isolate=strict mynet
podman run --uidmap 0:0:2000 --network mynet --rm -d --name ctr1 --network-alias ctr1alias docker.io/library/python python3 -m http.server 8080 --bind 0.0.0.0
podman run --uidmap 0:0:5000 --network mynet --rm docker.io/library/fedora curl -s -S http://ctr1:8080
podman run --uidmap 0:0:3000 --network mynet --rm docker.io/library/fedora curl -s -S http://ctr1alias:8080
```

### 47) Connecting to published port fails with `Connection reset by peer`

Connecting to a published port fails with `Connection reset by peer` when the
local address of the listening TCP socket is 127.0.0.1 in the container.

#### Symptom

Start a web server that is bound to 127.0.0.1 in a container.
The container name is _test_.

```
bind=127.0.0.1
podman run --rm -d \
           --name test \
           -p 8080:8080 \
           docker.io/library/python \
             python3 -m http.server 8080 --bind $bind
```

Run curl to access the web server

```
$ curl -s -S 127.0.0.1:8080/
curl: (56) Recv failure: Connection reset by peer
```

To show the port and local address of the listening TCP socket
in the container _test_, run

```
$ path=$(podman inspect --format '{{.NetworkSettings.SandboxKey}}' test)
$ echo $path
/run/user/1001/netns/netns-35f6df1f-1622-5d6d-3bbb-70f5d16e09bc
$ podman unshare nsenter -n=$path ss -tln
State   Recv-Q  Send-Q   Local Address:Port   Peer Address:Port
LISTEN  0       5            127.0.0.1:8080        0.0.0.0:*
```

#### Solution

Alternative 1

Let the web server bind to 0.0.0.0 instead of 127.0.0.1

```
bind=0.0.0.0
podman run --rm \
           -d \
           -p 127.0.0.1:8080:8080 \
           docker.io/library/python \
             python3 -m http.server 8080 --bind $bind
```

Run curl to access the web server

```
$ curl -s -S 127.0.0.1:8080/ | head -1
<!DOCTYPE HTML>
```

If you want to also allow access from the internet, use `-p 8080:8080` instead of `-p 127.0.0.1:8080:8080`

Alternative 2

Use [socket activation](https://github.com/containers/podman/blob/main/docs/tutorials/socket_activation.md#socket-activation-of-containers).
This alternative is only possible when the software in the container supports socket activation.

Alternative 3

Use `--network=host`

Start a web server that is bound to 127.0.0.1

```
bind=127.0.0.1
podman run --rm \
           -d \
           --network=host \
           docker.io/library/python \
             python3 -m http.server 8080 --bind $bind
```

Run curl to access the web server

```
$ curl -s -S 127.0.0.1:8080/ | head -1
<!DOCTYPE HTML>
```

Note: the option `-p` should not be provided when using `--network=host`

Note: this alternative is less secure than the other two.
For security considerations regarding using `--network=host`,
see [**podman-run(1)**](https://docs.podman.io/en/latest/markdown/podman-run.1.html#network-mode-net).

### 48) Pasta fails with `Listen failed` or rootless podman fails with `bind: permission denied`

Unprivileged users on a Linux system can not bind to ports below 1024 by default.
This limit can be configured in `/proc/sys/net/ipv4/ip_unprivileged_port_start`

#### Symptom

Pasta does not have the privileges to create a listening socket on a port below 1024.

```
$ cat /proc/sys/net/ipv4/ip_unprivileged_port_start
1024
$ podman run --rm -d -p 80:80 docker.io/library/nginx
Error: pasta failed with exit code 1:
Listen failed for HOST TCP port */80: Permission denied
Couldn't listen on requested TCP ports
```

A similar problem can be seen when using `--network=host`

```
$ cat /proc/sys/net/ipv4/ip_unprivileged_port_start
1024
$ podman run --rm --network=host docker.io/traefik/whoami
2026/05/04 13:54:20 Starting up on port 80
2026/05/04 13:54:20 listen tcp :80: bind: permission denied
```

#### Solution

Configure `ip_unprivileged_port_start` to allow unprivileged users to
bind to port numbers 80 and above.

```
$ sudo sh -c "echo 80 > /proc/sys/net/ipv4/ip_unprivileged_port_start"
$ cat /proc/sys/net/ipv4/ip_unprivileged_port_start
80
$ podman run --rm -d -p 80:80 docker.io/library/nginx
ad9a50a3728bf5d290fd809431a2876285c4dd9e715b70c5d25dec1e2323ff58
```

To permanently set the value `80`, create the file _/etc/sysctl.d/99-mysettings.conf_
with the contents:

```
net.ipv4.ip_unprivileged_port_start=80
```

and reload the configuration

```
sudo sysctl --system
```

### 49) Can't connect to host's main network interface

Connecting from a container to a service that is listening on
host's main network interface fails when using Pasta.

#### Symptom

Show the IP address of the main network interface

```
$ hostname -I
192.168.64.3 fd06:cd88:fba4:8d1:fa83:8dc1:f5fd:d3e0
```

Result: 192.168.64.3

Start a web server that listens on that IP address

```
$ addr=192.168.64.3
$ podman run
    --rm \
    -d \
    --network=host \
    docker.io/library/python \
      python3 -m http.server 8080 --bind $addr
bb662dc47c32f62dc735367dde43ae488fd5b64570c2e4415f0083d6617f5dec
```

Connecting to the service fails when running curl in another container.

```
$ podman run --rm docker.io/library/fedora curl -sS ${addr}:8080
curl: (7) Failed to connect to 192.168.64.3 port 8080 after 0 ms: Could not connect to server
```

#### Solution

Alternative 1

Instead of connecting to the IP address of the host's main network interface,
connect to `host.containers.internal` or `host.docker.internal`

```
$ podman run --rm docker.io/library/fedora curl -sS host.containers.internal:8080 | head -1
<!DOCTYPE HTML>
```

Alternative 2

Instead of connecting to `host.containers.internal` or `host.docker.internal`, you can
also connect to an arbitrary hostname such as `my.example.com`, by adding
the option `--add-host my.example.com:host-gateway`.

```
$ podman run --rm --add-host my.example.com:host-gateway docker.io/library/fedora curl -sS my.example.com:8080 | head -1
<!DOCTYPE HTML>
```

### 50) Cannot enable a quadlet unit

A user wants to configure a container unit service to start automatically on system boot.
The user mistakenly runs `systemctl enable` instead of adding an install section
to the container unit file. The following error message is printed
`Failed to enable unit: Unit` ... `is transient or generated`.

#### Symptom

Create directory

```
mkdir -p ~/.config/containers/systemd
```

Create file `~/.config/containers/systemd/my.container` containing

```
[Container]
Image=docker.io/library/alpine sh -c "sleep inf"
```

Reload the systemd manager

```
systemctl --user daemon-reload
```

Enable linger

```
sudo loginctl enable-linger $USER
```

Despite being incorrect, some users might believe that the following
command is needed

```
systemctl --user enable my.service
```

The command fails with the following error message

```
Failed to enable unit: Unit /run/user/1000/systemd/generator/my.service is transient or generated
```

#### Solution

Quadlets should not be enabled with `systemctl --user enable`, instead add
an install section to the container unit.

```
[Install]
WantedBy=multi-user.target
```

For details,
see [**podman-systemd.unit(5)**](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html#enabling-unit-files).
