####> This option file is used in:
####>   podman network create, podman-network.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Subnet=subnet`
<< else >>
#### **--subnet**=*subnet*
<< endif >>

The subnet in CIDR notation. Can be specified multiple times to allocate more than one subnet for this network.
<< if not is_quadlet >>
The argument order of the **--subnet**, **--gateway** and **--ip-range** options must match.
<< endif >>
This is useful to set a static IPv4 and IPv6 subnet.
