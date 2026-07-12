####> This option file is used in:
####>   podman network create, podman-network.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Internal=true`
<< else >>
#### **--internal**
<< endif >>

Restrict external access of this network when using a `bridge` network. Note when using the CNI backend
DNS will be automatically disabled, see **--disable-dns**.

When using the `macvlan` or `ipvlan` driver with this option, no default route will be added to the container.
Because it bypasses the host network stack, no additional restrictions can be set by Podman, and if a
privileged container is run, the container can set a default route itself. If this is a concern, then the
container connections should be blocked on the actual network gateway.

Using the `bridge` driver with this option has the following effects:
 - Global IP forwarding sysctls will not be changed in the host network namespace.
 - IP forwarding is disabled on the bridge interface instead of setting up a firewall.
 - No default route will be added to the container.

In all cases, aardvark-dns will only resolve container names with this option enabled.
Other queries will be answered with `NXDOMAIN`.
