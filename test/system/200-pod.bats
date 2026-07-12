#!/usr/bin/env bats

load helpers
load helpers.network

LOOPDEVICE=

# Emergency cleanup if loop test fails
function teardown() {
    if [[ -n "$LOOPDEVICE" ]]; then
        losetup -d $LOOPDEVICE
    fi

    basic_teardown
}

# CANNOT BE PARALLELIZED: requires empty pod list
@test "podman pod - basic tests" {
    run_podman pod list --noheading
    is "$output" "" "baseline: empty results from list --noheading"

    run_podman pod ls -n
    is "$output" "" "baseline: empty results from ls -n"

    run_podman pod ps --noheading
    is "$output" "" "baseline: empty results from ps --noheading"
}

# bats test_tags=ci:parallel
@test "podman pod top - containers in different PID namespaces" {
    # With infra=false, we don't get a /pause container
    no_infra='--infra=false'
    run_podman pod create $no_infra
    podid="$output"

    # Start two containers...
    run_podman run -d --pod $podid $IMAGE top -d 2
    cid1="$output"
    run_podman run -d --pod $podid $IMAGE top -d 2
    cid2="$output"

    # ...and wait for them to actually start.
    wait_for_output "PID \+PPID \+USER " $cid1
    wait_for_output "PID \+PPID \+USER " $cid2

    # Both containers have emitted at least one top-like line.
    # Now run 'pod top', and expect two 'top -d 2' processes running.
    run_podman pod top $podid
    is "$output" ".*root.*top -d 2.*root.*top -d 2" "two 'top' containers"

    # By default (podman pod create w/ default --infra) there should be
    # a /pause container.
    if [ -z "$no_infra" ]; then
        is "$output" ".*0 \+1 \+0 \+[0-9. ?s]\+/pause" "there is a /pause container"
    fi

    # Cannot remove pod while containers are still running. Error messages
    # differ slightly between local and remote; these are the common elements.
    run_podman 125 pod rm $podid
    assert "${lines[0]}" =~ "Error: not all containers could be removed from pod $podid: removing pod containers.*" \
           "pod rm while busy: error message line 1 of 3"
    assert "${lines[1]}" =~ "cannot remove container .* as it is running - running or paused containers cannot be removed without force: container state improper" \
           "pod rm while busy: error message line 2 of 3"
    assert "${lines[2]}" =~ "cannot remove container .* as it is running - running or paused containers cannot be removed without force: container state improper" \
           "pod rm while busy: error message line 3 of 3"

    # Clean up
    run_podman --noout pod rm -f -t 0 $podid
    is "$output" "" "output should be empty"
}


# bats test_tags=ci:parallel
@test "podman pod create - custom volumes" {
    skip_if_remote "CONTAINERS_CONF_OVERRIDE only affects server side"
    image="i.do/not/exist:image"
    tmpdir=$PODMAN_TMPDIR/pod-test
    mkdir -p $tmpdir
    containersconf=$tmpdir/containers.conf
    cat >$containersconf <<EOF
[containers]
volumes = ["/tmp:/foobar"]
EOF

    CONTAINERS_CONF_OVERRIDE=$containersconf run_podman pod create
    podid="$output"

    CONTAINERS_CONF_OVERRIDE=$containersconf run_podman create --pod $podid $IMAGE grep foobar /proc/mounts

    run_podman pod rm $podid
}


# bats test_tags=ci:parallel
@test "podman pod create - custom infra image" {
    skip_if_remote "CONTAINERS_CONF_OVERRIDE only affects server side"
    image="i.do/not/exist:image"
    tmpdir=$PODMAN_TMPDIR/pod-test
    mkdir -p $tmpdir
    containersconf=$tmpdir/containers.conf
    cat >$containersconf <<EOF
[engine]
infra_image="$image"
EOF

    run_podman 125 pod create --infra-image $image
    is "$output" ".*initializing source docker://$image:.*"

    CONTAINERS_CONF_OVERRIDE=$containersconf run_podman 125 pod create
    is "$output" ".*initializing source docker://$image:.*"

    CONTAINERS_CONF_OVERRIDE=$containersconf run_podman 125 create --pod new:test $IMAGE
    is "$output" ".*initializing source docker://$image:.*"
}

