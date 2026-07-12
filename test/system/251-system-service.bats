#!/usr/bin/env bats   -*- bats -*-
#
# Tests that require 'podman system service' but no other systemd aspects

load helpers
load helpers.systemd
load helpers.network

SERVICE_NAME="podman-service-$(random_string)"

function teardown() {
    # Ignore exit status: this is just a backup stop in case tests failed
    run systemctl stop "$SERVICE_NAME"

    basic_teardown
}

function _podman_system_service {
  systemd-run --unit=$SERVICE_NAME ${PODMAN%%-remote} system service "$@"
}

@test "podman system service <bad_scheme_uri> returns error" {
    unset REMOTESYSTEM_TRANSPORT

    skip_if_remote "podman system service unavailable over remote"
    run_podman 125 system service localhost:9292
    is "$output" "Error: API Service endpoint scheme \"localhost\" is not supported. Try tcp://localhost:9292 or unix://localhost:9292"

    run_podman 125 system service myunix.sock
    is "$output" "Error: API Service endpoint scheme \"\" is not supported. Try tcp://myunix.sock or unix://myunix.sock"
}

@test "podman system service unix: without two slashes still works" {
    unset REMOTESYSTEM_TRANSPORT

    skip_if_remote "podman system service unavailable over remote"
    URL=unix:$PODMAN_TMPDIR/myunix.sock

    _podman_system_service $URL --time=0
    wait_for_file $PODMAN_TMPDIR/myunix.sock

    run_podman --host $URL info --format '{{.Host.RemoteSocket.Path}}'
    is "$output" "$URL" "RemoteSocket.Path using unix:"

    systemctl stop $SERVICE_NAME
    rm -f $PODMAN_TMPDIR/myunix.sock
}

@test "podman system service: grpc listener" {
    unset REMOTESYSTEM_TRANSPORT

    skip_if_remote "podman system service unavailable over remote"
    URL=unix://$PODMAN_TMPDIR/myunix.sock

    _podman_system_service $URL --time=0
    wait_for_file $PODMAN_TMPDIR/myunix.sock

    nonce=$RANDOM
    run_podman_testing --url "$URL" noop '{"ignored":"'$nonce'"}'
    assert $status -eq 0
    is $(jq -c <<< "$output") '{"ignored":"'$nonce'"}' "noop responder"

    systemctl stop $SERVICE_NAME
    rm -f $PODMAN_TMPDIR/myunix.sock
}

@test "podman-system-service containers survive service stop" {
    unset REMOTESYSTEM_TRANSPORT

    skip_if_remote "podman system service unavailable over remote"
    local runtime=$(podman_runtime)
    if [[ "$runtime" != "crun" ]]; then
        skip "survival code only implemented in crun; you're using $runtime"
    fi

    port=$(random_free_port)
    URL=tcp://127.0.0.1:$port

    _podman_system_service $URL --time=0
    wait_for_port 127.0.0.1 $port

    # Start a long-running container.
    cname=keeps-running
    run_podman --url $URL run -d --name $cname $IMAGE top -d 2

    run_podman container inspect -l --format "{{.State.Running}}"
    is "$output" "true" "This should never fail"

    systemctl stop $SERVICE_NAME

    run_podman container inspect $cname --format "{{.State.Running}}"
    is "$output" "true" "Container is still running after podman server stops"

    run_podman rm -f -t 0 $cname
}

# This doesn't actually test podman system service, but we require it,
# so least-awful choice is to run from this test file.
@test "podman --host / -H options" {
    unset REMOTESYSTEM_TRANSPORT

    port=$(random_free_port)
    URL=tcp://127.0.0.1:$port

    # %%-remote makes this run real podman even when testing podman-remote
    _podman_system_service $URL --time=0
    wait_for_port 127.0.0.1 $port

    for opt in --host -H; do
      run_podman $opt $URL info --format '{{.Host.RemoteSocket.Path}}'
      is "$output" "$URL" "RemoteSocket.Path using $opt"
    done

    systemctl stop $SERVICE_NAME
}

