% podman-container 1

## NAME
podman\-container - Manage containers

## SYNOPSIS
**podman container** *subcommand*

## DESCRIPTION
The container command allows management of containers

## COMMANDS

| Command    | Man Page                                            | Description                                                                  |
| ---------  | --------------------------------------------------- | ---------------------------------------------------------------------------- |
| attach     | [podman-attach(1)](podman-attach.1.md)              | Attach to a running container.                                               |
| checkpoint | [podman-container-checkpoint(1)](podman-container-checkpoint.1.md)  | Checkpoint one or more running containers.                   |
| cleanup    | [podman-container-cleanup(1)](podman-container-cleanup.1.md)    | Clean up the container's network and mountpoints.                |
| clone      | [podman-container-clone(1)](podman-container-clone.1.md)      |  Create a copy of an existing container.                           |
| commit     | [podman-commit(1)](podman-commit.1.md)              | Create new image based on the changed container.                             |
| cp         | [podman-cp(1)](podman-cp.1.md)                      | Copy files/folders between a container and the local filesystem.             |
| create     | [podman-create(1)](podman-create.1.md)              | Create a new container.                                                      |
| diff       | [podman-container-diff(1)](podman-container-diff.1.md)        |  Inspect changes on a container's filesystem |
| exec       | [podman-exec(1)](podman-exec.1.md)                  | Execute a command in a running container.                                    |
| exists     | [podman-container-exists(1)](podman-container-exists.1.md)  | Check if a container exists in local storage                         |
| export     | [podman-export(1)](podman-export.1.md)              | Export a container's filesystem contents as a tar archive.                   |
| init       | [podman-init(1)](podman-init.1.md)                  | Initialize a container                                                       |
| inspect    | [podman-container-inspect(1)](podman-container-inspect.1.md)| Display a container's configuration.                                 |
| kill       | [podman-kill(1)](podman-kill.1.md)                  | Kill the main process in one or more containers.                             |
| list       | [podman-ps(1)](podman-ps.1.md)                      | List the containers on the system.(alias ls)                                 |
| logs       | [podman-logs(1)](podman-logs.1.md)                  | Display the logs of a container.                                             |
| mount      | [podman-mount(1)](podman-mount.1.md)                | Mount a working container's root filesystem.                                 |
| pause      | [podman-pause(1)](podman-pause.1.md)                | Pause one or more containers.                                                |
| port       | [podman-port(1)](podman-port.1.md)                  | List port mappings for the container.                                        |
| prune      | [podman-container-prune(1)](podman-container-prune.1.md)| Remove all stopped containers from local storage.                        |
| ps         | [podman-ps(1)](podman-ps.1.md)                      | Print out information about containers.                                      |
| rename     | [podman-rename(1)](podman-rename.1.md)              | Rename an existing container.                                                |
| restart    | [podman-restart(1)](podman-restart.1.md)            | Restart one or more containers.                                              |
| restore    | [podman-container-restore(1)](podman-container-restore.1.md)  | Restore one or more containers from a checkpoint.                  |
| rm         | [podman-rm(1)](podman-rm.1.md)                      | Remove one or more containers.                                               |
| run        | [podman-run(1)](podman-run.1.md)                    | Run a command in a container.                                                |
| runlabel   | [podman-container-runlabel(1)](podman-container-runlabel.1.md)  | Execute a command as described by a container-image label.       |
| start      | [podman-start(1)](podman-start.1.md)                | Start one or more containers.                                                |
| stats      | [podman-stats(1)](podman-stats.1.md)                | Display a live stream of one or more container's resource usage statistics.  |
| stop       | [podman-stop(1)](podman-stop.1.md)                  | Stop one or more running containers.                                         |
| top        | [podman-top(1)](podman-top.1.md)                    | Display the running processes of a container.                                |
| unmount    | [podman-unmount(1)](podman-unmount.1.md)            | Unmount a working container's root filesystem.(Alias unmount)                |
| unpause    | [podman-unpause(1)](podman-unpause.1.md)            | Unpause one or more containers.                                              |
| update     | [podman-update(1)](podman-update.1.md)              | Update the cgroup configuration of a given container.                        |
| wait       | [podman-wait(1)](podman-wait.1.md)                  | Wait on one or more containers to stop and print their exit codes.           |

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-exec(1)](podman-exec.1.md)**, **[podman-run(1)](podman-run.1.md)**
