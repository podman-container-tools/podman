#!/usr/bin/env bash

set -exo pipefail

DISABLE_REPO=()
if grep -s -r -q -F '[testing-farm-tag-repository]' /etc/yum.repos.d 2>/dev/null; then
    DISABLE_REPO=(--disable-repo=testing-farm-tag-repository)
fi

# This should work even when podman-next isn't installed. It'll fetch the
# highest versions available across all repos.
dnf -y upgrade --allowerasing "${DISABLE_REPO[@]}" --exclude=podman*
