# -*- bash -*-

# Podman command to run; may be podman-remote
PODMAN=${PODMAN:-podman}
QUADLET=${QUADLET:-/usr/libexec/podman/quadlet}

# Podman testing helper used in 331-system-check tests
PODMAN_TESTING=${PODMAN_TESTING:-$(dirname ${BASH_SOURCE})/../../bin/podman-testing}

# crun or runc, unlikely to change. Cache, because it's expensive to determine.
PODMAN_RUNTIME=

# Standard image to use for most tests
PODMAN_TEST_IMAGE_REGISTRY=${PODMAN_TEST_IMAGE_REGISTRY:-"quay.io"}
PODMAN_TEST_IMAGE_USER=${PODMAN_TEST_IMAGE_USER:-"libpod"}
PODMAN_TEST_IMAGE_NAME=${PODMAN_TEST_IMAGE_NAME:-"testimage"}
PODMAN_TEST_IMAGE_TAG=${PODMAN_TEST_IMAGE_TAG:-"20241011"}
PODMAN_TEST_IMAGE_FQN="$PODMAN_TEST_IMAGE_REGISTRY/$PODMAN_TEST_IMAGE_USER/$PODMAN_TEST_IMAGE_NAME:$PODMAN_TEST_IMAGE_TAG"

# Larger image containing systemd tools.
PODMAN_SYSTEMD_IMAGE_NAME=${PODMAN_SYSTEMD_IMAGE_NAME:-"systemd-image"}
PODMAN_SYSTEMD_IMAGE_TAG=${PODMAN_SYSTEMD_IMAGE_TAG:-"20240124"}
PODMAN_SYSTEMD_IMAGE_FQN="$PODMAN_TEST_IMAGE_REGISTRY/$PODMAN_TEST_IMAGE_USER/$PODMAN_SYSTEMD_IMAGE_NAME:$PODMAN_SYSTEMD_IMAGE_TAG"

# Remote image that we *DO NOT* fetch or keep by default; used for testing pull
# This has changed in 2021, from 0 through 3, various iterations of getting
# multiarch to work. It should change only very rarely.
PODMAN_NONLOCAL_IMAGE_TAG=${PODMAN_NONLOCAL_IMAGE_TAG:-"00000004"}
PODMAN_NONLOCAL_IMAGE_FQN="$PODMAN_TEST_IMAGE_REGISTRY/$PODMAN_TEST_IMAGE_USER/$PODMAN_TEST_IMAGE_NAME:$PODMAN_NONLOCAL_IMAGE_TAG"

# Because who wants to spell that out each time?
IMAGE=$PODMAN_TEST_IMAGE_FQN
SYSTEMD_IMAGE=$PODMAN_SYSTEMD_IMAGE_FQN

# Default timeout for a podman command.
PODMAN_TIMEOUT=${PODMAN_TIMEOUT:-120}

function add_podman_args {
  declare -n arrayptr=$1

  if is_remote ; then
    case "${REMOTESYSTEM_TRANSPORT}" in
    tcp|tls|mtls)
      arrayptr+=(--url="tcp://localhost:${REMOTESYSTEM_TCP_PORT}")
      ;;
    unix)
      arrayptr+=(--url="unix://${REMOTESYSTEM_UNIX_SOCK}")
    esac
    case "${REMOTESYSTEM_TRANSPORT}" in
    tls|mtls)
      arrayptr+=(--tls-ca="${REMOTESYSTEM_TLS_CA_CRT}")
      ;;
    esac
    case "${REMOTESYSTEM_TRANSPORT}" in
    mtls)
      arrayptr+=(
        --tls-cert="${REMOTESYSTEM_TLS_CLIENT_CRT}"
        --tls-key="${REMOTESYSTEM_TLS_CLIENT_KEY}"
      )
      ;;
    esac
  fi
}


export REMOTESYSTEM_TLS_CA_CRT=${BATS_SUITE_TMPDIR}/remotesystem.ca.crt.pem
export REMOTESYSTEM_TLS_CA_KEY=${BATS_SUITE_TMPDIR}/remotesystem.ca.key.pem
export REMOTESYSTEM_TLS_SERVER_CRT=${BATS_SUITE_TMPDIR}/remotesystem.server.crt.pem
export REMOTESYSTEM_TLS_SERVER_KEY=${BATS_SUITE_TMPDIR}/remotesystem.server.key.pem
export REMOTESYSTEM_TLS_CLIENT_CRT=${BATS_SUITE_TMPDIR}/remotesystem.client.crt.pem
export REMOTESYSTEM_TLS_CLIENT_KEY=${BATS_SUITE_TMPDIR}/remotesystem.client.key.pem
export REMOTESYSTEM_TLS_BOGUS_CRT=${BATS_SUITE_TMPDIR}/remotesystem.bogus.crt.pem
export REMOTESYSTEM_TLS_BOGUS_KEY=${BATS_SUITE_TMPDIR}/remotesystem.bogus.key.pem

# Full command to run podman, including remote flags. This is (re)set by basic_setup.
PODMAN_CMD=()

# Prompt to display when logging podman commands; distinguish root/rootless
_LOG_PROMPT='$'
if [ $(id -u) -eq 0 ]; then
    _LOG_PROMPT='#'
fi

# Invocations via su may not set this. Although all container tools make
# an effort to determine a default if unset, there are corner cases (rootless
# namespace preservation) that run before the default is set.
# For purposes of system tests (CI, gating, OpenQA) we force a default early.
# As of September 2024 we no longer test the default-setting logic in the
# tools.
if [[ -z "$XDG_RUNTIME_DIR" ]] && [[ "$(id -u)" -ne 0 ]]; then
    export XDG_RUNTIME_DIR=/run/user/$(id -u)
fi

# Used in helpers.network, needed here in teardown
PORT_LOCK_DIR=$BATS_SUITE_TMPDIR/reserved-ports

###############################################################################
# BEGIN tools for fetching & caching test images
#
# Registries are flaky: any time we have to pull an image, that's a risk.
#

# Store in a semipermanent location. Not important for CI, but nice for
# developers so test restarts don't hang fetching images.
export PODMAN_IMAGECACHE=${BATS_TMPDIR:-/tmp}/podman-systest-imagecache-$(id -u)
mkdir -p ${PODMAN_IMAGECACHE}

