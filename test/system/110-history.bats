#!/usr/bin/env bats

load helpers

# bats test_tags=ci:parallel
@test "podman history - basic tests" {
    tests="
                                 | .*[0-9a-f]\\\{12\\\} .* CMD .* LABEL
--format '{{.ID}} {{.Created}}'  | .*[0-9a-f]\\\{12\\\} .* ago
--human=false                    | .*[0-9a-f]\\\{12\\\} *[0-9-]\\\+T[0-9:]\\\+Z
-qH                              | .*[0-9a-f]\\\{12\\\}
--no-trunc                       | .*[0-9a-f]\\\{64\\\}
"

    defer-assertion-failures

    while read options expect; do
        if [ "$options" = "''" ]; then options=; fi

        eval set -- "$options"

        run_podman history "$@" $IMAGE
        is "$output" "$expect" "podman history $options"
    done < <(parse_table "$tests")
}

# bats test_tags=ci:parallel
@test "podman history - custom format" {
    run_podman history --format "--{{.ID}}--{{.Created}}--{{.CreatedBy}}--" $IMAGE

    defer-assertion-failures

    for i in $(seq 1 "${#lines[*]}"); do
        # most layer IDs are "<missing>" but at least one should be a SHA
        assert "${lines[$((i-1))]}" =~ "^--([0-9a-f]+|<missing>)--.* ago--/bin/sh -c.*" \
               "history line $i"
    done

    # Normally this should be "[$IMAGE]" (bracket IMAGE bracket).
    # When run in parallel with other tests, the image may have
    # other tags.
    run_podman history --format "{{.Tags}}" $IMAGE
    assert "${lines[0]}" =~ "( |\[)$IMAGE(\]| )" "podman history tags"
}

# bats test_tags=ci:parallel
@test "podman history - json" {
    # Sigh. Timestamp in .created can be '...Z' or '...-06:00'
    tests="
id        | [0-9a-f]\\\{64\\\}
created   | [0-9-]\\\+T[0-9:.]\\\+[Z0-9:+-]\\\+
size      | -\\\?[0-9]\\\+
"

    run_podman history --format json $IMAGE

    defer-assertion-failures

    while read field expect; do
        # HACK: we can't include '|' in the table
        if [ "$field" = "id" ]; then expect="$expect\|<missing>";fi

        # output is an array of dicts; check each one
        count=$(echo "$output" | jq '. | length')
        i=0
        while [ $i -lt $count ]; do
            actual=$(echo "$output" | jq -r ".[$i].$field")
            is "$actual" "$expect\$" "jq .[$i].$field"
            i=$(expr $i + 1)
        done
    done < <(parse_table "$tests")
}

# bats test_tags=ci:parallel
@test "podman image history Created" {
    # Values from image LIST
    run_podman image list --format '{{.CreatedSince}}\n{{.CreatedAt}}' $IMAGE
    imagelist_since="${lines[0]}"
    imagelist_at="${lines[1]}"

    assert "${imagelist_since}" =~ "^[0-9]+.* ago" \
           "image list: CreatedSince looks reasonable"
    assert "${imagelist_at}" =~ "^[0-9]+-[0-9]+-[0-9]+ [0-9:]+ \+0000 UTC\$" \
           "image list: CreatedAt looks reasonable"

    # Values from image HISTORY. For docker compatibility, this command now
    # honors $TZ (#18213) for CreatedAt.
    TZ=UTC run_podman image history --format '{{.CreatedSince}}\n{{.CreatedAt}}' $IMAGE
    imagehistory_since="${lines[0]}"
    imagehistory_at="${lines[1]}"

    assert "$imagehistory_since" == "$imagelist_since" \
           "CreatedSince from image history should == image list"

    # More docker compatibility: both commands emit ISO8601-ish dates but
    # with different separators so we need to compare date & time separately.
    assert "${imagehistory_at:0:10}" == "${imagelist_at:0:10}" \
           "CreatedAt (date) from image history should == image list"
    assert "${imagehistory_at:11:8}" == "${imagelist_at:11:8}" \
           "CreatedAt (time) from image history should == image list"
}

# vim: filetype=sh
