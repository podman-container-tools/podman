####> This option file is used in:
####>   podman network create, podman-network.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Gateway=ip`
<< else >>
#### **--gateway**=*ip*
<< endif >>

Define a gateway for the subnet. To provide a gateway address, a
*subnet* option is required. Can be specified multiple times.

<< if not is_quadlet >>
The argument order of the **--subnet**, **--gateway** and **--ip-range** options must match.
<< endif >>
