% podman-artifact 1

## NAME
podman\-artifact - Manage OCI artifacts

## SYNOPSIS
**podman artifact** *subcommand*

## DESCRIPTION
`podman artifact` is a set of subcommands that manage OCI artifacts.

OCI artifacts are a common way to distribute files that are associated with OCI images and
containers. Podman is capable of managing (pulling, inspecting, pushing) these artifacts
from its local "artifact store".

## SUBCOMMANDS

| Command | Man Page                                                   | Description                                                  |
|---------|------------------------------------------------------------|--------------------------------------------------------------|
| add     | [podman-artifact-add(1)](podman-artifact-add.1.md)         | Add an OCI artifact to local artifact store              |
| extract | [podman-artifact-extract(1)](podman-artifact-extract.1.md) | Extract an OCI artifact to a local path                      |
| inspect | [podman-artifact-inspect(1)](podman-artifact-inspect.1.md) | Inspect an OCI artifact                                      |
| ls      | [podman-artifact-ls(1)](podman-artifact-ls.1.md)           | List OCI artifacts in local store (alias list)                        |
| pull    | [podman-artifact-pull(1)](podman-artifact-pull.1.md)       | Pulls an artifact from a registry and stores it locally      |
| push    | [podman-artifact-push(1)](podman-artifact-push.1.md)       | Push an OCI artifact from local storage to an image registry |
| rm      | [podman-artifact-rm(1)](podman-artifact-rm.1.md)           | Remove one or more OCI artifacts from local storage          |


## SEE ALSO
**[podman(1)](podman.1.md)**

## HISTORY
Sept 2024, Originally compiled by Brent Baude <bbaude@redhat.com>
