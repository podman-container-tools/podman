####> This option file is used in:
####>   podman network create, podman-network.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `DisableDNS=true`
<< else >>
#### **--disable-dns**
<< endif >>

Disables the DNS plugin for this network which if enabled, can perform container to container name
resolution. It is only supported with the `bridge` driver, for other drivers it is always disabled.
