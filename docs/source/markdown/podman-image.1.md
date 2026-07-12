% podman-image 1

## NAME
podman\-image - Manage images

## SYNOPSIS
**podman image** *subcommand*

## DESCRIPTION
The image command allows the management of images

## COMMANDS

| Command  | Man Page                                            | Description                                                             |
| -------- | --------------------------------------------------- | ----------------------------------------------------------------------- |
| build    | [podman-build(1)](podman-build.1.md)                | Build a container using a Dockerfile.                                   |
| diff     | [podman-image-diff(1)](podman-image-diff.1.md)      | Inspect changes on an image's filesystem.                               |
| exists   | [podman-image-exists(1)](podman-image-exists.1.md)  | Check if an image exists in local storage.                              |
| history  | [podman-history(1)](podman-history.1.md)            | Show the history of an image.                                           |
| import   | [podman-import(1)](podman-import.1.md)              | Import a tarball and save it as a filesystem image.                     |
| inspect  | [podman-image-inspect(1)](podman-image-inspect.1.md)| Display an image's configuration.                                       |
| list     | [podman-images(1)](podman-images.1.md)              | List the container images on the system.(alias ls)                      |
| load     | [podman-load(1)](podman-load.1.md)                  | Load an image from the docker archive.                                  |
| mount    | [podman-image-mount(1)](podman-image-mount.1.md)    | Mount an image's root filesystem.                                       |
| prune    | [podman-image-prune(1)](podman-image-prune.1.md)    | Remove all unused images from the local store.                          |
| pull     | [podman-pull(1)](podman-pull.1.md)                  | Pull an image from a registry.                                          |
| push     | [podman-push(1)](podman-push.1.md)                  | Push an image from local storage to elsewhere.                          |
| rm       | [podman-rmi(1)](podman-rmi.1.md)                    | Remove one or more locally stored images.                               |
| save     | [podman-save(1)](podman-save.1.md)                  | Save an image to docker-archive or oci.                                 |
| scp      | [podman-image-scp(1)](podman-image-scp.1.md)        | Securely copy an image from one host to another.                        |
| search   | [podman-search(1)](podman-search.1.md)              | Search a registry for an image.                                         |
| sign     | [podman-image-sign(1)](podman-image-sign.1.md)      | Create a signature for an image.                                        |
| tag      | [podman-tag(1)](podman-tag.1.md)                    | Add an additional name to a local image.                                |
| tree     | [podman-image-tree(1)](podman-image-tree.1.md)      | Print layer hierarchy of an image in a tree format.                     |
| trust    | [podman-image-trust(1)](podman-image-trust.1.md)    | Manage container registry image trust policy.                           |
| unmount   | [podman-image-unmount(1)](podman-image-unmount.1.md)  | Unmount an image's root filesystem.                                  |
| untag    | [podman-untag(1)](podman-untag.1.md)                | Remove one or more names from a locally-stored image.                   |

## SEE ALSO
**[podman(1)](podman.1.md)**
