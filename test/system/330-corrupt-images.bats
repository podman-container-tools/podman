#!/usr/bin/env bats   -*- bats -*-
#
# All tests in here perform nasty manipulations on image storage.
#

load helpers

###############################################################################
# BEGIN setup/teardown

# Create a scratch directory; this is what we'll use for image store and cache
if [ -z "${PODMAN_CORRUPT_TEST_WORKDIR}" ]; then
    export PODMAN_CORRUPT_TEST_WORKDIR=$(mktemp -d --tmpdir=${BATS_TMPDIR:-${TMPDIR:-/tmp}} podman_corrupt_test.XXXXXX)
fi

PODMAN_CORRUPT_TEST_IMAGE_CANONICAL_FQIN=quay.io/libpod/alpine@sha256:634a8f35b5f16dcf4aaa0822adc0b1964bb786fca12f6831de8ddc45e5986a00
PODMAN_CORRUPT_TEST_IMAGE_TAGGED_FQIN=${PODMAN_CORRUPT_TEST_IMAGE_CANONICAL_FQIN%%@sha256:*}:test
PODMAN_CORRUPT_TEST_IMAGE_ID=961769676411f082461f9ef46626dd7a2d1e2b2a38e6a44364bcbecf51e66dd4

function setup() {
    skip_if_remote "none of these tests run under podman-remote"

    # DANGER! This completely changes the behavior of run_podman,
    # forcing it to use a quarantined directory. Make certain that
    # it gets unset in teardown.
    #
    # --pull-option=convert_images=true is incompatible with VFS, and can be set when testing composefs.
    _PODMAN_TEST_OPTS="--storage-driver=vfs --pull-option=convert_images=false $(podman_isolation_opts ${PODMAN_CORRUPT_TEST_WORKDIR})"
}

function teardown() {
    # No other tests should ever run with these scratch options
    unset _PODMAN_TEST_OPTS

    is_remote && return

    # Clean up
    umount ${PODMAN_CORRUPT_TEST_WORKDIR}/root/overlay || true
    if is_rootless; then
        run_podman unshare rm -rf ${PODMAN_CORRUPT_TEST_WORKDIR}/root
    else
        rm -rf ${PODMAN_CORRUPT_TEST_WORKDIR}/root
    fi
}

# END   setup/teardown
###############################################################################
# BEGIN primary test helper

# This is our main action, invoked by every actual test. It:
#    - creates a new empty rootdir
#    - populates it with our crafted test image
#    - removes [ manifest, blob ]
#    - confirms that "podman images" throws an error
#    - runs the specified command (rmi -a -f, prune, reset, etc)
#    - confirms that it succeeds, and also emits expected warnings
function _corrupt_image_test() {
    # Run this test twice: once removing manifest, once removing blob
    for what_to_rm in manifest blob; do
        # I have no idea, but this sometimes remains mounted
        umount ${PODMAN_CORRUPT_TEST_WORKDIR}/root/overlay || true
        # Start with a fresh storage root, load prefetched image into it.
        /bin/rm -rf ${PODMAN_CORRUPT_TEST_WORKDIR}/root
        mkdir -p ${PODMAN_CORRUPT_TEST_WORKDIR}/root
        run_podman load -i ${PODMAN_CORRUPT_TEST_WORKDIR}/img.tar
        # "podman load" restores it without a tag, which (a) causes rmi-by-name
        # to fail, and (b) causes "podman images" to exit 0 instead of 125
        run_podman tag ${PODMAN_CORRUPT_TEST_IMAGE_ID} ${PODMAN_CORRUPT_TEST_IMAGE_TAGGED_FQIN}

        # shortcut variable name
        local id=${PODMAN_CORRUPT_TEST_IMAGE_ID}

        case "$what_to_rm" in
            manifest)  rm_path=manifest ;;
            blob)      rm_path="=$(echo -n "sha256:$id" | base64 -w0)" ;;
            *)         die "Internal error: unknown action '$what_to_rm'" ;;
        esac

        # Corruptify, and confirm that 'podman images' throws an error
        rm -v ${PODMAN_CORRUPT_TEST_WORKDIR}/root/*-images/$id/${rm_path}
        run_podman 125 images
        is "$output" "Error: locating item named \".*\" for image with ID \"$id\" (consider removing the image to resolve the issue): file does not exist.*"

        # Run the requested command. Confirm it succeeds, with suitable warnings.
        run_podman 0+w $*
        # There are three different variations on the warnings, allow each...
        allow_warnings "Failed to determine parent of image: .*, ignoring the error" \
                       "Failed to determine if an image is a parent: .*, ignoring the error" \
                       "Failed to determine if an image is a manifest list: .*, ignoring the error"
        # ...but make sure we get at least one
        require_warning "Failed to determine (parent|if an image is) .*, ignoring the error"

        run_podman images -a --noheading
        is "$output" "" "podman images -a, after $*, is empty"
    done
}

# END   primary test helper
###############################################################################
# BEGIN first "test" does a one-time pull of our desired image

@test "podman corrupt images - initialize" {
    # Pull once, save cached copy.
    run_podman pull $PODMAN_CORRUPT_TEST_IMAGE_CANONICAL_FQIN
    run_podman save -o ${PODMAN_CORRUPT_TEST_WORKDIR}/img.tar \
               $PODMAN_CORRUPT_TEST_IMAGE_CANONICAL_FQIN
}

# END   first "test" does a one-time pull of our desired image
###############################################################################
# BEGIN actual tests

@test "podman corrupt images - rmi -f <image-id>" {
    _corrupt_image_test "rmi -f ${PODMAN_CORRUPT_TEST_IMAGE_ID}"
}

@test "podman corrupt images - rmi -f <image-tagged-name>" {
    _corrupt_image_test "rmi -f ${PODMAN_CORRUPT_TEST_IMAGE_TAGGED_FQIN}"
}

@test "podman corrupt images - rmi -f -a" {
    _corrupt_image_test "rmi -f -a"
}

@test "podman corrupt images - image prune" {
    _corrupt_image_test "image prune -a -f"
}

@test "podman corrupt images - system reset" {
    _corrupt_image_test "system reset -f"
}

# END   actual tests
###############################################################################
# BEGIN final cleanup

@test "podman corrupt images - cleanup" {
    rm -rf ${PODMAN_CORRUPT_TEST_WORKDIR}
}

# END   final cleanup
###############################################################################

# vim: filetype=sh