function _prefetch() {
     local want=$1

     # Do we already have it in image store?
     run_podman '?' image exists "$want"
     if [[ $status -eq 0 ]]; then
         return
     fi

    # No image. Do we have it already cached? (Replace / and : with --)
    local cachename=$(sed -e 's;[/:];--;g' <<<"$want")
    local cachepath="${PODMAN_IMAGECACHE}/${cachename}.tar"
    if [[ ! -e "$cachepath" ]]; then
        # Not cached. Fetch it and cache it. Retry twice, because of flakes.
        cmd="skopeo copy --preserve-digests docker://$want oci-archive:$cachepath"
        echo "$_LOG_PROMPT $cmd"
        run $cmd
        echo "$output"
        if [[ $status -ne 0 ]]; then
            echo "# 'pull $want' failed, will retry..." >&3
            sleep 5

            run $cmd
            echo "$output"
            if [[ $status -ne 0 ]]; then
                echo "# 'pull $want' failed again, will retry one last time..." >&3
                sleep 30
                $cmd
            fi
        fi
    fi

    # Kludge alert.
    # Skopeo has no --storage-driver, --root, or --runroot flags; those
    # need to be expressed in the destination string inside [brackets].
    # See containers-transports(5). So if we see those options in
    # _PODMAN_TEST_OPTS, transmogrify $want into skopeo form.
    skopeo_opts=''
    driver="$(expr "$_PODMAN_TEST_OPTS" : ".*--storage-driver \([^ ]\+\)" || true)"
    if [[ -n "$driver" ]]; then
        skopeo_opts+="$driver@"
    fi

    altroot="$(expr "$_PODMAN_TEST_OPTS" : ".*--root \([^ ]\+\)" || true)"
    if [[ -n "$altroot" ]] && [[ -d "$altroot" ]]; then
        skopeo_opts+="$altroot"

        altrunroot="$(expr "$_PODMAN_TEST_OPTS" : ".*--runroot \([^ ]\+\)" || true)"
        if [[ -n "$altrunroot" ]] && [[ -d "$altrunroot" ]]; then
            skopeo_opts+="+$altrunroot"
        fi
    fi

    if [[ -n "$skopeo_opts" ]]; then
        want="[$skopeo_opts]$want"
    fi

    # Cached image is now guaranteed to exist. Be sure to load it
    # with skopeo, not podman, in order to preserve metadata
    cmd="skopeo copy --all oci-archive:$cachepath containers-storage:$want"
    echo "$_LOG_PROMPT $cmd"
    $cmd
}

# END   tools for fetching & caching test images
###############################################################################
# BEGIN setup/teardown tools

# Provide common setup and teardown functions, but do not name them such!
# That way individual tests can override with their own setup/teardown,
# while retaining the ability to include these if they so desire.

# Setup helper: establish a test environment with exactly the images needed
function basic_setup() {
    # Temporary subdirectory, in which tests can write whatever they like
    # and trust that it'll be deleted on cleanup.
    # (BATS v1.3 and above provide $BATS_TEST_TMPDIR, but we still use
    # ancient BATS (v1.1) in RHEL gating tests.)
    PODMAN_TMPDIR=$(mktemp -d --tmpdir=${BATS_TMPDIR:-/tmp} podman_bats.XXXXXX)

    PODMAN_CMD=("${PODMAN}")
    add_podman_args PODMAN_CMD

    # runtime is not likely to change
    if [[ -z "$PODMAN_RUNTIME" ]]; then
        PODMAN_RUNTIME=$(podman_runtime)
    fi

    # In the unlikely event that a test runs is() before a run_podman()
    MOST_RECENT_PODMAN_COMMAND=

    # Test filenames must match ###-name.bats; use "[###] " as prefix
    run expr "$BATS_TEST_FILENAME" : "^.*/\([0-9]\{3\}\)-[^/]\+\.bats\$"
    # If parallel, use |nnn|. Serial, [nnn]
    if [[ -n "$PARALLEL_JOBSLOT" ]]; then
        BATS_TEST_NAME_PREFIX="|${output}| "
    else
        BATS_TEST_NAME_PREFIX="[${output}] "
    fi

    # By default, assert() and die() cause an immediate test failure.
    # Under special circumstances (usually long test loops), tests
    # can call defer-assertion-failures() to continue going, the
    # idea being that a large number of failures can show patterns.
    ASSERTION_FAILURES=
    immediate-assertion-failures
}

# bail-now is how we terminate a test upon assertion failure.
# By default, and the vast majority of the time, it just triggers
# immediate test termination; but see defer-assertion-failures, below.
function bail-now() {
    # "false" does not apply to "bail now"! It means "nonzero exit",
    # which BATS interprets as "yes, bail immediately".
    false
}

