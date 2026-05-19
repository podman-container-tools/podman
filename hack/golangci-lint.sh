#!/usr/bin/env bash

# Run golangci-lint with different sets of build tags.
set -e

# WARNING: This script executes on multiple operating systems that
# do not have the same version of Bash.  Specifically, Darwin uses
# a very old version, where modern features (like `declare -A`) are
# absent.

declare -a EXTRA_TAGS

# Prefer the one from $PATH, if available.
BIN="$(command -v golangci-lint || echo ./bin/golangci-lint)"

echo "Linting for GOOS=$GOOS"
case "$GOOS" in
  windows|darwin)
    # For Darwin and Windows, only "remote" linting is possible and required.
    TAGS="remote,containers_image_openpgp"
    ;;
  freebsd)
    TAGS="containers_image_openpgp"
    EXTRA_TAGS=(",remote")
    ;;
  *)
    # Assume Linux: run linter for various sets of build tags.
    TAGS="apparmor,seccomp,selinux"
    EXTRA_TAGS=(",systemd" ",remote")
esac

for EXTRA in "" "${EXTRA_TAGS[@]}"; do
  # Use set -x in a subshell to make it easy for a developer to copy-paste
  # the command-line to focus or debug a single, specific linting category.
  (
    set -x
    "$BIN" run --build-tags="${TAGS}${EXTRA}" "$@"
  )
done
