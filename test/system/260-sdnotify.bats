#!/usr/bin/env bats   -*- bats -*-
#
# Tests for systemd sdnotify
#

load helpers
load helpers.network
load helpers.registry

# Shared throughout this module: PID of socat process, and path to its log
_SOCAT_PID=
_SOCAT_LOG=

function setup() {
    skip_if_remote "systemd tests are meaningless over remote"

    # Skip if systemd is not running
    systemctl list-units &>/dev/null || skip "systemd not available"

    basic_setup

    # sdnotify fails with runc 1.0.0-3-dev2 on Ubuntu. Let's just
    # assume that we work only with crun, nothing else.
    runtime=$(podman_runtime)
    if [[ "$runtime" != "crun" ]]; then
        skip "this test only works with crun, not $runtime"
    fi
}

function teardown() {
    unset NOTIFY_SOCKET

    _stop_socat

    basic_teardown
}

###############################################################################
# BEGIN helpers

# Run socat process on a socket, logging to well-known path. Each received
# packet is logged with a newline appended, for ease of parsing the log file.
function _start_socat() {
    _SOCAT_LOG="$PODMAN_TMPDIR/socat.log"

    # Reset socat logfile to empty
    rm -f $_SOCAT_LOG
    touch $_SOCAT_LOG
    # Execute in subshell so we can close fd3 (which BATS uses).
    # This is a superstitious ritual to try to avoid leaving processes behind,
    # and thus prevent CI hangs.
    (exec socat unix-recvfrom:"$NOTIFY_SOCKET",fork \
          system:"(cat;echo) >> $_SOCAT_LOG" 3>&-) &
    _SOCAT_PID=$!

    # Wait for socat to create the socket file. This _should_ be
    # instantaneous, but can take a few seconds under high load
    for try in $(seq 1 10); do
        if [[ -e "$NOTIFY_SOCKET" ]]; then
            return
        fi
        sleep 0.5
    done
    die "Timed out waiting for socat to create $NOTIFY_SOCKET"
}

# Stop the socat background process and clean up logs
function _stop_socat() {
    if [[ -n "$_SOCAT_PID" ]]; then
        # Kill all child processes, then the process itself.
        # This is a superstitious incantation to avoid leaving processes behind.
        # The '|| true' is because only f35 leaves behind socat processes;
        # f33 (and perhaps others?) behave nicely. ARGH!
        pkill -P $_SOCAT_PID || true
        kill $_SOCAT_PID
    fi
    _SOCAT_PID=

    if [[ -n "$_SOCAT_LOG" ]]; then
        rm -f $_SOCAT_LOG
    fi
    _SOCAT_LOG=
}

# Check that MAINPID=xxxxx points to a running conmon process
function _assert_mainpid_is_conmon() {
    local mainpid=$(expr "$1" : ".*MAINPID=\([0-9]\+\)")
    test -n "$mainpid" || die "Could not parse '$1' as 'MAINPID=nnnn'"

    test -d /proc/$mainpid || die "sdnotify MAINPID=$mainpid - but /proc/$mainpid does not exist"

    # e.g. /proc/12345/exe -> /usr/bin/conmon
    local mainpid_bin=$(readlink /proc/$mainpid/exe)
    is "$mainpid_bin" ".*/conmon" "sdnotify MAINPID=$mainpid is conmon process"
}

# END   helpers
###############################################################################
# BEGIN tests themselves

# bats test_tags=ci:parallel
@test "sdnotify : ignore" {
    export NOTIFY_SOCKET=$PODMAN_TMPDIR/ignore.sock
    _start_socat

    run_podman create --rm --sdnotify=ignore $IMAGE printenv NOTIFY_SOCKET
    cid="$output"

    run_podman container inspect $cid --format "{{.Config.SdNotifyMode}} {{.Config.SdNotifySocket}}"
    is "$output" "ignore " "NOTIFY_SOCKET is not set with 'ignore' mode"

    run_podman 1 start --attach $cid
    is "$output" "" "\$NOTIFY_SOCKET in container"

    is "$(< $_SOCAT_LOG)" "" "nothing received on socket"
    _stop_socat
}

