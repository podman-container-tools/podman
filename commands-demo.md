![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

# libpod - library for running OCI-based containers in Pods

## Podman Commands

| Command                                                                  | Description                                                                | Demo                                                                                        | Script                                                                              |
| :----------------------------------------------------------------------- | :------------------------------------------------------------------------- |:--------------------------------------------------------------------------------------------| :---------------------------------------------------------------------------------- |
| [podman-attach(1)](https://podman.readthedocs.io/en/latest/markdown/podman-attach.1.html)                             | Attach to a running container                                              |
| [podman-build(1)](https://podman.readthedocs.io/en/latest/markdown/podman-build.1.html)                               | Build a container image using a Containerfile                              |
| [podman-commit(1)](https://podman.readthedocs.io/en/latest/markdown/podman-commit.1.html)                             | Create new image based on the changed container                            |
| [podman-container(1)](https://podman.readthedocs.io/en/latest/managecontainers.html)                                  | Manage Containers                                                          |
| [podman-container-checkpoint(1)](https://podman.readthedocs.io/en/latest/markdown/podman-container-checkpoint.1.html) | Checkpoint one or more running containers                                  |
| [podman-container-cleanup(1)](https://podman.readthedocs.io/en/latest/markdown/podman-container-cleanup.1.html)       | Clean up the container's network and mountpoints                           |
| [podman-container-exists(1)](https://podman.readthedocs.io/en/latest/markdown/podman-container-exists.1.html)         | Check if a container exists in local storage                              |
| [podman-container-prune(1)](https://podman.readthedocs.io/en/latest/markdown/podman-container-prune.1.html)           | Remove all stopped containers from local storage                           |
| [podman-container-restore(1)](https://podman.readthedocs.io/en/latest/markdown/podman-container-restore.1.html)       | Restore one or more containers from a checkpoint                           |
| [podman-container-runlabel(1)](https://podman.readthedocs.io/en/latest/markdown/podman-container-runlabel.1.html)     | Execute a command as described by a container image label                  |
| [podman-cp(1)](https://podman.readthedocs.io/en/latest/markdown/podman-cp.1.html)                                     | Copy files/folders between a container and the local filesystem            |
| [podman-create(1)](https://podman.readthedocs.io/en/latest/markdown/podman-create.1.html)                             | Create a new container                                                     |
| [podman-diff(1)](https://podman.readthedocs.io/en/latest/markdown/podman-diff.1.html)                                 | Inspect changes on a container or image's filesystem                       |
| [podman-events(1)](https://podman.readthedocs.io/en/latest/markdown/podman-events.1.html)                             | Monitor Podman events                                                      |
| [podman-exec(1)](https://podman.readthedocs.io/en/latest/markdown/podman-exec.1.html)                                 | Execute a command in a running container                                   |
| [podman-export(1)](https://podman.readthedocs.io/en/latest/markdown/podman-export.1.html)                             | Export container's filesystem contents as a tar archive                    |
| [podman-generate(1)](https://podman.readthedocs.io/en/latest/generate.html)                                           | Generate structured output based on Podman containers and pods             |
| [podman-generate-kube(1)](https://podman.readthedocs.io/en/latest/markdown/podman-generate-kube.1.html)               | Generate Kubernetes YAML based on a pod or container                       |
| [podman-generate-systemd(1)](https://podman.readthedocs.io/en/latest/markdown/podman-generate-systemd.1.html)         | Generate systemd unit file(s) for a container or pod. Not supported for the remote client  |
| [podman-history(1)](https://podman.readthedocs.io/en/latest/markdown/podman-history.1.html)                           | Show the history of an image                                               |
| [podman-image(1)](https://podman.readthedocs.io/en/latest/image.html)                                                 | Manage Images                                                              |
| [podman-image-diff(1)](https://podman.readthedocs.io/en/latest/markdown/podman-image-diff.html)                       | Inspect changes on an image's filesystem.                                  |
| [podman-image-exists(1)](https://podman.readthedocs.io/en/latest/markdown/podman-image-exists.1.html)                 | Check if an image exists in local storage                                  |
| [podman-image-prune(1)](https://podman.readthedocs.io/en/latest/markdown/podman-image-prune.1.html)                   | Remove all unused images from the local store                              |
| [podman-image-search(1)](https://podman.readthedocs.io/en/latest/markdown/podman-search.1.html)                       | Search a registry for an image.                                            |
| [podman-image-sign(1)](https://podman.readthedocs.io/en/latest/markdown/podman-image-sign.1.html)                     | Create a signature for an image                                            |
| [podman-image-tree(1)](https://podman.readthedocs.io/en/latest/markdown/podman-image-tree.1.html)                     | Print layer hierarchy of an image in a tree format                         |
| [podman-image-trust(1)](https://podman.readthedocs.io/en/latest/markdown/podman-image-trust.1.html)                   | Manage container registry image trust policy                               |
| [podman-images(1)](https://podman.readthedocs.io/en/latest/markdown/podman-images.1.html)                             | List images in local storage                                               |  | [Here](https://github.com/containers/Demos/blob/main/podman_cli/podman_images.sh) |
| [podman-import(1)](https://podman.readthedocs.io/en/latest/markdown/podman-import.1.html)                             | Import a tarball and save it as a filesystem image                         |
| [podman-info(1)](https://podman.readthedocs.io/en/latest/markdown/podman-info.1.html)                                 | Display Podman related system information                                  |
| [podman-init(1)](https://podman.readthedocs.io/en/latest/markdown/podman-init.1.html)                                 | Initialize one or more containers                                          |
| [podman-inspect(1)](https://podman.readthedocs.io/en/latest/markdown/podman-inspect.1.html)                           | Display a container or image's configuration                               | | [Here](https://github.com/containers/Demos/blob/main/podman_cli/podman_inspect.sh) |
| [podman-kill(1)](https://podman.readthedocs.io/en/latest/markdown/podman-kill.1.html)                                 | Kill the main process in one or more running containers                    |
| [podman-load(1)](https://podman.readthedocs.io/en/latest/markdown/podman-load.1.html)                                 | Load an image from a container image archive into container storage        |
| [podman-login(1)](https://podman.readthedocs.io/en/latest/markdown/podman-login.1.html)                               | Log in to a container registry                                             |
| [podman-logout(1)](https://podman.readthedocs.io/en/latest/markdown/podman-logout.1.html)                             | Log out of a container registry                                            |
| [podman-logs(1)](https://podman.readthedocs.io/en/latest/markdown/podman-logs.1.html)                                 | Display the logs of one or more containers                                 |
| [podman-mount(1)](https://podman.readthedocs.io/en/latest/markdown/podman-mount.1.html)                               | Mount a working container's root filesystem                                |
| [podman-network(1)](https://podman.readthedocs.io/en/latest/network.html)                                             | Manage Podman networks                                                 |
| [podman-network-create(1)](https://podman.readthedocs.io/en/latest/markdown/podman-network-create.1.html)             | Create a network                                                       |
| [podman-network-connect(1)](https://podman.readthedocs.io/en/latest/markdown/podman-network-connect.1.html)           | Connect a container to a network                                       |
| [podman-network-disconnect(1)](https://podman.readthedocs.io/en/latest/markdown/podman-network-disconnect.1.html)             | Disconnect a container from a network                                  |
| [podman-network-inspect(1)](https://podman.readthedocs.io/en/latest/markdown/podman-network-inspect.1.html)           | Display the network configuration for one or more networks                 |
| [podman-network-ls(1)](https://podman.readthedocs.io/en/latest/markdown/podman-network-ls.1.html)                     | Display a summary of networks                                          |
| [podman-network-rm(1)](https://podman.readthedocs.io/en/latest/markdown/podman-network-rm.1.html)                     | Remove one or more networks                                            |
| [podman-pause(1)](https://podman.readthedocs.io/en/latest/markdown/podman-pause.1.html)                               | Pause one or more running containers                                       |  | [Here](https://github.com/containers/Demos/blob/main/podman_cli/podman_pause_unpause.sh) |
| [podman-play(1)](https://podman.readthedocs.io/en/latest/play.html)                                                   | Play a pod                                                                 |
| [podman-play-kube(1)](https://podman.readthedocs.io/en/latest/markdown/podman-play-kube.1.html)                       | Create pods and containers based on Kubernetes YAML                        |
| [podman-pod(1)](https://podman.readthedocs.io/en/latest/pod.html)                                                     | Manage pods                                                                |
| [podman-pod-create(1)](https://podman.readthedocs.io/en/latest/markdown/podman-pod-create.1.html)                     | Create a new pod                                                           |
| [podman-pod-exists(1)](https://podman.readthedocs.io/en/latest/markdown/podman-pod-exists.1.html)                     | Check if a pod exists in local storage                                     |
| [podman-pod-inspect(1)](https://podman.readthedocs.io/en/latest/markdown/podman-pod-inspect.1.html)                   | Display information describing a pod                                       |
| [podman-pod-kill(1)](https://podman.readthedocs.io/en/latest/markdown/podman-pod-kill.1.html)                         | Kill the main process of each container in one or more pods                |
| [podman-pod-pause(1)](https://podman.readthedocs.io/en/latest/markdown/podman-pause.1.html)                           | Pause one or more containers                                               |
| [podman-pod-prune(1)](https://podman.readthedocs.io/en/latest/markdown/podman-pod-prune.1.html)                       | Remove all stopped pods and their containers                               |
| [podman-pod-ps(1)](https://podman.readthedocs.io/en/latest/markdown/podman-pod-ps.1.html)                             | Print out information about pods                                           |
| [podman-pod-restart](https://podman.readthedocs.io/en/latest/markdown/podman-pod-restart.1.html)                      | Restart one or more pods                                                   |
| [podman-pod-rm(1)](https://podman.readthedocs.io/en/latest/markdown/podman-pod-rm.1.html)                             | Remove one or more stopped pods and containers                             |
| [podman-pod-start(1)](https://podman.readthedocs.io/en/latest/markdown/podman-pod-start.1.html)                       | Start one or more pods                                                     |
| [podman-pod-stats(1)](https://podman.readthedocs.io/en/latest/markdown/podman-pod-stats.1.html)                       | Display a live stream of resource usage stats for containers in one or more pods       |                                                                                             |          |
| [podman-pod-stop(1)](https://podman.readthedocs.io/en/latest/markdown/podman-pod-stop.1.html)                         | Stop one or more pods                                                      |
| [podman-pod-top(1)](https://podman.readthedocs.io/en/latest/markdown/podman-pod-top.1.html)                           | Display the running processes of containers in a pod                       |
| [podman-pod-unpause(1)](https://podman.readthedocs.io/en/latest/markdown/podman-pod-unpause.1.html)                   | Unpause one or more pods.                                                  |
| [podman-port(1)](https://podman.readthedocs.io/en/latest/markdown/podman-port.1.html)                                 | List port mappings for a container                                         |
| [podman-ps(1)](https://podman.readthedocs.io/en/latest/markdown/podman-ps.1.html)                                     | Print out information about containers                                     |
| [podman-pull(1)](https://podman.readthedocs.io/en/latest/markdown/podman-pull.1.html)                                 | Pull an image from a registry                                              |
| [podman-push(1)](https://podman.readthedocs.io/en/latest/markdown/podman-push.1.html)                                 | Push an image from local storage to elsewhere                              | [play](https://asciinema.org/a/133276)                                                      |
| [podman-restart(1)](https://podman.readthedocs.io/en/latest/markdown/podman-restart.1.html)                           | Restart one or more containers                                             | [![...](/docs/source/markdown/play.png)](https://asciinema.org/a/jiqxJAxcVXw604xdzMLTkQvHM) |
| [podman-rm(1)](https://podman.readthedocs.io/en/latest/markdown/podman-rm.1.html)                                     | Remove one or more containers                                              |
| [podman-rmi(1)](https://podman.readthedocs.io/en/latest/markdown/podman-rmi.1.html)                                   | Remove one or more locally stored images                                   |
| [podman-run(1)](https://podman.readthedocs.io/en/latest/markdown/podman-run.1.html)                                   | Run a command in a new container                                           |
| [podman-save(1)](https://podman.readthedocs.io/en/latest/markdown/podman-save.1.html)                                 | Save an image to a container archive                                       |
| [podman-search(1)](https://podman.readthedocs.io/en/latest/markdown/podman-search.1.html)                             | Search a registry for an image                                             |
| [podman-secret(1)](https://podman.readthedocs.io/en/latest/markdown/podman-secret.1.html)                             | Manage podman secrets                                                      |
| [podman-secret-create(1)](https://podman.readthedocs.io/en/latest/markdown/podman-secret-create.1.html)               | Create a new secret                                                        |
| [podman-secret-inspect(1)](https://podman.readthedocs.io/en/latest/markdown/podman-secret-inspect.1.html)             |  Display detailed information on one or more secrets                       |
| [podman--secret-ls(1)](https://podman.readthedocs.io/en/latest/markdown/podman-secret-ls.1.html)                      | List all the available secrets                                             |
| [podman-secret-rm(1)](https://podman.readthedocs.io/en/latest/markdown/podman-secret-rm.1.html)                       | Remove one or more secrets                                                 |
| [podman-start(1)](https://podman.readthedocs.io/en/latest/markdown/podman-start.1.html)                               | Start one or more containers                                               |
| [podman-stats(1)](https://podman.readthedocs.io/en/latest/markdown/podman-stats.1.html)                               | Display a live stream of one or more container's resource usage statistics |
| [podman-stop(1)](https://podman.readthedocs.io/en/latest/markdown/podman-stop.1.html)                                 | Stops one or more running containers                                       |
| [podman-system(1)](https://podman.readthedocs.io/en/latest/system.html)                                               | Manage podman                                                              |
| [podman-system-df(1)](https://podman.readthedocs.io/en/latest/markdown/podman-system-df.1.html)                       | Show podman disk usage.                                                    |
| [podman-system-events(1)](https://podman.readthedocs.io/en/latest/markdown/podman-events.1.html)                      | Display Podman related system events.                                      |
| [podman-system-info(1)](https://podman.readthedocs.io/en/latest/markdown/podman-info.1.html)                          | Display Podman related system information.                                 |
| [podman-system-migrate(1)](https://podman.readthedocs.io/en/latest/markdown/podman-system-migrate.1.html)             | Migrate existing containers to a new podman version                        |
| [podman-system-prune(1)](https://podman.readthedocs.io/en/latest/markdown/podman-system-prune.1.html)                 | Remove all unused container, image and volume data                         |
| [podman-system-renumber(1)](https://podman.readthedocs.io/en/latest/markdown/podman-system-renumber.1.html)           | Migrate lock numbers to handle a change in maximum number of locks         |
| [podman-tag(1)](https://podman.readthedocs.io/en/latest/markdown/podman-tag.1.html)                                   | Add an additional name to a local image                                    | [play](https://asciinema.org/a/133803)                                                      |
| [podman-top(1)](https://podman.readthedocs.io/en/latest/markdown/podman-top.1.html)                                   | Display the running processes of a container                               |
| [podman-unmount(1)](https://podman.readthedocs.io/en/latest/markdown/podman-unmount.1.html)                             | Unmount a working container's root filesystem                              |
| [podman-unpause(1)](https://podman.readthedocs.io/en/latest/markdown/podman-unpause.1.html)                           | Unpause one or more containers                                             | | [Here](https://github.com/containers/Demos/blob/main/podman_cli/podman_pause_unpause.sh) |
| [podman-unshare(1)](https://podman.readthedocs.io/en/latest/markdown/podman-unshare.1.html)                           | Run a command inside of a modified user namespace                          |
| [podman-version(1)](https://podman.readthedocs.io/en/latest/markdown/podman-version.1.html)                           | Display the Podman version information                                     |
| [podman-volume(1)](https://podman.readthedocs.io/en/latest/volume.html)                                               | Manage Volumes                                                             |
| [podman-volume-create(1)](https://podman.readthedocs.io/en/latest/markdown/podman-volume-create.1.html)               | Create a new volume                                                        |
| [podman-volume-inspect(1)](https://podman.readthedocs.io/en/latest/markdown/podman-volume-inspect.1.html)             | Get detailed information on one or more volumes                            |
| [podman-volume-ls(1)](https://podman.readthedocs.io/en/latest/markdown/podman-volume-ls.1.html)                       | List all the available volumes                                             |
| [podman-volume-prune(1)](https://podman.readthedocs.io/en/latest/markdown/podman-volume-prune.1.html)                 | Remove all unused volumes                                                  |
| [podman-volume-rm(1)](https://podman.readthedocs.io/en/latest/markdown/podman-volume-rm.1.html)                       | Remove one or more volumes                                                 |
| [podman-wait(1)](https://podman.readthedocs.io/en/latest/markdown/podman-wait.1.html)                                 | Wait on one or more containers to stop and print their exit codes          |
