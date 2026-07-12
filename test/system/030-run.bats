#!/usr/bin/env bats

load helpers
load helpers.network
load helpers.registry

# bats test_tags=ci:parallel
@test "podman run - basic tests" {
    rand=$(random_string 30)

    err_no_such_cmd="Error:.*/no/such/command.*[Nn]o such file or directory"
    # runc: RHEL8 on 2023-07-17: "is a directory".
    # Everything else (crun; runc on debian): "permission denied"
    err_no_exec_dir="Error:.*\\\(exec.*\\\(permission denied\\\|is a directory\\\)\\\|is not a regular file\\\)"

    tests="
true              |   0 |
false             |   1 |
sh -c 'exit 32'   |  32 |
echo $rand        |   0 | $rand
/no/such/command  | 127 | $err_no_such_cmd
/etc              | 126 | $err_no_exec_dir
"

    defer-assertion-failures

    tests_run=0
    while read cmd expected_rc expected_output; do
        if [ "$expected_output" = "''" ]; then expected_output=""; fi

        # THIS IS TRICKY: this is what lets us handle a quoted command.
        # Without this incantation (and the "$@" below), the cmd string
        # gets passed on as individual tokens: eg "sh" "-c" "'exit" "32'"
        # (note unmatched opening and closing single-quotes in the last 2).
        # That results in a bizarre and hard-to-understand failure
        # in the BATS 'run' invocation.
        # This should really be done inside parse_table; I can't find
        # a way to do so.
        eval set "$cmd"

        run_podman $expected_rc run --rm $IMAGE "$@"
        is "$output" "$expected_output" "podman run $cmd - output"

        tests_run=$(expr $tests_run + 1)
    done < <(parse_table "$tests")

    # Make sure we ran the expected number of tests! Until 2019-09-24
    # podman-remote was only running one test (the "true" one); all
    # the rest were being silently ignored because of podman-remote
    # bug #4095, in which it slurps up stdin.
    is "$tests_run" "$(grep . <<<$tests | wc -l)" "Ran the full set of tests"
}

# bats test_tags=ci:parallel
@test "podman run - global runtime option" {
    skip_if_remote "runtime flag is not passed over remote"
    run_podman 126 --runtime-flag invalidflag run --rm $IMAGE
    is "$output" ".*invalidflag" "failed when passing undefined flags to the runtime"
}

# bats test_tags=ci:parallel
@test "podman run - containers.conf runtime options" {
    skip_if_remote "requires local containers.conf"

    containersConf=$PODMAN_TMPDIR/containers.conf
    cat >$containersConf <<EOF
[engine]
runtime="$(podman_runtime)"

[engine.runtimes_flags]
$(podman_runtime) = [
  "invalidflag",
]
EOF

    CONTAINERS_CONF="$containersConf" run_podman 126 run --rm $IMAGE
    is "$output" ".*invalidflag" "failed when passing undefined flags to the runtime"
}

# bats test_tags=ci:parallel
@test "podman run --memory=0 runtime option" {
    run_podman run --memory=0 --rm $IMAGE echo hello
    if is_rootless && ! is_cgroupsv2; then
        is "${lines[0]}" "Resource limits are not supported and ignored on cgroups V1 rootless systems" "--memory is not supported"
        is "${lines[1]}" "hello" "--memory is ignored"
    else
        is "$output" "hello" "failed to run when --memory is set to 0"
    fi
}

# 'run --preserve-fds' passes a number of additional file descriptors into the container
# bats test_tags=ci:parallel
@test "podman run --preserve-fds" {
    skip_if_remote "preserve-fds is meaningless over remote"

    content=$(random_string 20)
    echo "$content" > $PODMAN_TMPDIR/tempfile

    run_podman run --rm -i --preserve-fds=2 $IMAGE sh -c "cat <&4" 4<$PODMAN_TMPDIR/tempfile
    assert "$output" =~ "$content" "container read input from fd 4"
}

# 'run --preserve-fd' passes a list of additional file descriptors into the container
# bats test_tags=ci:parallel
@test "podman run --preserve-fd" {
    skip_if_remote "preserve-fd is meaningless over remote"

    runtime=$(podman_runtime)
    if [[ $runtime != "crun" ]]; then
        skip "runtime is $runtime; preserve-fd requires crun"
    fi

    content=$(random_string 20)
    echo "$content" > $PODMAN_TMPDIR/tempfile

    # /proc/self/fd will have 0 1 2, possibly 3 & 4, but no 2-digit fds other than 40
    run_podman run --rm -i --preserve-fd=9,40 $IMAGE sh -c '/bin/ls -C -w999 /proc/self/fd; cat <&9; cat <&40' 9<<<"fd9" 10</dev/null 40<$PODMAN_TMPDIR/tempfile
    assert "${lines[0]}" !~ [123][0-9] "/proc/self/fd must not contain 10-39"
    assert "${lines[1]}" = "fd9"       "cat from fd 9"
    assert "${lines[2]}" = "$content"  "cat from fd 40"
}

# bats test_tags=ci:parallel
@test "podman run - uidmapping has no /sys/kernel mounts" {
    skip_if_rootless "cannot umount as rootless"

    run_podman run --rm --uidmap 0:100:10000 $IMAGE mount
    assert "$output" !~ /sys/kernel "unwanted /sys/kernel in 'mount' output"

    run_podman run --rm --net host --uidmap 0:100:10000 $IMAGE mount
    assert "$output" !~ /sys/kernel \
           "unwanted /sys/kernel in 'mount' output (with --net=host)"
}

# 'run --rm' goes through different code paths and may lose exit status.
# See https://github.com/containers/podman/issues/3795
# bats test_tags=ci:parallel
@test "podman run --rm" {

    run_podman 0 run --rm $IMAGE /bin/true
    run_podman 1 run --rm $IMAGE /bin/false

    # Believe it or not, 'sh -c' resulted in different behavior
    run_podman 0 run --rm $IMAGE sh -c /bin/true
    run_podman 1 run --rm $IMAGE sh -c /bin/false
}

# bats test_tags=ci:parallel
@test "podman run --name" {
    randomname=c_$(safename)

    run_podman run -d --name $randomname $IMAGE sleep inf
    cid=$output

    run_podman ps --format '{{.Names}}--{{.ID}}'
    assert "$output" =~ "$randomname--${cid:0:12}"

    run_podman container exists $randomname
    run_podman container exists $cid

    # Done with live-container tests; now let's test after container finishes
    run_podman stop -t0 $cid

    # Container still exists even after stopping:
    run_podman container exists $randomname
    run_podman container exists $cid

    # ...but not after being removed:
    run_podman rm $cid
    run_podman 1 container exists $randomname
    run_podman 1 container exists $cid
}

# not parallelizable due to podman rm -a at end
@test "podman run --pull" {
    run_podman run --pull=missing $IMAGE true
    is "$output" "" "--pull=missing [present]: no output"

    run_podman run --pull=never $IMAGE true
    is "$output" "" "--pull=never [present]: no output"

    # Now test with a remote image which we don't have present (the 00 tag)
    NONLOCAL_IMAGE="$PODMAN_NONLOCAL_IMAGE_FQN"

    run_podman 125 run --pull=never $NONLOCAL_IMAGE true
    is "$output" "Error: $NONLOCAL_IMAGE: image not known" "--pull=never [with image not present]: error"

    run_podman run --pull=missing $NONLOCAL_IMAGE true
    is "$output" "Trying to pull .*" "--pull=missing [with image NOT PRESENT]: fetches"

    run_podman run --pull=missing $NONLOCAL_IMAGE true
    is "$output" "" "--pull=missing [with image PRESENT]: does not re-fetch"

    run_podman run --pull=always $NONLOCAL_IMAGE true
    is "$output" "Trying to pull .*" "--pull=always [with image PRESENT]: re-fetches"

    # NOTE: older version of podman would match "foo" against "myfoo". That
    # behaviour was changed with introduction of `containers/common/libimage`
    # which will only match at repository boundaries (/).
    run_podman 125 run --pull=never my$PODMAN_TEST_IMAGE_NAME true
    is "$output" "Error: my$PODMAN_TEST_IMAGE_NAME: image not known" \
       "podman run --pull=never with shortname (and implicit :latest)"

    # ...but if we add a :latest tag (without 'my'), it should now work
    run_podman tag $IMAGE ${PODMAN_TEST_IMAGE_NAME}:latest
    run_podman run --pull=never ${PODMAN_TEST_IMAGE_NAME} cat /home/podman/testimage-id
    is "$output" "$PODMAN_TEST_IMAGE_TAG" \
       "podman run --pull=never, with shortname, succeeds if img is present"

    run_podman rm -a
    run_podman rmi $NONLOCAL_IMAGE ${PODMAN_TEST_IMAGE_NAME}:latest
}

# 'run --rmi' deletes the image in the end unless it's used by another container
# CANNOT BE PARALLELIZED because other tests may use $NONLOCAL_IMAGE
@test "podman run --rmi" {
    # Name of a nonlocal image. It should be pulled in by the first 'run'
    NONLOCAL_IMAGE="$PODMAN_NONLOCAL_IMAGE_FQN"
    run_podman 1 image exists $NONLOCAL_IMAGE

    # Run a container, without --rm; this should block subsequent --rmi
    cname=c_$(safename)
    run_podman run --name /$cname $NONLOCAL_IMAGE /bin/true
    run_podman image exists $NONLOCAL_IMAGE

    # Now try running with --rmi : it should succeed, but not remove the image
    run_podman 0+w run --rmi --rm $NONLOCAL_IMAGE /bin/true
    require_warning "image is in use by a container" \
                    "--rmi should warn that the image was not removed"
    run_podman image exists $NONLOCAL_IMAGE

    # Remove the stray container, and run one more time with --rmi.
    run_podman rm /$cname
    run_podman run --rmi $NONLOCAL_IMAGE /bin/true
    run_podman 1 image exists $NONLOCAL_IMAGE

    run_podman 125 run --rmi --rm=false $NONLOCAL_IMAGE /bin/true
    is "$output" "Error: the --rmi option does not work without --rm" "--rmi should refuse to remove images when --rm=false set by user"

    # Try again with a detached container and verify it works
    cname=c_$(safename)
    run_podman run -d --name $cname --rmi $NONLOCAL_IMAGE /bin/true
    # 10 chances for the image to disappear
    for i in `seq 1 10`; do
        sleep 0.5
        run_podman '?' image exists $NONLOCAL_IMAGE
        if [[ $status == 1 ]]; then
           break
        fi
    done
    # Final check will error if image still exists
    run_podman 1 image exists $NONLOCAL_IMAGE

    # Verify that the inspect annotation is set correctly
    run_podman run -d --name $cname --rmi $NONLOCAL_IMAGE sleep 10
    run_podman inspect --format '{{ .HostConfig.AutoRemoveImage }} {{ .HostConfig.AutoRemove }}' $cname
    is "$output" "true true" "Inspect correctly shows image autoremove and normal autoremove"
    run_podman stop -t0 $cname
    run_podman 1 image exists $NONLOCAL_IMAGE
}

