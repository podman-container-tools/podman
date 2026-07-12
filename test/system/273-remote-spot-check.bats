#
# Tests that spot check connectivity for each of the supported remote transports,
# unix, tcp, tls, mtls

load helpers
load helpers.systemd
load helpers.network

SERVICE_NAME="podman-service-$(random_string)"

function setup() {
  basic_setup
}

function teardown() {
  # Ignore exit status: this is just a backup stop in case tests failed
  run systemctl-user stop "$SERVICE_NAME"
  rm -f $PODMAN_TMPDIR/myunix.sock

  basic_teardown
}

@test "unix remote" {
  unset REMOTESYSTEM_TRANSPORT

  URL=unix:$PODMAN_TMPDIR/myunix.sock

  systemd-run-user --unit=$SERVICE_NAME ${PODMAN%%-remote*} system service $URL --time=0
  wait_for_file $PODMAN_TMPDIR/myunix.sock

  # Variable works
  CONTAINER_HOST=$URL \
    run_podman \
    info --format '{{.Host.RemoteSocket.Path}}'
  is "$output" "$URL" "RemoteSocket.Path using unix:"
  # Flag works
  run_podman \
    --url="$URL" \
    info --format '{{.Host.RemoteSocket.Path}}'
  is "$output" "$URL" "RemoteSocket.Path using unix:"
  # Streaming command works
  run_podman \
    --url="$URL" \
    run --rm -i $IMAGE /bin/sh -c 'echo -n foo; sleep 0.1; echo -n bar; sleep 0.1; echo -n baz'
  is "$output" foobarbaz

  systemctl-user stop $SERVICE_NAME
  rm -f $PODMAN_TMPDIR/myunix.sock
}

@test "tcp remote" {
  unset REMOTESYSTEM_TRANSPORT

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  systemd-run-user --unit=$SERVICE_NAME ${PODMAN%%-remote*} system service $URL --time=0
  wait_for_port 127.0.0.1 $port

  # Variable works
  CONTAINER_HOST=$URL \
    run_podman \
    info --format '{{.Host.RemoteSocket.Path}}'
  is "$output" "$URL" "RemoteSocket.Path using unix:"
  # Flag works
  run_podman \
    --url="$URL" \
    info --format '{{.Host.RemoteSocket.Path}}'
  is "$output" "$URL" "RemoteSocket.Path using unix:"
  # Streaming command works
  run_podman \
    --url="$URL" \
    run --rm -i $IMAGE /bin/sh -c 'echo -n foo; sleep 0.1; echo -n bar; sleep 0.1; echo -n baz'
  is "$output" foobarbaz

  systemctl-user stop $SERVICE_NAME
}

@test "tls remote" {
  unset REMOTESYSTEM_TRANSPORT

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  systemd-run-user --unit=$SERVICE_NAME ${PODMAN%%-remote*} system service $URL --time=0 \
    --tls-key="${REMOTESYSTEM_TLS_SERVER_KEY}" \
    --tls-cert="${REMOTESYSTEM_TLS_SERVER_CRT}"
  wait_for_port 127.0.0.1 $port

  # Variable works
  CONTAINER_HOST=$URL \
    run_podman \
    --tls-ca="${REMOTESYSTEM_TLS_CA_CRT}" \
    info --format '{{.Host.RemoteSocket.Path}}'
  is "$output" "$URL" "RemoteSocket.Path using unix:"
  # Flag works
  run_podman \
    --url="$URL" \
    --tls-ca="${REMOTESYSTEM_TLS_CA_CRT}" \
    info --format '{{.Host.RemoteSocket.Path}}'
  is "$output" "$URL" "RemoteSocket.Path using unix:"
  # Streaming command works
  run_podman \
    --url="$URL" \
    --tls-ca="${REMOTESYSTEM_TLS_CA_CRT}" \
    run --rm -i $IMAGE /bin/sh -c 'echo -n foo; sleep 0.1; echo -n bar; sleep 0.1; echo -n baz'
  is "$output" foobarbaz

  systemctl-user stop $SERVICE_NAME
}

@test "mtls remote" {
  unset REMOTESYSTEM_TRANSPORT

  port=$(random_free_port)
  URL=tcp://127.0.0.1:$port

  systemd-run-user --unit=$SERVICE_NAME ${PODMAN%%-remote*} system service $URL --time=0 \
    --tls-client-ca="${REMOTESYSTEM_TLS_CA_CRT}" \
    --tls-key="${REMOTESYSTEM_TLS_SERVER_KEY}" \
    --tls-cert="${REMOTESYSTEM_TLS_SERVER_CRT}"
  wait_for_port 127.0.0.1 $port

  # Variable works
  CONTAINER_HOST=$URL \
    run_podman \
    --tls-key="${REMOTESYSTEM_TLS_CLIENT_KEY}" \
    --tls-cert="${REMOTESYSTEM_TLS_CLIENT_CRT}" \
    --tls-ca="${REMOTESYSTEM_TLS_CA_CRT}" \
    info --format '{{.Host.RemoteSocket.Path}}'
  is "$output" "$URL" "RemoteSocket.Path using unix:"
  # Flag works
  run_podman \
    --url="$URL" \
    --tls-key="${REMOTESYSTEM_TLS_CLIENT_KEY}" \
    --tls-cert="${REMOTESYSTEM_TLS_CLIENT_CRT}" \
    --tls-ca="${REMOTESYSTEM_TLS_CA_CRT}" \
    info --format '{{.Host.RemoteSocket.Path}}'
  is "$output" "$URL" "RemoteSocket.Path using unix:"
  # Streaming command works
  run_podman \
    --url="$URL" \
    --tls-key="${REMOTESYSTEM_TLS_CLIENT_KEY}" \
    --tls-cert="${REMOTESYSTEM_TLS_CLIENT_CRT}" \
    --tls-ca="${REMOTESYSTEM_TLS_CA_CRT}" \
    run --rm -i $IMAGE /bin/sh -c 'echo -n foo; sleep 0.1; echo -n bar; sleep 0.1; echo -n baz'
  is "$output" foobarbaz

  systemctl-user stop $SERVICE_NAME
}
