####> This option file is used in:
####>   podman network create, podman-network.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `InterfaceName=name`
<< else >>
#### **--interface-name**=*name*
<< endif >>

This option maps the *network_interface* option in the network config, see **podman network inspect**.
Depending on the driver, this can have different effects; for `bridge`, it uses the bridge interface name.
For `macvlan` and `ipvlan`, it is the parent device on the host. It is the same as `--opt parent=...`.