# bats test_tags=ci:parallel
@test "sdnotify : conmon" {
    export NOTIFY_SOCKET=$PODMAN_TMPDIR/conmon.sock
    _start_socat

    ctrname=c-$(safename)
    run_podman run -d --name $ctrname \
               --sdnotify=conmon \
               $IMAGE \
               sh -c 'printenv NOTIFY_SOCKET;echo READY;sleep 999'
    cid="$output"
    wait_for_ready $cid

    run_podman container inspect $cid --format "{{.Config.SdNotifyMode}} {{.Config.SdNotifySocket}}"
    is "$output" "conmon $NOTIFY_SOCKET"

    run_podman container inspect $ctrname --format "{{.State.ConmonPid}}"
    mainPID="$output"

    run_podman logs $ctrname
    is "$output" "READY" "\$NOTIFY_SOCKET in container"

    # loop-wait for the final READY line
    wait_for_file_content $_SOCAT_LOG "READY=1"

    # ...and confirm the entire file contents
    logcontents="$(< $_SOCAT_LOG)"
    assert "$logcontents" = "MAINPID=$mainPID
READY=1" "sdnotify sent MAINPID and READY"

    _assert_mainpid_is_conmon "$logcontents"

    # Done. Stop container, clean up.
    run_podman rm -f -t0 $cid
    _stop_socat
}

# These tests can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
# bats test_tags=ci:parallel
@test "sdnotify : container" {
    _prefetch $SYSTEMD_IMAGE

    export NOTIFY_SOCKET=$PODMAN_TMPDIR/container.sock
    _start_socat

    run_podman run -d --sdnotify=container $SYSTEMD_IMAGE \
               sh -c 'trap "touch /stop" SIGUSR1;printenv NOTIFY_SOCKET; echo READY; while ! test -f /stop;do sleep 0.1;done;systemd-notify --ready'
    cid="$output"
    wait_for_ready $cid

    run_podman container inspect $cid --format "{{.Config.SdNotifyMode}} {{.Config.SdNotifySocket}}"
    is "$output" "container $NOTIFY_SOCKET"

    run_podman logs $cid
    is "${lines[0]}" "/run/notify/notify.sock" "NOTIFY_SOCKET is passed to container"

    run_podman container inspect $cid --format "{{.State.ConmonPid}}"
    mainPID="$output"

    # Container does not send READY=1 until our signal. Until then, there must
    # be exactly one line in the log
    wait_for_file_content $_SOCAT_LOG "MAINPID=$mainPID"
    # ...and that line must contain the expected PID, nothing more
    assert "$(< $_SOCAT_LOG)" = "MAINPID=$mainPID" "Container has started, but must not indicate READY yet"

    # Done. Tell container to stop itself, and clean up
    run_podman kill -s USR1 $cid
    run_podman wait $cid

    wait_for_file_content $_SOCAT_LOG "READY=1"
    assert "$(< $_SOCAT_LOG)" = "MAINPID=$mainPID
READY=1" "Container log after ready signal"

    run_podman rm $cid
    _stop_socat
}

