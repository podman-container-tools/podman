####> This option file is used in:
####>   podman network create, podman-network.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `IPAMDriver=driver`
<< else >>
#### **--ipam-driver**=*driver*
<< endif >>

Set the ipam driver (IP Address Management Driver) for the network. When unset, Podman chooses an
ipam driver automatically based on the network driver.

Valid values are:

 - `dhcp`: IP addresses are assigned from a DHCP server on the network. When using the netavark backend
  the `netavark-dhcp-proxy.socket` must be enabled in order to start the DHCP proxy when a container is
  started, for CNI use the `cni-dhcp.socket` unit instead.
 - `host-local`: IP addresses are assigned locally.
 - `none`: No ip addresses are assigned to the interfaces.

View the driver in the **podman network inspect** output under the `ipam_options` field.
