#!/bin/bash

set -eo pipefail

# This script runs in the Cirrus CI environment, invoked from .cirrus.yml .
# It can also be invoked manually in a `hack/get_ci_cm.sh` environment,
# documentation of said usage is TBI.
#
# The principal deciding factor is the $TEST_FLAVOR envariable: for any
# given value 'xyz' there must be a function '_run_xyz' to handle that
# test. Several other envariables are used to differentiate further,
# most notably:
#
#    PODBIN_NAME  : "podman" (i.e. local) or "remote"
#    TEST_ENVIRON : 'host', or 'container'; desired environment in which to run
#    CONTAINER    : 1 if *currently* running inside a container, 0 if host
#

# shellcheck source=contrib/cirrus/lib.sh
source $(dirname $0)/lib.sh

showrun echo "starting"

function _run_validate-source() {
    # This target is only meant to be run on PRs.
    # We need the following env vars set for the git diff check below.
    req_env_vars CIRRUS_CHANGE_IN_REPO PR_BASE_SHA

    showrun make validate-source

    # make sure PRs have tests
    showrun make tests-included

    # make sure PRs have jira links (if needed for branch)
    showrun make test-jira-links-included

    # shellcheck disable=SC2154
    head=$CIRRUS_CHANGE_IN_REPO
    # shellcheck disable=SC2154
    base=$PR_BASE_SHA
    echo "_run_validate-source: head=$head  base=$base"
    diffs=$(git diff --name-only $base $head)

    # If PR touches renovate config validate it, as the image is very big only do so when needed
    if grep -E -q "^.github/renovate.json5" <<<"$diffs"; then
        msg "Checking renovate config."
        showrun podman run \
            -v ./.github/renovate.json5:/usr/src/app/renovate.json5:z \
            ghcr.io/renovatebot/renovate:latest \
            renovate-config-validator
    fi
}

function _run_unit() {
    # shellcheck disable=SC2154
    if [[ "$PODBIN_NAME" != "podman" ]]; then
        # shellcheck disable=SC2154
        die "$TEST_FLAVOR: Unsupported PODBIN_NAME='$PODBIN_NAME'"
    fi
    showrun make localunit
}

function _run_apiv2() {
    (
        showrun make localapiv2-bash
        source .venv/requests/bin/activate
        showrun make localapiv2-python
    ) |& logformatter
}

function _run_compose_v2() {
    showrun ./test/compose/test-compose |& logformatter
}

function _run_int() {
    dotest integration
}

function _run_sys() {
    dotest system
}

function _run_upgrade_test() {
    export SUPPRESS_BOLTDB_WARNING=true
    showrun bats test/upgrade |& logformatter
}

function _run_bud() {
    showrun ./test/buildah-bud/run-buildah-bud-tests |& logformatter
}

function _run_bindings() {
    # install ginkgo
    showrun make .install.ginkgo

    # if logformatter sees this, it can link directly to failing source lines
    local gitcommit_magic=
    if [[ -n "$GIT_COMMIT" ]]; then
        gitcommit_magic="/define.gitCommit=${GIT_COMMIT}"
    fi

    (echo "$gitcommit_magic" && \
        showrun make testbindings) |& logformatter
}

function _run_docker-py() {
    source .venv/docker-py/bin/activate
    showrun make run-docker-py-tests
}

function _run_endpoint() {
    showrun make test-binaries
    showrun make endpoint
}

function _run_farm() {
    msg "Testing podman farm."
    showrun bats test/farm |& logformatter
}

