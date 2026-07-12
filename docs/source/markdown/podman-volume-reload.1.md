% podman-volume-reload 1

## NAME
podman\-volume\-reload - Reload all volumes from volumes plugins

## SYNOPSIS
**podman volume reload**

## DESCRIPTION

**podman volume reload** checks all configured volume plugins and updates the libpod database with all available volumes.
Existing volumes are also removed from the database when they are no longer present in the plugin.

This command it is best effort and cannot guarantee a perfect state because plugins can be modified from the outside at any time.

Note: This command is not supported with podman-remote.

## EXAMPLES

Reload the volume plugins.
```
$ podman volume reload
Added:
vol6
Removed:
t3
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-volume(1)](podman-volume.1.md)**
