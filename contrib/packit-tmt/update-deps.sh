#!/usr/bin/env bash

set -exo pipefail

DISABLE_REPO=()
if grep -s -r -q -F '[testing-farm-tag-repository]' /etc/yum.repos.d 2>/dev/null; then
    DISABLE_REPO=(--disable-repo=testing-farm-tag-repository)
fi

# Only use podman-next for PRs targeting main.
# PACKIT_TARGET_BRANCH is set by Packit for pull-request jobs (the base branch).
TARGET_BRANCH="${PACKIT_TARGET_BRANCH:-unknown}"
echo "PACKIT_TARGET_BRANCH=${TARGET_BRANCH}"

if [ "$TARGET_BRANCH" != "main" ]; then
    echo "Target branch is '$TARGET_BRANCH' (not main), disabling podman-next repo"
    # dnf5 and dnf4 spell this command differently.
    PODMAN_NEXT_REPO=copr:copr.fedorainfracloud.org:rhcontainerbot:podman-next
    if dnf --version 2>/dev/null | grep -q 'dnf5 version'; then
        dnf config-manager disable --create-missing-dir "$PODMAN_NEXT_REPO"
    else
        dnf config-manager --set-disabled "$PODMAN_NEXT_REPO"
    fi
else
    echo "Target branch is '${TARGET_BRANCH}', podman-next repo will be used"
fi

# This should work even when podman-next isn't installed. It'll fetch the
# highest versions available across all repos.
dnf -y upgrade --allowerasing --setopt=allow_vendor_change=1 "${DISABLE_REPO[@]}" --exclude=podman*
