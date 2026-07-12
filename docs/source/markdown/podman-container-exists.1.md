% podman-container-exists 1

## NAME
podman\-container\-exists - Check if a container exists in local storage

## SYNOPSIS
**podman container exists** [*options*] *container*

## DESCRIPTION
**podman container exists** checks if a container exists in local storage. The *container ID* or *name* is used as input. Podman returns an exit code
of `0` when the container is found.  A `1` is returned otherwise. An exit code of `125` indicates there was an issue accessing the local storage.

## OPTIONS
#### **--external**

Check for external *containers* as well as Podman *containers*. These external *containers* are generally created via other container technology such as `Buildah` or `CRI-O`.\
The default is **false**.

**-h**, **--help**

Prints usage statement.\
The default is **false**.

## EXAMPLES

Check if a container called "webclient" exists in local storage. Here, the container does exist.
```
$ podman container exists webclient
$ echo $?
0
```

Check if a container called "webbackend" exists in local storage. Here, the container does not exist.
```
$ podman container exists webbackend
$ echo $?
1
```

Check if a container called "ubi8-working-container" created via Buildah exists in local storage. Here, the container does not exist.
```
$ podman container exists --external ubi8-working-container
$ echo $?
1
```

## SEE ALSO
**[podman(1)](podman.1.md)**

## HISTORY
November 2018, Originally compiled by Brent Baude <bbaude@redhat.com>
