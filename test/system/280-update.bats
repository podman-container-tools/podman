#!/usr/bin/env bats   -*- bats -*-
#
# Tests for podman update
#

load helpers

LOOPDEVICE=

function teardown() {
    if [[ -n "$LOOPDEVICE" ]]; then
        losetup -d $LOOPDEVICE
        LOOPDEVICE=
    fi
    basic_teardown
}


@test "podman update - test all options" {
    local cgv=1
    if is_cgroupsv2; then
        cgv=2;
    fi

    # Need a block device for blkio-weight-device testing
    local pass_loop_device=
    if ! is_rootless; then
        if is_cgroupsv2; then
            lofile=${PODMAN_TMPDIR}/disk.img
            fallocate -l 1k  ${lofile}
            LOOPDEVICE=$(losetup --show -f $lofile)
            pass_loop_device="--device $LOOPDEVICE"

            # Get maj:min (tr needed because losetup seems to use %2d)
            lomajmin=$(losetup -l --noheadings --output MAJ:MIN $LOOPDEVICE | tr -d ' ')
        fi
    fi

    # Shortcuts to make the table narrower
    local -a gig=(0 1073741824 2147483648 3221225472)
    local devicemax="1:5 rbps=10485760 wbps=31457280 riops=2000 wiops=4000"
    local mm=memory/memory

    # Format:
    #   --<option> = <value>  | rootless? | check: cgroups v1            | check: cgroups v2
    #
    # Requires very wide window to read. Sorry.
    #
    # FIXMEs:
    #  cpu-rt-period  (cgv1 only, on cpu/cpu.rt_period_us) works on RHEL8 but not on Ubuntu
    #  cpu-rt-runtime (cgv1 only, on cpu/cpu.rt_runtime_us) fails: error setting cgroup config for procHooks ...
    tests="
cpu-shares          =            512 | - | cpu/cpu.shares       = 512              | cpu.weight      = 20
cpus                =              5 | - | cpu/cpu.cfs_quota_us = 500000           | cpu.max         = 500000 100000
cpuset-cpus         =              0 | - | cpuset/cpuset.cpus   = 0                | cpuset.cpus     = 0
cpuset-mems         =              0 | - | cpuset/cpuset.mems   = 0                | cpuset.mems     = 0

memory              =             1G | 2 | $mm.limit_in_bytes       = ${gig[1]}    | memory.max      = ${gig[1]}
memory-swap         =             3G | 2 | $mm.memsw.limit_in_bytes = ${gig[3]}    | memory.swap.max = ${gig[2]}
memory-reservation  =           400M | 2 | $mm.soft_limit_in_bytes  = 419430400    | memory.low      = 419430400

blkio-weight        =            321 | - | -                                       | io.bfq.weight   = default 321 $lomajmin 98
blkio-weight-device = $LOOPDEVICE:98 | - | -                                       | io.bfq.weight   = default 321 $lomajmin 98

device-read-bps     = $LOOPDEVICE:10mb | - | -                                       | io.max          = $devicemax
device-read-iops    = $LOOPDEVICE:2000 | - | -                                       | io.max          = $devicemax
device-write-bps    = $LOOPDEVICE:30mb | - | -                                       | io.max          = $devicemax
device-write-iops   = $LOOPDEVICE:4000 | - | -                                       | io.max          = $devicemax
"

    # Run a container
    run_podman run ${pass_loop_device} -d $IMAGE sleep infinity
    cid="$output"

    # Pass 1: read the table above, gather up the options applicable
    # to this test environment (root/rootless, cgroups v1/v2)
    local -a opts
    local -A check
    while read opt works_rootless cgv1 cgv2; do
        if is_rootless; then
            local skipping="skipping --$opt : does not work rootless"
            if [[ $works_rootless = '-' ]]; then
                echo "[ $skipping ]"
                continue
            fi
            if [[ ! $works_rootless =~ $cgv ]]; then
                echo "[ $skipping on cgroups v$cgv ]"
                continue
            fi
        fi

        # Determine the "path = newvalue" string for this cgroup
        tuple=$cgv1
        if is_cgroupsv2; then
            tuple=$cgv2
        fi
        if [[ $tuple = '-' ]]; then
            echo "[ skipping --$opt : N/A on cgroups v$cgv ]"
            continue
        fi

        # Sigh. bfq doesn't exist on Debian (2024-03)
        read path op expect <<<"$tuple"
        if [[ ! -e /sys/fs/cgroup/$path ]]; then
            echo "[ skipping --$opt : /sys/fs/cgroup/$path does not exist ]"
            continue
        fi

        # OK: setting is applicable. Preserve it. (First removing whitespace)
        opt=${opt// /}
        opts+=("--$opt")
        check["--$opt"]=$tuple
    done < <(parse_table "$tests")

    # Now do the update in one fell swoop
    run_podman update "${opts[@]}" $cid

    # ...and check one by one
    defer-assertion-failures
    for opt in "${opts[@]}"; do
        read path op expect <<<"${check[$opt]}"
        run_podman exec $cid cat /sys/fs/cgroup/$path

        # Magic echo of unquoted-output converts newlines to spaces;
        # important for otherwise multiline blkio file.
        updated="$(echo $output)"
        assert "$updated" $op "$expect" "$opt ($path)"
    done
    immediate-assertion-failures

    # Clean up
    run_podman rm -f -t0 $cid
    if [[ -n "$LOOPDEVICE" ]]; then
        losetup -d $LOOPDEVICE
        LOOPDEVICE=
    fi
}

@test "podman update - set restart policy" {
    touch ${PODMAN_TMPDIR}/sentinel
    run_podman run --security-opt label=disable --name testctr -v ${PODMAN_TMPDIR}:/testdir -d $IMAGE sh -c "touch /testdir/alive; while test -e /testdir/sentinel; do sleep 0.1; done;"

    run_podman container inspect testctr --format "{{ .HostConfig.RestartPolicy.Name }}"
    is "$output" "no"

    run_podman update --restart always testctr

    run_podman container inspect testctr --format "{{ .HostConfig.RestartPolicy.Name }}"
    is "$output" "always"

    # Ensure the container is alive
    wait_for_file ${PODMAN_TMPDIR}/alive

    rm -f ${PODMAN_TMPDIR}/alive
    rm -f ${PODMAN_TMPDIR}/sentinel

    # Restart should ensure that the container comes back up and recreates the file
    wait_for_file ${PODMAN_TMPDIR}/alive

    run_podman rm -f -t0 testctr
}

# HealthCheck configuration

function nrand() {
   # 1-59 seconds. Don't exceed 59, because podman then shows as "1mXXs"
    echo $((1 + RANDOM % 58))
}

# bats test_tags=ci:parallel
@test "podman update - test all HealthCheck flags" {
    local ctrname="c-h-$(safename)"
    local msg="healthmsg-$(random_string)"
    local TMP_DIR_HEALTHCHECK="$PODMAN_TMPDIR/healthcheck"
    mkdir $TMP_DIR_HEALTHCHECK

    # flag-name      | value                | inspect format, .Config.Xxx
    tests="
    cmd              | echo $msg            | Healthcheck.Test
    interval         | $(nrand)s            | Healthcheck.Interval
    log-destination  | $TMP_DIR_HEALTHCHECK | HealthLogDestination
    max-log-count    | $(nrand)             | HealthMaxLogCount
    max-log-size     | $(nrand)             | HealthMaxLogSize
    on-failure       | restart              | HealthcheckOnFailureAction
    retries          | $(nrand)             | Healthcheck.Retries
    timeout          | $(nrand)s            | Healthcheck.Timeout
    start-period     | $(nrand)s            | Healthcheck.StartPeriod
    startup-cmd      | echo $msg            | StartupHealthCheck.Test
    startup-interval | $(nrand)s            | StartupHealthCheck.Interval
    startup-retries  | $(nrand)             | StartupHealthCheck.Retries
    startup-success  | $(nrand)             | StartupHealthCheck.Successes
    startup-timeout  | $(nrand)s            | StartupHealthCheck.Timeout
    "

    run_podman run -d --name $ctrname $IMAGE top
    cid="$output"

    # Pass 1: read the table above, gather up the options, format and expected values
    local -a opts
    local -A formats
    local -A checks
    while read opt value format ; do
        fullopt="--health-$opt=$value"
        opts+=("$fullopt")
        formats["$fullopt"]="{{.Config.$format}}"
        expected=$value
        # Special case for commands
        if [[ $opt =~ cmd ]]; then
            expected="[CMD-SHELL $value]"
        fi
        checks["$fullopt"]=$expected
    done < <(parse_table "$tests")

    # Now do the update in one fell swoop
    run_podman update "${opts[@]}" $ctrname

    # ...and check one by one
    defer-assertion-failures
    for opt in "${opts[@]}"; do
        run_podman inspect $ctrname --format "${formats[$opt]}"
        assert "$output" == "${checks[$opt]}" "$opt"
    done
    immediate-assertion-failures

    # Clean up
    run_podman rm -f -t0 $cid
}

# bats test_tags=ci:parallel
@test "podman update - test HealthCheck flags without HealthCheck commands" {
    local ctrname="c-h-$(safename)"

    # flag-name=value
    tests="
    interval=10s
    retries=5
    timeout=10s
    start-period=10s
    startup-interval=10s
    startup-retries=5
    startup-success=10
    startup-timeout=10s
    "

    run_podman run -d --name $ctrname $IMAGE top
    cid="$output"

    defer-assertion-failures
    for opt in $tests; do
        run_podman 125 update "--health-$opt" $ctrname
        assert "$output" =~ "healthcheck command is not set" "--$opt with no startup"
    done
    immediate-assertion-failures

    run_podman rm -f -t0 $cid
}

# bats test_tags=ci:parallel
@test "podman update - --no-healthcheck" {
    local msg="healthmsg-$(random_string)"
    local ctrname="c-h-$(safename)"

    run_podman run -d --name $ctrname                    \
                --health-cmd "echo $msg"                 \
                --health-startup-cmd "echo startup$msg"  \
                $IMAGE /home/podman/pause
    cid="$output"

    run_podman update $ctrname --no-healthcheck

    run_podman inspect $ctrname --format {{.Config.Healthcheck.Test}}
    assert "$output" == "[NONE]" "HealthCheck command is disabled"

    run_podman inspect $ctrname --format {{.Config.StartupHealthCheck}}
    assert "$output" == "<nil>" "startup HealthCheck command is disabled"

    run_podman rm -t 0 -f $ctrname
}

# bats test_tags=ci:parallel
@test "podman update - check behavior - change cmd and destination healthcheck" {
    local TMP_DIR_HEALTHCHECK="$PODMAN_TMPDIR/healthcheck"
    mkdir $TMP_DIR_HEALTHCHECK
    local ctrname="c-h-$(safename)"
    local msg="healthmsg-$(random_string)"

    run_podman run -d --name $ctrname     \
                --health-cmd "echo $msg"  \
                $IMAGE /home/podman/pause
    cid="$output"

    run_podman healthcheck run $ctrname
    is "$output" "" "output from 'podman healthcheck run'"

    # Run podman update in two separate runs to make sure HealthCheck is overwritten correctly.
    run_podman update $ctrname --health-cmd "echo healthmsg-new"

    run_podman update $ctrname --health-log-destination $TMP_DIR_HEALTHCHECK

    run_podman healthcheck run $ctrname
    is "$output" "" "output from 'podman healthcheck run'"

    healthcheck_log_path="${TMP_DIR_HEALTHCHECK}/${cid}-healthcheck.log"
    # The healthcheck is triggered by the podman when the container is started, but its execution depends on systemd.
    # And since `run_podman healthcheck run` is also run manually, it will result in two runs.
    count=$(grep -co "healthmsg-new" $healthcheck_log_path)
    assert "$count" -ge 1 "Number of matching health log messages"

    run_podman rm -t 0 -f $ctrname
}

# bats test_tags=ci:parallel
@test "podman update - resources on update are not changed unless requested" {
    local ctrname="c-h-$(safename)"
    run_podman run -d --name $ctrname \
                --pids-limit 1024     \
                $IMAGE /home/podman/pause

    run_podman update $ctrname --memory 100M

    # A Pid check is performed to ensure that other resource settings are not unset. https://github.com/containers/podman/issues/24610
    run_podman inspect $ctrname --format "{{.HostConfig.Memory}}\n{{.HostConfig.PidsLimit}}"
    assert ${lines[0]} == "104857600" ".HostConfig.Memory"
    assert ${lines[1]} == "1024" ".HostConfig.PidsLimit"

    run_podman rm -t 0 -f $ctrname
}

# bats test_tags=ci:parallel
@test "podman update - non-block device rejected by --*device* options" {
    local dev=/dev/zero # Not a block device.
    local block_opts=(
        "--blkio-weight-device=$dev:123"
        "--device-read-bps=$dev:10mb"
        "--device-write-bps=$dev:10mb"
        "--device-read-iops=$dev:1000"
        "--device-write-iops=$dev:1000"
    )
    run_podman run -d "$IMAGE" /home/podman/pause
    cid="$output"

    defer-assertion-failures
    for opt in "${block_opts[@]}"; do
        run_podman 125 update "$cid" "$opt"
        assert "$output" =~ "$dev: not a block device"
    done
    immediate-assertion-failures

    run_podman rm -t 0 -f "$cid"
}

# bats test_tags=ci:parallel
@test "podman update - set ulimits" {
    local ctrname="c-h-$(safename)"

    run_podman run -d --name $ctrname $IMAGE sh -c "echo 'nofile:' \$(ulimit -n); echo 'nproc:' \$(ulimit -u); top"

    # Update with single ulimit
    run_podman update --ulimit nofile=1024:1024 $ctrname

    # Restart container to apply ulimit changes
    run_podman restart $ctrname

    # Verify the nofile ulimit was updated in the main process
    run_podman logs $ctrname
    assert "$output" =~ "nofile: 1024" "nofile ulimit updated to 1024 in main process"

    # Update with multiple ulimits
    run_podman update --ulimit nofile=2048:2048 --ulimit nproc=512:512 $ctrname
    run_podman restart $ctrname
    run_podman logs $ctrname
    assert "$output" =~ "nofile: 2048" "nofile ulimit updated to 2048 in main process"
    assert "$output" =~ "nproc: 512" "nproc ulimit updated to 512 in main process"

    # Capture host values by running a temporary container with --ulimit host option
    run_podman run --rm --ulimit host $IMAGE sh -c "echo 'nofile:' \$(ulimit -n); echo 'nproc:' \$(ulimit -u)"
    local host_nofile=$(echo "$output" | awk '/^nofile:/ {print $2}')
    local host_nproc=$(echo "$output" | awk '/^nproc:/ {print $2}')

    # Update with -1 for unlimited
    run_podman update --ulimit nproc=-1:-1 $ctrname
    run_podman restart $ctrname
    run_podman logs $ctrname
    if is_rootless; then
        assert "$output" =~ "nproc: $host_nproc" "rootless ulimit should clamp to host limit after restart with -1"
    else
        assert "$output" =~ "nproc: unlimited" "ulimit should be unlimited after restart with -1"
    fi

    # Update with --ulimit host option
    run_podman update --ulimit host $ctrname
    run_podman restart $ctrname
    run_podman logs $ctrname
    assert "$output" =~ "nofile: $host_nofile" "nofile ulimit should match host value ($host_nofile)"
    assert "$output" =~ "nproc: $host_nproc" "nproc ulimit should match host value ($host_nproc)"

    # Error case - update with invalid ulimit syntax
    run_podman 125 update --ulimit nofile:1024:1024 $ctrname
    assert "$output" =~ "invalid ulimit argument" "Invalid ulimit syntax should fail"

    # Error case - update with invalid ulimit values (soft > hard)
    run_podman 125 update --ulimit nofile=2048:1024 $ctrname
    assert "$output" =~ "ulimit soft limit must be less than or equal to hard limit" "Invalid ulimit values should fail"

    # Clean up
    run_podman rm -t 0 -f $ctrname
}

# vim: filetype=sh
