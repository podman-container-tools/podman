#!/usr/bin/env bats   -*- bats -*-
#
# Tests generated configurations for systemd.
#

# bats file_tags=ci:parallel

load helpers
load helpers.systemd
load helpers.network

SERVICE_NAME=

UNIT_FILE=
TEMPLATE_FILE=

function setup() {
    skip_if_remote "systemd tests are meaningless over remote"

    SERVICE_NAME="podman-test-$(safename)"
    UNIT_FILE="$UNIT_DIR/$SERVICE_NAME.service"
    TEMPLATE_FILE="$UNIT_DIR/$SERVICE_NAME@.service"

    basic_setup
}

function teardown() {
    if [[ -n "$UNIT_FILE" ]] && [[ -e "$UNIT_FILE" ]]; then
        run systemctl stop "$SERVICE_NAME"
        if [ $status -ne 0 ]; then
            echo "# WARNING: systemctl stop failed in teardown: $output" >&3
        fi

        run systemctl reset-failed "$SERVICE_NAME"
        rm -f "$UNIT_FILE"
        systemctl daemon-reload
    fi

    basic_teardown
}

# Helper to atomically create a systemd unit file from a tmpfile
#
# Context:
#     $1  - file created by podman generate systemd; presumed to be in a tmpdir
#     $2  - desired service file path, presumed to be in /run
#
# We can't just mv one to the other, because mv is not atomic across
# filesystems. (We need atomic, to guarantee that there will never
# be an incomplete .service file). Hence the tmp extension.
# -Z is because /run and $TMPDIR have different SELinux contexts.
function mv-safely() {
    mv -Z "$1" "$2.tmp.$$" && mv -Z "$2.tmp.$$" "$2"
}

# Helper to start a systemd service running a container
function service_setup() {
    local extra_args="$1"
    # January 2024: we can no longer do "run_podman generate systemd" followed
    # by "echo $output >file", because generate-systemd is deprecated and now
    # says so loudly, to stderr, with no way to silence it. Since BATS gloms
    # stdout + stderr, that warning goes to the unit file. (Today's systemd
    # is forgiving about that, but RHEL8 systemd chokes with EINVAL)
    (
        cd $PODMAN_TMPDIR
        run_podman generate systemd --files --name \
               -e http_proxy -e https_proxy -e no_proxy \
               -e HTTP_PROXY -e HTTPS_PROXY -e NO_PROXY \
               $extra_args \
               --new $cname
        mv-safely "container-$cname.service" $UNIT_FILE
    )
    run_podman rm $cname

    systemctl daemon-reload

    # Also test enabling services (see #12438).
    run systemctl enable "$SERVICE_NAME"
    assert $status -eq 0 "Error enabling systemd unit $SERVICE_NAME: $output"

    systemctl_start "$SERVICE_NAME"

    run systemctl status "$SERVICE_NAME"
    assert $status -eq 0 "systemctl status $SERVICE_NAME: $output"
}

# Helper to stop a systemd service running a container
function service_cleanup() {
    run systemctl stop "$SERVICE_NAME"
    assert $status -eq 0 "Error stopping systemd unit $SERVICE_NAME: $output"

    # Regression test for #11304: confirm that unit stops into correct state
    local expected_state="$1"
    if [[ -n "$expected_state" ]]; then
        run systemctl show --property=ActiveState "$SERVICE_NAME"
        assert "$output" = "ActiveState=$expected_state" \
               "state of service after systemctl stop"
    fi

    run systemctl disable "$SERVICE_NAME"
    assert $status -eq 0 "Error disabling systemd unit $SERVICE_NAME: $output"

    run systemctl reset-failed "$SERVICE_NAME"

    rm -f "$UNIT_FILE"
    systemctl daemon-reload
}

# These tests can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
@test "podman generate - systemd - basic" {
    # Warn when a custom restart policy is used without --new (see #15284)
    run_podman create --restart=always $IMAGE
    cid="$output"
    run_podman 0+w generate systemd $cid
    require_warning "Container $cid has restart policy .*always.* which can lead to issues on shutdown" \
                    "generate systemd emits warning"
    run_podman rm -f $cid

    cname=c-$(safename)
    # See #7407 for --pull=always.
    run_podman create --pull=always --name $cname --label "io.containers.autoupdate=registry" $IMAGE \
        sh -c "trap 'echo Received SIGTERM, finishing; exit' SIGTERM; echo WAITING; while :; do sleep 0.1; done"

    # Start systemd service to run this container
    service_setup

    # Give container time to start; make sure output looks top-like
    sleep 2
    run_podman logs $cname
    is "$output" ".*WAITING.*" "running is waiting for signal"

    # All good. Stop service, clean up.
    # Also make sure the service is in the `inactive` state (see #11304).
    service_cleanup inactive
}

