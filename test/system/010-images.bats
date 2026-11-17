#!/usr/bin/env bats

load helpers

# CANNOT BE PARALLELIZED: relies on exact output from podman images
@test "podman images - basic output" {
    headings="REPOSITORY *TAG *IMAGE ID *CREATED *SIZE"

    run_podman images -a
    is "${lines[0]}" "$headings" "header line"
    is "${lines[1]}" "$PODMAN_TEST_IMAGE_REGISTRY/$PODMAN_TEST_IMAGE_USER/$PODMAN_TEST_IMAGE_NAME *$PODMAN_TEST_IMAGE_TAG *[0-9a-f]\+" "podman images output"

    # 'podman images' should emit headings even if there are no images
    # (but --root only works locally)
    if ! is_remote; then
        run_podman --storage-driver=vfs --root ${PODMAN_TMPDIR}/nothing-here-move-along images
        is "$output" "$headings" "'podman images' emits headings even w/o images"
    fi
}

# CANNOT BE PARALLELIZED: relies on exact output from podman images
@test "podman images - custom formats" {
    tests="
{{.ID}}                  |        [0-9a-f]\\\{12\\\}\\\$
{{.ID| upper}}           |        [0-9A-F]\\\{12\\\}\\\$
{{.Repository}}:{{.Tag}} | $PODMAN_TEST_IMAGE_FQN
{{.Labels.created_by}}   | test/system/build-testimage
{{.Labels.created_at}}   | 20[0-9-]\\\+T[0-9:]\\\+Z
"

    defer-assertion-failures

    while read fmt expect; do
        run_podman images --format "$fmt"
        is "$output" "$expect" "podman images --format '$fmt'"
    done < <(parse_table "$tests")

    run_podman images --format "{{.ID}}" --no-trunc
    is "$output" "sha256:[0-9a-f]\\{64\\}\$" "podman images --no-trunc"
}

# CANNOT BE PARALLELIZED: relies on exact output from podman images
@test "podman images - json" {
    # 'created': podman includes fractional seconds, podman-remote does not
    tests="
Names[0]          | $PODMAN_TEST_IMAGE_FQN
Id                |        [0-9a-f]\\\{64\\\}
Digest            | sha256:[0-9a-f]\\\{64\\\}
CreatedAt         | [0-9-]\\\+T[0-9:.]\\\+Z
Size              | [0-9]\\\+
Labels.created_by | test/system/build-testimage
Labels.created_at | 20[0-9-]\\\+T[0-9:]\\\+Z
"

    run_podman images -a --format json

    while read field expect; do
        actual=$(echo "$output" | jq -r ".[0].$field")
        dprint "# actual=<$actual> expect=<$expect}>"
        is "$actual" "$expect" "jq .$field"
    done < <(parse_table "$tests")
}

# CANNOT BE PARALLELIZED: relies on exact output from podman images
@test "podman images - history output" {
    # podman history is persistent: it permanently alters our base image.
    # Create a dummy image here so we leave our setup as we found it.
    # Multiple --name options confirm command-line override (last one wins)
    cname=c_$(safename)
    iname=i_$(safename)
    run_podman run --name ignore-me --name $cname $IMAGE true
    run_podman commit $cname $iname

    run_podman images $iname --format '{{ .History }}'
    is "$output" "localhost/$iname:latest" "image history with initial name"

    # Generate two randomish tags
    rand_name1="test-image-history-1-$(safename)"
    rand_name2="test-image-history-2-$(safename)"

    # Tag once, rmi, and make sure the tag name appears in history
    run_podman tag $iname $rand_name1
    run_podman rmi $rand_name1
    run_podman images $iname --format '{{ .History }}'
    is "$output" "localhost/$iname:latest, localhost/${rand_name1}:latest" "image history after one tag"

    # Repeat with second tag. Now both tags should be in history
    run_podman tag $iname $rand_name2
    run_podman rmi $rand_name2
    run_podman images $iname --format '{{ .History }}'
    is "$output" "localhost/$iname:latest, localhost/${rand_name2}:latest, localhost/${rand_name1}:latest" \
       "image history after two tags"

    run_podman rmi $iname
    run_podman rm $cname
}