exec_container() {
    local var_val
    local cmd
    # Required to be defined by caller
    # shellcheck disable=SC2154
    msg "Re-executing runner inside container: $CTR_FQIN"
    msg "************************************************************"

    req_env_vars CTR_FQIN TEST_ENVIRON CONTAINER SECRET_ENV_RE

    # Line-separated arguments which include shell-escaped special characters
    declare -a envargs
    while read -r var; do
        # Pass "-e VAR" on the command line, not "-e VAR=value". Podman can
        # do a much better job of transmitting the value than we can,
        # especially when value includes spaces.
        envargs+=("-e" "$var")
    done <<<"$(passthrough_envars)"

    # VM Images and Container images are built using (nearly) identical operations.
    set -x
    env CONTAINERS_REGISTRIES_CONF=/dev/null bin/podman pull -q $CTR_FQIN
    # shellcheck disable=SC2154
    exec bin/podman run --rm --privileged --net=host --cgroupns=host \
        -v `mktemp -d -p /var/tmp`:/var/tmp:Z \
        --tmpfs /tmp:mode=1777 \
        -v /dev/fuse:/dev/fuse \
        -v "$GOPATH:$GOPATH:Z" \
        --workdir "$GOSRC" \
        -e "CONTAINER=1" \
        "${envargs[@]}" \
        $CTR_FQIN bash -c "$SCRIPT_BASE/setup_environment.sh && $SCRIPT_BASE/runner.sh"
}

function _run_swagger() {
    local upload_filename
    local upload_bucket
    local download_url
    local envvarsfile
    req_env_vars GCPJSON GCPNAME GCPPROJECT CTR_FQIN

    # The filename and bucket depend on the automation context
    #shellcheck disable=SC2154,SC2153
    if [[ -n "$CIRRUS_PR" ]]; then
        upload_bucket="libpod-pr-releases"
        upload_filename="swagger-pr$CIRRUS_PR.yaml"
    elif [[ -n "$CIRRUS_TAG" ]]; then
        upload_bucket="libpod-master-releases"
        upload_filename="swagger-$CIRRUS_TAG.yaml"
    elif [[ "$CIRRUS_BRANCH" == "main" ]]; then
        upload_bucket="libpod-master-releases"
        # readthedocs versioning uses "latest" for "main" (default) branch
        upload_filename="swagger-latest.yaml"
    elif [[ -n "$CIRRUS_BRANCH" ]]; then
        upload_bucket="libpod-master-releases"
        upload_filename="swagger-$CIRRUS_BRANCH.yaml"
    else
        die "Unknown execution context, expected a non-empty value for \$CIRRUS_TAG, \$CIRRUS_BRANCH, or \$CIRRUS_PR"
    fi

    # Swagger validation takes a significant amount of time
    msg "Pulling \$CTR_FQIN '$CTR_FQIN' (background process)"
    showrun bin/podman pull --quiet $CTR_FQIN &

    cd $GOSRC
    showrun make swagger

    # Cirrus-CI Artifact instruction expects file here
    cp -v $GOSRC/pkg/api/swagger.yaml ./

    envvarsfile=$(mktemp -p '' .tmp_$(basename $0)_XXXXXXXX)
    trap "rm -f $envvarsfile" EXIT  # contains secrets
    # Warning: These values must _not_ be quoted, podman will not remove them.
    #shellcheck disable=SC2154
    cat <<eof >>$envvarsfile
GCPJSON=$GCPJSON
GCPNAME=$GCPNAME
GCPPROJECT=$GCPPROJECT
FROM_FILEPATH=$GOSRC/swagger.yaml
TO_GCSURI=gs://$upload_bucket/$upload_filename
eof

    msg "Waiting for backgrounded podman pull to complete..."
    wait %%
    showrun bin/podman run -it --rm --security-opt label=disable \
        --env-file=$envvarsfile \
        -v $GOSRC:$GOSRC:ro \
        --workdir $GOSRC \
        $CTR_FQIN
    rm -f $envvarsfile
}

