####> This option file is used in:
####>   podman network create, podman-network.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `IPRange=ip`
<< else >>
#### **--ip-range**=*range*
<< endif >>

Allocate container IP from a range. The range must be either a complete subnet in CIDR notation or be in
the `<startIP>-<endIP>` syntax which allows for a more flexible range compared to the CIDR subnet.
The *ip-range* option must be used with a *subnet* option. Can be specified multiple times.

<< if not is_quadlet >>
The argument order of the **--subnet**, **--gateway** and **--ip-range** options must match.
<< endif >>
