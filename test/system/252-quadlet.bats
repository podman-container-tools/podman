#!/usr/bin/env bats   -*- bats -*-
#
# Tests generated configurations for systemd.
#

# bats file_tags=ci:parallel

load helpers
load helpers.network
load helpers.registry
load helpers.systemd

UNIT_FILES=()
SERVICES_TO_STOP=()

function start_time() {
    sleep_to_next_second # Ensure we're on a new second with no previous logging
    STARTED_TIME=$(date "+%F %R:%S") # Start time for new log time
}

function setup() {
    skip_if_remote "quadlet tests are meaningless over remote"
    skip_if_journald_unavailable "Needed for RHEL. FIXME: we might be able to re-enable a subset of tests."

    test -x "$QUADLET" || die "Cannot run quadlet tests without executable \$QUADLET ($QUADLET)"

    start_time

    # Clear arrays for each test
    SERVICES_TO_STOP=()

    basic_setup
}

function teardown() {
    # Stop manually specified services
    for service in ${SERVICES_TO_STOP[@]}; do
        run systemctl stop "$service"
        if [ $status -ne 0 ]; then
           echo "# WARNING: systemctl stop failed in teardown: $output" >&3
        fi
        run systemctl reset-failed "$service"
    done

    for UNIT_FILE in ${UNIT_FILES[@]}; do
        if [[ -e "$UNIT_FILE" ]]; then
            local service=$(basename "$UNIT_FILE")
            # Skip stopping template services (those ending with '@')
            # as they cannot be stopped directly without an instance name
            if [[ ! "$service" =~ @\.service$ ]]; then
                run systemctl stop "$service"
                if [ $status -ne 0 ]; then
                   echo "# WARNING: systemctl stop failed in teardown: $output" >&3
                fi
            fi
            run systemctl reset-failed "$service"
            rm -f "$UNIT_FILE"
        fi
    done
    systemctl daemon-reload

    basic_teardown
}

# Converts the quadlet file and installs the result it in $UNIT_DIR
function run_quadlet() {
    local sourcefile="$1"
    local service=$(quadlet_to_service_name "$sourcefile")

    # quadlet always works on an entire directory, so copy the file
    # to transform to the given or newly created tmpdir
    local quadlet_tmpdir="$2"
    if [ -z "$quadlet_tmpdir" ]; then
        quadlet_tmpdir=$(mktemp -d --tmpdir=$PODMAN_TMPDIR quadlet.XXXXXX)
    fi
    cp $sourcefile $quadlet_tmpdir/

    echo "$_LOG_PROMPT $QUADLET $_DASHUSER $UNIT_DIR"
    QUADLET_UNIT_DIRS="$quadlet_tmpdir" run \
                     timeout --foreground -v --kill=10 $PODMAN_TIMEOUT \
                     $QUADLET $_DASHUSER $UNIT_DIR
    echo "$output"
    assert $status -eq 0 "Failed to convert quadlet file: $sourcefile"
    is "$output" "" "quadlet should report no errors"

    run cat $UNIT_DIR/$service
    assert $status -eq 0 "Could not cat $UNIT_DIR/$service"
    echo "$output"
    local content="$output"

    # Ensure this is teared down
    UNIT_FILES+=("$UNIT_DIR/$service")

    QUADLET_SERVICE_NAME="$service"
    QUADLET_SERVICE_CONTENT="$content"
    QUADLET_SYSLOG_ID="$(basename $service .service)"
    QUADLET_CONTAINER_NAME="systemd-$QUADLET_SYSLOG_ID"
}

function service_setup() {
    local service="$1"
    local option="$2"

    systemctl daemon-reload

    local startargs=""
    local statusexit=0
    local activestate="active"

    # If option wait, start and wait for service to exist
    if [ "$option" == "wait" ]; then
        startargs="--wait"
        statusexit=3
        local activestate="inactive"
    fi

    systemctl_start $startargs "$service"

    # FIXME FIXME FIXME: this is racy with short-lived containers!
    echo "$_LOG_PROMPT systemctl status $service"
    run systemctl status "$service"
    echo "$output"
    assert $status -eq $statusexit "systemctl status $service"

    echo "$_LOG_PROMPT systemctl show --value --property=ActiveState $service"
    run systemctl show --value --property=ActiveState "$service"
    echo "$output"
    assert $status -eq 0 "systemctl show $service"
    is "$output" $activestate
}

# Helper to stop a systemd service running a container
function service_cleanup() {
    local service="$1"
    local expected_state="$2"

    run systemctl stop "$service"
    assert $status -eq 0 "Error stopping systemd unit $service: $output"

    # Regression test for #11304: confirm that unit stops into correct state
    if [[ -n "$expected_state" ]]; then
        run systemctl show --property=ActiveState "$service"
        assert "$output" = "ActiveState=$expected_state" \
               "state of service $service after systemctl stop"
    fi

    # reset-failed necessary to clean up stray systemd cruft
    run systemctl reset-failed "$service"
    rm -f "$UNIT_DIR/$service"
    systemctl daemon-reload
}

function create_secret() {
    local secret_name=$(random_string)
    local secret_file=$PODMAN_TMPDIR/secret_$(random_string)
    local secret=$(random_string)

    echo $secret > $secret_file
    run_podman secret create $secret_name $secret_file

    SECRET_NAME=$secret_name
    SECRET=$secret
}

function remove_secret() {
    local secret_name="$1"

    run_podman secret rm $secret_name
}

function wait_for_journal() {
    local step=1
    local count=10
    local expect_str=

    while [ "$#" -gt 0 ]; do
        case "$1" in
            -s|--step)
                step="$2"
                shift 2
                ;;
            -c|--count)
                count="$2"
                shift 2
                ;;
            *)
                expect_str="$1"
                shift 1
                ;;
        esac
    done

    while [ "$count" -gt 0 ]; do
        run journalctl "--since=$STARTED_TIME" --unit="$QUADLET_SERVICE_NAME"
        if [[ "$output" =~ "$expect_str" ]]; then
            return
        fi
        sleep "$step"
        count=$(( count - 1 ))
    done
    die "Timed out waiting for '$expect_str' in journalctl output"
}

@test "quadlet - basic" {
    # Network=none is to work around a Pasta bug, can be removed once a patched Pasta is available.
    # Ref https://github.com/containers/podman/pull/21563#issuecomment-1965145324
    local quadlet_file=$PODMAN_TMPDIR/basic_$(safename).container
    cat > $quadlet_file <<EOF
[Service]
Environment=SUPPRESS_BOLTDB_WARNING=true
[Container]
Image=$IMAGE
# Note it is important that the trap is before the ready message,
# otherwise the signal handler may not registered in time before we do systemctl stop.
Exec=sh -c "echo STARTED CONTAINER; trap 'exit' SIGTERM; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; while :; do sleep 0.1; done"
Notify=yes
LogDriver=passthrough
Network=none
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    run -0 systemctl show --property=Wants --property=After "$QUADLET_SERVICE_NAME"
    service="network-online.target"
    if is_rootless; then
        service="podman-user-wait-network-online.service"
    fi
    assert "${lines[0]}" == "Wants=$service" "quadlet unit Wants network dependency"
    # Note systemd adds some other default services to After= so no exact match possible
    assert "${lines[1]}" =~ "After=.*$service.*" "quadlet unit After network dependency"

    # Check that we can read the logs from the container with podman logs even
    # with the `passthrough` driver.  The log may need a short period of time
    # to bubble up into the journal logs, so wait for it.
    wait_for_output "STARTED CONTAINER" $QUADLET_CONTAINER_NAME
    # Make sure it's an *exact* match, not just a substring (i.e. no spurious
    # warnings or other cruft).
    run_podman logs $QUADLET_CONTAINER_NAME
    assert "$output" == "STARTED CONTAINER" "exact/full match when using the 'passthrough' driver"

    # Also look for the logs via `journalctl`.
    run journalctl "--since=$STARTED_TIME" --unit="$QUADLET_SERVICE_NAME"
    is "$output" '.*STARTED CONTAINER.*'

    run_podman container inspect  --format "{{.State.Status}}" $QUADLET_CONTAINER_NAME
    is "$output" "running" "container should be started by systemd and hence be running"

    service_cleanup $QUADLET_SERVICE_NAME inactive
}

