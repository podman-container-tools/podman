####> This option file is used in:
####>   podman build, podman-build.unit.5.md.in, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Annotation=annotation=value [annotation=value ...]`
<< else >>
#### **--annotation**=*annotation=value*
<< endif >>

Add an image *annotation* (e.g. annotation=*value*) to the image metadata. Can
be used multiple times.

Note: this information is not present in Docker image formats, so it is
discarded when writing images in Docker formats.
