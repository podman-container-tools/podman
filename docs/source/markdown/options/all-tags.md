####> This option file is used in:
####>   podman podman-image.unit.5.md.in, pull
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `AllTags=true`
<< else >>
#### **--all-tags**, **-a**
<< endif >>

All tagged images in the repository are pulled.

*IMPORTANT: When using the all-tags flag, Podman does not iterate over the search registries in the **[containers-registries.conf(5)](https://github.com/containers/image/blob/main/docs/containers-registries.conf.5.md)** but always uses docker.io for unqualified image names.*
