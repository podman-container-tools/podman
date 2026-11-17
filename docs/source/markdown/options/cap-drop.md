####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `DropCapability=capability`
<< else >>
#### **--cap-drop**=*capability*
<< endif >>

Drop these capabilities from the default podman capability set, or `all` to drop all capabilities.

This is a space separated list of capabilities.
