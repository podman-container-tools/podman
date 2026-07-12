#!/usr/bin/env bats

load helpers
load helpers.network
load helpers.registry

# All tests in this file must be able to run in parallel
# bats file_tags=ci:parallel

# Runs once before all tests in this file
function setup_file() {
    if ! is_remote; then
        start_registry
        authfile=${PODMAN_LOGIN_WORKDIR}/auth-manifest.json
        run_podman login --tls-verify=false \
                   --username ${PODMAN_LOGIN_USER} \
                   --password-stdin \
                   --authfile=$authfile \
                   localhost:${PODMAN_LOGIN_REGISTRY_PORT} <<<"${PODMAN_LOGIN_PASS}"
        is "$output" "Login Succeeded!" "output from podman login"
    fi
}

function teardown() {
    # Enumerate every one of the manifest names used everywhere below
    echo "[ teardown - ignore 'image not known' errors below ]"
    run_podman '?' manifest rm "m-$(safename):1.0" \
               localhost:${PODMAN_LOGIN_REGISTRY_PORT}/"m-$(safename):1.0"

    basic_teardown
}

# Helper function for several of the tests which verifies compression.
#
#  Usage:  validate_instance_compression INDEX MANIFEST ARCH COMPRESSION
#
#     INDEX             instance which needs to be verified in
#                       provided manifest list.
#
#     MANIFEST          OCI manifest specification in json format
#
#     ARCH              instance architecture
#
#     COMPRESSION       compression algorithm name; e.g "zstd".
#
function validate_instance_compression {
  case $4 in

   gzip)
    run jq -r '.manifests['$1'].annotations' <<< $2
    # annotation is `null` for gzip compression
    assert "$output" = "null" ".manifests[$1].annotations (null means gzip)"
    ;;

  zstd)
    # annotation `'"io.github.containers.compression.zstd": "true"'` must be there for zstd compression
    run jq -r '.manifests['$1'].annotations."io.github.containers.compression.zstd"' <<< $2
    assert "$output" = "true" ".manifests[$1].annotations.'io.github.containers.compression.zstd' (io.github.containers.compression.zstd must be set)"
    ;;
  esac

  run jq -r '.manifests['$1'].platform.architecture' <<< $2
  assert "$output" = $3 ".manifests[$1].platform.architecture"
}

# Regression test for #8931
@test "podman images - bare manifest list" {
    # Create an empty manifest list and list images.

    run_podman inspect --format '{{.ID}}' $IMAGE
    iid=$output

    mname="m-$(safename):1.0"
    run_podman manifest create $mname
    mid=$output
    run_podman manifest inspect --verbose $mid
    is "$output" ".*\"mediaType\": \"application/vnd.docker.distribution.manifest.list.v2+json\"" "--insecure is a noop want to make sure manifest inspect is successful"
    run_podman manifest inspect -v $mid
    is "$output" ".*\"mediaType\": \"application/vnd.docker.distribution.manifest.list.v2+json\"" "--insecure is a noop want to make sure manifest inspect is successful"
    run_podman images --format '{{.ID}}' --no-trunc
    is "$output" ".*sha256:$iid" "Original image ID still shown in podman-images output"
    run_podman rmi $mname
}

@test "podman manifest --tls-verify and --authfile" {
    skip_if_remote "running a local registry doesn't work with podman-remote"

    manifest1_notag="localhost:${PODMAN_LOGIN_REGISTRY_PORT}/m-$(safename)"
    manifest1="$manifest1_notag:1.0"
    run_podman manifest create $manifest1
    mid=$output

    authfile=${PODMAN_LOGIN_WORKDIR}/auth-manifest.json
    run_podman manifest push --authfile=$authfile \
        --tls-verify=false $mid \
        $manifest1
    run_podman manifest rm $manifest1
    run_podman 1 manifest rm $manifest1
    is "$output" "Error: $manifest1: image not known" "Missing manifest is reported"
    run_podman manifest rm --ignore $manifest1
    is "$output" "" "Missing manifest is ignored"

    # Default is to require TLS; also test explicit opts
    for opt in '' '--insecure=false' '--tls-verify=true' "--authfile=$authfile"; do
        run_podman 125 manifest inspect $opt $manifest1
        assert "$output" =~ "Error: reading image \"docker://$manifest1\": fetching manifest 1.0 in $manifest1_notag: pinging container registry localhost:${PODMAN_LOGIN_REGISTRY_PORT}:.*x509" \
               "TLE check: fails (as expected) with ${opt:-default}"
    done

    run_podman manifest inspect --authfile=$authfile --tls-verify=false $manifest1
    is "$output" ".*\"mediaType\": \"application/vnd.docker.distribution.manifest.list.v2+json\"" "Verify --tls-verify=false --authfile works against an insecure registry"
    run_podman manifest inspect --authfile=$authfile --insecure $manifest1
    is "$output" ".*\"mediaType\": \"application/vnd.docker.distribution.manifest.list.v2+json\"" "Verify --insecure --authfile works against an insecure registry"
    REGISTRY_AUTH_FILE=$authfile run_podman manifest inspect --tls-verify=false $manifest1
    is "$output" ".*\"mediaType\": \"application/vnd.docker.distribution.manifest.list.v2+json\"" "Verify --tls-verify=false with REGISTRY_AUTH_FILE works against an insecure registry"
}

