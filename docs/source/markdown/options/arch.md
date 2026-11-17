####> This option file is used in:
####>   podman podman-build.unit.5.md.in, create, podman-image.unit.5.md.in, pull, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Arch=ARCH`
<< else >>
#### **--arch**=*ARCH*
<< endif >>

Override the architecture, defaults to the host, of the image to be pulled. For example, `arm`.
Unless overridden, subsequent lookups of the same image in the local storage matches this architecture, regardless of the host.