# These tests can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
# bats test_tags=ci:parallel
@test "sdnotify : healthy" {
    export NOTIFY_SOCKET=$PODMAN_TMPDIR/container.sock
    _start_socat

    wait_file="$PODMAN_TMPDIR/$(random_string).wait_for_me"
    run_podman 125 create --sdnotify=healthy $IMAGE
    is "$output" "Error: invalid argument: sdnotify policy \"healthy\" requires a healthcheck to be set"

    # Create a container with a simple `/bin/true` healthcheck that we need to
    # run manually.
    ctr=c-$(safename)
    run_podman create --name $ctr     \
            --health-cmd=/bin/true    \
            --health-retries=1        \
            --health-interval=disable \
            --sdnotify=healthy        \
            $IMAGE sleep infinity

    # Start the container in the background which will block until the
    # container turned healthy.  After that, create the wait_file which
    # indicates that start has returned.
    (timeout --foreground -v --kill=5 20 $PODMAN start $ctr && touch $wait_file) &

    run_podman wait --condition=running $ctr

    # Make sure that the MAINPID is set but without the READY message.
    run_podman container inspect $ctr --format "{{.State.ConmonPid}}"
    mainPID="$output"

    # Container does not send READY=1 until it runs a successful health check.
    # Until then, there must be exactly one line in the log
    wait_for_file_content $_SOCAT_LOG "MAINPID="
    # ...and that line must contain the expected PID, nothing more
    assert "$(< $_SOCAT_LOG)" = "MAINPID=$mainPID" "Container logs after start, prior to healthcheck run"

    # Now run the healthcheck and look for the READY message.
    run_podman healthcheck run $ctr
    is "$output" "" "output from 'podman healthcheck run'"

    # Wait for start to return.  At that point the READY message must have been
    # sent.
    wait_for_file_content $_SOCAT_LOG "READY=1"
    assert "$(< $_SOCAT_LOG)" = "MAINPID=$mainPID
READY=1" "Container log after healthcheck run"

    run_podman container inspect  --format "{{.State.Status}}" $ctr
    is "$output" "running" "make sure container is still running"

    run_podman rm -f -t0 $ctr

    # Disable until the race condition https://github.com/containers/podman/issues/22760 is fixed
    # ctr=$(random_string)
    # run_podman run --name $ctr                       \
    #         --health-cmd="touch /terminate"          \
    #         --sdnotify=healthy                       \
    #         $IMAGE sh -c 'while test \! -e /terminate; do sleep 0.1; done; echo finished'
    # is "$output" "finished" "make sure container exited successfully"
    # run_podman rm -f -t0 $ctr

    # ctr=$(random_string)
    # run_podman 12 run --name $ctr --rm               \
    #         --health-cmd="touch /terminate"          \
    #         --sdnotify=healthy                       \
    #         $IMAGE sh -c 'while test \! -e /terminate; do sleep 0.1; done; echo finished; exit 12'
    # is "$output" "finished" "make sure container exited"
    # run_podman rm -f -t0 $ctr

    _stop_socat
}

# bats test_tags=ci:parallel
@test "sdnotify : play kube - no policies" {
    # Create the YAMl file
    yaml_source="$PODMAN_TMPDIR/test.yaml"
    podname=p-$(safename)
    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $podname
spec:
  restartPolicy: "Never"
  containers:
  - command:
    - /bin/sh
    - -c
    - 'while :; do if test -e /rain/tears; then exit 0; fi; sleep 1; done'
    image: $IMAGE
    name: test
    resources: {}
    volumeMounts:
    - mountPath: /rain:z
      name: test-mountdir
  volumes:
  - hostPath:
      path: $PODMAN_TMPDIR
      type: Directory
    name: test-mountdir
EOF

    # The name of the service container is predictable: the first 12 characters
    # of the hash of the YAML file followed by the "-service" suffix
    yaml_sha=$(sha256sum $yaml_source)
    service_container="${yaml_sha:0:12}-service"

    export NOTIFY_SOCKET=$PODMAN_TMPDIR/conmon.sock
    _start_socat
    wait_for_file $_SOCAT_LOG

    run_podman play kube --service-container=true --log-driver journald $yaml_source

    # The service container is the main PID since no container has a custom
    # sdnotify policy.
    run_podman container inspect $service_container --format "{{.State.ConmonPid}}"
    main_pid="$output"

    # Tell pod to finish, then wait for all containers to stop
    touch $PODMAN_TMPDIR/tears
    run_podman container wait $service_container ${podname}-test

    # Make sure the containers have the correct policy.
    run_podman container inspect ${podname}-test $service_container --format "{{.Config.SdNotifyMode}}"
    is "$output" "ignore
ignore"

    wait_for_file_content $_SOCAT_LOG "READY=1"
    assert "$(< $_SOCAT_LOG)" = "MAINPID=$main_pid
READY=1" "sdnotify sent MAINPID and READY"

    _stop_socat

    # Clean up pod and pause image
    run_podman play kube --down $PODMAN_TMPDIR/test.yaml
}

