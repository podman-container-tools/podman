#!/usr/bin/env bash

set -eo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)

source "$SCRIPT_DIR/lib.sh"

AUTOMATION_RELEASE="${AUTOMATION_RELEASE:-$(get_automation_release)}"
LIMA_VM_NAME=podman-ci

REPO_DIR="$SCRIPT_DIR/../.."

parse_args "$@"

IMAGE="$DISTRO_NAME.x86_64.qcow2.zst"

# IMAGE_URL="~/Downloads/fedora-rawhide.x86_64.qcow2.zst"
IMAGE_URL="https://objectstorage.us-ashburn-1.oraclecloud.com/n/id0lmbbwgcdv/b/podman-ci-vm-images/o/releases/$AUTOMATION_RELEASE/$IMAGE"

trap "limactl delete --force $LIMA_VM_NAME" EXIT

limactl --yes start --plain --name=$LIMA_VM_NAME --cpus $(nproc) --memory 8 --nested-virt \
    --set ".images=[{\"location\":\"$IMAGE_URL\", \"arch\": \"x86_64\"}]" \
    "$SCRIPT_DIR/template.lima.yml"

limactl copy "$REPO_DIR" $LIMA_VM_NAME:/var/tmp/podman

# If binaries were downloaded/copied, make sure they are executable and have current timestamps
# so make doesn't rebuild them inside the VM.
if limactl shell $LIMA_VM_NAME test -d /var/tmp/podman/bin; then
    limactl shell $LIMA_VM_NAME sh -c "find /var/tmp/podman/bin -type f -exec chmod +x {} + -exec touch {} +"
fi

set +e

limactl shell --workdir /var/tmp/podman $LIMA_VM_NAME ./hack/ci/runner.sh "${@}"
rc=$?

limactl shell --workdir /var/tmp/podman $LIMA_VM_NAME sudo ./hack/ci/logcollector.sh journal &> "$SCRIPT_DIR/journal.log"

# Fix permissions of the cache directories so the host user can read/write them
limactl shell $LIMA_VM_NAME sh -c "sudo chown -R --reference=/var/tmp/podman /var/tmp/podman/.gocache /var/tmp/podman/.gomodcache || true"

# Copy the Go cache directories back to the host so they can be cached by Github Actions
if [[ -n "$GITHUB_ACTIONS" ]]; then
    if limactl shell $LIMA_VM_NAME test -d /var/tmp/podman/.gocache; then
        limactl copy $LIMA_VM_NAME:/var/tmp/podman/.gocache "$REPO_DIR/.gocache" || true
    fi
    if limactl shell $LIMA_VM_NAME test -d /var/tmp/podman/.gomodcache; then
        limactl copy $LIMA_VM_NAME:/var/tmp/podman/.gomodcache "$REPO_DIR/.gomodcache" || true
    fi
fi

# Copy the binaries out of the VM in gh actions so we can upload them as artifact
if [[ -n "$GITHUB_ACTIONS" && "$TEST" == build ]]; then
    limactl copy $LIMA_VM_NAME:/var/tmp/podman/bin "$REPO_DIR/bin" || die "failed to copy binaries"
fi

exit $rc
