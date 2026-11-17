![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

# Basic Setup and Use of Podman in a Rootless environment.

Prior to allowing users without root privileges to run Podman, the administrator must install or build Podman and complete the following configurations.

## Administrator Actions

### Installing Podman

For installing Podman, see the [installation instructions](https://podman.io/getting-started/installation).

### Building Podman

For building Podman, see the [build instructions](https://podman.io/getting-started/installation#building-from-scratch).

### Networking configuration

A user-mode networking tool for unprivileged network namespaces must be installed on the machine in order for Podman to run in a rootless environment.

Podman uses [pasta](https://passt.top/passt/about/#pasta) (provided by [passt](https://passt.top/passt/about/)) for rootless networking. Pasta fully supports IPv6 and is architecturally secure (runs in a separate process, uses modern Linux mechanisms for isolation etc).

Passt is [available on most Linux distributions](https://passt.top/passt/about/#availability) via their package distribution software such as `yum`, `dnf`, `apt`, `zypper`, etc. under the name `passt`.  If the package is not available, you can build and install `passt` from [its upstream](https://passt.top/passt/about/#try-it).

More details about pasta can be found in [this blog post](https://blog.podman.io/2024/03/podman-5-0-breaking-changes-in-detail/) and in **[podman-network(1)](https://github.com/containers/podman/blob/main/docs/source/markdown/podman-network.1.md#pasta)**.

> [!note]
> pasta's default situation of not being able to communicate between the container and the host has been fixed in Podman 5.3: see [Podman 5.3 changes for improved networking experience with pasta](https://blog.podman.io/2024/10/podman-5-3-changes-for-improved-networking-experience-with-pasta/).

### `/etc/subuid` and `/etc/subgid` configuration

Rootless Podman requires the user running it to have a range of UIDs listed in the files `/etc/subuid` and `/etc/subgid`.  The `shadow-utils` or `newuid` package provides these files on different distributions and they must be installed on the system.  Root privileges are required to add or update entries within these files.  The following is a summary from the [How does rootless Podman work?](https://opensource.com/article/19/2/how-does-rootless-podman-work) article by Dan Walsh on [opensource.com](https://opensource.com)

For each user that will be allowed to create containers, update `/etc/subuid` and `/etc/subgid` for the user with fields that look like the following.  Note that the values for each user must be unique.  If there is overlap, there is a potential for a user to use another user's namespace and they could corrupt it.

```
# cat /etc/subuid
johndoe:100000:65536
test:165536:65536
```

The format of this file is `USERNAME:UID:RANGE`

* username as listed in `/etc/passwd` or in the output of [`getpwent`](https://man7.org/linux/man-pages/man3/getpwent.3.html).
* The initial UID allocated for the user.
* The size of the range of UIDs allocated for the user.

This means the user `johndoe` is allocated UIDs 100000-165535 as well as their standard UID in the `/etc/passwd` file.

Rather than updating the files directly, the `usermod` program can be used to assign UIDs and GIDs to a user.

```
# usermod --add-subuids 100000-165535 --add-subgids 100000-165535 johndoe
grep johndoe /etc/subuid /etc/subgid
/etc/subuid:johndoe:100000:65536
/etc/subgid:johndoe:100000:65536
```

If you update either `/etc/subuid` or `/etc/subgid`, you need to stop all the running containers owned by the user and kill the pause process that is running on the system for that user.  This can be done automatically by running [`podman system migrate`](https://github.com/containers/podman/blob/main/docs/source/markdown/podman-system-migrate.1.md) as that user.

NOTE: Starting with shadow-utils 4.9, pluggable data sources for subid ranges can be configured via `/etc/nsswitch.conf`. SSSD provides a plugin (`libsubid_sss.so`) that can retrieve subordinate ID ranges from a central identity server. Instead of managing local `/etc/subuid` and `/etc/subgid` files. To enable this, configure `/etc/nsswitch.conf` with `subid: sss`. SSSD 2.6.0 added support for the IPA provider, and SSSD 2.12.0 extended this to the generic LDAP provider. For more details on centrally managed subordinate IDs with FreeIPA, see the [FreeIPA subordinate IDs documentation](https://freeipa.readthedocs.io/en/latest/designs/subordinate-ids.html).

#### Giving access to additional groups

Users can fully map additional groups to a container namespace if
those groups subordinated to the user:

```
# usermod --add-subgids 2000-2000 johndoe
grep johndoe /etc/subgid
```

This means the user `johndoe` can "impersonate" the group `2000` inside the
container. Note that it is usually not a good idea to subordinate active
user ids to other users, because it would allow user impersonation.

`johndoe` can use `--group-add keep-groups` to preserve the additional
groups, and `--gidmap="+g102000:@2000"` to map the group `2000` in the host
to the group `102000` in the container:

```
$ podman run \
  --rm \
  --group-add keep-groups \
  --gidmap="+g102000:@2000" \
  --volume "$PWD:/data:ro" \
  --workdir /data \
  alpine ls -lisa
```

### Enable unprivileged `ping`

(It is very unlikely that you will need to do this on a modern distro).

Users running in a non-privileged container may not be able to use the `ping` utility from that container.

If this is required, the administrator must verify that the UID of the user is part of the range in the `/proc/sys/net/ipv4/ping_group_range` file.

To change its value the administrator can use a call similar to: `sysctl -w "net.ipv4.ping_group_range=0 2000000"`.

To make the change persist, the administrator will need to add a file with the `.conf` file extension in `/etc/sysctl.d` that contains `net.ipv4.ping_group_range=0 $MAX_GID`, where `$MAX_GID` is the highest assignable GID of the user running the container.


## User Actions

The majority of the work necessary to run Podman in a rootless environment is on the shoulders of the machine’s administrator.

Once the Administrator has completed the setup on the machine and then the configurations for the user in `/etc/subuid` and `/etc/subgid`, the user can just start using any Podman command that they wish.

### User Configuration Files

The Podman configuration files for root reside in `/usr/share/containers` with overrides in `/etc/containers`.  In the rootless environment they reside in `${XDG_CONFIG_HOME}/containers` and are owned by each individual user.

Note: in environments without `XDG` environment variables, Podman internally sets the following defaults:

- `$XDG_CONFIG_HOME` = `$HOME/.config`
- `$XDG_DATA_HOME` = `$HOME/.local/share`
- `$XDG_RUNTIME_DIR` =
  - `/run/user/$UID` on `systemd` environments
  - `$TMPDIR/podman-run-$UID` otherwise

The three main configuration files are [containers.conf](https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md), [storage.conf](https://github.com/containers/storage/blob/main/docs/containers-storage.conf.5.md) and [registries.conf](https://github.com/containers/image/blob/main/docs/containers-registries.conf.5.md). The user can modify these files as they wish.

#### containers.conf
Podman reads

1. `/usr/share/containers/containers.conf`
2. `/etc/containers/containers.conf`
3. `${XDG_CONFIG_HOME}/containers/containers.conf`

if they exist, in that order. Each file can override the previous for particular fields.

#### storage.conf
For `storage.conf` the order is

1. `/etc/containers/storage.conf`
2. `${XDG_CONFIG_HOME}/containers/storage.conf`

In rootless Podman, certain fields in `/etc/containers/storage.conf` are ignored. These fields are:
```
graphroot=""
 container storage graph dir (default: "/var/lib/containers/storage")
 Default directory to store all writable content created by container storage programs.

runroot=""
 container storage run dir (default: "/run/containers/storage")
 Default directory to store all temporary writable content created by container storage programs.
```
In rootless Podman these fields default to
```
graphroot="${XDG_DATA_HOME}/containers/storage"
runroot="${XDG_RUNTIME_DIR}/containers"
```
[\$XDG_RUNTIME_DIR](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html#variables) defaults on most systems to `/run/user/$UID`.

#### registries
Registry configuration is read in this order

1. `/etc/containers/registries.conf`
2. `/etc/containers/registries.d/*`
3. `${XDG_CONFIG_HOME}/containers/registries.conf`

The files in the home directory should be used to configure rootless Podman for personal needs. These files are not created by default. Users can copy the files from `/usr/share/containers` or `/etc/containers` and modify them.

#### Authorization files
The default authorization file used by the `podman login` and `podman logout` commands is `${XDG_RUNTIME_DIR}/containers/auth.json`.

### Using volumes

Rootless Podman is not, and will never be, root; it's not a `setuid` binary, and gains no privileges when it runs. Instead, Podman makes use of a user namespace to shift the UIDs and GIDs of a block of users it is given access to on the host (via the `newuidmap` and `newgidmap` executables) and your own user within the containers that Podman creates.

If your container runs with the root user, then `root` in the container is actually your user on the host. UID/GID 1 is the first UID/GID specified in your user's mapping in `/etc/subuid` and `/etc/subgid`, etc. If you mount a directory from the host into a container as a rootless user, and create a file in that directory as root in the container, you'll see it's actually owned by your user on the host.

So, for example,

```
host$ whoami
john

# a folder which is empty
host$ ls /home/john/folder
host$ podman run -it -v /home/john/folder:/container/volume mycontainer /bin/bash

# Now I'm in the container
root@container# whoami
root
root@container# touch /container/volume/test
root@container# ls -l /container/volume
total 0
-rw-r--r-- 1 root root 0 May 20 21:47 test
root@container# exit

# I check again
host$ ls -l /home/john/folder
total 0
-rw-r--r-- 1 john john 0 May 20 21:47 test
```

We do recognize that this doesn't really match how many people intend to use rootless Podman - they want their UID inside and outside the container to match. Thus, we provide the `--userns=keep-id` flag, which ensures that your user is mapped to its own UID and GID inside the container.

It is also helpful to distinguish between running Podman as a rootless user, and a container which is built to run rootless. If the container you're trying to run has a `USER` which is not root, then when mounting volumes you **must** use `--userns=keep-id`. This is because the container user would not be able to become `root` and access the mounted volumes.

Another consideration in regards to volumes:

- When providing the path of a directory you'd like to bind-mount, the path needs to be provided as an absolute path
  or a relative path that starts with `.` (a dot), otherwise the string will be interpreted as the name of a named volume.

#### Permission denied on a bind-mount source

Inside the user namespace the process setting up the mount is `root`, but `CAP_DAC_OVERRIDE` only bypasses a file's mode when that file's user ID and group ID both have valid mappings in the namespace. That is the rule under "Operation of file-related capabilities" in **[user_namespaces(7)](https://man7.org/linux/man-pages/man7/user_namespaces.7.html)**. A directory owned by an unmapped ID is reported with the overflow ID, 65534 by default, and nothing overrides its mode.

World permissions are checked first, so this only comes up when the mode alone does not let you through. In each case below the parent is owned as shown, the bind mount source is the child inside it, and `/etc/subuid` has `johndoe:100000:65536`:

```
# parent mode 755, owned by root: the mode already allows it, mapping never comes up
host$ podman run --rm -v /tmp/open/child:/mnt alpine echo ok
ok

# parent mode 700, owned by root: root is not mapped, so nothing overrides the mode
host$ podman run --rm -v /tmp/private/child:/mnt alpine echo ok
Error: statfs /tmp/private/child: permission denied

# parent mode 700, owned by 100999, which the range maps to UID 1000 in the namespace
host$ podman run --rm -v /tmp/mapped/child:/mnt alpine echo ok
ok
```

`podman unshare ls -ldn` on the parent shows which case you are in: an owner of 65534 there means the ID is not mapped. `--userns=keep-id` does not change this, it changes which UID you are inside the container rather than which host IDs the namespace maps.

This is the file side of the warning above about subordinating active user ids: a range that covers UIDs real accounts use gives that user the override on their files in any namespace they create, which is why default ranges start at 100000.

## More information

If you are still experiencing problems running Podman in a rootless environment, please refer to the [Shortcomings of Rootless Podman](https://github.com/containers/podman/blob/main/rootless.md) page which lists known issues and solutions to known issues in this environment.

For more information on Podman and its subcommands, follow the links on the main [README.md](../../README.md#podman-information-for-developers) page or the [podman.io](https://podman.io) web site.
