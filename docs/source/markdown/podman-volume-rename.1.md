% podman-volume-rename 1

## NAME
podman\-volume\-rename - Rename a volume

## SYNOPSIS
**podman volume rename** *volume* *new_name*

## DESCRIPTION
Renames an existing volume. The following restrictions apply:

- The volume must not be in use by any containers (running or stopped).
- The volume must not be currently mounted (via **podman volume mount**).
- Only volumes using the **local** driver can be renamed; volumes backed by
  a volume plugin or the **image** driver cannot be renamed.

Renaming an anonymous volume converts it to a named volume.

## OPTIONS

None.

## EXAMPLES

Rename volume `mydata` to `data_backup`:
```
$ podman volume rename mydata data_backup
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-volume(1)](podman-volume.1.md)**, **[podman-volume-inspect(1)](podman-volume-inspect.1.md)**

## HISTORY
June 2026, Originally compiled by Podman Developers
