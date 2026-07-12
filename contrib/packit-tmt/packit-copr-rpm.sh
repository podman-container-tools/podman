#!/usr/bin/env bash

# This script handles any custom processing of the spec file using the `fix-spec-file`
# action in .packit.yaml. These steps only work on copr builds, not on official
# Fedora builds.

set -exo pipefail

GIT_TOPDIR=$(git rev-parse --show-toplevel)

. "$GIT_TOPDIR"/contrib/packit-tmt/packit-rpm-git-commit.sh

# Get Version from HEAD
VERSION=$(grep '^const RawVersion' version/rawversion/version.go | cut -d\" -f2)

# RPM Version can't take "-"
# shellcheck disable=SC2001
RPM_VERSION=$(echo "$VERSION" | sed -e 's/-/~/')

# Generate source tarball from HEAD
git-archive-all -C "$GIT_TOPDIR" --prefix="$PACKAGE-$VERSION/" "$GIT_TOPDIR/rpm/$PACKAGE-$VERSION.tar.gz"

# RPM Spec modifications

# Use the Version from HEAD in rpm spec
sed -i "s/^Version:.*/Version: $RPM_VERSION/" $SPEC_FILE

# Use Packit's supplied variable in the Release field in rpm spec.
sed -i "s/^Release:.*/Release: $PACKIT_RPMSPEC_RELEASE%{?dist}/" $SPEC_FILE

# Ensure last part of the release string is the git shortcommit without a
# prepended "g"
sed -i "/^Release: $PACKIT_RPMSPEC_RELEASE%{?dist}/ s/\(.*\)g/\1/" $SPEC_FILE

# Use above generated tarball as Source in rpm spec
sed -i "s/^Source0:.*.tar.gz/Source0: $PACKAGE-$VERSION.tar.gz/" $SPEC_FILE

# Update setup macro to use the correct build dir
sed -i "s/^%autosetup.*/%autosetup -Sgit -n %{name}-$VERSION/" $SPEC_FILE
