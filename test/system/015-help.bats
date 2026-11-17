#!/usr/bin/env bats
#
# Tests based on 'podman help'
#
# Find all commands listed by 'podman --help'. Run each one, make sure it
# provides its own --help output. If the usage message ends in '[command]',
# treat it as a subcommand, and recurse into its own list of sub-subcommands.
#
# Any usage message that ends in '[options]' is interpreted as a command
# that takes no further arguments; we confirm by running with 'invalid-arg'
# and confirming that it exits with error status and message.
#
load helpers

function check_help() {
    local count=0
    local -A found

    for cmd in $(_podman_commands "$@"); do
        # Skip the compose command which is calling `docker-compose --help`
        # and hence won't match the assumptions made below.
        if [[ "$cmd" == "compose" ]]; then
            continue
        fi
        # Human-readable podman command string, with multiple spaces collapsed
        command_string="podman $* $cmd"
        command_string=${command_string//  / }  # 'podman  x' -> 'podman x'

        dprint "$command_string --help"
        run_podman "$@" $cmd --help
        local full_help="$output"

        # The line immediately after 'Usage:' gives us a 1-line synopsis
        usage=$(echo "$full_help" | grep -A1 '^Usage:' | tail -1)
        assert "$usage" != "" "podman $cmd: no Usage message found"

        # e.g. 'podman ps' should not show 'podman container ps' in usage
        # Trailing space in usage handles 'podman system renumber' which
        # has no ' [options]'
        is "$usage " "  $command_string .*" "Usage string matches command"

        # Strip off the leading command string; we no longer need it
        usage=$(sed -e "s/^  $command_string \?//" <<<"$usage")

        # If usage ends in '[command]', recurse into subcommands
        if expr "$usage" : '\[command\]' >/dev/null; then
            found[subcommands]=1
            # (except for 'podman help', which is a special case)
            if [[ $cmd != "help" ]]; then
                check_help "$@" $cmd
            fi
            continue
        fi

        # We had someone write upper-case '[OPTIONS]' once. Prevent it.
        assert "$usage" !~ '\[OPTION' \
               "'options' string must be lower-case in usage"

        # We had someone do 'podman foo ARG [options]' one time. Yeah, no.
        assert "$usage" !~ '[A-Z].*\[option' \
               "'options' must precede arguments in usage"

        # Strip off '[options]' but remember if we've seen it.
        local has_options=
        if [[ $usage =~ \[options\] ]]; then
            has_options=1
            usage=$(sed -e 's/^\[options\] \?//' <<<"$usage")
        fi

        # From this point on, remaining argument descriptions must be UPPER CASE
        # e.g., 'podman cmd [options] arg' or 'podman cmd [arg]' are invalid.
        assert "$usage" !~ '[a-z]' \
               "$command_string: argument names must be UPPER CASE"

        # It makes no sense to have an optional arg followed by a mandatory one
        assert "$usage" !~ '\[.*\] [A-Z]' \
               "$command_string: optional args must be _after_ required ones"

        # Cross-check: if usage includes '[options]', there must be a
        # longer 'Options:' section in the full --help output; vice-versa,
        # if 'Options:' is in full output, usage line must have '[options]'.
        if [[ $has_options ]]; then
            if ! expr "$full_help" : ".*Options:" >/dev/null; then
                die "$command_string: Usage includes '[options]' but has no 'Options:' subsection"
            fi
        elif expr "$full_help" : ".*Options:" >/dev/null; then
            die "$command_string: --help has 'Options:' section but no '[options]' in synopsis"
        fi

        # If usage lists no arguments (strings in ALL CAPS), confirm
        # by running with 'invalid-arg' and expecting failure.
        if ! expr "$usage" : '.*[A-Z]' >/dev/null; then
            if [ "$cmd" != "help" ]; then
                dprint "$command_string invalid-arg"
                run_podman '?' "$@" $cmd invalid-arg
                is "$status" 125 \
                   "'$usage' indicates that the command takes no arguments. I invoked it with 'invalid-arg' and expected an error status"
                is "$output" "Error: .* takes no arguments" \
                   "'$usage' indicates that the command takes no arguments. I invoked it with 'invalid-arg' and expected the following error message"
            fi
            found[takes_no_args]=1
        fi

        # If command lists "-l, --latest" in help output, combine -l with arg.
        # This should be disallowed with a clear message.
        if expr "$full_help" : ".*-l, --latest" >/dev/null; then
            local nope="exec list port ps top"   # these can't be tested
            if is_rootless; then
                nope="$nope mount restore"       # these don't work rootless
            fi
            if ! grep -wq "$cmd" <<<$nope; then
                run_podman 125 "$@" $cmd -l nonexistent-container
                is "$output" "Error: .*--latest and \(containers\|pods\|arguments\) cannot be used together" \
                   "'$command_string' with both -l and container"

                # Combine -l and -a, too (but spell it as --all, because "-a"
                # means "attach" in podman container start)
                run_podman 125 "$@" $cmd --all --latest
                is "$output" "Error: \(--all and --latest cannot be used together\|--all, --latest and containers cannot be used together\|--all, --latest and arguments cannot be used together\|unknown flag\)" \
                   "'$command_string' with both --all and --latest"
            fi
        fi

        # If usage has required arguments, try running without them.
        if expr "$usage" : '[A-Z]' >/dev/null; then
            # Exceptions: these commands don't work rootless
            if is_rootless; then
                # "pause is not supported for rootless containers"
                if [[ "$cmd" = "pause" ]] || [[ "$cmd" = "unpause" ]]; then
                    continue
                fi
                # "network rm" too
                if [ "$@" = "network" -a "$cmd" = "rm" ]; then
                    continue
                fi
            fi

            # The </dev/null protects us from 'podman login' which will
            # try to read username/password from stdin.
            dprint "$command_string (without required args)"
            run_podman '?' "$@" $cmd </dev/null
            is "$status" 125 \
               "'$usage' indicates at least one required arg. I invoked it with no args and expected an error exit code"
            is "$output" "Error:.* \(require\|specif\|must\|provide\|need\|choose\|accepts\)" \
               "'$usage' indicates at least one required arg. I invoked it with no args and expected one of these error messages"

            found[required_args]=1
        fi

        # Commands with fixed number of arguments (i.e. no ellipsis): count
        # the required args, then invoke with one extra. We should get a
        # usage error.
        if ! expr "$usage" : ".*\.\.\."; then
            local n_args=$(wc -w <<<"$usage")

            run_podman '?' "$@" $cmd $(seq --format='x%g' 0 $n_args)
            is "$status" 125 \
               "'$usage' indicates a maximum of $n_args args. I invoked it with more, and expected this exit status"
            is "$output" "Error:.* \(takes no arguments\|requires exactly $n_args arg\|accepts at most\|too many arguments\|accepts $n_args arg(s), received\|accepts between .* and .* arg(s), received \)" \
               "'$usage' indicates a maximum of $n_args args. I invoked it with more, and expected one of these error messages"

            found[fixed_args]=1
        fi

        count=$(expr $count + 1)
    done

    # Any command that takes subcommands, prints its help and errors if called
    # without one.
    dprint "podman $*"
    run_podman '?' "$@"
    is "$status" 125 "'podman $*' without any subcommand - exit status"
    is "$output" ".*Usage:.*Error: missing command '.*$* COMMAND'" \
       "'podman $*' without any subcommand - expected error message"

    # Assume that 'NoSuchCommand' is not a command
    dprint "podman $* NoSuchCommand"
    run_podman '?' "$@" NoSuchCommand
    is "$status" 125 "'podman $* NoSuchCommand' - exit status"
    is "$output" "Error: unrecognized command .*$* NoSuchCommand" \
       "'podman $* NoSuchCommand' - expected error message"

    # This can happen if the output of --help changes, such as between
    # the old command parser and cobra.
    assert "$count" -gt 0 \
           "Internal error: no commands found in 'podman help $*' list"

    # Sanity check: make sure the special loops above triggered at least once.
    # (We've had situations where a typo makes the conditional never run)
    if [ -z "$*" ]; then
        for i in subcommands required_args takes_no_args fixed_args; do
            assert "${found[$i]}" != "" \
                   "Internal error: '$i' subtest did not trigger"
        done
    fi
}


# bats test_tags=ci:parallel
@test "podman help - basic tests" {
    skip_if_remote

    # Called with no args -- start with 'podman --help'. check_help() will
    # recurse for any subcommands.
    check_help

    # Test for regression of #7273 (spurious "--remote" help on output)
    for helpopt in help --help -h; do
        run_podman $helpopt
        is "${lines[0]}" "Manage pods, containers and images" \
           "podman $helpopt: first line of output"
    done

}

# vim: filetype=sh