# Invoked on teardown: will terminate immediately if there have been
# any deferred test failures; otherwise will reset back to immediate
# test termination on future assertions.
function immediate-assertion-failures() {
    function bail-now() {
        false
    }

    # Any backlog?
    if [[ -n "$ASSERTION_FAILURES" ]]; then
        local n=${#ASSERTION_FAILURES}
        ASSERTION_FAILURES=
        die "$n test assertions failed. Search for 'FAIL:' above this line." >&2
    fi
}

# Used in special test circumstances--typically multi-condition loops--to
# continue going even on assertion failures. The test will fail afterward,
# usually in teardown. This can be useful to show failure patterns.
function defer-assertion-failures() {
    function bail-now() {
        ASSERTION_FAILURES+="!"
    }
}

# Basic teardown: remove all pods and containers
function basic_teardown() {
    echo "# [teardown]" >&2
    # Debug for https://github.com/containers/podman/issues/22642
    ls -l $TMPDIR >&3

    PODMAN_CMD=("${PODMAN}")
    add_podman_args PODMAN_CMD

    # Free any ports reserved by our test
    if [[ -d $PORT_LOCK_DIR ]]; then
        mylocks=$(grep -wlr $BATS_SUITE_TEST_NUMBER $PORT_LOCK_DIR || true)
        if [[ -n "$mylocks" ]]; then
            rm -f $mylocks
        fi
    fi

    immediate-assertion-failures
    # Unlike normal tests teardown will not exit on first command failure
    # but rather only uses the return code of the teardown function.
    # This must be directly after immediate-assertion-failures to capture the error code
    local exit_code=$?

    # Leak check and state reset. Only run these when running tests serially!
    # (For parallel tests, we run a leak check only at the very end of all tests)
    if [[ -z "$PARALLEL_JOBSLOT" ]]; then
        # Check for leaks, but only if:
        #  1) test was successful (BATS_TEST_COMPLETED is set 1); and
        #  2) immediate-assertion-failures didn't fail (exit_code -eq 0); and
        #  3) PODMAN_BATS_LEAK_CHECK is set (usually only in cron).
        # As these podman commands are slow we do not want to do this by default
        # and only provide this as opt-in option. (#22909)
        if [[ "$BATS_TEST_COMPLETED" -eq 1 ]] && [[ $exit_code -eq 0 ]] && [[ -n "$PODMAN_BATS_LEAK_CHECK" ]]; then
            leak_check
            exit_code=$((exit_code + $?))
        fi

        # Some error happened (either in teardown itself or the actual test failed)
        # so do a full cleanup to ensure following tests start with a clean env.
        if [ $exit_code -gt 0 ] || [ -z "$BATS_TEST_COMPLETED" ]; then
            clean_setup
            exit_code=$((exit_code + $?))
        fi
    fi

    # Status file used in teardown_suite() to decide whether or not
    # to check for leaks
    if [[ "$BATS_TEST_COMPLETED" -ne 1 ]]; then
        rm -f "$BATS_SUITE_TMPDIR/all-tests-passed"
    fi

    command rm -rf $PODMAN_TMPDIR
    exit_code=$((exit_code + $?))
    return $exit_code
}


# Provide the above as default methods.
function setup() {
    basic_setup
}

function teardown() {
    basic_teardown
}


# Helpers useful for tests running rmi
function archive_image() {
    local image=$1

    # FIXME: refactor?
    archive_basename=$(echo $1 | tr -c a-zA-Z0-9._- _)
    archive=$BATS_TMPDIR/$archive_basename.tar

    run_podman save -o $archive $image
}

function restore_image() {
    local image=$1

    archive_basename=$(echo $1 | tr -c a-zA-Z0-9._- _)
    archive=$BATS_TMPDIR/$archive_basename.tar

    run_podman restore $archive
}

#######################
#  _run_podman_quiet  #  Helper for leak_check. Runs podman with no logging
#######################
function _run_podman_quiet() {
    # This should be the same as what run_podman() does.
    run timeout -v --foreground --kill=10 60 ${PODMAN_CMD[@]} $_PODMAN_TEST_OPTS "$@"
    if [[ $status -ne 0 ]]; then
        echo "# Error running command: podman $*"
        echo "$output"
        exit_code=$((exit_code + 1))
    fi
}

#####################
#  _leak_check_one  #  Helper for leak_check: shows leaked artifacts
#####################
#
# NOTE: plays fast & loose with variables! Reads $output, updates $exit_code
#
function _leak_check_one() {
    local what="$1"

    # Shown the first time we see a stray of this kind
    separator="vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv
"

    while read line; do
        if [[ -n "$line" ]]; then
            echo "${separator}*** Leaked $what: $line"
            separator=""
            exit_code=$((exit_code + 1))
        fi
    done <<<"$output"
}

################
#  leak_check  #  Look for, and warn about, stray artifacts
################
#
# Runs on test failure, or at end of all tests, or when PODMAN_BATS_LEAK_CHECK=1
#
# Note that all ps/ls commands specify a format where something useful
# (ID or name) is in the first column. This is not important today
# (July 2024) but may be useful one day: a future PR may run bats
# with --gather-test-outputs-in, which preserves logs of all tests.
# Why not today? Because that option is still buggy: (1) we need
# bats-1.11 to fix a more-than-one-slash-in-test-name bug, (2) as
# of July 2024 that option only copies logs of *completed* tests
# to the directory, so currently-running tests (the one running
# teardown, or, in parallel mode, any other running tests) are
# not seen. This renders that option less useful, and not worth
# bothering with at the moment. But please leave ID-or-name as
# the first column anyway because things may change and it's
# a reasonable format anyway.
#
function leak_check() {
    local exit_code=0

    # Volumes.
    _run_podman_quiet volume ls --format '{{.Name}} {{.Driver}}'
    _leak_check_one "volume"

    # Networks. "podman" and "podman-default-kube-network" are OK.
    _run_podman_quiet network ls --noheading
    output=$(grep -ve "^[0-9a-z]\{12\} *podman\(-default-kube-network\)\? *bridge\$" <<<"$output")
    _leak_check_one "network"

    # Pods, containers, and external (buildah) containers.
    _run_podman_quiet pod ls --format '{{.ID}} {{.Name}} status={{.Status}} ({{.NumberOfContainers}} containers)'
    _leak_check_one "pod"

    _run_podman_quiet ps -a --format '{{.ID}} {{.Image}} {{.Names}}  {{.Status}}'
    _leak_check_one "container"

    _run_podman_quiet ps -a --external --filter=status=unknown --format '{{.ID}} {{.Image}} {{.Names}}  {{.Status}}'
    _leak_check_one "storage container"

    # Images. Exclude our standard expected images.
    _run_podman_quiet images --all --format '{{.ID}} {{.Repository}}:{{.Tag}}'
    output=$(awk "\$2 != \"$IMAGE\" && \$2 != \"$PODMAN_SYSTEMD_IMAGE_FQN\" && \$2 !~ \"localhost/podman-pause:\" { print }" <<<"$output")
    _leak_check_one "image"

    return $exit_code
}

function clean_setup() {
    local actions=(
        "pod rm -t 0 --all --force --ignore"
            "rm -t 0 --all --force --ignore"
        "network prune --force"
        "volume rm -a -f"
    )
    for action in "${actions[@]}"; do
        _run_podman_quiet $action

        # The -f commands should never exit nonzero, but if they do we want
        # to know about it.
        #   FIXME: someday: also test for [[ -n "$output" ]] - can't do this
        #   yet because too many tests don't clean up their containers
        if [[ $status -ne 0 ]]; then
            echo "# [teardown] $_LOG_PROMPT podman $action" >&3
            for line in "${lines[@]}"; do
                echo "# $line" >&3
            done

            # Special case for timeout: check for locks (#18514)
            if [[ $status -eq 124 ]]; then
                echo "# [teardown] $_LOG_PROMPT podman system locks" >&3
                run "${PODMAN_CMD[@]}" system locks
                for line in "${lines[@]}"; do
                    echo "# $line" >&3
                done
            fi
        fi
    done

    # ...including external (buildah) ones
    _run_podman_quiet ps --all --external --format '{{.ID}} {{.Names}}'
    for line in "${lines[@]}"; do
        set $line
        echo "# setup(): removing stray external container $1 ($2)" >&3
        run_podman '?' rm -f $1
        if [[ $status -ne 0 ]]; then
            echo "# [setup] $_LOG_PROMPT podman rm -f $1" >&3
            for errline in "${lines[@]}"; do
                echo "# $errline" >&3
            done
        fi
    done

    # Clean up all images except those desired.
    # 2023-06-26 REMINDER: it is tempting to think that this is clunky,
    # wouldn't it be safer/cleaner to just 'rmi -a' then '_prefetch $IMAGE'?
    # Yes, but it's also tremendously slower: 29m for a CI run, to 39m.
    # Image loads are slow.
    found_needed_image=
    _run_podman_quiet images --all --format '{{.Repository}}:{{.Tag}} {{.ID}}'

    for line in "${lines[@]}"; do
        set $line
        if [[ "$1" == "$PODMAN_TEST_IMAGE_FQN" ]]; then
            if [[ -z "$PODMAN_TEST_IMAGE_ID" ]]; then
                # This will probably only trigger the 2nd time through setup
                PODMAN_TEST_IMAGE_ID=$2
            fi
            found_needed_image=1
        elif [[ "$1" == "$PODMAN_SYSTEMD_IMAGE_FQN" ]]; then
            # This is a big image, don't force unnecessary pulls
            :
        else
            # Always remove image that doesn't match by name
            echo "# setup(): removing stray image $1" >&3
            _run_podman_quiet rmi --force "$1"

            # Tagged image will have same IID as our test image; don't rmi it.
            if [[ $2 != "$PODMAN_TEST_IMAGE_ID" ]]; then
                echo "# setup(): removing stray image $2" >&3
                _run_podman_quiet rmi --force "$2"
            fi
        fi
    done

    # Make sure desired image is present
    if [[ -z "$found_needed_image" ]]; then
        _prefetch $PODMAN_TEST_IMAGE_FQN
    fi
}

# END   setup/teardown tools
###############################################################################
# BEGIN podman helpers

# Displays '[HH:MM:SS.NNNNN]' in command output. logformatter relies on this.
function timestamp() {
    date +'[%T.%N]'
}

################
#  run_podman  #  Invoke $PODMAN, with timeout, using BATS 'run'
################
#
# This is the preferred mechanism for invoking podman: first, it
# invokes $PODMAN, which may be 'podman-remote' or '/some/path/podman'.
#
# Second, we use 'timeout' to abort (with a diagnostic) if something
# takes too long; this is preferable to a CI hang.
#
# Third, we log the command run and its output. This doesn't normally
# appear in BATS output, but it will if there's an error.
#
# Next, we check exit status. Since the normal desired code is 0,
# that's the default; but the first argument can override:
#
#     run_podman 125  nonexistent-subcommand
#     run_podman '?'  some-other-command       # let our caller check status
#
# Since we use the BATS 'run' mechanism, $output and $status will be
# defined for our caller.
#
function run_podman() {
    # Number as first argument = expected exit code; default 0
    # "0+[we]" = require success, but allow warnings/errors
    local expected_rc=0
    local allowed_levels="dit"
    case "$1" in
        0\+[we]*)        allowed_levels+=$(expr "$1" : "^0+\([we]\+\)"); shift;;
        [0-9])           expected_rc=$1; shift;;
        [1-9][0-9])      expected_rc=$1; shift;;
        [12][0-9][0-9])  expected_rc=$1; shift;;
        '?')             expected_rc=  ; shift;;  # ignore exit code
    esac

    # Remember command args, for possible use in later diagnostic messages
    MOST_RECENT_PODMAN_COMMAND="podman $*"

    # BATS >= 1.5.0 treats 127 as a special case, adding a big nasty warning
    # at the end of the test run if any command exits thus. Silence it.
    #   https://bats-core.readthedocs.io/en/stable/warnings/BW01.html
    local silence127=
    if [[ "$expected_rc" = "127" ]]; then
        # We could use "-127", but that would cause BATS to fail if the
        # command exits any other status -- and default BATS failure messages
        # are much less helpful than the run_podman ones. "!" is more flexible.
        silence127="!"
    fi

    podman_args=()
    add_podman_args podman_args


    # stdout is only emitted upon error; this printf is to help in debugging
    printf "\n%s %s %s %s %s\n" "$(timestamp)" "$_LOG_PROMPT" $PODMAN "${podman_args[*]}" "$*"

    # BATS hangs if a subprocess remains and keeps FD 3 open; this happens
    # if podman crashes unexpectedly without cleaning up subprocesses.
    run $silence127 timeout --foreground -v --kill=10 $PODMAN_TIMEOUT $PODMAN "${podman_args[@]}" $_PODMAN_TEST_OPTS "$@" 3>/dev/null
    # without "quotes", multiple lines are glommed together into one
    if [ -n "$output" ]; then
        echo "$(timestamp) $output"

        # FIXME FIXME FIXME: instrumenting to track down #15488. Please
        # remove once that's fixed. We include the args because, remember,
        # bats only shows output on error; it's possible that the first
        # instance of the metacopy warning happens in a test that doesn't
        # check output, hence doesn't fail.
        if [[ "$output" =~ Ignoring.global.metacopy.option ]]; then
            echo "# YO! metacopy warning triggered by: podman $*" >&3
        fi
    fi
    if [ "$status" -ne 0 ]; then
        echo -n "$(timestamp) [ rc=$status ";
        if [ -n "$expected_rc" ]; then
            if [ "$status" -eq "$expected_rc" ]; then
                echo -n "(expected) ";
            else
                echo -n "(** EXPECTED $expected_rc **) ";
            fi
        fi
        echo "]"
    fi

    if [ "$status" -eq 124 ]; then
        if expr "$output" : ".*timeout: sending" >/dev/null; then
            # It's possible for a subtest to _want_ a timeout
            if [[ "$expected_rc" != "124" ]]; then
                echo "*** TIMED OUT ***"
                false
            fi
        fi
    fi

    if [ -n "$expected_rc" ]; then
        if [ "$status" -ne "$expected_rc" ]; then
            die "exit code is $status; expected $expected_rc"
        fi
    fi

    # Check for "level=<unexpected>" in output, because a successful command
    # should never issue unwanted warnings or errors. The "0+w" convention
    # (see top of function) allows our caller to indicate that warnings are
    # expected, e.g., "podman stop" without -t0.
    if [[ $status -eq 0 ]]; then
        # FIXME: don't do this on Debian or RHEL. runc is way too buggy:
        #   - #11784 - lstat /sys/fs/.../*.scope: ENOENT
        #   - #11785 - cannot toggle freezer: cgroups not configured
        # As of January 2024 the freezer one seems to be fixed in Debian-runc
        # but not in RHEL8-runc. The lstat one is closed-wontfix.
        if [[ $PODMAN_RUNTIME != "runc" ]]; then
            # FIXME: All kube commands emit unpredictable errors:
            #    "Storage for container <X> has been removed"
            #    "no container with ID <X> found in database"
            # These are level=error but we still get exit-status 0.
            # Just skip all kube commands completely
            if [[ ! "$*" =~ kube ]]; then
                if [[ "$output" =~ level=[^${allowed_levels}] ]]; then
                    die "Command succeeded, but issued unexpected warnings"
                fi
            fi
        fi
    fi
}

