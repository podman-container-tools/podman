####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, kube play, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Annotation=key=value`
<< else >>
#### **--annotation**=*key=value*
<< endif >>

Add an annotation to the container<<| or pod>>. This option can be set multiple times.
