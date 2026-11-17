#!/usr/bin/env bats   -*- bats -*-
#
# Basic tests for podman logs
#

load helpers

# bats test_tags=ci:parallel
@test "podman logs - basic test" {
    rand_string=$(random_string 40)

    run_podman create $IMAGE echo $rand_string
    cid="$output"

    run_podman logs $cid
    is "$output" ""  "logs on created container: empty"

    run_podman start --attach --interactive $cid
    is "$output" "$rand_string" "output from podman-start on created ctr"
    is "$output" "$rand_string" "logs of started container"

    run_podman logs $cid
    is "$output" "$rand_string" "output from podman-logs after container is run"

    # test --since with Unix timestamps
    run_podman logs --since 1000 $cid

    # test --until with Unix timestamps
    run_podman logs --until 1000 $cid

    run_podman rm $cid
}

function _log_test_tail() {
    local driver=$1

    run_podman run -d --log-driver=$driver $IMAGE sh -c "echo test1; echo test2"
    cid="$output"

    run_podman wait $cid
    run_podman logs --tail 1 --timestamps $cid
    log1="$output"
    assert "$log1" =~ "^[0-9-]+T[0-9:.]+([\+-][0-9:]+|Z) test2" \
           "logs should only show last line"

    run_podman restart $cid
    run_podman wait $cid

    run_podman logs -t --tail 1 $cid
    log2="$output"
    assert "$log2" =~ "^[0-9-]+T[0-9:.]+([\+-][0-9:]+|Z) test2" \
           "logs, after restart, shows only last line"

    assert "$log2" != "$log1" "log timestamps should differ"

    run_podman rm $cid
}

# CANNOT BE PARALLELIZED - #23750, events-backend=file cannot coexist with journal
@test "podman logs - tail test, k8s-file" {
    _log_test_tail k8s-file
}

# bats test_tags=ci:parallel
@test "podman logs - tail test, journald" {
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    _log_test_tail journald
}

function _additional_events_backend() {
    local driver=$1
    # Since PR#10431, 'logs -f' with journald driver is only supported with journald events backend.
    if [[ $driver = "journald" ]]; then
        run_podman info --format '{{.Host.EventLogger}}' >/dev/null
        if [[ $output != "journald" ]]; then
            echo "--events-backend journald"
        fi
    fi
}

function _log_test_multi() {
    local driver=$1

    skip_if_remote "logs does not support multiple containers when run remotely"

    local events_backend=$(_additional_events_backend $driver)

    # Simple helper to make the container starts, below, easier to read
    local cname1="c-ltm-1-$(safename)"
    local cname2="c-ltm-2-$(safename)"
    local -a cid
    doit() {
        run_podman ${events_backend} run --log-driver=$driver -d \
            --name "$1" $IMAGE sh -c "$2";
        cid+=($(echo "${output:0:12}"))
    }

    doit $cname1 "echo a1; echo a2"
    doit $cname2 "echo b1; echo b2"

    # Reading logs only guarantees the order for a single container,
    # when using multiple containers the line order between them can vary.
    run_podman ${events_backend} logs -f $cname1 $cname2
    assert "$output" =~ \
       ".*^${cid[0]} a1\$.*
${cid[0]} a2"   "Sequential output from c1"
    assert "$output" =~ \
       ".*^${cid[1]} b1\$.*
${cid[1]} b2"   "Sequential output from c2"

    run_podman rm -f -t0 ${cid[0]} ${cid[1]}
}

# CANNOT BE PARALLELIZED - #23750, events-backend=file cannot coexist with journal
@test "podman logs - multi k8s-file" {
    _log_test_multi k8s-file
}

# bats test_tags=ci:parallel
@test "podman logs - multi journald" {
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    _log_test_multi journald
}

function _log_test_restarted() {
    local driver=$1
    local events_backend=$(_additional_events_backend $driver)
    if [[ -n "${events_backend}" ]]; then
        skip_if_remote "remote does not support --events-backend"
    fi
    cname="c-ltr-$(safename)"
    run_podman run --log-driver=$driver ${events_backend} --name $cname $IMAGE sh -c 'start=0; if test -s log; then start=`tail -n 1 log`; fi; seq `expr $start + 1` `expr $start + 10` | tee -a log'
    run_podman ${events_backend} start -a $cname
    logfile=$(mktemp -p ${PODMAN_TMPDIR} logfileXXXXXXXX)
    "${PODMAN_CMD[@]}" $_PODMAN_TEST_OPTS ${events_backend} logs -f $cname > $logfile
    expected=$(mktemp -p ${PODMAN_TMPDIR} expectedXXXXXXXX)
    seq 1 20  > $expected
    diff -u ${expected} ${logfile}

    run_podman rm -f -t0 $cname
}

# CANNOT BE PARALLELIZED - #23750, events-backend=file cannot coexist with journal
@test "podman logs restarted - k8s-file" {
    _log_test_restarted k8s-file
}

