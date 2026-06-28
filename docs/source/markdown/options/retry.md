####> This option file is used in:
####>   podman artifact pull, artifact push, build, podman-build.unit.5.md.in, podman-container.unit.5.md.in, create, farm build, podman-image.unit.5.md.in, manifest push, pull, push, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Retry=attempts`
<< else >>
#### **--retry**=*attempts*
<< endif >>

Number of times to retry pulling or pushing images between the registry and
local storage in case of failure. Default is **3**.
