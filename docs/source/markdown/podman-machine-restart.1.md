% podman-machine-restart 1

## NAME
podman\-machine\-restart - Restart a virtual machine

## SYNOPSIS
**podman machine restart** [*name*]

## DESCRIPTION

Restarts a virtual machine for Podman.

The default machine name is `podman-machine-default`. If a machine name is not specified as an argument,
then `podman-machine-default` will be restarted.

Stopping an already stopped vm is not considered an error so running restart on a stopped vm just
starts it from a stopped state.

**podman machine restart** stops and then starts a Linux virtual machine where containers are run.

## OPTIONS

#### **--help**

Print usage statement.

## EXAMPLES

Restart a podman machine named myvm.
```
$ podman machine restart myvm
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
