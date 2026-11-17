# Podman Packaging

This document is intended for Podman *packagers*: those very few individuals
responsible for building and shipping Podman on Linux distributions.

Document verified accurate as of Podman 5.2, 2024-10-16.

## Building Podman

This document assumes you are able to build executables up to and
including `make install`.
See [Building from Source](https://podman.io/docs/installation#building-from-source)
on podman.io for possibly-outdated instructions.

## Package contents

Everything installed by `make install`, obviously.

Upstream splits Podman into multiple subpackages and we encourage you
to consider doing likewise: some users may not want `podman-remote`
or `-machine` or the test suite.

The best starting point is the
[RPM spec file](https://github.com/containers/podman/blob/main/rpm/podman.spec).
This illustrates the subpackage breakdown as well as top-level dependencies.


## Dependencies

Podman requires a *runtime*, a *runtime monitor*, a *pause process*,
and *networking tools*. In Fedora, some of these requirements are indirectly
specified via [container-libs/common](https://github.com/containers/container-libs/tree/main/common);
the nested tree looks like this:
```
    Podman
    ├── Requires: catatonit
    ├── Requires: conmon
    └── Requires: containers-common-extra
        ├── Requires: crun
        ├── Requires: netavark
        └── Requires: passt
```

### Runtime: crun

The only runtime supported upstream is [crun](https://github.com/containers/crun),
but different distros may wish to offer other options to their users. Your package
must, directly or indirectly, list a runtime prerequisite.

Heads up: you may end up being responsible for packaging this runtime, or at the
very least working closely with the package maintainer. The best starting point
for crun is its
[RPM spec file](https://github.com/containers/crun/blob/main/rpm/crun.spec).


### Pause process: catatonit

The pause process serves as a container `init`, reaping PIDs and handling signals.

As of this writing, Podman uses an external tool,
[catatonit](https://github.com/openSUSE/catatonit). This may be subject
to change in future Podman versions.

If you need to package catatonit, a good starting point might be its
[Fedora specfile](https://src.fedoraproject.org/rpms/catatonit/blob/rawhide/f/catatonit.spec).


### Runtime Monitor: conmon

The only working monitor is [conmon](https://github.com/containers/conmon).
There is a Rust implementation in the works,
[conmon-rs](https://github.com/containers/conmon-rs), but efforts
to make it work with Podman have stalled for years.

Heads up: you may end up being responsible for packaging conmon.
The best starting point is its
[RPM spec file](https://github.com/containers/conmon/blob/main/rpm/conmon.spec).


### Networking Tools: netavark, aardvark-dns, passt

Networking differs between *root* and *rootless*: [passt](https://passt.top/)
(also referred to as "pasta") is only needed for rootless.
[netavark](https://github.com/containers/netavark/) and
[aardvark-dns](https://github.com/containers/aardvark-dns/)
are needed for both root and rootless podman.

Heads up: you will probably end up being responsible for packaging
at least some of these. The best starting points are their respective
RPM spec files:
[netavark](https://github.com/containers/netavark/blob/main/rpm/netavark.spec),
[aardvark-dns](https://github.com/containers/aardvark-dns/blob/main/rpm/aardvark-dns.spec).

Netavark and aardvark-dns must be packaged in lockstep down
to the major-minor level: version `X.Y` of either is only
guaranteed to work with `X.Y` of the other. If you are responsible
for packaging these, make sure you set up interpackage dependencies
appropriately to prevent version mismatches between them.

## Metapackage: containers-common

This package provides config files, man pages, and (at the
packaging level) dependencies. There are good reasons for
keeping this as a separate package, the most important one
being that `buildah` and `skopeo` rely on this same content.
Also important is the ability for individual distros to
fine-tune config settings and dependencies.

You will probably be responsible for packaging this.
The best starting point is its
[RPM spec file](https://github.com/containers/container-libs/blob/main/common/rpm/containers-common.spec).