@test "quadlet conflict names" {
    # If two directories in the search have files with the same name, quadlet should
    # only process the first name
    dir1=$PODMAN_TMPDIR/$(random_string)
    dir2=$PODMAN_TMPDIR/$(random_string)
    local quadlet_file=basic_$(safename).container
    mkdir -p $dir1 $dir2

    cat > $dir1/$quadlet_file <<EOF
[Container]
Image=quay.io/libpod/this-is-the-one:wewant
Notify=yes
EOF

    cat > $dir2/$quadlet_file <<EOF
[Container]
Image=quay.io/libpod/bad-bad-bad:nonono
Notify=no
EOF
    QUADLET_UNIT_DIRS="$dir1:$dir2" run \
                    timeout --foreground -v --kill=10 $PODMAN_TIMEOUT \
                    $QUADLET --dryrun
    assert "$output" =~ "Notify=yes" "quadlet should show Notify=yes"
    assert "$output" !~ "Notify=no" "quadlet should not show Notify=no"
}

@test "quadlet - envvar" {
    local quadlet_file=$PODMAN_TMPDIR/envvar_$(safename).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo OUTPUT: \"\$FOOBAR\" \"\$BAR\""
Environment="FOOBAR=Foo  Bar" BAR=bar
LogDriver=passthrough
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME wait

    # Ensure we have the right output, sync is done via waiting for service to exit (service_setup wait)

    # Note: Here we have to filter by syslog id instead of unit, because there is a (known) race
    # condition where if the cgroup is cleaned up before journald sees the message, then the journal
    # doesn't know the cgroup  and thus not the unit. (See https://github.com/systemd/systemd/issues/2913)
    run journalctl "--since=$STARTED_TIME"  SYSLOG_IDENTIFIER="$QUADLET_SYSLOG_ID"
    is "$output" '.*OUTPUT: Foo  Bar bar.*'

    service_cleanup $QUADLET_SERVICE_NAME inactive
}

@test "quadlet - ContainerName and journal output check" {
    local quadlet_file=$PODMAN_TMPDIR/containername_$(safename).container
    local token_out="STDOUT$(random_string 10)"
    local token_err="STDERR$(random_string 10)"
    cat > $quadlet_file <<EOF
[Container]
ContainerName=customcontainername
Image=$IMAGE
Exec=sh -c "echo $token_out; echo $token_err 1>&2; top -d 10"
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we can access with the custom container name
    run_podman container inspect  --format "{{.State.Status}}" customcontainername
    is "$output" "running" "container should be started by systemd and hence be running"

    wait_for_journal "Started $QUADLET_SERVICE_NAME"

    run journalctl "--since=$STARTED_TIME" --unit="$QUADLET_SERVICE_NAME"
    assert "$output" =~ "$token_out" "Output can be found with journalctl"
    assert "$output" =~ "$token_err" "Error can be found with journalctl"
    assert "$output" =~ "Starting $QUADLET_SERVICE_NAME" "Status information can be found with journalctl"

    # log priority 3 in journalctl is err. This is documented in syslog(3)
    run journalctl "--since=$STARTED_TIME" --priority=3 --unit="$QUADLET_SERVICE_NAME"
    assert "$output" =~ "$token_err" "Error can be found with journalctl --priority=3"
    assert "$output" !~ "$token_out" "Output can not be found with journalctl --priority=3"

    service_cleanup $QUADLET_SERVICE_NAME failed
}

@test "quadlet - labels" {
    local quadlet_file=$PODMAN_TMPDIR/labels_$(safename).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=top
Label="foo=foo bar" "key=val"
Annotation="afoo=afoo bar"
Annotation="akey=aval"
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    run_podman container inspect --format "{{.Config.Labels.foo}}" $QUADLET_CONTAINER_NAME
    is "$output" "foo bar"
    run_podman container inspect --format "{{.Config.Labels.key}}" $QUADLET_CONTAINER_NAME
    is "$output" "val"
    run_podman container inspect --format "{{.Config.Annotations.afoo}}" $QUADLET_CONTAINER_NAME
    is "$output" "afoo bar"
    run_podman container inspect --format "{{.Config.Annotations.akey}}" $QUADLET_CONTAINER_NAME
    is "$output" "aval"

    service_cleanup $QUADLET_SERVICE_NAME failed
}

@test "quadlet - oneshot" {
    local quadlet_file=$PODMAN_TMPDIR/oneshot_$(safename).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=echo INITIALIZED

[Service]
Type=oneshot
RemainAfterExit=yes
EOF

    run_quadlet "$quadlet_file"

    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output. Output is synced by oneshot command exit
    run journalctl "--since=$STARTED_TIME"  SYSLOG_IDENTIFIER="$QUADLET_SYSLOG_ID"
    is "$output" '.*INITIALIZED.*'

    service_cleanup $QUADLET_SERVICE_NAME inactive
}

@test "quadlet - volume" {
    local quadlet_file=$PODMAN_TMPDIR/basic_$(safename).volume
    cat > $quadlet_file <<EOF
[Volume]
Label=foo=bar other="with space"
EOF

    run_quadlet "$quadlet_file"

    service_setup $QUADLET_SERVICE_NAME

    local volume_name=systemd-$(basename $quadlet_file .volume)
    run_podman volume ls
    is "$output" ".*local.*${volume_name}.*"

    run_podman volume inspect  --format "{{.Labels.foo}}" $volume_name
    is "$output" "bar"
    run_podman volume inspect  --format "{{.Labels.other}}" $volume_name
    is "$output" "with space"

    service_cleanup $QUADLET_SERVICE_NAME inactive
    run_podman volume rm $volume_name
}

@test "quadlet - volume - uid - gid" {
    local quadlet_file=$PODMAN_TMPDIR/basic_$(safename).volume
    cat > $quadlet_file <<EOF
[Volume]
UID=1234
GID=5678
EOF

    run_quadlet "$quadlet_file"

    service_setup $QUADLET_SERVICE_NAME

    local volume_name=systemd-$(basename $quadlet_file .volume)
    run_podman volume ls
    is "$output" ".*local.*${volume_name}.*"

    run_podman volume inspect  --format "{{.UID}}" $volume_name
    is "$output" "1234"

    run_podman volume inspect  --format "{{.GID}}" $volume_name
    is "$output" "5678"

    service_cleanup $QUADLET_SERVICE_NAME inactive
    run_podman volume rm $volume_name
}

# A quadlet container depends on a quadlet volume
@test "quadlet - volume dependency" {
    # Save the unit name to use as the volume for the container
    local quadlet_vol_unit=dep_$(safename).volume
    local quadlet_vol_file=$PODMAN_TMPDIR/${quadlet_vol_unit}
    cat > $quadlet_vol_file <<EOF
[Volume]
EOF

    local quadlet_tmpdir=$(mktemp -d --tmpdir=$PODMAN_TMPDIR quadlet.XXXXXX)
    # Have quadlet create the systemd unit file for the volume unit
    run_quadlet "$quadlet_vol_file" "$quadlet_tmpdir"

    # Save the volume service name since the variable will be overwritten
    local vol_service=$QUADLET_SERVICE_NAME
    local volume_name=systemd-$(basename $quadlet_vol_file .volume)

    local quadlet_file=$PODMAN_TMPDIR/user_$(safename).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=top
Volume=$quadlet_vol_unit:/tmp
EOF

    # Have quadlet create the systemd unit file for the container unit
    run_quadlet "$quadlet_file" "$quadlet_tmpdir"

    # Save the container service name for readability
    local container_service=$QUADLET_SERVICE_NAME

    # Volume should not exist
    run_podman 1 volume exists ${volume_name}

    # Start the container service which should also trigger the start of the volume service
    service_setup $container_service

    # Volume system unit should be active
    run systemctl show --property=ActiveState "$vol_service"
    assert "$output" = "ActiveState=active" \
           "volume should be active via dependency"

    # Volume should exist
    run_podman volume exists ${volume_name}

    # Shutdown the service and remove the volume
    service_cleanup $container_service failed
    run_podman volume rm $volume_name
}

