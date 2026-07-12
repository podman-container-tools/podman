####> This option file is used in:
####>   podman create, podman-image.unit.5.md.in, pull, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Variant=VARIANT`
<< else >>
#### **--variant**=*VARIANT*
<< endif >>

Use _VARIANT_ instead of the default architecture variant of the container image. Some images can use multiple variants of the arm architectures, such as arm/v5 and arm/v7.
