# -*- bats -*-
#
# tests of podman commands that require an interactive pty
#

load helpers

# bats file_tags=ci:parallel

###############################################################################
# BEGIN setup/teardown

function setup() {
    basic_setup

    # Each test runs with its own PTY, managed by socat.
    PODMAN_TEST_PTY=$PODMAN_TMPDIR/podman_pty
    PODMAN_DUMMY=$PODMAN_TMPDIR/podman_dummy
    PODMAN_SOCAT_PID=

    # Create a pty. Run under 'timeout' because BATS reaps child processes
    # and if we exit before killing socat, bats will hang forever.
    timeout 10 socat \
            PTY,link=$PODMAN_TEST_PTY,raw,echo=0 \
            PTY,link=$PODMAN_DUMMY,raw,echo=0 &
    PODMAN_SOCAT_PID=$!

    # Wait for pty
    retries=5
    while [[ ! -e $PODMAN_TEST_PTY ]]; do
        retries=$(( retries - 1 ))
        assert $retries -gt 0 "Timed out waiting for $PODMAN_TEST_PTY"
        sleep 0.5
    done
}

function teardown() {
    if [[ -n $PODMAN_SOCAT_PID ]]; then
        kill $PODMAN_SOCAT_PID
        PODMAN_SOCAT_PID=
    fi
    rm -f $PODMAN_TEST_PTY $PODMAN_DUMMY_PTY

    basic_teardown
}

# END   setup/teardown
###############################################################################
# BEGIN tests

@test "podman detects correct tty size" {
    # Set the pty to a random size. Make rows/columns odd/even, to guarantee
    # that they can never be the same
    rows=$(( 15 + RANDOM % 60 |   1 ))
    cols=$(( 15 + RANDOM % 60 & 126 ))
    stty rows $rows cols $cols <$PODMAN_TEST_PTY

    CR=$'\r'

    # ...and make sure stty under podman reads that.
    # This flakes often ("stty: standard input"), so, retry.
    run_podman run -it --name mystty $IMAGE stty size <$PODMAN_TEST_PTY
    if [[ "$output" =~ stty ]]; then
        echo "# stty flaked, retrying: $output" >&3
        run_podman rm -f mystty
        sleep 1
        run_podman run -it --name mystty $IMAGE stty size <$PODMAN_TEST_PTY
    fi
    is "$output" "$rows $cols$CR" "stty under podman run reads the correct dimensions"

    run_podman rm -t 0 -f mystty

    # FIXME: the checks below are flaking a lot (see #10710).

    # check that the same works for podman exec
#    run_podman run -d --name mystty $IMAGE top
#    run_podman exec -it mystty stty size <$PODMAN_TEST_PTY
#    is "$output" "$rows $cols" "stty under podman exec reads the correct dimensions"
#
#    run_podman rm -t 0 -f mystty
}


@test "podman load - will not read from tty" {
    run_podman 125 load <$PODMAN_TEST_PTY
    is "$output" \
       "Error: cannot read from terminal, use command-line redirection or the --input flag" \
       "Diagnostic from 'podman load' without redirection or -i"
}


@test "podman run --tty -i failure with no tty" {
    run_podman 0+w run --tty -i --rm $IMAGE echo hello < /dev/null
    require_warning "The input device is not a TTY.*" "-it _without_ a tty"

    CR=$'\r'
    run_podman run --tty -i --rm $IMAGE echo hello <$PODMAN_TEST_PTY
    is "$output" "hello$CR" "-it _with_ a pty"

    run_podman run --tty=false -i --rm $IMAGE echo hello < /dev/null
    is "$output" "hello" "-tty=false: no warning"

    run_podman run --tty -i=false --rm $IMAGE echo hello < /dev/null
    is "$output" "hello$CR" "-i=false: no warning"
}


@test "podman run -l passthrough-tty" {
    skip_if_remote

    # Requires conmon 2.1.10 or greater
    want=2.1.10
    run_podman info --format '{{.Host.Conmon.Path}}'
    conmon_path="$output"
    conmon_version=$($conmon_path --version | sed -ne 's/^.* version //p')
    if ! printf "%s\n%s\n" "$want" "$conmon_version" | sort --check=quiet --version-sort; then
        skip "need conmon >= $want; have $conmon_version"
    fi

    run tty <$PODMAN_TEST_PTY
    expected_tty="$output"

    run_podman run --rm -v/dev:/dev --log-driver=passthrough-tty $IMAGE tty <$PODMAN_TEST_PTY
    is "$output" "$expected_tty" "passthrough-tty: tty matches"
}

@test "podman volume export should fail when stdout is a tty" {
    run_podman volume create testVol
    run_podman run --rm -v testVol:/data $IMAGE sh -c "echo data > /data/file.txt"

    # Positive Case
    "${PODMAN_CMD[@]}" volume export testVol --output=/dev/null >$PODMAN_TEST_PTY ||
        die "$PODMAN volume export testVol --output=/dev/null failed when connected to terminal."

    # Negative Case
    local rc=0
    "${PODMAN_CMD[@]}" volume export testVol >$PODMAN_TEST_PTY 2>$PODMAN_TMPDIR/out || rc=$?

    is "$rc" "125" "Exit code should be 125"
    is "$(<$PODMAN_TMPDIR/out)" "Error: cannot write to terminal, use command-line redirection or the --output flag" "Should refuse to export to terminal."

    run_podman volume rm testVol --force
}

@test "podman inspect returns attached tty size" {

    # Skip on FreeBSD
    if [[ "$(uname -s)" == "FreeBSD" ]]; then
        skip "ConsoleSize not implemented on FreeBSD"
    fi

    rows=$(( 15 + RANDOM % 60 |   1 ))
    cols=$(( 15 + RANDOM % 60 & 126 ))

    cname=c-$(safename)

    stty rows $rows cols $cols <$PODMAN_TEST_PTY

    # Run container with attached tty in background.
    run_podman run -it --name $cname $IMAGE top <$PODMAN_TEST_PTY &

    # Wait for container to appear
    local timeout=10
    while :;do
          sleep 0.5
          run_podman '?' container exists $cname
          if [[ $status -eq 0 ]]; then
              break
          fi
          timeout=$((timeout - 1))
          if [[ $timeout -eq 0 ]]; then
              run_podman ps -a
              die "Timed out waiting for container $cname to start"
          fi
    done

    # Now that container exists, wait for it to be in running state
    _ensure_container_running $cname true

    run_podman inspect $cname --format '{{ index .HostConfig.ConsoleSize 0 }} {{ index .HostConfig.ConsoleSize 1 }}'
    is "$output" "$rows $cols" "podman returns non-zero size when tty is attached"

    run_podman rm -t 0 -f $cname

    # Run container without attached tty in detached mode.
    run_podman run -dt --name $cname $IMAGE top

    run_podman inspect $cname --format '{{ index .HostConfig.ConsoleSize 0 }} {{ index .HostConfig.ConsoleSize 1 }}'
    is "$output" "0 0" "podman returns zero size when tty is not attached"

    run_podman rm -t 0 -f $cname
}

# vim: filetype=sh