function _run_build() {
    # Ensure always start from clean-slate with all vendor modules downloaded
    showrun make clean
    showrun make vendor
    # shellcheck disable=SC2154
    showrun make -j $(nproc) --output-sync=target podman-release EXTRA_BUILDTAGS="$TEST_BUILD_TAGS" # includes podman, podman-remote, and docs

    # There's no reason to validate-binaries across multiple linux platforms
    # shellcheck disable=SC2154
    if [[ "$DISTRO_NV" =~ $FEDORA_NAME ]]; then
        showrun make -j $(nproc) --output-sync=target validate-binaries
    fi

    # Last-minute confirmation that we're testing the desired runtime.
    # This Can't Possibly Fail™ in regular CI; only when updating VMs.
    # $CI_DESIRED_RUNTIME must be defined in .cirrus.yml.
    req_env_vars CI_DESIRED_RUNTIME
    runtime=$(bin/podman info --format '{{.Host.OCIRuntime.Name}}')
    # shellcheck disable=SC2154
    if [[ "$runtime" != "$CI_DESIRED_RUNTIME" ]]; then
        die "Built podman is using '$runtime'; this CI environment requires $CI_DESIRED_RUNTIME"
    fi
    msg "Built podman is using expected runtime='$runtime'"
}

function _run_altbuild() {
    local -a arches
    local arch
    req_env_vars ALT_NAME
    # Var. defined in .cirrus.yml
    # shellcheck disable=SC2154
    msg "Performing alternate build: $ALT_NAME"
    msg "************************************************************"
    set -x
    cd $GOSRC
    case "$ALT_NAME" in
        *Each*)
            if [[ -z "$CIRRUS_PR" ]]; then
                echo ".....only meaningful on PRs"
                return
            fi
            showrun git fetch origin
            # The make-and-check-size script, introduced 2022-03-22 in #13518,
            # runs 'make' (the original purpose of this check) against
            # each commit, then checks image sizes to make sure that
            # none have grown beyond a given limit. That of course
            # requires a baseline, so our first step is to build the
            # branch point of the PR.
            local context_dir savedhead pr_base
            context_dir=$(mktemp -d --tmpdir make-size-check.XXXXXXX)
            savedhead=$(git rev-parse HEAD)
            # Push to PR base. First run of the script will write size files
            # shellcheck disable=SC2154
            pr_base=$PR_BASE_SHA
            showrun git checkout $pr_base
            showrun hack/make-and-check-size $context_dir
            # pop back to PR, and run incremental makes. Subsequent script
            # invocations will compare against original size.
            showrun git checkout $savedhead
            showrun git rebase $pr_base -x "hack/make-and-check-size $context_dir"
            rm -rf $context_dir
            ;;
        *Windows*)
	    showrun make .install.pre-commit
            showrun make lint GOOS=windows CGO_ENABLED=0
            showrun make podman-remote-release-windows_amd64.zip
            ;;
        *RPM*)
            showrun make package
            ;;
        Alt*x86*Cross)
            _build_altbuild_archs "386"
            ;;
        Alt*ARM*Cross)
            _build_altbuild_archs "arm"
            ;;
        Alt*Other*Cross)
            arches=(\
                ppc64le
                s390x)
            _build_altbuild_archs "${arches[@]}"
            ;;
        Alt*MIPS*Cross)
            arches=(\
                mips
                mipsle)
            _build_altbuild_archs "${arches[@]}"
            ;;
        Alt*MIPS64*Cross*)
            arches=(\
                mips64
                mips64le)
            _build_altbuild_archs "${arches[@]}"
            ;;
        Alt*RISCV64*Cross)
            _build_altbuild_archs "riscv64"
            ;;
        *)
            die "Unknown/Unsupported \$$ALT_NAME '$ALT_NAME'"
    esac
}

function _build_altbuild_archs() {
    for arch in "$@"; do
        msg "Building release archive for $arch"
        showrun make cross-binaries GOARCH=$arch
    done
}

function _run_release() {
    msg "podman info:"
    bin/podman info

    msg "Checking podman release (or potential release) criteria."
    # We're running under 'set -eo pipefail'; make sure this statement passes
    dev=$(bin/podman info |& grep -- -dev || echo -n '')
    if [[ -n "$dev" ]]; then
        die "Releases must never contain '-dev' in output of 'podman info' ($dev)"
    fi

    commit=$(bin/podman info --format='{{.Version.GitCommit}}' | tr -d '[:space:]')
    if [[ -z "$commit" ]]; then
        die "Releases must contain a non-empty Version.GitCommit in 'podman info'"
    fi
    msg "All OK"
}