@test "manifest list --add-compression with zstd:chunked" {
    skip_if_remote "running a local registry doesn't work with podman-remote"

    # Using TARGETARCH gives us distinct images for each arch
    dockerfile=$PODMAN_TMPDIR/Dockerfile
    cat >$dockerfile <<EOF
FROM scratch
ARG TARGETARCH
COPY Dockerfile /i-am-\${TARGETARCH}
EOF

    # Build two images, different arches, and add each to one manifest list
    local img="i-$(safename)"
    local manifestlocal="m-$(safename):1.0"
    run_podman manifest create $manifestlocal
    for arch in amd arm;do
        run_podman build --layers=false -t "$img-$arch" --platform linux/${arch}64 -f $dockerfile
        run_podman manifest add $manifestlocal containers-storage:localhost/"$img-$arch:latest"
    done

    # (for debugging)
    run_podman images -a

    # Push to local registry; the magic key here is --add-compression...
    local manifestpushed="localhost:${PODMAN_LOGIN_REGISTRY_PORT}/$manifestlocal"
    local authfile=${PODMAN_LOGIN_WORKDIR}/auth-manifest.json
    run_podman manifest push --authfile=$authfile --all --compression-format gzip --add-compression zstd:chunked --tls-verify=false $manifestlocal $manifestpushed

    # ...and use skopeo to confirm that each component has the right settings
    echo "$_LOG_PROMPT skopeo inspect ... $manifestpushed"
    smanifest=$(skopeo inspect --authfile=$authfile --tls-verify=false --raw docker://$manifestpushed)
    jq . <<<"$smanifest"

    validate_instance_compression "0" "$smanifest" "amd64" "gzip"
    validate_instance_compression "1" "$smanifest" "arm64" "gzip"
    validate_instance_compression "2" "$smanifest" "amd64" "zstd"
    validate_instance_compression "3" "$smanifest" "arm64" "zstd"

    run_podman manifest inspect --authfile=$authfile --tls-verify=false \
        $manifestpushed
    pmanifest="$output"

    objects=$(for obj in 0 1 2 3; do echo \
          ".manifests[$obj].annotations" \
          ".manifests[$obj].digest" \
          ".manifests[$obj].platform.architecture" \
          ".manifests[$obj].platform.os" \
          ".manifests[$obj].mediaType" \
          ".manifests[$obj].size" \
          ; done)
    for object in \
        $objects \
        '.schemaVersion' \
        '.mediaType' \
         ; do
        skopeoObj=$(jq -r "$object" <<<"$smanifest")
        podmanObj=$(jq -r "$object" <<<"$pmanifest")
        assert "$skopeoObj" != "$smanifest" "\"$object\" does not exist in skopeo result"
        assert "$podmanObj" == "$skopeoObj" "podman \"$object\" does not match skopeo"
    done

    run_podman rmi "$img-amd" "$img-arm"
    run_podman manifest rm $manifestlocal
}

function manifestListAddArtifactOnce() {
    echo listFlags="$listFlags"
    echo platformFlags="$platformFlags"
    echo typeFlag="$typeFlag"
    echo layerTypeFlag="$layerTypeFlag"
    echo configTypeFlag="$configTypeFlag"
    echo configFlag="$configFlag"
    echo titleFlag="$titleFlag"
    local index artifact firstdigest seconddigest config configSize defaulttype filetitle requested expected actual

    local authfile=${PODMAN_LOGIN_WORKDIR}/auth-manifest.json

    run_podman manifest create $listFlags $list
    run_podman manifest add $list ${platformFlags} --artifact ${typeFlag} ${layerTypeFlag} ${configTypeFlag} ${configFlag} ${titleFlag} ${PODMAN_TMPDIR}/listed.txt
    run_podman manifest add $list ${platformFlags} --artifact ${typeFlag} ${layerTypeFlag} ${configTypeFlag} ${configFlag} ${titleFlag} ${PODMAN_TMPDIR}/zeroes
    run_podman manifest inspect $list
    run_podman tag $list localhost:${PODMAN_LOGIN_REGISTRY_PORT}/$list
    run_podman manifest push --authfile=$authfile --tls-verify=false \
               localhost:${PODMAN_LOGIN_REGISTRY_PORT}/$list
    echo "skopeo inspect ..."
    run skopeo inspect --authfile=$authfile --tls-verify=false --raw \
        docker://localhost:${PODMAN_LOGIN_REGISTRY_PORT}/$list
    echo "$output"
    assert $status -eq 0 "skopeo inspect (status)"
    echo "$output"
    index="$output"

    if [[ -n "$listFlags" ]] ; then
        assert $(jq -r '.annotations["global"]' <<<"$index") == local \
               "listFlags=$listFlags, .annotations[global]"
    fi
    if [[ -n "$platformFlags" ]] ; then
        assert $(jq -r '.manifests[1].platform.os' <<<"$index") == linux \
               "platformFlags=$platformFlags, .platform.os"
        assert $(jq -r '.manifests[1].platform.architecture' <<<"$index") == amd64 \
               "platformFlags=$platformFlags, .platform.architecture"
    fi
    if [[ -n "$typeFlag" ]] ; then
        actual=$(jq -r '.manifests[0].artifactType' <<<"$index")
        assert "${actual#null}" == "${typeFlag#--artifact-type=}"
        actual=$(jq -r '.manifests[1].artifactType' <<<"$index")
        assert "${actual#null}" == "${typeFlag#--artifact-type=}"
    fi
    firstdigest=$(jq -r '.manifests[0].digest' <<<"$index")
    seconddigest=$(jq -r '.manifests[1].digest' <<<"$index")
    for digest in $firstdigest $seconddigest ; do
        case $digest in
        $firstdigest)
            filetitle=listed.txt
            defaulttype=text/plain
            ;;
        $seconddigest)
            filetitle=zeroes
            defaulttype=application/octet-stream
            ;;
        *)
            false
            ;;
        esac

        echo "skopeo inspect ... by digest"
        run skopeo inspect --raw --authfile=$authfile --tls-verify=false \
            docker://localhost:${PODMAN_LOGIN_REGISTRY_PORT}/${list%:*}@${digest}
        echo "$output"
        assert $status -eq 0 "skopeo inspect (status)"

        artifact="$output"
        if [[ -n "$typeFlag" ]] ; then
            actual=$(jq -r '.artifactType' <<<"$artifact")
            assert "${actual#null}" == "${typeFlag#--artifact-type=}" \
                   "typeFlag=$typeFlag, .artifactType"
        else
            actual=$(jq -r '.artifactType' <<<"$artifact")
            assert "${actual}" == application/vnd.unknown.artifact.v1 \
                   "typeFlag=NULL, .artifactType"
        fi
        if [ -n "$layerTypeFlag" ] ; then
            actual=$(jq -r '.layers[0].mediaType' <<<"$artifact")
            assert "${actual}" == "${layerTypeFlag#--artifact-layer-type=}" \
                   "layerTypeFlag=$layerTypeFlag, layer0.mediaType"
        else
            actual=$(jq -r '.layers[0].mediaType' <<<"$artifact")
            assert "${actual}" == "$defaulttype" \
                   "layerTypeFlag=NULL, layer0.mediaType"
        fi
        requested=${configTypeFlag#--artifact-config-type=}
        actual=$(jq -r '.config.mediaType' <<<"$artifact")
        if test -n "$requested" ; then
            assert "$actual" == "$requested" ".config.mediaType (requested)"
        else
            config=${configFlag#--artifact-config=}
            if [ -z "$config" ] ; then
                expected=application/vnd.oci.empty.v1+json
            else
                configSize=$(wc -c <"$config")
                if [ $configSize -gt 0 ] ; then
                    expected=application/vnd.oci.image.config.v1+json
                else
                    expected=application/vnd.oci.empty.v1+json
                fi
            fi
            assert "$actual" == "$expected" ".config.mediaType (default)"
        fi

        imgtitle=$(jq -r '.layers[0].annotations["org.opencontainers.image.title"]' <<<"$artifact")
        if test -n "$titleFlag" ; then
            assert "$imgtitle" == null "titleFlag=$titleFlag, .image.title"
        else
            assert "$imgtitle" == "$filetitle" \
                   "titleFlag=NULL, .image.title"
        fi
    done
    run_podman rmi $list localhost:${PODMAN_LOGIN_REGISTRY_PORT}/$list
}

@test "manifest list --add --artifact" {
    # Build a list and add some files to it, making sure to exercise and verify
    # every flag available.
    skip_if_remote "running a local registry doesn't work with podman-remote"
    local list="m-$(safename):1.0"
    truncate -s 20M ${PODMAN_TMPDIR}/zeroes
    echo oh yeah > ${PODMAN_TMPDIR}/listed.txt
    echo '{}' > ${PODMAN_TMPDIR}/minimum-config.json
    local listFlags platformFlags typeFlag configTypeFlag configFlag layerTypeFlag titleFlag
    for listFlags in "" "--annotation global=local" ; do
        manifestListAddArtifactOnce
    done
    for platformFlags in "" "--os=linux --arch=amd64" ; do
        manifestListAddArtifactOnce
    done
    for typeFlag in "" --artifact-type="" --artifact-type=application/octet-stream --artifact-type=text/plain ; do
        manifestListAddArtifactOnce
    done
    for configTypeFlag in "" --artifact-config-type=application/octet-stream --artifact-config-type=text/plain ; do
        for configFlag in "" --artifact-config= --artifact-config=${PODMAN_TMPDIR}/minimum-config.json ; do
            manifestListAddArtifactOnce
        done
    done
    for layerTypeFlag in "" --artifact-layer-type=application/octet-stream --artifact-layer-type=text/plain ; do
        manifestListAddArtifactOnce
    done
    for titleFlag in "" "--artifact-exclude-titles" ; do
        manifestListAddArtifactOnce
    done
}
# vim: filetype=sh
