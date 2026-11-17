####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--stage-labels**

Add metadata labels to all intermediate stage images of the multistage build,
including the final image (Default is `false`).
This option requires `--save-stages` to be enabled.

When enabled, all intermediate stage images and final image will be labeled with:
  - `io.buildah.stage.name`: The stage alias (from `FROM ... AS alias`), or stage position
    if no alias is specified
  - `io.buildah.stage.base`: The base image used by this stage (pullspec or image ID
    when stage uses another stage as base)

These labels make it easier to identify, query, and manage images from multi-stage builds.
