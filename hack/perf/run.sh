#!/usr/bin/env sh
source ./helpers.bash

setup
echo_bold "Run $RUNS containers in a row"
hyperfine --warmup 10 --runs $RUNS \
    --prepare "$ENGINE_A rm -f 123 || true" \
    --prepare "$ENGINE_B rm -f 123 || true" \
    "$ENGINE_A run --name=123 $IMAGE true" \
    "$ENGINE_B run --name=123 $IMAGE true"

setup
echo_bold "Run and remove $RUNS containers in a row"
hyperfine --warmup 10 --runs $RUNS \
    --prepare "$ENGINE_A rm -f 123 || true" \
    --prepare "$ENGINE_B rm -f 123 || true" \
    "$ENGINE_A run --rm --name=123 $IMAGE true" \
    "$ENGINE_B run --rm --name=123 $IMAGE true"

# Clean up
$ENGINE_A system prune -f >> /dev/null
$ENGINE_B system prune -f >> /dev/null
