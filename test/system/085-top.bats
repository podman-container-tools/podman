#!/usr/bin/env bats

load helpers

@test "podman top - basic tests" {
    run_podman run -d $IMAGE top
    cid=$output
    is "$cid" "[0-9a-f]\{64\}$"

    run_podman top $cid
    is "${lines[0]}" "USER[ \t]*PID[ \t]*PPID[ \t]*%CPU[ \t]*ELAPSED[ \t]*TTY[ \t]*TIME[ \t]*COMMAND" "podman top"
    is "${lines[1]}" "root[ \t]*1[ \t]*0[ \t]*0.000[ \t]*" "podman top"

    run_podman top $cid -eo pid,comm
    is "${lines[0]}" "[ \t]*PID[ \t]*COMMAND" "podman top -eo pid,comm Heading"
    is "${lines[1]}" "[ \t]*1[ \t]*top" "podman top -eo pid,comm processes"

    run_podman top $cid -eo "pid comm"
    is "${lines[0]}" "[ \t]*PID[ \t]*COMMAND" "podman top -eo "pid comm" Heading"
    is "${lines[1]}" "[ \t]*1[ \t]*top" "podman top -eo "pid comm" processes"

    run_podman rm -f $cid
}

# vim: filetype=sh
