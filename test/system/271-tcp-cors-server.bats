#!/usr/bin/env bats   -*- bats -*-
#
# Tests podman system service CORS enabled
#

load helpers
load helpers.network

SERVICE_NAME="podman_test_$(random_string)"

SERVICE_TCP_HOST="127.0.0.1"

SERVICE_FILE="$UNIT_DIR/$SERVICE_NAME.service"
SOCKET_FILE="$UNIT_DIR/$SERVICE_NAME.socket"

# bats test_tags=ci:parallel
@test "podman system service - tcp CORS" {
    skip_if_remote "system service tests are meaningless over remote"
    PORT=$(random_free_port)

    log=${PODMAN_TMPDIR}/system-service.log
    "${PODMAN_CMD[@]}" system service --cors="*" tcp:$SERVICE_TCP_HOST:$PORT -t 20 2> $log &
    podman_pid="$!"

    wait_for_port $SERVICE_TCP_HOST $PORT
    cmd="curl -s --max-time 10 -vvv $SERVICE_TCP_HOST:$PORT/_ping"
    echo "$_LOG_PROMPT $cmd"
    run -0 $cmd
    echo "$output"
    assert "$output" =~ " Access-Control-Allow-Origin: \*" \
           "access-control-allow-origin verifies CORS is set"

    kill $podman_pid
    wait $podman_pid || true

    # Running server over TCP is a bad idea. We should see a warning
    assert "$(< $log)" =~ "Using the Podman API service with TCP sockets" \
           "podman warns about server on TCP"
}

# bats test_tags=ci:parallel
@test "podman system service - tcp without CORS" {
    skip_if_remote "system service tests are meaningless over remote"
    PORT=$(random_free_port)
    "${PODMAN_CMD[@]}" system service tcp:$SERVICE_TCP_HOST:$PORT -t 20 &
    podman_pid="$!"

    wait_for_port $SERVICE_TCP_HOST $PORT
    cmd="curl -s --max-time 10 -vvv $SERVICE_TCP_HOST:$PORT/_ping"
    echo "$_LOG_PROMPT $cmd"
    run -0 $cmd
    echo "$output"

    assert "$output" !~ "Access-Control-Allow-Origin:" \
           "CORS header should not be present"

    kill $podman_pid
    wait $podman_pid || true
}

# bats test_tags=ci:parallel
@test "podman system service - CORS enabled in logs" {
    skip_if_remote "system service tests are meaningless over remote"

    PORT=$(random_free_port)
    run_podman 0+w system service --log-level="debug" --cors="*" -t 1 tcp:$SERVICE_TCP_HOST:$PORT
    is "$output" ".*CORS Headers were set to ..\*...*" "debug log confirms CORS headers set"
    assert "$output" =~ "level=warning msg=\"Using the Podman API service with TCP sockets without TLS is not recommended" \
           "TCP socket warning"
}

# vim: filetype=sh
