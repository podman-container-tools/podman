####> This option file is used in:
####>   podman podman-container.unit.5.md.in
####> If file is edited, make sure the changes
####> are applicable to all of those.
### `AutoUpdate=registry`

Indicates whether the container will be auto-updated ([podman-auto-update(1)](podman-auto-update.1.md)). The following values are supported:

* `registry`: Requires a fully-qualified image reference (e.g., quay.io/podman/stable:latest) to be used to create the container. This enforcement is necessary to know which image to actually check and pull. If an image ID was used, Podman does not know which image to check/pull anymore.

* `local`: Tells Podman to compare the image a container is using to the image with its raw name in local storage. If an image is updated locally, Podman simply restarts the systemd unit executing the container.
