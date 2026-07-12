#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman pause/unpause functionality
#

load helpers
load helpers.systemd

# SHOULD NOT BE PARALLELIZED! The use of podman ps -a makes it timing sensitive
# as this command takes locks of all containers and parallel containers may
# block for a while.
@test "podman pause/unpause" {
    if is_rootless && ! is_cgroupsv2; then
        skip "'podman pause' (rootless) only works with cgroups v2"
    fi

    cname="c-$(safename)"
    run_podman run -d --name $cname $IMAGE \
               sh -c 'while :;do date +%s;sleep 1;done'
    cid="$output"
    # Wait for first time value
    wait_for_output '[0-9]\{10,\}' $cid

    # Pause container, sleep a bit, unpause, sleep again to give process
    # time to write a new post-restart time value. Pause by CID, unpause
    # by name, just to exercise code paths. While paused, check 'ps'
    # and 'inspect', then check again after restarting.
    run_podman --noout pause $cid
    is "$output" "" "output should be empty"
    run_podman inspect --format '{{.State.Status}}' $cid
    is "$output" "paused" "podman inspect .State.Status"
    sleep 3
    run_podman ps -a --format '{{.ID}} {{.Names}} {{.Status}}'
    assert "$output" =~ ".*${cid:0:12} $cname Paused.*" "podman ps on paused container"
    run_podman unpause $cname
    run_podman ps -a --format '{{.ID}} {{.Names}} {{.Status}}'
    assert "$output" =~ ".*${cid:0:12} $cname Up .*" "podman ps on resumed container"
    sleep 1

    # Get full logs, and iterate through them computing delta_t between entries
    run_podman logs $cid
    i=1
    max_delta=0
    while [ $i -lt ${#lines[*]} ]; do
        this_delta=$(( ${lines[$i]} - ${lines[$(($i - 1))]} ))
        if [ $this_delta -gt $max_delta ]; then
            max_delta=$this_delta
        fi
        i=$(( $i + 1 ))
    done

    # There should be a 3-4 second gap, *maybe* 5. Never 1 or 2, that
    # would imply that the container never paused.
    is "$max_delta" "[3456]" "delta t between paused and restarted"

    run_podman rm -t 0 -f $cname

    # Pause/unpause on nonexistent name or id - these should all fail
    run_podman 125 pause $cid
    assert "$output" =~ "no container with name or ID \"$cid\" found: no such container"
    run_podman 125 pause $cname
    assert "$output" =~ "no container with name or ID \"$cname\" found: no such container"
    run_podman 125 unpause $cid
    assert "$output" =~ "no container with name or ID \"$cid\" found: no such container"
    run_podman 125 unpause $cname
    assert "$output" =~ "no container with name or ID \"$cname\" found: no such container"
}

# CANNOT BE PARALLELIZED! (because of unpause --all)
@test "podman unpause --all" {
    if is_rootless && ! is_cgroupsv2; then
        skip "'podman pause' (rootless) only works with cgroups v2"
    fi

    cname="c-$(safename)"
    cname_notrunning="c-notrunning-$(safename)"

    run_podman create --name $cname_notrunning $IMAGE
    run_podman run -d --name $cname $IMAGE sleep 100
    cid="$output"
    run_podman pause $cid
    run_podman inspect --format '{{.State.Status}}' $cid
    is "$output" "paused" "podman inspect .State.Status"
    run_podman unpause --all
    is "$output" "$cid" "podman unpause output"
    run_podman ps --format '{{.ID}} {{.Names}} {{.Status}}'
    is "$output" "${cid:0:12} $cname Up.*" "podman ps on resumed container"

    run_podman rm -t 0 -f $cname $cname_notrunning
}

# bats test_tags=ci:parallel
@test "podman pause/unpause with HealthCheck interval" {
    if is_rootless && ! is_cgroupsv2; then
        skip "'podman pause' (rootless) only works with cgroups v2"
    fi

    local ctrname="c-$(safename)"
    local msg="healthmsg-$(random_string)"

    run_podman run -d --name $ctrname     \
                --health-cmd "echo $msg"  \
                --health-interval 1s      \
                $IMAGE /home/podman/pause
    cid="$output"

    run_podman healthcheck run $ctrname
    is "$output" "" "output from 'podman healthcheck run'"

    # We checking only timer because checking of service caused unexpected exit code 3 of systemctl status.
    # Since the status check can be executed when HealthCheck was exited, this caused a termination error code 3
    # for systemctl status. Because service was in dead state because HealthCheck exited.
    # https://github.com/containers/podman/issues/25204
    run -0 systemctl status $cid-*.timer
    assert "$output" =~ "active" "service should be running"

    run_podman --noout pause $ctrname
    assert "$output" == "" "output should be empty"

    run -0 systemctl status $cid-*.{service,timer}
    assert "$output" == "" "service should not be running"

    run_podman --noout unpause $ctrname
    assert "$output" == "" "output should be empty"

    run_podman healthcheck run $ctrname
    is "$output" "" "output from 'podman healthcheck run'"

    run -0 systemctl status $cid-*.timer
    assert "$output" =~ "active" "service should be running"

    run_podman rm -t 0 -f $ctrname

    # Important check for https://github.com/containers/podman/issues/22884
    # We never should leak the unit files, healthcheck uses the cid in name so just grep that.
    # (Ignore .scope units, those are conmon and can linger for 5 minutes)
    # (Ignore .mount, too. They are created/removed by systemd based on the actual real mounts
    #  on the host and that is async and might be slow enough in CI to cause failures.)
    run -0 systemctl list-units --quiet "*$cid*"
    except_scope_mount=$(grep -vF ".scope " <<<"$output" | { grep -vF ".mount" || true; } )
    assert "$except_scope_mount" == "" "Healthcheck systemd unit cleanup: no units leaked"
}
# vim: filetype=sh