# bats test_tags=ci:parallel
@test "podman logs restarted journald" {
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    _log_test_restarted journald
}

# CANNOT BE PARALLELIZED - #23750, events-backend=file cannot coexist with journal
@test "podman logs - journald log driver requires journald events backend" {
    skip_if_remote "remote does not support --events-backend"
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    cname="c-$(safename)"
    run_podman --events-backend=file run --log-driver=journald -d --name $cname --replace $IMAGE ls /
    run_podman --events-backend=file logs $cname
    run_podman 125 --events-backend=file logs --follow $cname
    is "$output" "Error: using --follow with the journald --log-driver but without the journald --events-backend (file) is not supported" "journald logger requires journald eventer"
    run_podman rm $cname
}

function _log_test_since() {
    local driver=$1

    s_before="before_$(random_string)_${driver}"
    s_after="after_$(random_string)_${driver}"

    cname="c-lts-$(safename)"
    before=$(date --iso-8601=seconds)
    run_podman run --log-driver=$driver -d --name $cname $IMAGE sh -c \
        "echo $s_before; trap 'echo $s_after; exit' SIGTERM; while :; do sleep 0.1; done"
    wait_for_output "$s_before" $cname

    # sleep a second to make sure the date is after the first echo
    # (We could instead use iso-8601=ns but seconds feels more real-world)
    sleep 1
    after=$(date --iso-8601=seconds)
    run_podman stop $cname

    run_podman logs $cname
    is "$output" \
        "$s_before
$s_after"

    run_podman logs --since $before $cname
    is "$output" \
        "$s_before
$s_after"

    run_podman logs --since $after $cname
    is "$output" "$s_after"
    run_podman rm -t 1 -f $cname
}

# CANNOT BE PARALLELIZED - #23750, events-backend=file cannot coexist with journal
@test "podman logs - since k8s-file" {
    _log_test_since k8s-file
}

# bats test_tags=ci:parallel
@test "podman logs - since journald" {
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    _log_test_since journald
}

function _log_test_until() {
    local driver=$1

    s_before="before_$(random_string)_${driver}"
    s_after="after_$(random_string)_${driver}"

    cname="c-ltu-$(safename)"
    before=$(date --iso-8601=seconds)
    sleep 1
    run_podman run --log-driver=$driver -d --name $cname $IMAGE sh -c \
        "echo $s_before; trap 'echo $s_after; exit' SIGTERM; while :; do sleep 0.1; done"

    # sleep a second to make sure the date is after the first echo
    sleep 1
    run_podman stop $cname
    run_podman wait $cname

    # Sigh. Stupid journald has a lag. Wait a few seconds for it to catch up.
    retries=20
    s_both="$s_before
$s_after"
    while [[ $retries -gt 0 ]]; do
        run_podman logs $cname
        if [[ "$output" = "$s_both" ]]; then
            break
        fi
        retries=$((retries - 1))
        sleep 0.1
    done
    assert $retries -gt 0 \
           "Timed out waiting for before&after in podman logs: $output"

    run_podman logs --until $before $cname
    is "$output" "" "podman logs --until before"

    after=$(date --date='+1 second' --iso-8601=ns)

    run_podman logs --until $after $cname
    is "$output" "$s_both" "podman logs --until after"
    run_podman rm -t 0 -f $cname
}

# CANNOT BE PARALLELIZED - #23750, events-backend=file cannot coexist with journal
@test "podman logs - until k8s-file" {
    _log_test_until k8s-file
}

# bats test_tags=ci:parallel
@test "podman logs - until journald" {
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    _log_test_until journald
}

function _log_test_follow() {
    local driver=$1
    cname="c-ltf-$(safename)"
    contentA=$(random_string)
    contentB=$(random_string)
    contentC=$(random_string)
    local events_backend=$(_additional_events_backend $driver)

    if [[ -n "${events_backend}" ]]; then
        skip_if_remote "remote does not support --events-backend"
    fi

    # Note: it seems we need at least three log lines to hit #11461.
    run_podman ${events_backend} run --log-driver=$driver --name $cname $IMAGE sh -c "echo $contentA; echo $contentB; echo $contentC"
    run_podman ${events_backend} logs -f $cname
    is "$output" "$contentA
$contentB
$contentC" "logs -f on exited container works"

    run_podman ${events_backend} rm -t 0 -f $cname
}

# CANNOT BE PARALLELIZED - #23750, events-backend=file cannot coexist with journal
@test "podman logs - --follow k8s-file" {
    _log_test_follow k8s-file
}

# bats test_tags=ci:parallel
@test "podman logs - --follow journald" {
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    _log_test_follow journald
}

