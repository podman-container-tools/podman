![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

# Customizing the Podman Machine OS Image

The Podman machine runs on a bootc-based OS image hosted at
`quay.io/podman/machine-os`. While the default image covers the most common use
cases, you may need to customize it - for example, to add extra package
dependencies, enable additional kernel modules, or extend the set of emulated
CPU architectures available inside the machine.

This tutorial walks through building a custom machine OS image and applying it
to a Podman machine, using the addition of extra CPU architecture support beyond
the default `amd64` and `arm64` as a worked example.

The examples below use `quay.io` as the container registry for storing and
distributing the custom image. This is not a requirement - `podman machine os
apply` supports multiple image sources and transports, so it is possible to
build and apply an image entirely locally without a remote registry. See the
[podman-machine-os-apply(1)](https://github.com/containers/podman/blob/main/docs/source/markdown/podman-machine-os-apply.1.md)
man page for the full list of supported URI forms.

## Prerequisites

- Podman installed and working on your local system
- An account on a container registry (this tutorial uses `quay.io`)
- The version of the machine OS image you want to extend (check the
  [machine-os tags](https://quay.io/repository/podman/machine-os?tab=tags)
  to find the right tag)

## Step 1: Create the Containerfile

Create a `Containerfile` that layers your customizations on top of the base
machine OS image. Replace `<version>` with the tag matching your current Podman
machine OS version.

The example below installs `qemu-user-static`, which enables emulation of
additional CPU architectures such as `s390x`, `ppc64le`, and `riscv64`:

```dockerfile
FROM quay.io/podman/machine-os:<version>
RUN dnf -y install qemu-user-static
```

Add any other packages or configuration changes your use case requires in the
same Containerfile.

**NOTE**: The base image is a bootc-compatible OS image, not a regular
application container image. Only changes that are valid for a bootc image
(installed packages, dropped-in config files, etc.) are appropriate here.

## Step 2: Build the Custom Image

Build the image and tag it for your registry. Replace `<username>` and
`<version>` with your registry username and the same version tag used in the
Containerfile.

```console
podman build -f Containerfile -t quay.io/<username>/machine-os-custom:<version>
```

## Step 3: Push the Image to a Registry

Push the newly built image to your registry so that `podman machine os apply`
can reach it.

```console
podman push quay.io/<username>/machine-os-custom:<version>
```

## Step 4: Initialize a Podman Machine

If you do not already have a Podman machine running, initialize and start one
now. If an existing machine is already running you can skip this step.

```console
podman machine init --now
```

## Step 5: Apply the Custom OS Image

Use `podman machine os apply` to rebase the machine onto your custom image.
The `--restart` flag stops and restarts the machine automatically so the new OS
takes effect immediately.

```console
podman machine os apply --restart docker://quay.io/<username>/machine-os-custom:<version>
```

## Verification

Once the machine restarts, verify that the customization is in place. For the
extra-architectures example, try running a container for a non-native
architecture:

```console
podman run --rm --platform linux/s390x docker.io/library/alpine uname -m
```

You should see `s390x` (or whichever architecture you chose) printed to the
console.
