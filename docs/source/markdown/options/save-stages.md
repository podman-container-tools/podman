####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--save-stages**

Preserve intermediate stage images instead of removing them after the build completes (Default is `false`). By default, Buildah removes intermediate stage images to save space.
This option keeps those images, which can be useful for debugging multi-stage builds or for reusing intermediate stages in subsequent builds.

`--save-stages` can be used with `--layers` and subsequent builds with `--layers` can use the preserved intermediate layers as cache.

When combined with `--stage-labels`, all stage images (including the final image) will include metadata labels for easier identification and management.