# CANNOT BE PARALLELIZED - uses naked ps
@test "podman pod - communicating between pods" {
    podname=pod$(random_string)
    run_podman 1 pod exists $podname
    run_podman pod create --infra=true --name=$podname
    podid="$output"
    run_podman pod exists $podname
    run_podman pod exists $podid

    # (Assert that output is formatted, not a one-line blob: #8021)
    run_podman pod inspect $podname
    assert "${#lines[*]}" -ge 10 "Output from 'pod inspect'; see #8011"

    # Randomly-assigned port in the 5xxx range
    port=$(random_free_port)

    # Listener. This will exit as soon as it receives a message.
    run_podman run -d --pod $podname $IMAGE nc -l -p $port
    cid1="$output"

    # (While we're here, test the 'Pod' field of 'podman ps'. Expect two ctrs)
    run_podman ps --format '{{.Pod}}'
    newline="
"
    is "$output" "${podid:0:12}${newline}${podid:0:12}" "ps shows 2 pod IDs"

    # Talker: send the message via common port on localhost
    message=$(random_string 15)
    run_podman run --rm --pod $podname $IMAGE \
               sh -c "echo $message | nc 127.0.0.1 $port"

    # Back to the first (listener) container. Make sure message was received.
    run_podman logs $cid1
    is "$output" "$message" "message sent from one container to another"

    # Clean up. First the nc -l container...
    run_podman rm $cid1

    # ...then rm the pod and verify that it's gone
    run_podman pod rm $podname

    # Pod no longer exists
    run_podman 1 pod exists $podid
    run_podman 1 pod exists $podname
}

# bats test_tags=ci:parallel
@test "podman pod - communicating via /dev/shm " {
    podname="p-$(safename)"
    run_podman 1 pod exists $podname
    run_podman pod create --infra=true --name=$podname
    podid="$output"
    run_podman pod exists $podname
    run_podman pod exists $podid

    run_podman run --rm --pod $podname $IMAGE touch /dev/shm/test1
    run_podman run --rm --pod $podname $IMAGE ls /dev/shm/test1
    is "$output" "/dev/shm/test1"

    # ...then rm the pod and confirm it's gone
    run_podman pod rm $podname

    # Pod no longer exists
    run_podman 1 pod exists $podid
    run_podman 1 pod exists $podname
}

# Random byte
function octet() {
    echo $(( $RANDOM & 255 ))
}

# random MAC address: convention seems to be that 2nd lsb=1, lsb=0
# (i.e. 0bxxxxxx10) in the first octet guarantees a private space.
# FIXME: I can't find a definitive reference for this though
# Generate the address IN CAPS (A-F), but we will test it in lowercase.
function random_mac() {
    local mac=$(printf "%02X" $(( $(octet) & 242 | 2 )) )
    for i in $(seq 2 6); do
        mac+=$(printf ":%02X" $(octet))
    done

    echo $mac
}

# Random RFC1918 IP address
function random_ip() {
    local ip="172.20"
    for i in 1 2;do
        ip+=$(printf ".%d" $(octet))
    done
    echo $ip
}


