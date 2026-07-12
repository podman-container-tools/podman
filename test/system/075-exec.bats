#!/usr/bin/env bats   -*- bats -*-
#
# Tests for podman exec
#

load helpers

# bats test_tags=ci:parallel
@test "podman exec - basic test" {
    rand_filename=$(random_string 20)
    rand_content=$(random_string 50)

    # Start a container. Write random content to random file, then stay
    # alive as long as file exists. (This test will remove that file soon.)
    run_podman run -d $IMAGE sh -c \
               "echo $rand_content >/$rand_filename;echo READY;while [ -f /$rand_filename ]; do sleep 1; done"
    cid="$output"
    wait_for_ready $cid

    run_podman exec $cid sh -c "cat /$rand_filename"
    is "$output" "$rand_content" "Can exec and see file in running container"


    # Specially defined situations: exec a dir, or no such command.
    # We don't check the full error message because runc & crun differ.
    #
    # UPDATE 2023-07-17 runc on RHEL8 (but not Debian) now says "is a dir"
    # and exits 255 instead of 126 as it does everywhere else.
    run_podman '?' exec $cid /etc
    is "$output" ".*\(permission denied\|is a directory\)"  \
       "podman exec /etc"
    assert "$status" -ne 0 "exit status from 'exec /etc'"
    run_podman 127 exec $cid /no/such/command
    is "$output" ".*such file or dir"   "podman exec /no/such/command"

    run_podman 125 exec $cid
    is "$output" ".*must provide a non-empty command to start an exec session"   "podman exec must include a command"

    # Done. Tell the container to stop.
    # The '-d' is because container exit is racy: the exec process itself
    # could get caught and killed by cleanup, causing this step to exit 137
    run_podman exec -d $cid rm -f /$rand_filename

    run_podman wait $cid
    is "$output" "0"   "output from podman wait (container exit code)"

    run_podman rm $cid
}

@test "podman exec - leak check" {
    skip_if_remote "test is meaningless over remote"

    # Start a container in the background then run exec command
    # three times and make sure no any exec pid hash file leak
    run_podman run -td $IMAGE /bin/sh
    cid="$output"

    is "$(find_exec_pid_files)" "" "exec pid hash file indeed doesn't exist"

    for i in {1..3}; do
        run_podman exec $cid /bin/true
    done

    is "$(find_exec_pid_files)" "" "there isn't any exec pid hash file leak"

    # Ensure file is there while container is running
    run_podman exec -d $cid sleep 100
    is "$(find_exec_pid_files)" '.*containers.*exec_pid' "exec_pid file found"

    run_podman rm -t 0 -f $cid
}

# Issue #4785 - piping to exec statement - fixed in #4818
# Issue #5046 - piping to exec truncates results (actually a conmon issue)
# bats test_tags=ci:parallel
@test "podman exec - cat from stdin" {
    run_podman run -d $IMAGE top
    cid="$output"

    echo_string=$(random_string 20)
    run_podman exec -i $cid cat < <(echo $echo_string)
    is "$output" "$echo_string" "output read back from 'exec cat'"

    # #5046 - large file content gets lost via exec
    # Generate a large file with random content; get a hash of its content
    local bigfile=${PODMAN_TMPDIR}/bigfile
    dd if=/dev/urandom of=$bigfile bs=1024 count=1500
    expect=$(sha512sum $bigfile | awk '{print $1}')
    # Transfer it to container, via exec, make sure correct #bytes are sent
    run_podman exec -i $cid dd of=/tmp/bigfile bs=512 <$bigfile
    is "${lines[0]}" "3000+0 records in"  "dd: number of records in"
    is "${lines[1]}" "3000+0 records out" "dd: number of records out"
    # Verify sha. '% *' strips off the path, keeping only the SHA
    run_podman exec $cid sha512sum /tmp/bigfile
    is "${output% *}" "$expect " "SHA of file in container"

    # Clean up
    run_podman rm -f -t0 $cid
}

# #6829 : add username to /etc/passwd inside container if --userns=keep-id
# bats test_tags=ci:parallel
@test "podman exec - with keep-id" {
    skip_if_not_rootless "--userns=keep-id only works in rootless mode"
    # Multiple --userns options confirm command-line override (last one wins)
    run_podman run -d --userns=private --userns=keep-id $IMAGE sh -c 'echo READY;top'
    cid="$output"
    wait_for_ready $cid

    run_podman exec $cid id -un
    is "$output" "$(id -un)" "container is running as current user"

    run_podman rm -f -t0 $cid
}

