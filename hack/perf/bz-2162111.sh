#!/usr/bin/env sh
source ./helpers.bash

tmp=$(mktemp -d)

file_a=$(mktemp -p $tmp --suffix '.file_a')
file_b=$(mktemp -p $tmp --suffix '.file_b')
dd if=/dev/zero of=$file_a bs=1024 count=1024 status=none
dd if=/dev/zero of=$file_b bs=1024 count=1024 status=none

volume_name="bz-2162111"
container_name="bz-2162111"
network_name="bz-2162111"

$ENGINE_A system prune -f >> /dev/null
$ENGINE_B system prune -f >> /dev/null
$ENGINE_A network create $network_name >> /dev/null
$ENGINE_B network create $network_name >> /dev/null

container_cmd="--name $container_name \
	--stop-timeout=0 \
	--network-alias alias_a \
	--network-alias alias_b \
	--network=$network_name \
	-v /dev/log:/dev/log:rw,z \
	-v $volume_name:/var/core:rw,z \
	-v $file_a:/home/file_a:rw \
	-v $file_b:/home/file_b:rw \
	--cap-drop=ALL \
	--ulimit nofile=1024:2048 \
	$IMAGE"

# Script to clean up before each benchmark below
prepare_sh=$(mktemp -p $tmp --suffix '.prepare.sh')
cat > $prepare_sh << EOF
\$ENGINE rm -f \$(\$ENGINE ps -aq)
\$ENGINE volume prune -f
\$ENGINE volume create $volume_name
EOF
echo_bold "Prepare script: $prepare_sh"

# Script to create container below
create_sh=$(mktemp -p $tmp --suffix '.create.sh')
cat > $create_sh << EOF
\$ENGINE create $container_cmd true
EOF
echo_bold "Create script: $create_sh"

# Script to run container below
run_sh=$(mktemp -p $tmp --suffix '.run.sh')
cat > $run_sh << EOF
# Make sure to remove the container from a previous run
\$ENGINE rm -f $container_name || true
# Run the container
\$ENGINE run -d $container_cmd top > /dev/null
EOF
echo_bold "Run script: $run_sh"

echo ""
echo "----------------------------------------------------"
echo
echo_bold "Create $NUM_CONTAINERS containers"
hyperfine --warmup 10 --runs $RUNS \
    --prepare "ENGINE=$ENGINE_A sh $prepare_sh" \
    --prepare "ENGINE=$ENGINE_B sh $prepare_sh" \
    "$ENGINE_A create $container_cmd true" \
    "$ENGINE_B create $container_cmd true"

echo ""
echo "----------------------------------------------------"
echo
echo_bold "Start $NUM_CONTAINERS containers"
hyperfine --warmup 10 --runs $RUNS \
    --prepare "ENGINE=$ENGINE_A sh $prepare_sh; ENGINE=$ENGINE_A sh $create_sh" \
    --prepare "ENGINE=$ENGINE_B sh $prepare_sh; ENGINE=$ENGINE_B sh $create_sh" \
    "$ENGINE_A start $container_name" \
    "$ENGINE_B start $container_name"

echo ""
echo "----------------------------------------------------"
echo
echo_bold "Stop $NUM_CONTAINERS containers"
hyperfine --warmup 10 --runs $RUNS \
    --prepare "ENGINE=$ENGINE_A sh $run_sh" \
    --prepare "ENGINE=$ENGINE_B sh $run_sh" \
    "$ENGINE_A stop $container_name" \
    "$ENGINE_B stop $container_name"

echo ""
echo "----------------------------------------------------"
echo
echo_bold "Remove $NUM_CONTAINERS containers"
hyperfine --warmup 10 --runs $RUNS \
    --prepare "ENGINE=$ENGINE_A sh $prepare_sh; ENGINE=$ENGINE_A sh $create_sh" \
    --prepare "ENGINE=$ENGINE_B sh $prepare_sh; ENGINE=$ENGINE_B sh $create_sh" \
    "$ENGINE_A rm -f $container_name" \
    "$ENGINE_B rm -f $container_name"

# Clean up
$ENGINE_A system prune -f >> /dev/null
$ENGINE_B system prune -f >> /dev/null
