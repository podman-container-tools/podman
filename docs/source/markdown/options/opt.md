####> This option file is used in:
####>   podman network create, podman-network.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Options=option`
<< else >>
#### **--opt**, **-o**=*option*
<< endif >>

Set driver specific options.

All drivers accept the `mtu`, `metric`, `no_default_route` and options.

- `mtu`: Sets the Maximum Transmission Unit (MTU) and takes an integer value.
- `metric` Sets the Route Metric for the default route created in every container joined to this network. Accepts a positive integer value. Can only be used with the Netavark network backend.
- `no_default_route`: If set to `true`, Podman will not automatically add a default route to subnets. Routes can still be added
manually by creating a custom route using `--route`.

Additionally the `bridge` driver supports the following options:

- `vlan`: This option assigns a VLAN tag and enables vlan\_filtering. Defaults to none.
- `isolate`: This option isolates networks by blocking traffic between those that have this option enabled.
- `com.docker.network.bridge.name`: This option assigns the given name to the created Linux Bridge
- `com.docker.network.driver.mtu`: Sets the Maximum Transmission Unit (MTU) and takes an integer value.
- `vrf`: This option assigns a Virtual Routing and Forwarding (VRF) routing table to the bridge interface. It accepts the VRF name and defaults to none. Can only be used with the Netavark network backend.
- `mode`: This option sets the specified bridge mode on the interface. Defaults to `managed`. Supported values:
  - `managed`: Podman creates and deletes the bridge and changes sysctls of it. It adds firewall rules to masquerade outgoing traffic, as well as setup port forwarding for incoming traffic using DNAT.
  - `unmanaged`: Podman uses an existing bridge. It must exist by the time you want to start a container which uses the network. There will be no NAT or port forwarding, even if such options were passed while creating the container.

The `macvlan` and `ipvlan` driver support the following options:

- `parent`: The host device which is used for the macvlan interface. Defaults to the default route interface.
- `mode`: This option sets the specified ip/macvlan mode on the interface.
  - Supported values for `macvlan` are `bridge`, `private`, `vepa`, `passthru`. Defaults to `bridge`.
  - Supported values for `ipvlan` are `l2`, `l3`, `l3s`. Defaults to `l2`.

Additionally the `macvlan` driver supports the `bclim` option:

- `bclim`: Set the threshold for broadcast queueing. Must be a 32 bit integer. Setting this value to `-1` disables broadcast queueing altogether.