# A quadlet container template depends on a quadlet volume and network templates
@test "quadlet - template dependency" {
    # Save the unit name to use as the volume template for the container template
    local quadlet_vol_unit=dep_$(safename)@.volume
    local quadlet_vol_file=$PODMAN_TMPDIR/${quadlet_vol_unit}
    cat > $quadlet_vol_file <<EOF
[Volume]
EOF

    local quadlet_tmpdir=$(mktemp -d --tmpdir=$PODMAN_TMPDIR quadlet.XXXXXX)
    # Have quadlet create the systemd unit file for the volume template unit
    run_quadlet "$quadlet_vol_file" "$quadlet_tmpdir"

    # Save the volume service name since the variable will be overwritten
    local vol_service=$QUADLET_SERVICE_NAME
    local volume_name=systemd-$(basename $quadlet_vol_file .volume)
    # For template units, the volume name should have -%i appended
    volume_name=${volume_name%@}-%i

    # Save the unit name to use as the network template for the container template
    local quadlet_net_unit=dep_$(safename)@.network
    local quadlet_net_file=$PODMAN_TMPDIR/${quadlet_net_unit}
    cat > $quadlet_net_file <<EOF
[Network]
EOF

    # Have quadlet create the systemd unit file for the network template unit
    run_quadlet "$quadlet_net_file" "$quadlet_tmpdir"

    # Save the network service name since the variable will be overwritten
    local net_service=$QUADLET_SERVICE_NAME
    local network_name=systemd-$(basename $quadlet_net_file .network)
    # For template units, the network name should have -%i appended
    network_name=${network_name%@}-%i

    local quadlet_file=$PODMAN_TMPDIR/user_$(safename)@.container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=top
Volume=$quadlet_vol_unit:/tmp
Network=$quadlet_net_unit
EOF

    # Have quadlet create the systemd unit file for the container template unit
    run_quadlet "$quadlet_file" "$quadlet_tmpdir"

    # Save the container service name for readability
    local container_service=$QUADLET_SERVICE_NAME

    # Create instance names for the template units
    local instance_name="test"
    local vol_service_instance="${vol_service%@*}@${instance_name}.service"
    local net_service_instance="${net_service%@*}@${instance_name}.service"
    local container_service_instance="${container_service%@*}@${instance_name}.service"
    local volume_name_instance="systemd-dep_$(safename)-${instance_name}"
    local network_name_instance="systemd-dep_$(safename)-${instance_name}"

    # Volume should not exist
    run_podman 1 volume exists ${volume_name_instance}
    # Network should not exist
    run_podman 1 network exists ${network_name_instance}

    # Start the container service instance which should also trigger the start of the volume service instance
    service_setup $container_service_instance

    # Add the service instances to SERVICES_TO_STOP for proper cleanup
    # SERVICES_TO_STOP+=("$container_service_instance")
    SERVICES_TO_STOP+=("$vol_service_instance")
    SERVICES_TO_STOP+=("$net_service_instance")

    # Volume system unit instance should be active
    run systemctl show --property=ActiveState "$vol_service_instance"
    assert "$output" = "ActiveState=active" \
           "volume template instance should be active via dependency"

    # Network system unit instance should be active
    run systemctl show --property=ActiveState "$net_service_instance"
    assert "$output" = "ActiveState=active" \
           "network template instance should be active via dependency"

    # Volume should exist
    run_podman volume exists ${volume_name_instance}

    # Network should exist
    run_podman network exists ${network_name_instance}

    # Clean up the created resources
    service_cleanup $container_service_instance failed
    run_podman volume rm $volume_name_instance
    run_podman network rm $network_name_instance
}

# A quadlet container depends on a named quadlet volume
@test "quadlet - named volume dependency" {
    local volume_name="v-$(safename)"

    # Save the unit name to use as the volume for the container
    local quadlet_vol_unit=dep_$(safename).volume
    local quadlet_vol_file=$PODMAN_TMPDIR/${quadlet_vol_unit}
    cat > $quadlet_vol_file <<EOF
[Volume]
VolumeName=$volume_name
EOF

    # Have quadlet create the systemd unit file for the volume unit
    local quadlet_tmpdir=$(mktemp -d --tmpdir=$PODMAN_TMPDIR quadlet.XXXXXX)
    run_quadlet "$quadlet_vol_file" "$quadlet_tmpdir"

    # Save the volume service name since the variable will be overwritten
    local vol_service=$QUADLET_SERVICE_NAME

    local quadlet_file=$PODMAN_TMPDIR/user_$(safename).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=top
Volume=$quadlet_vol_unit:/tmp
EOF

    # Have quadlet create the systemd unit file for the container unit
    run_quadlet "$quadlet_file" "$quadlet_tmpdir"

    # Save the container service name for readability
    local container_service=$QUADLET_SERVICE_NAME
    local container_name=systemd-$(basename $quadlet_file .container)

    # Volume should not exist
    run_podman 1 volume exists ${volume_name}

    # Start the container service which should also trigger the start of the volume service
    service_setup $container_service

    # Volume system unit should be active
    run systemctl show --property=ActiveState "$vol_service"
    assert "$output" = "ActiveState=active" "volume should be active via dependency"

    # Volume should exist
    run_podman volume exists ${volume_name}

    # Container should be attached to defined volume
    run_podman container inspect --format "{{(index .Mounts 0).Name}}" $container_name
    assert "$output" = "$volume_name" "container should be attached to network $volume_name"

    # Shutdown the service and remove the volume
    service_cleanup $container_service failed
    run_podman volume rm $volume_name
}

@test "quadlet - network" {
    local quadlet_file=$PODMAN_TMPDIR/basic_$(safename).network
    cat > $quadlet_file <<EOF
[Network]
Label=foo=bar other="with space"
EOF

    run_quadlet "$quadlet_file"

    service_setup $QUADLET_SERVICE_NAME

    local network_name=systemd-$(basename $quadlet_file .network)
    run_podman network exists $network_name

    run_podman network inspect  --format "{{.Labels.foo}}" $network_name
    is "$output" "bar"
    run_podman network inspect  --format "{{.Labels.other}}" $network_name
    is "$output" "with space"

    service_cleanup $QUADLET_SERVICE_NAME inactive
    run_podman network rm $network_name
}

@test "quadlet - network delete with dependencies" {
    # Save the unit name to use as the network for the container
    local network_name=$(safename)
    local quadlet_network_unit=dep_$(safename).network
    local quadlet_network_file=$PODMAN_TMPDIR/${quadlet_network_unit}
    cat > $quadlet_network_file <<EOF
[Network]
NetworkName=${network_name}
NetworkDeleteOnStop=true
EOF

    local quadlet_tmpdir=$(mktemp -d --tmpdir=$PODMAN_TMPDIR quadlet.XXXXXX)
    # Have quadlet create the systemd unit file for the network unit
    run_quadlet "$quadlet_network_file" "$quadlet_tmpdir"

    # Save the network service name since the variable will be overwritten
    local network_service=$QUADLET_SERVICE_NAME

    local quadlet_container_file=$PODMAN_TMPDIR/user_$(safename).container
    cat > $quadlet_container_file <<EOF
[Container]
Image=$IMAGE
Exec=top
Network=$quadlet_network_unit
EOF

    run_quadlet "$quadlet_container_file" "$quadlet_tmpdir"

    # Save the container service name for readability
    local container_service=$QUADLET_SERVICE_NAME

    # Network should not exist
    run_podman 1 network exists $network_name

    # Start the container service
    service_setup $container_service

    # Network system unit should be active
    run systemctl show --property=ActiveState "$network_service"
    assert "$output" = "ActiveState=active" \
           "network should be active via dependency"

    # Network should exist
    run_podman network exists $network_name

    # Stop the Network Service
    service_cleanup $network_service inactive

    # Container system unit should be active
    run systemctl show --property=ActiveState "$container_service"
    assert "$output" = "ActiveState=failed" \
           "container service should be failed via dependency"

    # Network should not exist
    run_podman 1 network exists $network_name
}

# A quadlet container depends on a quadlet network
@test "quadlet - network dependency" {
    # Save the unit name to use as the network for the container
    local quadlet_network_unit=dep_$(safename).network
    local quadlet_network_file=$PODMAN_TMPDIR/${quadlet_network_unit}
    cat > $quadlet_network_file <<EOF
[Network]
EOF

    local quadlet_tmpdir=$(mktemp -d --tmpdir=$PODMAN_TMPDIR quadlet.XXXXXX)
    # Have quadlet create the systemd unit file for the network unit
    run_quadlet "$quadlet_network_file" "$quadlet_tmpdir"

    # Save the volume service name since the variable will be overwritten
    local network_service=$QUADLET_SERVICE_NAME
    local network_name=systemd-$(basename $quadlet_network_file .network)

    local quadlet_file=$PODMAN_TMPDIR/user_$(safename).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=top
Network=$quadlet_network_unit
EOF

    run_quadlet "$quadlet_file" "$quadlet_tmpdir"

    # Save the container service name for readability
    local container_service=$QUADLET_SERVICE_NAME

    # Network should not exist
    run_podman 1 network exists $network_name

    service_setup $container_service

    # Network system unit should be active
    run systemctl show --property=ActiveState "$network_service"
    assert "$output" = "ActiveState=active" \
           "network should be active via dependency"

    # Network should exist
    run_podman network exists $network_name

    service_cleanup $QUADLET_SERVICE_NAME failed
    run_podman network rm $network_name
}