# 'run --conmon-pidfile --cid-file' makes sure we don't regress on these flags.
# Both are critical for systemd units.
# bats test_tags=ci:parallel
@test "podman run --conmon-pidfile --cidfile" {
    pidfile=${PODMAN_TMPDIR}/pidfile
    cidfile=${PODMAN_TMPDIR}/cidfile

    # Write random content to the cidfile to make sure its content is truncated
    # on write.
    echo "$(random_string 120)" > $cidfile

    cname=c_$(safename)
    run_podman run --name $cname \
               --conmon-pidfile=$pidfile \
               --cidfile=$cidfile \
               --detach \
               $IMAGE sleep infinity
    cid="$output"

    is "$(< $cidfile)" "$cid" "contents of cidfile == container ID"

    # Cross-check --conmon-pidfile against 'podman inspect'
    local conmon_pid_from_file=$(< $pidfile)
    run_podman inspect --format '{{.State.ConmonPid}}' $cid
    local conmon_pid_from_inspect="$output"
    is "$conmon_pid_from_file" "$conmon_pid_from_inspect" \
       "Conmon pid in pidfile matches what 'podman inspect' claims"

    # /proc/PID/exe should be a symlink to a conmon executable
    # FIXME: 'echo' and 'ls' are to help debug #7580, a CI flake
    echo "conmon pid = $conmon_pid_from_file"
    ls -l /proc/$conmon_pid_from_file
    is "$(readlink /proc/$conmon_pid_from_file/exe)" ".*/conmon"  \
       "conmon pidfile (= PID $conmon_pid_from_file) points to conmon process"

    # All OK. Kill container.
    run_podman rm -f -t0 $cid
    if [[ -e $cidfile ]]; then
        die "cidfile $cidfile should be removed along with container"
    fi
}

# bats test_tags=ci:parallel
@test "podman run docker-archive" {
    skip_if_remote "podman-remote does not support docker-archive"

    # Create an image that, when run, outputs a random magic string
    cname=c_$(safename)
    expect=$(random_string 20)
    run_podman run --name $cname --entrypoint="[\"/bin/echo\",\"$expect\"]" $IMAGE
    is "$output" "$expect" "podman run --entrypoint echo-randomstring"

    # Save it as a tar archive
    iname=i_$(safename)
    run_podman commit $cname $iname
    archive=$PODMAN_TMPDIR/archive.tar
    run_podman save --quiet $iname -o $archive
    is "$output" "" "podman save"

    # Clean up image and container from container storage...
    run_podman rmi $iname
    run_podman rm $cname

    # ... then confirm we can run from archive. This re-imports the image
    # and runs it, producing our random string as the last line.
    run_podman run --name ${cname}_2 docker-archive:$archive
    is "${lines[0]}" "Getting image source signatures" "podman run docker-archive, first line of output"
    is "$output" ".*Copying blob"     "podman run docker-archive"
    is "$output" ".*Copying config"   "podman run docker-archive"
    is "$output" ".*Writing manifest" "podman run docker-archive"
    is "${lines[-1]}" "$expect" "podman run docker-archive: expected random string output"

    # Clean up container as well as re-imported image
    run_podman rm ${cname}_2
    run_podman rmi $iname

    # Repeat the above, with podman-create and podman-start.
    run_podman create docker-archive:$archive
    cid=${lines[-1]}

    run_podman start --attach $cid
    is "$output" "$expect" "'podman run' of 'podman-create docker-archive'"

    # Clean up.
    run_podman rm $cid
    run_podman rmi $iname
}

# #6735 : complex interactions with multiple user namespaces
# The initial report has to do with bind mounts, but that particular
# symptom only manifests on a fedora container image -- we have no
# reproducer on alpine. Checking directory ownership is good enough.
# bats test_tags=ci:parallel
@test "podman run : user namespace preserved root ownership" {
    keep="--userns=keep-id"
    is_rootless || keep=""
    for priv in "" "--privileged"; do
        for user in "--user=0" "--user=100"; do
            for keepid in "" ${keep}; do
                opts="$priv $user $keepid"
                run_podman run --rm $opts $IMAGE stat -c '%u:%g:%n' $dir /etc /usr
                remove_same_dev_warning      # grumble
                is "${lines[0]}" "0:0:/etc" "run $opts /etc"
                is "${lines[1]}" "0:0:/usr" "run $opts /usr"
            done
        done
    done
}

# #6829 : add username to /etc/passwd inside container if --userns=keep-id
# bats test_tags=ci:parallel
@test "podman run : add username to /etc/passwd if --userns=keep-id" {
    skip_if_not_rootless "--userns=keep-id only works in rootless mode"
    # Default: always run as root
    run_podman run --rm $IMAGE id -un
    is "$output" "root" "id -un on regular container"

    # This would always work on root, but is new behavior on rootless: #6829
    # adds a user entry to /etc/passwd
    whoami=$(id -un)
    run_podman run --rm --userns=keep-id $IMAGE id -un
    is "$output" "$whoami" "username on container with keep-id"

    # Setting user should also set $HOME (#8013).
    # Test setup below runs three cases: one with an existing home dir
    # and two without (one without any volume mounts, one with a misspelled
    # username). In every case, initial cwd should be /home/podman because
    # that's the container-defined WORKDIR. In the case of an existing
    # home dir, $HOME and ~ (passwd entry) will be /home/user; otherwise
    # they should be /home/podman.
    if is_rootless; then
        tests="
                |  /home/podman /home/podman /home/podman    | no vol mount
/home/x$whoami  |  /home/podman /home/podman /home/podman    | bad vol mount
$HOME           |  /home/podman $HOME $HOME                  | vol mount
"
        while read vol expect name; do
            opts=
            if [[ "$vol" != "''" ]]; then
                opts="-v $vol"
            fi
            run_podman run --rm $opts --userns=keep-id \
                   $IMAGE sh -c 'echo $(pwd;printenv HOME;echo ~)'
            is "$output" "$expect" "run with --userns=keep-id and $name sets \$HOME"
        done < <(parse_table "$tests")
    fi

    # --privileged should make no difference
    run_podman run --rm --privileged --userns=keep-id $IMAGE id -un
    remove_same_dev_warning      # grumble
    is "$output" "$(id -un)" "username on container with keep-id"

    # ...but explicitly setting --user should override keep-id
    run_podman run --rm --privileged --userns=keep-id --user=0 $IMAGE id -un
    remove_same_dev_warning      # grumble
    is "$output" "root" "--user=0 overrides keep-id"
}

# #6991 : /etc/passwd is modifiable
# bats test_tags=ci:parallel
@test "podman run : --userns=keep-id: passwd file is modifiable" {
    skip_if_not_rootless "--userns=keep-id only works in rootless mode"
    run_podman run -d --userns=keep-id --cap-add=dac_override $IMAGE top
    cid="$output"

    # Assign a UID that is (a) not in our image /etc/passwd and (b) not
    # the same as that of the user running the test script; this guarantees
    # that the added passwd entry will be what we expect.
    #
    # For GID, we have to use one that already exists in the container. And
    # unfortunately, 'adduser' requires a string name. We use 999:ping
    local uid=4242
    if [[ $uid == $(id -u) ]]; then
        uid=4343
    fi

    gecos="$(random_string 6) $(random_string 8)"
    run_podman exec --user root $cid adduser -u $uid -G ping -D -g "$gecos" -s /bin/sh newuser3
    is "$output" "" "output from adduser"
    run_podman exec $cid tail -1 /etc/passwd
    is "$output" "newuser3:x:$uid:999:$gecos:/home/newuser3:/bin/sh" \
       "newuser3 added to /etc/passwd in container"

    run_podman rm -f -t0 $cid
}

# For #7754: json-file was equating to 'none'
# bats test_tags=ci:parallel
@test "podman run --log-driver" {
    # '-' means that LogPath will be blank and there's no easy way to test
    tests="
none      | -
journald  | -
k8s-file  | y
json-file | f
"

    defer-assertion-failures

    while read driver do_check; do
        msg=$(random_string 15)
        cname=c_$(safename)
        run_podman run --name $cname --log-driver $driver $IMAGE echo $msg
        is "$output" "$msg" "basic output sanity check (driver=$driver)"

        # Simply confirm that podman preserved our argument as-is
        run_podman inspect --format '{{.HostConfig.LogConfig.Type}}' $cname
        is "$output" "$driver" "podman inspect: driver"

        # If LogPath is non-null, check that it exists and has a valid log
        run_podman inspect --format '{{.HostConfig.LogConfig.Path}}' $cname
        if [[ $do_check != '-' ]]; then
            is "$output" "/.*" "LogPath (driver=$driver)"
            if ! test -e "$output"; then
                die "LogPath (driver=$driver) does not exist: $output"
            fi
            # eg 2020-09-23T13:34:58.644824420-06:00 stdout F 7aiYtvrqFGJWpak
            is "$(< $output)" "[0-9T:.+-]\+ stdout F $msg" \
               "LogPath contents (driver=$driver)"
        else
            is "$output" "" "LogPath (driver=$driver)"
        fi

        if [[ $driver != 'none' ]]; then
            if [[ $driver = 'journald' ]] && journald_unavailable; then
                # Cannot perform check
                :
            else
                run_podman logs $cname
                is "$output" "$msg" "podman logs, with driver '$driver'"
            fi
        else
            run_podman 125 logs $cname
            if ! is_remote; then
                is "$output" ".*this container is using the 'none' log driver, cannot read logs.*" \
                   "podman logs, with driver 'none', should fail with error"
            fi
        fi
        run_podman rm $cname
    done < <(parse_table "$tests")

    # Invalid log-driver argument
    run_podman 125 run --log-driver=InvalidDriver $IMAGE true
    is "$output" "Error: running container create option: invalid log driver: invalid argument" \
       "--log-driver InvalidDriver"
}

