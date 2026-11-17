#!/usr/bin/env bash

# This script will update the Version field in the spec which is set to 0 by
# default. Useful for local manual rpm builds where the Version needs to be set
# correctly.

set -eox pipefail

PACKAGE=podman
SPEC_FILE=$PACKAGE.spec
VERSION=$(grep '^const RawVersion' ../version/rawversion/version.go | cut -d\" -f2)
RPM_VERSION=$(echo $VERSION | sed -e 's/^v//' -e 's/-/~/g')

# Update spec file to use local changes
sed -i "s/^Version:.*/Version: $RPM_VERSION/" $SPEC_FILE
sed -i "s/^Source0:.*/Source0: $PACKAGE-$VERSION.tar.gz/" $SPEC_FILE
sed -i "s/^%autosetup.*/%autosetup -Sgit -n %{name}-$VERSION/" $SPEC_FILE

# Generate Source0 archive from HEAD
(cd .. && git archive --format=tar.gz --prefix=$PACKAGE-$VERSION/ HEAD -o rpm/$PACKAGE-$VERSION.tar.gz)
