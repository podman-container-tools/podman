#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman rm
#

load helpers

# bats test_tags=ci:parallel
@test "podman rm" {
    cname=c-$(safename)
    run_podman run --name $cname $IMAGE /bin/true

    # Don't care about output, just check exit status (it should exist)
    run_podman 0 inspect $cname

    # container should be in output of 'ps -a'
    run_podman ps -a
    is "$output" ".* $IMAGE .*/true .* $cname" "Container present in 'ps -a'"

    # Remove container; now 'inspect' should fail
    run_podman rm $cname
    is "$output" "$cname" "display raw input"
    run_podman 125 inspect $cname
    is "$output" "\[\].Error: no such object: \"$cname\""
    run_podman 125 wait $cname
    is "$output" "Error: no container with name or ID \"$cname\" found: no such container"
    run_podman wait --ignore $cname
    is "$output" "-1" "wait --ignore will mark missing containers with -1"
}

# bats test_tags=ci:parallel
@test "podman rm - running container, w/o and w/ force" {
    run_podman run -d $IMAGE sleep 5
    cid="$output"

    # rm should fail
    run_podman 2 rm $cid
    is "$output" "Error: cannot remove container $cid as it is running - running or paused containers cannot be removed without force: container state improper" "error message"

    # rm -f should succeed
    run_podman rm -t 0 -f $cid
}

# bats test_tags=ci:parallel
@test "podman rm container from storage" {
    if is_remote; then
        skip "only applicable for local podman"
    fi
    cname=c-$(safename)
    run_podman create --name $cname $IMAGE /bin/true

    # Create a container that podman does not know about
    external_cid=$(buildah from $IMAGE)

    # Plain 'exists' should fail, but should succeed with --external
    run_podman 1 container exists $external_cid
    run_podman container exists --external $external_cid

    # rm should succeed
    run_podman rm $cname $external_cid
}

# bats test_tags=ci:parallel
@test "podman rm <-> run --rm race" {
    OCIDir=/run/$(podman_runtime)

    if is_rootless; then
        OCIDir=/run/user/$(id -u)/$(podman_runtime)
    fi

    # A container's lock is released before attempting to stop it.  This opens
    # the window for race conditions that led to #9479.
    run_podman run --rm -d $IMAGE sleep infinity
    local cid="$output"
    run_podman rm -f -t0 $cid

    # Check the OCI runtime directory has removed.
    is "$(ls $OCIDir | grep $cid)" "" "The OCI runtime directory should have been removed"
}

# bats test_tags=ci:parallel
@test "podman rm --depend" {
    run_podman create $IMAGE
    dependCid=$output
    run_podman create --net=container:$dependCid $IMAGE
    cid=$output
    run_podman 125 rm $dependCid
    is "$output" "Error: container $dependCid has dependent containers which must be removed before it:.*" "Fail to remove because of dependencies"
    run_podman rm --depend $dependCid
    is "$output" ".*$cid" "Container should have been removed"
    is "$output" ".*$dependCid" "Depend container should have been removed"
}

# I'm sorry! This test takes 13 seconds. There's not much I can do about it,
# please know that I think it's justified: podman 1.5.0 had a strange bug
# in with exit status was not preserved on some code paths with 'rm -f'
# or 'podman run --rm' (see also 030-run.bats). The test below is a bit
# kludgy: what we care about is the exit status of the killed container,
# not 'podman rm', but BATS has no provision (that I know of) for forking,
# so what we do is start the 'rm' beforehand and monitor the exit status
# of the 'sleep' container.
#
# See https://github.com/containers/podman/issues/3795
# bats test_tags=ci:parallel
@test "podman rm -f" {
    cname=c-$(safename)
    ( sleep 3; run_podman rm -t 0 -f $cname ) &
    run_podman 137 run --name $cname $IMAGE sleep 30
}

# bats test_tags=ci:parallel
@test "podman container rm --force bogus" {
    run_podman 1 container rm bogus-$(safename)
    is "$output" "Error: no container with ID or name \"bogus-$(safename)\" found: no such container" "Should print error"
    run_podman container rm --force bogus-$(safename)
    is "$output" "" "Should print no output"

    run_podman create --name testctr-$(safename) $IMAGE
    run_podman container rm --force bogus-$(safename) testctr-$(safename)
    assert "$output" = "testctr-$(safename)" "should delete test"

    run_podman ps -a -q
    assert "$output" !~ "$(safename)" "container should be removed"
}

# DO NOT CHANGE "sleep infinity"! This is how we get a container to
# remain in state "stopping" for long enough to check it.
# $1 = container name, $2 = optional stop-timeout (default 2)
function __run_healthcheck_container() {
    local stop_timeout=${2:-2}
    run_podman run -d --name $1 \
               --health-cmd /bin/false \
               --health-interval 1s \
               --health-retries 2 \
               --health-timeout 1s \
               --health-on-failure=stop \
               --stop-timeout=$stop_timeout \
               --health-start-period 0 \
               --stop-signal SIGTERM \
               $IMAGE sleep infinity
}

# bats test_tags=ci:parallel
@test "podman container rm doesn't affect stopping containers" {
    local cname=c-$(safename)
    __run_healthcheck_container $cname
    local cid=$output

    # We'll use the PID later to confirm that container is not running
    run_podman inspect --format '{{.State.Pid}}' $cname
    local pid=$output

    # rm without -f should fail, because container is running/stopping.
    # We have no way to guarantee that we see 'stopping', but at a very
    # minimum we need to check at least one rm failure
    local rm_failures=0
    for i in {1..20}; do
        run_podman '?' rm $cname
        if [[ $status -eq 0 ]]; then
            break
        fi

        # rm failed. Confirm that it's for the right reason.
        assert "$output" =~ "Error: cannot remove container $cid as it is .* - running or paused containers cannot be removed without force: container state improper" \
               "Expected error message from podman rm"
        rm_failures=$((rm_failures + 1))
        sleep 0.5
    done

    # At this point, container should be gone
    run_podman 1 container exists $cname
    run_podman 1 container exists $cid

    assert "$rm_failures" -gt 0 "we want at least one failure from podman-rm"

    if kill -0 $pid; then
        die "Container $cname process is still running (pid $pid)"
    fi
}

# bats test_tags=ci:parallel
@test "podman container rm --force doesn't leave running processes" {
    local cname=c-$(safename)
    __run_healthcheck_container $cname 20
    local cid=$output

    # We'll use the PID later to confirm that container is not running
    run_podman inspect --format '{{.State.Pid}}' $cname
    local pid=$output

    for i in {1..20}; do
        run_podman inspect $cname --format '{{.State.Status}}'
        if [ "$output" = "stopping" ]; then
            run_podman rm -f $cname
            if kill -0 $pid; then
                die "Container $cname process is still running (pid $pid)"
            fi
            return
        fi

        sleep 0.5
    done

    die "Container never entered 'stopping' state"
}

# bats test_tags=ci:parallel
@test "podman rm after killed conmon" {
    cname=c_$(safename)
    run_podman run -d --name $cname $IMAGE sleep 1000

    run_podman inspect --format '{{ .State.ConmonPid }}' $cname
    conmon_pid=$output

    kill -9 ${conmon_pid}

    # When conmon is killed, the ns_handles may become stale and produce a warning
    # about creating a new rootless user namespace. This is expected behavior.
    run_podman 0+w rm -f -t0 $cname

    run_podman 125 container inspect $cname
    assert "$output" =~ "no such container \"$cname\"" "Container should be removed"
}

# vim: filetype=sh
