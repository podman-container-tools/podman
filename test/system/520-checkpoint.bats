#!/usr/bin/env bats   -*- bats -*-
#
# test podman checkpoint. Similar in many ways to our pause tests.
#

load helpers
load helpers.network

CHECKED_ROOTLESS=
function setup() {
    # None of these tests work rootless....
    if is_rootless; then
        # ...however, is that a genuine cast-in-stone limitation, or one
        # that can some day be fixed? If one day some PR removes that
        # restriction, fail loudly here, so the developer can enable tests.
        if [[ -n "$CHECKED_ROOTLESS" ]]; then
            run_podman '?' container checkpoint -l
            is "$output" "Error: checkpointing a container requires root" \
               "Confirming that rootless checkpoint doesn't work. If that changed, please reexamine this test file!"
            CHECKED_ROOTLESS=y
        fi
        skip "checkpoint does not work rootless"
    fi

    basic_setup

    # Note basic_setup defines $PODMAN_RUNTIME so this must be after it

    # As of 2024-05, crun on Debian is not built with criu support:
    # https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=1008249
    runtime="$PODMAN_RUNTIME"
    run $runtime checkpoint --help
    if [[ $status -ne 0 ]]; then
        skip "runtime $runtime does not support checkpoint/restore"
    fi
}

# bats test_tags=ci:parallel
@test "podman checkpoint - basic test" {
    run_podman run -d $IMAGE sh -c 'while :;do cat /proc/uptime; sleep 0.1;done'
    local cid="$output"

    # Wait for container to start emitting output
    wait_for_output '[1-9]\+' $cid

    # Checkpoint, and confirm via inspect
    run_podman container checkpoint $cid
    # FIXME: remove the `.*` prefix after fix packaged for https://github.com/checkpoint-restore/criu/pull/1706
    is "$output" ".*$cid" "podman container checkpoint"

    run_podman container inspect \
               --format '{{.State.Status}}:{{.State.Running}}:{{.State.Paused}}:{{.State.Checkpointed}}' $cid
    is "$output" "exited:false:false:true" "State. Status:Running:Pause:Checkpointed"

    # Plan A was to do something similar to 080-pause.bats: sleep for long
    # enough to cause a gap in the timestamps in the log. But checkpoint
    # doesn't seem to work like that: upon restore, even if we sleep a long
    # time, the newly-started container seems to pick back up close to
    # where it left off. (Maybe it's something about /proc/uptime?)
    # Anyway, scratch Plan A. Plan B is simply to make sure that the
    # restarted container spits something out.
    run_podman logs $cid
    local nlines_before="${#lines[*]}"

    # Restart immediately and confirm state
    run_podman container restore $cid
    is "$output" "$cid" "podman container restore"

    # Note that upon restore, .Checkpointed reverts to false (#12117)
    run_podman container inspect \
               --format '{{.State.Status}}:{{.State.Running}}:{{.State.Paused}}:{{.State.Checkpointed}}' $cid
    is "$output" "running:true:false:false" \
       "State. Status:Running:Pause:Checkpointed"

    # Re-fetch logs, and ensure that they continue growing.
    # Allow a short while for container process to actually restart.
    local retries=10
    while [[ $retries -gt 0 ]]; do
        run_podman logs $cid
        local nlines_after="${#lines[*]}"
        if [[ $nlines_after -gt $nlines_before ]]; then
            break
        fi
        sleep 0.1
        retries=$((retries - 1))
    done
    assert "$retries" -gt 0 \
           "Container failed to output new lines after first restore"

    # Same thing again: test for https://github.com/containers/crun/issues/756
    # in which, after second checkpoint/restore, we lose logs
    run_podman container checkpoint $cid
    run_podman container logs $cid
    nlines_before="${#lines[*]}"
    run_podman container restore $cid

    # Same as above, confirm that we get new output
    retries=10
    while [[ $retries -gt 0 ]]; do
        run_podman logs $cid
        local nlines_after="${#lines[*]}"
        if [[ $nlines_after -gt $nlines_before ]]; then
            break
        fi
        sleep 0.1
        retries=$((retries - 1))
    done
    assert "$retries" -gt 0 \
           "stdout went away after second restore (crun issue 756)"

    run_podman rm -t 0 -f $cid
}

# CANNOT BE PARALLELIZED: checkpoint -a
@test "podman checkpoint/restore print IDs or raw input" {
    # checkpoint/restore -a must print the IDs
    run_podman run -d $IMAGE top
    ctrID="$output"
    run_podman container checkpoint -a
    is "$output" "$ctrID"
    run_podman container restore -a
    is "$output" "$ctrID"

    # checkpoint/restore $input must print $input
    cname=c-$(safename)
    run_podman run -d --name $cname $IMAGE top
    run_podman container checkpoint $cname
    is "$output" $cname
    run_podman container restore $cname
    is "$output" $cname

    run_podman rm -t 0 -f $ctrID $cname
}