# bats test_tags=ci:parallel
@test "podman run --log-driver journald" {
    skip_if_remote "We cannot read journalctl over remote."

    # We can't use journald on RHEL as rootless, either: rhbz#1895105
    skip_if_journald_unavailable

    msg=$(random_string 20)
    pidfile="${PODMAN_TMPDIR}/$(random_string 20)"

    # Multiple --log-driver options to confirm that last one wins
    cname=c_$(safename)
    run_podman run --name $cname --log-driver=none --log-driver journald \
               --conmon-pidfile $pidfile $IMAGE echo $msg

    journalctl --output cat  _PID=$(cat $pidfile)
    is "$output" "$msg" "check that journalctl output equals the container output"

    run_podman rm $cname
}

# bats test_tags=ci:parallel
@test "podman run --tz" {
    # This file will always have a constant reference timestamp
    local testfile=/home/podman/testimage-id

    run_podman run --rm $IMAGE date -r $testfile
    is "$output" "Sun Sep 13 12:26:40 UTC 2020" "podman run with no TZ"

    # Multiple --tz options; confirm that the last one wins
    run_podman run --rm --tz=US/Eastern --tz=Iceland --tz=America/New_York \
               $IMAGE date -r $testfile
    is "$output" "Sun Sep 13 08:26:40 EDT 2020" "podman run with --tz=America/New_York"

    # --tz=local pays attention to /etc/localtime, not $TZ. We set TZ anyway,
    # to make sure podman ignores it; and, because this test is locale-
    # dependent, we pick an obscure zone (+1245) that is unlikely to
    # collide with any of our testing environments.
    #
    # To get a reference timestamp we run 'date' locally. This requires
    # that GNU date output matches that of alpine; this seems to be true
    # as of testimage:20220615.
    run date --date=@1600000000 --iso=seconds
    expect="$output"
    TZ=Pacific/Chatham run_podman run --rm --tz=local $IMAGE date -Iseconds -r $testfile
    is "$output" "$expect" "podman run with --tz=local, matches host"

    # Force a TZDIR env as local should not try to use the TZDIR at all, #23550.
    # This used to fail with: stat /usr/share/zoneinfo/local: no such file or directory.
    TZDIR=/usr/share/zoneinfo run_podman run --rm --tz=local $IMAGE date -Iseconds -r $testfile
    is "$output" "$expect" "podman run with --tz=local ignored TZDIR"
}

# bats test_tags=ci:parallel
@test "podman run --tz with zoneinfo" {
    _prefetch $SYSTEMD_IMAGE

    # First make sure that zoneinfo is actually in the image otherwise the test is pointless
    run_podman run --rm $SYSTEMD_IMAGE ls /usr/share/zoneinfo

    run_podman run --rm --tz Europe/Berlin $SYSTEMD_IMAGE readlink /etc/localtime
    assert "$output" == "../usr/share/zoneinfo/Europe/Berlin" "localtime is linked correctly"
}

# run with --runtime should preserve the named runtime
# bats test_tags=ci:parallel
@test "podman run : full path to --runtime is preserved" {
    skip_if_remote "podman-remote does not support --runtime option"

    # Get configured runtime
    run_podman info --format '{{.Host.OCIRuntime.Path}}'
    runtime="$output"

    # Assumes that /var/tmp is not mounted noexec; this is usually safe
    new_runtime="/var/tmp/myruntime$(random_string 12)"
    cp --preserve $runtime $new_runtime

    run_podman run -d --runtime "$new_runtime" $IMAGE sleep 60
    cid="$output"

    run_podman inspect --format '{{.OCIRuntime}}' $cid
    is "$output" "$new_runtime" "podman inspect shows configured runtime"
    run_podman kill $cid
    run_podman wait $cid
    run_podman rm $cid
    rm -f $new_runtime
}

# bats test_tags=ci:parallel
@test "podman --noout run should not print output" {
    cname=c_$(safename)
    run_podman --noout run -d --name $cname $IMAGE echo hi
    is "$output" "" "output should be empty"
    run_podman wait $cname
    run_podman --noout rm $cname
    is "$output" "" "output should be empty"
}

# bats test_tags=ci:parallel
@test "podman --noout create should not print output" {
    cname=c_$(safename)
    run_podman --noout create --name $cname $IMAGE echo hi
    is "$output" "" "output should be empty"
    run_podman --noout rm $cname
    is "$output" "" "output should be empty"
}

# bats test_tags=ci:parallel
@test "podman --out run should save the container id" {
    outfile=${PODMAN_TMPDIR}/out-results

    # first we'll need to run something, write its output to a file, and then read its contents.
    cname=c_$(safename)
    run_podman --out $outfile run -d --name $cname $IMAGE echo hola
    is "$output" "" "output should be redirected"
    run_podman wait $cname

    # compare the container id against the one in the file
    run_podman container inspect --format '{{.Id}}' $cname
    is "$output" "$(<$outfile)" "container id should match"

    run_podman --out /dev/null rm $cname
    is "$output" "" "output should be empty"
}

# bats test_tags=ci:parallel
@test "podman --out create should save the container id" {
    outfile=${PODMAN_TMPDIR}/out-results

    # first we'll need to run something, write its output to a file, and then read its contents.
    cname=c_$(safename)
    run_podman --out $outfile create --name $cname $IMAGE echo hola
    is "$output" "" "output should be redirected"

    # compare the container id against the one in the file
    run_podman container inspect --format '{{.Id}}' $cname
    is "$output" "$(<$outfile)" "container id should match"

    run_podman --out /dev/null rm $cname
    is "$output" "" "output should be empty"
}

# Regression test for issue #8082
# bats test_tags=ci:parallel
@test "podman run : look up correct image name" {
    # Create a 2nd tag for the local image.
    local newtag="localhost/r_$(safename)/i_$(safename)"
    run_podman tag $IMAGE $newtag

    # Create a container with the 2nd tag and make sure that it's being
    # used.  #8082 always inaccurately used the 1st tag.
    run_podman create $newtag
    cid="$output"

    run_podman inspect --format "{{.ImageName}}" $cid
    is "$output" "$newtag:latest" \
       "container .ImageName is the container-create name"

    # Same thing, but now with a :tag, and making sure it works with --name
    newtag2="${newtag}:$(random_string 6|tr A-Z a-z)"
    run_podman tag $IMAGE $newtag2

    cname="c_$(safename)"
    run_podman create --name $cname $newtag2
    run_podman inspect --format "{{.ImageName}}" $cname
    is "$output" "$newtag2" \
       "container .ImageName is the container-create name, with :tag"

    # Clean up.
    run_podman rm $cid $cname
    run_podman untag $IMAGE $newtag $newtag2
}

# Regression test for issue #8558
# CANNOT BE PARALLELIZED: temporarily untags $IMAGE
@test "podman run on untagged image: make sure that image metadata is set" {
    run_podman inspect $IMAGE --format "{{.ID}}"
    imageID="$output"

    # prior to #8623 `podman run` would error out on untagged images with:
    # Error: both RootfsImageName and RootfsImageID must be set if either is set: invalid argument
    run_podman untag $IMAGE

    run_podman run --rm $randomname $imageID true
    run_podman tag $imageID $IMAGE
}

# bats test_tags=ci:parallel
@test "podman inspect includes image data" {
    randomname=c_$(safename)

    run_podman inspect $IMAGE --format "{{.ID}}"
    expected="$IMAGE $output"

    # .RepoDigests gives us an array of [quay.io/libpod/testimage@sha256:xxxx ...]
    # .ImageDigest (below) only gives us the sha256:xxxx part.
    run_podman inspect $IMAGE --format "{{json .RepoDigests}}"
    # ...so, strip off the prefix and get an array of sha strings
    declare -a expectedDigests=($(jq -r '.[]' <<<"$output" |\
                                      sed -e "s;^${PODMAN_TEST_IMAGE_REGISTRY}/${PODMAN_TEST_IMAGE_USER}/${PODMAN_TEST_IMAGE_NAME}@;;"))

    run_podman run --name $randomname $IMAGE true
    run_podman container inspect $randomname --format "{{.ImageName}} {{.Image}}"
    is "$output" "$expected"
    run_podman container inspect $randomname --format "{{.ImageDigest}}"
    local actualDigest="$output"

    local found=
    for digest in "${expectedDigests[@]}"; do
        echo "checking $digest"
        if [[ "$actualDigest" = "$digest" ]]; then
            found=1
            break
        fi
    done

    test -n "$found" || die "container .ImageDigest does not match any of image .RepoDigests"

    run_podman rm -f -t0 $randomname
}

# bats test_tags=ci:parallel
@test "Verify /run/.containerenv exist" {
    # Nonprivileged container: file exists, but must be empty
    for opt in "" "--tmpfs=/run" "--tmpfs=/run --init" "--read-only" "--systemd=always"; do
        run_podman run --rm $opt $IMAGE stat -c '%s' /run/.containerenv
        is "$output" "0" "/run/.containerenv exists and is empty: podman run ${opt}"
    done

    run_podman 1 run --rm -v ${PODMAN_TMPDIR}:/run:Z $IMAGE stat -c '%s' /run/.containerenv
    is "$output" "stat: can't stat '/run/.containerenv': No such file or directory" "do not create .containerenv on bind mounts"

    # Prep work: get ID of image; make a cont. name; determine if we're rootless
    run_podman inspect --format '{{.ID}}' $IMAGE
    local iid="$output"

    random_cname=c_$(safename)
    local rootless=0
    if is_rootless; then
        rootless=1
    fi

    run_podman run --privileged --rm --name $random_cname $IMAGE \
               sh -c '. /run/.containerenv; echo $engine; echo $name; echo $image; echo $id; echo $imageid; echo $rootless'

    # FIXME: on some CI systems, 'run --privileged' emits a spurious
    # warning line about dup devices. Ignore it.
    remove_same_dev_warning

    is "${lines[0]}" "podman-.*"      'containerenv : $engine'
    is "${lines[1]}" "$random_cname"  'containerenv : $name'
    is "${lines[2]}" "$IMAGE"         'containerenv : $image'
    is "${lines[3]}" "[0-9a-f]\{64\}" 'containerenv : $id'
    is "${lines[4]}" "$iid"           'containerenv : $imageid'
    is "${lines[5]}" "$rootless"      'containerenv : $rootless'
}