# bats test_tags=ci:parallel
@test "sdnotify : play kube - with policies" {
    skip_if_journald_unavailable

    _prefetch $SYSTEMD_IMAGE

    # Create the YAMl file
    podname=p-$(safename)
    yaml_source="$PODMAN_TMPDIR/test.yaml"
    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $podname
  annotations:
    io.containers.sdnotify:   "container"
    io.containers.sdnotify/b: "conmon"
spec:
  restartPolicy: "Never"
  containers:
  - command:
    - /bin/sh
    - -c
    - 'printenv NOTIFY_SOCKET; echo READY; while ! test -f /stop;do sleep 0.1;done'
    image: $SYSTEMD_IMAGE
    name: a
  - command:
    - /bin/sh
    - -c
    - 'echo READY; top'
    image: $IMAGE
    name: b
EOF
    container_a="${podname}-a"
    container_b="${podname}-b"

    # The name of the service container is predictable: the first 12 characters
    # of the hash of the YAML file followed by the "-service" suffix
    yaml_sha=$(sha256sum $yaml_source)
    service_container="${yaml_sha:0:12}-service"

    export NOTIFY_SOCKET=$PODMAN_TMPDIR/conmon.sock
    _start_socat

    # Run `play kube` in the background as it will wait for all containers to
    # send the READY=1 message.
    timeout --foreground -v --kill=10 60 \
        $PODMAN play kube --service-container=true --log-driver journald $yaml_source &>/dev/null &

    # Wait for both containers to be running
    containers_running=
    for i in $(seq 1 20); do
        run_podman "?" container wait $container_a $container_b --condition="running"
        if [[ $status == 0 ]]; then
            containers_running=1
            break
        fi
        sleep 0.5
        # Just for debugging
        run_podman ps -a
    done
    if [[ -z "$containers_running" ]]; then
        die "container $container_a and/or $container_b did not start"
    fi

    wait_for_ready $container_a
    # Make sure the containers have the correct policy
    run_podman container inspect $container_a $container_b $service_container --format "{{.Config.SdNotifyMode}}"
    is "$output" "container
conmon
ignore"

    is "$(< $_SOCAT_LOG)" "" "nothing received on socket"

    # Make sure the container received a "proxy" socket and is not using the
    # one of `kube play`
    run_podman container inspect $container_a --format "{{.Config.SdNotifySocket}}"
    assert "$output" != $NOTIFY_SOCKET

    run_podman logs $container_a
    is "${lines[0]}" "/run/notify/notify.sock" "NOTIFY_SOCKET is passed to container"

    # Send the READY message.  Doing it in an exec session helps debug
    # potential issues.
    run_podman exec --env NOTIFY_SOCKET="/run/notify/notify.sock" $container_a /usr/bin/systemd-notify --ready

    # Instruct the container to stop.
    # Run detached as the `exec` session races with the cleanup process
    # of the exiting container (see #10825).
    run_podman exec -d $container_a /bin/touch /stop

    run_podman container wait $container_a
    run_podman container inspect $container_a --format "{{.State.ExitCode}}"
    is "$output" "0" "container exited cleanly after sending READY message"

    wait_for_file_content $_SOCAT_LOG "READY=1"

    # (for debugging)
    echo;echo "$_LOG_PROMPT cat $_SOCAT_LOG"
    run cat $_SOCAT_LOG
    echo "$output"

    assert "$output" =~ "MAINPID=.*
READY=1" "sdnotify sent MAINPID and READY"

    # Make sure that Podman is the service's MainPID
    main_pid=$(head -n1 <<<"$output" | awk -F= '{print $2}')
    is "$(</proc/$main_pid/comm)" "podman" "podman is the service mainPID ($main_pid)"
    _stop_socat

    # Clean up pod and pause image
    run_podman play kube --down $yaml_source
}

