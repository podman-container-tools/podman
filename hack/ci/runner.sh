#!/usr/bin/env bash

# This script is only intended to be run inside the lima VM to configure it and start the tests.
# Do not run locally.

set -eo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)

source "$SCRIPT_DIR/lib.sh"

echo "::group::Test Setup"

parse_args "$@"

export GOCACHE=/var/tmp/podman/.gocache
export GOMODCACHE=/var/tmp/podman/.gomodcache

PRESERVE_ENVS="CI_USE_REGISTRY_CACHE,CI_DESIRED_COMPOSEFS,OCI_RUNTIME,CGROUP_MANAGER,STORAGE_FS,STORAGE_OPTIONS_OVERLAY,STORAGE_OPTIONS_VFS,PODMAN_UPGRADE_FROM,GOCACHE,GOMODCACHE"
# run as root or or not
SUDO=""
if [[ "$PRIV" == "root" ]]; then
    SUDO="sudo --non-interactive --preserve-env=$PRESERVE_ENVS"
fi

CI_DESIRED_STORAGE=overlay

case "$DISTRO_NAME" in
fedora-current)
    ;;
fedora-prior)
    CI_DESIRED_STORAGE=vfs
    ;;
fedora-rawhide)
    # On rawhide enable composefs testing
    CI_DESIRED_COMPOSEFS="composefs"
    # Enable sequoia testing
    TEST_BUILD_TAGS="containers_image_sequoia"

    # mount a tmpfs for the container storage. This is a work around for the staging pull composefs flake.
    # FIXME: https://github.com/containers/podman/issues/28813
    sudo mount -t tmpfs -o size=75%,mode=0700 none /var/lib/containers
    ;;
debian-sid)
    ;;
*)
    die "Unknown DISTRO_NAME passed $DISTRO_NAME"
    ;;
esac

# As of July 2024, CI VMs come built-in with a registry.
LCR=/var/cache/local-registry/local-cache-registry
if [[ -x $LCR ]]; then
    # Images in cache registry are prepopulated at the time
    # VMs are built. If any PR adds a dependency on new images,
    # those must be fetched now, at VM start time. This should
    # be rare, and must be fixed in next automation images build.
    while read new_image; do
        $LCR cache $new_image
    done < <(grep '^[^#]' test/NEW-IMAGES || true)
fi

## Used in tests so we need to export them
export CI_DESIRED_STORAGE

# Marker for the tests so they can insist the CI_DESIRED_* values are set
# instead of silently skipping. GITHUB_ACTIONS is no use here, it is not
# carried into the VM, and anyone can run our tests under github actions.
export PODMAN_CI=1

# composefs is only written into the rootful config below, so only tell the
# tests to expect it when we actually run rootful.
if [[ "$PRIV" == "root" ]]; then
    export CI_DESIRED_COMPOSEFS
fi

### SETUP HERE

# Custom storage.conf setup to test different drivers
conf=/etc/containers/storage.conf
if [[ -e $conf ]]; then
    die "FATAL! INTERNAL ERROR! Cannot override $conf"
fi
sudo tee $conf << EOF
[storage]
driver = "$CI_DESIRED_STORAGE"
EOF

if [[ -n "$CI_DESIRED_COMPOSEFS" ]]; then
    # composefs only works as root so we must set it in the rootful config
    sudo mkdir /etc/containers/storage.rootful.conf.d/
    conf=/etc/containers/storage.rootful.conf.d/99-composefs.conf
    sudo tee $conf << EOF

# BEGIN CI-enabled composefs
[storage.options]
pull_options = {enable_partial_images = "true", use_hard_links = "false", ostree_repos="", convert_images = "true"}

[storage.options.overlay]
use_composefs = "true"
# END CI-enabled composefs
EOF

    # KLUDGE ALERT! Magic options needed for testing composefs.
    # This option was intended for passing one arg to --storage-opt
    # but we're hijacking it to pass an extra option+arg. And it
    # actually works.
    # This is needed for the e2e tests as they do not use the config file.
    if [[ "$PRIV" == "root" ]]; then
        export STORAGE_OPTIONS_OVERLAY='overlay.use_composefs=true --pull-option=enable_partial_images=true --pull-option=convert_images=true'
    fi
fi

# Machine image is not cached by design.
if [[ "$TEST" != machine ]]; then
    # Install test registries.conf
    sudo install -v -D -m 644 ./test/registries-cached.conf /etc/containers/registries.conf
fi

