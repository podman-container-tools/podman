#!/bin/bash

set -x

#
# This script is intended to help developers contribute to the podman project. It
# checks various pre-CI checks like building, linting, man-pages, etc.  It is meant
# to be run in a specific container environment.

build() {
    err=""

    echo "Building windows"
    if ! GOOS=windows CGO_ENABLED=0 go build -tags "$REMOTETAGS" -o bin/podman-remote-windows ./cmd/podman; then
        err+="\n - Windows "
    fi

    echo "Building darwin"
    if ! GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -tags "$REMOTETAGS" -o bin/podman-remote-darwin ./cmd/podman; then
        err+="\n - Darwin "
    fi

    echo "Building podman binaries"
    if ! make binaries; then
        err+="\n - Additional Binaries "
    fi

    if [ ! -z "$err" ]
    then
        echo -e "\033[31mFailed to build: ${err}\033[0m">&2
        exit 1
    fi
}

validate(){
    echo "Running validation tooling"

    # golangci-lint gobbles memory.
    # By default, podman machines only have 2GB memory,
    # often causing the linter be killed when run on Darwin/Windows
    mem=$(awk '/MemTotal/ {print $2}' /proc/meminfo)
    if (( $((mem)) < 3900000 )); then
        echo -e "\033[33mWarning: Your machine may not have sufficient memory (< 4 GB)to run the linter. \
If the process is killed, please allocate more memory.\033[0m">&2
    fi

    make validate
}

build
validate
