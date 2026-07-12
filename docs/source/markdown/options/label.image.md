####> This option file is used in:
####>   podman build, podman-build.unit.5.md.in, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Label=label`
<< else >>
#### **--label**=*label*
<< endif >>

Add an image *label* (e.g. label=*value*) to the image metadata. Can be used
multiple times.

Users can set a special LABEL **io.containers.capabilities=CAP1,CAP2,CAP3** in
a Containerfile that specifies the list of Linux capabilities required for the
container to run properly. This label specified in a container image tells
Podman to run the container with just these capabilities. Podman launches the
container with just the specified capabilities, as long as this list of
capabilities is a subset of the default list.

If the specified capabilities are not in the default set, Podman prints an error
message and runs the container with the default capabilities.
