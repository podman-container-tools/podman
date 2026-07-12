#!/usr/bin/env bash
#
# regression tests for helpers.bash
#
# Some of those helper functions are fragile, and we don't want to break
# anything if we have to mess with them.
#

source "$(dirname $0)"/helpers.bash
source "$(dirname $0)"/helpers.network.bash

die() {
    echo "$(basename $0): $*" >&2
    exit 1
}

# Iterator and return code; updated in check_result()
testnum=0
rc=0

# Possibly used by the code we're testing
PODMAN_TMPDIR=$(mktemp -d --tmpdir=${TMPDIR:-/tmp} podman_helper_tests.XXXXXX)
trap 'rm -rf $PODMAN_TMPDIR' 0

# Used by random_free_port.
PORT_LOCK_DIR=$PODMAN_TMPDIR/reserved-ports

###############################################################################
# BEGIN test the parse_table helper

function check_result {
    testnum=$(expr $testnum + 1)
    if [ "$1" = "$2" ]; then
        # Multi-level echo flattens newlines, makes success messages readable
        echo $(echo "ok $testnum $3 = $1")
    else
        echo "not ok $testnum $3"
        echo "#  expected: $2"
        echo "#    actual: $1"
        rc=1
    fi
}

# IMPORTANT NOTE: you have to do
#      this: while ... done < <(parse_table)
#   and not: parse_table | while read ...
#
# ...because piping to 'while' makes it a subshell, hence testnum and rc
# will not be updated.
#
while read x y z; do
    check_result "$x" "a" "parse_table simple: column 1"
    check_result "$y" "b" "parse_table simple: column 2"
    check_result "$z" "c" "parse_table simple: column 3"
done < <(parse_table "a | b | c")

# More complicated example, with spaces
while read x y z; do
    check_result "$x" "a b"   "parse_table with spaces: column 1"
    check_result "$y" "c d"   "parse_table with spaces: column 2"
    check_result "$z" "e f g" "parse_table with spaces: column 3"
done < <(parse_table "a b | c d | e f g")

# Multi-row, with spaces and with blank lines
table="
a     | b   | c d e
d e f | g h | i j
"
declare -A expect=(
    [0,0]="a"
    [0,1]="b"
    [0,2]="c d e"
    [1,0]="d e f"
    [1,1]="g h"
    [1,2]="i j"
)
row=0
while read x y z;do
    check_result "$x" "${expect[$row,0]}" "parse_table multi_row[$row,0]"
    check_result "$y" "${expect[$row,1]}" "parse_table multi_row[$row,1]"
    check_result "$z" "${expect[$row,2]}" "parse_table multi_row[$row,2]"
    row=$(expr $row + 1)
done < <(parse_table "$table")

# Backslash handling. The first element should have none, the second some
while read x y;do
    check_result "$x" '[0-9]{2}'    "backslash test - no backslashes"
    check_result "$y" '[0-9]\{3\}'  "backslash test - one backslash each"
done < <(parse_table "[0-9]{2}  | [0-9]\\\{3\\\}")

# Empty strings. I wish we could convert those to real empty strings.
while read x y z; do
    check_result "$x" "''" "empty string - left-hand"
    check_result "$y" "''" "empty string - middle"
    check_result "$z" "''" "empty string - right"
done < <(parse_table "  |  |")

# Quotes
while read x y z;do
    check_result "$x" "a 'b c'"     "single quotes"
    check_result "$y" "d \"e f\" g" "double quotes"
    check_result "$z" "h"           "no quotes"

    # FIXME FIXME FIXME: this is the only way I can find to get bash-like
    # splitting of tokens. It really should be done inside parse_table
    # but I can't find any way of doing so. If you can find a way, please
    # update this test and any BATS tests that rely on quoting.
    eval set "$x"
    check_result "$1" "a"     "single quotes - token split - 1"
    check_result "$2" "b c"   "single quotes - token split - 2"
    check_result "$3" ""      "single quotes - token split - 3"

    eval set "$y"
    check_result "$1" "d"     "double quotes - token split - 1"
    check_result "$2" "e f"   "double quotes - token split - 2"
    check_result "$3" "g"     "double quotes - token split - 3"
done < <(parse_table "a 'b c' | d \"e f\" g | h")

# Split on '|' only when bracketed by spaces or at beginning/end of line
while read x y z;do
    check_result "$x" "|x"    "pipe in strings - pipe at start"
    check_result "$y" "y|y1"  "pipe in strings - pipe in middle"
    check_result "$z" "z|"    "pipe in strings - pipe at end"
done < <(parse_table "|x | y|y1 | z|")

# END   test the parse_table helper
###############################################################################
# BEGIN dprint

function dprint_test_1() {
    dprint "$*"
}

