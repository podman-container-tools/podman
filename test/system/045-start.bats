#!/usr/bin/env bats   -*- bats -*-

load helpers

@test "podman start --all - start all containers" {
    # Run a bunch of short-lived containers, with different --restart settings
    run_podman run -d $IMAGE /bin/true
    cid_none_implicit="$output"
    run_podman run -d --restart=no $IMAGE /bin/false
    cid_none_explicit="$output"
    run_podman run -d --restart=on-failure $IMAGE /bin/true
    cid_on_failure="$output"

    # Run one longer-lived one.
    run_podman run -d --restart=always $IMAGE sleep 20
    cid_always="$output"

    run_podman wait $cid_none_implicit $cid_none_explicit $cid_on_failure

    run_podman start --all
    is "$output" ".*$cid_none_implicit" "started: container with no --restart"
    is "$output" ".*$cid_none_explicit" "started: container with --restart=no"
    is "$output" ".*$cid_on_failure" "started: container with --restart=on-failure"
    assert "$output" !~ "$cid_always" \
           "podman start --all should not restart a running container"

    run_podman wait $cid_none_implicit $cid_none_explicit $cid_on_failure

    run_podman rm $cid_none_implicit $cid_none_explicit $cid_on_failure
    run_podman 0+w stop -t 1 $cid_always
    if ! is_remote; then
        require_warning "StopSignal SIGTERM failed to stop container .*, resorting to SIGKILL"
    fi
    run_podman rm $cid_always
}

@test "podman start --all with incompatible options" {
    expected="Error: either start all containers or the container(s) provided in the arguments"
    run_podman 125 start --all 12333
    is "$output" "$expected" "start --all, with args, throws error"
}

@test "podman start --filter - start only containers that match the filter" {
    c1="c1_always_$(random_string 15)"
    c2="c2_on_failure_$(random_string 15)"
    c3="c3_always_$(random_string 15)"

    run_podman create --name=$c1 --restart=always $IMAGE /bin/true
    c1_id="$output"
    run_podman create --name=$c2 --restart=on-failure $IMAGE /bin/true
    c2_id="$output"
    run_podman create --name=$c3 --restart=always $IMAGE /bin/true
    c3_id="$output"

    # Start via --filter
    run_podman start --filter restart-policy=always
    # Output order not sorted wrt creation time, so we need two regexes
    is "$output" ".*$c1_id.*" "--filter finds container 1"
    is "$output" ".*$c3_id.*" "--filter finds container 3"

    # start again, before this fix it could panic
    run_podman start --filter restart-policy=always

    # Start via filtered names
    run_podman start --filter restart-policy=on-failure $c2 $c3
    is "$output" "$c2" "--filter finds container 2"

    # Nothing on match
    run_podman start --filter restart-policy=none --all
    is "$output" ""

    run_podman rm -f $c1 $c2 $c3
}

@test "podman start --filter invalid-restart-policy - return error" {
    run_podman run -d $IMAGE /bin/true
    cid="$output"
    run_podman 125 start --filter restart-policy=fakepolicy $cid
    is "$output" "Error: fakepolicy invalid restart policy" \
       "CID of restart-policy=<not-exists> container"
    run_podman rm -f $cid
}

@test "podman start --all --filter" {
    run_podman run -d $IMAGE /bin/true
    cid_exited_0="$output"
    run_podman run -d $IMAGE /bin/false
    cid_exited_1="$output"

    run_podman wait $cid_exited_0 $cid_exited_1
    run_podman start --all --filter exited=0
    is "$output" "$cid_exited_0"

    run_podman rm -f $cid_exited_0 $cid_exited_1
}

@test "podman start print IDs or raw input" {
    # start --all must print the IDs
    run_podman create $IMAGE top
    ctrID="$output"
    run_podman start --all
    is "$output" "$ctrID"

    # start $input must print $input
    cname=$(random_string)
    run_podman create --name $cname $IMAGE top
    run_podman start $cname
    is "$output" $cname

    run_podman rm -t 0 -f $ctrID $cname
}

@test "podman start again with lower ulimit -u" {
    skip_if_not_rootless "tests ulimit -u changes in the rootless scenario"
    skip_if_remote "test relies on control of ulimit -u (not possible for remote)"
    # get current ulimit -u
    nrpoc_limit=$(ulimit -Hu)

    # create container
    run_podman create $IMAGE echo "hello"
    ctrID="$output"

    # inspect
    run_podman inspect $ctrID
    assert "$output" =~ '"Ulimits": \[\]' "Ulimits has to be empty after create"

    # start container for the first time
    run_podman start $ctrID
    is "$output" "$ctrID"

    # inspect
    run_podman inspect $ctrID --format '{{range .HostConfig.Ulimits}}{{if eq .Name "RLIMIT_NPROC" }}{{.Soft}}:{{.Hard}}{{end}}{{end}}'
    assert "$output" == "${nrpoc_limit}:${nrpoc_limit}" "Ulimit has to match ulimit -Hu"

    # lower ulimit -u by one
    ((nrpoc_limit--))

    # set new ulimit -u
    ulimit -u $nrpoc_limit

    # start container for the second time
    run_podman start $ctrID
    is "$output" "$ctrID"

    # inspect
    run_podman inspect $ctrID --format '{{range .HostConfig.Ulimits}}{{if eq .Name "RLIMIT_NPROC" }}{{.Soft}}:{{.Hard}}{{end}}{{end}}'
    assert "$output" == "${nrpoc_limit}:${nrpoc_limit}" "Ulimit has to match new ulimit -Hu"

    run_podman rm -t 0 -f $ctrID $cname
}

# Regression test for https://github.com/containers/podman/issues/25965
# bats test_tags=ci:parallel
@test "podman start attach with created --rm container" {
    local msg=c-$(safename)
    run_podman create --rm $IMAGE echo "$msg"
    cid="$output"

    run_podman start -a $cid
    assert "$output" == "$msg" "attach printed the expected output"

    # container must be removed on its own as it was created with --rm
    run_podman 1 container exists $cid
}

# vim: filetype=sh