# ***WARNING*** ***WARNING*** ***WARNING*** ***WARNING***
#    Please see gitlab comment in setup_environment.sh
# ***WARNING*** ***WARNING*** ***WARNING*** ***WARNING***
function _run_gitlab() {
    rootless_uid=$(id -u)
    systemctl enable --now --user podman.socket
    export DOCKER_HOST=unix:///run/user/${rootless_uid}/podman/podman.sock
    export CONTAINER_HOST=$DOCKER_HOST
    cd $GOPATH/src/gitlab.com/gitlab-org/gitlab-runner
    set +e
    go test -v ./executors/docker |& tee $GOSRC/gitlab-runner-podman.log
    ret=$?
    set -e
    # This file is collected and parsed by Cirrus-CI so must be in $GOSRC
    cat $GOSRC/gitlab-runner-podman.log | \
        go-junit-report > $GOSRC/gitlab-runner-podman.xml
    return $ret
}


# Name pattern for logformatter output file, derived from environment
function output_name() {
    # .cirrus.yml defines this as a short readable string for web UI
    std_name_fmt=$(sed -ne 's/^.*std_name_fmt \"\(.*\)\"/\1/p' <.cirrus.yml)
    test -n "$std_name_fmt" || die "Could not grep 'std_name_fmt' from .cirrus.yml"

    # Interpolate envariables. 'set -u' throws fatal if any are undefined
    (
        set -u
        eval echo "$std_name_fmt" | tr ' ' '-'
    )
}

function logformatter() {
    if [[ "$CI" == "true" ]]; then
        # Requires stdin and stderr combined!
        cat - \
            |& awk --file "${CIRRUS_WORKING_DIR}/${SCRIPT_BASE}/timestamp.awk" \
            |& "${CIRRUS_WORKING_DIR}/${SCRIPT_BASE}/logformatter" "$(output_name)"
    else
        # Assume script is run by a human, they want output immediately
        cat -
    fi
}

# Handle local|remote integration|system testing in a uniform way
dotest() {
    local testsuite="$1"
    req_env_vars testsuite CONTAINER TEST_ENVIRON PRIV_NAME

    # shellcheck disable=SC2154
    if ((CONTAINER==0)) && [[ "$TEST_ENVIRON" == "container" ]]; then
        exec_container  # does not return
    fi;

    # containers/automation sets this to 0 for its dbg() function
    # but the e2e integration tests are also sensitive to it.
    unset DEBUG

    # shellcheck disable=SC2154
    local localremote="$PODBIN_NAME"
    case "$PODBIN_NAME" in
        podman)  localremote="local" ;;
    esac

    # We've had some oopsies where tests invoke 'podman' instead of
    # /path/to/built/podman. Let's catch those.
    sudo rm -f /usr/bin/podman /usr/bin/podman-remote
    fallback_podman=$(type -p podman || true)
    if [[ -n "$fallback_podman" ]]; then
        die "Found fallback podman '$fallback_podman' in \$PATH; tests require none, as a guarantee that we're testing the right binary."
    fi

    # Catch invalid "TMPDIR == /tmp" assumptions; PR #19281
    TMPDIR=$(mktemp --tmpdir -d CI_XXXX)
    # tmp dir is commonly 1777 to allow all user to read/write
    chmod 1777 $TMPDIR
    export TMPDIR
    fstype=$(findmnt -n -o FSTYPE --target $TMPDIR)
    if [[ "$fstype" != "tmpfs" ]]; then
        die "The CI test TMPDIR is not on a tmpfs mount, we need tmpfs to make the tests faster"
    fi

    showrun make ${localremote}${testsuite} PODMAN_SERVER_LOG=$PODMAN_SERVER_LOG EXTRA_BUILDTAGS="$TEST_BUILD_TAGS" \
        |& logformatter

    # FIXME: https://github.com/containers/podman/issues/22642
    # Cannot delete this due cleanup errors, as the VM is basically
    # done after this anyway let's not block on this for now.
    rm -rf $TMPDIR
    unset TMPDIR
}

