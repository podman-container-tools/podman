load helpers
load helpers.network
load helpers.registry

@test "podman pull with policy flag" {
    skip_if_remote "tests depend on start_registry which does not work with podman-remote"
    start_registry

    local registry=localhost:${PODMAN_LOGIN_REGISTRY_PORT}
    local image_for_test=$registry/i-$(safename):$(random_string)
    local authfile=$PODMAN_TMPDIR/authfile.json

    run_podman login --authfile=$authfile \
        --tls-verify=false \
        --username ${PODMAN_LOGIN_USER} \
        --password ${PODMAN_LOGIN_PASS} \
        $registry

    # Generate a test image and push it to the registry.
    # For safety in parallel runs, test image must be isolated
    # from $IMAGE. A simple add-tag will not work. (#23756)
    run_podman create -q $IMAGE true
    local tmpcid=$output
    run_podman commit -q $tmpcid $image_for_test
    local image_id=$output
    run_podman rm $tmpcid
    run_podman image push --tls-verify=false --authfile=$authfile $image_for_test
    # Remove the local image to make sure it will be pulled again
    run_podman image rm --ignore $image_for_test

    # Test invalid policy
    run_podman 125 pull --tls-verify=false --authfile $authfile --policy invalid $image_for_test
    assert "$output" = "Error: unsupported pull policy \"invalid\""

    # Test policy=never with image not present
    run_podman 125 pull --tls-verify=false --authfile $authfile --policy never $image_for_test
    assert "$output" = "Error: $image_for_test: image not known"

    # Test policy=missing with image not present (should succeed)
    run_podman pull --tls-verify=false --authfile $authfile --policy missing $image_for_test
    assert "$output" =~ "Writing manifest to image destination"

    # Test policy=missing with image present (should not pull again)
    run_podman pull --tls-verify=false --authfile $authfile --policy missing $image_for_test
    assert "$output" = $image_id

    # Test policy=always (should always pull)
    run_podman pull --tls-verify=false --authfile $authfile --policy always $image_for_test
    assert "$output" =~ "Writing manifest to image destination"

    # Test policy=newer with image present and no new image(should not pull again)
    run_podman pull --tls-verify=false --authfile $authfile --policy newer $image_for_test
    assert "$output" = $image_id

    run_podman image rm --ignore $image_for_test
}

@test "podman pull with policy flag - remote" {
    # Make sure image is not pulled when policy is never
    run_podman 125 pull --policy never quay.io/libpod/i-do-not-exist
    assert "$output" = "Error: quay.io/libpod/i-do-not-exist: image not known"
}