# bats test_tags=ci:parallel
@test "podman run with --net=host and --port prints warning" {
    rand=$(random_string 10)

    run_podman run --rm -p 8080 --net=host $IMAGE echo $rand
    is "${lines[0]}" \
       "Port mappings have been discarded because \"host\" network namespace mode does not support them" \
       "Warning is emitted before container output"
    is "${lines[1]}" "$rand" "Container runs successfully despite warning"
}

# bats test_tags=ci:parallel
@test "podman run with --net=none and --port prints warning" {
    rand=$(random_string 10)

    run_podman run --rm -p 8080 --net=none $IMAGE echo $rand
    is "${lines[0]}" \
       "Port mappings have been discarded because \"none\" network namespace mode does not support them" \
       "Warning is emitted before container output"
    is "${lines[1]}" "$rand" "Container runs successfully despite warning"
}

# bats test_tags=ci:parallel
@test "podman run with --net=container:id and --port prints warning" {
    rand=$(random_string 10)

    run_podman run -d --name=$rand $IMAGE top
    cid=$output
    run_podman run --rm -p 8080 --net=container:$cid $IMAGE echo $rand
    is "${lines[0]}" \
       "Port mappings have been discarded because \"container\" network namespace mode does not support them" \
       "Warning is emitted before container output"
    is "${lines[1]}" "$rand" "Container runs successfully despite warning"

    # Cleanup
    run_podman container rm -f -t0 $cid
}

# bats test_tags=ci:parallel
@test "podman run - check workdir" {
    # Workdirs specified via the CLI are not created on the root FS.
    run_podman 126 run --rm --workdir /i/do/not/exist $IMAGE pwd
    # Note: remote error prepends an attach error.
    is "$output" "Error: .*workdir \"/i/do/not/exist\" does not exist on container.*"

    testdir=$PODMAN_TMPDIR/volume
    mkdir -p $testdir
    randomcontent=$(random_string 10)
    echo "$randomcontent" > $testdir/content

    # Workdir does not exist on the image but is volume mounted.
    run_podman run --rm --workdir /IamNotOnTheImage -v $testdir:/IamNotOnTheImage:Z $IMAGE cat content
    is "$output" "$randomcontent" "cat random content"

    # Workdir does not exist on the image but is created by the runtime as it's
    # a subdir of a volume.
    run_podman run --rm --workdir /IamNotOntheImage -v $testdir/content:/IamNotOntheImage/foo:Z $IMAGE cat foo
    is "$output" "$randomcontent" "cat random content"

    # Make sure that running on a read-only rootfs works (#9230).
    if ! is_rootless && ! is_remote; then
        # image mount is hard to test as a rootless user
        # and does not work remotely
        run_podman image mount $IMAGE
        mountpoint="$output"

        rootfs=$PODMAN_TMPDIR/rootfs
        cp -a "$mountpoint" "$rootfs"
        # Make rootfs actually read only
        mount --bind -o ro "$rootfs" "$rootfs"

        run_podman image unmount $IMAGE

        randomname=c_$(safename)
        # :O (overlay) required with rootfs; see #14504
        run_podman run --name=$randomname --rootfs $rootfs:O echo "Hello world"
        is "$output" "Hello world"

        run_podman container inspect $randomname --format "{{.ImageDigest}}"
        is "$output" "" "Empty image digest for --rootfs container"

        umount "$rootfs"

        run_podman rm -f -t0 $randomname
    fi
}

# https://github.com/containers/podman/issues/9096
# podman exec may truncate stdout/stderr; actually a bug in conmon:
# https://github.com/containers/conmon/issues/236
# CANNOT BE PARALLELIZED due to "-l"
@test "podman run - does not truncate or hang with big output" {
    # Size, in bytes, to dd and to expect in return
    char_count=700000

    # Container name; primarily needed when running podman-remote
    cname=c_$(safename)

    # This is one of those cases where BATS is not the best test framework.
    # We can't do any output redirection, because 'run' overrides it so
    # as to preserve $output. We can't _not_ do redirection, because BATS
    # doesn't like NULs in $output (and neither would humans who might
    # have to read them in an error log).
    # Workaround: write to a log file, and don't attach stdout.
    run_podman run --name $cname --attach stderr --log-driver k8s-file \
               $IMAGE dd if=/dev/zero count=$char_count bs=1
    is "${lines[0]}" "$char_count+0 records in"  "dd: number of records in"
    is "${lines[1]}" "$char_count+0 records out" "dd: number of records out"

    # We don't have many tests for '-l'. This is as good a place as any
    local dash_l=-l
    if is_remote; then
        dash_l=$cname
    fi

    # Now find that log file, and count the NULs in it.
    # The log file is of the form '<timestamp> <P|F> <data>', where P|F
    # is Partial/Full; I think that's called "kubernetes log format"?
    run_podman inspect $dash_l --format '{{.HostConfig.LogConfig.Path}}'
    logfile="$output"

    count_zero=$(tr -cd '\0' <$logfile | wc -c)
    is "$count_zero" "$char_count" "count of NULL characters in log"

    # Clean up
    run_podman rm $cname
}

# bats test_tags=ci:parallel
@test "podman run - do not set empty HOME" {
    # Regression test for #9378.
    run_podman run --rm --user 100 $IMAGE printenv
    is "$output" ".*HOME=/.*"
}

# bats test_tags=ci:parallel
@test "podman run --timeout - basic test" {
    cname=c_$(safename)
    t0=$SECONDS
    run_podman 255 run --name $cname --timeout 2 $IMAGE sleep 60
    t1=$SECONDS
    # Confirm that container is stopped. Podman-remote unfortunately
    # cannot tell the difference between "stopped" and "exited", and
    # spits them out interchangeably, so we need to recognize either.
    run_podman inspect --format '{{.State.Status}} {{.State.ExitCode}}' $cname
    is "$output" "\\(stopped\|exited\\) \-1" \
       "Status and exit code of stopped container"

    # This operation should take
    # exactly 10 seconds. Give it some leeway.
    delta_t=$(( $t1 - $t0 ))
    assert "$delta_t" -gt 1 "podman stop: ran too quickly!"
    assert "$delta_t" -le 6 "podman stop: took too long"

    run_podman rm $cname
}

# bats test_tags=ci:parallel
@test "podman run no /etc/mtab " {
    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir

    cat >$tmpdir/Dockerfile <<EOF
FROM $IMAGE
RUN rm /etc/mtab
EOF
    expected="'/etc/mtab' -> '/proc/mounts'"

    # --layers=false needed to work around buildah#5674 parallel flake
    local iname=nomtab-$(safename)
    run_podman build -t $iname --layers=false $tmpdir
    run_podman run --rm $iname stat -c %N /etc/mtab
    is "$output" "$expected" "/etc/mtab should be created"

    run_podman rmi $iname
}

# bats test_tags=ci:parallel
@test "podman run --hostuser tests" {
    skip_if_not_rootless "test whether hostuser is successfully added"
    user=$(id -un)
    run_podman 1 run --rm $IMAGE grep $user /etc/passwd
    run_podman run --hostuser=$user --rm $IMAGE grep $user /etc/passwd

    # Issue #19800
    # Use the rootless user ID which is likely to be octal on CI as new user IDs start with 1000.
    userid=$(id -u)
    if [[ $userid =~ ^[0-7]+$ ]]; then
        run_podman run --user=$user --hostuser=$user --rm $IMAGE id -u
        is "$output" "$userid"
    fi

    group=$(id -gn)
    groupid=$(id -g)
    userspec=$user:$groupid
    run_podman run --hostuser=$userid --user $userspec --rm $IMAGE sh -c 'echo $(id -un):$(id -g)'
    is "$output" "$userspec"

    run_podman run --hostuser=$userid --user $userspec --group-entry="$group:x:$groupid:" --rm $IMAGE sh -c 'echo $(id -un):$(id -gn)'
    is "$output" "$user:$group"

    run_podman 126 run --hostuser=$userid --user "$user:$group" --rm $IMAGE sh -c 'echo $(id -un):$(id -gn)'
    is "$output" "Error:.* no matching entries in group file"

    run_podman run --hostuser=$userid --rm $IMAGE grep $userid /etc/passwd
    run_podman run --hostuser=$userid --user $userid --rm $IMAGE grep $userid /etc/passwd
    user=bogus
    run_podman 126 run --hostuser=$user --rm $IMAGE grep $user /etc/passwd
}

# bats test_tags=ci:parallel
@test "podman run --device-cgroup-rule tests" {
    if is_rootless; then
        run_podman 125 run --device-cgroup-rule="b 7:* rmw" --rm $IMAGE
        is "$output" "Error: device cgroup rules are not supported in rootless mode or in a user namespace"
        return
    fi

    run_podman run --device-cgroup-rule="b 7:* rmw" --rm $IMAGE
    run_podman run --device-cgroup-rule="c 7:* rmw" --rm $IMAGE
    run_podman run --device-cgroup-rule="a 7:1 rmw" --rm $IMAGE
    run_podman run --device-cgroup-rule="a 7 rmw" --rm $IMAGE
    run_podman 125 run --device-cgroup-rule="b 7:* rmX" --rm $IMAGE
    is "$output" "Error: invalid device access in device-access-add: X"
    run_podman 125 run --device-cgroup-rule="b 7:2" --rm $IMAGE
    is "$output" 'Error: invalid device cgroup rule requires type, major:Minor, and access rules: "b 7:2"'
    run_podman 125 run --device-cgroup-rule="x 7:* rmw" --rm $IMAGE
    is "$output" "Error: invalid device type in device-access-add: x"
    run_podman 125 run --device-cgroup-rule="a a:* rmw" --rm $IMAGE
    is "$output" "Error: strconv.ParseUint: parsing \"a\": invalid syntax"
}

# bats test_tags=ci:parallel
@test "podman run closes stdin" {
    random_1=$(random_string 25)
    run_podman run -i --rm $IMAGE cat <<<"$random_1"
    is "$output" "$random_1" "output matches STDIN"
}

