# This must be sourced from other scripts to work.

OS_RELEASE_VER="$(
    source /etc/os-release
    echo $VERSION_ID | tr -d '.'
)"
OS_RELEASE_ID="$(
    source /etc/os-release
    echo $ID
)"
OS_REL_VER="$OS_RELEASE_ID-$OS_RELEASE_VER"

function die() {
    echo "$1" >&2
    exit 1
}

function parse_args() {
    # TEST name, i.e. int or sys
    TEST=
    # i.e. fedora-current
    DISTRO_NAME=
    # local or remote podman
    MODE=local
    # root or rootless
    PRIV=rootless
    case "$#" in
    2)
        TEST=$1
        DISTRO_NAME=$2
        ;;
    3)
        TEST=$1
        PRIV=$2
        DISTRO_NAME=$3
        ;;
    4)
        TEST=$1
        MODE=$2
        PRIV=$3
        DISTRO_NAME=$4
        ;;
    *)
        die "Invalid number of arguments $#, need 2-4"
        ;;
    esac

    validate_distro "$DISTRO_NAME"
    validate_mode "$MODE"
}

function validate_distro() {
    case "$1" in
    "fedora-current" | "fedora-prior" | "fedora-rawhide" | "debian-sid")
        ;;
    *)
        die "Unknown DISTRO_NAME '$1' set"
        ;;
    esac
}

function validate_mode() {
    case "$1" in
    "local" | "remote")
        ;;
    *)
        # upgrade test uses mode to pass the upgrade version
        if [[ "$TEST" != "upgrade" ]]; then
            die "Unknown MODE '$1' set"
        fi
        ;;
    esac
}

# Remove all files provided by the distro version of podman.
# All VM cache-images used for testing include the distro podman because (1) it's
# required for podman-in-podman testing and (2) it somewhat simplifies the task
# of pulling in necessary prerequisites packages as the set can change over time.
# For general CI testing however, calling this function makes sure the system
# can only run the compiled source version.
function remove_packaged_podman_files() {
    echo "Removing packaged podman files to prevent conflicts with source build and testing."

    # If any binaries are resident they could cause unexpected pollution
    for unit in podman.socket podman-auto-update.timer; do
        for state in enabled active; do
            if sudo systemctl --quiet is-$state $unit; then
                echo "Warning: $unit found $state prior to packaged-file removal"
                sudo systemctl --quiet disable $unit || true
                sudo systemctl --quiet stop $unit || true
            fi
        done
    done

    # OS_RELEASE_ID is defined by automation-library
    # shellcheck disable=SC2154
    if [[ "$OS_RELEASE_ID" =~ "debian" ]]; then
        LISTING_CMD="dpkg-query -L podman"
    else
        LISTING_CMD="rpm -ql podman"
    fi

    # delete the podman socket in case it has been created previously.
    # Do so without running podman, lest that invocation initialize unwanted state.
    sudo rm -f /run/podman/podman.sock /run/user/$(id -u)/podman/podman.sock || true

    # yum/dnf/dpkg may list system directories, only remove files
    $LISTING_CMD | while read fullpath; do
        # Sub-directories may contain unrelated/valuable stuff
        if [[ -d "$fullpath" ]]; then continue; fi
        sudo rm -f "$fullpath"
    done
}
