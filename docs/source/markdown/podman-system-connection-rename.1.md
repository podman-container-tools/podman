% podman-system-connection-rename 1

## NAME
podman\-system\-connection\-rename - Rename the destination for Podman service

## SYNOPSIS
**podman system connection rename** *old* *new*

## DESCRIPTION
Rename ssh destination from *old* to *new*.

## EXAMPLE

Rename the specified connection:
```
$ podman system connection rename laptop devel
```
## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system(1)](podman-system.1.md)**, **[podman-system-connection(1)](podman-system-connection.1.md)**

## HISTORY
July 2020, Originally compiled by Jhon Honce (jhonce at redhat dot com)
