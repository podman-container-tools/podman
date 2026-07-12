#!/usr/bin/env bash

set -exo pipefail

uname -r

loginctl enable-linger "$ROOTLESS_USER"

rpm -q \
    aardvark-dns \
    buildah \
    conmon \
    container-selinux \
    containers-common \
    criu \
    crun \
    netavark \
    passt \
    podman \
    podman-tests \
    skopeo \
    systemd

export system_service_cmd="/usr/bin/podman system service --timeout=0 &"
export test_cmd="whoami && cd /usr/share/podman/test/system && PODMAN_TESTING=/usr/bin/podman-testing bats ."

if [[ -z $1 ]]; then
    if [[ $PODMAN == "/usr/bin/podman-remote" ]]; then
        eval "$system_service_cmd"
    fi
    eval "$test_cmd"
elif [[ $1 == "rootless" ]]; then
    if [[ $PODMAN == "/usr/bin/podman-remote" ]]; then
        su - "$ROOTLESS_USER" -c "eval $system_service_cmd"
    fi
    su - "$ROOTLESS_USER" -c "eval $test_cmd"
fi

# Kill all podman processes for remote tests
if [[ $PODMAN == "/usr/bin/podman-remote" ]]; then
    killall -q podman
fi
exit 0
