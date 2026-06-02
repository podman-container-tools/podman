#!/bin/bash
#
# make-and-check-size.sh - wrapper around 'make' that also checks binary growth
#
# Introduced 2022-03-22 in #13518.
#
# This script is intended to be run via 'git rebase -x', in a form such as:
#
#     context_dir=$(mktemp -d --tmpdir make-size-check.XXXXXXX)
#     git rebase ${GIT_BASE_BRANCH}^ -x "hack/ci/make-and-check-size.sh $context_dir"
#     rm -rf $context_dir
#
# (Carefully note the '^' next to GIT_BASE_BRANCH!)
#
# A 'git rebase -x' has long been a part of our usual CI; it guarantees
# that each commit (whether in a single- or multi-commit PR) can be
# compiled individually.
#
# By adding the '^' to GIT_BASE_BRANCH we establish a baseline and store
# the binary sizes of each file (podman, podman-remote) prior to our PR.
#
# context_dir is a temporary directory used to store the original sizes
# of each binary file under bin/
#
# *IMPORTANT NOTE*: this script will leave the git checkout in a funky state!
# (because we rebase onto a nonterminal commit). I believe this is OK, since
# this script is only invoked in CI (from the validate-source workflow) on a
# throwaway checkout. Running this in a development environment would yield
# unpredictable results anyway, by rebasing onto origin/main by default and by
# leaving an aborted rebase on failure.
#
ME=$(basename $0)

###############################################################################
# BEGIN end-user-customizable settings

# Maximum allowable size, in bytes
MAX_BIN_GROWTH=$((50 * 1024))

# Github label which allows overriding this check
OVERRIDE_LABEL=bloat_approved

# END   end-user-customizable settings
###############################################################################

#
# Helper function: checks whether the size growth has been approved.
#
function bloat_approved() {
    # Argument is the actual size increase in this build.
    # FIXME: 2022-03-21: this is not actually used atm, but Ed hopes some day
    #        to implement a more robust size-override mechanism, such as by
    #        requiring a MAX_BIN_GROWTH=nnn statement in github comments.
    local actual_growth="$1"

    local var
    for var in PR_NUMBER GITHUB_TOKEN GITHUB_REPOSITORY; do
        if [[ -z "${!var}" ]]; then
            echo "$ME: cannot query github: \$$var is undefined" >&2
            return 1
        fi
    done

    labels=$(curl --fail -s \
        -H "Authorization: bearer $GITHUB_TOKEN" \
        -H "Accept: application/vnd.github+json" \
        "https://api.github.com/repos/${GITHUB_REPOSITORY}/pulls/$PR_NUMBER" |
        jq -r '.labels[].name')

    grep -F -x -q "$OVERRIDE_LABEL" <<< "$labels"
}

# ACTUAL CODE BEGINS HERE
set -e

# Must be invoked with one argument, an existing context directory
context_dir=${1?Missing CONTEXT-DIR argument}
if [[ ! -d $context_dir ]]; then
    echo "$ME: directory '$context_dir' does not exist"
    exit 1
fi

# This is the original (and primary) purpose of this check: if 'make' fails,
# there is no point in continuing.  Show at least the commit title since
# the ID may not match anything human recognisable.
echo
echo "Building: $(git log -n 1 --no-show-signature --oneline)"
make

# Determine size of each built file.
# - If this is our first time through, preserve that size in a tmpfile
# - On all subsequent runs, compare built size to initial size
for bin in bin/*;do
    size=$(stat -c %s $bin)

    saved_size_file=$context_dir/$(basename $bin)
    if [[ -e $saved_size_file ]]; then
        # Not the first time through: compare to original size
        size_orig=$(< $saved_size_file)
        delta_size=$(( size - size_orig ))

        printf "%-20s size=%9d  delta=%6d\n" $bin $size $delta_size
        if [[ "$bin" = bin/podman-testing ]]; then
            continue # We compute / list this for completeness, but size does not matter to users.
        fi
        if [[ $delta_size -gt $MAX_BIN_GROWTH ]]; then
            separator=$(printf "%.0s*" {1..75})   # row of stars, for highlight
            echo "$separator"
            echo "* $bin grew by $delta_size bytes; max allowed is $MAX_BIN_GROWTH."
            echo "*"
            if bloat_approved $delta_size; then
                echo "* Continuing due to '$OVERRIDE_LABEL' label"
                echo "*"
                echo "$separator"
            else
                echo "* Please investigate, and fix if possible."
                echo "*"
                echo "* A repo admin can override by setting the $OVERRIDE_LABEL label"
                echo "$separator"
                exit 1
            fi
        fi
    else
        # First time through: preserve original file size
        echo $size >$saved_size_file
        printf "%-20s size=%9d\n" $bin $size
    fi
done
