#!/usr/bin/env bats   -*- bats -*-
#
# Creates errors that should be caught by `system check`, and verifies
# that they are caught and remedied, even if it requires discarding some
# data in read-write layers.
#

load helpers

@test "podman system check - unmanaged layers" {
    run_podman_testing create-storage-layer
    layerID="$output"
    run_podman_testing create-storage-layer --parent=$layerID
    run_podman 125 system check
    assert "$output" =~ "layer in lower level storage driver not accounted for" "output from 'podman system check' with unmanaged layers"
    run_podman system check -r
    run_podman system check
}

@test "podman system check - unused layers" {
    run_podman_testing create-layer
    layerID="$output"
    run_podman_testing create-layer --parent=$layerID
    run_podman system check
    run_podman 125 system check -m 0
    assert "$output" =~ "layer not referenced" "output from 'podman system check' with unused layers"
    run_podman system check -m 0 -r
    run_podman system check -m 0
}

@test "podman system check - layer content digest changed" {
    run_podman_testing create-layer
    layerID="$output"
    make_layer_blob 8 ${PODMAN_TMPDIR}/archive.tar
    run_podman_testing populate-layer --layer=$layerID --file=${PODMAN_TMPDIR}/archive.tar
    run_podman_testing create-image --layer=$layerID
    imageID="$output"
    testing_make_image_metadata_for_layer_blobs $imageID ${PODMAN_TMPDIR}/archive.tar
    run_podman create $imageID
    make_layer_blob 1 ${PODMAN_TMPDIR}/archive.tar
    run_podman_testing modify-layer --layer=$layerID --file=${PODMAN_TMPDIR}/archive.tar
    run_podman 125 system check
    assert "$output" =~ "checksum failed" "output from 'podman system check' with modified layer contents"
    run_podman 125 system check -r
    run_podman 0+w system check -r -f
    run_podman system check
}

@test "podman system check - layer content added" {
    run_podman_testing create-layer
    layerID="$output"
    make_layer_blob 8 ${PODMAN_TMPDIR}/archive.tar
    run_podman_testing populate-layer --layer=$layerID --file=${PODMAN_TMPDIR}/archive.tar
    run_podman_testing create-image --layer=$layerID
    imageID="$output"
    testing_make_image_metadata_for_layer_blobs $imageID ${PODMAN_TMPDIR}/archive.tar
    run_podman create $imageID
    make_layer_blob 100 101 ${PODMAN_TMPDIR}/archive.tar
    run_podman_testing modify-layer --layer=$layerID --file=${PODMAN_TMPDIR}/archive.tar
    run_podman 125 system check
    assert "$output" =~ "content modified" "output from 'podman system check' with unexpected content added to layer"
    run_podman 125 system check -r
    run_podman 0+w system check -r -f
    run_podman system check
}

@test "podman system check - storage image layer missing" {
    run_podman_testing create-layer
    layerID="$output"
    make_layer_blob 8 ${PODMAN_TMPDIR}/archive.tar
    run_podman_testing populate-layer --layer=$layerID --file=${PODMAN_TMPDIR}/archive.tar
    run_podman_testing create-image --layer=$layerID
    imageID="$output"
    testing_make_image_metadata_for_layer_blobs $imageID ${PODMAN_TMPDIR}/archive.tar
    run_podman create $imageID
    run_podman_testing remove-layer --layer=$layerID
    run_podman 125 system check
    assert "$output" =~ "image layer is missing" "output from 'podman system check' with missing layer"
    run_podman 125 system check -r
    run_podman 0+w system check -r -f
    run_podman system check
}

@test "podman system check - storage container image missing" {
    run_podman_testing create-layer
    layerID="$output"
    make_layer_blob 8 ${PODMAN_TMPDIR}/archive.tar
    run_podman_testing populate-layer --layer=$layerID --file=${PODMAN_TMPDIR}/archive.tar
    run_podman_testing create-image --layer=$layerID
    imageID="$output"
    testing_make_image_metadata_for_layer_blobs $imageID ${PODMAN_TMPDIR}/archive.tar
    run_podman create $imageID
    run_podman_testing remove-image --image=$imageID
    run_podman 125 system check -m 0
    assert "$output" =~ "image missing" "output from 'podman system check' with missing image"
    run_podman 125 system check -r -m 0
    run_podman 0+w system check -r -f -m 0
    run_podman system check -m 0
}

