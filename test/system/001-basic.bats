#!/usr/bin/env bats
#
# Simplest set of podman tests. If any of these fail, we have serious problems.
#

load helpers
load helpers.network

# Override standard setup! We don't yet trust podman-images or podman-rm
function setup() {
    # Makes test logs easier to read
    BATS_TEST_NAME_PREFIX="[001] "
}

#### DO NOT ADD ANY TESTS HERE! ADD NEW TESTS AT BOTTOM!

@test "podman version emits reasonable output" {
    run_podman version

    # First line of podman version is "Client: *Podman Engine".
    # Just delete it (i.e. remove the first entry from the 'lines' array)
    if expr "${lines[0]}" : "Client: *" >/dev/null; then
        lines=("${lines[@]:1}")
    fi

    is "${lines[0]}" "Version:[ ]\+[1-9][0-9.]\+" "Version line 1"
    is "$output" ".*Go Version: \+"               "'Go Version' in output"
    is "$output" ".*API Version: \+"		  "API version in output"

    # Test that build date is reasonable, e.g. after 2019-01-01
    local built=$(expr "$output" : ".*Built: \+\(.*\)" | head -n1)
    local built_t=$(date --date="$built" +%s)
    assert "$built_t" -gt 1546300800 "Preposterous 'Built' time in podman version"

    run_podman -v
    is "$output" "podman.*version \+"               "'Version line' in output"
}

@test "podman info" {
    # These will be displayed on the test output stream, offering an
    # at-a-glance overview of important system configuration details
    local -a want=(
        'Arch:{{.Host.Arch}}'
        'OS:{{.Host.Distribution.Distribution}}{{.Host.Distribution.Version}}'
        'Runtime:{{.Host.OCIRuntime.Name}}'
        'Rootless:{{.Host.Security.Rootless}}'
        'Events:{{.Host.EventLogger}}'
        'Logdriver:{{.Host.LogDriver}}'
        'Cgroups:{{.Host.CgroupsVersion}}+{{.Host.CgroupManager}}'
        'Net:{{.Host.NetworkBackend}}'
        'DB:{{.Host.DatabaseBackend}}'
        'Store:{{.Store.GraphDriverName}}'
    )
    run_podman info --format "$(IFS='/' echo ${want[@]})"
    echo "# $output" >&3
}


@test "podman --context emits reasonable output" {
    if ! is_remote; then
        skip "only applicable on podman-remote"
    fi

    # All we care about here is that the command passes
    run_podman --context=default version

    (
      unset REMOTESYSTEM_TRANSPORT
      # This one must fail
      PODMAN=${PODMAN%%--url*} run_podman 125 --context=swarm version
      is "$output" \
         "Error: read cli flags: connection \"swarm\" not found" \
         "--context=swarm should fail"
    )
}

@test "podman can pull an image" {
    run_podman rmi -a -f

    # This is a risk point: it will fail if the registry or network are flaky
    run_podman pull $IMAGE

    # Regression test for https://github.com/containers/image/pull/1615
    # Make sure no progress lines are duplicated
    local -A line_seen
    for line in "${lines[@]}"; do
        if [[ -n "${line_seen[$line]}" ]]; then
            die "duplicate podman-pull output line: $line"
        fi
        line_seen[$line]=1
    done

    # Also make sure that the tag@digest syntax is supported.
    run_podman inspect --format "{{ .Digest }}" $IMAGE
    digest=$output
    run_podman pull $IMAGE@$digest

    # Now untag the digest reference again.
    run_podman untag $IMAGE $IMAGE@$digest

    # Make sure the original image is still present (#11557).
    run_podman image exists $IMAGE
}

# PR #7212: allow --remote anywhere before subcommand, not just as 1st flag
@test "podman-remote : really is remote, works as --remote option" {
    if ! is_remote; then
        skip "only applicable on podman-remote"
    fi

    # First things first: make sure our podman-remote actually is remote!
    run_podman version
    is "$output" ".*Server:" "the given podman path really contacts a server"

    # $PODMAN may be a space-separated string, e.g. if we include a --url.
    # Split it into its components; remove "-remote" from the command path;
    # and preserve any other args if present.
    local -a podman_as_array=($PODMAN)
    local    podman_path=${podman_as_array[0]}
    local    podman_non_remote=${podman_path%%-remote}
    local -a podman_args=("${podman_as_array[@]:1}")

    # This always worked: running "podman --remote ..."
    PODMAN="${podman_non_remote} --remote ${podman_args[@]}" run_podman version
    is "$output" ".*Server:" "podman --remote: contacts server"

    # This was failing: "podman --foo --bar --remote".
    PODMAN="${podman_non_remote} --log-level=error ${podman_args[@]} --remote" run_podman version
    is "$output" ".*Server:" "podman [flags] --remote: contacts server"

    # ...but no matter what, --remote is never allowed after subcommand
    PODMAN="${podman_non_remote} ${podman_args[@]}" run_podman 125 version --remote
    is "$output" "Error: unknown flag: --remote
See 'podman version --help'" "podman version --remote"
}

@test "podman-remote: defaults" {
    skip_if_remote "only applicable on a local run"

    # By default, podman should include '--remote' in its help output
    run_podman --help
    assert "$output" =~ " --remote " "podman --help includes the --remote option"

    # When it detects CONTAINER_HOST or _CONNECTION, --remote is not an option
    CONTAINER_HOST=foobar run_podman --help
    assert "$output" !~ " --remote " \
           "podman --help, with CONTAINER_HOST set, should not show --remote"

    CONTAINER_CONNECTION=foobar run_podman --help
    assert "$output" !~ " --remote " \
           "podman --help, with CONTAINER_CONNECTION set, should not show --remote"

    # When it detects --url or --connection, --remote is not an option
    run_podman --url foobar --help
    assert "$output" !~ " --remote " \
           "podman --help, with --url set, should not show --remote"

    run_podman --connection foobar --help
    assert "$output" !~ " --remote " \
           "podman --help, with --connection set, should not show --remote"
}

