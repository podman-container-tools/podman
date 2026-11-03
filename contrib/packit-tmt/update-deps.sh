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
    # Persistently disable for the rest of the TF run.
    # dnf5 (incl. Fedora 43's 5.2) accepts setopt REPO.enabled=0; dnf4 uses --set-disabled.
    PODMAN_NEXT_REPO=copr:copr.fedorainfracloud.org:rhcontainerbot:podman-next
    if ! dnf config-manager setopt --create-missing-dir "${PODMAN_NEXT_REPO}.enabled=0"; then
        dnf config-manager --set-disabled "$PODMAN_NEXT_REPO"
    fi
else
    echo "Target branch is '${TARGET_BRANCH}', podman-next repo will be used"
fi

# This should work even when podman-next isn't installed. It'll fetch the
# highest versions available across all repos.
dnf -y upgrade --allowerasing --setopt=allow_vendor_change=1 "${DISABLE_REPO[@]}" --exclude=podman*
