#!/usr/bin/env bats   -*- bats -*-
#
# Various command-line parsing regression tests that don't fit in elsewhere
#

load helpers

@test "podman cli parsing - quoted args - #2574" {
    # 1.1.2 fails with:
    #   Error: invalid argument "true=\"false\"" for "-l, --label" \
    #      flag: parse error on line 1, column 5: bare " in non-quoted-field
    run_podman run --rm --label 'true="false"' $IMAGE true
}

@test "podman flag error" {
    local name="podman"
    if is_remote; then
        name="podman-remote"
    fi
    run_podman 125 run -h
    is "$output" "Error: flag needs an argument: 'h' in -h
See '$name run --help'" "expected error output"

    run_podman 125 bad --invalid
    is "$output" "Error: unknown flag: --invalid
See '$name --help'" "expected error output"
}

###############################################################################
# BEGIN tests for environment-variable passthrough

# Helper for all tests below. Reads output file containing 'env -0' dump,
# then cross-checks it against the '$expect' associative array.
# KLUDGE NOTE: this function relies on 'declare -A expect' from caller.
# Gross, but it's a reasonable tradeoff.
function _check_env {
    local resultsfile="$1"

    # resultsfile contains the output of 'env -0' from the container.
    # Translate that info a local associative array.
    declare -A env_results
    # -d '' means NUL delimiter
    while read -d '' result;do
        # Split on equal sign into key and val. -d '' lets us read a
        # multiline string (containing newlines). But since there is no
        # actual NUL in the string (because bash does not allow them),
        # the 'read' will fail with an EOF error; hence the ||true
        IFS='=' read -d '' key val <<<"$result" || true

        # Got them, but (sigh again) bash adds a trailing newline. Nuke it.
        env_results[$key]="${val%$'\n'}"

        # For debugging failures
        printf "_check_env: got %q = %q\n" "$key" "${env_results[$key]}"
    done <"$resultsfile"

    # Compare against $expect. 'found' protects us from coding errors; it
    # would be easy to mistype all those dollar-curly-bang-ats and end up
    # with an empty loop.
    local found=0
    for key in "${!expect[@]}"; do
        want="${expect[$key]}"
        assert "${env_results[$key]}" = "$want" "\$$key"
        found=$((found + 1))
    done
    assert "$found" -gt 3 "Sanity check to make sure we're not NOPing"
}