# A quadlet container depends on a named quadlet network
@test "quadlet - named network dependency" {
    # Save the unit name to use as the network for the container
    local quadlet_network_unit=dep_$(safename).network
    local quadlet_network_file=$PODMAN_TMPDIR/${quadlet_network_unit}
    cat > $quadlet_network_file <<EOF
[Network]
NetworkName=foo
EOF

    # Have quadlet create the systemd unit file for the network unit
    local quadlet_tmpdir=$(mktemp -d --tmpdir=$PODMAN_TMPDIR quadlet.XXXXXX)
    run_quadlet "$quadlet_network_file" "$quadlet_tmpdir"

    # Save the network service name since the variable will be overwritten
    local network_service=$QUADLET_SERVICE_NAME
    local network_name="foo"

    local quadlet_file=$PODMAN_TMPDIR/user_$(safename).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=top
Network=$quadlet_network_unit
EOF

    run_quadlet "$quadlet_file" "$quadlet_tmpdir"

    # Save the container service name for readability
    local container_service=$QUADLET_SERVICE_NAME
    local container_name=systemd-$(basename $quadlet_file .container)

    # Network should not exist
    run_podman 1 network exists $network_name

    service_setup $container_service

    # Network system unit should be active
    run systemctl show --property=ActiveState "$network_service"
    assert "$output" = "ActiveState=active" "network should be active via dependency"

    # Network should exist
    run_podman network exists $network_name

    # Container should be attached to defined network
    run_podman container inspect --format "{{index .NetworkSettings.Networks \"$network_name\"}}" $container_name
    assert "$output" != "<nil>" "container should be attached to network $network_name"

    service_cleanup $QUADLET_SERVICE_NAME failed
    run_podman network rm $network_name
}

@test "quadlet kube - basic" {
    # Create the YAMl file
    pod_name="p-$(safename)"
    container_name="c-$(safename)"
    yaml_source="$PODMAN_TMPDIR/basic_$(safename).yaml"
    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $pod_name
spec:
  containers:
  - command:
    - "sh"
    args:
    - "-c"
    - "echo STARTED CONTAINER; top -b"
    image: $IMAGE
    name: $container_name
EOF

    # Create the Quadlet file
    local quadlet_file=$PODMAN_TMPDIR/basic_$(safename).kube
    cat > $quadlet_file <<EOF
[Kube]
Yaml=${yaml_source}
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output.
    wait_for_output "STARTED CONTAINER" $pod_name-$container_name

    run_podman container inspect  --format "{{.State.Status}}" $pod_name-$container_name
    is "$output" "running" "container should be started by systemd and hence be running"

    service_cleanup $QUADLET_SERVICE_NAME inactive
}

@test "quadlet kube - named network dependency" {
    # Save the unit name to use as the network for the container
    local quadlet_network_unit=dep_$(safename).network
    local quadlet_network_file=$PODMAN_TMPDIR/${quadlet_network_unit}
    local network_name="n-$(safename)"
    cat > $quadlet_network_file <<EOF
[Network]
NetworkName=${network_name}
EOF

    # Have quadlet create the systemd unit file for the network unit
    local quadlet_tmpdir=$(mktemp -d --tmpdir=$PODMAN_TMPDIR quadlet.XXXXXX)
    run_quadlet "$quadlet_network_file" "$quadlet_tmpdir"

    # Save the network service name since the variable will be overwritten
    local network_service=$QUADLET_SERVICE_NAME

    # Create the YAMl file
    pod_name="p-$(safename)"
    container_name="c-$(safename)"
    yaml_source="$PODMAN_TMPDIR/basic_$(safename).yaml"
    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $pod_name
spec:
  containers:
  - command:
    - "sh"
    args:
    - "-c"
    - "echo STARTED CONTAINER; top -b"
    image: $IMAGE
    name: $container_name
EOF

    # Create the Quadlet file
    local quadlet_file=$PODMAN_TMPDIR/basic_$(safename).kube
    cat > $quadlet_file <<EOF
[Kube]
Yaml=${yaml_source}
Network=$quadlet_network_unit
EOF

    # Network should not exist
    run_podman 1 network exists $network_name

    run_quadlet "$quadlet_file" "$quadlet_tmpdir"
    service_setup $QUADLET_SERVICE_NAME

    # Network system unit should be active
    run systemctl show --property=ActiveState "$network_service"
    assert "$output" = "ActiveState=active" "network should be active via dependency"

    # Network should exist
    run_podman network exists $network_name

    # Ensure we have output.
    wait_for_output "STARTED CONTAINER" $pod_name-$container_name

    run_podman container inspect  --format "{{.State.Status}}" $pod_name-$container_name
    assert "$output" =~ "running" "container should be started by systemd and hence be running"

    # Container should be attached to defined network
    run_podman container inspect --format "{{index .NetworkSettings.Networks \"$network_name\"}}" $pod_name-$container_name
    assert "$output" != "<nil>" "container should be attached to network $network_name"

    service_cleanup $QUADLET_SERVICE_NAME inactive
    run_podman network rm $network_name
}

@test "quadlet - rootfs" {
    skip_if_rootless

    run_podman image mount $IMAGE
    mountpoint="$output"

    rootfs=$PODMAN_TMPDIR/rootfs
    cp -a "$mountpoint" "$rootfs"

    run_podman image unmount $IMAGE

    local quadlet_file=$PODMAN_TMPDIR/basic_$(safename).container
    cat > $quadlet_file <<EOF
[Container]
Rootfs=$rootfs:O
Exec=sh -c "echo STARTED CONTAINER; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; top -b"
Notify=yes
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    wait_for_output "STARTED CONTAINER" $QUADLET_CONTAINER_NAME

    # Done. Clean up.
    service_cleanup $QUADLET_SERVICE_NAME failed
}

@test "quadlet - selinux disable" {
    skip_if_no_selinux
    local quadlet_file=$PODMAN_TMPDIR/basic_$(safename).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
SecurityLabelDisable=true
Exec=sh -c "echo STARTED CONTAINER; top -b"
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output. Output is synced via sd-notify (socat in Exec)
    wait_for_output "STARTED CONTAINER" $QUADLET_CONTAINER_NAME

    run_podman container inspect  --format "{{.ProcessLabel}}" $QUADLET_CONTAINER_NAME
    is "$output" "" "container should be started without specifying a Process Label"

    service_cleanup $QUADLET_SERVICE_NAME failed
}

@test "quadlet - selinux labels" {
    skip_if_no_selinux
    NAME=name$(safename)
    local quadlet_file=$PODMAN_TMPDIR/basic_$(safename).container
    cat > $quadlet_file <<EOF
[Container]
ContainerName=$NAME
Image=$IMAGE
SecurityLabelType=spc_t
SecurityLabelLevel=s0:c100,c200
SecurityLabelFileType=container_ro_file_t
Exec=sh -c "echo STARTED CONTAINER; top -b"
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output. Output is synced via sd-notify (socat in Exec)
    wait_for_output "STARTED CONTAINER" $NAME

    run_podman container ps
    run_podman container inspect  --format "{{.ProcessLabel}}" $NAME
    is "$output" "system_u:system_r:spc_t:s0:c100,c200" "container should be started with correct Process Label"
    run_podman container inspect  --format "{{.MountLabel}}" $NAME
    is "$output" "system_u:object_r:container_ro_file_t:s0:c100,c200" "container should be started with correct Mount Label"

    service_cleanup $QUADLET_SERVICE_NAME failed
}

@test "quadlet - secret as environment variable" {
    create_secret

    local quadlet_file=$PODMAN_TMPDIR/basic_$(safename).container
    cat > $quadlet_file <<EOF
[Container]
ContainerName=$NAME
Image=$IMAGE
Secret=$SECRET_NAME,type=env,target=MYSECRET
Exec=sh -c "echo STARTED CONTAINER; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; top -b"
Notify=yes
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output. Output is synced via sd-notify (socat in Exec)
    wait_for_output "STARTED CONTAINER" $QUADLET_CONTAINER_NAME

    run_podman exec $QUADLET_CONTAINER_NAME /bin/sh -c "printenv MYSECRET"
    is "$output" $SECRET

    service_cleanup $QUADLET_SERVICE_NAME failed
    remove_secret $SECRET_NAME
}

@test "quadlet - secret as a file" {
    create_secret

    local quadlet_file=$PODMAN_TMPDIR/basic_$(safename).container
    cat > $quadlet_file <<EOF
[Container]
ContainerName=$NAME
Image=$IMAGE
Secret=$SECRET_NAME,type=mount,target=/root/secret
Exec=sh -c "echo STARTED CONTAINER; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; top -b"
Notify=yes
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output. Output is synced via sd-notify (socat in Exec)
    wait_for_output "STARTED CONTAINER" $QUADLET_CONTAINER_NAME

    run_podman exec $QUADLET_CONTAINER_NAME /bin/sh -c "cat /root/secret"
    is "$output" $SECRET

    service_cleanup $QUADLET_SERVICE_NAME failed
    remove_secret $SECRET_NAME
}

