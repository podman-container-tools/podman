####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--config**=*source[,option=value...]*

Add a configuration asset to the container. The config must already exist and
can be created with **podman config create**. This option can be specified more
than once.

The config is mounted as a read-only file. By default, it is mounted at
`/<config-name>` inside the container.

The following options are supported:

- `source`: Name or ID of the config. Required when using the key-value form.
- `target`: Absolute path of the file inside the container.
- `uid`: UID that owns the file. Defaults to 0.
- `gid`: GID that owns the file. Defaults to 0.
- `mode`: File permissions in octal. Defaults to `0444`.
- `type=mount`: Mount the config as a file. This is the only supported type.

For example, mount a config at `/etc/app/config.yaml`:

```
podman run --config source=app-config,target=/etc/app/config.yaml IMAGE
```