# bats test_tags=ci:parallel
@test "podman pod create - hashtag AllTheOptions" {
    mac=$(random_mac)
    add_host_ip=$(random_ip)
    add_host_n=$(random_string | tr A-Z a-z).$(random_string | tr A-Z a-z).xyz

    dns_server=$(random_ip)
    dns_opt="ndots:$(octet)"
    dns_search=$(random_string 15 | tr A-Z a-z).abc

    hostname=$(random_string | tr A-Z a-z)."host$(safename)".net

    labelname="label-$(random_string 11)"
    labelvalue="labelvalue-$(random_string 22)"

    pod_id_file=${PODMAN_TMPDIR}/pod-id-file

    # Randomly-assigned ports in the 5xxx and 6xxx range
    port_in=$(random_free_port 5000-5999)
    port_out=$(random_free_port 6000-6999)

    # Create a pod with all the desired options
    # FIXME: --ip=$ip fails:
    #      Error adding network: failed to allocate all requested IPs
    local mac_option="--mac-address=$mac"

    # Create a custom image so we can test --infra-image and -command.
    # It will have a randomly generated infra command, using the
    # existing 'pause' script in our testimage. We assign a bogus
    # entrypoint to confirm that --infra-command will override.
    local infra_image="infra_image_$(safename)"
    local infra_command="/pause_$(safename)"
    local infra_name="infra_container_$(safename)"
    # --layers=false needed to work around buildah#5674 parallel flake
    run_podman build -t $infra_image --layers=false - << EOF
FROM $IMAGE
RUN ln /home/podman/pause $infra_command
ENTRYPOINT ["/original-entrypoint-should-be-overridden"]
EOF

    local podname="pod-$(safename)"
    if is_rootless; then
        mac_option=
    fi
    run_podman pod create --name=$podname                \
               --pod-id-file=$pod_id_file                \
               $mac_option                               \
               --hostname=$hostname                      \
               --add-host   "$add_host_n:$add_host_ip"   \
               --dns        "$dns_server"                \
               --dns-search "$dns_search"                \
               --dns-option "$dns_opt"                   \
               --publish    "$port_out:$port_in"         \
               --label      "${labelname}=${labelvalue}" \
               --infra-image   "$infra_image"            \
               --infra-command "$infra_command"          \
               --infra-name "$infra_name"
    local pod_id="$output"

    # Check --pod-id-file
    is "$(<$pod_id_file)" "$pod_id" "contents of pod-id-file"

    # Get ID of infra container
    run_podman pod inspect --format '{{(index .Containers 0).ID}}' $podname
    local infra_cid="$output"
    # confirm that entrypoint is what we set
    run_podman container inspect --format '{{.Config.Entrypoint}}' $infra_cid
    is "$output" "[${infra_command}]" "infra-command took effect"
    # confirm that infra container name is set
    run_podman container inspect --format '{{.Name}}' $infra_cid
    is "$output" "$infra_name" "infra-name took effect"

    # Check each of the options
    if [ -n "$mac_option" ]; then
        run_podman run --rm --pod $podname $IMAGE ip link show
        # 'ip' outputs hex in lower-case, ${expr,,} converts UC to lc
        is "$output" ".* link/ether ${mac,,} " "requested MAC address was set"
    fi

    run_podman run --rm --pod $podname $IMAGE hostname
    is "$output" "$hostname" "--hostname set the hostname"
    run_podman 125 run --rm --pod $podname --hostname foobar $IMAGE hostname
    is "$output" ".*invalid config provided: cannot set hostname when joining the pod UTS namespace: invalid configuration" "--hostname should not be allowed in share UTS pod"

    run_podman run --rm --pod $pod_id $IMAGE cat /etc/hosts
    is "$output" ".*$add_host_ip[[:blank:]]$add_host_n" "--add-host was added"
    is "$output" ".*	$hostname"            "--hostname is in /etc/hosts"
    #               ^^^^ this must be a tab, not a space

    run_podman run --rm --pod $podname $IMAGE cat /etc/resolv.conf
    is "$output" ".*nameserver $dns_server"  "--dns [server] was added"
    is "$output" ".*search $dns_search"      "--dns-search was added"
    is "$output" ".*options $dns_opt"        "--dns-option was added"

    # pod inspect
    run_podman pod inspect --format '{{.Name}}: {{.ID}} : {{.NumContainers}} : {{.Labels}}' $podname
    is "$output" "$podname: $pod_id : 1 : map\[${labelname}:${labelvalue}]" \
       "pod inspect --format ..."

    # pod ps
    run_podman pod ps --format '{{.ID}} {{.Name}} {{.Status}} {{.Labels}}'
    assert "$output" =~ "${pod_id:0:12} $podname Running ${labelname}=${labelvalue}"  "pod ps"

    run_podman pod ps --no-trunc --filter "label=${labelname}=${labelvalue}" --format '{{.ID}}'
    is "$output" "$pod_id" "pod ps --filter label=..."

    # Test local port forwarding, as well as 'ps' output showing ports
    # Run 'nc' in a container, waiting for input on the published port.
    c_name="ctr-$(safename)"
    run_podman run -d --pod $podname --name $c_name $IMAGE nc -l -p $port_in
    cid="$output"

    # Try running another container also listening on the same port.
    run_podman 1 run --pod $podname --name dsfsdfsdf $IMAGE nc -l -p $port_in
    is "$output" "nc: bind: Address in use" \
       "two containers cannot bind to same port"

    # make sure we can ping; failure here might mean that capabilities are wrong
    run_podman run --rm --pod $podname $IMAGE ping -c1 127.0.0.1
    run_podman run --rm --pod $podname $IMAGE ping -c1 $hostname

    # While the container is still running, run 'podman ps' (no --format)
    # and confirm that the output includes the published port
    run_podman ps --filter id=$cid
    is "${lines[1]}" "${cid:0:12}  $IMAGE  nc -l -p $port_in .* 0.0.0.0:$port_out->$port_in/tcp  $c_name" \
       "output of 'podman ps'"

    # send a random string to the container. This will cause the container
    # to output the string to its logs, then exit.
    teststring=$(random_string 30)
    echo "$teststring" > /dev/tcp/127.0.0.1/$port_out

    # Confirm that the container log output is the string we sent it.
    run_podman wait $cid
    run_podman logs $cid
    is "$output" "$teststring" "test string received on container"

    # Finally, confirm the infra-container and -command. We run this late,
    # not at pod creation, to give the infra container time to start & log.
    run_podman logs $infra_cid
    is "$output" "Confirmed: testimage pause invoked as $infra_command" \
       "pod ran with our desired infra container + command"

    # Clean up
    run_podman rm $cid
    run_podman pod rm -t 0 -f --pod-id-file $pod_id_file
    if [[ -e $pod_id_file ]]; then
        die "pod-id-file $pod_id_file should be removed along with pod"
    fi
    run_podman rmi $infra_image
}

