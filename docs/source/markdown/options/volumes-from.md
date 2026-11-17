####> This option file is used in:
####>   podman create, pod clone, pod create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--volumes-from**=*CONTAINER[:OPTIONS]*

Mount volumes from the specified container(s). Used to share volumes between
containers<<| and pods>>. The *options* is a comma-separated list with the following available elements:

* **rw**|**ro**
* **z**

Mounts already mounted volumes from a source container onto another
<<container|pod>>. _CONTAINER_ may be a name or ID.
To share a volume, use the --volumes-from option when running
the target container. Volumes can be shared even if the source container
is not running.

By default, Podman mounts the volumes in the same mode (read-write or
read-only) as it is mounted in the source container.
This can be changed by adding a `ro` or `rw` _option_.

Labeling systems like SELinux require that proper labels are placed on volume
content mounted into a <<container|pod>>. Without a label, the security system might
prevent the processes running inside the container from using the content. By
default, Podman does not change the labels set by the OS.

To change a label in the <<container|pod>> context, add `z` to the volume mount.
This suffix tells Podman to relabel file objects on the shared volumes. The `z`
option tells Podman that two entities share the volume content. As a result,
Podman labels the content with a shared content label. Shared volume labels allow
all containers to read/write content.

If the location of the volume from the source container overlaps with
data residing on a target <<container|pod>>, then the volume hides
that data on the target.
