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

*IMPORTANT: With **--all-tags**, Podman does **not** walk the unqualified-search registries from
**[containers-registries.conf(5)](https://github.com/containers/image/blob/main/docs/containers-registries.conf.5.md)**
for short (unqualified) names. Unqualified names are resolved as if they lived on **docker.io**
(for example `alpine` becomes `docker.io/library/alpine`). To pull every tag from another registry,
use a fully qualified image name such as `quay.io/myorg/myimage` or `registry.example.com/ns/name`.*
