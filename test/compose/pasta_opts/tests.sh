# -*- bash -*-

ctr_name="pasta_opts-alpine-1"

podman exec "$ctr_name" ip link
like "$output" "mtu 1280" "$testname : mtu is set to 1280"
