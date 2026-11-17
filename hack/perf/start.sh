#!/usr/bin/env sh
source ./helpers.bash

setup
echo_bold "Start $RUNS containers in a row"
hyperfine --warmup 10 --runs $RUNS \
    --prepare "$ENGINE_A rm -f 123 || true; $ENGINE_A create --name=123 $IMAGE true" \
    --prepare "$ENGINE_B rm -f 123 || true; $ENGINE_B create --name=123 $IMAGE true" \
    "$ENGINE_A start 123" \
    "$ENGINE_B start 123"

# Clean up
$ENGINE_A system prune -f >> /dev/null
$ENGINE_B system prune -f >> /dev/null
