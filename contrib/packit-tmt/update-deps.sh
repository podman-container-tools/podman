#!/usr/bin/env bash

set -exo pipefail

DISABLE_REPO=()
if grep -s -r -q -F '[testing-farm-tag-repository]' /etc/yum.repos.d 2>/dev/null; then
    DISABLE_REPO=(--disable-repo=testing-farm-tag-repository)
fi

# Only use podman-next for PRs targeting main branch
# PACKIT_TARGET_BRANCH is set by Packit for PR jobs (the base/target branch)
TARGET_BRANCH="${PACKIT_TARGET_BRANCH:-unknown}"

echo "DEBUG: PACKIT_TARGET_BRANCH='$PACKIT_TARGET_BRANCH'"
echo "DEBUG: TARGET_BRANCH='$TARGET_BRANCH'"

if [ "$TARGET_BRANCH" != "main" ]; then
    echo "Target branch is '$TARGET_BRANCH' (not main), disabling podman-next repo"
    DISABLE_REPO+=(--disable-repo='copr:copr.fedorainfracloud.org:rhcontainerbot:podman-next')
else
    echo "Target branch is 'main', podman-next repo will be used"
fi

# This should work even when podman-next isn't installed. It'll fetch the
# highest versions available across all repos.
dnf -y upgrade --allowerasing --setopt=allow_vendor_change=1 "${DISABLE_REPO[@]}" --exclude=podman*
