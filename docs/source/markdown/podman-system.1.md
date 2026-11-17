% podman-system 1

## NAME
podman\-system - Manage podman

## SYNOPSIS
**podman system** *subcommand*

## DESCRIPTION
The system command allows management of the podman systems

## COMMANDS

| Command    | Man Page                                                     | Description                                                              |
| -------    | ------------------------------------------------------------ | ------------------------------------------------------------------------ |
| check      | [podman-system-check(1)](podman-system-check.1.md)           | Perform consistency checks on image and container storage.
| connection | [podman-system-connection(1)](podman-system-connection.1.md) | Manage the destination(s) for Podman service(s)                          |
| hyperv-prep| [podman-system-hyperv-prep(1)](podman-system-hyperv-prep.1.md) | A Windows administrator command to prepare a host that is going to run Hyper-V based Podman machines |
| df         | [podman-system-df(1)](podman-system-df.1.md)                 | Show podman disk usage.                                                  |
| events     | [podman-events(1)](podman-events.1.md)                       | Monitor Podman events                                                    |
| info       | [podman-info(1)](podman-info.1.md)                           | Display Podman related system information.                               |
| migrate    | [podman-system-migrate(1)](podman-system-migrate.1.md)       | Migrate existing containers to a new podman version.                     |
| prune      | [podman-system-prune(1)](podman-system-prune.1.md)           | Remove all unused pods, containers, images, networks, and volume data.   |
| renumber   | [podman-system-renumber(1)](podman-system-renumber.1.md)     | Migrate lock numbers to handle a change in maximum number of locks.      |
| reset      | [podman-system-reset(1)](podman-system-reset.1.md)           | Reset storage back to initial state.                                     |
| service    | [podman-system-service(1)](podman-system-service.1.md)       | Run an API service                                                       |

## SEE ALSO
**[podman(1)](podman.1.md)**
