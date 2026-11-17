####> This option file is used in:
####>   podman network create, podman-network.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `IPv6=true`
<< else >>
#### **--ipv6**
<< endif >>

Enable IPv6 (Dual Stack) networking. If no subnets are given, it allocates an IPv4 and an IPv6 subnet.
