#!/usr/bin/env bats   -*- bats -*-
# shellcheck disable=SC2096
#
# Tests for podman build
#

load helpers

@test "podman --ipc=host" {
    hostipc="$(readlink /proc/self/ns/ipc)"
    run_podman run --name IPC --ipc=host $IMAGE readlink /proc/self/ns/ipc
    is "$output" "$hostipc" "HostIPC and container IPC should be same"
    run_podman inspect IPC --format '{{ .HostConfig.IpcMode }}'
    is "$output" "host" "host mode should be selected"
    run_podman rm IPC
}

@test "podman --ipc=none" {
    hostipc="$(readlink /proc/self/ns/ipc)"
    run_podman run --ipc=none --name IPC $IMAGE readlink /proc/self/ns/ipc
    assert "$output" != "$hostipc" "containeripc should != hostipc"
    run_podman inspect IPC --format '{{ .HostConfig.IpcMode }}'
    is "$output" "none" "none mode should be selected"
    run_podman rm IPC

    run_podman 1 run --rm --ipc=none $IMAGE ls /dev/shm
    is "$output" "ls: /dev/shm: No such file or directory" "Should fail with missing /dev/shm"
}

@test "podman --ipc=private" {
    hostipc="$(readlink /proc/self/ns/ipc)"
    run_podman run -d --ipc=private --name test $IMAGE sleep 100
    assert "$output" != "$hostipc" "containeripc should != hostipc"
    run_podman inspect test --format '{{ .HostConfig.IpcMode }}'
    is "$output" "private" "private mode should be selected"

    run_podman 125 run --ipc=container:test --rm $IMAGE readlink /proc/self/ns/ipc
    is "$output" ".*is not allowed: non-shareable IPC (hint: use IpcMode:shareable for the donor container)" "Containers should not share private ipc namespace"
    run_podman stop -t 0 test
    run_podman rm test
}

@test "podman --ipc=shareable" {
    hostipc="$(readlink /proc/self/ns/ipc)"
    run_podman run -d --ipc=shareable --name test $IMAGE sleep 100
    assert "$output" != "$hostipc" "containeripc(shareable) should != hostipc"
    run_podman inspect test --format '{{ .HostConfig.IpcMode }}'
    is "$output" "shareable" "shareable mode should be selected"

    run_podman run --ipc=container:test --rm $IMAGE readlink /proc/self/ns/ipc
    assert "$output" != "$hostipc" "containeripc(:test) should != hostipc"

    run_podman stop -t 0 test
    run_podman rm test
}

@test "podman --ipc=container@test" {
    hostipc="$(readlink /proc/self/ns/ipc)"
    run_podman run -d --name test $IMAGE sleep 100
    containerid=$output
    run_podman inspect test --format '{{ .HostConfig.IpcMode }}'
    is "$output" "shareable" "shareable mode should be selected"
    run_podman exec test readlink /proc/self/ns/ipc
    assert "$output" != "$hostipc" "containeripc(exec) should != hostipc"

    testipc=$output
    run_podman run --name IPC --ipc=container:test $IMAGE readlink /proc/self/ns/ipc
    assert "$output" = "$testipc" "Containers should share ipc namespace"
    run_podman inspect IPC --format '{{ .HostConfig.IpcMode }}'
    is "$output" "container:$containerid" "ipc mode should be selected"
    run_podman rm IPC

    run_podman stop -t 0 test
    run_podman rm test
}

# vim: filetype=sh
