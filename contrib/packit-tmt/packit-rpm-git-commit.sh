#!/usr/bin/env bash

# Updates the rpm spec with the upstream git SHA. Works for both copr and koji
# builds via Packit actions. See .packit.yaml for usage.

set -exo pipefail

PACKAGE=podman

# Set path to rpm spec file
SPEC_FILE=rpm/$PACKAGE.spec

# Get short sha
GIT_COMMIT=$(git rev-parse HEAD)

# Update LDFLAGS to show commit id for Copr builds
sed -i "s/^GIT_COMMIT=.*/GIT_COMMIT=\"$GIT_COMMIT\"/" $SPEC_FILE