function run_podman_testing() {
    printf "\n%s %s %s %s\n" "$(timestamp)" "$_LOG_PROMPT" "$PODMAN_TESTING" "$*"
    run $PODMAN_TESTING "$@"
    if [[ $status -ne 0 ]]; then
        echo "$output"
        die "Unexpected error from testing helper, which should always always succeed"
    fi
}

# Wait for certain output from a container, indicating that it's ready.
function wait_for_output {
    local sleep_delay=1
    local how_long=$PODMAN_TIMEOUT
    local expect=
    local cid=

    # Arg processing. A single-digit number is how long to sleep between
    # iterations; a 2- or 3-digit number is the total time to wait; all
    # else are, in order, the string to expect and the container name/ID.
    local i
    for i in "$@"; do
        if expr "$i" : '[0-9]\+$' >/dev/null; then
            if [ $i -le 9 ]; then
                sleep_delay=$i
            else
                how_long=$i
            fi
        elif [ -z "$expect" ]; then
            expect=$i
        else
            cid=$i
        fi
    done

    [ -n "$cid" ] || die "FATAL: wait_for_output: no container name/ID in '$*'"

    t1=$(expr $SECONDS + $how_long)
    while [ $SECONDS -lt $t1 ]; do
        run_podman 0+w logs $cid
        logs=$output
        if expr "$logs" : ".*$expect" >/dev/null; then
            return
        fi

        # Barf if container is not running
        run_podman inspect --format '{{.State.Running}}' $cid
        if [ $output != "true" ]; then
            run_podman inspect --format '{{.State.ExitCode}}' $cid
            exitcode=$output

            # One last chance: maybe the container exited just after logs cmd
            run_podman 0+w logs $cid
            if expr "$logs" : ".*$expect" >/dev/null; then
                return
            fi

            die "Container exited (status: $exitcode) before we saw '$expect': $logs"
        fi

        sleep $sleep_delay
    done

    die "timed out waiting for '$expect' from $cid"
}

