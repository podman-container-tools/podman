#!/usr/bin/env sh
source ./helpers.bash

setup
echo_bold "Remove $RUNS containers in a row"
hyperfine --warmup 10 --runs $RUNS \
    --prepare "$ENGINE_A create --name=123 $IMAGE" \
    --prepare "$ENGINE_B create --name=123 $IMAGE" \
    "$ENGINE_A rm 123" \
    "$ENGINE_B rm 123"

# Clean up
$ENGINE_A system prune -f >> /dev/null
$ENGINE_B system prune -f >> /dev/null