@test "podman system check - storage layer data missing" {
    run_podman_testing create-layer
    layerID="$output"
    make_layer_blob ${PODMAN_TMPDIR}/archive.tar
    run_podman_testing populate-layer --layer=$layerID --file=${PODMAN_TMPDIR}/archive.tar
    make_random_file ${PODMAN_TMPDIR}/random-data.bin
    run_podman_testing create-layer-data --key=foo --file=${PODMAN_TMPDIR}/random-data.bin --layer=$layerID
    run_podman_testing create-image --layer=$layerID
    imageID="$output"
    testing_make_image_metadata_for_layer_blobs $imageID ${PODMAN_TMPDIR}/archive.tar
    run_podman create $imageID
    run_podman_testing remove-layer-data --key=foo --layer=$layerID
    run_podman 125 system check
    assert "$output" =~ "layer data item is missing" "output from 'podman system check' with missing layer data"
    run_podman 125 system check -r
    run_podman 0+w system check -r -f
    run_podman system check
}

@test "podman system check - storage image data missing" {
    run_podman_testing create-layer
    layerID="$output"
    make_layer_blob 8 ${PODMAN_TMPDIR}/archive.tar
    run_podman_testing populate-layer --layer=$layerID --file=${PODMAN_TMPDIR}/archive.tar
    run_podman_testing create-image --layer=$layerID
    imageID="$output"
    testing_make_image_metadata_for_layer_blobs $imageID ${PODMAN_TMPDIR}/archive.tar
    make_random_file ${PODMAN_TMPDIR}/random-data.bin
    run_podman_testing create-image-data --key=foo --file=${PODMAN_TMPDIR}/random-data.bin --image=$imageID
    run_podman create $imageID
    run_podman_testing remove-image-data --key=foo --image=$imageID
    run_podman 125 system check
    assert "$output" =~ "image data item is missing" "output from 'podman system check' with missing image data"
    run_podman 125 system check -r
    run_podman 0+w system check -r -f
    run_podman system check
}

@test "podman system check - storage image data modified" {
    run_podman_testing create-layer
    layerID="$output"
    make_layer_blob 8 ${PODMAN_TMPDIR}/archive.tar
    run_podman_testing populate-layer --layer=$layerID --file=${PODMAN_TMPDIR}/archive.tar
    run_podman_testing create-image --layer=$layerID
    imageID="$output"
    testing_make_image_metadata_for_layer_blobs $imageID ${PODMAN_TMPDIR}/archive.tar
    run_podman create $imageID
    make_random_file ${PODMAN_TMPDIR}/random-data.bin
    run_podman_testing create-image-data --key=foo --file=${PODMAN_TMPDIR}/random-data.bin --image=$imageID
    make_random_file ${PODMAN_TMPDIR}/random-data.bin
    run_podman_testing modify-image-data --key=foo --file=${PODMAN_TMPDIR}/random-data.bin --image=$imageID
    run_podman 125 system check
    assert "$output" =~ "image data item has incorrect" "output from 'podman system check' with modified image data"
    run_podman 125 system check -r
    run_podman 0+w system check -r -f
    run_podman system check
}

@test "podman system check - container data missing" {
    run_podman_testing create-layer
    layerID="$output"
    make_layer_blob 8 ${PODMAN_TMPDIR}/archive.tar
    run_podman_testing populate-layer --layer=$layerID --file=${PODMAN_TMPDIR}/archive.tar
    run_podman_testing create-image --layer=$layerID
    imageID="$output"
    testing_make_image_metadata_for_layer_blobs $imageID ${PODMAN_TMPDIR}/archive.tar
    run_podman create $imageID
    containerID="$output"
    make_random_file ${PODMAN_TMPDIR}/random-data.bin
    run_podman_testing create-container-data --key=foo --file=${PODMAN_TMPDIR}/random-data.bin --container=$containerID
    run_podman_testing remove-container-data --key=foo --container=$containerID
    run_podman 125 system check
    assert "$output" =~ "container data item is missing" "output from 'podman system check' with missing container data"
    run_podman 125 system check -r
    run_podman 0+w system check -r -f
    run_podman system check
    run_podman rmi $imageID
}

