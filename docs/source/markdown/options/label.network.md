####> This option file is used in:
####>   podman network create, podman-network.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Label=key=value [key=value ...]`
<< else >>
#### **--label**=*key=value*
<< endif >>

Set one or more OCI labels on the network.