_run_machine-linux() {
    showrun make localmachine |& logformatter
}

# Nearly every task in .cirrus.yml makes use of this shell script
# wrapped by /usr/bin/time to collect runtime statistics.  Because the
# --output option is used to log stats to a file, every child-process
# inherits an open FD3 pointing at the log.  However, some testing
# operations depend on making use of FD3, and so it must be explicitly
# closed here (and for all further child-processes).
# STATS_LOGFILE assumed empty/undefined outside of Cirrus-CI (.cirrus.yml)
# shellcheck disable=SC2154
exec 3<&-

msg "************************************************************"
# Required to be defined by caller
# shellcheck disable=SC2154
msg "Runner executing $TEST_FLAVOR $PODBIN_NAME-tests as $PRIV_NAME on $DISTRO_NV($OS_REL_VER)"
if ((CONTAINER)); then
    # shellcheck disable=SC2154
    msg "Current environment container image: $CTR_FQIN"
else
    # shellcheck disable=SC2154
    msg "Current environment VM image: $VM_IMAGE_NAME"
fi
msg "************************************************************"

((${SETUP_ENVIRONMENT:-0})) || \
    die "Expecting setup_environment.sh to have completed successfully"

if [[ "$UID" -eq 0 ]] && ((CONTAINER==0)); then
    # start ebpf cleanup tracer (#23487)
    msg "start ebpf cleanup tracer"
    # replace zero bytes to make the log more readable
    bpftrace $GOSRC/hack/podman_cleanup_tracer.bt |& \
        tr '\0' ' ' >$GOSRC/podman-cleanup-tracer.log &
    TRACER_PID=$!
fi

# shellcheck disable=SC2154
if [[ "$PRIV_NAME" == "rootless" ]] && [[ "$UID" -eq 0 ]]; then
    # Remove /var/lib/cni, it is not required for rootless cni.
    # We have to test that it works without this directory.
    # https://github.com/containers/podman/issues/10857
    rm -rf /var/lib/cni

    # This must be done at the last second, otherwise `make` calls
    # in setup_environment (as root) will balk about ownership.
    msg "Recursively chowning \$GOPATH and \$GOSRC to $ROOTLESS_USER"
    if [[ $PRIV_NAME = "rootless" ]]; then
        chown -R $ROOTLESS_USER:$ROOTLESS_USER "$GOPATH" "$GOSRC"
    fi

    req_env_vars ROOTLESS_USER
    msg "Re-executing runner through ssh as user '$ROOTLESS_USER'"
    msg "************************************************************"
    set -x
    exec ssh $ROOTLESS_USER@localhost \
            -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no \
            -o CheckHostIP=no $GOSRC/$SCRIPT_BASE/runner.sh
    # Does not return!
fi
# else: not running rootless, do nothing special

# Dump important package versions. Before 2022-11-16 this took place as
# a separate .cirrus.yml step, but it really belongs here.
$(dirname $0)/logcollector.sh packages
msg "************************************************************"


cd "${GOSRC}/"

handler="_run_${TEST_FLAVOR}"

if [ "$(type -t $handler)" != "function" ]; then
    die "Unknown/Unsupported \$TEST_FLAVOR=$TEST_FLAVOR"
fi

# Unset NOTIFY_SOCKET based on: https://github.com/containers/podman/pull/27514#issuecomment-3529125596
unset NOTIFY_SOCKET

showrun $handler

if [[ -n "$TRACER_PID" ]]; then
    # ignore any error here
    kill "$TRACER_PID" || true
fi

showrun echo "finished"
