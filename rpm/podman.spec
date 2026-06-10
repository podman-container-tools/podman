%global with_debug 1

%if 0%{?with_debug}
%global _find_debuginfo_dwz_opts %{nil}
%global _dwz_low_mem_die_limit 0
%else
%global debug_package %{nil}
%endif

%global gomodulesmode GO111MODULE=on

%if %{defined fedora}
%define build_with_btrfs 1
# qemu-system* isn't packageed for CentOS Stream / RHEL
%define qemu 1
# bats is included in the default repos (No epel/copr etc.)
%define distro_bats 1
%if %{?fedora} >= 43
%define sequoia 1
%endif
%endif

%if %{defined copr_username}
%define copr_build 1
%if "%{copr_username}" == "rhcontainerbot" && "%{copr_projectname}" == "podman-next"
%define next_build 1
%endif
%endif

# Only RHEL and CentOS Stream rpms are built with fips-enabled go compiler
%if %{defined rhel}
%define fips_enabled 1
%endif

%global container_base_path go.podman.io

# For LDFLAGS
%global ld_project %{container_base_path}/%{name}/v6
%global ld_libpod %{ld_project}/libpod

# podman-machine subpackage will be present only on these architectures
%global machine_arches x86_64 aarch64

%if %{defined copr_build}
%define build_origin Copr: %{?copr_username}/%{?copr_projectname}
%else
%define build_origin %{?packager}
%endif

Name: podman
%if %{defined next_build}
Epoch: 102
%else
Epoch: 5
%endif
# DO NOT TOUCH the Version string!
# The TRUE source of this specfile is:
# https://github.com/containers/podman/blob/main/rpm/podman.spec
# If that's what you're reading, Version must be 0, and will be updated by Packit for
# copr and koji builds.
# If you're reading this on dist-git, the version is automatically filled in by Packit.
Version: 0
# The `AND` needs to be uppercase in the License for SPDX compatibility
License: Apache-2.0 AND BSD-2-Clause AND BSD-3-Clause AND ISC AND MIT AND MPL-2.0
Release: %autorelease
%if %{defined golang_arches_future}
ExclusiveArch: %{golang_arches_future}
%else
ExclusiveArch: aarch64 ppc64le s390x x86_64 riscv64
%endif
Summary: Manage Pods, Containers and Container Images
URL: https://%{name}.io/
# All SourceN files fetched from upstream
Source0: https://github.com/containers/%{name}/archive/v%{version_no_tilde}.tar.gz
Provides: %{name}-manpages = %{epoch}:%{version}-%{release}
BuildRequires: %{_bindir}/envsubst
%if %{defined build_with_btrfs}
BuildRequires: btrfs-progs-devel
%endif
BuildRequires: gcc
BuildRequires: glib2-devel
BuildRequires: glibc-devel
BuildRequires: glibc-static
BuildRequires: golang
BuildRequires: git-core
%if %{undefined rhel} || 0%{?rhel} >= 10
BuildRequires: go-rpm-macros
%endif
BuildRequires: gpgme-devel
BuildRequires: libassuan-devel
BuildRequires: libgpg-error-devel
BuildRequires: libseccomp-devel
BuildRequires: libselinux-devel
BuildRequires: shadow-utils-subid-devel
BuildRequires: pkgconfig
BuildRequires: make
BuildRequires: man-db
BuildRequires: sqlite-devel
BuildRequires: systemd
BuildRequires: systemd-devel
Requires: catatonit
Requires: conmon >= 2:2.1.7-2
# Podman 6 requires the new config file layout.
Requires: containers-common-extra >= 5:0.68.0
%if %{defined sequoia}
Requires: podman-sequoia
%endif

Obsoletes: %{name}-quadlet <= 5:4.4.0-1
Provides: %{name}-quadlet = %{epoch}:%{version}-%{release}

%description
%{name} (Pod Manager) is a fully featured container engine that is a simple
daemonless tool.  %{name} provides a Docker-CLI comparable command line that
eases the transition from other container engines and allows the management of
pods, containers and images.  Simply put: alias docker=%{name}.
Most %{name} commands can be run as a regular user, without requiring
additional privileges.

%{name} uses Buildah(1) internally to create container images.
Both tools share image (not container) storage, hence each can use or
manipulate images (but not containers) created by the other.


%package docker
Summary: Emulate Docker CLI using %{name}
BuildArch: noarch
Requires: %{name} = %{epoch}:%{version}-%{release}
Conflicts: docker
Conflicts: docker-latest
Conflicts: docker-ce
Conflicts: docker-ee
Conflicts: moby-engine

%description docker
This package installs a script named docker that emulates the Docker CLI by
executes %{name} commands, it also creates links between all Docker CLI man
pages and %{name}.

