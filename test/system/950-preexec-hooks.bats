#!/usr/bin/env bats
#
# Tests for podman preexec hooks
#

load helpers
load helpers.network

# The existence of this file allows preexec hooks to run.
preexec_hook_ok_file=/etc/containers/podman_preexec_hooks.txt

function setup() {
    basic_setup
}

function teardown() {
    if [[ -n "$preexec_hook_ok_file" ]]; then
        sudo -n rm -f $preexec_hook_ok_file || true
    fi

    basic_teardown
}

@test "podman preexec hook" {
    # This file does not exist on any CI system nor any developer system
    # nor actually anywhere in the universe except a small small set of
    # places with very specific requirements. If we find this file on
    # our test system, it could be a leftover from prior testing, or
    # basically just something very weird. So, fail loudly if we see it.
    # No podman developer ever wants this file to exist.
    if [[ -e $preexec_hook_ok_file ]]; then
        # Unset the variable, so we don't delete it in teardown
        msg="File already exists (it should not): $preexec_hook_ok_file"
        preexec_hook_ok_file=

        die "$msg"
    fi

    # Good. File does not exist. Now see if we can TEMPORARILY create it.
    sudo -n touch $preexec_hook_ok_file || skip "test requires sudo"

    preexec_hook_dir=$PODMAN_TMPDIR/auth
    mkdir -p $preexec_hook_dir
    preexec_hook_script=$preexec_hook_dir/pull_check.sh

    cat > $preexec_hook_script <<EOF
#!/bin/sh
if echo \$@ | grep "pull foobar"; then
    exit 42
fi
exit 43
EOF
    chmod +x $preexec_hook_script

    PODMAN_PREEXEC_HOOKS_DIR=$preexec_hook_dir run_podman 42 pull foobar
    PODMAN_PREEXEC_HOOKS_DIR=$preexec_hook_dir run_podman 43 version

    sudo -n rm -f $preexec_hook_ok_file || true

    # no hooks-ok file, everything should now work again (HOOKS_DIR is ignored)
    PODMAN_PREEXEC_HOOKS_DIR=$preexec_hook_dir run_podman version
}