# bats test_tags=ci:parallel
@test "podman pod create should fail when infra-name is already in use" {
    local infra_name="infra_container_$(safename)"
    local infra_image="quay.io/libpod/k8s-pause:3.5"
    local pod_name="p-$(safename)"

    run_podman --noout pod create --name $pod_name --infra-name "$infra_name" --infra-image "$infra_image"
    is "$output" "" "output from pod create should be empty"

    run_podman 125 pod create --infra-name "$infra_name"
    assert "$output" =~ "^Error: .*: the container name \"$infra_name\" is already in use by .* You have to remove that container to be able to reuse that name: that name is already in use" \
           "Trying to create two pods with same infra-name"

    run_podman pod rm -f $pod_name
    run_podman rmi $infra_image
}

# bats test_tags=ci:parallel
@test "podman pod create --share" {
    local pod_name="p-$(safename)"
    run_podman 125 pod create --share bogus --name $pod_name
    is "$output" ".*invalid kernel namespace to share: bogus. Options are: cgroup, ipc, net, pid, uts or none" \
       "pod test for bogus --share option"
    run_podman pod create --share ipc --name $pod_name
    run_podman pod inspect $pod_name --format "{{.SharedNamespaces}}"
    is "$output" "[ipc]"
    run_podman run --rm --pod $pod_name --hostname foobar $IMAGE hostname
    is "$output" "foobar" "--hostname should work with non share UTS namespace"
    run_podman pod create --share +pid --replace --name $pod_name
    run_podman pod inspect $pod_name --format "{{.SharedNamespaces}}"
    for ns in uts pid ipc net; do
        is "$output" ".*$ns"
    done

    run_podman pod rm -f $pod_name
}

