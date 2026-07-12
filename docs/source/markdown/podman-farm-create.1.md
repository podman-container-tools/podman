% podman-farm-create 1

## NAME
podman\-farm\-create - Create a new farm

## SYNOPSIS
**podman farm create** *name* [*connections*]

## DESCRIPTION
Create a new farm with connections that Podman knows about which were added via the
*podman system connection add* command.

An empty farm can be created without adding any connections to it. Add or remove
connections from a farm via the *podman farm update* command.

## EXAMPLES

Create the specified farm with no connections:
```
$ podman farm create farm2
```

Create a farm with connections:
```
$ podman farm create farm1 f37 f38
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-farm(1)](podman-farm.1.md)**, **[podman-system-connection(1)](podman-system-connection.1.md)**, **[podman-system-connection-add(1)](podman-system-connection-add.1.md)**

## HISTORY
July 2023, Originally compiled by Urvashi Mohnani (umohnani at redhat dot com)
