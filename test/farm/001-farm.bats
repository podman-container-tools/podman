#!/usr/bin/env bats
#
# Tests of podman farm commands
#

load helpers.bash

@test "farm - check farm has been created" {
    run_podman farm ls
    assert "$output" =~ $FARMNAME
    assert "$output" =~ "test-node"
}

@test "farm - build on local only" {
    iname="test-image-1"
    empty_farm="empty-farm"
    # create an empty farm
    run_podman farm create $empty_farm
    run_podman farm build --farm $empty_farm --authfile $AUTHFILE --tls-verify=false -t $REGISTRY/$iname $FARM_TMPDIR
    assert "$output" =~ "Local builder ready"

    # get the system architecture
    run_podman info --format '{{.Host.Arch}}'
    ARCH=$output
    # inspect manifest list built and saved in local containers-storage
    run_podman manifest inspect $iname
    assert "$output" =~ $ARCH

    echo "# skopeo inspect ..."
    run skopeo inspect "$@" --tls-verify=false --authfile $AUTHFILE docker://$REGISTRY/$iname
    echo "$output"
    is "$status" "0" "skopeo inspect - exit status"

    # FIXME-someday: why do we need the prune?
    run_podman manifest rm $iname
    run_podman image prune -f
}

@test "farm - build on farm node only with --cleanup" {
    iname="test-image-2"
    run_podman farm build --cleanup --local=false --authfile $AUTHFILE --tls-verify=false -t $REGISTRY/$iname $FARM_TMPDIR
    assert "$output" =~ "Farm \"$FARMNAME\" ready"
    # get the system architecture
    run_podman info --format '{{.Host.Arch}}'
    ARCH=$output
    # inspect manifest list built and saved in local containers-storage
    run_podman manifest inspect $iname
    assert "$output" =~ $ARCH

    echo "# skopeo inspect ..."
    run skopeo inspect "$@" --tls-verify=false --authfile $AUTHFILE docker://$REGISTRY/$iname
    echo "$output"
    is "$status" "0" "skopeo inspect - exit status"

    # see if we can ssh into node to check the image was cleaned up
    run ssh $ROOTLESS_USER@localhost podman images --filter dangling=true --noheading
    assert "$output" = "" "podman images on remote host"

    # check that no image was built locally
    run_podman images --filter dangling=true --noheading
    assert "$output" = "" "podman images on local host"

    run_podman manifest rm $iname
    run_podman image prune -f
}

@test "farm - build on farm node and local" {
    iname="test-image-3"
    run_podman farm build --authfile $AUTHFILE --tls-verify=false -t $REGISTRY/$iname $FARM_TMPDIR
    assert "$output" =~ "Farm \"$FARMNAME\" ready"

    # get the system architecture
    run_podman info --format '{{.Host.Arch}}'
    ARCH=$output
    # inspect manifest list built and saved
    run_podman manifest inspect $iname
    assert "$output" =~ $ARCH

    echo "# skopeo inspect ..."
    run skopeo inspect "$@" --tls-verify=false --authfile $AUTHFILE docker://$REGISTRY/$iname
    echo "$output"
    is "$status" "0" "skopeo inspect - exit status"

    run_podman manifest rm $iname
    run_podman image prune -f
}

@test "farm - build on farm node only with registries.conf" {
    cat >$PODMAN_TMPDIR/registries.conf <<EOF
[[registry]]
location="$REGISTRY"
insecure=true
EOF

    iname="test-image-4"
    CONTAINERS_REGISTRIES_CONF="$PODMAN_TMPDIR/registries.conf" run_podman farm build --authfile $AUTHFILE -t $REGISTRY/$iname $FARM_TMPDIR
    assert "$output" =~ "Farm \"$FARMNAME\" ready"

    # get the system architecture
    CONTAINERS_REGISTRIES_CONF="$PODMAN_TMPDIR/registries.conf" run_podman info --format '{{.Host.Arch}}'
    ARCH=$output
    # inspect manifest list built and saved
    CONTAINERS_REGISTRIES_CONF="$PODMAN_TMPDIR/registries.conf" run_podman manifest inspect $iname
    assert "$output" =~ $ARCH

    echo "# skopeo inspect ..."
    run skopeo inspect "$@" --tls-verify=false --authfile $AUTHFILE docker://$REGISTRY/$iname
    echo "$output"
    is "$status" "0" "skopeo inspect - exit status"

    run_podman manifest rm $iname
    run_podman image prune -f
}

# Test out podman-remote

@test "farm - build on farm node only (podman-remote)" {
    iname="test-image-5"
    # ManifestAdd only
    echo "Running test with ManifestAdd only..."
    run_podman --remote farm build --authfile $AUTHFILE --tls-verify=false -t $REGISTRY/$iname $FARM_TMPDIR
    assert "$output" =~ "Farm \"$FARMNAME\" ready"

    # ManifestListClear and ManifestAdd
    echo "Running test with ManifestListClear and ManifestAdd..."
    run_podman --remote farm build --authfile $AUTHFILE --tls-verify=false -t $REGISTRY/$iname $FARM_TMPDIR
    assert "$output" =~ "Farm \"$FARMNAME\" ready"

    # get the system architecture
    run_podman --remote info --format '{{.Host.Arch}}'
    ARCH=$output
    # inspect manifest list built and saved
    run_podman manifest inspect $iname
    assert "$output" =~ $ARCH

    echo "# skopeo inspect ..."
    run skopeo inspect "$@" --tls-verify=false --authfile $AUTHFILE docker://$REGISTRY/$iname
    echo "$output"
    is "$status" "0" "skopeo inspect - exit status"

    run_podman manifest rm $iname
    run_podman image prune -f
}