# #11496: podman-remote loses output
# bats test_tags=ci:parallel
@test "podman exec/run - missing output" {
    local bigfile=${PODMAN_TMPDIR}/bigfile
    local newfile=${PODMAN_TMPDIR}/newfile
    # create a big file, bigger than the 8K buffer size
    base64 /dev/urandom | head -c 20K > $bigfile

    run_podman run --rm -v $bigfile:/tmp/test:Z $IMAGE cat /tmp/test
    printf "%s" "$output" > $newfile
    # use cmp to compare the files, this is very helpful since it will
    # tell us the first wrong byte in case this fails
    run cmp $bigfile $newfile
    is "$output" "" "run output is identical with the file"

    run_podman run -d --stop-timeout 0 -v $bigfile:/tmp/test:Z $IMAGE sleep inf
    cid="$output"

    run_podman exec $cid cat /tmp/test
    printf "%s" "$output" > $newfile
    # use cmp to compare the files, this is very helpful since it will
    # tell us the first wrong byte in case this fails
    run cmp $bigfile $newfile
    is "$output" "" "exec output is identical with the file"

    # Clean up
    run_podman rm -t 0 -f $cid
}

# bats test_tags=ci:parallel
@test "podman run umask" {
    umask="0724"
    run_podman run --rm -q $IMAGE grep Umask /proc/self/status
    is "$output" "Umask:.*0022" "default_umask should not be modified"

    run_podman run -q --rm --umask $umask $IMAGE grep Umask /proc/self/status
    is "$output" "Umask:.*$umask" "umask should be modified"

    # FIXME: even in December 2023, exec test fails with Debian runc (1.1.10).
    # And even if we some day get a fixed version on Debian, these tests have
    # to pass on RHEL, and we have no control over runc version there.
    if [[ "$(podman_runtime)" == "runc" ]]; then
        echo "# Passed run test; skipping exec because runtime != crun" >&3
        return
    fi

    run_podman run -q -d --umask $umask $IMAGE sleep inf
    cid=$output
    run_podman exec $cid grep Umask /proc/self/status
    is "$output" "Umask:.*$umask" "exec umask should match container umask"
    run_podman exec $cid sh -c "touch /foo; stat -c '%a' /foo"
    is "$output" "42" "umask should apply to newly created file"

    run_podman rm -f -t0 $cid
}

# bats test_tags=ci:parallel
@test "podman exec --tty" {
    # Run all tests, report failures at end
    defer-assertion-failures

    # Outer loops: different variations on the RUN container
    for run_opt_t in "" "-t"; do
        for run_term_env in "" "explicit_RUN_term"; do
            local run_opt_env=
            if [[ -n "$run_term_env" ]]; then
                run_opt_env="--env=TERM=$run_term_env"
            fi
            cname="c-${run_opt_t}-${run_term_env}-$(safename)"
            run_podman run -d $run_opt_t $run_opt_env --name $cname $IMAGE top

            # Inner loops: different variations on EXEC
            for exec_opt_t in "" "-t"; do
                for exec_term_env in "" "explicit_EXEC_term"; do
                    # What to expect.
                    local expected=
                    # if -t is set anywhere, either run or exec, go with xterm
                    if [[ -n "$run_opt_t$exec_opt_t" ]]; then
                        expected="xterm"
                    fi
                    # ...unless overridden by explicit --env
                    if [[ -n "$run_term_env$exec_term_env" ]]; then
                        # (exec overrides run)
                        expected="${exec_term_env:-$run_term_env}"
                    fi

                    local exec_opt_env=
                    if [[ -n "$exec_term_env" ]]; then
                        exec_opt_env="--env=TERM=$exec_term_env"
                    fi

                    local desc="run $run_opt_t $run_opt_env, exec $exec_opt_t $exec_opt_env"
                    TERM=exec-term run_podman exec $exec_opt_t $exec_opt_env $cname sh -c 'echo -n $TERM'
                    assert "$output" = "$expected" "$desc"
                done
            done

            run_podman rm -f -t0 $cname
        done
    done
}