function _log_test_follow_since() {
    local driver=$1
    cname="c-ltfs-$(safename)"
    content=$(random_string)
    local events_backend=$(_additional_events_backend $driver)

    if [[ -n "${events_backend}" ]]; then
        skip_if_remote "remote does not support --events-backend"
    fi

    run_podman ${events_backend} run --log-driver=$driver --name $cname $IMAGE echo "$content"
    # Using --since 0s can flake because the log might written in the second as the logs call is made.
    # The -1s makes sure we only read logs that would be created 1s in the future which cannot happen.
    run_podman ${events_backend} logs --since -1s -f $cname
    assert "$output" == "" "logs --since -f on exited container works"

    run_podman ${events_backend} rm -t 0 -f $cname

    # Now do the same with a running container to check #16950.
    run_podman ${events_backend} run --log-driver=$driver --name $cname -d $IMAGE \
        sh -c "sleep 1; while :; do echo $content && sleep 1; done"

    # sleep is required to make sure the podman event backend no longer sees the start event in the log
    # This value must be greater or equal than the value given in --since below
    sleep 0.2

    # Make sure podman logs actually follows by giving a low timeout and check that the command times out
    # Option after container name should work as well: https://github.com/containers/podman/issues/25653
    PODMAN_TIMEOUT=3 run_podman 124 ${events_backend} logs --since 0.1s $cname -f
    assert "$output" =~ "$content
timeout: sending signal TERM to command.*" "logs --since -f on running container works"

    run_podman ${events_backend} rm -t 0 -f $cname
}

# CANNOT BE PARALLELIZED - #23750, events-backend=file cannot coexist with journal
@test "podman logs - --since --follow k8s-file" {
    _log_test_follow_since k8s-file
}

# bats test_tags=ci:parallel
@test "podman logs - --since --follow journald" {
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    _log_test_follow_since journald
}

function _log_test_follow_until() {
    local driver=$1
    cname="c-ltfu-$(safename)"
    content=$(random_string)
    local events_backend=$(_additional_events_backend $driver)

    if [[ -n "${events_backend}" ]]; then
        skip_if_remote "remote does not support --events-backend"
    fi

    run_podman ${events_backend} run --log-driver=$driver --name $cname -d $IMAGE \
        sh -c "n=1;while :; do echo $content--\$n; n=\$((n+1));sleep 0.1; done"

    t0=$(date +%s%3N)
    # The logs command should exit after the until time even when follow is set
    PODMAN_TIMEOUT=10 run_podman ${events_backend} logs --until 3s -f $cname
    t1=$(date +%s%3N)
    logs_seen="$output"

    # The delta should be 3 but could be longer on a slow CI system
    delta_t_ms=$(( $t1 - $t0 ))
    assert $delta_t_ms -gt 2000 "podman logs --until: exited too early!"
    assert $delta_t_ms -lt 5000 "podman logs --until: exited too late!"

    # Impossible to know how many lines we'll see, but require at least two
    assert "$logs_seen" =~ "$content--1
$content--2.*" "logs --until -f on running container works"

    run_podman ${events_backend} rm -t 0 -f $cname
}

# CANNOT BE PARALLELIZED - #23750, events-backend=file cannot coexist with journal
@test "podman logs - --until --follow k8s-file" {
    _log_test_follow_until k8s-file
}

# bats test_tags=ci:parallel
@test "podman logs - --until --follow journald" {
    # We can't use journald on RHEL as rootless: rhbz#1895105
    skip_if_journald_unavailable

    _log_test_follow_until journald
}

# https://github.com/containers/podman/issues/19545
# CANNOT BE PARALLELIZED - #23750, events-backend=file cannot coexist with journal
@test "podman logs --tail, k8s-file with partial lines" {
    cname="c-$(safename)"

    # "-t" gives us ^Ms (CRs) in the log
    run_podman run --name $cname --log-driver k8s-file -t $IMAGE echo hi

    # Hand-craft a log file with partial lines and carriage returns
    run_podman inspect --format '{{.HostConfig.LogConfig.Path}}' $cname
    logpath="$output"
    timestamp=$(head -n1 "$logpath" | awk '{print $1}')
    cr=$'\r'
    nl=$'\n'
    # Delete, don't overwrite, in case conmon still has the fd open
    rm -f $logpath
    cat > $logpath <<EOF
$timestamp stdout F podman1$cr
$timestamp stdout P podman2
$timestamp stdout F $cr
$timestamp stdout F podman3$cr
EOF

    # FIXME: remove after 2024-01-01 if no more flakes seen.
    cat -vET $logpath

    expect1="podman3${cr}"
    expect2="podman2${cr}${nl}podman3${cr}"
    expect3="podman1${cr}${nl}podman2${cr}${nl}podman3${cr}"

    # This always worked
    run_podman logs --tail 1 $cname
    assert "$output" = "$expect1" "--tail 1"

    # Prior to this PR, the first line would be "^M" without the podman
    run_podman logs --tail 2 $cname
    assert "$output" = "$expect2" "--tail 2"

    # Confirm that we won't overrun
    for i in 3 4 5; do
        run_podman logs --tail $i $cname
        assert "$output" = "$expect3" "--tail $i"
    done

    run_podman rm $cname
}

# vim: filetype=sh