# CANNOT BE PARALLELIZED: relies on exact output from podman images
@test "podman images - filter" {
    # Multiple --format options confirm command-line override (last one wins)
    run_podman inspect --format '{{.XYZ}}' --format '{{.ID}}' $IMAGE
    iid=$output

    run_podman images --noheading --filter=after=$iid
    is "$output" "" "baseline: empty results from filter (after)"

    run_podman images --noheading --filter=before=$iid
    is "$output" "" "baseline: empty results from filter (before)"

    # Create a dummy container, then commit that as an image. We will
    # now be able to use before/after/since queries
    cname=c_$(safename)
    iname=i_$(safename)
    run_podman run --name $cname $IMAGE true
    run_podman commit -q  $cname $iname
    new_iid=$output

    # (refactor common options for legibility)
    opts='--noheading --no-trunc --format={{.ID}}--{{.Repository}}:{{.Tag}}'

    run_podman images ${opts} --filter=after=$iid
    is "$output" "sha256:$new_iid--localhost/$iname:latest" "filter: after"

    # Same thing, with 'since' instead of 'after'
    run_podman images ${opts} --filter=since=$iid
    is "$output" "sha256:$new_iid--localhost/$iname:latest" "filter: since"

    run_podman images ${opts} --filter=before=$iname
    is "$output" "sha256:$iid--$IMAGE" "filter: before"

    run_podman 125 image list -f json
    is "$output" 'Error: invalid image filter "json": must be in the format "filter=value or filter!=value"' "Invalid filter"

    # Clean up
    run_podman rmi $iname
    run_podman rm  $cname
}

# Regression test for https://github.com/containers/podman/issues/7651
# in which "podman pull image-with-sha" causes "images -a" to crash
# CANNOT BE PARALLELIZED: relies on exact output from podman images
@test "podman images -a, after pulling by sha " {
    # This test requires that $IMAGE be 100% the same as the registry one
    run_podman rmi -a -f
    _prefetch $IMAGE

    # Get a baseline for 'images -a'
    run_podman images -a
    local images_baseline="$output"

    # Get the digest of our local test image. We need to do this in two steps
    # because 'podman inspect' only works reliably on *IMAGE ID*, not name.
    # See https://github.com/containers/podman/issues/3761
    run_podman inspect --format '{{.Id}}' $IMAGE
    local iid="$output"
    run_podman inspect --format '{{.Digest}}' $iid
    local sha="$output"

    local imgbase="${PODMAN_TEST_IMAGE_REGISTRY}/${PODMAN_TEST_IMAGE_USER}/${PODMAN_TEST_IMAGE_NAME}"
    local fqin="${imgbase}@$sha"

    # This will always pull, because even though it's the same image we
    # already have, podman doesn't actually know that.
    run_podman pull $fqin
    is "$output" "Trying to pull ${fqin}\.\.\..*" "output of podman pull"

    # Prior to #7654, this would crash and burn. Now podman recognizes it
    # as the same image and, even though it internally tags it with the
    # sha, still only shows us one image (which should be our baseline)
    #
    # WARNING! If this test fails, we're going to see a lot of failures
    # in subsequent tests due to 'podman ps' showing the '@sha' tag!
    # I choose not to add a complicated teardown() (with 'rmi @sha')
    # because the failure window here is small, and if it fails it
    # needs attention anyway. So if you see lots of failures, but
    # start here because this is the first one, fix this problem.
    # You can (probably) ignore any subsequent failures showing '@sha'
    # in the error output.
    #
    # WARNING! This test is likely to fail for an hour or so after
    # building a new testimage (via build-testimage script), because
    # two consecutive 'podman images' may result in a one-minute
    # difference in the "XX minutes ago" output. This is OK to ignore.
    run_podman images -a
    is "$output" "$images_baseline" "images -a, after pull: same as before"

    # Clean up: this should simply untag, not remove
    run_podman rmi $fqin
    is "$output" "Untagged: $fqin" "podman rmi untags, does not remove"

    # ...and now we should still have our same image.
    run_podman images -a
    is "$output" "$images_baseline" "after podman rmi @sha, still the same"
}

