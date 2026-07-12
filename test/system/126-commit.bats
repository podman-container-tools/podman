#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman commit
#

load helpers

# Helper: use poll(2) on a cgroup.events file to detect state changes
# without polling.
_cgroup_watch() {
    python3 -c "
import select, os, sys
fd = os.open(sys.argv[1], os.O_RDONLY)
os.read(fd, 4096)
p = select.poll()
p.register(fd, select.POLLPRI | select.POLLERR)
print('poll ready', flush=True)
events = p.poll(int(sys.argv[2]))
os.close(fd)
print('event' if events else 'no-event')
" "$@"
}

# bats test_tags=ci:parallel
@test "podman commit --pause default" {
    type -p python3 &>/dev/null || skip "python3 needed for cgroup poll(2)"

    cname="c-$(safename)"
    run_podman run -d --name $cname $IMAGE sleep infinity

    run_podman inspect --format '{{.State.Pid}}' $cname
    local pid="$output"

    local cgpath
    cgpath=$(< /proc/$pid/cgroup)
    cgpath=${cgpath#*::}
    local events_file="/sys/fs/cgroup${cgpath}/cgroup.events"
    if [ ! -f "$events_file" ]; then
        skip "cannot find cgroup.events for container"
    fi

    local watch_log="${PODMAN_TMPDIR}/cgroup-watch.log"

    # Start a background watcher before commit.
    _cgroup_watch "$events_file" 10000 > "$watch_log" &
    local watcher_pid=$!
    # Wait for the watcher to be ready before triggering commit.
    retries=50
    while ! grep -q "poll ready" "$watch_log" && [ $retries -gt 0 ]; do
        sleep 0.1
        retries=$((retries - 1))
    done

    # Commit without explicit --pause flag: default is true, so the
    # cgroup must receive a freeze event.
    local imgname="i-$(safename)"
    run_podman commit -q $cname $imgname
    wait $watcher_pid || true
    assert "$(tail -1 $watch_log)" = "event" \
           "commit should pause (freeze) the container by default"

    # Now test --pause=false: no freeze event expected.
    _cgroup_watch "$events_file" 3000 > "$watch_log" &
    watcher_pid=$!
    retries=50
    while ! grep -q "poll ready" "$watch_log" && [ $retries -gt 0 ]; do
        sleep 0.1
        retries=$((retries - 1))
    done

    run_podman commit -q --pause=false $cname "${imgname}-nopause"
    wait $watcher_pid || true
    assert "$(tail -1 $watch_log)" = "no-event" \
           "commit --pause=false should not freeze the container"

    # Clean up
    run_podman rm -f -t0 $cname
    run_podman rmi $imgname "${imgname}-nopause"
}

# vim: filetype=sh
