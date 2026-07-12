% podman-system-connection 1

## NAME
podman\-system\-connection - Manage the destination(s) for Podman service(s)

## SYNOPSIS
**podman system connection** *subcommand*

## DESCRIPTION
Manage the destination(s) for Podman service(s).

The user is prompted for the ssh login password or key file passphrase as required. The `ssh-agent` is supported if it is running.

Podman manages the system connection by writing and reading the `podman-connections.json` file located under
`$XDG_CONFIG_HOME/containers` or if the env is not set it defaults to `$HOME/.config/containers`.
Or the `PODMAN_CONNECTIONS_CONF` environment variable can be set to a full file path which podman
will use instead.
This file is managed by the podman commands and should never be edited by users directly. To manually
configure the connections use `service_destinations` in containers.conf.

If the ReadWrite column in the **podman system connection list** output is set to true the connection is
stored in the `podman-connections.json` file otherwise it is stored in containers.conf and can therefore
not be edited with the **podman system connection** commands.

## COMMANDS

| Command  | Man Page                                                                      | Description                                                |
| -------- | ----------------------------------------------------------------------------- | ---------------------------------------------------------- |
| add      | [podman-system-connection\-add(1)](podman-system-connection-add.1.md)         | Record destination for the Podman service                  |
| default  | [podman-system-connection\-default(1)](podman-system-connection-default.1.md) | Set named destination as default for the Podman service    |
| list     | [podman-system-connection\-list(1)](podman-system-connection-list.1.md)       | List the destination for the Podman service(s)             |
| remove   | [podman-system-connection\-remove(1)](podman-system-connection-remove.1.md)   | Delete named destination                                   |
| rename   | [podman-system-connection\-rename(1)](podman-system-connection-rename.1.md)   | Rename the destination for Podman service                  |

## EXAMPLE

List system connections:
```
$ podman system connection list
Name URI                                           Identity	      Default  ReadWrite
devl ssh://root@example.com/run/podman/podman.sock ~/.ssh/id_rsa  true     true
```
## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system(1)](podman-system.1.md)**

## HISTORY
June 2020, Originally compiled by Jhon Honce (jhonce at redhat dot com)