%package tests
Summary: Tests for %{name}

Requires: %{name} = %{epoch}:%{version}-%{release}
%if %{defined distro_bats}
Requires: bats
%endif
Requires: attr
Requires: jq
Requires: skopeo
Requires: nmap-ncat
Requires: httpd-tools
Requires: openssl
Requires: socat
Requires: buildah
Requires: gnupg
Requires: xfsprogs

%description tests
%{summary}

This package contains system tests for %{name}. Only intended to be used for
gating tests. Not supported for end users / customers.

%package remote
Summary: (Experimental) Remote client for managing %{name} containers

%description remote
Remote client for managing %{name} containers.

This experimental remote client is under heavy development. Please do not
run %{name}-remote in production.

%{name}-remote uses the version 2 API to connect to a %{name} client to
manage pods, containers and container images. %{name}-remote supports ssh
connections as well.

%package -n %{name}sh
Summary: Confined login and user shell using %{name}
Requires: %{name} = %{epoch}:%{version}-%{release}
Provides: %{name}-shell = %{epoch}:%{version}-%{release}
Provides: %{name}-%{name}sh = %{epoch}:%{version}-%{release}

%description -n %{name}sh
%{name}sh provides a confined login and user shell with access to volumes and
capabilities specified in user quadlets.

It is a symlink to %{_bindir}/%{name} and execs into the `%{name}sh` container
when `%{_bindir}/%{name}sh` is set as a login shell or set as os.Args[0].

%ifarch %{machine_arches}
%package machine
Summary: Metapackage for setting up %{name} machine
Requires: %{name} = %{epoch}:%{version}-%{release}
Requires: gvisor-tap-vsock
%if %{defined qemu}
%ifarch aarch64
Requires: qemu-system-aarch64-core
%endif
%ifarch x86_64
Requires: qemu-system-x86-core
%endif
%else
Requires: qemu-kvm
%endif
Requires: qemu-img
Requires: virtiofsd
ExclusiveArch: x86_64 aarch64

%description machine
This subpackage installs the dependencies for %{name} machine, for more see:
https://docs.podman.io/en/latest/markdown/podman-machine.1.html
%endif

%prep
%autosetup -Sgit -n %{name}-%{version_no_tilde}
sed -i 's;@@PODMAN@@\;$(BINDIR);@@PODMAN@@\;%{_bindir};' Makefile

# cgroups-v1 is supported on rhel9
%if 0%{?rhel} == 9
sed -i '/DELETE ON RHEL9/,/DELETE ON RHEL9/d' libpod/runtime.go
%endif

%build
%set_build_flags
export CGO_CFLAGS=$CFLAGS

# These extra flags present in $CFLAGS have been skipped for now as they break the build
CGO_CFLAGS=$(echo $CGO_CFLAGS | sed 's/-flto=auto//g')
CGO_CFLAGS=$(echo $CGO_CFLAGS | sed 's/-Wp,D_GLIBCXX_ASSERTIONS//g')
CGO_CFLAGS=$(echo $CGO_CFLAGS | sed 's/-specs=\/usr\/lib\/rpm\/redhat\/redhat-annobin-cc1//g')

%ifarch x86_64
export CGO_CFLAGS+=" -m64 -mtune=generic -fcf-protection=full"
%endif

export GOPROXY=direct

LDFLAGS="-X %{ld_libpod}/define.buildInfo=${SOURCE_DATE_EPOCH:-$(date +%s)} \
         -X \"%{ld_libpod}/define.buildOrigin=%{build_origin}\" \
         -X %{ld_libpod}/config._installPrefix=%{_prefix} \
         -X %{ld_libpod}/config._etcDir=%{_sysconfdir} \
         -X %{ld_project}/pkg/systemd/quadlet._binDir=%{_bindir}"

# This variable will be set by Packit actions. See .packit.yaml in the root dir
# of the repo (upstream as well as Fedora dist-git).
GIT_COMMIT=""
LDFLAGS="$LDFLAGS -X %{ld_libpod}/define.gitCommit=$GIT_COMMIT"

# build rootlessport first
%gobuild -o bin/rootlessport ./cmd/rootlessport

export BASEBUILDTAGS="seccomp $(hack/systemd_tag.sh) $(hack/libsubid_tag.sh) libsqlite3 grpcnotrace"

# libtrust_openssl buildtag switches to using the FIPS-compatible func
# `ecdsa.HashSign`.
# Ref 1: https://github.com/golang-fips/go/blob/main/patches/015-add-hash-sign-verify.patch#L22
# Ref 2: https://github.com/containers/libtrust/blob/main/ec_key_openssl.go#L23
%if %{defined fips_enabled}
export BASEBUILDTAGS="$BASEBUILDTAGS libtrust_openssl"
%endif

