#!/usr/bin/env bash

set -exo pipefail

# This should work even when podman-next isn't installed. It'll fetch the
# highest versions available across all repos.
dnf -y upgrade --allowerasing --disable-repo=testing-farm-tag-repository --exclude=podman*
