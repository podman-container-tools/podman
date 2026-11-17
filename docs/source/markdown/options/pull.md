####> This option file is used in:
####>   podman podman-build.unit.5.md.in, podman-container.unit.5.md.in, create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Pull=policy`
<< else >>
#### **--pull**=*policy*
<< endif >>

Pull image policy. The default is **missing**.

- **always**: Always pull the image and throw an error if the pull fails.
- **missing**: Pull the image only when the image is not in the local containers storage.  Throw an error if no image is found and the pull fails.
- **never**: Never pull the image but use the one from the local containers storage.  Throw an error if no image is found.
- **newer**: Pull if the image on the registry is newer than the one in the local containers storage.  An image is considered to be newer when the digests are different.  Comparing the time stamps is prone to errors.  Pull errors are suppressed if a local image was found.