@test "quadlet - volume path using systemd %T specifier" {
    # "specifier" is systemd-speak for "replaceable fields"; see systemd.unit(5)
    #
    # Step 1: determine what systemd is using for %T. There does not
    # seem to be any systemctly way to find this.
    percent_t_file="${PODMAN_TMPDIR}/foo"
    local service=get-percent-t.$(safename).service
    local unitfile=${UNIT_DIR}/$service
    cat >$unitfile <<EOF
[Unit]
Description=Get the value of percent T

[Service]
ExecStart=/bin/bash -c "echo %T >$percent_t_file"
Type=oneshot
EOF
    systemctl daemon-reload
    systemctl_start --wait $service
    percent_t=$(< $percent_t_file)
    # Clean up. Don't bother to systemctl-reload, service_setup does that below.
    rm -f $unitfile

    # Sanity check: just make sure it's not "/"
    assert "${#percent_t}" -ge 4 "sanity check: length of %T ($percent_t)"

    # Step 2: Make a subdirectory in %T, and in there, a scratch file
    local tmp_path=$(mktemp -d --tmpdir=${percent_t} quadlet.volume.XXXXXX)
    local tmp_subdir=$(basename $tmp_path)
    local file_name="f$(random_string 10).txt"
    local file_content="data_$(random_string 15)"
    echo $file_content > $tmp_path/$file_name

    local quadlet_file=$PODMAN_TMPDIR/basic_$(safename).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Volume=%T/$tmp_subdir:/test_content:Z
Exec=sh -c "echo STARTED CONTAINER; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; top"
Notify=yes
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    run_podman exec $QUADLET_CONTAINER_NAME cat /test_content/$file_name
    is "$output" "$file_content" "contents of testfile in container volume"

    service_cleanup $QUADLET_SERVICE_NAME

    rm -rf $tmp_path
}

@test "quadlet - tmpfs" {
    local quadlet_file=$PODMAN_TMPDIR/basic_$(safename).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=top
Tmpfs=/tmpfs1
Tmpfs=/tmpfs2:ro
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    run_podman container inspect  --format '{{index .HostConfig.Tmpfs "/tmpfs1"}}' $QUADLET_CONTAINER_NAME
    is "$output" "rprivate,nosuid,nodev,tmpcopyup" "regular tmpfs mount"

    run_podman container inspect  --format '{{index .HostConfig.Tmpfs "/tmpfs2"}}' $QUADLET_CONTAINER_NAME
    is "$output" "ro,rprivate,nosuid,nodev,tmpcopyup" "read-only tmpfs mount"

    run_podman container inspect  --format '{{index .HostConfig.Tmpfs "/tmpfs3"}}' $QUADLET_CONTAINER_NAME
    is "$output" "" "nonexistent tmpfs mount"

    service_cleanup $QUADLET_SERVICE_NAME failed
}

@test "quadlet - userns" {
    local quadlet_file=$PODMAN_TMPDIR/basic_$(safename).container
    cat > $quadlet_file <<EOF
[Container]
Image=$IMAGE
Exec=top
UserNS=keep-id:uid=200,gid=210
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    run_podman container inspect --format '{{.Config.CreateCommand}}' $QUADLET_CONTAINER_NAME
    is "${output/* --userns keep-id:uid=200,gid=210 */found}" "found"

    service_cleanup $QUADLET_SERVICE_NAME failed
}

@test "quadlet - exit-code propagation" {
   pod_name="p-$(safename)"
   container_name="c-$(safename)"
   exit_tests="
all  | true  | 0   | inactive
all  | false | 137 | failed
none | false | 0   | inactive
"
   while read exit_code_prop cmd exit_code service_state; do
      local basename=propagate-${exit_code_prop}-${cmd}_$(safename)
      local quadlet_file=$PODMAN_TMPDIR/$basename.kube
      local yaml_file=$PODMAN_TMPDIR/$basename.yaml

      cat > $yaml_file <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $pod_name
spec:
  restartPolicy: Never
  containers:
    - name: $container_name
      image: $IMAGE
      command:
      - $cmd
EOF
       cat > $quadlet_file <<EOF
[Kube]
Yaml=$yaml_file
ExitCodePropagation=$exit_code_prop
LogDriver=journald
EOF

      run_quadlet "$quadlet_file"
      run systemctl status $QUADLET_SERVICE_NAME

      yaml_sha=$(sha256sum $yaml_file)
      service_container="${yaml_sha:0:12}-service"

      service_setup $QUADLET_SERVICE_NAME

      # Ensure we have output. Output is synced via sd-notify (socat in Exec)
      # When running under heavy load (e.g. parallel tests), it
      # may take a little while for service to reach Started
      for tries in $(seq 1 5); do
          run journalctl "--since=$STARTED_TIME" --unit="$QUADLET_SERVICE_NAME"
          if [[ "$output" =~ "Started.*\.service" ]]; then
              break
          fi
          sleep 1
      done
      is "$output" '.*Started.*\.service.*'

      # Opportunistic test: confirm that the Propagation field got set.
      # This is racy, because the container is short-lived and quadlet
      # cleans up on exit (via kube-down in ExecStopPost). So we use '?'
      # and only check output if the inspect succeeds.
      run_podman '?' container inspect --format '{{.KubeExitCodePropagation}}' $service_container
      if [[ $status -eq 0 ]]; then
          is "$output" "$exit_code_prop" \
             "$basename: service container has the expected policy set in its annotations"
      else
          assert "$output" =~ "no such container \"$service_container\"" \
                 "$basename: unexpected error from podman container inspect"
      fi

      # Container must stop of its own accord before we call service_cleanup(),
      # otherwise the 'systemctl stop' there may affect the unit's status.
      # Again, use '?' to handle the abovementioned race condition.
      run_podman '?' wait $service_container
      if [[ $status -eq 0 ]]; then
          assert "$output" = "$exit_code" \
                 "$basename: service container reflects expected exit code"
      else
          assert "$output" =~ "no container with name or ID" \
                 "$basename: unexpected error from podman wait"
      fi

      # This is the actual propagation check
      service_cleanup $QUADLET_SERVICE_NAME $service_state
      run_podman ps -a
      assert "$output" !~ "$(safename)" \
             "all containers are cleaned up even in case of errors"
   done < <(parse_table "$exit_tests")
}

@test "quadlet kube - Working Directory" {
    yaml_source="$PODMAN_TMPDIR/basic_$(safename).yaml"
    local_path=local_path$(random_string)
    pod_name="p-$(safename)"
    container_name="c-$(safename)"

    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $pod_name
spec:
  containers:
  - command:
    - "sh"
    args:
    - "-c"
    - "echo STARTED CONTAINER; top -b"
    image: $IMAGE
    name: $container_name
    volumeMounts:
    - mountPath: /test
      name: test-volume
  volumes:
  - name: test-volume
    hostPath:
      # directory location on host
      path: ./$local_path
      # this field is optional
      type: DirectoryOrCreate
EOF

    # Create the Quadlet file
    local quadlet_file=$PODMAN_TMPDIR/basic_$(safename).kube
    cat > $quadlet_file <<EOF
[Kube]
Yaml=${yaml_source}
SetWorkingDirectory=yaml
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output.
    wait_for_output "STARTED CONTAINER" $pod_name-$container_name

    run_podman container inspect  --format "{{.State.Status}}" $pod_name-$container_name
    is "$output" "running" "container should be started by systemd and hence be running"

    run_podman ps

    run_podman exec $pod_name-$container_name /bin/sh -c "echo hello > /test/test.txt"
    is $(cat $PODMAN_TMPDIR/$local_path/test.txt) "hello"

    service_cleanup $QUADLET_SERVICE_NAME inactive
}

# https://github.com/containers/podman/issues/20667
@test "quadlet kube - start error" {
    local port=$(random_free_port)
    # Create the YAMl file
    pod_name="p-$(safename)"
    container_name="c-$(safename)"
    yaml_source="$PODMAN_TMPDIR/start_err$(safename).yaml"
    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: $pod_name
spec:
  containers:
  - command:
    - "top"
    image: $IMAGE
    name: $container_name
    ports:
    - containerPort: 80
      hostPort: $port
EOF

    # Bind the port to force a an error when starting the pod
    timeout --foreground -v --kill=10 20 socat TCP-LISTEN:$port,bind=127.0.0.1,fork - &
    socat_pid=$!

    # Create the Quadlet file
    local quadlet_file=$PODMAN_TMPDIR/start_err_$(safename).kube
    cat > $quadlet_file <<EOF
[Kube]
Yaml=${yaml_source}
EOF

    run_quadlet "$quadlet_file"

    run -0 systemctl daemon-reload

    echo "$_LOG_PROMPT systemctl start $QUADLET_SERVICE_NAME"
    run systemctl start $QUADLET_SERVICE_NAME
    echo $output
    assert $status -eq 1 "systemctl start should report failure"

    kill "$socat_pid"

    run -0 systemctl show --property=ActiveState $QUADLET_SERVICE_NAME
    assert "$output" == "ActiveState=failed" "unit must be in failed state"

    echo "$_LOG_PROMPT journalctl -u $QUADLET_SERVICE_NAME"
    run -0 journalctl -eu $QUADLET_SERVICE_NAME
    assert "$output" =~ "$port.*ddress already in use" "journal contains the real podman start error"
}

