% podman-machine-rm 1

## NAME
podman\-machine\-rm - Remove a virtual machine

## SYNOPSIS
**podman machine rm** [*options*] [*name*]

## DESCRIPTION

Remove a virtual machine and its related files.  What is actually deleted
depends on the virtual machine type.  For all virtual machines, the generated
podman system connections are deleted.  The ignition files
generated for that VM are also removed as is its image file on the filesystem.

Users get a display of what is deleted and are required to confirm unless the option `--force`
is used.

The default machine name is `podman-machine-default`. If a machine name is not specified as an argument,
then `podman-machine-default` will be removed.

Rootless only.

## OPTIONS

#### **--force**, **-f**

Stop and delete without confirmation.

#### **--help**

Print usage statement.

#### **--save-ignition**

Do not delete the generated ignition file.

#### **--save-image**

Do not delete the VM image.

## EXAMPLES

Remove the specified Podman machine.
```
$ podman machine rm test1

The following files will be deleted:

/home/user/.config/containers/podman/machine/qemu/test1.ign
/home/user/.local/share/containers/podman/machine/qemu/test1_fedora-coreos-33.20210315.1.0-qemu.x86_64.qcow2
/home/user/.config/containers/podman/machine/qemu/test1.json

Are you sure you want to continue? [y/N] y
```

Remove the specified Podman machine even if it is running.
```
$ podman machine rm -f test1
$
```
## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