# bats test_tags=ci:parallel
@test "podman run defaultenv" {
    run_podman run --rm $IMAGE printenv
    assert "$output" !~ "TERM=" "env doesn't include TERM by default"
    assert "$output" =~ "container=podman" "env includes container=podman"

    run_podman 1 run -t=false --rm $IMAGE printenv TERM
    assert "$output" == "" "env doesn't include TERM"

    run_podman run -t=true --rm $IMAGE printenv TERM    # uses CRLF terminators
    assert "$output" == $'xterm\r' "env includes default TERM"

    run_podman run -t=false -e TERM=foobar --rm $IMAGE printenv TERM
    assert "$output" == "foobar" "env includes TERM"

    run_podman run --unsetenv=TERM --rm $IMAGE printenv
    assert "$output" =~ "container=podman" "env includes container=podman"
    assert "$output" != "TERM" "unwanted TERM environment variable despite --unsetenv=TERM"

    run_podman run --unsetenv-all --rm $IMAGE /bin/printenv
    for v in TERM container PATH; do
        assert "$output" !~ "$v" "variable present despite --unsetenv-all"
    done

    run_podman run --unsetenv-all --env TERM=abc --rm $IMAGE /bin/printenv
    assert "$output" =~ "TERM=abc" \
           "missing TERM environment variable despite TERM being set on commandline"
}

# bats test_tags=ci:parallel
@test "podman run - no /etc/hosts" {
    if [[ -z "$container" ]]; then
        skip "Test is too dangerous to run in a non-container environment"
    fi
    skip_if_rootless "cannot move /etc/hosts file as a rootless user"

    local hosts_tmp=/etc/hosts.RENAME-ME-BACK-TO-JUST-HOSTS
    if [[ -e $hosts_tmp ]]; then
        die "Internal error: leftover backup hosts file: $hosts_tmp"
    fi
    mv /etc/hosts $hosts_tmp
    run_podman '?' run --rm --add-host "foo.com:1.2.3.4" $IMAGE cat "/etc/hosts"
    mv $hosts_tmp /etc/hosts
    assert "$status" = 0 \
           "podman run without /etc/hosts file should work"
    assert "$output" =~ "^1\.2\.3\.4[[:blank:]]foo\.com.*" \
           "users can add hosts even without /etc/hosts"
}

# rhbz#1854566 : $IMAGE has incorrect permission 555 on the root '/' filesystem
# bats test_tags=ci:parallel
@test "podman run image with filesystem permission" {
    # make sure the IMAGE image have permissiong of 555 like filesystem RPM expects
    run_podman run --rm $IMAGE stat -c %a /
    is "$output" "555" "directory permissions on /"
}

# rhbz#1763007 : the --log-opt for podman run does not work as expected
# bats test_tags=ci:parallel
@test "podman run with log-opt option" {
    # Pseudorandom size of the form N.NNN. The '| 1' handles '0.NNN' or 'N.NN0',
    # which podman displays as 'NNN kB' or 'N.NN MB' respectively.
    size=$(printf "%d.%03d" $(($RANDOM % 10 | 1)) $(($RANDOM % 100 | 1)))
    run_podman run -d --rm --log-opt max-size=${size}m $IMAGE sleep 5
    cid=$output
    run_podman inspect --format "{{ .HostConfig.LogConfig.Size }}" $cid
    is "$output" "${size}MB"
    run_podman rm -t 0 -f $cid

    # Make sure run_podman tm -t supports -1 option
    run_podman rm -t -1 -f $cid
}

# bats test_tags=ci:parallel
@test "podman run --kernel-memory warning" {
    # Not sure what situations this fails in, but want to make sure warning shows.
    run_podman '?' run --rm --kernel-memory 100 $IMAGE false
    is "$output" ".*The --kernel-memory flag is no longer supported. This flag is a noop." "warn on use of --kernel-memory"

}

# rhbz#1902979 : podman run fails to update /etc/hosts when --uidmap is provided
# bats test_tags=ci:parallel
@test "podman run update /etc/hosts" {
    HOST=$(random_string 25)
    run_podman run --uidmap 0:10001:10002 --rm --hostname ${HOST} $IMAGE grep ${HOST} /etc/hosts
    is "${lines[0]}" ".*${HOST}.*"
}

# bats test_tags=ci:parallel
@test "podman run doesn't override oom-score-adj" {
    current_oom_score_adj=$(cat /proc/self/oom_score_adj)
    run_podman run --rm $IMAGE cat /proc/self/oom_score_adj
    is "$output" "$current_oom_score_adj" "different oom_score_adj in the container"

    oomscore=$((current_oom_score_adj+1))
    run_podman run --oom-score-adj=$oomscore --rm $IMAGE cat /proc/self/oom_score_adj
    is "$output" "$oomscore" "one more then default oomscore"

    skip_if_remote "containersconf needs to be set on server side"
    oomscore=$((oomscore+1))
    containersconf=$PODMAN_TMPDIR/containers.conf
    cat >$containersconf <<EOF
[containers]
oom_score_adj=$oomscore
EOF
    CONTAINERS_CONF_OVERRIDE=$PODMAN_TMPDIR/containers.conf run_podman run --rm $IMAGE cat /proc/self/oom_score_adj
    is "$output" "$oomscore" "two more then default oomscore"

    oomscore=$((oomscore+1))
    CONTAINERS_CONF_OVERRIDE=$PODMAN_TMPDIR/containers.conf run_podman run --oom-score-adj=$oomscore --rm $IMAGE cat /proc/self/oom_score_adj
    is "$output" "$oomscore" "--oom-score-adj should override containers.conf"
}

# issue 19829
# bats test_tags=ci:parallel
@test "rootless podman clamps oom-score-adj if it is lower than the current one" {
    skip_if_not_rootless
    skip_if_remote
    if grep -- -1000 /proc/self/oom_score_adj; then
        skip "the current oom-score-adj is already -1000"
    fi
    run_podman 0+w run --oom-score-adj=-1000 --rm $IMAGE true
    require_warning "Requested oom_score_adj=.* is lower than the current one, changing to "
}

# CVE-2022-1227 : podman top joins container mount NS and uses nsenter from image
# bats test_tags=ci:parallel
@test "podman top does not use nsenter from image" {
    keepid="--userns=keep-id"
    is_rootless || keepid=""

    tmpdir=$PODMAN_TMPDIR/build-test
    mkdir -p $tmpdir
    tmpbuilddir=$tmpdir/build
    mkdir -p $tmpbuilddir
    dockerfile=$tmpbuilddir/Dockerfile
    cat >$dockerfile <<EOF
FROM $IMAGE
RUN rm /usr/bin/nsenter; \
echo -e "#!/bin/sh\nfalse" >> /usr/bin/nsenter; \
chmod +x /usr/bin/nsenter
EOF

    # --layers=false needed to work around buildah#5674 parallel flake
    test_image="cve_2022_1227_test-$(safename)"
    run_podman build -t $test_image --layers=false $tmpbuilddir
    run_podman run -d ${keepid} $test_image top
    ctr="$output"
    run_podman top $ctr huser,user
    run_podman rm -f -t0 $ctr
    run_podman rmi $test_image
}

# bats test_tags=ci:parallel
@test "podman create --security-opt" {
    run_podman create --security-opt no-new-privileges=true $IMAGE
    run_podman rm $output
    run_podman create --security-opt no-new-privileges:true $IMAGE
    run_podman rm $output
    run_podman create --security-opt no-new-privileges=false $IMAGE
    run_podman rm $output
    run_podman create --security-opt no-new-privileges $IMAGE
    run_podman rm $output
}

# bats test_tags=ci:parallel
@test "podman run --device-read-bps" {
    skip_if_rootless "cannot use this flag in rootless mode"

    if test \! -e /dev/nullb0; then
        skip "/dev/nullb0 not present, use 'modprobe null_blk nr_devices=1' to create it"
    fi

    local cid
    local dev_maj_min=$(stat -c %Hr:%Lr /dev/nullb0)

    # this test is a triple check on blkio flags since they seem to sneak by the tests
    if is_cgroupsv2; then
        run_podman run -dt --device-read-bps=/dev/nullb0:1M $IMAGE top
        cid=$output
        run_podman exec -it $output cat /sys/fs/cgroup/io.max
        is "$output" ".*$dev_maj_min rbps=1048576 wbps=max riops=max wiops=max" "throttle devices passed successfully.*"
    else
        run_podman run -dt --device-read-bps=/dev/nullb0:1M $IMAGE top
        cid=$output
        run_podman exec -it $output cat /sys/fs/cgroup/blkio/blkio.throttle.read_bps_device
        is "$output" ".*$dev_maj_min 1048576" "throttle devices passed successfully.*"
    fi
    run_podman container rm -f -t0 $cid
}

# bats test_tags=ci:parallel
@test "podman run failed --rm " {
    port=$(random_free_port)

    # Container names must sort alphanumerically
    c_ok=c1_$(safename)
    c_fail_no_rm=c2_$(safename)
    c_fail_with_rm=c3_$(safename)

    # Run two containers with the same port bindings. The second must fail
    run_podman     run -p $port:80 --rm -d --name $c_ok           $IMAGE top
    run_podman 126 run -p $port:80      -d --name $c_fail_no_rm   $IMAGE top
    assert "$output" =~ "ddress already in use"
    run_podman 126 run -p $port:80 --rm -d --name $c_fail_with_rm $IMAGE top
    assert "$output" =~ "ddress already in use"
    # Prior to #15060, the third container would still show up in ps -a
    run_podman ps -a --sort names --format '--{{.Image}}-{{.Names}}--'
    assert "$output" !~ "$c_fail_with_rm" \
           "podman ps -a must not show failed container run with --rm"
    assert "$output" =~ "--$IMAGE-${c_ok}--.*--$IMAGE-${c_fail_no_rm}--" \
           "podman ps -a shows running & failed containers"

    run_podman container rm -f -t 0 $c_ok $c_fail_no_rm
}

# bats test_tags=ci:parallel
@test "podman run --attach stdin prints container ID" {
    ctr_name="container-$(safename)"
    run_podman run --name $ctr_name --attach stdin $IMAGE echo hello
    run_output=$output
    run_podman inspect --format "{{.Id}}" $ctr_name
    ctr_id=$output
    is "$run_output" "$ctr_id" "Did not find container ID in the output"
    run_podman rm $ctr_name
}

