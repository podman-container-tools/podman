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

echo "::group::Starting VM"
limactl --yes start --plain --name=$LIMA_VM_NAME --cpus $(nproc) --memory 8 --nested-virt \
    --set ".images=[{\"location\":\"$IMAGE_URL\", \"arch\": \"x86_64\"}]" \
    "$SCRIPT_DIR/template.lima.yml"

limactl shell $LIMA_VM_NAME mkdir -p /var/tmp/podman-container-tools

limactl copy -r "$REPO_DIR" $LIMA_VM_NAME:/var/tmp/podman-container-tools/podman

echo "::endgroup::"

set +e

limactl shell --preserve-env --workdir /var/tmp/podman-container-tools/podman $LIMA_VM_NAME ./hack/ci/runner.sh "${@}"
rc=$?

echo "::group::Collecting logs"
limactl copy -r $LIMA_VM_NAME:/var/tmp/podman-container-tools/podman/hack/ci/logs/ $SCRIPT_DIR/logs
echo "::endgroup::"

# TODO: figure out how to cache the binaries from the build job to the actual test tasks
# Copy the binaries out of the VM in gh actions so we can upload them as artifact
# if [[ -n "$GITHUB_ACTIONS" && "$TEST" == build ]]; then
#    limactl copy $LIMA_VM_NAME:/var/tmp/podman-container-tools/podman/bin "$REPO_DIR/bin" || die "failed to copy binaries"
# fi

exit $rc