# bats test_tags=ci:parallel
@test "podman pod create --pod new:$POD --hostname" {
    local pod_name="p-$(safename)"
    run_podman run --rm --pod "new:$pod_name" --hostname foobar $IMAGE hostname
    is "$output" "foobar" "--hostname should work when creating a new:pod"
    run_podman pod rm $pod_name
    run_podman run --rm --pod "new:$pod_name" $IMAGE hostname
    is "$output" "$pod_name" "new:POD should have hostname name set to podname"
    run_podman pod rm $pod_name
}

# bats test_tags=ci:parallel
@test "podman rm --force to remove infra container" {
    local pod_name="p-$(safename)"
    run_podman create --pod "new:$pod_name" $IMAGE
    container_ID="$output"
    run_podman pod inspect --format "{{.InfraContainerID}}" $pod_name
    infra_ID="$output"

    run_podman 125 container rm $infra_ID
    is "$output" ".* and cannot be removed without removing the pod"
    run_podman 125 container rm --force $infra_ID
    is "$output" ".* and cannot be removed without removing the pod"

    run_podman container rm --depend $infra_ID
    is "$output" ".*$infra_ID.*"
    is "$output" ".*$container_ID.*"

    # Now make sure that --force --all works as well
    run_podman create --pod "new:$pod_name" $IMAGE
    container_1_ID="$output"
    run_podman create --pod "$pod_name" $IMAGE
    container_2_ID="$output"
    run_podman create $IMAGE
    container_3_ID="$output"
    run_podman pod inspect --format "{{.InfraContainerID}}" $pod_name
    infra_ID="$output"

    run_podman container rm --force --depend $infra_ID
    assert "$output" =~ ".*$infra_ID.*"        "removed infra container"
    assert "$output" =~ ".*$container_1_ID.*"  "removed container 1"
    assert "$output" =~ ".*$container_2_ID.*"  "removed container 2"
    assert "$output" !~ ".*$container_3_ID.*"  "container 3 should not have been removed!"

    run_podman container rm $container_3_ID
}

# bats test_tags=ci:parallel
@test "podman pod create share net" {
    podname="p-$(safename)"
    run_podman pod create --name $podname
    run_podman pod inspect $podname --format {{.InfraConfig.HostNetwork}}
    is "$output" "false" "Default network sharing should be false"
    run_podman pod rm $podname

    run_podman pod create --share ipc  --network private $podname
    run_podman pod inspect $podname --format {{.InfraConfig.HostNetwork}}
    is "$output" "false" "Private network sharing with only ipc should be false"
    run_podman pod rm $podname

    run_podman pod create --name $podname --share net  --network private
    run_podman pod inspect $podname --format {{.InfraConfig.HostNetwork}}
    is "$output" "false" "Private network sharing with only net should be false"

    run_podman pod create --share net --network host --replace $podname
    run_podman pod inspect $podname --format {{.InfraConfig.HostNetwork}}
    is "$output" "true" "Host network sharing with only net should be true"
    run_podman pod rm $podname

    run_podman pod create --name $podname --share ipc --network host
    run_podman pod inspect $podname --format {{.InfraConfig.HostNetwork}}
    is "$output" "true" "Host network sharing with only ipc should be true"
    run_podman pod rm $podname
}

# bats test_tags=ci:parallel
@test "pod exit policies" {
    # Test setting exit policies
    run_podman pod create
    podID="$output"
    run_podman pod inspect $podID --format "{{.ExitPolicy}}"
    is "$output" "continue" "default exit policy"
    run_podman pod rm $podID

    run_podman pod create --exit-policy stop
    podID="$output"
    run_podman pod inspect $podID --format "{{.ExitPolicy}}"
    is "$output" "stop" "custom exit policy"
    run_podman pod rm $podID

    run_podman 125 pod create --exit-policy invalid
    is "$output" "Error: .*running pod create option: invalid pod exit policy: \"invalid\"" "invalid exit policy"

    # Test exit-policy behaviour
    run_podman pod create --exit-policy continue
    podID="$output"
    run_podman run --pod $podID $IMAGE true
    run_podman pod inspect $podID --format "{{.State}}"
    _ensure_pod_state $podID Degraded
    run_podman pod rm $podID

    run_podman pod create --exit-policy stop
    podID="$output"
    run_podman run --pod $podID $IMAGE true
    run_podman pod inspect $podID --format "{{.State}}"
    _ensure_pod_state $podID Exited
    run_podman pod rm -t -1 -f $podID
}