# bats test_tags=ci:parallel
@test "podman checkpoint --export, with volumes" {
    skip_if_aarch64 "FIXME #28576: selinux problem only on aarch64"
    skip_if_remote "Test uses --root/--runroot, which are N/A over remote"

    local p_opts="$(podman_isolation_opts ${PODMAN_TMPDIR}) --events-backend file"
    # prefetch image to avoid registry pulls because this is using a
    # unique root which does not have the image already present.
    # _PODMAN_TEST_OPTS is used to overwrite the podman options to
    # make the function aware of the custom --root.
    _PODMAN_TEST_OPTS="$p_opts --storage-driver $(podman_storage_driver)" _prefetch $IMAGE

    # Create a volume, find unused network port, and create a webserv container
    volname=v-$(safename)
    run_podman $p_opts volume create $volname
    local cname=c-$(safename)
    local host_port=$(random_free_port)
    local server=http://127.0.0.1:$host_port

    run_podman $p_opts run -d --name $cname --volume $volname:/myvol \
               -p $host_port:80 \
               -w /myvol \
               $IMAGE sh -c "/bin/busybox-extras httpd -p 80;echo $cname >cname;echo READY;while :;do cat /proc/uptime >mydate.tmp;mv -f mydate.tmp mydate;sleep 0.1;done"
    local cid="$output"
    _PODMAN_TEST_OPTS="$p_opts" wait_for_ready $cid

    # Confirm that container responds
    run curl --max-time 3 -s $server/cname
    is "$output" "$cname" "curl $server/cname"
    run curl --max-time 3 -s $server/mydate
    local date_oldroot="$output"

    # Checkpoint...
    run_podman $p_opts container checkpoint \
               --ignore-rootfs \
               --export=$PODMAN_TMPDIR/$cname.tar.gz \
               $cname

    # ...confirm that port is now closed
    run curl --max-time 1 -s $server/mydate
    is "$status" "7" "cannot connect to port $host_port while container is down"

    # ...now restore it to our regular root
    run_podman container restore --import=$PODMAN_TMPDIR/$cname.tar.gz
    is "$output" "$cid"

    # Inspect (on regular root). Note that, unlike the basic test above,
    # .State.Checkpointed here is *false*.
    run_podman container inspect \
               --format '{{.State.Status}}:{{.State.Running}}:{{.State.Paused}}:{{.State.Checkpointed}}' $cname
    is "$output" "running:true:false:false" "State. Status:Running:Pause:Checkpointed"

    # Pause a moment to let the restarted container update the timestamp file
    sleep .3
    run curl --max-time 3 -s $server/mydate
    local date_newroot="$output"
    assert "$date_newroot" != "$date_oldroot" \
           "Restored container did not update the timestamp file"

    run_podman exec $cid cat /myvol/cname
    is "$output" "$cname" "volume transferred fine"

    run_podman rm -t 0 -f $cid
    run_podman volume rm -f $volname
}

# FIXME: test --leave-running

# bats test_tags=ci:parallel
@test "podman checkpoint --file-locks" {
    action='flock test.lock sh -c "while [ -e /wait ];do sleep 0.5;done;for i in 1 2 3;do echo \$i;sleep 0.5;done"'
    run_podman run -d $IMAGE sh -c "touch /wait; touch test.lock; echo READY; $action & $action & wait"
    local cid="$output"

    # Wait for container to start emitting output
    wait_for_ready $cid

    # Checkpoint, and confirm via inspect
    run_podman container checkpoint --file-locks $cid
    is "$output" "$cid" "podman container checkpoint"

    run_podman container inspect \
               --format '{{.State.Status}}:{{.State.Running}}:{{.State.Paused}}:{{.State.Checkpointed}}' $cid
    is "$output" "exited:false:false:true" "State. Status:Running:Pause:Checkpointed"

    # Restart immediately and confirm state
    run_podman container restore --file-locks $cid
    is "$output" "$cid" "podman container restore"

    # Signal the container to continue; this is where the 1-2-3s will come from
    # The '-d' is because container exit is racy: the exec process itself
    # could get caught and killed by cleanup, causing this step to exit 137
    run_podman exec -d $cid rm /wait

    # Wait for the container to stop
    run_podman wait $cid

    run_podman logs $cid
    trim=$(sed -z -e 's/[\r\n]\+//g' <<<"$output")
    is "$trim" "READY123123" "File lock restored"

    run_podman rm $cid
}

