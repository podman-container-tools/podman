% podman-system-connection-remove 1

## NAME
podman\-system\-connection\-remove - Delete named destination

## SYNOPSIS
**podman system connection remove** [*options*] *name*

## DESCRIPTION
Delete named ssh destination.

## OPTIONS

#### **--all**, **-a**

Remove all connections.

## EXAMPLE

Remove the specified system connection:
```
$ podman system connection remove production
```

Remove all system connections:
```
$ podman system connection remove --all
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system(1)](podman-system.1.md)**, **[podman-system-connection(1)](podman-system-connection.1.md)**

## HISTORY
July 2020, Originally compiled by Jhon Honce (jhonce at redhat dot com)
