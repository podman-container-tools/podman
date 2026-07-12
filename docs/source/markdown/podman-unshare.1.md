% podman-unshare 1

## NAME
podman\-unshare - Run a command inside of a modified user namespace

## SYNOPSIS
**podman unshare** [*options*] [*command*]

## DESCRIPTION
Launches a process (by default, *$SHELL*) in a new user namespace. The user
namespace is configured so that the invoking user's UID and primary GID appear
to be UID 0 and GID 0, respectively.  Any ranges which match that user and
group in `/etc/subuid` and `/etc/subgid` are also mapped in as themselves with the
help of the *newuidmap(1)* and *newgidmap(1)* helpers.

**podman unshare** is useful for troubleshooting unprivileged operations and for
manually clearing storage and other data related to images and containers.

It is also useful to use the **podman mount** command.  If an unprivileged user wants to mount and work with a container, then they need to execute
**podman unshare**.  Executing **podman mount** fails for unprivileged users unless the user is running inside a **podman unshare** session.

The unshare session defines two environment variables:

- **CONTAINERS_GRAPHROOT**: the path to the persistent container's data.
- **CONTAINERS_RUNROOT**: the path to the volatile container's data.

*IMPORTANT: This command is not available with the remote Podman client.*

## OPTIONS

#### **--help**, **-h**

Print usage statement

#### **--rootless-netns**

Join the rootless network namespace used for netavark networking. It can be used to
connect to a rootless container via IP address (bridge networking). This is otherwise
not possible from the host network namespace.

## Exit Codes

The exit code from `podman unshare` gives information about why the container
failed to run or why it exited.  When `podman unshare` commands exit with a non-zero code,
the exit codes follow the `chroot` standard, see below:

  **125** The error is with podman **_itself_**

    $ podman unshare --foo; echo $?
    Error: unknown flag: --foo
    125

  **126** Executing a _contained command_ and the _command_ cannot be invoked

    $ podman unshare /etc; echo $?
    Error: fork/exec /etc: permission denied
    126

  **127** Executing a _contained command_ and the _command_ cannot be found

    $ podman unshare foo; echo $?
    Error: fork/exec /usr/bin/bogus: no such file or directory
    127

  **Exit code** _contained command_ exit code

    $ podman unshare /bin/sh -c 'exit 3'; echo $?
    3

## EXAMPLE

Execute specified command in rootless user namespace:
```
$ podman unshare id
uid=0(root) gid=0(root) groups=0(root),65534(nobody)
```

Show user namespace mappings for rootless containers:
```
$ podman unshare cat /proc/self/uid_map /proc/self/gid_map
         0       1000          1
         1      10000      65536
         0       1000          1
         1      10000      65536
```


Show rootless netns information in user namespace for rootless containers:
```
$ podman unshare --rootless-netns ip addr
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host
       valid_lft forever preferred_lft forever
2: tap0: <BROADCAST,UP,LOWER_UP> mtu 65520 qdisc fq_codel state UNKNOWN group default qlen 1000
    link/ether aa:8c:0b:73:98:f6 brd ff:ff:ff:ff:ff:ff
    inet 10.0.2.100/24 brd 10.0.2.255 scope global tap0
       valid_lft forever preferred_lft forever
    inet6 fd00::a88c:bff:fe73:98f6/64 scope global dynamic mngtmpaddr
       valid_lft 86389sec preferred_lft 14389sec
    inet6 fe80::a88c:bff:fe73:98f6/64 scope link
       valid_lft forever preferred_lft forever
```


## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-mount(1)](podman-mount.1.md)**, **namespaces(7)**, **newuidmap(1)**, **newgidmap(1)**, **user\_namespaces(7)**