# Shortcut for the lazy
function wait_for_ready {
    wait_for_output 'READY' "$@"
}

###################
#  wait_for_file  #  Returns once file is available on host
###################
function wait_for_file() {
    local file=$1                       # The path to the file
    local _timeout=${2:-5}              # Optional; default 5 seconds

    # Wait
    while [ $_timeout -gt 0 ]; do
        test -e $file && return
        sleep 1
        _timeout=$(( $_timeout - 1 ))
    done

    die "Timed out waiting for $file"
}

###########################
#  wait_for_file_content  #  Like wait_for_output, but with files (not ctrs)
###########################
function wait_for_file_content() {
    local file=$1                       # The path to the file
    local content=$2                    # What to expect in the file
    local _timeout=${3:-5}              # Optional; default 5 seconds

    while :; do
        grep -q "$content" "$file" && return

        test $_timeout -gt 0 || die "Timed out waiting for '$content' in $file"

        _timeout=$(( $_timeout - 1 ))
        sleep 1

        # For debugging. Note that file does not necessarily exist yet.
        if [[ -e "$file" ]]; then
            echo "[ wait_for_file_content: retrying wait for '$content' in: ]"
            sed -e 's/^/[ /' -e 's/$/ ]/' <"$file"
        else
            echo "[ wait_for_file_content: $file does not exist (yet) ]"
        fi
    done
}

# END   podman helpers
###############################################################################
# BEGIN miscellaneous tools

# Shortcuts for common needs:
function is_rootless() {
    [ "$(id -u)" -ne 0 ]
}

function is_remote() {
    [[ "$PODMAN" =~ -remote ]]
}

function is_cgroupsv1() {
    # WARNING: This will break if there's ever a cgroups v3
    ! is_cgroupsv2
}

# True if cgroups v2 are enabled
function is_cgroupsv2() {
    cgroup_type=$(stat -f -c %T /sys/fs/cgroup)
    test "$cgroup_type" = "cgroup2fs"
}

# True if podman is using netavark
function is_netavark() {
    run_podman info --format '{{.Host.NetworkBackend}}'
    if [[ "$output" =~ netavark ]]; then
        return 0
    fi
    return 1
}

function is_aarch64() {
    [ "$(uname -m)" == "aarch64" ]
}

function selinux_enabled() {
    /usr/sbin/selinuxenabled 2> /dev/null
}

# Returns the OCI runtime *basename* (typically crun or runc). Much as we'd
# love to cache this result, we probably shouldn't.
function podman_runtime() {
    # This function is intended to be used as '$(podman_runtime)', i.e.
    # our caller wants our output. It's unsafe to use run_podman().
    runtime=$("${PODMAN_CMD[@]}" info --format '{{ .Host.OCIRuntime.Name }}' 2>/dev/null)
    basename "${runtime:-[null]}"
}

# Returns the storage driver: 'overlay' or 'vfs'
function podman_storage_driver() {
    run_podman info --format '{{.Store.GraphDriverName}}' >/dev/null
    # Should there ever be a new driver
    case "$output" in
        overlay) ;;
        vfs)     ;;
        *)       die "Unknown storage driver '$output'; if this is a new driver, please review uses of this function in tests." ;;
    esac
    echo "$output"
}

# Given a (scratch) directory path, returns a set of command-line options
# for running an isolated podman that will not step on system podman. Set:
#  - rootdir, so we don't clobber real images or storage;
#  - tmpdir, so we use an isolated DB; and
#  - runroot, out of an abundance of paranoia
function podman_isolation_opts() {
    local path=${1?podman_isolation_opts: missing PATH arg}

    for opt in root runroot tmpdir;do
        mkdir -p $path/$opt
        echo " --$opt $path/$opt"
    done
}

# rhbz#1895105: rootless journald is unavailable except to users in
# certain magic groups; which our testuser account does not belong to
# (intentional: that is the RHEL default, so that's the setup we test).
function journald_unavailable() {
    if ! is_rootless; then
        # root must always have access to journal
        return 1
    fi

    run journalctl -n 1
    if [[ $status -eq 0 ]]; then
        return 1
    fi

    if [[ $output =~ permission ]]; then
        return 0
    fi

    # This should never happen; if it does, it's likely that a subsequent
    # test will fail. This output may help track that down.
    echo "WEIRD: 'journalctl -n 1' failed with a non-permission error:"
    echo "$output"
    return 1
}

# Wait for the pod (1st arg) to transition into the state (2nd arg)
function _ensure_pod_state() {
    for i in {0..5}; do
        run_podman pod inspect $1 --format "{{.State}}"
        if [[ $output == "$2" ]]; then
            return
        fi
        sleep 0.5
    done

    die "Timed out waiting for pod $1 to enter state $2"
}

# Wait for the container's (1st arg) running state (2nd arg)
function _ensure_container_running() {
    for i in {0..20}; do
        run_podman container inspect $1 --format "{{.State.Running}}"
        if [[ $output == "$2" ]]; then
            return
        fi
        sleep 0.5
    done

    die "Timed out waiting for container $1 to enter state running=$2"
}

###########################
#  _add_label_if_missing  #  make sure skip messages include rootless/remote
###########################
function _add_label_if_missing() {
    local msg="$1"
    local want="$2"

    if [ -z "$msg" ]; then
        echo
    elif expr "$msg" : ".*$want" &>/dev/null; then
        echo "$msg"
    else
        echo "[$want] $msg"
    fi
}

