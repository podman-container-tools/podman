% podman-machine-os-apply 1

## NAME
podman\-machine\-os\-apply - Apply an OCI image to a Podman Machine's OS

## SYNOPSIS
**podman machine os apply** [*options*] *uri* [vm]

## DESCRIPTION

Apply machine OS changes from an OCI image.

The Podman virtual machine has the capability to rebase itself from the content of an OCI image.
`podman machine image apply` takes an OCI image with container native ostree functionality and rebases itself on that image.

By default, Podman machines on Mac, Linux, and Windows Hyper-V use a customized rpm-ostree based distribution (Fedora CoreOS). Machines based on Microsoft WSL use a
customized distribution and cannot be updated with this command.

Note: WSL-based machines are upgradable by using the `podman machine ssh <machine_name>` command followed by `sudo dnf update`.  This can, however, result in unexpected results in
Podman client and server version differences.

The applying of the OCI image is done by a command called `bootc` and specifically `bootc switch`.  By default, this command
takes an OCI registry image reference like `quay.io/custom/machine-os:latest`.  However, `bootc` also
understands references with different transports. At present, Podman will support the following transports:

* containers-storage
* oci
* oci-archive
* registry

Examples for these transports in URI form are provided below.

Podman machine images are stored as OCI images at `quay.io/podman/machine-os`. When applying an image using this
command, the fully qualified OCI reference name must be used including tag where the tag is the
version of Podman that is inside the VM. By default, Podman will attempt to pull only the statement
version as itself.

The default machine name is `podman-machine-default`. If a machine name is not specified as an argument,
then the OS changes will be applied to `podman-machine-default`.

## OPTIONS

#### **--help**

Print usage statement.

#### **--restart**

Restart VM after applying changes.

## EXAMPLES

Update the default Podman machine to the latest development version of the
bootable OCI image.

Note: This may result in having a newer Podman version inside the machine
than the client.  Unexpected results may occur.

Apply a new custom operating system from an OCI bootable image on quay.
```
$ podman machine os apply quay.io/custom/machine-os:latest
```

Apply a new custom operating system to a specific machine from an OCI bootable image on quay.
```
$ podman machine os apply quay.io/custom/machine-os:latest mymachine
```

Apply a new custom operating system that was pulled or built on your existing machine by the unprivileged user.
Note the use of brackets around the path because this command is run by the root user.
```
$ podman machine os apply containers-storage:[/home/core/.local/share/containers/storage]localhost/mycustomimage:latest
```

Apply a new custom operating system that was pulled or built on your existing machine by the privileged user.  Note that
because a privileged user built or pulled the image, `bootc` will resolve that users storage and the path
is not needed.
```
$ podman machine os apply containers-storage:localhost/mycustomimage:latest
```

Apply a new custom operating system from an OCI archive in tar form.
```
$ podman machine os apply oci-archive:/tmp/oci-image.tar
```

Apply a new custom operating system from an OCI formatted directory.
```
$ podman machine os apply oci:/tmp/oci-image/
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**, **[podman-machine-os(1)](podman-machine-os.1.md)**

## HISTORY
February 2023, Originally compiled by Ashley Cui <acui@redhat.com>
