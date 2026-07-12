#!/usr/bin/env bats   -*- bats -*-
#
# Tests for automatically update images for containerized services
#

load helpers
load helpers.network
load helpers.registry
load helpers.systemd

export SNAME_FILE

function setup() {
    skip_if_remote "systemd tests are meaningless over remote"
    basic_setup

    SNAME_FILE=${PODMAN_TMPDIR}/services
}

function teardown() {
    if [[ -e $SNAME_FILE ]]; then
        while read line; do
            if [[ "$line" =~ "podman-auto-update" ]]; then
                echo "Stop timer: $line.timer"
                systemctl stop $line.timer
                systemctl disable $line.timer
            else
                systemctl stop $line
            fi
            rm -f $UNIT_DIR/$line.{service,timer}
        done < $SNAME_FILE

        rm -f $SNAME_FILE
    fi
    SNAME_FILE=

    run_podman rmi -f                              \
            quay.io/libpod/alpine:latest           \
            quay.io/libpod/busybox:latest          \
            quay.io/libpod/localtest:latest        \
            quay.io/libpod/autoupdatebroken:latest \
            quay.io/libpod/test:latest

    # The rollback tests may leave some dangling images behind, so let's prune
    # them to leave a clean state.
    run_podman image prune -f
    basic_teardown
}