# https://github.com/containers/podman/issues/25786
@test "quadlet kube - pod without containers" {
    local port=$(random_free_port)
    # Create the YAML file
    pod_name="p-$(safename)"
    yaml_source="$PODMAN_TMPDIR/no_cons_$(safename).yaml"
    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: $pod_name
EOF

    # Create the Quadlet file
    local quadlet_file=$PODMAN_TMPDIR/no_cons_$(safename).kube
    cat > $quadlet_file <<EOF
[Kube]
Yaml=${yaml_source}
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    run_podman pod ps --format "{{.Name}}--{{.Status}}"
    assert "$output" =~ "$pod_name--Running" "pod is running"

    service_cleanup $QUADLET_SERVICE_NAME inactive
}

@test "quadlet - image files" {
    local quadlet_tmpdir=$PODMAN_TMPDIR/quadlets

    local registry=localhost:${PODMAN_LOGIN_REGISTRY_PORT}
    local image_for_test=$registry/i-$(safename):$(random_string)
    local authfile=$PODMAN_TMPDIR/authfile.json

    local quadlet_image_unit=image_test_$(safename).image
    local quadlet_image_file=$PODMAN_TMPDIR/$quadlet_image_unit
    cat > $quadlet_image_file <<EOF
[Image]
Image=$image_for_test
AuthFile=$authfile
TLSVerify=false
EOF

    local quadlet_volume_unit=image_test_$(safename).volume
    local quadlet_volume_file=$PODMAN_TMPDIR/$quadlet_volume_unit
    local volume_name=systemd-$(basename $quadlet_volume_file .volume)
    cat > $quadlet_volume_file <<EOF
[Volume]
Driver=image
Image=$quadlet_image_unit
EOF

    local quadlet_container_unit=image_test_$(safename).container
    local quadlet_container_file=$PODMAN_TMPDIR/$quadlet_container_unit
    cat > $quadlet_container_file <<EOF
[Container]
Image=$quadlet_image_unit
Volume=$quadlet_volume_unit:/vol
Exec=sh -c "echo STARTED CONTAINER; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; sleep inf"
EOF

    # In order to test image pull but without possible Network issues,
    # this test uses an additional registry.
    # Start the registry and populate the authfile that we can use for the test.
    start_registry
    run_podman login --authfile=$authfile \
        --tls-verify=false \
        --username ${PODMAN_LOGIN_USER} \
        --password ${PODMAN_LOGIN_PASS} \
        $registry

    # Generate a test image and push it to the registry.
    # For safety in parallel runs, test image must be isolated
    # from $IMAGE. A simple add-tag will not work. (#23756)
    run_podman create -q $IMAGE true
    local tmpcid=$output
    run_podman commit -q $tmpcid $image_for_test
    run_podman rm $tmpcid
    run_podman image push --tls-verify=false --authfile=$authfile $image_for_test

    # Remove the local image to make sure it will be pulled again
    run_podman image rm --ignore $image_for_test

    # Use the same directory for all quadlet files to make sure later steps access previous ones
    mkdir $quadlet_tmpdir

    # Have quadlet create the systemd unit file for the image unit
    run_quadlet "$quadlet_image_file" "$quadlet_tmpdir"
    # Save the image service name since the variable will be overwritten
    local image_service=$QUADLET_SERVICE_NAME

    # Have quadlet create the systemd unit file for the volume unit
    run_quadlet "$quadlet_volume_file" "$quadlet_tmpdir"
    # Save the image service name since the variable will be overwritten
    local volume_service=$QUADLET_SERVICE_NAME

    # Image should not exist
    run_podman 1 image exists ${image_for_test}
    # Volume should not exist
    run_podman 1 volume exists ${volume_name}

    # Have quadlet create the systemd unit file for the image unit
    run_quadlet "$quadlet_container_file" "$quadlet_tmpdir"
    local container_service=$QUADLET_SERVICE_NAME
    local container_name=$QUADLET_CONTAINER_NAME

    service_setup $container_service

    # Image system unit should be active
    run systemctl show --property=ActiveState "$image_service"
    assert "$output" = "ActiveState=active" \
           "quadlet - image files: image should be active via dependency but is not"

    # Volume system unit should be active
    run systemctl show --property=ActiveState "$volume_service"
    assert "$output" = "ActiveState=active" \
           "quadlet - image files: volume should be active via dependency but is not"

    # Image should exist
    run_podman image exists ${image_for_test}

    # Volume should exist
    run_podman volume exists ${volume_name}

    # Verify that the volume was created correctly
    run_podman volume inspect --format "{{ .Driver }}" $volume_name
    assert "$output" = "image" \
           "quadlet - image files: volume driver should be image"

    run_podman volume inspect --format "{{ .Options.image }}" $volume_name
    assert "$output" = "$image_for_test" \
           "quadlet - image files: the image for the volume should be $image_for_test"

    # Verify that the container mounts the volume
    run_podman container inspect --format "{{(index .Mounts 0).Type}}" $container_name
    assert "$output" = "volume" \
           "quadlet - image files: container should be attached to a volume of type volume"

    run_podman container inspect --format "{{(index .Mounts 0).Name}}" $container_name
    assert "$output" = "$volume_name" \
           "quadlet - image files: container should be attached to the volume named $volume_name"

    run_podman exec $QUADLET_CONTAINER_NAME cat /home/podman/testimage-id
    assert "$output" = $PODMAN_TEST_IMAGE_TAG \
           "quadlet - image files: incorrect testimage-id in root"

    run_podman exec $QUADLET_CONTAINER_NAME cat /vol/home/podman/testimage-id
    assert "$output" = $PODMAN_TEST_IMAGE_TAG \
            "quadlet - image files: incorrect testimage-id in bound volume"

    # Shutdown the service and remove the volume
    service_cleanup $container_service failed
    run_podman volume rm $volume_name
    run_podman image rm --ignore $image_for_test
}

@test "quadlet - kube oneshot" {
    local quadlet_tmpdir=$PODMAN_TMPDIR/quadlets
    local test_random_string=r_$(safename)

    local quadlet_kube_volume_name=test-volume_$test_random_string
    local quadlet_kube_volume_yaml_file=$PODMAN_TMPDIR/volume_$test_random_string.yaml
    cat > $quadlet_kube_volume_yaml_file <<EOF
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: $quadlet_kube_volume_name
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
EOF

    local quadlet_kube_volume_unit_file=$PODMAN_TMPDIR/volume_$test_random_string.kube
    cat > $quadlet_kube_volume_unit_file <<EOF
[Kube]
Yaml=$quadlet_kube_volume_yaml_file

[Service]
Type=oneshot
RemainAfterExit=yes
EOF

    local pod_name="p-$(safename)"
    local container_name="c-$(safename)"
    local quadlet_kube_pod_yaml_file=$PODMAN_TMPDIR/pod_$test_random_string.yaml
    cat > $quadlet_kube_pod_yaml_file <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $pod_name
spec:
  containers:
  - command:
    - "sh"
    args:
    - "-c"
    - "echo STARTED CONTAINER; top -b"
    image: $IMAGE
    name: $container_name
    volumeMounts:
    - name: storage
      mountPath: /mnt/storage
  volumes:
  - name: storage
    persistentVolumeClaim:
      claimName: $quadlet_kube_volume_name
EOF

    # Use the same directory for all quadlet files to make sure later steps access previous ones
    mkdir $quadlet_tmpdir

    # Have quadlet create the systemd unit file for the kube based volume unit
    run_quadlet "$quadlet_kube_volume_unit_file" "$quadlet_tmpdir"
    # Save the volume service name since the variable will be overwritten
    local volume_service=$QUADLET_SERVICE_NAME

    # Volume should not exist
    run_podman 1 volume exists ${quadlet_kube_volume_name}

    local quadlet_kube_pod_unit_file=$PODMAN_TMPDIR/pod_$test_random_string.kube
    cat > $quadlet_kube_pod_unit_file <<EOF
[Kube]
Yaml=$quadlet_kube_pod_yaml_file

[Unit]
Requires=$volume_service
After=$volume_service
EOF

    # Have quadlet create the systemd unit file for the pod unit
    run_quadlet "$quadlet_kube_pod_unit_file" "$quadlet_tmpdir"
    local pod_service=$QUADLET_SERVICE_NAME

    service_setup $pod_service

    # Volume system unit should be active
    run systemctl show --property=ActiveState "$volume_service"
    assert "$output" = "ActiveState=active" \
           "quadlet - kube oneshot: volume should be active via dependency but is not"

    # Volume should exist
    run_podman volume exists ${quadlet_kube_volume_name}

    run_podman container inspect --format "{{(index .Mounts 0).Type}}" $pod_name-$container_name
    assert "$output" = "volume" \
           "quadlet - kube oneshot: volume .Type"

    run_podman container inspect --format "{{(index .Mounts 0).Name}}" $pod_name-$container_name
    assert "$output" = "$quadlet_kube_volume_name" \
           "quadlet - kube oneshot: volume .Name"

    # Shutdown the service and remove the volume
    service_cleanup $pod_service inactive
    run_podman volume rm $quadlet_kube_volume_name
}