# bats test_tags=ci:parallel
@test "pod exit policies - play kube" {
    # play-kube sets the exit policy to "stop"
    local name="p-$(safename)"

    kubeFile="apiVersion: v1
kind: Pod
metadata:
  name: $name
spec:
  containers:
  - command:
    - \"true\"
    image: $IMAGE
    name: ctr
  restartPolicy: OnFailure"

    echo "$kubeFile" > $PODMAN_TMPDIR/test.yaml
    run_podman play kube $PODMAN_TMPDIR/test.yaml
    run_podman pod inspect $name --format "{{.ExitPolicy}}"
    is "$output" "stop" "custom exit policy"
    _ensure_pod_state $name Exited
    run_podman pod rm $name
}

# bats test_tags=ci:parallel
@test "pod resource limits" {
    skip_if_remote "resource limits only implemented on non-remote"
    skip_if_rootless "resource limits only work with root"

    # create loopback device
    lofile=${PODMAN_TMPDIR}/disk.img
    fallocate -l 1k  ${lofile}
    LOOPDEVICE=$(losetup --show -f $lofile)

    # tr needed because losetup seems to use %2d
    lomajmin=$(losetup -l --noheadings --output MAJ:MIN $LOOPDEVICE | tr -d ' ')
    run grep -w bfq /sys/block/$(basename ${LOOPDEVICE})/queue/scheduler
    if [ $status -ne 0 ]; then
        losetup -d $LOOPDEVICE
        LOOPDEVICE=
        skip "BFQ scheduler is not supported on the system"
    fi
    echo bfq > /sys/block/$(basename ${LOOPDEVICE})/queue/scheduler

    # FIXME: #15464: blkio-weight-device not working
    expected_limits="
cpu.max         | 500000 100000
memory.max      | 5242880
memory.swap.max | 1068498944
io.bfq.weight   | default 50
io.max          | $lomajmin rbps=1048576 wbps=1048576 riops=max wiops=max
"

    defer-assertion-failures
    for cgm in systemd cgroupfs; do
        local name="p-resources-$cgm-$(safename)"
        run_podman --cgroup-manager=$cgm pod create --name=$name --cpus=5 --memory=5m --memory-swap=1g --cpu-shares=1000 --cpuset-cpus=0 --cpuset-mems=0 --device-read-bps=${LOOPDEVICE}:1mb --device-write-bps=${LOOPDEVICE}:1mb --blkio-weight=50
        run_podman --cgroup-manager=$cgm pod start $name
        run_podman pod inspect --format '{{.CgroupPath}}' $name
        local cgroup_path="$output"

        while read unit expect; do
            local actual=$(< /sys/fs/cgroup/$cgroup_path/$unit)
            is "$actual" "$expect" "resource limit under $cgm: $unit"
        done < <(parse_table "$expected_limits")
        run_podman --cgroup-manager=$cgm pod rm -f $name
    done

    # Clean up, and prevent duplicate cleanup in teardown
    losetup -d $LOOPDEVICE
    LOOPDEVICE=
}

# CANNOT BE PARALLELIZED: rm -a
@test "podman pod ps doesn't race with pod rm" {
    # create a few pods
    for i in {0..10}; do
        run_podman pod create
    done

    # and delete them
    "${PODMAN_CMD[@]}" pod rm -a &

    # pod ps should not fail while pods are deleted
    run_podman pod ps -q

    # wait for pod rm -a
    wait
}

