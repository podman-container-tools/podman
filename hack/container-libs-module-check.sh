#!/usr/bin/env bash

set -e

function die() {
    echo "$1" >&2
    exit 1
}

# All these modules are part of the same git monorepo, https://github.com/podman-container-tools/container-libs/
MODULES=(go.podman.io/common go.podman.io/image/v5 go.podman.io/storage)
VERSIONS=()

for module in "${MODULES[@]}"; do
    VERSIONS+=($(go list -m -f '{{.Version}}' $module))
done

commit=""
is_tag=false
is_commit=false
for version in "${VERSIONS[@]}"; do
    # match pseudo version format, https://go.dev/ref/mod#pseudo-versions
    if [[ $version =~ .*[0-9]{14}-([0-9a-f]{12}) ]]; then
        new_commit="${BASH_REMATCH[1]}"
        if [[ "$commit" != "" && "$commit" != "$new_commit" ]]; then
            die "go.mod contains different commits for the container-libs repo, please ensure all three modules use the same commit. commit 1: $commit, commit 2: $new_commit"
        fi
        commit="$new_commit"
        is_commit=true
    else
        is_tag=true
    fi
done

if [[ $is_commit = true && $is_tag = true ]]; then
    die "go.mod contains tagged versions and commits for the container-libs repo, please either only use tagged versions or only specific commits for all three modules"
fi
