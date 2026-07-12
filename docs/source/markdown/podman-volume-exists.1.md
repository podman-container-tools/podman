% podman-volume-exists 1

## NAME
podman\-volume\-exists - Check if the given volume exists

## SYNOPSIS
**podman volume exists** *volume*

## DESCRIPTION
**podman volume exists** checks if a volume exists. Podman returns an exit code
of `0` when the volume is found. A `1` is returned otherwise. An exit code of
`125` indicates there was another issue.


## OPTIONS

#### **--help**, **-h**

Print usage statement

## EXAMPLE

Check if a volume called `myvol` exists (the volume does actually exist).
```
$ podman volume exists myvol
$ echo $?
0
$
```

Check if a volume called `mysql` exists (the volume does not actually exist).
```
$ podman volume exists mysql
$ echo $?
1
$
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-volume(1)](podman-volume.1.md)**

## HISTORY
January 2021, Originally compiled by Paul Holzinger `<paul.holzinger@web.de>`