# Regression test for https://github.com/containers/podman/issues/27414
# bats test_tags=ci:parallel
@test "podman run with empty --detach-keys" {
    # Empty string should disable detaching, not error with "invalid detach keys"
    run_podman run --rm --detach-keys="" $IMAGE echo "success"
    is "$output" "success" "empty detach-keys should work"
}

# 15895: --privileged + --systemd = hide /dev/ttyNN
# bats test_tags=ci:parallel
@test "podman run --privileged as root with systemd will not mount /dev/tty" {
    skip_if_rootless "this test only makes sense as root"

    # First, confirm that we _have_ /dev/ttyNN devices on the host.
    # ('skip' would be nicer in some sense... but could hide a regression.
    # Fedora, RHEL, Debian, Ubuntu, Gentoo, all have /dev/ttyN, so if
    # this ever triggers, it means a real problem we should know about.)
    vt_tty_devices_count=$(find /dev -regex '/dev/tty[0-9].*' | wc -w)
    assert "$vt_tty_devices_count" != "0" \
           "Expected at least one /dev/ttyN device on host"

    # Ok now confirm that without --systemd, podman exposes ttyNN devices
    run_podman run --rm -d --privileged $IMAGE ./pause
    cid="$output"

    run_podman exec $cid sh -c "find /dev -regex '/dev/tty[0-9].*' | wc -w"
    assert "$output" = "$vt_tty_devices_count" \
           "ls /dev/tty* without systemd; should have lots of ttyN devices"
    run_podman stop -t 0 $cid

    # Actual test for 15895: with --systemd, no ttyN devices are passed through
    run_podman run -d --privileged --stop-signal=TERM --systemd=always $IMAGE top
    cid="$output"

    run_podman exec $cid sh -c "find /dev -regex '/dev/tty[0-9].*' | wc -w"
    assert "$output" = "0" \
           "ls /dev/tty[0-9] with --systemd=always: should have no ttyN devices"

    # Make sure run_podman stop supports -1 option
    run_podman stop -t -1 $cid
    run_podman rm -t -1 -f $cid
}

# bats test_tags=ci:parallel
@test "podman run --privileged as rootless will not mount /dev/tty\d+" {
    skip_if_not_rootless "this test as rootless"

    # First, confirm that we _have_ /dev/ttyNN devices on the host.
    # ('skip' would be nicer in some sense... but could hide a regression.
    # Fedora, RHEL, Debian, Ubuntu, Gentoo, all have /dev/ttyN, so if
    # this ever triggers, it means a real problem we should know about.)
    vt_tty_devices_count=$(find /dev -regex '/dev/tty[0-9].*' | wc -w)
    assert "$vt_tty_devices_count" != "0" \
           "Expected at least one /dev/ttyN device on host"

    run_podman run --rm -d --privileged $IMAGE ./pause
    cid="$output"

    run_podman exec $cid sh -c "find /dev -regex '/dev/tty[0-9].*' | wc -w"
    assert "$output" = "0" \
           "ls /dev/tty[0-9]: should have no ttyN devices"

    run_podman stop -t 0 $cid
}

# 16925: --privileged + --systemd = share non-virtual-terminal TTYs (both rootful and rootless)
# bats test_tags=ci:parallel
@test "podman run --privileged as root with systemd mounts non-vt /dev/tty devices" {
    # First, confirm that we _have_ non-virtual terminal /dev/tty* devices on
    # the host.
    non_vt_tty_devices_count=$(find /dev -regex '/dev/tty[^0-9].*' | wc -w)
    if [ "$non_vt_tty_devices_count" -eq 0 ]; then
        skip "The server does not have non-vt TTY devices"
    fi

    # Verify that all the non-vt TTY devices got mounted in the container
    run_podman run --rm -d --privileged --systemd=always $IMAGE ./pause
    cid="$output"
    run_podman '?' exec $cid find /dev -regex '/dev/tty[^0-9].*'
    assert "$status" = 0 \
           "No non-virtual-terminal TTY devices got mounted in the container"
    assert "$(echo "$output" | wc -w)" = "$non_vt_tty_devices_count" \
           "Some non-virtual-terminal TTY devices are missing in the container"
    run_podman stop -t 0 $cid
}

# bats test_tags=ci:parallel
@test "podman run read-only from containers.conf" {
    containersconf=$PODMAN_TMPDIR/containers.conf
    cat >$containersconf <<EOF
[containers]
read_only=true
EOF

    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman 1 run --rm $IMAGE touch /testro
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman run --rm --read-only=false $IMAGE touch /testrw

    files="/tmp/a /var/tmp/b /dev/c /dev/shm/d /run/e"
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman run --rm $IMAGE touch $files
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman run --rm --read-only=false $IMAGE touch $files
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman run --rm --read-only=false --read-only-tmpfs=true $IMAGE touch $files
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman run --rm --read-only-tmpfs=true $IMAGE touch $files

    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman 1 run --rm --read-only-tmpfs=false $IMAGE touch $files
    assert "$output" == "touch: /tmp/a: Read-only file system
touch: /var/tmp/b: Read-only file system
touch: /dev/c: Read-only file system
touch: /dev/shm/d: Read-only file system
touch: /run/e: Read-only file system"
}

# bats test_tags=ci:parallel
@test "podman run ulimit from containers.conf" {
    skip_if_remote "containers.conf has to be set on remote, only tested on E2E test"
    containersconf=$PODMAN_TMPDIR/containers.conf
    # Safe minimum: anything under 27 barfs w/ "crun: ... Too many open files"
    nofile1=$((30 + RANDOM % 10000))
    nofile2=$((30 + RANDOM % 10000))
    cat >$containersconf <<EOF
[containers]
default_ulimits = [
  "nofile=${nofile1}:${nofile1}",
]
EOF

    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman run --rm $IMAGE grep "Max open files" /proc/self/limits
    assert "$output" =~ " ${nofile1}  * ${nofile1}  * files"
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman run --ulimit nofile=${nofile2}:${nofile2} --rm $IMAGE grep "Max open files" /proc/self/limits
    assert "$output" =~ " ${nofile2}  * ${nofile2}  * files"
}

# bats test_tags=ci:parallel
@test "podman run ulimit with -1" {
    max=unlimited
    if is_rootless; then
        run ulimit -c -H
        max=$output
        if [[ "$max" != "unlimited" ]] && [[ $max -lt 1000 ]]; then
            skip "ulimit -c == $max, test requires >= 1000"
        fi
    fi

    ctr="c-h-$(safename)"
    run_podman run -d --name $ctr --ulimit core=-1:-1 $IMAGE /home/podman/pause

    run_podman inspect $ctr --format "{{.HostConfig.Ulimits}}"
    assert "$output" =~ "RLIMIT_CORE -1 -1" "ulimit core is not set to unlimited"

    run_podman rm -f $ctr

    run_podman run --ulimit core=-1:-1 --rm $IMAGE grep core /proc/self/limits
    assert "$output" =~ " ${max}  * ${max}  * bytes"

    run_podman run --ulimit core=1000:-1 --rm $IMAGE grep core /proc/self/limits
    assert "$output" =~ " 1000  * ${max}  * bytes"

    run_podman 125 run --ulimit core=-1:1000 --rm $IMAGE grep core /proc/self/limits
    is "$output" "Error: ulimit option \"core=-1:1000\" requires name=SOFT:HARD, failed to be parsed: ulimit soft limit must be less than or equal to hard limit: soft: -1 (unlimited), hard: 1000"
}

# bats test_tags=ci:parallel
@test "podman run - can use maximum ulimit value" {
    skip_if_remote "cannot check local ulimits with podman remote"
    run ulimit -n -H
    max=$output
    run_podman run --rm --ulimit=nofile=$max:$max $IMAGE sh -c 'ulimit -n -H'
    is "$output" "$max" "wrong ulimit value"

    run_podman run --rm $IMAGE sh -c 'ulimit -n -H'
    default_value=$output

    # Set the current ulimit smaller than the default value
    ulimit -n -SH $((default_value - 1))

    run_podman run --rm $IMAGE sh -c 'ulimit -n -H'

    if is_rootless; then
        # verify that the value was clamped to the maximum allowed
        is "$output" "$(ulimit -n -H)" "wrong ulimit value"
    else
        # when running as root check that the current environment does not affect
        # the ulimit set inside the container.
        is "$output" "$default_value" "wrong ulimit value"
    fi
}

# bats test_tags=ci:parallel
@test "podman run - ulimits have the correct default values" {
    expected_nofile=1048576
    expected_nproc=1048576

    # clamp the expected values in rootless mode when they are
    # greater than the current limits.
    if is_rootless; then
        nofile=$(ulimit -n -H)
        if [[ $nofile -lt $expected_nofile ]]; then
            expected_nofile=$nofile
        fi
        nproc=$(ulimit -u -H)
        if [[ $nproc -lt $expected_nproc ]]; then
            expected_nproc=$nproc
        fi
    fi

    # validate that nofile and nproc are both set to the correct value
    run_podman run --rm $IMAGE sh -c 'ulimit -n -H'
    is "$output" "$expected_nofile" "wrong ulimit -n default value"

    run_podman run --rm $IMAGE sh -c 'ulimit -u -H'
    is "$output" "$expected_nproc" "wrong ulimit -u default value"
}

# bats test_tags=ci:parallel
@test "podman run bad --name" {
    randomname=c_$(safename)
    run_podman 125 create --name "$randomname/bad" $IMAGE
    run_podman create --name "/$randomname" $IMAGE
    run_podman ps -a --filter name="^/$randomname$" --format '{{ .Names }}'
    is "$output" "$randomname" "Should be able to find container by name"
    run_podman rm "/$randomname"
    run_podman 125 create --name "$randomname/" $IMAGE
}

# bats test_tags=ci:parallel
@test "podman run --net=host --cgroupns=host with read only cgroupfs" {
    # verify that the last /sys/fs/cgroup mount is read-only
    run_podman run --net=host --cgroupns=host --rm $IMAGE sh -c "grep ' / /sys/fs/cgroup ' /proc/self/mountinfo | tail -n 1"
    assert "$output" =~ "/sys/fs/cgroup ro"

    # verify that it works also with a cgroupns
    run_podman run --net=host --cgroupns=private --rm $IMAGE sh -c "grep ' / /sys/fs/cgroup ' /proc/self/mountinfo | tail -n 1"
    assert "$output" =~ "/sys/fs/cgroup ro"
}

