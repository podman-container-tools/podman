% podman-manifest-exists 1

## NAME
podman\-manifest\-exists - Check if the given manifest list exists in local storage

## SYNOPSIS
**podman manifest exists** *manifest*

## DESCRIPTION
**podman manifest exists** checks if a manifest list exists on local storage.
Podman returns an exit code of `0` when the manifest is found. A `1` is
returned otherwise.
An exit code of `125` indicates there was another issue.


## OPTIONS

#### **--help**, **-h**

Print usage statement.

## EXAMPLE

Check if a manifest list called `list1` exists (the manifest list does actually exist):
```
$ podman manifest exists list1
$ echo $?
0
```

Check if a manifest called `mylist` exists (the manifest list does not actually exist):
```
$ podman manifest exists mylist
$ echo $?
1
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-manifest(1)](podman-manifest.1.md)**

## HISTORY
January 2021, Originally compiled by Paul Holzinger `<paul.holzinger@web.de>`
