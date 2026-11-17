####> This option file is used in:
####>   podman podman-build.unit.5.md.in, podman-container.unit.5.md.in, podman-image.unit.5.md.in, podman-kube.unit.5.md.in, podman-network.unit.5.md.in, podman-pod.unit.5.md.in, podman-volume.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `ContainersConfModule=module`
<< else >>
#### **--module**=*module*
<< endif >>

Load the specified containers.conf(5) module.

This option can be listed multiple times.