# parse_table works, might as well use it
#
#  <value of PODMAN_TEST_DEBUG> | <blank for no msg, - for msg> | <desc>
#
table="
                           |   | debug unset
dprint_test                | - | substring match
dprint_test_1              | - | exact match
dprint_test_10             |   | caller name mismatch
xxx yyy zzz                |   | multiple callers, no match
dprint_test_1 xxx yyy zzz  | - | multiple callers, match at start
xxx dprint_test_1 yyy zzz  | - | multiple callers, match in middle
xxx yyy zzz dprint_test_1  | - | multiple callers, match at end
"
while read var expect name; do
    random_string=$(random_string 20)
    PODMAN_TEST_DEBUG="$var" result=$(dprint_test_1 "$random_string" 3>&1)
    expect_full=""
    if [ -n "$expect" -a "$expect" != "''" ]; then
        expect_full="# dprint_test_1() : $random_string"
    fi
    check_result "$result" "$expect_full" "DEBUG='$var' - $name"
done < <(parse_table "$table")

# END   dprint
###############################################################################
# BEGIN remove_same_dev_warning

# Test-helper function: runs remove_same_dev_warning, compares resulting
# value of $lines and $output to expected values given on command line
function check_same_dev() {
    local testname="$1"; shift
    local -a expect_lines=("$@")
    local nl="
"

    remove_same_dev_warning

    # After processing, check the expected number of lines
    check_result "${#lines[@]}" "${#@}" "$testname: expected # of lines"

    # ...and each expected line
    local expect_output=""
    local i=0
    while [ $i -lt ${#expect_lines[@]} ]; do
        check_result "${lines[$i]}" "${expect_lines[$i]}" "$testname: line $i"
        expect_output+="${expect_lines[$i]}$nl"
        i=$(( i + 1 ))
    done

    # ...and the possibly-multi-line $output
    check_result "$output" "${expect_output%%$nl}"  "$testname: output"
}

# Simplest case: nothing removed.
declare -a lines=("a b c" "d" "e f")
check_same_dev "abc" "a b c" "d" "e f"

# Confirm that the warning message is removed from the beginning
declare -a lines=(
    "WARNING: The same type, major and minor should not be used for multiple devices."
    "a"
    "b"
    "c"
)
check_same_dev "warning is removed" a b c

# ...and from the middle (we do not expect to see this)
declare -a lines=(
    "WARNING: The same type, major and minor should not be used for multiple devices."
    "a"
    "b"
    "WARNING: The same type, major and minor should not be used for multiple devices."
    "c"
)
check_same_dev "multiple warnings removed" a b c

# Corner case: two lines of output, only one of which we care about
declare -a lines=(
    "WARNING: The same type, major and minor should not be used for multiple devices."
    "this is the only line we care about"
)
check_same_dev "one-line output" "this is the only line we care about"

# Corner case: one line of output, but we expect zero.
declare -a lines=(
    "WARNING: The same type, major and minor should not be used for multiple devices."
)
check_same_dev "zero-line output"

# END   remove_same_dev_warning
###############################################################################
# BEGIN random_free_port

# Assumes that 16700 is open
found=$(random_free_port 16700-16700)

check_result "$found" "16700" "random_free_port"

# END   random_free_port
###############################################################################
# BEGIN subnet_in_use  ...  because that's complicated

# Override ip command
function ip() {
    echo "default foo"
    echo "192.168.0.0/16"
    echo "172.17.2.3/30"
    echo "172.128.0.0/9"
}

# x.y.z | result (1 = in use, 0 = not in use - opposite of exit code)
table="
172 |   0 |   0  | 0
172 |   0 | 255  | 0
172 |   1 |   1  | 0
172 |   1 |   2  | 0
172 |   1 |   3  | 0
172 |  17 |   1  | 0
172 |  17 |   2  | 1
172 |  17 |   3  | 0
172 | 127 |   0  | 0
172 | 128 |   0  | 1
172 | 255 |   2  | 1
192 | 168 |   1  | 1
"

while read n1 n2 n3 expect; do
    subnet_in_use $n1 $n2 $n3
    actual=$?
    check_result "$((1 - $actual))" "$expect" "subnet_in_use $n1.$n2.$n3"
done < <(parse_table "$table")

unset -f ip

# END   subnet_in_use
###############################################################################
# BEGIN check_assert
#
# This is way, way more complicated than it should be. The purpose is
# to generate readable error messages should any of the tests ever fail.
#

# Args: the last one is "" (expect to pass) or non-"" (expect that as msg).
# All other args are what we feed to assert()
function check_assert() {
    local argv=("$@")
    testnum=$(expr $testnum + 1)

    # Final arg: "" to expect pass, anything else is expected error message
    local expect="${argv[-1]}"
    unset 'argv[-1]'

    # Descriptive test name. If multiline, use sed to make the rest '[...]'
    local testname="assert ${argv[*]}"
    testname="$(sed -z -e 's/[\r\n].\+/ [...]/' <<<"$testname")"

    # HERE WE GO. This is the actual test.
    actual=$(assert "${argv[@]}" 2>&1)
    status=$?

    # Now compare actual to expect.
    if [[ -z "$expect" ]]; then
        # expect: pass
        if [[ $status -eq 0 ]]; then
            # got: pass
            echo "ok $testnum $testname"
        else
            # got: fail
            echo "not ok $testnum $testname"
            echo "# expected success; got:"
            local -a actual_split
            IFS=$'\n' read -rd '' -a actual_split <<<"$actual" || true
            if [[ "${actual_split[0]}" =~ 'vvvvv' ]]; then
                unset 'actual_split[0]'
                unset 'actual_split[1]'
                unset 'actual_split[-1]'
                actual_split=("${actual_split[@]}")
            fi
            for line in "${actual_split[@]}"; do
                echo "#     $line"
            done
            rc=1
        fi
    else
        # expect: fail
        if [[ $status -eq 0 ]]; then
            # got: pass
            echo "not ok $testnum $testname"
            echo "# expected it to fail, but it passed"
            rc=1
        else
            # Expected failure, got failure. But is it the desired failure?

            # Split what we got into lines, and remove the top/bottom borders
            local -a actual_split
            IFS=$'\n' read -rd '' -a actual_split <<<"$actual" || true
            if [[ "${actual_split[0]}" =~ 'vvvvv' ]]; then
                unset 'actual_split[0]'
                unset 'actual_split[1]'
                unset 'actual_split[-1]'
                actual_split=("${actual_split[@]}")
            fi

            # Split the expect string into lines, and remove first if empty
            local -a expect_split
            IFS=$'\n' read -rd '' -a expect_split <<<"$expect" || true
            if [[ -z "${expect_split[0]}" ]]; then
                unset 'expect_split[0]'
                expect_split=("${expect_split[@]}")
            fi

            if [[ "${actual_split[*]}" = "${expect_split[*]}" ]]; then
                # Yay.
                echo "ok $testnum $testname"
            else
                # Nope. Mismatch between actual and expected output
                echo "not ok $testnum $testname"
                rc=1

                # Ugh, this is complicated. Try to produce a useful err msg.
                local n_e=${#expect_split[*]}
                local n_a=${#actual_split[*]}
                local n_max=${n_e}
                if [[ $n_max -lt $n_a ]]; then
                    n_max=${n_a}
                fi
                printf "#    %-35s | actual\n" "expect"
                printf "#    ----------------------------------- | ------\n"
                for i in $(seq 0 $((${n_max}-1))); do
                    local e="${expect_split[$i]}"
                    local a="${actual_split[$i]}"
                    local same=' '
                    local eq='='
                    if [[ "$e" != "$a" ]]; then
                        same='!'
                        eq='|'
                    fi
                    printf "#  %s %-35s %s %s\n" "$same" "$e" "$eq" "$a"
                done
            fi
        fi
    fi
}

# Positive tests
check_assert "a"    =  "a"     ""
check_assert "abc"  =~ "a"     ""
check_assert "abc"  =~ "b"     ""
check_assert "abc"  =~ "c"     ""
check_assert "abc"  =~ "a.*c"  ""
check_assert "a"   !=  "b"     ""

# Simple Failure tests
check_assert "a" = "b" "
#| expected: = b
#|   actual:   a"

# This is the one that triggered #17509
expect="abcd efg
hijk lmnop"
actual="abcd efg

hijk lmnop"
check_assert "$actual" = "$expect" "
#| expected: = abcd efg
#|         >   hijk lmnop
#|   actual:   abcd efg
#|         >   ''
#|         >   hijk lmnop"

# Undesired carriage returns
cr=$'\r'
expect="this is line 1
this is line 2"
actual="this is line 1$cr
this is line 2$cr"
check_assert "$actual" = "$expect" "
#| expected: = this is line 1
#|         >   this is line 2
#|   actual:   \$'this is line 1\r'
#|         >   \$'this is line 2\r'"

# Anchored expressions; the 2nd and 3rd are 15 and 17 characters, not 16
check_assert "0123456789abcdef"  =~ "^[0-9a-f]{16}\$" ""
check_assert "0123456789abcde"   =~ "^[0-9a-f]{16}\$" "
#| expected: =~ \^\[0-9a-f\]\{16\}\\$
#|   actual:    0123456789abcde"
check_assert "0123456789abcdeff"  =~ "^[0-9a-f]{16}\$" "
#| expected: =~ \^\[0-9a-f\]\{16\}\\$
#|   actual:    0123456789abcdeff"

# END   check_assert
###############################################################################

exit $rc
