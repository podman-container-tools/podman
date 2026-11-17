####> This option file is used in:
####>   podman build, podman-build.unit.5.md.in, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `ForceRM=`
<< else >>
#### **--force-rm**
<< endif >>

Always remove intermediate containers after a build, even if the build fails (default true).
