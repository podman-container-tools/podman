# -*- bash -*-

load ../system/helpers.bash

export FARM_TMPDIR=$(mktemp -d --tmpdir=${BATS_TMPDIR:-/tmp} podman_bats.XXXXXX)

function setup(){
    basic_setup

    # Always create the same containerfile
    cat >$FARM_TMPDIR/Containerfile <<EOF
FROM $IMAGE
RUN arch | tee /arch.txt
RUN date | tee /built.txt
EOF
}

function teardown(){
    basic_teardown
}