# Tests #7199 (Restore "table" --format from V1)
#
# Tag our image with different-length strings; confirm table alignment
# CANNOT BE PARALLELIZED: relies on exact output from podman images
@test "podman images - table format" {
    # Craft two tags such that they will bracket $IMAGE on either side (above
    # and below). This assumes that $IMAGE is quay.io or foo.com or simply
    # not something insane that will sort before 'aaa' or after 'zzz'.
    local aaa_name=a.b/c
    local aaa_tag=d
    local zzz_name=zzzzzzzzzz.yyyyyyyyy/xxxxxxxxx
    local zzz_tag=$(random_string 15)

    # Helper function to check one line of tabular output; all this does is
    # generate a line with the given repo/tag, formatted to the width of the
    # widest image, which is the zzz one. Fields are separated by TWO spaces.
    function _check_line() {
        local lineno=$1
        local name=$2
        local tag=$3

        is "${lines[$lineno]}" \
           "$(printf '%-*s  %-*s  %s' ${#zzz_name} ${name} ${#zzz_tag} ${tag} $iid)" \
           "podman images, $testname, line $lineno"
    }

    function _run_format_test() {
        local testname=$1
        local format=$2

        run_podman images --sort repository --format "$format"

        line_no=0
        if [[ $format == table* ]]; then
            # skip headers from table command
            line_no=1
        fi

        _check_line $line_no ${aaa_name} ${aaa_tag}
        _check_line $((line_no+1)) "${PODMAN_TEST_IMAGE_REGISTRY}/${PODMAN_TEST_IMAGE_USER}/${PODMAN_TEST_IMAGE_NAME}" "${PODMAN_TEST_IMAGE_TAG}"
        _check_line $((line_no+2)) ${zzz_name} ${zzz_tag}
    }

    # Begin the test: tag $IMAGE with both the given names
    run_podman tag $IMAGE ${aaa_name}:${aaa_tag}
    run_podman tag $IMAGE ${zzz_name}:${zzz_tag}

    # Get the image ID, used to verify output below (all images share same IID)
    run_podman inspect --format '{{.ID}}' $IMAGE
    iid=${output:0:12}

    # Run the test: this will output three column-aligned rows. Test them.
    _run_format_test 'table' 'table {{.Repository}} {{.Tag}} {{.ID}}'

    # Clean up.
    run_podman rmi ${aaa_name}:${aaa_tag} ${zzz_name}:${zzz_tag}
}

# CANNOT BE PARALLELIZED: relies on exact output from podman images
@test "podman images - rmi -af removes all containers and pods" {
    pname=p_$(safename)
    run_podman create --pod new:$pname $IMAGE

    run_podman inspect --format '{{.ID}}' $IMAGE
    imageID=$output

    run_podman pod inspect --format "{{.InfraContainerID}}" $pname
    infra_ID="$output"

    run_podman 2 rmi -a
    is "$output" "Error: image used by .*: image is in use by a container: consider listing external containers and force-removing image"

    run_podman rmi -af
    is "$output" "Untagged: $IMAGE
Deleted: $imageID" "image gets removed"

    run_podman images --noheading
    is "$output" ""
    run_podman ps --all --noheading --no-trunc
    assert "$output" =~ ".*$infra_ID.*" "infra container still running"
    run_podman pod ps --noheading
    assert "$output" =~ ".*$pname.*" "pod still running"

    # Clean up
    run_podman pod rm -a
}

# CANNOT BE PARALLELIZED: relies on exact output from podman images
@test "podman images - rmi -f can remove infra images" {
    pname=p_$(safename)

    # Create a custom image so we can test --infra-image and -command.
    # It will have a randomly generated infra command, using the
    # existing 'pause' script in our testimage. We assign a bogus
    # entrypoint to confirm that --infra-command will override.
    local pauseImage="infra_image_$(safename)"
    # --layers=false needed to work around buildah#5674 parallel flake
    run_podman build -t $pauseImage --layers=false - << EOF
FROM $IMAGE
ENTRYPOINT ["/home/podman/pause"]
EOF

    run_podman --noout pod create --name $pname --infra-image "$pauseImage"
    run_podman create --pod $pname $IMAGE

    run_podman inspect --format '{{.ID}}' $pauseImage
    pauseID=$output

    run_podman 2 rmi $pauseImage
    is "$output" "Error: image used by .* image is in use by a container: consider listing external containers and force-removing image"

    run_podman rmi -f $pauseImage
    is "$output" "Untagged: localhost/$pauseImage:latest
Deleted: $pauseID"

    # Force-removing the infra container removes the pod and all its containers.
    run_podman ps --all --noheading
    is "$output" ""
    run_podman pod ps --noheading
    is "$output" ""

    # Other images are still present.
    run_podman image exists $IMAGE
}

# bats test_tags=ci:parallel
@test "podman rmi --ignore" {
    random_image_name=i_$(safename)
    run_podman 1 rmi $random_image_name
    is "$output" "Error: $random_image_name: image not known.*"
    run_podman rmi --ignore $random_image_name
    is "$output" ""
}

# bats test_tags=ci:parallel
@test "podman image rm --force bogus" {
    run_podman 1 image rm bogus
    is "$output" "Error: bogus: image not known" "Should print error"
    run_podman image rm --force bogus
    is "$output" "" "Should print no output"

    _prefetch $IMAGE
    random_image_name=i_$(safename)
    run_podman image tag $IMAGE $random_image_name
    run_podman image rm --force bogus $random_image_name
    assert "$output" = "Untagged: localhost/$random_image_name:latest" "removed image"

    run_podman images
    assert "$output" !~ "$random_image_name" "image must be removed"
}

