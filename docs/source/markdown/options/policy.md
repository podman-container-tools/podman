####> This option file is used in:
####>   podman podman-image.unit.5.md.in, pull
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Policy=always`
<< else >>
#### **--policy**
<< endif >>

Pull image policy. The default is **always**.

- `always`: Always pull the image and throw an error if the pull fails.
- `missing`: Only pull the image if it could not be found in the local containers storage. Throw an error if no image could be found and the pull fails.
- `never`: Never pull the image; only use the local version. Throw an error if the image is not present locally.
- `newer`: Pull if the image on the registry is newer than the one in the local containers storage. An image is considered to be newer when the registry manifest digest differs from the local image. Comparing timestamps is prone to errors across registries and local builds. If the registry is unreachable and a local image exists, pull errors are suppressed.
