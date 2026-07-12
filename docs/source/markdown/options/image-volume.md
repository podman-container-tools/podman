####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `ImageVolume=mode`
<< else >>
#### **--image-volume**=**bind** | *tmpfs* | *ignore*
<< endif >>

Tells Podman how to handle the builtin image volumes. Default is **bind**.

- **bind**: An anonymous named volume is created and mounted into the container.
- **tmpfs**: The volume is mounted onto the container as a tmpfs, which allows the users to create
content that disappears when the container is stopped.
- **ignore**: All volumes are just ignored and no action is taken.
