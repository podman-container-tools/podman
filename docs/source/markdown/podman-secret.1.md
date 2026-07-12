% podman-secret 1

## NAME
podman\-secret - Manage podman secrets

## SYNOPSIS
**podman secret** *subcommand*

## DESCRIPTION
podman secret is a set of subcommands that manage secrets.

## SUBCOMMANDS

| Command | Man Page                                               | Description                                            |
| ------- | ------------------------------------------------------ | ------------------------------------------------------ |
| create  | [podman-secret-create(1)](podman-secret-create.1.md)   | Create a new secret                                    |
| exists  | [podman-secret-exists(1)](podman-secret-exists.1.md)   | Check if the given secret exists                       |
| inspect | [podman-secret-inspect(1)](podman-secret-inspect.1.md) | Display detailed information on one or more secrets    |
| ls      | [podman-secret-ls(1)](podman-secret-ls.1.md)           | List all available secrets                             |
| rm      | [podman-secret-rm(1)](podman-secret-rm.1.md)           | Remove one or more secrets                             |

## SEE ALSO
**[podman(1)](podman.1.md)**

## HISTORY
January 2021, Originally compiled by Ashley Cui <acui@redhat.com>
