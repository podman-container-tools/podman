# -*- bash -*-

podman network inspect --format='{{ range . }} {{ .Options.mtu }} {{ end }}' update_network_mtu_default
like "$output" "9000" "$testname : network mtu is set"

podman network inspect --format='{{ range . }} {{ .NetworkInterface }} {{ end }}' update_network_mtu_default
like "$output" "docker0" "$testname: network interface is set"

podman network inspect --format='{{ range . }} {{ .Options.mode }} {{ end }}' update_network_mtu_macvlan_net
like "$output" "bridge" "$testname : network mode is set"