@test "podman run --env" {
    # Environment variable names, with their settings.
    declare -A expect=(
        [simple]="abc"
        [special]="bcd#e!f|g hij=klmnop"
        [bel]=$'\a'
        [withnl]=$'aaa\nbbb'
        [we.ird*na#me!?]="yeah... weird indeed"
    )

    # Convert to command-line form, "--env X=Y" for each of the above
    declare -a env_args
    for v in "${!expect[@]}"; do
        env_args+=("--env" "$v=${expect[$v]}")
    done

    # Special case, test short option "-e"
    expect[dash_e]="short opt"
    env_args+=("-e" "dash_e=${expect[dash_e]}")

    # Use 'env -0' to write NUL-delimited strings to a file:
    #  - NUL-delimited, because otherwise we can't handle multiline strings
    #  - file, because bash does not allow NUL in strings
    # results will be read and checked by helper function above.
    local resultsfile="$PODMAN_TMPDIR/envresults"
    touch $resultsfile
    run_podman run --rm -v "$resultsfile:/envresults:Z"  \
               "${env_args[@]}"                          \
               $IMAGE sh -c 'env -0 >/envresults'

    _check_env $resultsfile
}


@test "podman run/exec --env-file" {
    declare -A expect=(
        [simple]="abc"
        [special]="bcd#e!f|g hij=lmnop"
        [bel]=$'\a'
        [withnl]=$'"line1\nline2"'
        [withquotes]='"withquotes"'
        [withsinglequotes]="'withsingle'"
    )

    # Special cases, cannot be handled in our loop
    local weirdname="got-star"
    local infile2="this is set in env-file 2"

    # Write two files, so we confirm that podman can accept multiple values
    # and that the second will override the first
    local envfile1="$PODMAN_TMPDIR/envfile-in-1,withcomma"
    local envfile2="$PODMAN_TMPDIR/envfile-in-2"
    cat >$envfile1 <<EOF
infile2=this is set in env-file-1 and should be overridden in env-file-2
simple=THIS SHOULD BE OVERRIDDEN
simple=BY THE EXPECT VALUE WRITTEN BELOW

# Empty lines and comments ignored
EOF
    for v in "${!expect[@]}"; do
        echo "$v=${expect[$v]}" >>$envfile1
    done

    # Remember, just because a token isn't a valid bash/shell variable
    # identified doesn't mean it's not a valid environment variable.
    cat >$envfile2 <<EOF
infile2=$infile2
weird*na#me!=$weirdname

# Comments ignored
EOF

    # FIXME: add tests for 'var' and 'var*' (i.e. from environment)

    # For debugging
    echo "$_LOG_PROMPT cat $envfile1 $envfile2:"
    cat -vET $envfile1
    echo "-----------------"
    cat -vET $envfile2

    # See above for reasoning behind 'env -0' and a results file
    local resultsfile="$PODMAN_TMPDIR/envresults"
    touch $resultsfile
    run_podman run --rm -v "$resultsfile:/envresults:Z" \
               --env-file $envfile1                     \
               --env-file $envfile2                     \
               $IMAGE sh -c 'env -0 >/envresults'

    expect[withnl]=$'"line1'
    expect[weird*na#me!]=$weirdname

    _check_env $resultsfile

    # Now check the same with podman exec
    run_podman run -d --name testctr        \
            -v "$resultsfile:/envresults:Z" \
            $IMAGE top

    run_podman exec --env-file $envfile1 \
            --env-file $envfile2 testctr \
            sh -c 'env -0 >/envresults'

    _check_env $resultsfile

    run_podman rm -f -t0 testctr
}

# Obscure feature: '--env FOO*' will pass all env starting with FOO
@test "podman run --env with glob" {
    # Set a bunch of different envariables with a common prefix
    local prefix="env$(random_string 10)"

    # prefix by itself
    eval export $prefix=\"just plain basename\"
    declare -A expect=([$prefix]="just plain basename")

    for i in 1 a x _ _xyz CAPS_;do
        eval export $prefix$i="$i"
        expect[$prefix$i]="$i"
    done

    # passthrough is case-sensitive; upper-case var should not be relayed
    prefix_caps=${prefix^^}
    eval export $prefix_caps="CAPS-NOT-ALLOWED"
    expect[$prefix_caps]=

    # Asterisk only valid at end
    export NOTREALLYRANDOMBUTPROBABLYNOTDEFINED="probably not defined"

    local resultsfile="$PODMAN_TMPDIR/envresults"
    touch $resultsfile
    run_podman run --rm -v "$resultsfile:/envresults:Z" \
               --env "${prefix}*"                       \
               --env 'NOT*DEFINED'                      \
               $IMAGE sh -c 'env -0 >/envresults'

    _check_env $resultsfile

    if grep "DEFINED" "$resultsfile"; then
        die "asterisk in middle (NOT*DEFINED) got expanded???"
    fi

    # Same, with --env-file
    local envfile="$PODMAN_TMPDIR/envfile-in-1,withcomma"
    cat >$envfile <<EOF
$prefix*
NOT*DEFINED
EOF

    run_podman run --rm -v "$resultsfile:/envresults:Z" \
               --env-file $envfile                      \
               $IMAGE sh -c 'env -0 >/envresults'

    # UGLY! If this fails, the error message will not make it clear if the
    # failure was in --env of --env-file. It can be determined by skimming
    # up and looking at the run_podman command, so I choose to leave as-is.
    _check_env $resultsfile
}


@test "podman create --label-file" {
    declare -A expect=(
        [simple]="abc"
        [special]="bcd#e!f|g hij=lmnop"
        [withquotes]='"withquotes"'
        [withsinglequotes]="'withsingle'"
    )

    # Write two files, so we confirm that podman can accept multiple values
    # and that the second will override the first
    local labelfile1="$PODMAN_TMPDIR/label-file1,withcomma"
    local labelfile2="$PODMAN_TMPDIR/label-file2"

        cat >$labelfile1 <<EOF
simple=value1

# Comments ignored
EOF

    for v in "${!expect[@]}"; do
        echo "$v=${expect[$v]}" >>$labelfile2
    done

    run_podman create --rm --name testctr --label-file $labelfile1  \
               --label-file $labelfile2 $IMAGE

    for v in "${!expect[@]}"; do
        run_podman inspect testctr --format "{{index .Config.Labels \"$v\"}}"
        assert "$output" == "${expect[$v]}" "label $v"
    done

    run_podman rm testctr
}



# vim: filetype=sh