# This functions is used for handle the basic step in auto-update related
# tests. Including following steps:
#   1. Generate a random container name and echo it to output.
#   2. Tag the fake image before test
#   3. Start a container with io.containers.autoupdate
#   4. Generate the service file from the container
#   5. Remove the origin container
#   6. Start the container from service
#   7. Use this fully-qualified image instead of 2)
function generate_service() {
    local target_img_basename=$1
    local autoupdate=$2
    local command=$3
    local extraArgs=$4
    local noTag=$5
    local requires=$6

    # Unless specified, set a default command.
    if [[ -z "$command" ]]; then
        command="top -d 120"
    fi

    # Container name. Include the autoupdate type, to make debugging easier.
    # IMPORTANT: variable 'cname' is passed (out of scope) up to caller!
    cname=c_${autoupdate//\'/}_$(random_string)
    target_img="quay.io/libpod/$target_img_basename:latest"
    if [[ -n "$7" ]]; then
        target_img="$7"
    fi

    if [[ -z "$noTag" ]]; then
        run_podman tag $IMAGE $target_img
    fi

    if [[ -n "$autoupdate" ]]; then
        label="--label io.containers.autoupdate=$autoupdate"
    else
        label=""
    fi

    if [[ -n "$requires" ]]; then
        requires="--requires=$requires"
    fi

    run_podman create $extraArgs --name $cname $label $target_img $command

    (cd $UNIT_DIR; run_podman generate systemd --new --files --name $requires $cname)
    echo "container-$cname" >> $SNAME_FILE
    run_podman rm -t 0 -f $cname

    systemctl daemon-reload
    systemctl_start container-$cname
    systemctl status container-$cname

    # Original image ID.
    # IMPORTANT: variable 'ori_image' is passed (out of scope) up to caller!
    run_podman inspect --format "{{.Image}}" $cname
    ori_image=$output
}

function _wait_service_ready() {
    local sname=$1

    local timeout=6
    while [[ $timeout -gt 1 ]]; do
        if systemctl -q is-active $sname; then
            return
        fi
        sleep 1
        let timeout=$timeout-1
    done

    # Print service status as debug information before failed the case
    systemctl status $sname
    die "Timed out waiting for $sname to start"
}

# Wait for container to update, as confirmed by its image ID changing
function _confirm_update() {
    local cname=$1
    local old_iid=$2

    # Image has already been pulled, so this shouldn't take too long
    local timeout=10
    while [[ $timeout -gt 0 ]]; do
        run_podman '?' inspect --format "{{.Image}}" $cname
        if [[ $status != 0 ]]; then
            if [[ $output =~ (no such object|does not exist in database): ]]; then
                # this is ok, it just means the container is being restarted
                :
            else
                die "podman inspect $cname failed unexpectedly"
            fi
        elif [[ $output != $old_iid ]]; then
            return
        fi
        sleep 1
        timeout=$((timeout - 1))
    done

    die "Timed out waiting for $cname to update; old IID=$old_iid"
}

@test "podman auto-update - validate input" {
    # Fully-qualified image reference is required
    run_podman create --label io.containers.autoupdate=registry $IMAGE
    run_podman rm -f "$output"

    # Short name does not work
    shortname="shortname:latest"
    run_podman image tag $IMAGE $shortname
    run_podman 125 create --label io.containers.autoupdate=registry $shortname
    is "$output" "Error: short name: auto updates require fully-qualified image reference: \"$shortname\""

    # Requires docker (or no) transport
    archive=$PODMAN_TMPDIR/archive.tar
    run_podman save -o $archive $IMAGE
    run_podman 125 create --label io.containers.autoupdate=registry docker-archive:$archive
    is "$output" ".*Error: auto updates require the docker image transport but image is of transport \"docker-archive\""

    run_podman rmi $shortname
}

# This test can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
@test "podman auto-update - label io.containers.autoupdate=image" {
    since=$(date --iso-8601=seconds)
    run_podman auto-update
    is "$output" ""
    run_podman events --filter type=system --since $since --stream=false
    is "$output" ""

    # Generate two units.  The first "parent" to be auto updated, the second
    # "child" depends on/requires the "parent" and is expected to get restarted
    # as well on auto updates (regression test for #18926).
    generate_service alpine image
    ctr_parent=$cname
    _wait_service_ready container-$ctr_parent.service

    generate_service alpine image "" "" "" "container-$ctr_parent.service"
    ctr_child=$cname
    _wait_service_ready container-$ctr_child.service
    run_podman container inspect --format "{{.ID}}" $ctr_child
    old_child_id=$output

    since=$(date --iso-8601=seconds)
    run_podman auto-update --dry-run --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*container-$ctr_parent.service,quay.io/libpod/alpine:latest,pending,registry.*" "Image update is pending."
    run_podman events --filter type=system --since $since --stream=false
    is "$output" ".* system auto-update"

    since=$(date --iso-8601=seconds)
    run_podman auto-update --rollback=false --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" "Trying to pull.*" "Image is updated."
    is "$output" ".*container-$ctr_parent.service,quay.io/libpod/alpine:latest,true,registry.*" "Image is updated."
    run_podman events --filter type=system --since $since --stream=false
    is "$output" ".* system auto-update"

    # Confirm that the update was successful and that the child container/unit
    # has been restarted as well.
    _confirm_update $ctr_parent $ori_image
    run_podman container inspect --format "{{.ID}}" $ctr_child
    assert "$output" != "$old_child_id" \
        "child container/unit has not been restarted during update"
    run_podman container inspect --format "{{.ID}}" $ctr_child
    run_podman container inspect --format "{{.State.Status}}" $ctr_child
    is "$output" "running" "child container is in running state"
}

@test "podman auto-update - label io.containers.autoupdate=image with rollback" {
    # FIXME: this test should exercise the authfile label to have a regression
    # test for #11171.

    # Note: the autoupdatebroken image is empty on purpose so it cannot be
    # executed and force a rollback.  The rollback test for the local policy
    # is exercising the case where the container doesn't send a ready message.
    image=quay.io/libpod/autoupdatebroken

    run_podman tag $IMAGE $image
    generate_service autoupdatebroken image

    _wait_service_ready container-$cname.service
    run_podman auto-update --dry-run --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*container-$cname.service,$image:latest,pending,registry.*" "Image update is pending."

    run_podman container inspect --format "{{.Image}}" $cname
    oldID="$output"

    run_podman inspect --format "{{.ID}}" $cname
    containerID="$output"

    run_podman auto-update --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" "Trying to pull.*" "Image is updated."
    is "$output" ".*container-$cname.service,$image:latest,rolled back,registry.*" "Image has been rolled back."

    run_podman container inspect --format "{{.Image}}" $cname
    is "$output" "$oldID" "container rolled back to previous image"

    run_podman container inspect --format "{{.ID}}" $cname
    assert "$output" != "$containerID" \
           "container has not been restarted during rollback"
}

@test "podman auto-update - label io.containers.autoupdate=disabled" {
    generate_service alpine disabled

    _wait_service_ready container-$cname.service
    run_podman auto-update
    is "$output" "" "Image is not updated when autoupdate=disabled."

    run_podman inspect --format "{{.Image}}" $cname
    is "$output" "$ori_image" "Image ID should not change"
}

@test "podman auto-update - label io.containers.autoupdate=fakevalue" {
    fakevalue=fake_$(random_string)
    generate_service alpine $fakevalue

    _wait_service_ready container-$cname.service
    run_podman 125 auto-update
    assert "$output" =~ 'auto-updating container "[0-9a-f]{64}": invalid auto-update policy "'"$fakevalue"'": valid policies are \["disabled" "image" "local" "registry"\]' "invalid policy setup"

    run_podman inspect --format "{{.Image}}" $cname
    is "$output" "$ori_image" "Image ID should not change"
}

@test "podman auto-update - label io.containers.autoupdate=local" {
    generate_service localtest local
    _wait_service_ready container-$cname.service

    image=quay.io/libpod/localtest:latest
    run_podman commit --change CMD=/bin/bash $cname $image
    run_podman image inspect --format "{{.ID}}" $image

    run_podman auto-update --dry-run --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*container-$cname.service,quay.io/libpod/localtest:latest,pending,local.*" "Image update is pending."

    run_podman auto-update --rollback=false --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*container-$cname.service,quay.io/libpod/localtest:latest,true,local.*" "Image is updated."

    _confirm_update $cname $ori_image
}

# This test can fail in dev. environment because of SELinux.
# quick fix: chcon -t container_runtime_exec_t ./bin/podman
@test "podman auto-update - label io.containers.autoupdate=local with rollback" {
    # sdnotify fails with runc 1.0.0-3-dev2 on Ubuntu. Let's just
    # assume that we work only with crun, nothing else.
    # [copied from 260-sdnotify.bats]
    runtime=$(podman_runtime)
    if [[ "$runtime" != "crun" ]]; then
        skip "this test only works with crun, not $runtime"
    fi

    _prefetch $SYSTEMD_IMAGE

    dockerfile1=$PODMAN_TMPDIR/Dockerfile.1
    cat >$dockerfile1 <<EOF
FROM $SYSTEMD_IMAGE
RUN echo -e "#!/bin/sh\n\
printenv NOTIFY_SOCKET; echo READY; systemd-notify --ready;\n\
trap 'echo Received SIGTERM, finishing; exit' SIGTERM; echo WAITING; while :; do sleep 0.1; done" \
>> /runme
RUN chmod +x /runme
EOF

    dockerfile2=$PODMAN_TMPDIR/Dockerfile.2
    cat >$dockerfile2 <<EOF
FROM $SYSTEMD_IMAGE
RUN echo -e "#!/bin/sh\n\
exit 1" >> /runme
RUN chmod +x /runme
EOF
    image=test

    # Generate a healthy image that will run correctly.
    run_podman build -t quay.io/libpod/$image -f $dockerfile1

    generate_service $image local /runme --sdnotify=container noTag
    _wait_service_ready container-$cname.service

    run_podman auto-update --dry-run --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*container-$cname.service,quay.io/libpod/$image:latest,false,local.*" "No update available"

    # Generate an unhealthy image that will fail.
    run_podman build -t quay.io/libpod/$image -f $dockerfile2
    run_podman image inspect --format "{{.ID}}" $image
    newID="$output"

    run_podman auto-update --dry-run --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*container-$cname.service,quay.io/libpod/$image:latest,pending,local.*" "Image updated is pending"

    # Note: we rollback automatically by default.
    run_podman auto-update --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*container-$cname.service,quay.io/libpod/$image:latest,rolled back,local.*" "Rolled back to old image"

    # Make sure that new container is not using the new image ID anymore.
    _confirm_update $cname $newID
}

@test "podman auto-update with multiple services" {
    # Preserve original image ID, to confirm that it changes (or not)
    run_podman inspect --format "{{.Id}}" $IMAGE
    local img_id="$output"

    local cnames=()
    local -A expect_update
    local -A will_update=([image]=1 [registry]=1 [local]=1)

    local fakevalue=fake_$(random_string)
    for auto_update in image registry "" disabled "''" $fakevalue local
    do
        local img_base="alpine"
        if [[ $auto_update == "registry" ]]; then
            img_base="busybox"
        elif [[ $auto_update == "local" ]]; then
            img_base="localtest"
        fi
        generate_service $img_base $auto_update
        cnames+=($cname)
        if [[ $auto_update == "local" ]]; then
            local_cname=$cname
        fi

        if [[ -n "$auto_update" && -n "${will_update[$auto_update]}" ]]; then
            expect_update[$cname]=1
        fi
    done

    # Make sure all services are ready.
    for cname in "${cnames[@]}"; do
        _wait_service_ready container-$cname.service
    done
    run_podman commit --change CMD=/bin/bash $local_cname quay.io/libpod/localtest:latest
    # Exit code is expected, due to invalid 'fakevalue'
    run_podman 125 auto-update --rollback=false
    update_log=$output
    assert "$update_log" =~ '.*Error: auto-updating container "[0-9a-f]{64}": invalid auto-update policy.*' "invalid policy setup"

    local n_updated=$(grep -c 'Trying to pull' <<<"$update_log")
    is "$n_updated" "2" "Number of images updated from registry."

    for cname in "${!expect_update[@]}"; do
        is "$update_log" ".*$cname.*" "container with auto-update policy image updated"
        # Just because podman says it fetched, doesn't mean it actually updated
        _confirm_update $cname $img_id
    done

    # Final confirmation that all image IDs have/haven't changed
    for cname in "${cnames[@]}"; do
        run_podman inspect --format "{{.Image}}" $cname
        if [[ -n "${expect_update[$cname]}" ]]; then
            assert "$output" != "$img_id" "$cname: image ID did not change"
        else
            assert "$output" = "$img_id" "Image ID should not be changed."
        fi
    done
}

@test "podman auto-update using systemd" {
    skip_if_journald_unavailable

    generate_service alpine image

    cat >$UNIT_DIR/podman-auto-update-$cname.timer <<EOF
[Unit]
Description=Podman auto-update testing timer

[Timer]
OnActiveSec=0s
Persistent=true

[Install]
WantedBy=timers.target
EOF
    cat >$UNIT_DIR/podman-auto-update-$cname.service <<EOF
[Unit]
Description=Podman auto-update testing service
Documentation=man:podman-auto-update(1)
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
ExecStart=$PODMAN auto-update
Environment="http_proxy=${http_proxy}"
Environment="HTTP_PROXY=${HTTP_PROXY}"
Environment="https_proxy=${https_proxy}"
Environment="HTTPS_PROXY=${HTTPS_PROXY}"
Environment="no_proxy=${no_proxy}"
Environment="NO_PROXY=${NO_PROXY}"

[Install]
WantedBy=default.target
EOF

    echo "podman-auto-update-$cname" >> $SNAME_FILE
    systemctl enable --now podman-auto-update-$cname.timer
    systemctl list-timers --all

    # systemd       <245 displays 'Started Podman auto-update ...'
    # systemd 245 - <250 displays 'Finished Podman auto-update ...'
    # systemd 250 - ???? displays 'Finished <unit name> - Podman auto-...'
    local expect='(Started|Finished.*) Podman auto-update testing service'
    local failed_start=failed
    local count=0
    while [ $count -lt 120 ]; do
        run journalctl -n 15 -u podman-auto-update-$cname.service
        if [[ "$output" =~ $expect ]]; then
            failed_start=
            break
        fi
        ((count+=1))
        sleep 1
    done

    if [[ -n "$failed_start" ]]; then
        echo "journalctl output:"
        sed -e 's/^/  /' <<<"$output"
        die "Did not find expected string '$expect' in journalctl output for $cname"
    fi

    _confirm_update $cname $ori_image
}

@test "podman-kube@.service template with rollback" {
    # sdnotify fails with runc 1.0.0-3-dev2 on Ubuntu. Let's just
    # assume that we work only with crun, nothing else.
    # [copied from 260-sdnotify.bats]
    runtime=$(podman_runtime)
    if [[ "$runtime" != "crun" ]]; then
        skip "this test only works with crun, not $runtime"
    fi

    _prefetch $SYSTEMD_IMAGE
    install_kube_template

    dockerfile1=$PODMAN_TMPDIR/Dockerfile.1
    cat >$dockerfile1 <<EOF
FROM $SYSTEMD_IMAGE
RUN echo -e "#!/bin/sh\n\
printenv NOTIFY_SOCKET; echo READY; systemd-notify --ready;\n\
trap 'echo Received SIGTERM, finishing; exit' SIGTERM; echo WAITING; while :; do sleep 0.1; done" \
>> /runme
RUN chmod +x /runme
EOF

    dockerfile2=$PODMAN_TMPDIR/Dockerfile.2
    cat >$dockerfile2 <<EOF
FROM $SYSTEMD_IMAGE
RUN echo -e "#!/bin/sh\n\
exit 1" >> /runme
RUN chmod +x /runme
EOF
    local_image=localhost/image:$(random_string 10)

    # Generate a healthy image that will run correctly.
    run_podman build -t $local_image -f $dockerfile1
    run_podman image inspect --format "{{.ID}}" $local_image
    oldID="$output"

    # Create the YAMl file
    yaml_source="$PODMAN_TMPDIR/test.yaml"
    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  annotations:
      io.containers.autoupdate: "registry"
      io.containers.autoupdate/b: "local"
      io.containers.sdnotify/b: "container"
  labels:
    app: test
  name: test_pod
spec:
  containers:
  - command:
    - top
    image: $IMAGE
    name: a
  - command:
    - /runme
    image: $local_image
    name: b
EOF

    # Dispatch the YAML file
    service_name="podman-kube@$(systemd-escape $yaml_source).service"
    systemctl_start $service_name
    systemctl is-active $service_name

    # Make sure the containers are properly configured
    run_podman auto-update --dry-run --format "{{.Unit}},{{.Container}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*$service_name,.* (test_pod-a),$IMAGE,false,registry.*" "global auto-update policy gets applied"
    is "$output" ".*$service_name,.* (test_pod-b),$local_image,false,local.*" "container-specified auto-update policy gets applied"

    # Generate a broken image that will fail.
    run_podman build -t $local_image -f $dockerfile2
    run_podman image inspect --format "{{.ID}}" $local_image
    newID="$output"

    assert "$oldID" != "$newID" "broken image really is a new one"

    # Make sure container b sees the new image
    run_podman auto-update --dry-run --format "{{.Unit}},{{.Container}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*$service_name,.* (test_pod-a),$IMAGE,false,registry.*" "global auto-update policy gets applied"
    is "$output" ".*$service_name,.* (test_pod-b),$local_image,pending,local.*" "container b sees the new image"

    # Now update and check for the rollback
    run_podman auto-update --format "{{.Unit}},{{.Container}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*$service_name,.* (test_pod-a),$IMAGE,rolled back,registry.*" "container a was rolled back as the update of b failed"
    is "$output" ".*$service_name,.* (test_pod-b),$local_image,rolled back,local.*" "container b was rolled back as its update has failed"

    # Clean up
    systemctl stop $service_name
    run_podman rmi -f $local_image $newID $oldID
    run_podman network rm podman-default-kube-network
    rm -f $UNIT_DIR/$unit_name
}

@test "podman auto-update - pod" {
    dockerfile=$PODMAN_TMPDIR/Dockerfile
    cat >$dockerfile <<EOF
FROM $IMAGE
RUN touch /123
EOF

    podname=$(random_string)
    ctrname=$(random_string)
    podunit="$UNIT_DIR/pod-$podname.service.*"
    ctrunit="$UNIT_DIR/container-$ctrname.service.*"
    local_image=localhost/image:$(random_string 10)

    run_podman tag $IMAGE $local_image

    run_podman pod create --name=$podname
    run_podman create --label "io.containers.autoupdate=local" --pod=$podname --name=$ctrname $local_image top

    # cd into the unit dir to generate the two files.
    pushd "$UNIT_DIR"
    run_podman generate systemd --name --new --files $podname
    is "$output" ".*$podunit.*"
    is "$output" ".*$ctrunit.*"
    popd

    systemctl daemon-reload

    systemctl_start pod-$podname.service
    _wait_service_ready container-$ctrname.service

    run_podman pod inspect --format "{{.State}}" $podname
    is "$output" "Running" "pod is in running state"
    run_podman container inspect --format "{{.State.Status}}" $ctrname
    is "$output" "running" "container is in running state"

    run_podman pod inspect --format "{{.ID}}" $podname
    podid="$output"
    run_podman container inspect --format "{{.ID}}" $ctrname
    ctrid="$output"

    # Note that the pod's unit is listed below, not the one of the container.
    run_podman auto-update --dry-run --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*pod-$podname.service,$local_image,false,local.*" "No update available"

    run_podman build -t $local_image -f $dockerfile

    run_podman auto-update --dry-run --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*pod-$podname.service,$local_image,pending,local.*" "Image updated is pending"

    run_podman auto-update --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" ".*pod-$podname.service,$local_image,true,local.*" "Service has been restarted"
    _wait_service_ready container-$ctrname.service

    run_podman pod inspect --format "{{.ID}}" $podname
    assert "$output" != "$podid" "pod has been recreated"
    run_podman container inspect --format "{{.ID}}" $ctrname
    assert "$output" != "$ctrid" "container has been recreated"

    run systemctl stop pod-$podname.service
    assert $status -eq 0 "Error stopping pod systemd unit: $output"

    run_podman pod rm -f $podname
    run_podman rmi $local_image
    rm -f $podunit $ctrunit
    systemctl daemon-reload
}

@test "podman-auto-update --authfile"  {
    # Test the three supported ways of using authfiles with auto updates
    # 1) Passed via --authfile CLI flag
    # 2) Passed via the REGISTRY_AUTH_FILE env variable
    # 3) Via a label at container creation where 1) and 2) will be ignored

    registry=localhost:${PODMAN_LOGIN_REGISTRY_PORT}
    image_on_local_registry=$registry/name:tag
    authfile=$PODMAN_TMPDIR/authfile.json

    # First, start the registry and populate the authfile that we can use for the test.
    start_registry
    run_podman login --authfile=$authfile \
        --tls-verify=false \
        --username ${PODMAN_LOGIN_USER} \
        --password ${PODMAN_LOGIN_PASS} \
        $registry

    # Push the image to the registry and pull it down again to make sure we
    # have the identical digest in the local storage
    run_podman push --tls-verify=false --creds "${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS}" $IMAGE $image_on_local_registry
    run_podman pull --tls-verify=false --creds "${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS}" $image_on_local_registry

    # Generate a systemd service with the "registry" auto-update policy running
    # "top" inside the image we just pushed to the local registry.
    generate_service "" registry top "" "" "" $image_on_local_registry
    ctr=$cname
    _wait_service_ready container-$ctr.service

    run_podman 125 auto-update
    is "$output" \
       ".*Error: checking image updates for container .*: x509: .*"

    run_podman 125 auto-update --tls-verify=false
    is "$output" \
       ".*Error: checking image updates for container .*: authentication required"

    # Test 1)
    run_podman auto-update --authfile=$authfile --tls-verify=false --dry-run --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" "container-$ctr.service,$image_on_local_registry,false,registry" "auto-update works with authfile"

    # Test 2)
    REGISTRY_AUTH_FILE=$authfile run_podman auto-update --tls-verify=false --dry-run --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" "container-$ctr.service,$image_on_local_registry,false,registry" "auto-update works with env var"
    systemctl stop container-$ctr.service
    run_podman rm -f -t0 --ignore $ctr

    # Create a container with the auth-file label
    generate_service "" registry top "--label io.containers.autoupdate.authfile=$authfile" "" "" $image_on_local_registry
    ctr=$cname
    _wait_service_ready container-$ctr.service

    # Test 3)
    # Also make sure that the label takes precedence over the CLI flag.
    run_podman auto-update --authfile=/dev/null --tls-verify=false --dry-run --format "{{.Unit}},{{.Image}},{{.Updated}},{{.Policy}}"
    is "$output" "container-$ctr.service,$image_on_local_registry,false,registry" "auto-update works with authfile container label"
    run_podman rm -f -t0 --ignore $ctr
    run_podman rmi $image_on_local_registry
}

# vim: filetype=sh