# bats test_tags=ci:parallel
@test "podman run - idmapped mounts" {
    skip_if_rootless "idmapped mounts work only with root for now"

    skip_if_remote "userns=auto is set on the server"

    grep -E -q "^containers:" /etc/subuid || skip "no IDs allocated for user 'containers'"

    # the TMPDIR must be accessible by different users as the following tests use different mappings
    chmod 755 $PODMAN_TMPDIR

    run_podman image mount $IMAGE
    src="$output"

    # we cannot use idmap on top of overlay, so we need a copy
    romount=$PODMAN_TMPDIR/rootfs
    cp -a "$src" "$romount"

    run_podman image unmount $IMAGE

    # check if the underlying file system supports idmapped mounts
    run_podman '?' run --security-opt label=disable --rm --uidmap=0:1000:10000 --rootfs $romount:idmap true
    if [[ $status -ne 0 ]]; then
        # Note bash regex only works if the right hand side is unquoted so we have to use this extra var.
        match="failed to create idmapped mount.* invalid argument"
        if [[ "$output" =~ $match ]]; then
            skip "idmapped mounts not supported"
        fi
        # Any other error is fatal
        die "Cannot create idmap mount: $output"
    fi
    ensure_no_mountpoint "$romount"

    mkdir -p $PODMAN_TMPDIR/shared-volume
    # test that there are no mount leaks also when a shared volume is used (with a shared volume the rootfs propagation is set to shared).
    run_podman run --security-opt label=disable --rm --uidmap=0:1000:10000 -v $PODMAN_TMPDIR/shared-volume:/a-shared-volume:shared --rootfs $romount:idmap stat -c %u:%g /bin
    is "$output" "0:0"
    ensure_no_mountpoint "$romount"

    run_podman run --security-opt label=disable --uidmap=0:1000:10000 --rm --rootfs "$romount:idmap=uids=0-1001-10000;gids=0-1002-10000" stat -c %u:%g /bin
    is "$output" "1:2"
    ensure_no_mountpoint "$romount"

    touch $romount/testfile
    chown 2000:2000 $romount/testfile
    run_podman run --security-opt label=disable --uidmap=0:1000:200 --rm --rootfs "$romount:idmap=uids=@2000-1-1;gids=@2000-1-1" stat -c %u:%g /testfile
    is "$output" "1:1"
    ensure_no_mountpoint "$romount"

    # verify that copyup with an empty idmap volume maintains the original ownership with different mappings and --rootfs
    myvolume=my-volume-$(safename)
    run_podman volume create $myvolume
    mkdir $romount/volume
    chown 1000:1000 $romount/volume
    for FROM in 1000 2000; do
        run_podman run --security-opt label=disable --rm --uidmap=0:$FROM:10000 -v $myvolume:/volume:idmap --rootfs $romount stat -c %u:%g /volume
        is "$output" "0:0"
        ensure_no_mountpoint "$romount"
    done
    run_podman volume rm $myvolume

    # verify that copyup with an empty idmap volume maintains the original ownership with different mappings
    myvolume=my-volume-$(safename)
    for FROM in 1000 2000; do
        run_podman run --rm --uidmap=0:$FROM:10000 -v $myvolume:/etc:idmap $IMAGE stat -c %u:%g /etc/passwd
        is "$output" "0:0"
    done
    run_podman volume rm $myvolume

    rm -rf $romount
}

# bats test_tags=ci:parallel
@test "podman run --restart=always/on-failure -- wait" {
    # regression test for #18572 to make sure Podman waits less than 20 seconds
    ctr=c_$(safename)
    for policy in always on-failure; do
        run_podman run -d --restart=$policy --name=$ctr $IMAGE false
        PODMAN_TIMEOUT=20 run_podman wait $ctr
        is "$output" "1" "container should exit 1 (policy: $policy)"
        run_podman rm -f -t0 $ctr
    done
}

# bats test_tags=ci:parallel
@test "podman run --restart preserves hooks-dir" {
    # regression test for #17935 to ensure hooks are run on successful restarts
    ctr=c_$(safename)
    hooksdir=$PODMAN_TMPDIR/hooks_$(safename)

    skip_if_remote "--hooks-dir is not usable with remote"

    mkdir -p "$hooksdir"
    cat > "$hooksdir/settings.json" <<EOF
{
    "version": "1.0.0",
    "when": { "always": true },
    "hook": {
        "path": "$hooksdir/hook.sh"
    },
    "stages": ["prestart", "poststop"]
}
EOF
    cat >"$hooksdir/hook.sh" <<EOF
#!/bin/sh
# consume stdin
cat > /dev/null
echo ran >> "$hooksdir/log"
EOF
    chmod +x "$hooksdir/hook.sh"

    run_podman run -d --restart=on-failure:2 \
        --name="$ctr" --hooks-dir="$hooksdir" $IMAGE false

    wait_for_restart_count "$ctr" 2 "restart preserves hooks-dir"

    # also make sure 3rd restart has finished
    # wait also waits until 'cleanup' is done, so poststop hook is
    # done running after this command
    run_podman wait "$ctr"
    run_podman rm "$ctr"

    # check prestart and poststop ran 3 times each
    assert "$(wc -l < "$hooksdir/log")" = 6

    rm -r "$hooksdir"
}

# bats test_tags=ci:parallel
@test "podman run - custom static_dir" {
    # regression test for #19938 to make sure the cleanup process uses the same
    # static_dir and writes the exit code.  If not, podman-run will run into
    # it's 20 sec timeout waiting for the exit code to be written.

    skip_if_remote "CONTAINERS_CONF_OVERRIDE redirect does not work on remote"
    containersconf=$PODMAN_TMPDIR/containers.conf
    static_dir=$PODMAN_TMPDIR/static_dir
cat >$containersconf <<EOF
[engine]
static_dir="$static_dir"
EOF
    ctr=c_$(safename)
    CONTAINERS_CONF_OVERRIDE=$containersconf PODMAN_TIMEOUT=20 run_podman run --name=$ctr $IMAGE true
    CONTAINERS_CONF_OVERRIDE=$containersconf PODMAN_TIMEOUT=20 run_podman inspect --format "{{.ID}}" $ctr
    cid="$output"
    # Since the container has been run with custom static_dir (where the libpod
    # DB is stored), the default podman should not see it.
    run_podman 1 container exists $ctr
    run_podman 1 container exists $cid
    CONTAINERS_CONF_OVERRIDE=$containersconf run_podman rm -f -t0 $ctr
}

# bats test_tags=ci:parallel
@test "podman --authfile=nonexistent-path" {
    # List of commands to be tested. These all share a common authfile check.
    #
    # Table format is:
    #   podman command | arguments | '-' if it does not work with podman-remote
    echo "from $IMAGE" > $PODMAN_TMPDIR/Containerfile
    tests="
auto-update          |                  | -
build                | $PODMAN_TMPDIR   |
container runlabel   | run $IMAGE       | -
create               | $IMAGE argument  |
image sign           | $IMAGE           | -
kube play            | argument         |
logout               | $IMAGE           |
manifest add         | $IMAGE argument  |
manifest inspect     | $IMAGE           |
manifest push        | $IMAGE           |
pull                 | $IMAGE           |
push                 | $IMAGE           |
run --rm             | $IMAGE false     |
search               | $IMAGE           |
"

    bogus=$PODMAN_TMPDIR/bogus-authfile
    touch $PODMAN_TMPDIR/Containerfile

    defer-assertion-failures

    while read command args local_only;do
        # skip commands that don't work in podman-remote
        if [[ "$local_only" = "-" ]]; then
            if is_remote; then
                continue
            fi
        fi

        # parse_table gives us '' (two single quotes) for empty columns
        if [[ "$args" = "''" ]]; then args=;fi

        run_podman 125 $command --authfile=$bogus $args
        assert "$output" = "Error: credential file is not accessible: faccessat $bogus: no such file or directory" \
           "$command --authfile=nonexistent-path"

        if [[ "$command" != "logout" ]]; then
           REGISTRY_AUTH_FILE=$bogus run_podman '?' $command $args
           assert "$output" !~ "credential file is not accessible" \
              "$command REGISTRY_AUTH_FILE=nonexistent-path"

           # "create" leaves behind a container. Clean it up.
           if [[ "$command" = "create" ]]; then
               run_podman container rm $output
           fi
        fi
    done < <(parse_table "$tests")
}

# bats test_tags=ci:parallel
@test "podman --syslog and environment passed to conmon" {
    skip_if_remote "--syslog is not supported for remote clients"
    skip_if_journald_unavailable

    run_podman run -d -q --syslog $IMAGE sleep infinity
    cid="$output"

    run_podman container inspect $cid --format "{{ .State.ConmonPid }}"
    conmon_pid="$output"
    is "$(< /proc/$conmon_pid/cmdline)" ".*--exit-command-arg--syslog.*" "conmon's exit-command has --syslog set"
    conmon_env="$(< /proc/$conmon_pid/environ)"
    assert "$conmon_env" =~ "BATS_TEST_TMPDIR" "entire env is passed down to conmon (incl. BATS variables)"
    assert "$conmon_env" !~ "NOTIFY_SOCKET=" "NOTIFY_SOCKET is not included (incl. BATS variables)"
    if ! is_rootless; then
        assert "$conmon_env" !~ "DBUS_SESSION_BUS_ADDRESS=" "DBUS_SESSION_BUS_ADDRESS is not included (incl. BATS variables)"
    fi

    run_podman rm -f -t0 $cid
}

# bats test_tags=ci:parallel
@test "podman create container with conflicting name" {
    local cname=c_$(safename)
    local output_msg_ext="^Error: .*: the container name \"$cname\" is already in use by .* You have to remove that container to be able to reuse that name: that name is already in use by an external entity, or use --replace to instruct Podman to do so."
    local output_msg="^Error: .*: the container name \"$cname\" is already in use by .* You have to remove that container to be able to reuse that name: that name is already in use, or use --replace to instruct Podman to do so."
    if is_remote; then
        output_msg_ext="^Error: .*: the container name \"$cname\" is already in use by .* You have to remove that container to be able to reuse that name: that name is already in use by an external entity"
        output_msg="^Error: .*: the container name \"$cname\" is already in use by .* You have to remove that container to be able to reuse that name: that name is already in use"
    fi

    # external container
    buildah from --name $cname scratch

    run_podman 125 create --name $cname $IMAGE
    assert "$output" =~ "$output_msg_ext" "Trying to create two containers with same name"

    run_podman container rm $cname

    run_podman --noout create --name $cname $IMAGE

    run_podman 125 create --name $cname $IMAGE
    assert "$output" =~ "$output_msg" "Trying to create two containers with same name"

    run_podman container rm $cname
}

