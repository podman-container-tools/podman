####> This option file is used in:
####>   podman create, podman-image.unit.5.md.in, pull, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `OS=os`
<< else >>
#### **--os**=*OS*
<< endif >>

Override the OS, defaults to the host, of the image to be pulled. For example, `windows`.
Unless overridden, subsequent lookups of the same image in the local storage matches this OS, regardless of the host.
