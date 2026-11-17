#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman partial pulls support
#

load helpers
load helpers.network
load helpers.registry

###############################################################################
# BEGIN filtering - none of these tests will work with podman-remote

function setup() {
    skip_if_remote "zstd:chunked tests depend on start_registry (requires changing storage options); and on setting specific storage options. Neither works with podman-remote"

    basic_setup
    start_registry
}

# END   filtering - none of these tests will work with podman-remote
###############################################################################
# BEGIN actual tests
# BEGIN primary podman push/pull tests

@test "push and pull zstd chunked image" {
    image1=localhost:${PODMAN_LOGIN_REGISTRY_PORT}/img1-$(safename)

    globalargs="--pull-option enable_partial_images=true"
    pushpullargs="--cert-dir ${PODMAN_LOGIN_WORKDIR}/trusted-registry-cert-dir \
                  --creds ${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS}"

    dockerfile1=$PODMAN_TMPDIR/Dockerfile.1
    cat >$dockerfile1 <<EOF
FROM $IMAGE
RUN touch /
EOF

    run_podman $globalargs build --squash-all -t $image1 -f $dockerfile1 .
    run_podman $globalargs push \
               --compression-format zstd:chunked \
               $pushpullargs \
               $image1

    run_podman $globalargs rmi $image1

    run_podman $globalargs pull \
               $pushpullargs \
               $image1

    run -0 skopeo inspect containers-storage:$image1
    assert "$output" =~ "application/vnd.oci.image.layer.v1.tar\+zstd" "pulled image must be zstd-compressed"

    dockerfile2=$PODMAN_TMPDIR/Dockerfile.2
    cat >$dockerfile2 <<EOF
FROM $image1
RUN touch /new-file
EOF

    dockerfile3=$PODMAN_TMPDIR/Dockerfile.3
    cat >$dockerfile3 <<EOF
FROM $image1
RUN touch /new-file-2
EOF

    image2=localhost:${PODMAN_LOGIN_REGISTRY_PORT}/img2-$(safename)
    image3=localhost:${PODMAN_LOGIN_REGISTRY_PORT}/img3-$(safename)

    run_podman $globalargs build -t $image2 -f $dockerfile2 .
    run_podman $globalargs build -t $image3 -f $dockerfile3 .

    run_podman $globalargs push \
               --compression-format zstd:chunked \
               $pushpullargs \
               $image2

    run_podman $globalargs push \
               --compression-format zstd:chunked \
               $pushpullargs \
               $image3

    run_podman $globalargs diff $image3 $image2
    sorted_output=$(sort <<< $output | tr -d '\n')
    assert "$sorted_output" = "A /new-file-2D /new-file"

    run_podman $globalargs rmi $image2 $image3

    run_podman --log-level debug $globalargs pull \
               $pushpullargs \
               $image2
    if [ "$(podman_storage_driver)" != vfs ]; then # VFS does not implement partial pulls
        assert "$output" =~ "Retrieved partial blob" # A spot check that we are really using the partial-pull code path
    fi

    run -0 skopeo inspect containers-storage:$image2
    assert "$output" =~ "application/vnd.oci.image.layer.v1.tar\+zstd" "pulled image must be zstd-compressed"

    run_podman $globalargs pull \
               $pushpullargs \
               $image3

    run -0 skopeo inspect containers-storage:$image3
    assert "$output" =~ "application/vnd.oci.image.layer.v1.tar\+zstd" "pulled image must be zstd-compressed"

    run_podman $globalargs diff $image3 $image2
    sorted_output=$(sort <<< $output | tr -d '\n')
    assert "$sorted_output" = "A /new-file-2D /new-file"

    for image in $image1 $image2 $image3; do
        push_dir=$PODMAN_TMPDIR/dir-image

        # let's use the dir transport as it gives us directly the uncompressed tar
        run_podman push $image dir:$push_dir

        # grab the inspect data before attempting a save/load cycle
        run_podman inspect $image
        inspect_data=$output

        # Test for #24283: would fail with 'archive/tar: write too long'
        run_podman save -o $PODMAN_TMPDIR/image.tar $image

        # replace the image with a "podman load" from what was stored
        run_podman rmi $image
        run_podman load < $PODMAN_TMPDIR/image.tar

        rm -f $PODMAN_TMPDIR/image.tar

        # validate the data we got from "podman inspect"
        for layer in $(jq -r '.[].RootFS.Layers.[] | gsub("^sha256:"; "")' <<< $inspect_data); do
            layer_file=$push_dir/$layer
            # the checksum for the layer is already validated, but for the sake
            # of the test let's check it again
            run -0 sha256sum < $layer_file
            assert "$output" = "$layer  -" "digest mismatch for layer $layer for $image"
        done
        rm -rf $push_dir
    done

    run_podman $globalargs rmi $image1 $image2 $image3
}


last_dir_digest=""

function dir_digest() {
    # inspired on https://www.gnu.org/software/tar/manual/html_node/Reproducibility.html
    TARFLAGS="--numeric-owner --sort=name --format=posix --pax-option=exthdr.name=%d/PaxHeaders/%f --pax-option=delete=atime,delete=ctime --clamp-mtime --mtime='1970-01-01T01:01:01Z'"

    tardest=$PODMAN_TMPDIR/tmp.tar

    run -0 tar -C $1 -cf $tardest $TARFLAGS .
    run -0 sha256sum < $tardest
    last_dir_digest=$(echo $output | tr -d ' -')
    rm -f $tardest
}

function mount_image_and_take_digest() {
    run_podman $globalargs image mount $1
    dir_digest $output
    run_podman $globalargs image umount $1
}

@test "zstd chunked does not modify image content" {
    skip_if_remote "need to mount an image" # remote tests are already skipped in setup, this is one more reason.
    skip_if_rootless "need to mount the image without unshare"

    image=localhost:${PODMAN_LOGIN_REGISTRY_PORT}/img-$(safename)

    globalargs="--storage-driver $(podman_storage_driver) --pull-option enable_partial_images=true"
    pushpullargs="--cert-dir ${PODMAN_LOGIN_WORKDIR}/trusted-registry-cert-dir \
                  --creds ${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS}"

    dockerfile=$PODMAN_TMPDIR/Dockerfile
    cat >$dockerfile <<EOF
FROM $IMAGE
RUN mkdir /new-files; seq 100 | while read i; do echo \$i > /new-files/\$i; done
EOF
    run_podman $globalargs build --squash-all -t $image -f $dockerfile .

    mount_image_and_take_digest $image
    digest1=$last_dir_digest

    run_podman $globalargs push \
               $pushpullargs \
               --compression-format zstd:chunked \
               $image

    run_podman $globalargs rmi $image

    run_podman --log-level debug $globalargs pull \
               $pushpullargs \
               $image
    if [ "$(podman_storage_driver)" != vfs ]; then # VFS does not implement partial pulls
        assert "$output" =~ "Retrieved partial blob" # A spot check that we are really using the partial-pull code path
    fi

    # expect that the image contains exactly the same data as before
    mount_image_and_take_digest $image
    digest2=$last_dir_digest

    assert "$digest2" = "$digest1" "image content changed after push/pull"

    run_podman $globalargs rmi $image
}

# END   cooperation with skopeo
# END   actual tests
###############################################################################

# vim: filetype=sh
