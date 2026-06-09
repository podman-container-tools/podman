#!/usr/bin/env bash

set -e

# shellcheck source=hack/ci/lib.sh
source $(dirname $0)/lib.sh

# Assume there are other log collection commands to follow - Don't
# let one break another that may be useful, but also keep any
# actual script-problems fatal so they are noticed right away.
showrun() {
    echo '+ '$(printf " %q" "$@")
    set +e
    echo '------------------------------------------------------------'
    "$@"
    local status=$?
    [[ $status -eq 0 ]] ||
        echo "[ rc = $status -- proceeding anyway ]"
    echo '------------------------------------------------------------'
    set -e
}

bad_os_id_ver() {
    die "Unknown OS '$OS_RELEASE_ID'"
}

case $1 in
audit)
    case $OS_RELEASE_ID in
    debian) showrun cat /var/log/kern.log ;;
    fedora) showrun cat /var/log/audit/audit.log ;;
    *) bad_os_id_ver ;;
    esac
    ;;
df) showrun df -lhTx tmpfs ;;
journal) showrun journalctl -b ;;
podman) showrun ./bin/podman system info ;;
packages)
    # These names are common to Fedora and Debian
    PKG_NAMES=(
        aardvark-dns
        buildah
        conmon
        containernetworking-plugins
        criu
        crun
        golang
        netavark
        passt
        podman
        skopeo
    )
    case $OS_RELEASE_ID in
    fedora)
        cat /etc/fedora-release
        PKG_LST_CMD='rpm -q --qf=%{N}-%{V}-%{R}-%{ARCH}\n'
        PKG_NAMES+=(
            container-selinux
            containers-common
            libseccomp
        )
        ;;
    debian)
        cat /etc/issue
        PKG_LST_CMD='dpkg-query --show --showformat=${Package}-${Version}-${Architecture}\n'
        PKG_NAMES+=(
            golang-github-containers-common
            golang-github-containers-image
            libseccomp2
        )
        ;;
    *) bad_os_id_ver ;;
    esac
    echo "Kernel: " $(uname -r)
    echo "Cgroups: " $(stat -f -c %T /sys/fs/cgroup)
    # Any not-present packages will be listed as such
    $PKG_LST_CMD "${PKG_NAMES[@]}" | sort -u
    ;;
ip) showrun sh -c "ip addr && ip route && ip -6 route" ;;
esac
