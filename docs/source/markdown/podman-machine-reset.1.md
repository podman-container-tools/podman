% podman-machine-reset 1

## NAME
podman\-machine\-reset - Reset Podman machines and environment

## SYNOPSIS
**podman machine reset** [*options*]

## DESCRIPTION

Reset your Podman machine environment.  This command stops any running machines
and then removes them.  Configuration and data files are then removed.  Data files
would include machine disk images and any previously pulled cache images.  When
this command is run, all of your Podman machines will have been deleted.

## OPTIONS

#### **--force**, **-f**

Reset without confirmation.

#### **--help**

Print usage statement.


## EXAMPLES

```
$ podman machine reset
Warning: this command will delete all existing podman machines
and all of the configuration and data directories for Podman machines

The following machine(s) will be deleted:

dev
podman-machine-default

Are you sure you want to continue? [y/N] y
$
```

Force reset without confirmation.
```
$ podman machine reset --force
$
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
Feb 2024, Originally compiled by Brent Baude<bbaude@redhat.com>