# https://issues.redhat.com/browse/RHEL-14469
# bats test_tags=ci:parallel
@test "podman run - /run must not be world-writable in systemd containers" {
    _prefetch $SYSTEMD_IMAGE

    run_podman run -d --rm $SYSTEMD_IMAGE /usr/sbin/init
    cid=$output

    # runc has always been 755; crun < 1.11 was 777
    run_podman exec $cid stat -c '%a' /run
    assert "$output" = "755" "stat /run"

    run_podman rm -f -t0 $cid
}

# bats test_tags=ci:parallel
@test "podman run with mounts.conf missing" {
    skip_if_remote "--default-mounts-file is not supported for remote clients"
    MOUNTS_CONF=$PODMAN_TMPDIR/mounts.conf
    run_podman run --rm --default-mounts-file=${MOUNTS_CONF} $IMAGE echo test1
    assert "$output" = "test1" "No warning messages on missing mounts file"

    touch ${MOUNTS_CONF}

    run_podman run --rm --default-mounts-file=${MOUNTS_CONF} $IMAGE echo test2
    assert "$output" = "test2" "No warning messages on empty mounts file"

    echo /tmp/bogus > ${MOUNTS_CONF}
    run_podman run --rm --default-mounts-file=${MOUNTS_CONF} $IMAGE echo test3
    assert "$output" = "test3" "No warning messages on missing content in mounts file"

    randfile=$(random_string 30)
    randcontent=$(random_string 30)
    mkdir -p $PODMAN_TMPDIR/mounts
    echo $randcontent > $PODMAN_TMPDIR/mounts/$randfile
    echo $PODMAN_TMPDIR/mounts:/run/secrets > ${MOUNTS_CONF}
    run_podman run --rm --default-mounts-file=${MOUNTS_CONF} $IMAGE cat /run/secrets/$randfile
    assert "$output" = "$randcontent" "mounts should appear in container"
}

# bats test_tags=ci:parallel
@test "podman run - rm pod if container creation failed with -pod new:" {
    cname=c_$(safename)
    run_podman run -d --name $cname $IMAGE hostname
    cid=$output

    podname=pod_$(safename)
    run_podman 125 run --rm --pod "new:$podname" --name $cname $IMAGE hostname
    is "$output" ".*creating container storage: the container name \"$cname\" is already in use by"

    # pod should've been cleaned up
    # if container creation failed
    run_podman 1 pod exists $podname

    run_podman rm $cid
}

# bats test_tags=ci:parallel
@test "podman run - no entrypoint" {
    run_podman 127 run --rm --rootfs "$PODMAN_TMPDIR"

    # runc and crun emit different diagnostics
    runtime=$(podman_runtime)
    case "$runtime" in
        crun) expect='\(executable file `` not found in $PATH\|cannot find `` in $PATH\): No such file or directory: OCI runtime attempted to invoke a command that was not found' ;;
        runc) expect='runc: runc create failed: unable to start container process:.* exec: "": executable file not found in $PATH: OCI runtime attempted to invoke a command that was not found' ;;
        *)    skip "Unknown runtime '$runtime'" ;;
    esac

    # The '.*' in the error below is for dealing with podman-remote, which
    # includes "error preparing container <sha> for attach" in output.
    is "$output" "Error.* $expect" "podman emits useful diagnostic when no entrypoint is set"
}

# bats test_tags=ci:parallel
@test "podman run - stopping loop" {
    skip_if_remote "this doesn't work with with the REST service"

    cname=c_$(safename)
    run_podman run -d --name $cname --stop-timeout 240 $IMAGE sh -c 'echo READY; sleep 999'
    cid="$output"
    wait_for_ready $cname

    run_podman inspect --format '{{ .State.ConmonPid }}' $cname
    conmon_pid=$output

    ${PODMAN} stop $cname &
    stop_pid=$!

    timeout=20
    while :;do
        sleep 0.5
        run_podman container inspect --format '{{.State.Status}}' $cname
        if [[ "$output" = "stopping" ]]; then
            break
        fi
        timeout=$((timeout - 1))
        if [[ $timeout == 0 ]]; then
            run_podman ps -a
            die "Timed out waiting for container to acknowledge signal"
        fi
    done

    kill -9 ${stop_pid}
    kill -9 ${conmon_pid}

    # Unclear why `-t0` is required here, works locally without.
    # But it shouldn't hurt and does make the test pass...
    PODMAN_TIMEOUT=5 run_podman 125 stop -t0 $cname
    is "$output" "Error: container .* conmon exited prematurely, exit code could not be retrieved: conmon process killed" "correct error on missing conmon"

    # This should be safe because stop is guaranteed to call cleanup?
    run_podman inspect --format "{{ .State.Status }}" $cname
    is "$output" "exited" "container has successfully transitioned to exited state after stop"

    run_podman rm -f -t0 $cname
}

# bats test_tags=ci:parallel
@test "podman run - no-hostname" {
    randomname=c_$(safename)
    echo "\
from $IMAGE
RUN umount /etc/hostname; rm /etc/hostname
" > $PODMAN_TMPDIR/Containerfile
    run_podman build -t $randomname --cap-add SYS_ADMIN ${PODMAN_TMPDIR}

    run_podman run --rm $randomname ls /etc/hostname

    run_podman 1 run --no-hostname --rm $randomname ls /etc/hostname
    is "$output" "ls: /etc/hostname: No such file or directory" "container did not add /etc/hostname"

    run_podman rmi $randomname
}

@test "podman run --log-opt size= and containers.conf log_size_max" {
    skip_if_remote "remote does not support CONTAINERS_CONF"

    containersconf=$PODMAN_TMPDIR/containers.conf
    cat >$containersconf <<EOF
[containers]
log_driver = "k8s-file"
log_size_max = 400000000
EOF

    c1name=c1_$(safename)
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman create --name $c1name $IMAGE ls /
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman inspect --format '{{ .HostConfig.LogConfig.Size }}' $c1name
    is "$output" "400MB"

    c2name=c2_$(safename)
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman create --name $c2name --log-opt max-size=8000 $IMAGE ls /
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman inspect --format '{{ .HostConfig.LogConfig.Size }}' $c2name
    is "$output" "8kB"

    run_podman rm -f $c1name $c2name
}

# bats test_tags=networking,registry
@test "podman run with --cert-dir" {
    skip_if_remote "cert-dir option not working via remote"

    test -n "$PODMAN_LOGIN_REGISTRY_PORT" || skip "registry not set up"

    start_registry

    image=localhost:${PODMAN_LOGIN_REGISTRY_PORT}/cert-dir-run-test-$(safename)

    # First push an image to our test registry
    run_podman push \
               --cert-dir ${PODMAN_LOGIN_WORKDIR}/trusted-registry-cert-dir \
               --creds ${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS} \
               $IMAGE $image

    # Run without --cert-dir should fail (TLS verification error)
    run_podman 125 run --rm \
               --creds ${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS} \
               $image echo "this should fail"

    # Run with --cert-dir should succeed (will pull the image)
    run_podman run --rm \
               --cert-dir ${PODMAN_LOGIN_WORKDIR}/trusted-registry-cert-dir \
               --creds ${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS} \
               $image true

    # Clean up, and it would fail if the $image was not pulled
    run_podman rmi $image
}

@test "podman run host env leak" {
    # We need to create a invalid image config env value without "="
    skopeo copy containers-storage:$IMAGE dir:$PODMAN_TMPDIR
    config_digest=$(jq -r .config.digest $PODMAN_TMPDIR/manifest.json)
    plain_digest=${config_digest#*:}
    newfile="$PODMAN_TMPDIR/newfile"
    # Append bad env to existing image envs
    jq  '.config.Env += ["HOST*"]' $PODMAN_TMPDIR/$plain_digest >$newfile
    # Get new digest and size so we can update the manifest
    newdigest="$(sha256sum $newfile | cut -d" " -f 1)"
    size=$(stat -c %s $newfile)
    mv $newfile $PODMAN_TMPDIR/$newdigest
    jq ".config.digest = \"sha256:$newdigest\" | .config.size=$size" $PODMAN_TMPDIR/manifest.json > $PODMAN_TMPDIR/manifest.json.new
    mv $PODMAN_TMPDIR/manifest.json.new $PODMAN_TMPDIR/manifest.json

    image="localhost/envimage:123"
    skopeo copy dir:$PODMAN_TMPDIR containers-storage:$image

    run_podman image inspect $image --format '{{.Config.Env}}'
    assert "$output" =~ "HOST\*" "invalid env in image"

    HOSTENV=123 run_podman 125 run --rm $image printenv HOSTENV
    assert "$output" =~ 'invalid image env variable "HOST\*"' "Host env leak from image env on podman run"

    podname="p-$(safename)"
    ctrname="c-$(safename)"

    fname="$PODMAN_TMPDIR/kube_$(safename).yaml"
    echo "
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $podname
spec:
  restartPolicy: Never
  containers:
    - name: $ctrname
      image: $image
      command:
      - printenv
      - HOSTENV
" > $fname

    run_podman 125 kube play $fname
    assert "$output" =~ 'invalid image env variable "HOST\*"' "Host env leak from image env on kube play"

    run_podman pod rm $podname
    run_podman rmi $image
}

# bats test_tags=ci:parallel
@test "podman inspect - Args does not duplicate Path for single command" {
    cname=c_$(safename)

    run_podman run --name $cname -d $IMAGE top -b
    run_podman inspect $cname --format '{{.Path}}'
    is "$output" "top" ".Path is the command"
    run_podman inspect $cname --format '{{join .Args " "}}'
    is "$output" "-b" ".Args holds only the arguments after the command"
    run_podman rm -f $cname

    run_podman run --name $cname -d $IMAGE top
    run_podman inspect $cname --format '{{.Path}}'
    is "$output" "top" ".Path is the command"
    run_podman inspect $cname --format '{{len .Args}}'
    is "$output" "0" ".Args should be empty when command has no arguments"
    run_podman rm -f $cname
}

# vim: filetype=sh