# bats test_tags=ci:parallel
@test "podman images - commit docker with comment" {
    cname=c_$(safename)
    iname=i_$(safename)
    run_podman run --name $cname -d $IMAGE top
    run_podman 125 commit -m comment $cname $iname
    assert "$output" == "Error: messages are only compatible with the docker image format (-f docker)" "podman should fail unless docker format"

    # Without -q: verbose output, but only on podman-local, not remote
    run_podman commit $cname --format docker -m comment ${iname}_2
    if ! is_remote; then
        assert "$output" =~ "Getting image.*Writing manif" \
               "Without -q, verbose output"
    fi

    # With -q, both local and remote: only an image ID
    run_podman commit -q $cname --format docker -m comment ${iname}_3
    assert "$output" =~ "^[0-9a-f]{64}\$" \
           "With -q, output is a commit ID, no warnings or other output"

    run_podman rmi ${iname}_2 ${iname}_3
    run_podman rm $cname --force -t 0
}

# CANNOT BE PARALLELIZED: relies on exact set of images in store
@test "podman pull image with additional store" {
    skip_if_remote "only works on local"

    # overlay or vfs
    local storagedriver="$(podman_storage_driver)"

    local imstore=$PODMAN_TMPDIR/imagestore
    local sconf=$PODMAN_TMPDIR/storage.conf
    cat >$sconf <<EOF
[storage]
driver="$storagedriver"

[storage.options]
additionalimagestores = [ "$imstore/root" ]
EOF

    # If composefs (root only) is enabled, we must mirror that setting in our conf
    if grep -q 'BEGIN CI-enabled composefs' /etc/containers/storage.conf; then
        sed -ne '/BEGIN CI-enabled composefs/,/END CI-enabled composefs/p' /etc/containers/storage.conf \
            | grep -vF '[storage.options]' >>$sconf
    fi

    _prefetch $IMAGE
    skopeo copy containers-storage:$IMAGE \
           containers-storage:\[${storagedriver}@${imstore}/root+${imstore}/runroot\]$IMAGE

    # IMPORTANT! Use -2/-1 indices, not 0/1, because $SYSTEMD_IMAGE may be
    # present in store, and if it is it will precede $IMAGE.
    CONTAINERS_STORAGE_CONF=$sconf run_podman images -a -n --format "{{.Id}} {{.Repository}}:{{.Tag}} {{.ReadOnly}}"
    assert "${#lines[*]}" -ge 2 "at least 2 lines from 'podman images'"
    assert "${lines[-2]}" =~ ".*$IMAGE false" "image from readwrite store"
    assert "${lines[-1]}" =~ ".*$IMAGE true" "image from readonly store"
    id=${lines[-2]%% *}

    CONTAINERS_STORAGE_CONF=$sconf run_podman pull -q $IMAGE
    is "$output" "$id" "pull -q $IMAGE, using storage.conf"

    # $IMAGE might now be reusing layers from the additional store;
    # Removing the additional store underneath can result in dangling layer references.
    # Try to fix that up.
    CONTAINERS_STORAGE_CONF=$sconf run_podman rmi $IMAGE
    _prefetch $IMAGE

    run_podman --root $imstore/root rmi --all
}

# bats test_tags=ci:parallel
@test "podman images with concurrent removal" {
    skip_if_remote "following test is not supported for remote clients"
    local count=5

    # First build $count images
    for i in $(seq --format '%02g' 1 $count); do
        cat >$PODMAN_TMPDIR/Containerfile <<EOF
FROM $IMAGE
RUN echo $i
EOF
        run_podman build -q -t img-$i-$(safename) $PODMAN_TMPDIR
    done

    run_podman images
    # Now remove all images in parallel and in the background and make sure
    # that listing all images does not fail (see BZ 2216700).
    for i in $(seq --format '%02g' 1 $count); do
        timeout --foreground -v --kill=10 60 \
                "${PODMAN_CMD[@]}" rmi img-$i-$(safename) &
    done

    tries=100
    while [[ "$output" =~ "$(safename)" ]] && [[ $tries -gt 0 ]]; do
        run_podman images --format "{{.ID}} {{.Names}}"
        tries=$((tries - 1))
    done

    if [[ $tries -eq 0 ]]; then
        die "Timed out waiting for images to be removed"
    fi

    wait
}


# vim: filetype=sh
