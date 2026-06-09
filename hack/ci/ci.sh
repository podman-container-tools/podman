#!/usr/bin/env bash

set -eo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)

source "$SCRIPT_DIR/lib.sh"

AUTOMATION_RELEASE="20260520t200858z"
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

set +e

limactl shell --workdir /var/tmp/podman $LIMA_VM_NAME ./hack/ci/runner.sh "${@}"
rc=$?

limactl shell --workdir /var/tmp/podman $LIMA_VM_NAME sudo ./hack/ci/logcollector.sh journal &> "$SCRIPT_DIR/journal.log"

# TODO: figure out how to cache the binaries from the build job to the actual test tasks
# Copy the binaries out of the VM in gh actions so we can upload them as artifact
# if [[ -n "$GITHUB_ACTIONS" && "$TEST" == build ]]; then
#    limactl copy $LIMA_VM_NAME:/var/tmp/podman/bin "$REPO_DIR/bin" || die "failed to copy binaries"
# fi

exit $rc
