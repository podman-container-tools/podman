# -*- bash -*-
#
# BATS helpers for systemd-related functionality
#

# For tests which write systemd unit files
UNIT_DIR="/run/systemd/system"
_DASHUSER=
if is_rootless; then
    UNIT_DIR="${XDG_RUNTIME_DIR}/systemd/user"
    # Why isn't systemd smart enough to figure this out on its own?
    _DASHUSER="--user"
fi

mkdir -p $UNIT_DIR

journalctl_raw() {
    # Run the journalctl command directly without any wrapper
    timeout --foreground -v --kill=10 $PODMAN_TIMEOUT journalctl "$@"
}

JOURNALCTL_SUPPORT_USER=false
run journalctl_raw --user -b -n 1
if [[ $output != *"No entries"* ]]; then
    JOURNALCTL_SUPPORT_USER=true
fi

systemctl() {
    timeout --foreground -v --kill=10 $PODMAN_TIMEOUT systemctl $_DASHUSER "$@"
}

journalctl() {
    if is_rootless && ! $JOURNALCTL_SUPPORT_USER; then
        # For systems that --user not working, need to update the options manually
        # for our testing. Otherwise journalctl with --user will always report
        # -- No entries --
        options=()
        for arg in "$@"; do
            opt=("$arg")
            if [[ "$arg" =~ "--unit" ]]; then
                opt=("${arg/--unit/--user-unit}")
            elif [[ "$arg" == "-u" ]]; then
                opt=("--user-unit")
            elif [[ "$arg" =~ ^-u || "$arg" =~ ^-[a-zA-Z]+u ]]; then
                opt=("${arg/u/}" "--user-unit")
            fi
            options+=("${opt[@]}")
        done
        journalctl_raw "${options[@]}"
    else
        journalctl_raw $_DASHUSER "$@"
    fi
}

systemd-run() {
    timeout --foreground -v --kill=10 $PODMAN_TIMEOUT systemd-run $_DASHUSER "$@";
}

# "systemctl start" is special: when it fails, it doesn't give any useful info.
# This helper fixes that.
systemctl_start() {
    # Arg processing. First arg might be "--wait"...
    local wait=
    if [[ "$1" = "--wait" ]]; then
        wait="$1"
        shift
    fi
    # ...but beyond that, only one arg is allowed
    local unit="$1"
    shift
    assert "$*" = "" "systemctl_start invoked with spurious args"

    echo "$_LOG_PROMPT systemctl $wait start $unit"
    run systemctl $wait start "$unit"
    echo "$output"
    if [[ $status -eq 0 ]]; then
        return
    fi

    # Failed. This is our value added.
    echo
    echo "***** systemctl start $unit -- FAILED!"
    echo
    echo "$_LOG_PROMPT systemctl status $unit"
    run systemctl status "$unit"
    echo "$output"
    echo
    echo "$_LOG_PROMPT journalctl -xeu $unit"
    run journalctl -xeu "$unit"
    echo "$output"
    false
}

install_kube_template() {
    # If running from a podman source directory, build and use the source
    # version of the play-kube-@ unit file
    unit_name="podman-kube@.service"
    unit_file_in="contrib/systemd/system/${unit_name}.in"
    if [[ -e $unit_file_in ]]; then
        unit_file_out=$UNIT_DIR/$unit_name
        sed -e "s;@@PODMAN@@;$PODMAN;g" <$unit_file_in >$unit_file_out.tmp.$$ \
            && mv $unit_file_out.tmp.$$ $unit_file_out
    elif [[ "$PODMAN" = "/usr/bin/podman" ]]; then
        # Not running from a source directory. This is expected in gating,
        # and is probably OK, but it could fail on a misinstalled setup.
        # Maintainer will only see this warning in case of test failure.
        echo "WARNING: Test will rely on system-installed unit files." >&2
    else
        skip "No $unit_file_in, and PODMAN=$PODMAN"
    fi
}

quadlet_to_service_name() {
    local filename=$(basename -- "$1")
    local extension="${filename##*.}"
    local filename="${filename%.*}"
    local suffix=""
    local is_template=""

    # Check if this is a template unit (ends with @)
    if [[ "$filename" == *@ ]]; then
        is_template="@"
        filename="${filename%@}"
    fi

    if [ "$extension" == "volume" ]; then
        suffix="-volume"
    elif [ "$extension" == "network" ]; then
        suffix="-network"
    elif [ "$extension" == "image" ]; then
        suffix="-image"
    elif [ "$extension" == "pod" ]; then
        suffix="-pod"
    elif [ "$extension" == "build" ]; then
        suffix="-build"
    elif [ "$extension" == "artifact" ]; then
        suffix="-artifact"
    fi

    echo "$filename$suffix$is_template.service"
}