# Check that just calling "podman-remote" prints the usage message even
# without a running endpoint. Use "podman --remote" for this as this works the same.
@test "podman-remote: check for command usage message without a running endpoint" {
    if is_remote; then
        skip "only applicable on a local run since this requires no endpoint"
    fi

    run_podman 125 --remote
    is "$output" ".*Usage:" "podman --remote show usage message without running endpoint"
}

# This is for development only; it's intended to make sure our timeout
# in run_podman continues to work. This test should never run in production
# because it will, by definition, fail.
@test "timeout" {
    if [ -z "$PODMAN_RUN_TIMEOUT_TEST" ]; then
        skip "define \$PODMAN_RUN_TIMEOUT_TEST to enable this test"
    fi
    PODMAN_TIMEOUT=10 run_podman run $IMAGE sleep 90
    echo "*** SHOULD NEVER GET HERE"
}


# Too many tests rely on jq for parsing JSON.
#
# If absolutely necessary, one could establish a convention such as
# defining PODMAN_TEST_SKIP_JQ=1 and adding a skip_if_no_jq() helper.
# For now, let's assume this is not absolutely necessary.
@test "jq is installed and produces reasonable output" {
    type -path jq >/dev/null || die "FATAL: 'jq' tool not found."

    run jq -r .a.b < <(echo '{ "a": { "b" : "you found me" } }')
    is "$output" "you found me" "sample invocation of 'jq'"
}

@test "podman --log-level recognizes log levels" {
    run_podman 1 --log-level=telepathic version
    is "$output" 'Log Level "telepathic" is not supported.*'

    run_podman --log-level=trace   version
    if ! is_remote; then
        # podman-remote does not do any trace logging
        assert "$output" =~ " level=trace " "log-level=trace"
    fi
    assert "$output" =~ " level=debug " "log-level=trace includes debug"
    assert "$output" =~ " level=info "  "log-level=trace includes info"
    assert "$output" !~ " level=warn"   "log-level=trace does not show warn"

    run_podman --log-level=debug   version
    assert "$output" !~ " level=trace " "log-level=debug does not show trace"
    assert "$output" =~ " level=debug " "log-level=debug"
    assert "$output" =~ " level=info "  "log-level=debug includes info"
    assert "$output" !~ " level=warn"   "log-level=debug does not show warn"

    run_podman --log-level=info    version
    assert "$output" !~ " level=trace " "log-level=info does not show trace"
    assert "$output" !~ " level=debug " "log-level=info does not show debug"
    assert "$output" =~ " level=info "  "log-level=info"

    run_podman --log-level=warn    version
    assert "$output" !~ " level=" "log-level=warn shows no logs at all"

    run_podman --log-level=warning version
    assert "$output" !~ " level=" "log-level=warning shows no logs at all"

    run_podman --log-level=error   version
    assert "$output" !~ " level=" "log-level=error shows no logs at all"

    # docker compat
    run_podman --debug   version
    assert "$output" =~ " level=debug " "podman --debug gives debug output"
    run_podman -D        version
    assert "$output" =~ " level=debug " "podman -D gives debug output"

    run_podman 1 --debug --log-level=panic version
    is "$output" "Setting --log-level and --debug is not allowed"
}

# Tests --noout for commands that do not enter the engine
@test "podman --noout properly suppresses output" {
run_podman --noout system connection ls
    is "$output" "" "output should be empty"
}

# Tests --noout to ensure that the output fd can be written to.
@test "podman --noout is actually writing to /dev/null" {
    skip_if_remote "unshare only works locally"
    skip_if_not_rootless "unshare requires rootless"
    run_podman --noout unshare ls
    is "$output" "" "output should be empty"
}

@test "podman version --out writes matching version to a json" {
    run_podman version

    # copypasta from version check. we're doing this to extract the version.
    if expr "${lines[0]}" : "Client: *" >/dev/null; then
        lines=("${lines[@]:1}")
    fi

    # get the version number so that we have something to compare with.
    IFS=: read version_key version_number <<<"${lines[0]}"
    is "$version_key" "Version" "Version line"

    # now we can output everything as some json. we can't use PODMAN_TMPDIR since basic_setup
    # isn't being used in setup() due to being unable to trust podman-images or podman-rm.
    outfile=$(mktemp -p ${BATS_TEST_TMPDIR} veroutXXXXXXXX)
    run_podman --out $outfile version -f json

    # extract the version from the file.
    run jq -r --arg field "$version_key" '.Client | .[$field]' $outfile
    is "$output" ${version_number} "Version matches"
}

@test "podman - shutdown engines" {
    run_podman --log-level=debug run --rm $IMAGE true
    is "$output" ".*Shutting down engines.*"
    run_podman 125 --log-level=debug run dockah://rien.de/rien:latest
    is "$output" ".*Shutting down engines.*"
}

@test "release" {
  [[ "${RELEASE_TESTING:-false}" == "true" ]] || \
    skip "Release testing may be enabled by setting \$RELEASE_TESTING = 'true'."

  run_podman --version

  assert "$output" "!~" "dev" "The Podman version string does not mention 'dev'."
}

# vim: filetype=sh
