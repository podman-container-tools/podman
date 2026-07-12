% podman-volume-rm 1

## NAME
podman\-volume\-rm - Remove one or more volumes

## SYNOPSIS
**podman volume rm** [*options*] *volume* [...]

## DESCRIPTION

Removes one or more volumes. Only volumes that are not being used are removed.
If a volume is being used by a container, an error is returned unless the **--force**
flag is being used. To remove all volumes, use the **--all** flag.
Volumes can be removed individually by providing their full name or a unique partial name.

## OPTIONS

#### **--all**, **-a**

Remove all volumes.

#### **--force**, **-f**

Remove a volume by force.
If it is being used by containers, the containers are removed first.

#### **--help**

Print usage statement

#### **--time**, **-t**=*seconds*

Seconds to wait before forcibly stopping running containers that are using the specified volume. The --force option must be specified to use the --time option. Use -1 for infinite wait.

## EXAMPLES

Remove multiple specified volumes.
```
$ podman volume rm myvol1 myvol2
```

Remove all volumes.
```
$ podman volume rm --all
```

Remove the specified volume even if it is in use. Note, this removes all containers using the volume.
```
$ podman volume rm --force myvol
```

## Exit Status
  **0**   All specified volumes removed

  **1**   One of the specified volumes did not exist, and no other failures

  **2**   One of the specified volumes is being used by a container

  **125** The command fails for any other reason

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-volume(1)](podman-volume.1.md)**

## HISTORY
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
