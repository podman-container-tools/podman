# -*- bash -*-

load "../system/helpers"
load "../system/helpers.network"	# For random_free_port()

setup() {
    :
}

teardown() {
    :
}

# skip a test when the given version is older than the currently tested one
skip_if_version_older() {
    if version_is_older_than $1; then
        skip "${2-test is only meaningful when upgrading from $1 or later}"
    fi
}

version_is_older_than() {
    # The '##v' strips off leading 'v'
    printf '%s\n%s\n' "${PODMAN_UPGRADE_FROM##v}" "$1" | sort --check=quiet --version-sort
}