@test "podman autoupdate local" {
    # Note that the entrypoint may be a JSON string which requires preserving the quotes (see #12477)
    cname=c-$(safename)

    # Create a scratch image (copy of our regular one)
    image_copy=base-$(safename)
    run_podman tag $IMAGE $image_copy

    # Create a container based on that
    run_podman create --name $cname --label "io.containers.autoupdate=local" --entrypoint '["top"]' $image_copy

    # Start systemd service to run this container
    service_setup

    # Give container time to start; make sure output looks top-like
    wait_for_output 'Load average' $cname

    # Run auto-update and check that it restarted the container
    run_podman commit --change "CMD=/bin/bash" $cname $image_copy
    run_podman auto-update
    is "$output" ".*$SERVICE_NAME.*" "autoupdate local restarted container"

    # All good. Stop service, clean up.
    service_cleanup
    run_podman rmi $image_copy
}

# These tests can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
@test "podman generate systemd - envar" {
    cname=c-$(safename)
    FOO=value BAR=%s run_podman create --name $cname --env FOO -e BAR --env MYVAR=myval \
        $IMAGE sh -c 'printenv && echo READY; trap 'exit' SIGTERM; while :; do sleep 0.1; done'

    # Start systemd service to run this container
    service_setup

    # Give container time to start and print output
    # wait_for_ready returns directly if the logs matches and preserves $output
    # for us so we do not have to call podman logs again here if we match the env below.
    wait_for_ready $cname

    is "$output" ".*FOO=value.*" "FOO environment variable set"
    is "$output" ".*BAR=%s.*" "BAR environment variable set"
    is "$output" ".*MYVAR=myval.*" "MYVAL environment variable set"

    # All good. Stop service, clean up.
    service_cleanup
}

# Regression test for #11438
@test "podman generate systemd - restart policy & timeouts" {
    cname=c1-$(safename)
    run_podman create --restart=always --name $cname $IMAGE
    run_podman generate systemd --new $cname
    is "$output" ".*Restart=always.*" "Use container's restart policy if set"
    run_podman generate systemd --new --restart-policy=on-failure $cname
    is "$output" ".*Restart=on-failure.*" "Override container's restart policy"

    cname2=c2-$(safename)
    run_podman create --restart=unless-stopped --name $cname2 $IMAGE
    run_podman generate systemd --new $cname2
    is "$output" ".*Restart=always.*" "unless-stopped translated to always"

    cname3=c3-$(safename)
    run_podman create --restart=on-failure:42 --name $cname3 $IMAGE
    run_podman generate systemd --new $cname3
    is "$output" ".*Restart=on-failure.*" "on-failure:xx is parsed correctly"
    is "$output" ".*StartLimitBurst=42.*" "on-failure:xx is parsed correctly"

    run_podman rm -t 0 -f $cname $cname2 $cname3
}

function set_listen_env() {
    export LISTEN_PID="100" LISTEN_FDS="1" LISTEN_FDNAMES="listen_fdnames"
}

function unset_listen_env() {
    unset LISTEN_PID LISTEN_FDS LISTEN_FDNAMES
}