function generate_exit_code_yaml {
    local fname=$1
    local cmd1=$2
    local cmd2=$3
    local sdnotify_policy=$4
    local podname=p-$(safename)
    echo "
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $podname
  annotations:
    io.containers.sdnotify: "$sdnotify_policy"
spec:
  restartPolicy: Never
  containers:
    - name: ctr1
      image: $IMAGE
      command:
      - $cmd1
    - name: ctr2
      image: $IMAGE
      command:
      - $cmd2
" > $fname
}

# bats test_tags=ci:parallel
@test "podman kube play - exit-code propagation" {
    fname=$PODMAN_TMPDIR/$(random_string).yaml

    # Create a test matrix with the following arguments:
    # exit-code propagation | ctr1 command | ctr2 command | service-container exit code
    exit_tests="
all  | true  | true  | 0
all  | true  | false | 0
all  | false | false | 137
any  | true  | true  | 0
any  | false | true  | 137
any  | false | false | 137
none | true  | true  | 0
none | true  | false | 0
none | false | false | 0
"

    # In each iteration we switch between the sdnotify policy ignore and conmon.
    # We could run them in a loop for each case but the test is slow so let's
    # just switch between them as it should cover both policies sufficiently.
    # Note because of this make sure to have at least two exit code cases directly
    # after each other above so both polices will get at least once the error case.
    # The first run is using the default sdnotify policy of "ignore".
    # In this case, the service container serves as the main PID of the service
    # to have a minimal resource footprint.  The second run is using the
    # "conmon" sdnotify policy in which case Podman needs to serve as the main
    # PID to act as an sdnotify proxy; there Podman will wait for the service
    # container to exit and reflects its exit code.
    sdnotify_policy=ignore
    while read exit_code_prop cmd1 cmd2 exit_code; do
        generate_exit_code_yaml $fname $cmd1 $cmd2 $sdnotify_policy
        yaml_sha=$(sha256sum $fname)
        service_container="${yaml_sha:0:12}-service"
        podman_exit=$exit_code
        if [[ $sdnotify_policy == "ignore" ]];then
             podman_exit=0
        fi
        run_podman $podman_exit kube play --service-exit-code-propagation="$exit_code_prop" --service-container $fname
        # Make sure that there are no error logs (e.g., #19715)
        assert "$output" !~ "error msg="
        run_podman container inspect --format '{{.KubeExitCodePropagation}}' $service_container
        is "$output" "$exit_code_prop" "service container has the expected policy set in its annotations"
        run_podman wait $service_container
        is "$output" "$exit_code" "service container exit code (propagation: $exit_code_prop, policy: $sdnotify_policy, cmds: $cmd1 + $cmd2)"
        run_podman kube down $fname

        # in each iteration switch between conmon/ignore policy to get coverage for both
        if [[ $sdnotify_policy == "ignore" ]]; then
            sdnotify_policy=conmon
        else
            sdnotify_policy=ignore
        fi
    done < <(parse_table "$exit_tests")

    # A final smoke test to make sure bogus policies lead to an error
    run_podman 125 kube play --service-exit-code-propagation=bogus --service-container $fname
    is "$output" "Error: unsupported exit-code propagation \"bogus\"" "error on unsupported exit-code propagation"
}

