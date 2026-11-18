% podman-quadlet-exists 1

## NAME
podman\-quadlet\-exists - Check if the given quadlet exists

## SYNOPSIS
**podman quadlet exists** *quadlet*

## DESCRIPTION
**podman quadlet exists** checks if a quadlet file with the given name is
present under a Quadlet unit directory. The quadlet must be given by its
full file name, including the extension (for example
`myquadlet.container`). Podman returns an exit code of `0` when the quadlet is
found. A `1` is returned otherwise. An exit code of `125` indicates there was
another issue, such as an unsupported file extension.

## OPTIONS

#### **--help**, **-h**

Print usage statement

## EXAMPLE

Check if a quadlet called `myquadlet.container` exists (the quadlet does actually exist).
```
$ podman quadlet exists myquadlet.container
$ echo $?
0
$
```

Check if a quadlet called `other.container` exists (the quadlet does not actually exist).
```
$ podman quadlet exists other.container
$ echo $?
1
$
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-quadlet(1)](podman-quadlet.1.md)**, **[podman-systemd.unit(5)](podman-systemd.unit.5.md)**