# Add Root user namespace for --userns=auto support in tests
for which in uid gid; do
    if ! grep -qE '^containers:' /etc/sub$which; then
        echo 'containers:10000000:1048576' | sudo tee --append /etc/sub$which
    fi
done

# Load null_blk to use /dev/nullb0 for testing block
# devices limits
sudo modprobe null_blk nr_devices=1 || :

# Ensure our CI uses the cache registry
export CI_USE_REGISTRY_CACHE=1

if [[ "$TEST" != build && "$TEST" != unit ]]; then
    ## Remove packaged podman and install the compiled podman
    remove_packaged_podman_files
    make docs binaries EXTRA_BUILDTAGS="$TEST_BUILD_TAGS"
    sudo make install PREFIX=/usr ETCDIR=/etc
fi

# Setup git user, bud tests need this.
$SUDO git config --global user.name "Podman CI"
$SUDO git config --global user.email "no-reply@podman.io"

echo "::endgroup::" # Test Setup

### LOG various relevant things

echo "::group::Logging system info"
"$SCRIPT_DIR/logcollector.sh" packages
"$SCRIPT_DIR/logcollector.sh" ip
echo "::endgroup::" # Logging system info

mkdir -p "$SCRIPT_DIR/logs"
# Log the journal at the end.
trap "sudo \"$SCRIPT_DIR/logcollector.sh\" journal &> \"$SCRIPT_DIR/logs/journal-$TEST_NAME.log\"" EXIT

### TEST functions

function logformatter() {
    # Requires stdin and stderr combined!
    awk --file "${SCRIPT_DIR}/timestamp.awk" |&
        (
            cd "${SCRIPT_DIR}/logs"
            "${SCRIPT_DIR}/logformatter" "$TEST_NAME"
        )
}

function run_build() {
    # Ensure always start from clean-slate with all vendor modules downloaded
    make clean
    # make vendor
    # shellcheck disable=SC2154
    make -j $(nproc) --output-sync=target podman-release EXTRA_BUILDTAGS="$TEST_BUILD_TAGS" # includes podman, podman-remote, and docs

    # There's no reason to validate-binaries across multiple linux platforms
    # shellcheck disable=SC2154
    if [[ "$DISTRO_NAME" == fedora-current ]]; then
        make -j $(nproc) --output-sync=target validate-binaries

        # This will generate completion scripts so make sure the tree is clean
        SUGGESTION="run 'make completions' and commit all changes" ./hack/tree_status.sh
    fi
}

function run_apiv2() {
    virtualenv .venv/requests
    source .venv/requests/bin/activate
    pip install --upgrade pip
    pip install --requirement ./test/apiv2/python/requirements.txt
    $SUDO sh -c "(
        rc=0
        make localapiv2-bash || rc=\$?
        source .venv/requests/bin/activate
        make localapiv2-python || rc=\$?
        exit \$rc
    )" |& logformatter
}

function run_bindings() {
    make .install.ginkgo
    $SUDO make testbindings |& logformatter
}

function run_bud() {
    local args=()
    if [[ "$MODE" == "remote" ]]; then
        args+=(--remote)
    fi
    $SUDO ./test/buildah-bud/run-buildah-bud-tests "${args[@]}" |& logformatter
}

function run_compose_v2() {
    # FIXME do not hard code the version here, and likely it would be best to embed this in the VM image to begin with.
    sudo curl --fail -SL https://github.com/docker/compose/releases/download/v2.32.3/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose
    sudo chmod +x /usr/local/bin/docker-compose
    $SUDO ./test/compose/test-compose |& logformatter
}

function run_docker_py() {
    virtualenv .venv/docker-py
    source .venv/docker-py/bin/activate
    pip install --upgrade pip
    pip install --requirement ./test/python/requirements.txt
    $SUDO sh -c "source .venv/docker-py/bin/activate && make run-docker-py-tests" |& logformatter
}

function run_unit() {
    $SUDO make localunit
}

function run_upgrade() {
    export SUPPRESS_BOLTDB_WARNING=true
    export PODMAN_UPGRADE_FROM=${MODE}
    $SUDO bats test/upgrade |& logformatter
}

function run_int() {
    $SUDO make ${MODE}integration |& logformatter
}

function run_sys() {
    $SUDO make ${MODE}system |& logformatter
}

function run_machine() {
    $SUDO make ${MODE}machine
}

function run_farm() {
    # The farm tests add a system connection to ourselves over ssh to localhost
    # and then talk to the rootless API socket through it.
    systemctl --user enable --now podman.socket

    $SUDO bats test/farm |& logformatter
}

echo "Starting Test"

run_$TEST