######################
#  skip_if_no_ssh #  ...with an optional message
######################
function skip_if_no_ssh() {
    if no_ssh; then
        local msg=$(_add_label_if_missing "$1" "ssh")
        skip "${msg:-not applicable with no ssh binary}"
    fi
}

######################
#  skip_if_rootless  #  ...with an optional message
######################
function skip_if_rootless() {
    if is_rootless; then
        local msg=$(_add_label_if_missing "$1" "rootless")
        skip "${msg:-not applicable under rootless podman}"
    fi
}

######################
#  skip_if_not_rootless  #  ...with an optional message
######################
function skip_if_not_rootless() {
    if ! is_rootless; then
        local msg=$(_add_label_if_missing "$1" "rootful")
        skip "${msg:-not applicable under rootlfull podman}"
    fi
}

####################
#  skip_if_remote  #  ...with an optional message
####################
function skip_if_remote() {
    if is_remote; then
        local msg=$(_add_label_if_missing "$1" "remote")
        skip "${msg:-test does not work with podman-remote}"
    fi
}

########################
#  skip_if_no_selinux  #
########################
function skip_if_no_selinux() {
    if [ ! -e /usr/sbin/selinuxenabled ]; then
        skip "selinux not available"
    elif ! /usr/sbin/selinuxenabled; then
        skip "selinux disabled"
    fi
}

##################################
#  skip_if_journald_unavailable  #  rhbz#1895105: rootless journald permissions
##################################
function skip_if_journald_unavailable {
    if journald_unavailable; then
        skip "Cannot use rootless journald on this system"
    fi
}

function skip_if_aarch64 {
    if is_aarch64; then
        skip "${msg:-Cannot run this test on aarch64 systems}"
    fi
}

#########
#  die  #  Abort with helpful message
#########
function die() {
    # FIXME: handle multi-line output
    echo "#/vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv"  >&2
    echo "#| FAIL: $*"                                           >&2
    echo "#\\^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^" >&2
    bail-now
}

