# -*- bash -*-

ctr_name="two_networks-con1-1"
podman container inspect "$ctr_name" --format '{{len .NetworkSettings.Networks}}'
is "$output" "2" "$testname : Container is connected to both networks"
podman container inspect "$ctr_name" --format '{{.NetworkSettings.Networks}}'
like "$output" "two_networks_net1" "$testname : First network name exists"
like "$output" "two_networks_net2" "$testname : Second network name exists"
