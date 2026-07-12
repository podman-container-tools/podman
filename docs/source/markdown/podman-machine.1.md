% podman-machine 1

## NAME
podman\-machine - Manage Podman's virtual machine

## SYNOPSIS
**podman machine** *subcommand*

## DESCRIPTION
`podman machine` is a set of subcommands that manage Podman's virtual machine.

Podman on MacOS and Windows requires a virtual machine. This is because containers are Linux -
containers do not run on any other OS because containers' core functionality are
tied to the Linux kernel. Podman machine must be used to manage MacOS and Windows machines,
but can be optionally used on Linux.

All `podman machine` commands are rootless only.

NOTE: The podman-machine configuration file is managed under the
`$XDG_CONFIG_HOME/containers/podman/machine/` directory. Changing the `$XDG_CONFIG_HOME`
environment variable while the machines are running can lead to unexpected behavior.

Podman machine behaviour can be modified via the [machine] section in the containers.conf(5) file.

Podman is based on virtual machine providers. The following table describes which providers are
supported by platform.  The asterisk denotes the default provider for the platform.

| Platform | Provider |
| --------  |----------|
| Linux | qemu*    |
| MacOS | libkrun* |
| MacOS | applehv  |
| Windows | wsl*     |
| Windows | hyperv   |

## SUBCOMMANDS

| Command | Man Page                                                 | Description                                                     |
|---------|----------------------------------------------------------|-----------------------------------------------------------------|
| cp      | [podman-machine-cp(1)](podman-machine-cp.1.md)           | Securely copy contents between the host and the virtual machine |
| info    | [podman-machine-info(1)](podman-machine-info.1.md)       | Display machine host info                                       |
| init    | [podman-machine-init(1)](podman-machine-init.1.md)       | Initialize a new virtual machine                                |
| inspect | [podman-machine-inspect(1)](podman-machine-inspect.1.md) | Inspect one or more virtual machines                            |
| list    | [podman-machine-list(1)](podman-machine-list.1.md)       | List virtual machines                                           |
| os      | [podman-machine-os(1)](podman-machine-os.1.md)           | Manage a Podman virtual machine's OS                            |
| reset   | [podman-machine-reset(1)](podman-machine-reset.1.md)     | Reset Podman machines and environment                           |
| rm      | [podman-machine-rm(1)](podman-machine-rm.1.md)           | Remove a virtual machine                                        |
| set     | [podman-machine-set(1)](podman-machine-set.1.md)         | Set a virtual machine setting                                   |
| ssh     | [podman-machine-ssh(1)](podman-machine-ssh.1.md)         | SSH into a virtual machine                                      |
| start   | [podman-machine-start(1)](podman-machine-start.1.md)     | Start a virtual machine                                         |
| stop    | [podman-machine-stop(1)](podman-machine-stop.1.md)       | Stop a virtual machine                                          |

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine-cp(1)](podman-machine-cp.1.md)**, **[podman-machine-info(1)](podman-machine-info.1.md)**, **[podman-machine-init(1)](podman-machine-init.1.md)**, **[podman-machine-list(1)](podman-machine-list.1.md)**, **[podman-machine-os(1)](podman-machine-os.1.md)**, **[podman-machine-rm(1)](podman-machine-rm.1.md)**, **[podman-machine-ssh(1)](podman-machine-ssh.1.md)**, **[podman-machine-start(1)](podman-machine-start.1.md)**, **[podman-machine-stop(1)](podman-machine-stop.1.md)**, **[podman-machine-inspect(1)](podman-machine-inspect.1.md)**, **[podman-machine-reset(1)](podman-machine-reset.1.md)**, **[containers.conf(5)](https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md)**

### Troubleshooting

See [podman-troubleshooting(7)](https://github.com/containers/podman/blob/main/troubleshooting.md)
for solutions to common issues.

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
