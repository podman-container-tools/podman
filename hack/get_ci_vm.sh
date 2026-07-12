#!/usr/bin/env bash

#
# For help and usage information, simply execute the script w/o any arguments.
#
# This script is intended to be run by Red Hat podman developers who need
# to debug problems specifically related to Cirrus-CI automated testing.
# It requires that you have been granted prior access to create VMs in
# google-cloud.  For non-Red Hat contributors, VMs are available as-needed,
# with supervision upon request.

set -e

SCRIPT_FILEPATH=$(realpath "${BASH_SOURCE[0]}")
SCRIPT_DIRPATH=$(dirname "$SCRIPT_FILEPATH")
REPO_DIRPATH=$(realpath "$SCRIPT_DIRPATH/../")

# Help detect what get_ci_vm container called this script
GET_CI_VM="${GET_CI_VM:-0}"
in_get_ci_vm() {
    if ((GET_CI_VM == 0)); then
        echo "Error: $1 is not intended for use in this context"
        exit 2
    fi
}

# get_ci_vm APIv1 container entrypoint calls into this script
# to obtain required repo. specific configuration options.
if [[ "$1" == "--config" ]]; then
    in_get_ci_vm "$1" # handles GET_CI_VM==0 case
    case "$GET_CI_VM" in
    1)
        cat << EOF
DESTDIR="/var/tmp/go/src/github.com/containers/podman"
UPSTREAM_REPO="https://github.com/containers/podman.git"
CI_ENVFILE="/etc/ci_environment"
GCLOUD_PROJECT="libpod-218412"
GCLOUD_IMGPROJECT="libpod-218412"
GCLOUD_CFG="libpod"
GCLOUD_ZONE="${GCLOUD_ZONE:-us-central1-a}"
GCLOUD_CPUS="2"
GCLOUD_MEMORY="4Gb"
GCLOUD_DISK="200"
EOF
        ;;
    2)
        # get_ci_vm APIv2 configuration details
        echo "AWS_PROFILE=containers"
        ;;
    *)
        echo "Error: Your get_ci_vm container image is too old."
        ;;
    esac
elif [[ "$1" == "--setup" ]]; then
    in_get_ci_vm "$1"
    unset GET_CI_VM
    # get_ci_vm container entrypoint calls us with this option on the
    # Cirrus-CI environment instance, to perform repo.-specific setup.
    cd $REPO_DIRPATH
    echo "+ Loading ./hack/ci/lib.sh" > /dev/stderr
    source ./hack/ci/lib.sh
    echo "+ Mimicking .cirrus.yml build_task" > /dev/stderr
    make install.tools
    make binaries
    make docs
    echo "+ Running environment setup" > /dev/stderr
    ./hack/ci/setup_environment.sh
else
    # Pass this repo and CLI args into container for VM creation/management
    mkdir -p $HOME/.config/gcloud/ssh
    mkdir -p $HOME/.aws
    podman run -it --rm \
        --tz=local \
        -e NAME="$USER" \
        -e SRCDIR=/src \
        -e GCLOUD_ZONE="$GCLOUD_ZONE" \
        -e A_DEBUG="${A_DEBUG:-0}" \
        -v $REPO_DIRPATH:/src:O \
        -v $HOME/.config/gcloud:/root/.config/gcloud:z \
        -v $HOME/.config/gcloud/ssh:/root/.ssh:z \
        -v $HOME/.aws:/root/.aws:z \
        quay.io/libpod/get_ci_vm:latest "$@"
fi