@test "quadlet - kube down force" {
    local test_random_string=$(random_string)

    local quadlet_kube_volume_name=test-volume_$test_random_string
    local pod_name="p-$(safename)"
    local container_name="c-$(safename)"
    local quadlet_kube_pod_yaml_file=$PODMAN_TMPDIR/pod_$test_random_string.yaml
    cat > $quadlet_kube_pod_yaml_file <<EOF
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: $quadlet_kube_volume_name
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $pod_name
spec:
  containers:
  - command:
    - "sh"
    args:
    - "-c"
    - "echo STARTED CONTAINER; top -b"
    image: $IMAGE
    name: $container_name
    volumeMounts:
    - name: storage
      mountPath: /mnt/storage
  volumes:
  - name: storage
    persistentVolumeClaim:
      claimName: $quadlet_kube_volume_name
EOF

    local quadlet_kube_pod_unit_file=$PODMAN_TMPDIR/pod_$test_random_string.kube
    cat > $quadlet_kube_pod_unit_file <<EOF
[Kube]
Yaml=$quadlet_kube_pod_yaml_file
KubeDownForce=true
EOF

    # Have quadlet create the systemd unit file for the pod unit
    run_quadlet "$quadlet_kube_pod_unit_file" "$quadlet_tmpdir"
    local pod_service=$QUADLET_SERVICE_NAME

    # Volume should not exist
    run_podman 1 volume exists ${quadlet_kube_volume_name}

    service_setup $pod_service

    # Volume should exist
    run_podman volume exists ${quadlet_kube_volume_name}

    run_podman container inspect --format "{{(index .Mounts 0).Type}}" $pod_name-$container_name
    assert "$output" = "volume" \
           "quadlet - kube oneshot: volume .Type"

    run_podman container inspect --format "{{(index .Mounts 0).Name}}" $pod_name-$container_name
    assert "$output" = "$quadlet_kube_volume_name" \
           "quadlet - kube oneshot: volume .Name"

    # Shutdown the service
    service_cleanup $pod_service failed

    # Volume should not exist
    run_podman 1 volume exists ${quadlet_kube_volume_name}
}

@test "quadlet - image tag" {
    local quadlet_tmpdir=$PODMAN_TMPDIR/quadlets
    local archive_file=$PODMAN_TMPDIR/archive-file.tar
    local image_for_test=localhost/i-$(safename):$(random_string)

    local quadlet_image_unit=image_test_$(safename).image
    local quadlet_image_file=$PODMAN_TMPDIR/$quadlet_image_unit
    cat > $quadlet_image_file <<EOF
[Image]
Image=docker-archive:$archive_file
ImageTag=$image_for_test
EOF

    local quadlet_volume_unit=image_test_$(safename).volume
    local quadlet_volume_file=$PODMAN_TMPDIR/$quadlet_volume_unit
    local volume_name=systemd-$(basename $quadlet_volume_file .volume)
    cat > $quadlet_volume_file <<EOF
[Volume]
Driver=image
Image=$quadlet_image_unit
EOF

    local quadlet_container_unit=image_test_$(safename).container
    local quadlet_container_file=$PODMAN_TMPDIR/$quadlet_container_unit
    cat > $quadlet_container_file <<EOF
[Container]
Image=$IMAGE
Exec=sh -c "echo STARTED CONTAINER; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; sleep inf"
Volume=$quadlet_volume_unit:/vol
EOF

    # Generate a test image, save it into a file, and remove it.
    # For safety in parallel runs, test image must be isolated
    # from $IMAGE. A simple add-tag will not work. (#23756)
    run_podman create -q $IMAGE true
    local tmpcid=$output
    run_podman commit -q $tmpcid $image_for_test
    run_podman rm $tmpcid
    run_podman image save --format docker-archive --output $archive_file $image_for_test
    run_podman image rm $image_for_test

    # Use the same directory for all quadlet files to make sure later steps access previous ones
    mkdir $quadlet_tmpdir

    # Have quadlet create the systemd unit file for the image unit
    run_quadlet "$quadlet_image_file" "$quadlet_tmpdir"
    # Save the image service name since the variable will be overwritten
    local image_service=$QUADLET_SERVICE_NAME

    # Have quadlet create the systemd unit file for the volume unit
    run_quadlet "$quadlet_volume_file" "$quadlet_tmpdir"
    # Save the image service name since the variable will be overwritten
    local volume_service=$QUADLET_SERVICE_NAME

    # Image should not exist
    run_podman 1 image exists ${image_for_test}
    # Volume should not exist
    run_podman 1 volume exists ${volume_name}

    # Have quadlet create the systemd unit file for the image unit
    run_quadlet "$quadlet_container_file" "$quadlet_tmpdir"
    local container_service=$QUADLET_SERVICE_NAME
    local container_name=$QUADLET_CONTAINER_NAME

    service_setup $container_service

    # Image system unit should be active
    run systemctl show --property=ActiveState "$image_service"
    assert "$output" = "ActiveState=active" \
           "quadlet - image tag: image service ActiveState"

    # Volume system unit should be active
    run systemctl show --property=ActiveState "$volume_service"
    assert "$output" = "ActiveState=active" \
           "quadlet - image tag: volume service ActiveState"

    # Image should exist
    run_podman image exists ${image_for_test}

    # Volume should exist
    run_podman volume exists ${volume_name}

    run_podman exec $QUADLET_CONTAINER_NAME cat /vol/home/podman/testimage-id
    assert "$output" = $PODMAN_TEST_IMAGE_TAG \
            "quadlet - image files: incorrect testimage-id in bound volume"

    # Shutdown the service and remove the image
    service_cleanup $container_service failed
    run_podman volume rm $volume_name
    run_podman image rm --ignore $image_for_test
}

@test "quadlet - pod simple" {
    local quadlet_tmpdir=$PODMAN_TMPDIR/quadlets

    local test_pod_name=pod_test_$(safename)
    local quadlet_pod_unit=$test_pod_name.pod
    local quadlet_pod_file=$PODMAN_TMPDIR/$quadlet_pod_unit
    cat > $quadlet_pod_file <<EOF
[Pod]
PodName=$test_pod_name
EOF

    local quadlet_container_unit=pod_test_$(safename).container
    local quadlet_container_file=$PODMAN_TMPDIR/$quadlet_container_unit
    cat > $quadlet_container_file <<EOF
[Container]
Image=$IMAGE
# Note it is important that the trap is before the ready message,
# otherwise the signal handler may not registered in time before we do systemctl stop.
Exec=sh -c "echo STARTED CONTAINER; trap 'exit' SIGTERM; echo "READY=1" | socat -u STDIN unix-sendto:\$NOTIFY_SOCKET; while :; do sleep 0.1; done"
Pod=$quadlet_pod_unit
Notify=yes
EOF

    # Use the same directory for all quadlet files to make sure later steps access previous ones
    mkdir $quadlet_tmpdir

    # Have quadlet create the systemd unit file for the pod unit
    run_quadlet "$quadlet_pod_file" "$quadlet_tmpdir"
    # Save the pod service name since the variable will be overwritten
    local pod_service=$QUADLET_SERVICE_NAME

    # Have quadlet create the systemd unit file for the container unit
    run_quadlet "$quadlet_container_file" "$quadlet_tmpdir"
    local container_service=$QUADLET_SERVICE_NAME
    local container_name=$QUADLET_CONTAINER_NAME

    # Start the pod service
    service_setup $pod_service

    # Pod should exist
    run_podman pod exists ${test_pod_name}

    # Pod exit policy should be stop by default
    run_podman pod inspect --format "{{.ExitPolicy}}" ${test_pod_name}
    assert "$output" = "stop" \
            "quadlet - pod: default ExitPolicy should be stop"

    # Wait for systemd to activate the container service
    wait_for_command_output "systemctl show --property=ActiveState $container_service" "ActiveState=active"

    # Container should exist
    run_podman container exists ${container_name}

    # Shutdown the service
    service_cleanup $pod_service inactive

    # It might take a few seconds to go inactive, esp. under heavy load
    for tries in $(seq 1 5); do
        run systemctl show --property=ActiveState "$container_service"
        if [[ "$output" = "ActiveState=inactive" ]]; then
            break
        fi
        sleep 1
    done
    assert "ActiveState=inactive" \
           "quadlet - pod base: container service ActiveState"

    # Container should not exist
    run_podman 1 container exists ${container_name}
}