# CANNOT BE PARALLELIZED. I don't know why. It flakes. Still investigating.
@test "podman pull - EXTEND_TIMEOUT_USEC" {
    # Make sure that Podman extends the start timeout via DBUS when running
    # inside a systemd unit (i.e., with NOTIFY_SOCKET set).  Extending the
    # timeout works by continuously sending EXTEND_TIMEOUT_USEC; Podman does
    # this at most 10 times, adding up to ~5min.

    image_on_local_registry=localhost:${PODMAN_LOGIN_REGISTRY_PORT}/i-$(safename):tag
    registry_flags="--tls-verify=false --creds ${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS}"
    start_registry

    export NOTIFY_SOCKET=$PODMAN_TMPDIR/notify-$(safename).sock
    _start_socat

    run_podman push $registry_flags $IMAGE $image_on_local_registry

    run_podman pull $registry_flags $image_on_local_registry
    is "${lines[1]}" "Pulling image //$image_on_local_registry inside systemd: setting pull timeout to 5m0s" "NOTIFY_SOCKET is passed to container"

    echo "$_LOG_PROMPT cat $_SOCAT_LOG"
    run cat $_SOCAT_LOG
    # The 'echo's help us debug failed runs
    echo "$output"
    is "$output" "EXTEND_TIMEOUT_USEC=30000000"

    run_podman rmi $image_on_local_registry
    _stop_socat
}

# bats test_tags=ci:parallel
@test "podman system service" {
    # This test makes sure that podman-system-service uses the NOTIFY_SOCKET
    # correctly and that it unsets it after sending the expected MAINPID and
    # READY message by making sure no EXTEND_TIMEOUT_USEC is sent on pull.

    # Start a local registry and pre-populate it with an image we'll pull later on.
    image_on_local_registry=localhost:${PODMAN_LOGIN_REGISTRY_PORT}/name:tag
    registry_flags="--tls-verify=false --creds ${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS}"
    start_registry
    run_podman push $registry_flags $IMAGE $image_on_local_registry

    export NOTIFY_SOCKET=$PODMAN_TMPDIR/notify.sock
    podman_socket="unix://$PODMAN_TMPDIR/podman.sock"
    envfile=$PODMAN_TMPDIR/envfile
    _start_socat

    (timeout --foreground -v --kill=10 30 $PODMAN system service -t0 $podman_socket &)

    wait_for_file $_SOCAT_LOG
    local timeout=10
    while [[ $timeout -gt 0 ]]; do
        run cat $_SOCAT_LOG
        # The 'echo's help us debug failed runs
        echo "socat log:"
        echo "$output"

        if [[ "$output" =~ "READY=1" ]]; then
            break
        fi
        timeout=$((timeout - 1))
        assert $timeout -gt 0 "Timed out waiting for podman-system-service to send expected data over NOTIFY_SOCKET"
        sleep 0.5
    done

    assert "$output" =~ "MAINPID=.*
READY=1" "podman-system-service sends expected data over NOTIFY_SOCKET"
    mainpid=${lines[0]:8}

    # Now pull remotely and make sure that the service does _not_ extend the
    # timeout; the NOTIFY_SOCKET should be unset at that point.
    run_podman --url $podman_socket pull $registry_flags $image_on_local_registry

    run cat $_SOCAT_LOG
    # The 'echo's help us debug failed runs
    echo "socat log:"
    echo "$output"
    assert "$output" !~ "EXTEND_TIMEOUT_USEC="

    # Give the system-service 5sec to terminate before killing it.
    kill -TERM $mainpid
    timeout=5
    while :;do
        if ! kill -0 $mainpid; then
            # Yay, it's gone
            break
        fi

        timeout=$((timeout - 1))
        if [[ $timeout -eq 0 ]]; then
            kill -KILL $mainpid
            break
        fi
        sleep 1
    done

    run_podman rmi $image_on_local_registry
    _stop_socat
}
# vim: filetype=sh