# CANNOT BE PARALLELIZED: naked ps
@test "podman pod rm --force bogus" {
    run_podman 1 pod rm bogus
    is "$output" "Error: .*bogus.*: no such pod" "Should print error"
    run_podman pod rm -t -1 --force bogus
    is "$output" "" "Should print no output"

    run_podman pod create --name testpod
    run_podman pod rm --force bogus testpod
    assert "$output" =~ "[0-9a-f]{64}" "rm pod"
    run_podman pod ps -q
    assert "$output" = "" "no pods listed"
}

# bats test_tags=ci:parallel
@test "podman pod create on failure" {
    podname="p-$(safename)"
    nwname="n-$(safename)"

    run_podman 125 pod create --network $nwname --name $podname
    # FIXME: podman and podman-remote do not return the same error message
    # but consistency would be nice
    is "$output" "Error: .*unable to find network with name or ID $nwname: network not found"

    # Make sure the pod doesn't get created on failure
    run_podman 1 pod exists $podname
}

# bats test_tags=ci:parallel
@test "podman pod create restart tests" {
    podname="p-$(safename)"

    run_podman pod create --restart=on-failure --name $podname
    run_podman create --name test-ctr --pod $podname $IMAGE
    run_podman container inspect --format '{{ .HostConfig.RestartPolicy.Name }}' test-ctr
    is "$output" "on-failure" "container inherits from pod"

    run_podman create --replace --restart=always --name test-ctr --pod $podname $IMAGE
    run_podman container inspect --format '{{ .HostConfig.RestartPolicy.Name }}' test-ctr
    is "$output" "always" "container overrides restart policy from pod"

    run_podman pod rm -f $podname
}

# Helper used by pod ps --filter test. Creates one pod or container
# with a UNIQUE two-character CID prefix.
function thingy_with_unique_id() {
    local what="$1"; shift              # pod or container
    local how="$1"; shift               # e.g. "--name p1c1 --pod p1"

    while :;do
          local try_again=

          run_podman $what create $how
          # This is our return value; it propagates up to caller's namespace
          id="$output"

          # Make sure the first two characters aren't already used in an ID
          for existing_id in "$@"; do
              if [[ -z "$try_again" ]]; then
                  if [[ "${existing_id:0:2}" == "${id:0:2}" ]]; then
                      run_podman $what rm $id
                      try_again=1
                  fi
              fi
          done

          if [[ -z "$try_again" ]]; then
              # Nope! groovy! caller gets $id
              return
          fi
    done
}

