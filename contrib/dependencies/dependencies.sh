#!/usr/bin/env bash

set -e -o pipefail

GODA=${GODA:-goda}


# default to podman
command=podman
declare -a args=()
declare -a build_tags=()

# Parse Arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
    -c | --command)
        if [[ "$2" == "podman-remote" ]]; then
            command=podman
            build_tags+=("remote")
        else
            command="$2"
        fi
        shift
        ;;
    -t | --tags)
        build_tags+=("$2")
        shift
        ;;
    *) args+=("$1") ;;
    esac
    shift
done

function create_goda_expr() {
    local expr="./cmd/$command:all"
    for tag in ${build_tags[*]}; do
        expr="$tag=1($expr)"
    done
    echo $expr
}

function run_list() {
    expr=$(create_goda_expr)
    $GODA list -std -h - "$expr"
}

function run_tree() {
    expr=$(create_goda_expr)
    $GODA tree -std "$expr"
}

function run_why() {
    expr=$(create_goda_expr)
    $GODA list -std -h - "incoming($expr, $1)"
}

function run_diff() {
    expr=$(create_goda_expr)
    cb=$(git branch --show-current)
    tmpdir=$(mktemp -d)
    git checkout $1
    $GODA list -std -h - "$expr" | sort >$tmpdir/one
    git checkout ${2:-$cb}
    $GODA list -std -h - "$expr" | sort  >$tmpdir/two
    diff --color $tmpdir/one $tmpdir/two || true
    rm -rf "$tmpdir"
    git checkout "$cb"
}

function run_cut() {
    expr=$(create_goda_expr)
    $GODA cut -std $expr
}

function run_weight() {
    $GODA weight bin/$command
}

function run_weight-diff() {
    cb=$(git branch --show-current)
    local file="bin/$command"
    git checkout $1
    make $file
    mv $file $file.1
    git checkout ${2:-$cb}
    make $file
    $GODA weight-diff -miss $file.1 $file
    rm $file.1
    git checkout "$cb"
}

run_${args[0]} "${args[@]:1}"