# Regression test for https://github.com/containers/podman/issues/17749
@test "podman-system-service --log-level=trace should be able to hijack" {
    unset REMOTESYSTEM_TRANSPORT

    skip_if_remote "podman system service unavailable over remote"

    port=$(random_free_port)
    URL=tcp://127.0.0.1:$port

    _podman_system_service $URL --time=0
    wait_for_port 127.0.0.1 $port

    out=o-$(random_string)
    cname=c-$(random_string)
    run_podman --url $URL run --name $cname $IMAGE echo $out
    assert "$output" == "$out" "service is able to hijack and stream output back"

    run_podman --url $URL rm $cname
    systemctl stop $SERVICE_NAME
}

@test "podman-system-service --tls-cert without --tls-key fails to start" {
    unset REMOTESYSTEM_TRANSPORT

  skip_if_remote "podman system service unavailable over remote"

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  run_podman 125 system service "tcp://localhost:${port}" \
    --tls-cert="${REMOTESYSTEM_TLS_SERVER_CRT}"
  is "$output" ".* --tls-cert provided without --tls-key"
}

@test "podman-system-service --tls-key without --tls-cert fails to start" {
    unset REMOTESYSTEM_TRANSPORT

  skip_if_remote "podman system service unavailable over remote"

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  run_podman 125 system service "tcp://localhost:${port}" \
    --tls-key="${REMOTESYSTEM_TLS_SERVER_KEY}"
  is "$output" ".* --tls-key provided without --tls-cert"
}

@test "podman-system-service --tls-key=missing fails to start" {
    unset REMOTESYSTEM_TRANSPORT

  skip_if_remote "podman system service unavailable over remote"

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  run_podman 125 system service "tcp://localhost:${port}" --tls-key=no-such-file.pem --tls-cert="${REMOTESYSTEM_TLS_SERVER_CRT}"
  is "$output" ".* no-such-file.pem: no such file or directory"
}

@test "podman-system-service --tls-cert=missing fails to start" {
    unset REMOTESYSTEM_TRANSPORT

  skip_if_remote "podman system service unavailable over remote"

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  run_podman 125 system service "tcp://localhost:${port}" --tls-key="${REMOTESYSTEM_TLS_SERVER_KEY}" --tls-cert=no-such-file.pem
  is "$output" ".* no-such-file.pem: no such file or directory"
}

@test "podman-system-service --tls-client-ca=missing fails to start" {
    unset REMOTESYSTEM_TRANSPORT

  skip_if_remote "podman system service unavailable over remote"

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  run_podman 125 system service "tcp://localhost:${port}" \
    --tls-key="${REMOTESYSTEM_TLS_SERVER_KEY}" \
    --tls-cert="${REMOTESYSTEM_TLS_SERVER_CRT}" \
    --tls-client-ca=no-such-file.pem
  is "$output" ".* no-such-file.pem: no such file or directory"
}

@test "podman-system-service --tls-key=malformed fails to start" {
    unset REMOTESYSTEM_TRANSPORT

  skip_if_remote "podman system service unavailable over remote"

  echo 'not a cert' >"${PODMAN_TMPDIR}/not-a-cert.pem"

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  run_podman 125 system service "${URL}" \
    --tls-key="${PODMAN_TMPDIR}/not-a-cert.pem" \
    --tls-cert="${REMOTESYSTEM_TLS_SERVER_CRT}"
  is "$output" ".* failed to find any PEM data in key input"
}

@test "podman-system-service --tls-cert=malformed fails to start" {
    unset REMOTESYSTEM_TRANSPORT

  skip_if_remote "podman system service unavailable over remote"

  echo 'not a cert' >"${PODMAN_TMPDIR}/not-a-cert.pem"

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  run_podman 125 system service "${URL}" \
    --tls-key="${REMOTESYSTEM_TLS_SERVER_KEY}" \
    --tls-cert="${PODMAN_TMPDIR}/not-a-cert.pem"
  is "$output" ".* failed to find any PEM data in certificate input"
}

@test "podman-system-service --tls-client-ca=malformed fails to start" {
    unset REMOTESYSTEM_TRANSPORT

  skip_if_remote "podman system service unavailable over remote"

  echo 'not a cert' >"${PODMAN_TMPDIR}/not-a-cert.pem"

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  run_podman 125 system service "${URL}" \
    --tls-key="${REMOTESYSTEM_TLS_SERVER_KEY}" \
    --tls-cert="${REMOTESYSTEM_TLS_SERVER_CRT}" \
    --tls-client-ca="${PODMAN_TMPDIR}/not-a-cert.pem"
  is "$output" ".* ${PODMAN_TMPDIR}/not-a-cert.pem: non-PEM data found"
}

