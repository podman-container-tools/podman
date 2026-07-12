% podman-container-cleanup 1

## NAME
podman\-container\-cleanup - Clean up the container's network and mountpoints

## SYNOPSIS
**podman container cleanup** [*options*] *container* [*container* ...]

## DESCRIPTION
**podman container cleanup** cleans up exited *containers* by removing all mountpoints and network configurations from the host. The *container name* or *ID* can be used. The cleanup command does not remove the *containers*. Running *containers* are not cleaned up.\
Sometimes container mount points and network stacks can remain if the podman command was killed or the *container* ran in daemon mode. This command is automatically executed when *containers* are run in daemon mode by the `conmon process` when the *container* exits.

## OPTIONS
#### **--all**, **-a**

Clean up all *containers*.\
The default is **false**.\
*IMPORTANT: This OPTION does not need a container name or ID as input argument.*

#### **--exec**=*session*

Clean up an exec session for a single *container*.
It can only be specified if a single *container* is being cleaned up (conflicts with **--all** as such). If **--rm** is not specified, temporary files for the exec session are cleaned up; if it is, the exec session is removed from the *container*.\
*IMPORTANT: Conflicts with **--rmi** as the container is not being cleaned up so the image cannot be removed.*

#### **--latest**, **-l**

Instead of providing the *container ID* or *name*, use the last created *container*. The default is **false**.
*IMPORTANT: This OPTION is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines. This OPTION does not need a container name or ID as input argument.*

#### **--rm**

After cleanup, remove the *container* entirely.\
The default is **false**.

#### **--rmi**

After cleanup, remove the image entirely.\
The default is **false**.

## EXAMPLES
Clean up the container "mywebserver".
```
$ podman container cleanup mywebserver
```

Clean up the containers with the names "mywebserver", "myflaskserver", "860a4b23".
```
$ podman container cleanup mywebserver myflaskserver 860a4b23
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-container(1)](podman-container.1.md)**, **[conmon(8)](https://github.com/containers/conmon/blob/main/docs/conmon.8.md)**

## HISTORY
Jun 2018, Originally compiled by Dan Walsh <dwalsh@redhat.com>
