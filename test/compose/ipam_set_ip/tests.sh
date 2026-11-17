# -*- bash -*-

ctr_name="ipam_set_ip-test-1"
podman container inspect "$ctr_name" --format '{{ .NetworkSettings.Networks.ipam_set_ip_net1.IPAddress }}'
is "$output" "10.123.0.253" "$testname : ip address is set"
podman container inspect "$ctr_name" --format '{{ .NetworkSettings.Networks.ipam_set_ip_net1.MacAddress }}'
is "$output" "32:b5:b2:55:48:72" "$testname : mac address is set"
