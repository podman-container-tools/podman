![buildah logo](https://cdn.rawgit.com/containers/buildah/main/logos/buildah-logo_large.png)

# Installation Instructions

## Installing packaged versions of buildah

### [Arch Linux](https://www.archlinux.org)

```bash
sudo pacman -S buildah
```

### [CentOS](https://www.centos.org)

Buildah is available in the default Extras repos for CentOS 7 and in
the AppStream repo for CentOS 8 and Stream, however the available version often
lags the upstream release.

```bash
sudo yum -y install buildah
```

### [Debian](https://debian.org)

The buildah package is available in
the [Bookworm](https://packages.debian.org/bookworm/buildah), which
is the current stable release (Debian 12), as well as Debian Unstable/Sid.

```bash
# Debian Stable/Bookworm or Unstable/Sid
sudo apt-get update
sudo apt-get -y install buildah
```


### [Fedora](https://www.fedoraproject.org)

```bash
sudo dnf -y install buildah
```

### [Fedora SilverBlue](https://silverblue.fedoraproject.org)

Installed by default

### [Fedora CoreOS](https://coreos.fedoraproject.org)

Not Available.  Must be installed via package layering.

rpm-ostree install buildah

Note: [`podman`](https://podman.io) build is available by default.

### [Gentoo](https://www.gentoo.org)
[app-containers/buildah](https://packages.gentoo.org/packages/app-containers/buildah)
```bash
sudo emerge app-containers/buildah
```

### [openSUSE](https://www.opensuse.org)

```bash
sudo zypper install buildah
```

### [openSUSE Kubic](https://kubic.opensuse.org)

transactional-update pkg in buildah

### [RHEL7](https://www.redhat.com/en/technologies/linux-platforms/enterprise-linux)

Subscribe, then enable Extras channel and install buildah.

```bash
sudo subscription-manager repos --enable=rhel-7-server-extras-rpms
sudo yum -y install buildah
```

#### [Raspberry Pi OS arm64 (beta)](https://downloads.raspberrypi.org/raspios_arm64/images/)

Raspberry Pi OS use the standard Debian's repositories,
so it is fully compatible with Debian's arm64 repository.
You can simply follow the [steps for Debian](#debian) to install buildah.


### [RHEL8 Beta](https://www.redhat.com/en/blog/powering-its-future-while-preserving-present-introducing-red-hat-enterprise-linux-8-beta?intcmp=701f2000001Cz6OAAS)

```bash
sudo yum module enable -y container-tools:1.0
sudo yum module install -y buildah
```

### [Ubuntu](https://www.ubuntu.com)

The buildah package is available in the official repositories for Ubuntu 20.10
and newer.

```bash
# Ubuntu 20.10 and newer
sudo apt-get -y update
sudo apt-get -y install buildah
```

# Building from scratch

## System Requirements

### Kernel Version Requirements
To run Buildah on Red Hat Enterprise Linux or CentOS, version 7.4 or higher is required.
On other Linux distributions Buildah requires a kernel version that supports the OverlayFS and/or fuse-overlayfs filesystem -- you'll need to consult your distribution's documentation to determine a minimum version number.

### runc Requirement

Buildah uses `runc` to run commands when `buildah run` is used, or when `buildah build`
encounters a `RUN` instruction, so you'll also need to build and install a compatible version of
[runc](https://github.com/opencontainers/runc) for Buildah to call for those cases.  If Buildah is installed
via a package manager such as yum, dnf or apt-get, runc will be installed as part of that process.

## Package Installation

Buildah is available on several software repositories and can be installed via a package manager such
as yum, dnf or apt-get on a number of Linux distributions.

## Installation from GitHub

Prior to installing Buildah, install the following packages on your Linux distro:
* make
* golang (Requires version 1.13 or higher.)
* bats
* btrfs-progs-devel
* bzip2
* git
* go-md2man
* gpgme-devel
* glib2-devel
* libassuan-devel
* libseccomp-devel
* runc (Requires version 1.0 RC4 or higher.)
* containers-common

### Fedora

In Fedora, you can use this command:

```
 dnf -y install \
    make \
    golang \
    bats \
    btrfs-progs-devel \
    glib2-devel \
    gpgme-devel \
    libassuan-devel \
    libseccomp-devel \
    git \
    bzip2 \
    go-md2man \
    runc \
    containers-common
```

Then to install Buildah on Fedora follow the steps in this example:

```
  git clone https://github.com/containers/buildah
  cd buildah
  make
  sudo make install
  buildah --help
```

### RHEL, CentOS

In RHEL and CentOS, run this command to install the build dependencies:

```
 yum -y install \
    make \
    golang \
    bats \
    btrfs-progs-devel \
    glib2-devel \
    gpgme-devel \
    libassuan-devel \
    libseccomp-devel \
    git \
    bzip2 \
    go-md2man \
    runc \
    skopeo-containers
```

The build steps for Buildah on RHEL or CentOS are the same as for Fedora, above.

### openSUSE

On openSUSE Tumbleweed, install go via `zypper in go`, then run this command:

```
 zypper in make \
    git \
    golang \
    runc \
    bzip2 \
    libgpgme-devel \
    libseccomp-devel \
    libbtrfs-devel \
    go-md2man
```

The build steps for Buildah on SUSE / openSUSE are the same as for Fedora, above.


### Ubuntu/Debian

In Ubuntu 22.10 (Karmic) or Debian 12 (Bookworm) you can use these commands:

```
  sudo apt-get -y -qq update
  sudo apt-get -y install bats btrfs-progs git go-md2man golang libapparmor-dev libglib2.0-dev libgpgme11-dev libseccomp-dev libselinux1-dev make runc skopeo libbtrfs-dev
```

The build steps for Buildah on Debian or Ubuntu are the same as for Fedora, above.

## Vendoring - Dependency Management

This project is using [go modules](https://github.com/golang/go/wiki/Modules) for dependency management.  If the CI is complaining about a pull request leaving behind an unclean state, it is very likely right about it.  After changing dependencies, make sure to run `make vendor-in-container` to synchronize the code with the go module and repopulate the `./vendor` directory.

## Configuration files

The following configuration files are required in order for Buildah to run appropriately.  The
majority of these files are commonly contained in the `containers-common` package.

### [registries.conf](https://github.com/containers/container-libs/blob/main/image/registries.conf)

#### Man Page: [containers-registries.conf.5](https://github.com/containers/container-libs/blob/main/image/docs/containers-registries.conf.5.md)

`/usr/share/containers/registries.conf`, `/etc/containers/registries.conf`, `$HOME/.config/containers/registries.conf`

registries.conf is the configuration file which specifies which container registries should be consulted when completing image names which do not include a registry or domain portion.

#### Example from the Fedora `containers-common` package

```
cat /etc/containers/registries.conf
# For more information on this configuration file, see containers-registries.conf(5).
#
# NOTE: RISK OF USING UNQUALIFIED IMAGE NAMES
# We recommend always using fully qualified image names including the registry
# server (full dns name), namespace, image name, and tag
# (e.g., registry.redhat.io/ubi8/ubi:latest). Pulling by digest (i.e.,
# quay.io/repository/name@digest) further eliminates the ambiguity of tags.
# When using short names, there is always an inherent risk that the image being
# pulled could be spoofed. For example, a user wants to pull an image named
# `foobar` from a registry and expects it to come from myregistry.com. If
# myregistry.com is not first in the search list, an attacker could place a
# different `foobar` image at a registry earlier in the search list. The user
# would accidentally pull and run the attacker's image and code rather than the
# intended content. We recommend only adding registries which are completely
# trusted (i.e., registries which don't allow unknown or anonymous users to
# create accounts with arbitrary names). This will prevent an image from being
# spoofed, squatted or otherwise made insecure.  If it is necessary to use one
# of these registries, it should be added at the end of the list.
#
# # An array of host[:port] registries to try when pulling an unqualified image, in order.
unqualified-search-registries = ["registry.fedoraproject.org", "registry.access.redhat.com", "docker.io", "quay.io"]
#
# [[registry]]
# # The "prefix" field is used to choose the relevant [[registry]] TOML table;
# # (only) the TOML table with the longest match for the input image name
# # (taking into account namespace/repo/tag/digest separators) is used.
# #
# # If the prefix field is missing, it defaults to be the same as the "location" field.
# prefix = "example.com/foo"
#
# # If true, unencrypted HTTP as well as TLS connections with untrusted
# # certificates are allowed.
# insecure = false
#
# # If true, pulling images with matching names is forbidden.
# blocked = false
#
# # The physical location of the "prefix"-rooted namespace.
# #
# # By default, this equal to "prefix" (in which case "prefix" can be omitted
# # and the [[registry]] TOML table can only specify "location").
# #
# # Example: Given
# #   prefix = "example.com/foo"
# #   location = "internal-registry-for-example.net/bar"
# # requests for the image example.com/foo/myimage:latest will actually work with the
# # internal-registry-for-example.net/bar/myimage:latest image.
# location = "internal-registry-for-example.com/bar"
#
# # (Possibly-partial) mirrors for the "prefix"-rooted namespace.
# #
# # The mirrors are attempted in the specified order; the first one that can be
# # contacted and contains the image will be used (and if none of the mirrors contains the image,
# # the primary location specified by the "registry.location" field, or using the unmodified
# # user-specified reference, is tried last).
# #
# # Each TOML table in the "mirror" array can contain the following fields, with the same semantics
# # as if specified in the [[registry]] TOML table directly:
# # - location
# # - insecure
# [[registry.mirror]]
# location = "example-mirror-0.local/mirror-for-foo"
# [[registry.mirror]]
# location = "example-mirror-1.local/mirrors/foo"
# insecure = true
# # Given the above, a pull of example.com/foo/image:latest will try:
# # 1. example-mirror-0.local/mirror-for-foo/image:latest
# # 2. example-mirror-1.local/mirrors/foo/image:latest
# # 3. internal-registry-for-example.net/bar/image:latest
# # in order, and use the first one that exists.

# Enforcing mode for short names is default for Fedora 34 and newer
short-name-mode="enforcing"
```

### [mounts.conf](https://src.fedoraproject.org/rpms/skopeo/blob/main/f/mounts.conf)

`/usr/share/containers/mounts.conf` and optionally `/etc/containers/mounts.conf`

The mounts.conf files specify volume mount files or directories that are automatically mounted inside containers when executing the `buildah run` or `buildah build` commands.  Container processes can then use this content.  The volume mount content does not get committed to the final image.  This file is usually provided by the containers-common package.

Usually these directories are used for passing secrets or credentials required by the package software to access remote package repositories.

For example, a mounts.conf with the line "`/usr/share/rhel/secrets:/run/secrets`", the content of `/usr/share/rhel/secrets` directory is mounted on `/run/secrets` inside the container.  This mountpoint allows Red Hat Enterprise Linux subscriptions from the host to be used within the container.  It is also possible to omit the destination if it's equal to the source path.  For example, specifying `/var/lib/secrets` will mount the directory into the same container destination path `/var/lib/secrets`.

Note this is not a volume mount. The content of the volumes is copied into container storage, not bind mounted directly from the host.

#### Example from the Fedora `containers-common` package:

```
cat /usr/share/containers/mounts.conf
/usr/share/rhel/secrets:/run/secrets
```

### [seccomp.json](https://src.fedoraproject.org/rpms/skopeo/blob/main/f/seccomp.json)

`/usr/share/containers/seccomp.json`

seccomp.json contains the list of seccomp rules to be allowed inside of
containers.  This file is usually provided by the containers-common package.

The link above takes you to the seccomp.json

### [policy.json](https://github.com/containers/skopeo/blob/main/default-policy.json)

`/etc/containers/policy.json`

#### Man Page: [policy.json.5](https://github.com/containers/image/blob/main/docs/policy.json.md)


#### Example from the Fedora `containers-common` package:

```
cat /etc/containers/policy.json
{
    "default": [
	{
	    "type": "insecureAcceptAnything"
	}
    ],
    "transports":
	{
	    "docker-daemon":
		{
		    "": [{"type":"insecureAcceptAnything"}]
		}
	}
}
```

## Debug with Delve and the like

To make a source debug build without optimizations use `BUILDDEBUG=1`, like:
```
make all BUILDDEBUG=1
```

## Vendoring

Buildah uses Go Modules for vendoring purposes.  If you need to update or add a vendored package into Buildah, please follow this procedure:
 * Enter into your sandbox `src/github.com/containers/buildah` and ensure that the GOPATH variable is set to the directory prior as noted above.
 * `export GO111MODULE=on`
 * `go get` the needed version:
     * Assuming you want to 'bump' the `github.com/containers/storage` package to version 1.12.13, use this command: `go get github.com/containers/storage@v1.12.13`
     *  Assuming that you want to 'bump' the `github.com/containers/storage` package to a particular commit, use this command: `go get github.com/containers/storage@e307568568533c4afccdf7b56df7b4493e4e9a7b`
 * `make vendor-in-container`
 * `make`
 * `make install`
 * Then add any updated or added files with `git add` then do a `git commit` and create a PR.

### Vendor from your own fork

If you wish to vendor in your personal fork to try changes out (assuming containers/storage in the below example):

 * `go mod edit -replace github.com/containers/storage=github.com/{mygithub_username}/storage@YOUR_BRANCH`
 * `make vendor-in-container`

To revert
 * `go mod edit -dropreplace github.com/containers/storage`
 * `make vendor-in-container`

To speed up fetching dependencies, you can use a [Go Module Proxy](https://proxy.golang.org) by setting `GOPROXY=https://proxy.golang.org`.