# bats test_tags=ci:parallel
@test "podman checkpoint/restore ip and mac handling" {
    # Broken only debian as it seems the host's /etc/hosts file keeps changing
    # which causes false positives in the before/after restore comparison.
    OS_RELEASE_ID="${OS_RELEASE_ID:-$(source /etc/os-release; echo $ID)}"
    if [[ "$OS_RELEASE_ID" == "debian" ]]; then
        skip "Test flakes on debian in CI"
    fi
    # Refer to https://github.com/containers/podman/issues/16666#issuecomment-1337860545
    # for the correct behavior, this should cover all cases listed there.
    local netname="net-$(safename)"
    local subnet="$(random_rfc1918_subnet)"
    run_podman network create --subnet "$subnet.0/24" $netname

    run_podman run -d --network $netname $IMAGE top
    cid="$output"
    # get current ip and mac
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    ip1="$output"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    mac1="$output"

    # There is a weird flake, where the hosts content changed after restore and I don't know why.
    # Because we start from a /etc/hosts base on the host print that.
    echo "hosts file on the host"
    cat /etc/hosts

    run_podman exec $cid cat /etc/hosts /etc/resolv.conf
    pre_hosts_resolv_conf_output="$output"

    run_podman container checkpoint $cid
    is "$output" "$cid"
    run_podman container restore $cid
    is "$output" "$cid"

    # now get mac and ip after restore they should be the same
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    ip2="$output"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    mac2="$output"

    # Make sure hosts and resolv.conf are the same after restore (#22901)
    run_podman exec $cid cat /etc/hosts /etc/resolv.conf
    assert "$output" == "$pre_hosts_resolv_conf_output" "hosts/resolv.conf must be the same after checkpoint"

    assert "$ip2" == "$ip1" "ip after restore should match"
    assert "$mac2" == "$mac1" "mac after restore should match"

    # restart the container we should get a new ip/mac because they are not static
    run_podman restart $cid

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    ip3="$output"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    mac3="$output"

    # the ip/mac should be different this time
    assert "$ip3" != "$ip1" "ip after restart should be different"
    assert "$mac3" != "$mac1" "mac after restart should be different"

    # restore with --ignore-static-ip/mac
    run_podman container checkpoint $cid
    is "$output" "$cid"
    run_podman container restore --ignore-static-ip --ignore-static-mac $cid
    is "$output" "$cid"

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    ip4="$output"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    mac4="$output"

    # the ip/mac should be different this time
    assert "$ip4" != "$ip3" "ip after restore --ignore-static-ip should be different"
    assert "$mac4" != "$mac3" "mac after restore --ignore-static-mac should be different"

    local archive=$PODMAN_TMPDIR/checkpoint.tar.gz

    # now checkpoint and export the container
    run_podman container checkpoint --export "$archive" $cid
    is "$output" "$cid"
    # remove container
    run_podman rm -t 0 -f $cid

    # restore it without new name should keep the ip/mac, we also get a new container id
    run_podman container restore --import "$archive"
    cid="$output"

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    ip5="$output"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    mac5="$output"
    assert "$ip5" == "$ip4" "ip after restore --import should match"
    assert "$mac5" == "$mac4" "mac after restore --import should match"

    run_podman rm -t 0 -f $cid

    # now restore it again but with --name this time, it should not keep the
    # mac and ip to allow restoring the same container with different names
    # at the same time
    newname="newc-$(safename)"
    run_podman container restore --import "$archive" --name $newname
    cid="$output"

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    ip6="$output"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    mac6="$output"
    assert "$ip6" != "$ip5" "ip after restore --import --name should be different"
    assert "$mac6" != "$mac5" "mac after restore --import --name should be different"

    run_podman rm -t 0 -f $cid

    # now create a container with a static mac and ip
    local static_ip="$subnet.2"
    local static_mac="92:d0:c6:0a:29:38"
    run_podman run -d --network "$netname:ip=$static_ip,mac=$static_mac" $IMAGE top
    cid="$output"

    run_podman container checkpoint $cid
    is "$output" "$cid"
    run_podman container restore --ignore-static-ip --ignore-static-mac $cid
    is "$output" "$cid"

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    ip7="$output"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    mac7="$output"
    assert "$ip7" != "$static_ip" "static ip after restore --ignore-static-ip should be different"
    assert "$mac7" != "$static_mac" "static mac after restore --ignore-static-mac should be different"

    # restart the container to make sure the change is actually persistent in the config and not just set for restore
    run_podman restart $cid

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    ip8="$output"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    mac8="$output"
    assert "$ip8" != "$static_ip" "static ip after restore --ignore-static-ip and restart should be different"
    assert "$mac8" != "$static_mac" "static mac after restore --ignore-static-mac and restart should be different"
    assert "$ip8" != "$ip7" "static ip after restore --ignore-static-ip and restart should be different"
    assert "$mac8" != "$ip" "static mac after restore --ignore-static-mac and restart should be different"

    run_podman rm -t 0 -f $cid

    # now create container again and try the same again with --export and --import
    run_podman run -d --network "$netname:ip=$static_ip,mac=$static_mac" $IMAGE top
    cid="$output"

    run_podman container checkpoint --export "$archive" $cid
    is "$output" "$cid"
    # remove container
    run_podman rm -t 0 -f $cid

    # restore normal should keep static ip
    run_podman container restore --import "$archive"
    cid="$output"

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    ip9="$output"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    mac9="$output"
    assert "$ip9" == "$static_ip" "static ip after restore --import should match"
    assert "$mac9" == "$static_mac" "static mac after restore --import should match"

    # restart the container to make sure the change is actually persistent in the config and not just set for restore
    run_podman restart $cid
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    ip10="$output"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    mac10="$output"
    assert "$ip10" == "$static_ip" "static ip after restore --import and restart should match"
    assert "$mac10" == "$static_mac" "static mac after restore --import and restart should match"

    run_podman rm -t 0 -f $cid

    # restore normal without keeping static ip/mac
    run_podman container restore --ignore-static-ip --ignore-static-mac --import "$archive"
    cid="$output"

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    ip11="$output"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    mac11="$output"
    assert "$ip11" != "$static_ip" "static ip after restore --import --ignore-static-ip should be different"
    assert "$mac11" != "$static_mac" "static mac after restore --import --ignore-static-mac should be different"

    # restart the container to make sure the change is actually persistent in the config and not just set for restore
    run_podman restart $cid

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    ip12="$output"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    mac12="$output"
    assert "$ip12" != "$static_ip" "static ip after restore --import --ignore-static-ip and restart should be different"
    assert "$mac12" != "$static_mac" "static mac after restore --ignore-static-mac and restart should be different"
    assert "$ip12" != "$ip11" "static ip after restore --import --ignore-static-ip and restart should be different"
    assert "$mac12" != "$ip11" "static mac after restore --ignore-static-mac and restart should be different"

    run_podman rm -t 0 -f $cid
    run_podman network rm $netname
}