# build %%{name}
export BUILDTAGS="$BASEBUILDTAGS $(hack/btrfs_installed_tag.sh)"

%if %{defined sequoia}
export BUILDTAGS="$BUILDTAGS containers_image_sequoia"
%endif

%gobuild -o bin/%{name} ./cmd/%{name}

# build %%{name}-remote
export BUILDTAGS="$BASEBUILDTAGS exclude_graphdriver_btrfs remote"
%gobuild -o bin/%{name}-remote ./cmd/%{name}

# build quadlet
export BUILDTAGS="$BASEBUILDTAGS $(hack/btrfs_installed_tag.sh)"
%gobuild -o bin/quadlet ./cmd/quadlet

# build %%{name}-testing
export BUILDTAGS="$BASEBUILDTAGS $(hack/btrfs_installed_tag.sh)"
%gobuild -o bin/podman-testing ./cmd/podman-testing

# reset LDFLAGS for plugins binaries
LDFLAGS=''

%{__make} docs docker-docs

%install
install -dp %{buildroot}%{_unitdir}
PODMAN_VERSION=%{version} %{__make} DESTDIR=%{buildroot} PREFIX=%{_prefix} ETCDIR=%{_sysconfdir} \
       install.bin \
       install.man \
       install.systemd \
       install.completions \
       install.docker \
       install.docker-docs \
       install.remote \
       install.testing

sed -i 's;%{buildroot};;g' %{buildroot}%{_bindir}/docker

# do not include docker and podman-remote man pages in main package
for file in `find %{buildroot}%{_mandir}/man[157] -type f | sed "s,%{buildroot},," | grep -v -e %{name}sh.1 -e remote -e docker`; do
    echo "$file*" >> %{name}.file-list
done

rm -f %{buildroot}%{_mandir}/man5/docker*.5

install -d -p %{buildroot}%{_datadir}/%{name}/test/system
cp -pav test/system %{buildroot}%{_datadir}/%{name}/test/

%ifarch %{machine_arches}
# symlink virtiofsd in %%{name} libexecdir for machine subpackage
ln -s ../virtiofsd %{buildroot}%{_libexecdir}/%{name}
%if !%{defined qemu}
ln -s ../qemu-kvm %{buildroot}%{_libexecdir}/%{name}/qemu-system-%{arch}
%endif
%endif

#define license tag if not already defined
%{!?_licensedir:%global license %doc}

# Include empty check to silence rpmlint warning
%check

%files -f %{name}.file-list
%license LICENSE vendor/modules.txt
%doc README.md CONTRIBUTING.md install.md transfer.md
%{_bindir}/%{name}
%dir %{_libexecdir}/%{name}
%{_libexecdir}/%{name}/rootlessport
%{_libexecdir}/%{name}/quadlet
%{_datadir}/bash-completion/completions/%{name}
# By "owning" the site-functions dir, we don't need to Require zsh
%dir %{_datadir}/zsh/site-functions
%{_datadir}/zsh/site-functions/_%{name}
%dir %{_datadir}/fish/vendor_completions.d
%{_datadir}/fish/vendor_completions.d/%{name}.fish
%{_unitdir}/%{name}*
%{_userunitdir}/%{name}*
%{_tmpfilesdir}/%{name}.conf
%{_systemdgeneratordir}/%{name}-system-generator
%{_systemdusergeneratordir}/%{name}-user-generator

%files docker
%{_bindir}/docker
%{_mandir}/man1/docker*.1*
%{_sysconfdir}/profile.d/%{name}-docker.*
%{_tmpfilesdir}/%{name}-docker.conf
%{_user_tmpfilesdir}/%{name}-docker.conf

%files remote
%license LICENSE
%{_bindir}/%{name}-remote
%{_mandir}/man1/%{name}-remote*.*
%{_datadir}/bash-completion/completions/%{name}-remote
%dir %{_datadir}/fish/vendor_completions.d
%{_datadir}/fish/vendor_completions.d/%{name}-remote.fish
%dir %{_datadir}/zsh/site-functions
%{_datadir}/zsh/site-functions/_%{name}-remote

%files tests
%{_bindir}/%{name}-testing
%{_datadir}/%{name}/test

%files -n %{name}sh
%{_bindir}/%{name}sh
%{_mandir}/man1/%{name}sh.1*

%ifarch %{machine_arches}
%files machine
%dir %{_libexecdir}/%{name}
%{_libexecdir}/%{name}/virtiofsd
%if !%{defined qemu}
%{_libexecdir}/%{name}/qemu-system-%{arch}
%endif
%endif

%changelog
%autochangelog
