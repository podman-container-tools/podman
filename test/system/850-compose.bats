#!/usr/bin/env bats   -*- bats -*-
#
# Smoke tests for the podman-compose command.  test/compose takes care of functional tests.
#

load helpers

@test "podman compose - smoke tests" {
    fake_compose_bin="$PODMAN_TMPDIR/fake_compose"
    cat >$fake_compose_bin <<EOF
#!/bin/bash
if [[ "\$@" == "fail" ]]; then
    exit 42
fi
if [[ "\$@" == "env" ]]; then
    printenv DOCKER_HOST DOCKER_BUILDKIT DOCKER_CONFIG
    exit 0
fi
echo "arguments: \$@"
EOF

    compose_conf="$PODMAN_TMPDIR/compose.conf"
    cat >$compose_conf <<EOF
[engine]
compose_providers = ["$fake_compose_bin"]
compose_warning_logs = false
EOF

    random_data="--foo=bar --random=$(random_string 15) -f /path/to/file ignore me"

    # Make sure that the fake compose binary is used and that error reporting works
    PODMAN_COMPOSE_PROVIDER=$fake_compose_bin run_podman 125 compose --help
    is "$output" ".*executing $fake_compose_bin --help: .*: permission denied"

    # Make the fake one executable and check the --help output
    chmod +x $fake_compose_bin
    PODMAN_COMPOSE_PROVIDER=$fake_compose_bin run_podman compose --help
    is "$output" "Run compose workloads via an external provider .*arguments: --help"

    # No argument yields the help message as well
    PODMAN_COMPOSE_PROVIDER=$fake_compose_bin run_podman compose
    is "$output" "Run compose workloads via an external provider .*arguments: "

    # Make sure that the provider can be specified via containers.conf and that
    # the warning logs can be turned off
    CONTAINERS_CONF_OVERRIDE=$compose_conf run_podman compose --help
    is "$output" "Run compose workloads via an external provider .*arguments: --help"
    assert "$output" !~ ".*Executing external compose provider.*"

    # Run with bogus arguments and make sure they're being returned
    CONTAINERS_CONF_OVERRIDE=$compose_conf run_podman compose $random_data
    is "$output" "arguments: $random_data"

    # Make sure Podman returns the exit code of the compose provider
    CONTAINERS_CONF_OVERRIDE=$compose_conf run_podman 42 compose fail

    # Make sure the three env variables are set (and parsed)
    op='=~'
    url=".*/podman.sock"
    # if we run remote with --url check the url arg is honored
    if [[ "$PODMAN" =~ "--url" ]]; then
        # get the url from the podman string
        url="${PODMAN##*--url }"
        url="${url%% *}"
        op='='
    elif is_remote && [[ "${REMOTESYSTEM_TRANSPORT}" =~ tcp|tls|mtls ]]; then
        url="tcp://localhost:${REMOTESYSTEM_TCP_PORT}"
        op='='
    elif is_remote && [[ "${REMOTESYSTEM_TRANSPORT}" =~ unix ]]; then
        url="unix://${REMOTESYSTEM_UNIX_SOCK}"
        op='='
    fi
    # podman-remote test might run with --url so unset this because the socket will be used otherwise
    CONTAINERS_CONF_OVERRIDE=$compose_conf run_podman compose env
    assert "${lines[0]}" $op "$url" "line 1 of 3 (DOCKER_HOST)"
    assert "${lines[1]}" = "0"      "line 2 of 3 (DOCKER_BUILDKIT)"
    assert "${lines[2]}" = ""       "line 3 of 3 (DOCKER_CONFIG)"

    DOCKER_HOST="$random_data" DOCKER_CONFIG="$random_data" CONTAINERS_CONF_OVERRIDE=$compose_conf run_podman compose env
    is "${lines[0]}" "$random_data"
    is "${lines[1]}" "0"
    is "${lines[2]}" "$random_data"
}
