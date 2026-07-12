#!/usr/bin/env bats   -*- bats -*-
#
# Tests podman system service under systemd socket activation
#

load helpers
load helpers.registry
load helpers.systemd

function setup_file() {
    # We have to stop the background registry here. These tests kill the podman pause
    # process which means commands after that are in a new one and when the cleanup
    # later tries to stop the registry container it will be in the wrong ns and can fail.
    # https://github.com/containers/podman/pull/21563#issuecomment-1960047648
    stop_registry
}

SERVICE_NAME="podman_test_$(random_string)"

SERVICE_SOCK_ADDR="/run/podman/$SERVICE_NAME.sock"
if is_rootless; then
    SERVICE_SOCK_ADDR="$XDG_RUNTIME_DIR/podman/$SERVICE_NAME.sock"
fi

SERVICE_FILE="$UNIT_DIR/$SERVICE_NAME.service"
SOCKET_FILE="$UNIT_DIR/$SERVICE_NAME.socket"

# URL to use for ping
_PING=http://placeholder-hostname/libpod/_ping

function setup() {
    skip_if_remote "systemd tests are meaningless over remote"

    basic_setup

    cat > $SERVICE_FILE <<EOF
[Unit]
Description=Podman API Service
Requires=$SERVICE_NAME.socket
After=$SERVICE_NAME.socket
Documentation=man:podman-system-service(1)
StartLimitIntervalSec=0

[Service]
Type=exec
KillMode=process
Environment=LOGGING="--log-level=info"
ExecStart=$PODMAN $LOGGING system service -t 2
EOF
    cat > $SOCKET_FILE <<EOF
[Unit]
Description=Podman API Socket
Documentation=man:podman-system-service(1)

[Socket]
ListenStream=%t/podman/$SERVICE_NAME.sock
SocketMode=0660

[Install]
WantedBy=sockets.target
EOF

    # ensure pause process/ns_handles are removed before each test runs
    if is_rootless; then
        local pause_pid_file="$XDG_RUNTIME_DIR/libpod/tmp/pause.pid"
        local ns_handles_file="$XDG_RUNTIME_DIR/libpod/tmp/ns_handles"
        if [ -f $pause_pid_file ]; then
            kill -9 $(< $pause_pid_file) 2> /dev/null
            rm -f $pause_pid_file
        fi
        rm -f $ns_handles_file
    fi
    systemctl_start "$SERVICE_NAME.socket"
}

function teardown() {
    systemctl stop "$SERVICE_NAME.socket"
    rm -f "$SERVICE_FILE" "$SOCKET_FILE"
    systemctl daemon-reload
    basic_teardown
}

@test "podman system service - socket activation - no container" {
    run curl -s --max-time 3 --unix-socket $SERVICE_SOCK_ADDR $_PING
    echo "curl output: $output"
    is "$status" "0" "curl exit status"
    is "$output" "OK" "podman service responds normally"
}

@test "podman system service - socket activation - existing container" {
    run_podman run -d $IMAGE sleep 90
    cid="$output"

    run curl -s --max-time 3 --unix-socket $SERVICE_SOCK_ADDR $_PING
    echo "curl output: $output"
    is "$status" "0" "curl exit status"
    is "$output" "OK" "podman service responds normally"

    run_podman rm -f -t 0 $cid
}

@test "podman system service - socket activation - kill rootless pause" {
    if ! is_rootless; then
        skip "there is no pause process when running rootful"
    fi
    run_podman run -d $IMAGE sleep 90
    cid="$output"

    local pause_pid_file="$XDG_RUNTIME_DIR/libpod/tmp/pause.pid"
    local ns_handles_file="$XDG_RUNTIME_DIR/libpod/tmp/ns_handles"

    if [ -f $ns_handles_file ]; then
        echo "Removing ns_handles file: $ns_handles_file"
        rm -f $ns_handles_file
    elif [ -f $pause_pid_file ]; then
        echo "kill -9 $(< $pause_pid_file) [pause process]"
        kill -9 $(< $pause_pid_file)
    else
        die "Neither ns_handles file nor pause.pid file exists"
    fi

    run curl -s --max-time 3 --unix-socket $SERVICE_SOCK_ADDR $_PING
    echo "curl output: $output"
    is "$status" "0" "curl exit status"
    is "$output" "OK" "podman service responds normally"

    run_podman rm -f -t 0 $cid
}

# vim: filetype=sh