# bats test_tags=ci:parallel
@test "podman pod ps --filter" {
    local -A podid
    local -A ctrid

    # Setup: create three pods, each with three containers, all of them with
    # unique (distinct) first two characters of their pod/container ID.
    for p in 1 2 3;do
        # no infra, please! That creates an extra container with a CID
        # that may collide with our other ones, and it's too hard to fix.
        podname="p-${p}-$(safename)"
        thingy_with_unique_id "pod" "--infra=false --name $podname" \
                              ${podid[*]} ${ctrid[*]}
        podid[$p]=$id

        for c in 1 2 3; do
            thingy_with_unique_id "container" \
                                  "--pod $podname --name $podname-c${c} $IMAGE true" \
                                  ${podid[*]} ${ctrid[*]}
            ctrid[$p$c]=$id
        done
    done

    # for debugging; without this, on test failure it's too hard to
    # associate IDs with names
    run_podman pod ps
    run_podman ps -a

    # Normally (sequential Bats) we can do equality checks on ps output,
    # because thingy_with_unique_id() guarantees that we won't have collisions
    # in the first two characters of the hash. When running in parallel,
    # there's no such guarantee.
    local op="="
    if [[ -n "$PARALLEL_JOBSLOT" ]]; then
        op="=~"
    fi

    # Test: ps and filter for each pod and container, by ID
    defer-assertion-failures
    for p in 1 2 3; do
        local pid=${podid[$p]}
        local podname="p-$p-$(safename)"

        # Search by short pod ID, longer pod ID, pod ID regex, and pod name
        # ps by short ID, longer ID, regex, and name
        for filter in "id=${pid:0:2}" "id=${pid:0:10}" "id=^${pid:0:2}" "name=$podname"; do
            run_podman pod ps --filter=$filter --format '{{.Name}}:{{.Id}}'
            assert "$output" $op "$podname:${pid:0:12}" "pod $p, filter=$filter"
        done

        # ps by negation (regex) of our pid, should find all other pods
        f1="^[^${pid:0:1}]"
        f2="^.[^${pid:1:1}]"
        run_podman pod ps --filter=id="$f1" --filter=id="$f2" --format '{{.Name}}'
        assert "${#lines[*]}" -ge "2"  "filter=$f1 + $f2 finds at least 2 pods"
        assert "$output" !~ "$podname" "filter=$f1 + $f2 does not find pod $p"
        # Confirm that the other two pods _are_ in our list
        for notp in 1 2 3; do
            if [[ $notp -ne $p ]]; then
                assert "$output" =~ "p-$notp-$(safename)" "filter=$f1 + $f2 finds pod $notp"
            fi
        done
        # Search by *container* ID
        for c in 1 2 3;do
            local cid=${ctrid[$p$c]}
            local podname="p-$p-$(safename)"
            for filter in "ctr-ids=${cid:0:2}" "ctr-ids=^${cid:0:2}.*"; do
                run_podman pod ps --filter=$filter --format '{{.Name}}:{{.Id}}'
                assert "$output" $op "$podname:${pid:0:12}" \
                       "pod $p, container $c, filter=$filter"
            done
        done
    done

    # Multiple filters, multiple pods
    run_podman pod ps --filter=ctr-ids=${ctrid[12]} \
                      --filter=ctr-ids=${ctrid[23]} \
                      --filter=ctr-ids=${ctrid[31]} \
                      --format='{{.Name}}' --sort=name
    assert "$(echo $output)" == "p-1-$(safename) p-2-$(safename) p-3-$(safename)" \
           "multiple ctr-ids filters"

    # Clean up
    run_podman pod rm -f ${podid[*]}
}


# bats test_tags=ci:parallel
@test "podman pod cleans cgroup and keeps limits" {
    skip_if_remote "we cannot check cgroup settings"

    for infra in true false; do
        run_podman pod create --infra=$infra --memory=256M
        podid="$output"
        run_podman run -d --pod $podid $IMAGE top -d 2

        run_podman pod inspect $podid --format "{{.CgroupPath}}"
        result="$output"
        assert "$result" =~ "/" ".CgroupPath is a valid path"

        if is_cgroupsv2; then
           cgroup_path=/sys/fs/cgroup/$result
        else
           cgroup_path=/sys/fs/cgroup/memory/$result
        fi

        if test ! -e $cgroup_path; then
            die "the cgroup $cgroup_path does not exist"
        fi

        run_podman pod stop -t 0 $podid
        if test -e $cgroup_path; then
            die "the cgroup $cgroup_path should not exist after pod stop"
        fi

        run_podman pod start $podid
        if test ! -e $cgroup_path; then
            die "the cgroup $cgroup_path does not exist"
        fi

        # validate that cgroup limits are in place after a restart
        # issue #19175
        if is_cgroupsv2; then
           memory_limit_file=$cgroup_path/memory.max
        else
           memory_limit_file=$cgroup_path/memory.limit_in_bytes
        fi
        assert "$(< $memory_limit_file)" = "268435456" "Contents of $memory_limit_file"

        run_podman pod rm -t 0 -f $podid
        if test -e $cgroup_path; then
            die "the cgroup $cgroup_path should not exist after pod rm"
        fi
    done
}

@test "podman pod inspect ordering" {
    local pod_name="p-$(safename)"
    run_podman pod create $pod_name

    run_podman pod inspect --format '{{ .SharedNamespaces }}' $pod_name
    assert "$output" == "[ipc net uts]"

    run_podman pod rm $pod_name
}

# vim: filetype=sh