# This test reproduces https://github.com/containers/podman/issues/20432
# In order to reproduce the issue, the image in the FROM must no be available locally
# and must not have a tag. The first forces Pull and the second the resolution where the crash occurs
# Using a local registry does not work since kube play does not pass the autofile and tls-verify flags to the build
@test "quadlet - kube build from unavailable image with no tag" {
    local quadlet_tmpdir=$PODMAN_TMPDIR/quadlets

    # FIXME: how to make this parallel-safe? Can we?
    local untagged_image=quay.io/libpod/busybox
    local built_image="built-$(safename)"
    local yaml_dir=$quadlet_tmpdir/$built_image
    local build_dir=$yaml_dir/$built_image

    # Use the same directory for all quadlet files to make sure later steps access previous ones
    mkdir -p $build_dir

    container_file_path=$build_dir/Containerfile
    cat >$container_file_path << EOF
FROM $untagged_image
EOF

    # Create the YAMl file
    pod_name="p-$(safename)"
    container_name="c-$(safename)"
    yaml_source="$yaml_dir/build_$(safename).yaml"
    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $pod_name
spec:
  containers:
  - command:
    - "sh"
    args:
    - "-c"
    - "echo STARTED CONTAINER; sleep inf"
    image: $built_image
    name: $container_name
EOF

    # Create the Quadlet file
    local quadlet_file=$quadlet_tmpdir/build_$(safename).kube
    cat > $quadlet_file <<EOF
[Kube]
Yaml=${yaml_source}
PodmanArgs=--build
SetWorkingDirectory=yaml
EOF

    # Make sure the tagged image is not locally available
    run_podman rmi -i $untagged_image:latest

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME

    # Ensure we have output.
    wait_for_output "STARTED CONTAINER" $pod_name-$container_name

    run_podman container inspect  --format "{{.State.Status}}" $pod_name-$container_name
    is "$output" "running" "container should be started by systemd and hence be running"

    service_cleanup $QUADLET_SERVICE_NAME inactive
    run_podman rmi $untagged_image:latest $built_image
}

@test "quadlet - drop-in files" {
    local quadlet_tmpdir="${PODMAN_TMPDIR}/dropins"

    local quadlet_file="truncated-$(safename).container"

    local -A dropin_dirs=(
        [toplevel]=container.d
        [truncated]=truncated-.container.d
        [quadlet]="${quadlet_file}.d"
    )

    # Table of drop-in .conf files. Format is:
    #
    #    apply | dir | parent dir (optional) | filename | [Section] | Content=...
    #
    # The parent dir used as regression test for https://github.com/containers/podman/issues/26555
    local dropin_files="
y | toplevel  |        | 10 | [Unit]      | Description=Test File for Dropin Configuration
n | toplevel  |        | 99 | [Install]   | WantedBy=default.target
y | truncated |        | 50 | [Container] | ContainerName=truncated-dropins
n | truncated |        | 99 | [Service]   | Restart=always
n | truncated |        | 99 | [Install]   | WantedBy=multiuser.target
y | quadlet   |        | 99 | [Service]   | RestartSec=60s
n | toplevel  |        | 30 | [Service]   | Environment=test=wrong
y | truncated | subdir | 30 | [Service]   | Environment=test=right
"

    # Pass 1: Create all drop-in directories and files
    while read apply dir parent file section content; do
        # By default parent will be '' when left empty in the table and not actually an empty string,
        # make sure we set it to an empty string instead.
        if [[ "$parent" == "''" ]]; then
            parent=""
        fi
        # Parent can be empty which is fine, dir//file is the same as dir/file and what we want here.
        local d="${quadlet_tmpdir}/$parent/${dropin_dirs[${dir}]}"
        mkdir -p "${d}"

        local f="${d}/${file}.conf"
        echo "${section}" >>"${f}"
        echo "${content}" >>"${f}"
    done < <(parse_table "${dropin_files}")

    # Create the base quadlet file
    quadlet_base="${PODMAN_TMPDIR}/${quadlet_file}"
    cat > "${quadlet_base}" <<EOF
[Container]
Image="${IMAGE}"
EOF

    # Generate the quadlet file from the base file and any drop-in .conf files.
    run_quadlet "${quadlet_base}" "${quadlet_tmpdir}"

    # Pass 2: test whether the expected .conf files are applied
    # and the overridden .conf files are not.
    while read apply dir parent file section content; do
        if [[ "$parent" == "''" ]]; then
            parent=""
        fi
        if [[ "${apply}" = "y" ]]; then
            assert "${QUADLET_SERVICE_CONTENT}" =~ "${content}" "Set in ${parent}/${dir}/${file}.conf"
        else
            assert "${QUADLET_SERVICE_CONTENT}" !~ "${content}" "Set in ${parent}/${dir}/${file}.conf but should have been overridden"
        fi
    done < <(parse_table "${dropin_files}")
}

@test "quadlet - artifact" {
    local quadlet_tmpdir=$PODMAN_TMPDIR/quadlets

    local registry=localhost:${PODMAN_LOGIN_REGISTRY_PORT}
    local artifact_for_test=$registry/test-artifact:$(random_string)
    local authfile=$PODMAN_TMPDIR/authfile.json

    # Create a test artifact file
    local test_artifact_dir=$PODMAN_TMPDIR/test-artifact
    mkdir -p $test_artifact_dir
    echo "test artifact content $(random_string)" > $test_artifact_dir/test-file.txt

    # In order to test artifact pull but without possible Network issues,
    # this test uses an additional registry.
    # Start the registry and populate the authfile that we can use for the test.
    start_registry
    run_podman login --authfile=$authfile \
        --tls-verify=false \
        --username ${PODMAN_LOGIN_USER} \
        --password ${PODMAN_LOGIN_PASS} \
        $registry

    # Create and push a test artifact to the registry
    run_podman artifact add $artifact_for_test $test_artifact_dir/test-file.txt
    run_podman artifact push --tls-verify=false --authfile=$authfile $artifact_for_test

    # Remove the local artifact to make sure it will be pulled again
    run_podman artifact rm $artifact_for_test

    # Create artifact quadlet file
    local artifact_file=$PODMAN_TMPDIR/test-artifact.artifact
    cat >$artifact_file << EOF
[Artifact]
Artifact=$artifact_for_test
AuthFile=$authfile
TLSVerify=false
EOF

    run_quadlet "$artifact_file"
    service_setup $QUADLET_SERVICE_NAME

    # Wait for the service to complete (it's a oneshot service)
    local timeout=30
    local count=0
    while [ $count -lt $timeout ]; do
        run systemctl show --value --property=ActiveState "$QUADLET_SERVICE_NAME"
        if [ "$output" = "active" ]; then
            break
        fi
        sleep 1
        count=$((count + 1))
    done

    # Verify artifact was pulled
    run_podman artifact ls
    assert $status -eq 0 "Failed to list artifacts"
    # Extract the repository and tag parts separately since artifact ls shows them in columns
    local artifact_repo=$(echo "$artifact_for_test" | cut -d: -f1)
    local artifact_tag=$(echo "$artifact_for_test" | cut -d: -f2)
    assert "$output" =~ "$artifact_repo.*$artifact_tag" "Artifact should exist after quadlet service runs"

    # Clean up
    run_podman artifact rm $artifact_for_test
}

# Following issue: https://github.com/containers/podman/issues/24599
# Make sure future changes do not break
@test "quadlet - build with pull" {
    local quadlet_tmpdir=$PODMAN_TMPDIR/quadlets

    mkdir $quadlet_tmpdir

    local container_file_path=$quadlet_tmpdir/Containerfile
    cat >$container_file_path << EOF
FROM $IMAGE
EOF

    local image_tag=quay.io/i-$(safename):$(random_string)
    local quadlet_file=$PODMAN_TMPDIR/pull_$(safename).build
    cat >$quadlet_file << EOF
[Build]
ImageTag=$image_tag
File=$container_file_path
Pull=never
EOF

    run_quadlet "$quadlet_file"
    service_setup $QUADLET_SERVICE_NAME "wait"

    run_podman rmi -i $image_tag
}
# vim: filetype=sh
