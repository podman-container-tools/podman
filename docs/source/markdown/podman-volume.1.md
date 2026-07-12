% podman-volume 1

## NAME
podman\-volume - Simple management tool for volumes

## SYNOPSIS
**podman volume** *subcommand*

## DESCRIPTION
podman volume is a set of subcommands that manage volumes.

## SUBCOMMANDS

| Command | Man Page                                               | Description                                                                    |
| ------- | ------------------------------------------------------ | ------------------------------------------------------------------------------ |
| create  | [podman-volume-create(1)](podman-volume-create.1.md)   | Create a new volume.                                                           |
| exists  | [podman-volume-exists(1)](podman-volume-exists.1.md)   | Check if the given volume exists.                                              |
| export  | [podman-volume-export(1)](podman-volume-export.1.md)   | Export volume to external tar.                                                 |
| import  | [podman-volume-import(1)](podman-volume-import.1.md)   | Import tarball contents into an existing podman volume.                        |
| inspect | [podman-volume-inspect(1)](podman-volume-inspect.1.md) | Get detailed information on one or more volumes.                               |
| ls      | [podman-volume-ls(1)](podman-volume-ls.1.md)           | List all the available volumes.                                                |
| mount   | [podman-volume-mount(1)](podman-volume-mount.1.md)     | Mount a volume filesystem.                                                     |
| prune   | [podman-volume-prune(1)](podman-volume-prune.1.md)     | Remove unused volumes.                                                         |
| reload  | [podman-volume-reload(1)](podman-volume-reload.1.md)   | Reload all volumes from volumes plugins.                                       |
| rename  | [podman-volume-rename(1)](podman-volume-rename.1.md)   | Rename a volume.                                                               |
| rm      | [podman-volume-rm(1)](podman-volume-rm.1.md)           | Remove one or more volumes.                                                    |
| unmount | [podman-volume-unmount(1)](podman-volume-unmount.1.md) | Unmount a volume.                                                     |

## SEE ALSO
**[podman(1)](podman.1.md)**

## HISTORY
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
