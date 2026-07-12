####> This option file is used in:
####>   podman build, podman-build.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Variant=VARIANT`
<< else >>
#### **--variant**=*VARIANT*
<< endif >>

Set the architecture variant of the image to be built, and that of the base
image to be pulled, if the build uses one, to the provided value instead of
using the architecture variant of the build host.