@test "podman-system-service --tls-key=cert fails to start" {
    unset REMOTESYSTEM_TRANSPORT

  skip_if_remote "podman system service unavailable over remote"

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  run_podman 125 system service "${URL}" \
    --tls-key="${REMOTESYSTEM_TLS_SERVER_CRT}" \
    --tls-cert="${REMOTESYSTEM_TLS_SERVER_CRT}"
  is "$output" ".*found a certificate rather than a key.*"
}

@test "podman-system-service --tls-cert=key fails to start" {
    unset REMOTESYSTEM_TRANSPORT

  skip_if_remote "podman system service unavailable over remote"

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  run_podman 125 system service "${URL}" \
    --tls-key="${REMOTESYSTEM_TLS_SERVER_KEY}" \
    --tls-cert="${REMOTESYSTEM_TLS_SERVER_KEY}"
  is "$output" ".* PEM inputs may have been switched"
}

@test "podman-system-service --tls-client-ca=key fails to start" {
    unset REMOTESYSTEM_TRANSPORT

  skip_if_remote "podman system service unavailable over remote"

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  run_podman 125 system service "${URL}" \
    --tls-key="${REMOTESYSTEM_TLS_SERVER_KEY}" \
    --tls-cert="${REMOTESYSTEM_TLS_SERVER_CRT}" \
    --tls-client-ca="${REMOTESYSTEM_TLS_CA_KEY}"
  is "$output" ".* ${REMOTESYSTEM_TLS_CA_KEY}: non-certificate type \`.*\` PEM data found"
}

@test "podman-system-service --tls-cert --tls-key refuses HTTP client" {
    unset REMOTESYSTEM_TRANSPORT

  skip_if_remote "podman system service unavailable over remote"

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  _podman_system_service $URL --time=0 \
    --tls-key="${REMOTESYSTEM_TLS_SERVER_KEY}" \
    --tls-cert="${REMOTESYSTEM_TLS_SERVER_CRT}" \
    --tls-client-ca="${REMOTESYSTEM_TLS_CA_CRT}"

  wait_for_port 127.0.0.1 $port

  run_podman 125 --url $URL system info
  is "$output" ".* ping response was 400"
  systemctl stop $SERVICE_NAME
}

@test "podman-system-service --tls-cert --tls-key --tls-client-ca refuses client without cert" {
    unset REMOTESYSTEM_TRANSPORT

  skip_if_remote "podman system service unavailable over remote"

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  _podman_system_service $URL --time=0 \
    --tls-key="${REMOTESYSTEM_TLS_SERVER_KEY}" \
    --tls-cert="${REMOTESYSTEM_TLS_SERVER_CRT}" \
    --tls-client-ca="${REMOTESYSTEM_TLS_CA_CRT}"

  wait_for_port 127.0.0.1 $port

  run_podman 125 --url $URL --tls-ca="${REMOTESYSTEM_TLS_CA_CRT}" system info
  is "$output" ".* remote error: tls: certificate required"
  systemctl stop $SERVICE_NAME
}

@test "podman-system-service --tls-cert --tls-key --tls-client-ca refuses client untrusted cert" {
    unset REMOTESYSTEM_TRANSPORT

  skip_if_remote "podman system service unavailable over remote"

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  _podman_system_service $URL --time=0 \
    --tls-key="${REMOTESYSTEM_TLS_SERVER_KEY}" \
    --tls-cert="${REMOTESYSTEM_TLS_SERVER_CRT}" \
    --tls-client-ca="${REMOTESYSTEM_TLS_CA_CRT}"

  wait_for_port 127.0.0.1 $port

  run_podman 125 \
    --url $URL \
    --tls-key="${REMOTESYSTEM_TLS_BOGUS_KEY}" \
    --tls-cert="${REMOTESYSTEM_TLS_BOGUS_CRT}" \
    --tls-ca="${REMOTESYSTEM_TLS_CA_CRT}" \
    system info
  # This is not a copy-paste error from above, the Go HTTPS server provides the same error message for
  # "you didn't provide a cert"
  # and
  # "you didn't provide a cert *that I trust*"
  # This is allegedly to make it "more secure"
  is "$output" ".* remote error: tls: certificate required"
  systemctl stop $SERVICE_NAME
}