# bats test_tags=ci:parallel
@test "podman exec - does not leak session IDs on invalid command" {
    run_podman run -d $IMAGE top
    cid="$output"

    for i in {1..3}; do
        run_podman 127 exec $cid blahblah
        run_podman 125 exec -d $cid blahblah
    done

    run_podman inspect --format "{{len .ExecIDs}}" $cid
    assert "$output" = "0" ".ExecIDs must be empty"

    run_podman rm -f -t0 $cid
}

# 'exec --preserve-fd' passes a list of additional file descriptors into the container
# bats test_tags=ci:parallel
@test "podman exec --preserve-fd" {
    skip_if_remote "preserve-fd is meaningless over remote"

    runtime=$(podman_runtime)
    if [[ $runtime != "crun" ]]; then
        skip "runtime is $runtime; preserve-fd requires crun"
    fi

    run_podman run -d $IMAGE top
    cid="$output"

    content=$(random_string 20)
    echo "$content" > $PODMAN_TMPDIR/tempfile

    # /proc/self/fd will have 0 1 2, possibly 3 & 4, but no 2-digit fds other than 40
    run_podman exec --preserve-fd=9,40 $cid sh -c '/bin/ls -C -w999 /proc/self/fd; cat <&9; cat <&40' 9<<<"fd9" 10</dev/null 40<$PODMAN_TMPDIR/tempfile
    assert "${lines[0]}" !~ [123][0-9] "/proc/self/fd must not contain 10-39"
    assert "${lines[1]}" = "fd9"       "cat from fd 9"
    assert "${lines[2]}" = "$content"  "cat from fd 40"

    run_podman rm -f -t0 $cid
}

# bats test_tags=ci:parallel
@test "podman exec - additional groups" {
    run_podman run -d $IMAGE top
    cid="$output"

    run_podman exec $cid id -g nobody
    nobody_id="$output"

    run_podman exec $cid grep -h ^Groups: /proc/1/status /proc/self/status
    assert "${lines[0]}" = "${lines[1]}" "must have the same additional groups"

    run_podman exec --user root $cid grep -h ^Groups: /proc/1/status /proc/self/status
    assert "${lines[0]}" = "${lines[1]}" "must have the same additional groups"

    run_podman exec --user root:root $cid id -G
    assert "${output}" = "0" "must have only 0 gid"

    run_podman exec --user nobody $cid id -G
    assert "${output}" = "${nobody_id}" "must have only nobody gid"

    run_podman exec --user nobody:nobody $cid id -G
    assert "${output}" = "${nobody_id}" "must have only nobody gid"

    run_podman rm -f -t0 $cid

    # Now test with --group-add

    run_podman run --group-add 1,2,3,4,5,6,7,8,9,10 -d $IMAGE top
    cid="$output"

    run_podman exec $cid grep -h ^Groups: /proc/1/status /proc/self/status
    assert "${lines[0]}" = "${lines[1]}" "must have the same additional groups"

    run_podman exec --user 0 $cid grep -h ^Groups: /proc/1/status /proc/self/status
    assert "${lines[0]}" = "${lines[1]}" "must have the same additional groups"

    run_podman exec --user root $cid grep -h ^Groups: /proc/1/status /proc/self/status
    assert "${lines[0]}" = "${lines[1]}" "must have the same additional groups"

    run_podman exec --user root:root $cid id -G
    assert "$output" = "0 1 2 3 4 5 6 7 8 9 10" "must have only the explicit groups added and 0"

    run_podman exec --user 0:0 $cid id -G
    assert "$output" = "0 1 2 3 4 5 6 7 8 9 10" "must have only the explicit groups added and 0"

    run_podman exec --user nobody $cid id -G
    assert "$output" = "$nobody_id 1 2 3 4 5 6 7 8 9 10" "must have only the explicit groups added and nobody"

    run_podman exec --user nobody:nobody $cid id -G
    assert "$output" = "$nobody_id 1 2 3 4 5 6 7 8 9 10" "must have only the explicit groups added and nobody"

    run_podman exec --user root:nobody $cid id -G
    assert "$output" = "$nobody_id 1 2 3 4 5 6 7 8 9 10" "must have only the explicit groups added and 0"

    run_podman rm -f -t0 $cid
}
# vim: filetype=sh
