#!/usr/bin/env sh
source ./helpers.bash

setup
echo_bold "Stop $RUNS containers in a row"
hyperfine --warmup 10 --runs $RUNS \
    --prepare "$ENGINE_A rm -f 123 || true; $ENGINE_A run -d --name=123 $IMAGE top" \
    --prepare "$ENGINE_B rm -f 123 || true; $ENGINE_B run -d --name=123 $IMAGE top" \
    "$ENGINE_A stop 123" \
    "$ENGINE_B stop 123"

# Clean up
$ENGINE_A system prune -f >> /dev/null
$ENGINE_B system prune -f >> /dev/null
