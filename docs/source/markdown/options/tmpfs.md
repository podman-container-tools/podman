####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `Tmpfs=`
<< else >>
#### **--tmpfs**=*fs*
<< endif >>

Create a tmpfs mount.

Mount a temporary filesystem (**tmpfs**) mount into a container, for example:

```
$ podman <<subcommand>> -d --tmpfs /tmp:rw,size=787448k,mode=1777 my_image
```

This command mounts a **tmpfs** at _/tmp_ within the container. The supported mount
options are the same as the Linux default mount flags. If no options are specified,
the system uses the following options:
**rw,noexec,nosuid,nodev**.

By default, Podman enables **tmpcopyup** on tmpfs mounts, which copies the contents
of the underlying image directory into the tmpfs before mounting it. This also
applies when the tmpfs destination is inside a volume or bind mount: files from
the parent mount are copied into the tmpfs, so the parent content remains visible.
To mount an empty tmpfs that shadows a parent mount's subtree, use the
**notmpcopyup** option:

```
$ podman <<subcommand>> --volume myvolume:/data --tmpfs /data/sub:notmpcopyup my_image
```
