% podman-system-connection-default 1

## NAME
podman\-system\-connection\-default - Set named destination as default for the Podman service

## SYNOPSIS
**podman system connection default** *name*

## DESCRIPTION
Set named ssh destination as default destination for the Podman service.

## EXAMPLE

Set the specified connection as default:
```
$ podman system connection default production
```
## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system(1)](podman-system.1.md)**, **[podman-system-connection(1)](podman-system-connection.1.md)**

## HISTORY
July 2020, Originally compiled by Jhon Honce (jhonce at redhat dot com)
