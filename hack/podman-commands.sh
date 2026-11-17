#!/usr/bin/env bash
#
# Compare commands listed by 'podman help' against those in 'man podman'.
# Recurse into subcommands as well.
#
# Because we read metadoc files in the `docs` directory, this script
# must run from the top level of a git checkout. FIXME: if necessary,
# it could instead run 'man podman-XX'; my thinking is that this
# script should run early in CI.
#

# override with, e.g.,  PODMAN=./bin/podman-remote
PODMAN=${PODMAN:-./bin/podman}

function die() {
    echo "FATAL: $*" >&2
    exit 1
}

# Run 'podman help' (possibly against a subcommand, e.g. 'podman help image')
# and return a list of each first word under 'Available Commands', that is,
# the command name but not its description.
function podman_commands() {
    $PODMAN help "$@" |
        awk '/^Available Commands:/{ok=1;next}/^Options:/{ok=0}ok { print $1 }' |
        grep .

    # Special case: podman-completion is a hidden command
    # it does not show in podman help so add it here
    if [[ -z "$@" ]]; then
        echo "completion"
    fi
}

# Read a list of subcommands from a command's metadoc
function podman_man() {
    if [ "$@" = "podman" ]; then
        # podman itself.
        # This md file has a table of the form:
        #   | [podman-cmd(1)\[(podman-cmd.1.md)   | Description ... |
        # For all such, print the 'cmd' portion (the one in brackets).
        sed -ne 's/^|\s\+\[podman-\([a-z]\+\)(1.*/\1/p' < docs/source/markdown/$1.1.md

        # Special case: there is no podman-help man page, nor need for such.
        echo "help"
        # Auto-update differs from other commands as it's a single command, not
        # a main and sub-command split by a dash.
        echo "auto-update"
    elif [ "$@" = "podman-image-trust" ]; then
        # Special case: set and show aren't actually in a table in the man page
        echo set
        echo show
    else
        # podman subcommand.
        # Each md file has a table of the form:
        #    | cmd | [podman-cmd(1)](podman-cmd.1.md) | Description ... |
        # For all such we find, with 'podman- in the second column, print the
        # first column (with whitespace trimmed)
        awk -F\| '$3 ~ /podman-/ { gsub(" ","",$2); print $2 }' < docs/source/markdown/$1.1.md
    fi
}

# The main thing. Compares help and man page; if we find subcommands, recurse.
rc=0
function compare_help_and_man() {
    echo
    echo "checking: podman $@"

    # e.g. podman, podman-image, podman-volume
    local basename=$(echo podman "$@" | sed -e 's/ /-/g')

    podman_commands "$@" | sort > /tmp/${basename}_help.txt
    podman_man $basename | sort > /tmp/${basename}_man.txt

    diff -u /tmp/${basename}_help.txt /tmp/${basename}_man.txt || rc=1

    # Now look for subcommands, e.g. container, image
    for cmd in $(< /tmp/${basename}_help.txt); do
        usage=$($PODMAN "$@" $cmd --help | grep -A1 '^Usage:' | tail -1)

        # if string ends in '[command]', recurse into its subcommands
        if expr "$usage" : '.*\[command\]$' > /dev/null; then
            compare_help_and_man "$@" $cmd
        fi
    done

    rm -f /tmp/${basename}_{help,man}.txt
}

compare_help_and_man

if [ $rc -ne 0 ]; then
    cat << EOF

**************************
** INTERPRETING RESULTS **
**************************************************************************
*
* The above results show differences between 'podman --help' and
* podman man pages.
*
* The 'checking:' header indicates the specific command (and possibly
* subcommand) being tested, e.g. podman --help vs docs/source/podman.1.md.
*
* A '-' indicates a subcommand present in 'podman --help' but not the
* corresponding man page.
*
* A '+' indicates a subcommand present in the man page but not --help.
*
**************************************************************************
EOF
fi

exit $rc