############
#  assert  #  Compare actual vs expected string; fail if mismatch
############
#
# Compares string (default: $output) against the given string argument.
# By default we do an exact-match comparison against $output, but there
# are two different ways to invoke us, each with an optional description:
#
#      assert               "EXPECT" [DESCRIPTION]
#      assert "RESULT" "OP" "EXPECT" [DESCRIPTION]
#
# The first form (one or two arguments) does an exact-match comparison
# of "$output" against "EXPECT". The second (three or four args) compares
# the first parameter against EXPECT, using the given OPerator. If present,
# DESCRIPTION will be displayed on test failure.
#
# Examples:
#
#   assert "this is exactly what we expect"
#   assert "${lines[0]}" =~ "^abc"  "first line begins with abc"
#
function assert() {
    local actual_string="$output"
    local operator='=='
    local expect_string="$1"
    local testname="$2"

    case "${#*}" in
        0)   die "Internal error: 'assert' requires one or more arguments" ;;
        1|2) ;;
        3|4) actual_string="$1"
             operator="$2"
             expect_string="$3"
             testname="$4"
             ;;
        *)   die "Internal error: too many arguments to 'assert'" ;;
    esac

    # Comparisons.
    # Special case: there is no !~ operator, so fake it via '! x =~ y'
    local not=
    local actual_op="$operator"
    if [[ $operator == '!~' ]]; then
        not='!'
        actual_op='=~'
    fi
    if [[ $operator == '=' || $operator == '==' ]]; then
        # Special case: we can't use '=' or '==' inside [[ ... ]] because
        # the right-hand side is treated as a pattern... and '[xy]' will
        # not compare literally. There seems to be no way to turn that off.
        if [ "$actual_string" = "$expect_string" ]; then
            return
        fi
    elif [[ $operator == '!=' ]]; then
        # Same special case as above
        if [ "$actual_string" != "$expect_string" ]; then
            return
        fi
    else
        if eval "[[ $not \$actual_string $actual_op \$expect_string ]]"; then
            return
        elif [ $? -gt 1 ]; then
            die "Internal error: could not process 'actual' $operator 'expect'"
        fi
    fi

    # Test has failed. Get a descriptive test name.
    if [ -z "$testname" ]; then
        testname="${MOST_RECENT_PODMAN_COMMAND:-[no test name given]}"
    fi

    # Display optimization: the typical case for 'expect' is an
    # exact match ('='), but there are also '=~' or '!~' or '-ge'
    # and the like. Omit the '=' but show the others; and always
    # align subsequent output lines for ease of comparison.
    local op=''
    local ws=''
    if [ "$operator" != '==' ]; then
        op="$operator "
        ws=$(printf "%*s" ${#op} "")
    fi

    # This is a multi-line message, which may in turn contain multi-line
    # output, so let's format it ourself to make it more readable.
    local expect_split
    mapfile -t expect_split <<<"$expect_string"
    local actual_split
    mapfile -t actual_split <<<"$actual_string"

    # bash %q is really nice, except for the way it backslashes spaces
    local -a expect_split_q
    for line in "${expect_split[@]}"; do
        local q=$(printf "%q" "$line" | sed -e 's/\\ / /g')
        expect_split_q+=("$q")
    done
    local -a actual_split_q
    for line in "${actual_split[@]}"; do
        local q=$(printf "%q" "$line" | sed -e 's/\\ / /g')
        actual_split_q+=("$q")
    done

    printf "#/vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv\n"    >&2
    printf "#|     FAIL: %s\n" "$testname"                        >&2
    printf "#| expected: %s%s\n" "$op" "${expect_split_q[0]}"     >&2
    local line
    for line in "${expect_split_q[@]:1}"; do
        printf "#|         > %s%s\n" "$ws" "$line"                >&2
    done
    printf "#|   actual: %s%s\n" "$ws" "${actual_split_q[0]}"     >&2
    for line in "${actual_split_q[@]:1}"; do
        printf "#|         > %s%s\n" "$ws" "$line"                >&2
    done
    printf "#\\^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^\n"   >&2
    bail-now
}

########
#  is  #  **DEPRECATED**; see assert() above
########
function is() {
    local actual="$1"
    local expect="$2"
    local testname="${3:-${MOST_RECENT_PODMAN_COMMAND:-[no test name given]}}"

    local is_expr=
    if [ -z "$expect" ]; then
        if [ -z "$actual" ]; then
            # Both strings are empty.
            return
        fi
        expect='[no output]'
    elif [[ "$actual" = "$expect" ]]; then
        # Strings are identical.
        return
    else
        # Strings are not identical. Are there wild cards in our expect string?
        if expr "$expect" : ".*[^\\][\*\[]" >/dev/null; then
            # There is a '[' or '*' without a preceding backslash.
            is_expr=' (using expr)'
        elif [[ "${expect:0:1}" = '[' ]]; then
            # String starts with '[', e.g. checking seconds like '[345]'
            is_expr=' (using expr)'
        fi
        if [[ -n "$is_expr" ]]; then
            if expr "$actual" : "$expect" >/dev/null; then
                return
            fi
        fi
    fi

    # This is a multi-line message, which may in turn contain multi-line
    # output, so let's format it ourself to make it more readable.
    local -a actual_split
    readarray -t actual_split <<<"$actual"
    printf "#/vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv\n" >&2
    printf "#|     FAIL: $testname\n"                          >&2
    printf "#| expected: '%s'%s\n" "$expect" "$is_expr"        >&2
    printf "#|   actual: '%s'\n" "${actual_split[0]}"          >&2
    local line
    for line in "${actual_split[@]:1}"; do
        printf "#|         > '%s'\n" "$line"                   >&2
    done
    printf "#\\^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^\n" >&2
    bail-now
}

####################
#  allow_warnings  #  check cmd output for warning messages other than these
####################
#
# HEADS UP: Operates on '$lines' array, so, must be invoked after run_podman
#
function allow_warnings() {
    for line in "${lines[@]}"; do
        if [[ "$line" =~ level=[we] ]]; then
            local ok=
            for pattern in "$@"; do
                if [[ "$line" =~ $pattern ]]; then
                   ok=ok
                fi
            done
            if [[ -z "$ok" ]]; then
                die "Unexpected warning/error in command results: $line"
            fi
        fi
    done
}

#####################
#  require_warning  #  Require the given message, but disallow any others
#####################
# Optional 2nd argument is a message to display if warning is missing
function require_warning() {
    local expect="$1"
    local msg="${2:-Did not find expected warning/error message}"
    assert "$output" =~ "$expect" "$msg"
    allow_warnings "$expect"
}

############
#  dprint  #  conditional debug message
############
#
# Set PODMAN_TEST_DEBUG to the name of one or more functions you want to debug
#
# Examples:
#
#    $ PODMAN_TEST_DEBUG=parse_table bats .
#    $ PODMAN_TEST_DEBUG="test_podman_images test_podman_run" bats .
#
function dprint() {
    test -z "$PODMAN_TEST_DEBUG" && return

    caller="${FUNCNAME[1]}"

    # PODMAN_TEST_DEBUG is a space-separated list of desired functions
    # e.g. "parse_table test_podman_images" (or even just "table")
    for want in $PODMAN_TEST_DEBUG; do
        # Check if our calling function matches any of the desired strings
        if expr "$caller" : ".*$want" >/dev/null; then
            echo "# ${FUNCNAME[1]}() : $*" >&3
            return
        fi
    done
}


#################
#  parse_table  #  Split a table on '|' delimiters; return space-separated
#################
#
# See sample .bats scripts for examples. The idea is to list a set of
# tests in a table, then use simple logic to iterate over each test.
# Columns are separated using '|' (pipe character) because sometimes
# we need spaces in our fields.
#
function parse_table() {
    while read line; do
        test -z "$line" && continue

        declare -a row=()
        while read col; do
            dprint "col=<<$col>>"
            row+=("$col")
        done <  <(echo "$line" | sed -E -e 's/(^|\s)\|(\s|$)/\n /g' | sed -e 's/^ *//' -e 's/\\/\\\\/g')
        # the above seds:
        #   1) Convert '|' to newline, but only if bracketed by spaces or
        #      at beginning/end of line (this allows 'foo|bar' in tests);
        #   2) then remove leading whitespace;
        #   3) then double-escape all backslashes

        printf "%q " "${row[@]}"
        printf "\n"
    done <<<"$1"
}


###################
#  random_string  #  Returns a pseudorandom human-readable string
###################
#
# Numeric argument, if present, is desired length of string
#
function random_string() {
    local length=${1:-10}

    head /dev/urandom | tr -dc a-zA-Z0-9 | head -c$length
}

##############
#  safename  #  Returns a pseudorandom string suitable for container/image/etc names
##############
#
# Name will include the bats test number and a pseudorandom element,
# eg "t123-xyz123". safename() will return the same string across
# multiple invocations within a given test; this makes it easier for
# a maintainer to see common name patterns.
#
# String is lower-case so it can be used as an image name
#
function safename() {
    # FIXME: I don't think these can ever fail. Remove checks once I'm sure.
    test -n "$BATS_SUITE_TMPDIR"
    test -n "$BATS_SUITE_TEST_NUMBER"
    safenamepath=$BATS_SUITE_TMPDIR/.safename.$BATS_SUITE_TEST_NUMBER
    if [[ ! -e $safenamepath ]]; then
        echo -n "t${BATS_SUITE_TEST_NUMBER}-$(random_string 8 | tr A-Z a-z)" >$safenamepath
    fi
    cat $safenamepath
}

#########################
#  find_exec_pid_files  #  Returns nothing or exec_pid hash files
#########################
#
# Return exec_pid hash files if exists, otherwise, return nothing
#
function find_exec_pid_files() {
    _run_podman_quiet info --format '{{.Store.GraphRoot}}'
    local storage_path="$output"

    if [ ! -d "$storage_path" ]; then
        echo "error: storage path does not exist"
    fi

    find "$storage_path" -type f -iname 'exec_pid'
}


#############################
#  remove_same_dev_warning  #  Filter out useless warning from output
#############################
#
# On some CI systems, 'podman run --privileged' emits a useless warning:
#
#    WARNING: The same type, major and minor should not be used for multiple devices.
#
# This obviously screws us up when we look at output results.
#
# This function removes the warning from $output and $lines. We don't
# do a full string match because there's another variant of that message:
#
#    WARNING: Creating device "/dev/null" with same type, major and minor as existing "/dev/foodevdir/null".
#
# (We should never again see that precise error ever again, but we could
# see variants of it).
#
function remove_same_dev_warning() {
    # No input arguments. We operate in-place on $output and $lines

    local i=0
    local -a new_lines=()
    while [[ $i -lt ${#lines[@]} ]]; do
        if expr "${lines[$i]}" : 'WARNING: .* same type, major' >/dev/null; then
            :
        else
            new_lines+=("${lines[$i]}")
        fi
        i=$(( i + 1 ))
    done

    lines=("${new_lines[@]}")
    output=$(printf '%s\n' "${lines[@]}")
}

# run 'podman help', parse the output looking for 'Available Commands';
# return that list.
function _podman_commands() {
    dprint "$@"
    # &>/dev/null prevents duplicate output
    run_podman help "$@" &>/dev/null
    awk '/^Available Commands:/{ok=1;next}/^Options:/{ok=0}ok { print $1 }' <<<"$output" | grep .
}

##########################
#  sleep_to_next_second  #  Sleep until second rolls over
##########################

function sleep_to_next_second() {
    sleep 0.$(printf '%04d' $((10000 - 10#$(date +%4N))))
}

function wait_for_command_output() {
    local cmd="$1"
    local want="$2"
    local tries=20
    local sleep_delay=0.5

    case "${#*}" in
        2) ;;
        4) tries="$3"
           sleep_delay="$4"
           ;;
        *) die "Internal error: 'wait_for_command_output' requires two or four arguments" ;;
    esac

    while [[ $tries -gt 0 ]]; do
        echo "$_LOG_PROMPT $cmd"
        run $cmd
        echo "$output"
        if [[ "$output" = "$want" ]]; then
            return
        fi

        sleep $sleep_delay
        tries=$((tries - 1))
    done
    die "Timed out waiting for '$cmd' to return '$want'"
}

function make_random_file() {
    dd if=/dev/urandom of="$1" bs=1 count=${2:-$((${RANDOM} % 8192 + 1024))} status=none
}

###########################
# ensure there is no mount point at the specified path
###########################
function ensure_no_mountpoint() {
    local path="$1"
    if findmnt "$path"; then
        die "there is a mountpoint at $path"
    fi
}

###########################
# ensure container has been restarted requested times
###########################
function wait_for_restart_count() {
    local cname="$1"
    local count="$2"
    local tname="$3"

    local timeout=10
    while :; do
        # Previously this would fail as the container would run out of ips after 5 restarts.
        run_podman inspect --format "{{.RestartCount}}" $cname
        if [[ "$output" == "$2" ]]; then
            break
        fi

        timeout=$((timeout - 1))
        if [[ $timeout -eq 0 ]]; then
            die "Timed out waiting for RestartCount with $tname"
        fi
        sleep 0.5
    done
}

function gen-cert-pair {
  cn=$1 key=$2 cert=$3
  shift 3
  openssl req -x509 \
    -quiet \
    -nodes \
    -newkey rsa:4096 -keyout "${key}" \
    -out "${cert}" \
    -days 1 \
    -subj "/C=??/ST=System/L=Test/O=Containers/OU=Podman/CN=${cn}" \
    "$@"
}

function gen-signed-cert-pair {
  cn=$1 key=$2 cert=$3 ca_key=$4 ca_cert=$5
  shift 5
  gen-cert-pair "${cn}" \
    "${key}" "${cert}" \
    -CAkey "${ca_key}" -CA "${ca_cert}" \
    "$@"
}

function gen-tls {
  rm -f \
    "${REMOTESYSTEM_TLS_CA_KEY}" "${REMOTESYSTEM_TLS_CA_CRT}" \
    "${REMOTESYSTEM_TLS_CLIENT_KEY}" "${REMOTESYSTEM_TLS_CLIENT_CRT}" \
    "${REMOTESYSTEM_TLS_SERVER_KEY}" "${REMOTESYSTEM_TLS_SERVER_CRT}" \
    "${REMOTESYSTEM_TLS_BOGUS_KEY}" "${REMOTESYSTEM_TLS_BOGUS_CRT}"

  # CA
  gen-cert-pair "ca" \
    "${REMOTESYSTEM_TLS_CA_KEY}" "${REMOTESYSTEM_TLS_CA_CRT}" \
    -addext basicConstraints=critical,CA:TRUE,pathlen:1
  # Client, signed by CA
  gen-signed-cert-pair "client" \
    "${REMOTESYSTEM_TLS_CLIENT_KEY}" "${REMOTESYSTEM_TLS_CLIENT_CRT}" \
    "${REMOTESYSTEM_TLS_CA_KEY}" "${REMOTESYSTEM_TLS_CA_CRT}" \

  # Server, signed by CA, valid for localhost, 127.0.0.1
  # NOTE: Go refuses certs without SAN's
  gen-signed-cert-pair "localhost" \
    "${REMOTESYSTEM_TLS_SERVER_KEY}" "${REMOTESYSTEM_TLS_SERVER_CRT}" \
    "${REMOTESYSTEM_TLS_CA_KEY}" "${REMOTESYSTEM_TLS_CA_CRT}" \
    -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"

  # Bogus, self-signed
  gen-cert-pair "bogus" \
    "${REMOTESYSTEM_TLS_BOGUS_KEY}" "${REMOTESYSTEM_TLS_BOGUS_CRT}" \
    -addext basicConstraints=critical,CA:TRUE,pathlen:1
}


function systemd-run-user {
  args=()
  if [ "${UID}" != '0' ]; then
    args+=(--user)
  fi
  systemd-run "${args[@]}" "$@"
}

function systemctl-user {
  args=()
  if [ "${UID}" != '0' ]; then
    args+=(--user)
  fi
  systemctl "${args[@]}" "$@"
}

SUITE_SERVICE_NAME="podman-service-$(random_string)"
SUITE_PIDFILE="${BATS_SUITE_TMPDIR}/podman-system-service.pid"
function start-suite-podman-system-service {
    service_args=()
    case "${REMOTESYSTEM_TRANSPORT}" in
    tls|mtls)
      service_args+=(
        --tls-cert="${REMOTESYSTEM_TLS_SERVER_CRT}"
        --tls-key="${REMOTESYSTEM_TLS_SERVER_KEY}"
      )
      ;;
    esac
    case "${REMOTESYSTEM_TRANSPORT}" in
    mtls)
      service_args+=(--tls-client-ca="${REMOTESYSTEM_TLS_CA_CRT}")
      ;;
    esac
    case "${REMOTESYSTEM_TRANSPORT}" in
    tcp|tls|mtls)
      service_args+=("tcp://localhost:${REMOTESYSTEM_TCP_PORT}")
      ;;
    unix)
      rm "${REMOTESYSTEM_UNIX_SOCK}"
      service_args+=("unix://${REMOTESYSTEM_UNIX_SOCK}")
    esac

    # TODO: In the future, use systemd if possible
    # systemd-run-user --unit=$SUITE_SERVICE_NAME ${PODMAN%%-remote*} system service "${service_args[@]}" --time=0
    ${PODMAN%%-remote*} system service "${service_args[@]}" --time=0 &> "${PODMAN_SERVER_LOG:-/dev/null}" &
    echo $! > "${SUITE_PIDFILE}"

    retry=5
    while [ $retry -ge 0 ]; do
      echo Waiting for system service...
      sleep 1
      "${PODMAN_CMD[@]}" system info && break
      retry=$(expr $retry - 1)
    done
    if [ $retry -lt 0 ]; then
      echo "Error: ./bin/podman system service did not come up" >&2
      exit 1
    fi
}


# END   miscellaneous tools
###############################################################################
