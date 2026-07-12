#!/usr/bin/env bats   -*- bats -*-
#
# PR #15673: For all commands that accept --format '{{.GoTemplate}}',
# invoke with --format '{{"\n"}}' and make sure they don't choke.
#

load helpers

function teardown() {
    # In case test fails: clean up all the entities we created
    run_podman '?' machine rm -f "m-$(safename)"
    run_podman '?' secret rm "s-$(safename)"
    run_podman '?' pod rm -f "p-$(safename)"
    run_podman '?' rm -f -t0 "c-$(safename)"
    run_podman '?' artifact rm "a-$(safename)"

    basic_teardown
}

# podman machine is finicky. Assume we can't run it, but see below for more.
can_run_podman_machine=

# podman stats, too
can_run_stats=

# Main test loop. Recursively runs 'podman [subcommand] help', looks for:
#    > '[command]', which indicates, recurse; or
#    > '--format', in which case we
#      > check autocompletion, look for Go templates, in which case we
#        > run the command with --format '{{"\n"}}' and make sure it passes
function check_subcommand() {
    for cmd in $(_podman_commands "$@"); do
        # Skip the compose command which is calling `docker-compose --help`
        # and hence won't match the assumptions made below.
        if [[ "$cmd" == "compose" ]]; then
            continue
        fi
        # Human-readable podman command string, with multiple spaces collapsed
        # Special case: 'podman machine' can only be run under ideal conditions
        if [[ "$cmd" = "machine" ]] && [[ -z "$can_run_podman_machine" ]]; then
            continue
        fi
        if [[ "$cmd" = "stats" ]] && [[ -z "$can_run_stats" ]]; then
            continue
        fi

        # Human-readable podman command string, with multiple spaces collapsed
        command_string="podman $* $cmd"
        command_string=${command_string//  / }  # 'podman  x' -> 'podman x'

        # Run --help, decide if this is a subcommand with subcommands
        run_podman "$@" $cmd --help
        local full_help="$output"

        # The line immediately after 'Usage:' gives us a 1-line synopsis
        usage=$(echo "$full_help" | grep -A1 '^Usage:' | tail -1)
        assert "$usage" != "" "podman $cmd: no Usage message found"

        # Strip off the leading command string; we no longer need it
        usage=$(sed -e "s/^  $command_string \?//" <<<"$usage")

        # If usage ends in '[command]', recurse into subcommands
        if expr "$usage" : '\[command\]' >/dev/null; then
            # (except for 'podman help', which is a special case)
            if [[ $cmd != "help" ]]; then
                check_subcommand "$@" $cmd
            fi
            continue
        fi

        # Not a subcommand-subcommand. Look for --format option
        if [[ ! "$output" =~ "--format" ]]; then
            continue
        fi

        # Have --format. Make sure it's a Go-template option, not like --push
        run_podman __completeNoDesc "$@" "$cmd" --format '{{.'
        if [[ ! "$output" =~ \{\{\.[A-Z] ]]; then
            continue
        fi

        # Got one.
        dprint "$command_string has --format"

        # Whatever is needed to make a runnable command
        local extra=${extra_args[$command_string]}
        if [[ -n "$extra" ]]; then
            # Cross off our list
            unset extra_args["$command_string"]
        fi

        # This is what does the work. We run with '?' so we can offer
        # better error messages than just "exited with error status".
        run_podman '?' "$@" "$cmd" $extra --format '{{"\n"}}'

        # Special-case workaround for quay flakes
        if [[ "$status" -ne 0 ]] && [[ "$*" =~ search ]] && [[ "$output" =~ quay.io ]]; then
            echo "# Ignoring probable quay flake: $output" >&3
            continue
        fi

        # Output must always be empty.
        #
        #  - If you see "unterminated quoted string" here, there's a
        #    regression, and you need to fix --format (see PR #15673)
        #
        #  - If you see any other error, it probably means that someone
        #    added a new podman subcommand that supports --format but
        #    needs some sort of option or argument to actually run.
        #    See 'extra_args_table' below.
        #
        assert "$output" = "" "$command_string --format '{{\"\n\"}}'"

        # *Now* check exit status. This should never, ever, ever trigger!
        # If it does, it means the podman command failed without an err msg!
        assert "$status" = "0" \
               "$command_string --format '{{\"\n\"}}' failed with no output!"
    done
}

# Test entry point
# bats test_tags=ci:parallel
@test "check Go template formatting" {
    skip_if_remote

    ctrname="c-$(safename)"
    podname="p-$(safename)"
    secretname="s-$(safename)"
    # Artifact strict typing requires fully qualified name
    artifactname="foo.io/myrepo/$(safename)"
    # Setup: some commands need a container, pod, secret, ...
    run_podman run -d --name $ctrname $IMAGE top
    run_podman pod create $podname
    run_podman secret create $secretname /etc/hosts
    run_podman artifact add $artifactname /etc/hosts

    # For 'search' and 'image search': if local cache registry is available,
    # use it. This bypasses quay, and thus prevents flakes.
    searchargs=$IMAGE
    if [[ -n "$CI_USE_REGISTRY_CACHE" ]]; then
        # FIXME: someday: find a way to refactor the hardcoded port
        searchargs="--tls-verify=false 127.0.0.1:60333/we/dontactuallyneed:arealimage"
    fi

    # Most commands can't just be run with --format; they need an argument or
    # option. This table defines what those are.
    extra_args_table="
history           | $IMAGE
image history     | $IMAGE
image inspect     | $IMAGE
container inspect | $ctrname
inspect           | $ctrname

artifact inspect  | $artifactname
volume inspect    | -a
secret inspect    | $secretname
network inspect   | podman
ps                | -a

image search      | $searchargs
search            | $searchargs

pod inspect       | $podname

events            | --stream=false --events-backend=file
system events     | --stream=false --events-backend=file
"



    # ...or machine. But podman machine is ultra-finicky, it fails as root
    # or if qemu is missing. Instead of checking for all the possible ways
    # to skip it, just try running init. If it works, we can test it.
    machinename="m-$(safename)"
    run_podman '?' machine init --image=/dev/null $machinename
    if [[ $status -eq 0 ]]; then
        can_run_podman_machine=true
        extra_args_table+="
machine inspect   | $machinename
"
    fi

    # Similarly, 'stats' cannot run rootless under cgroups v1
    if ! is_rootless || is_cgroupsv2; then
        can_run_stats=true
        extra_args_table+="
container stats   | --no-stream
pod stats         | --no-stream
stats             | --no-stream
"
    fi

    # Convert the table at top to an associative array, keyed on subcommand
    declare -A extra_args
    while read subcommand extra; do
        extra_args["podman $subcommand"]=$extra
    done < <(parse_table "$extra_args_table")

    # Run the test
    check_subcommand

    # Clean up
    run_podman pod rm $podname
    run_podman rm -f -t0 $ctrname
    run_podman secret rm $secretname
    run_podman '?' machine rm -f $machinename
    run_podman artifact rm $artifactname

    # Make sure there are no leftover commands in our table - this would
    # indicate a typo in the table, or a flaw in our logic such that
    # we're not actually recursing.
    local leftovers="${!extra_args[@]}"
    assert "$leftovers" = "" "Did not find (or test) subcommands:"
}

# vim: filetype=sh
