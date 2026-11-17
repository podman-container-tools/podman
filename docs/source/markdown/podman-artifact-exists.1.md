% podman-artifact-exists 1

## NAME
podman\-artifact\-exists - Check if an OCI artifact exists in local storage

## SYNOPSIS
**podman artifact exists** *artifact*

## DESCRIPTION
**podman artifact exists** checks if an artifact exists in local storage. The **name**
or **digest** (full or partial) of the artifact may be used as input. Podman returns an
exit code of `0` when the artifact is found. A `1` is returned otherwise. An exit code
of `125` indicates there was another issue.

## OPTIONS

#### **--help**, **-h**

Print usage statement.

## EXAMPLES

Check if an artifact called `quay.io/myimage/myartifact:latest` exists in local storage (the artifact does actually exist):
```shell
$ podman artifact exists quay.io/myimage/myartifact:latest
$ echo $?
0
```

Check if an artifact with digest `c4dfb1609ee2` exists in local storage (the artifact does not actually exist):
```shell
$ podman artifact exists c4dfb1609ee2
$ echo $?
1
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-artifact(1)](podman-artifact.1.md)**
