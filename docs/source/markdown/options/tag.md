####> This option file is used in:
####>   podman build, podman-build.unit.5.md.in, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `ImageTag=imageName`
<< else >>
#### **--tag**, **-t**=*imageName*
<< endif >>

Specifies the name which is assigned to the resulting image if the build process completes successfully.
If _imageName_ does not include a registry name, the registry name *localhost* is prepended to the image name.