@test "podman system check - container data modified" {
    run_podman_testing create-layer
    layerID="$output"
    make_layer_blob 8 ${PODMAN_TMPDIR}/archive.tar
    run_podman_testing populate-layer --layer=$layerID --file=${PODMAN_TMPDIR}/archive.tar
    run_podman_testing create-image --layer=$layerID
    imageID="$output"
    testing_make_image_metadata_for_layer_blobs $imageID ${PODMAN_TMPDIR}/archive.tar
    run_podman create $imageID
    containerID="$output"
    make_random_file ${PODMAN_TMPDIR}/random-data.bin
    run_podman_testing create-container-data --key=foo --file=${PODMAN_TMPDIR}/random-data.bin --container=$containerID
    make_random_file ${PODMAN_TMPDIR}/random-data.bin
    run_podman_testing modify-container-data --key=foo --file=${PODMAN_TMPDIR}/random-data.bin --container=$containerID
    run_podman 125 system check
    assert "$output" =~ "container data item has incorrect" "output from 'podman system check' with modified container data"
    run_podman 125 system check -r
    run_podman 0+w system check -r -f
    run_podman system check
    run_podman rmi $imageID
}

function make_layer_blob() {
    local tmpdir=$(mktemp -d --tmpdir=${PODMAN_TMPDIR} make_layer_blob.XXXXXX)
    local blobfile
    local seqargs
    for arg in "${@}" ; do
        seqargs="${blobfile:+$seqargs $blobfile}"
        blobfile="$arg"
    done
    seqargs="${seqargs:-8}"
    local filelist=
    for file in $(seq ${seqargs}); do
        dd if=/dev/urandom of="$tmpdir/file$file" bs=1 count=$((1024 + $file)) status=none
        filelist="$filelist file$file"
    done
    tar -c --owner=root:0 --group=root:0 -f "$blobfile" -C "$tmpdir" $filelist
}

function testing_make_image_metadata_for_layer_blobs() {
    local tmpdir=$(mktemp -d --tmpdir=${PODMAN_TMPDIR} make_image_metadata.XXXXXX)
    local imageID=$1
    shift
    echo '{"config":{},"rootfs":{"type":"layers","diff_ids":[' > $tmpdir/config.json
    echo '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","layers":[' > $tmpdir/manifest
    local comma=
    for blob in "$@" ; do
        local sum=$(sha256sum $blob)
        sum=${sum%% *}
        local size=$(wc -c $blob)
        size=${size%% *}
        echo $comma '"sha256:'$sum'"' >> $tmpdir/config.json
        echo $comma '{"digest":"sha256:'$sum'","size":'$size',"mediaType":"application/vnd.oci.image.layer.v1.tar"}' >> $tmpdir/manifest
        comma=,
    done
    echo ']}}' >> $tmpdir/config.json
    sum=$(sha256sum $tmpdir/config.json)
    sum=${sum%% *}
    size=$(wc -c $tmpdir/config.json)
    size=${size%% *}
    echo '],"config":{"digest":"sha256:'$sum'","size":'$size',"mediaType":"application/vnd.oci.image.config.v1+json"}}' >> $tmpdir/manifest
    run_podman_testing create-image-data -i $imageID -k sha256:$sum -f $tmpdir/config.json
    sum=$(sha256sum $tmpdir/manifest)
    sum=${sum%% *}
    run_podman_testing create-image-data -i $imageID -k manifest-sha256:$sum -f $tmpdir/manifest
    run_podman_testing create-image-data -i $imageID -k manifest -f $tmpdir/manifest
}

# vim: filetype=sh