function check_listen_env() {
    local stdenv="$1"
    local context="$2"
    if is_remote; then
        is "$output" "$stdenv" "LISTEN Environment did not pass: $context"
    else
        out=$(for o in $output; do echo $o; done| sort)
        std=$(echo "$stdenv
LISTEN_PID=1
LISTEN_FDS=1
LISTEN_FDNAMES=listen_fdnames" | sort)
       echo "<$out>"
       echo "<$std>"
       is "$out" "$std" "LISTEN Environment passed: $context"
    fi
}

@test "podman pass LISTEN environment " {
    # Note that `--hostname=host1` makes sure that all containers have the same
    # environment.
    run_podman run --hostname=host1 --rm $IMAGE printenv
    stdenv=$output

    # podman run
    set_listen_env
    run_podman run --hostname=host1 --rm $IMAGE printenv
    unset_listen_env
    check_listen_env "$stdenv" "podman run"

    # podman start
    run_podman create --hostname=host1 --rm $IMAGE printenv
    cid="$output"
    set_listen_env
    run_podman start --attach $cid
    unset_listen_env
    check_listen_env "$stdenv" "podman start"
}

@test "podman generate - systemd template" {
    cname=c-$(safename)
    run_podman create --name $cname $IMAGE top

    # See note in service_setup() above re: using --files
    (
        cd $PODMAN_TMPDIR
        run_podman generate systemd --template --files -n $cname
        mv-safely "container-$cname.service" $TEMPLATE_FILE
    )
    run_podman rm -f $cname

    systemctl daemon-reload

    INSTANCE="$SERVICE_NAME@1.service"
    systemctl_start "$INSTANCE"

    run systemctl status "$INSTANCE"
    assert $status -eq 0 "systemctl status $INSTANCE: $output"

    run systemctl stop "$INSTANCE"
    assert $status -eq 0 "Error stopping systemd unit $INSTANCE: $output"

    run systemctl reset-failed "$INSTANCE"

    rm -f $TEMPLATE_FILE
    systemctl daemon-reload
}

@test "podman generate - systemd template no support for pod" {
    cname=c-$(safename)
    podname=p-$(safename)
    run_podman pod create --name $podname
    run_podman run --pod $podname -dt --name $cname $IMAGE top

    run_podman 125 generate systemd --new --template -n $podname
    is "$output" ".*--template is not supported for pods.*" "Error message contains 'not supported'"

    run_podman rm -f $cname
    run_podman pod rm -f $podname
}

@test "podman generate - systemd template only used on --new" {
    cname=c-$(safename)
    run_podman create --name $cname $IMAGE top
    run_podman 125 generate systemd --new=false --template -n $cname
    is "$output" ".*--template cannot be set" "Error message should be '--template requires --new'"
    run_podman rm $cname
}

@test "podman --cgroup=cgroupfs doesn't show systemd warning" {
    DBUS_SESSION_BUS_ADDRESS= run_podman --log-level warning --cgroup-manager=cgroupfs info -f ''
    is "$output" "" "output should be empty"
}

@test "podman --systemd sets container_uuid" {
    cname=c-$(safename)
    run_podman run --systemd=always --name $cname $IMAGE printenv container_uuid
    container_uuid=$output
    run_podman inspect $cname --format '{{ .ID }}'
    is "${container_uuid}" "${output:0:32}" "UUID should be first 32 chars of Container id"
    run_podman rm $cname
}

# https://github.com/containers/podman/issues/13153
@test "podman rootless-netns processes should be in different cgroup" {
    is_rootless || skip "only meaningful for rootless"

    cname=c-$(safename)
    local netname=testnet-$(safename)

    # create network and container with network
    run_podman network create $netname
    run_podman create --name $cname --network $netname $IMAGE top

    # run container in systemd unit
    service_setup

    # run second container with network
    cname2=c2-$(safename)
    run_podman run -d --name $cname2 --network $netname $IMAGE top

    # stop systemd container
    service_cleanup

    pasta_iface=$(default_ifname 4)
    assert "$pasta_iface" != "" "pasta_iface is set"

    # now check that the rootless netns pasta process is still alive and working
    run_podman unshare --rootless-netns ip addr
    is "$output" ".*$pasta_iface.*" "pasta interface exists in the netns"
    run_podman exec $cname2 nslookup google.com

    run_podman rm -f -t0 $cname2
    run_podman network rm -f $netname
}

@test "podman create --health-on-failure=kill" {
    cname=c-$(safename)
    run_podman create --name $cname                  \
               --health-cmd /home/podman/healthcheck \
               --health-on-failure=kill              \
               --health-retries=1                    \
               --restart=on-failure                  \
               $IMAGE /home/podman/pause

    # run container in systemd unit
    service_setup "-e DISABLE_HC_SYSTEMD=true"

    run_podman container inspect $cname --format "{{.ID}}"
    oldID="$output"

    run_podman healthcheck run $cname

    # Now cause the healthcheck to fail
    run_podman exec $cname touch /uh-oh

    # healthcheck should now fail, with exit status 1 and 'unhealthy' output
    run_podman 1 healthcheck run $cname
    is "$output" "unhealthy" "output from 'podman healthcheck run'"

    # What is expected to happen now:
    #  1) The container gets killed as the health check has failed
    #  2) Systemd restarts the service as the restart policy is set to "on-failure"
    #  3) The /uh-oh file is gone and $cname has another ID

    # Wait at most 10 seconds for the service to be restarted
    local timeout=10
    while [[ $timeout -gt 1 ]]; do
        # Possible outcomes:
        #  - status 0, old container is still terminating: sleep and retry
        #  - status 0, new CID: yay, break
        #  - status 1, container not found: sleep and retry
        run_podman '?' container inspect $cname --format '{{.ID}}'
        if [[ $status == 0 ]]; then
            if [[ "$output" != "$oldID" ]]; then
                break
            fi
        fi
        sleep 1
        let timeout=$timeout-1
    done

    run_podman healthcheck run $cname

    # stop systemd container
    service_cleanup
}

@test "podman-kube@.service template" {
    install_kube_template
    # Create the YAMl file
    yaml_source="$PODMAN_TMPDIR/test.yaml"
    podname=p-$(safename)
    c1=c1-$(safename)
    c2=c2-$(safename)
    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  annotations:
      io.containers.autoupdate: "local"
      io.containers.autoupdate/$c2: "registry"
  labels:
    app: test
  name: $podname
spec:
  containers:
  - command:
    - sh
    - -c
    - echo c1 stdout; echo c1 stderr 1>&2; trap 'exit' SIGTERM; while :; do sleep 0.1; done
    image: $IMAGE
    name: $c1
  - command:
    - sh
    - -c
    - echo c2 stdout; echo c2 stderr 1>&2; trap 'exit' SIGTERM; while :; do sleep 0.1; done
    image: $IMAGE
    name: $c2
EOF

    # Dispatch the YAML file
    service_name="podman-kube@$(systemd-escape $yaml_source).service"
    systemctl_start $service_name
    systemctl is-active $service_name

    # Make sure that Podman is the service's MainPID
    run systemctl show --property=MainPID --value $service_name
    is "$(</proc/$output/comm)" "conmon" "podman is the service mainPID"

    # The name of the service container is predictable: the first 12 characters
    # of the hash of the YAML file followed by the "-service" suffix
    yaml_sha=$(sha256sum $yaml_source)
    service_container="${yaml_sha:0:12}-service"

    # Make sure that the service container exists and runs.
    run_podman container inspect $service_container --format "{{.State.Running}}"
    is "$output" "true"

    # Check for an error when trying to remove the service container
    run_podman 125 container rm $service_container
    is "$output" "Error: container .* is the service container of pod(s) .* and cannot be removed without removing the pod(s)"

    # containers/podman/issues/17482: verify that the log-driver for the Pod's containers is NOT passthrough
    for name in "c1" "c2"; do
        run_podman container inspect ${podname}-${!name} --format "{{.HostConfig.LogConfig.Type}}"
        assert $output != "passthrough"
        # check that we can get the logs with passthrough when we run in a systemd unit
        run_podman logs ${podname}-${!name}
        assert "$output" == "$name stdout
$name stderr" "logs work with passthrough"
    done

    # we cannot assume the ordering between a b, this depends on timing and would flake in CI
    # use --names so we do not have to get the ID
    run_podman pod logs --names $podname
    assert "$output" =~ ".*^${podname}-${c1} c1 stdout.*" "logs from container 1 shown"
    assert "$output" =~ ".*^${podname}-${c2} c2 stdout.*" "logs from container 2 shown"

    # Add a simple `auto-update --dry-run` test here to avoid too much redundancy
    # with 255-auto-update.bats
    run_podman auto-update --dry-run --format "{{.Unit}},{{.Container}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*$service_name,.* (${podname}-${c1}),$IMAGE,false,local.*" "global auto-update policy gets applied"
    is "$output" ".*$service_name,.* (${podname}-${c2}),$IMAGE,false,registry.*" "container-specified auto-update policy gets applied"

    # Kill the pod and make sure the service is not running.
    run_podman pod kill $podname
    for i in {0..20}; do
        # echos are for debugging test flakes
        echo "$_LOG_PROMPT systemctl is-active $service_name"
        run systemctl is-active $service_name
        echo "$output"
        if [[ "$output" == "inactive" ]]; then
            break
        fi
        sleep 0.5
    done
    is "$output" "inactive" "systemd service transitioned to 'inactive' state: $service_name"

    # Now stop and start the service again.
    systemctl stop $service_name
    systemctl_start $service_name
    systemctl is-active $service_name
    run_podman container inspect $service_container --format "{{.State.Running}}"
    is "$output" "true"

    # Clean up
    systemctl stop $service_name
    run_podman 1 container exists $service_container
    run_podman 1 pod exists $podname
}

@test "podman generate - systemd - DEPRECATED" {
    run_podman generate systemd --help
    is "$output" ".*[DEPRECATED] command:"
    is "$output" ".*\[DEPRECATED\] Generate systemd units.*"

    cname=c-$(safename)
    run_podman create --name $cname $IMAGE
    run_podman generate systemd $cname >/dev/null
    is "$output" ".*[DEPRECATED] command:"
    run_podman generate --help
    is "$output" ".*\[DEPRECATED\] Generate systemd units"
    run_podman rm $cname
}

@test "podman passes down the KillSignal and StopTimeout setting" {
    ctr=systemd_test_$(safename)

    run_podman run -d --name $ctr --stop-signal 5 --stop-timeout 7 --rm $IMAGE top
    run_podman inspect $ctr --format '{{ .Id }}'
    id="$output"

    run systemctl show -p TimeoutStopUSec "libpod-${id}.scope"
    assert "$output" == "TimeoutStopUSec=7s"

    run systemctl show -p KillSignal "libpod-${id}.scope"
    assert "$output" == "KillSignal=5"

    # Clean up
    run_podman rm -t 0 -f $ctr
}
# vim: filetype=sh