# rhbz#2177611 : podman breaks checkpoint/restore
# CANNOT BE PARALLELIZED: --latest
@test "podman checkpoint/restore the latest container" {
    skip_if_remote "podman-remote does not support --latest option"
    # checkpoint/restore -l must print the IDs
    run_podman run -d $IMAGE top
    ctrID="$output"
    run_podman container checkpoint --latest
    is "$output" "$ctrID"

    run_podman container inspect \
               --format '{{.State.Status}}:{{.State.Running}}:{{.State.Paused}}:{{.State.Checkpointed}}' $ctrID
    is "$output" "exited:false:false:true" "State. Status:Running:Pause:Checkpointed"

    run_podman container restore -l
    is "$output" "$ctrID"

    run_podman container inspect \
               --format '{{.State.Status}}:{{.State.Running}}:{{.State.Paused}}:{{.State.Checkpointed}}' $ctrID
    is "$output" "running:true:false:false" "State. Status:Running:Pause:Checkpointed"

    run_podman rm -t 0 -f $ctrID
}

@test "podman checkpoint/restore - publish with default_host_ips" {
    skip_if_remote "CONTAINERS_CONF_OVERRIDE redirect does not work on remote"

    local port=$(random_free_port)
    local archive=$PODMAN_TMPDIR/checkpoint.tar.gz
    containersconf=$PODMAN_TMPDIR/containers.conf
    cat >$containersconf <<EOF
[network]
  default_host_ips = ["127.0.0.1"]
EOF

    run_podman run -d $IMAGE top
    local cid="$output"

    run_podman container checkpoint --export "$archive" $cid
    is "$output" "$cid" "podman container checkpoint"
    run_podman rm -t 0 -f $cid

    CONTAINERS_CONF_OVERRIDE=$containersconf run_podman container restore --import "$archive" --publish $port:80
    local newcid="$output"

    CONTAINERS_CONF_OVERRIDE=$containersconf run_podman inspect $newcid --format "{{.HostConfig.PortBindings}}"
    assert "$output" =~ "127.0.0.1" "HostIP should be 127.0.0.1 from config"

    run_podman rm -t 0 -f $newcid
}

# vim: filetype=sh
