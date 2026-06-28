% podman-export 1

## NAME
podman\-export - Export a container's filesystem contents as a tar archive

## SYNOPSIS
**podman export** [*options*] *container*

**podman container export** [*options*] *container*

## DESCRIPTION
**podman export** exports the filesystem of a container and saves it as a tarball
on the local machine. **podman export** writes to STDOUT by default and can be
redirected to a file using the `--output` flag.
The image of the container exported by **podman export** can be imported by **podman import**.
To export image(s) with parent layers, use **podman save**.
Note: `:` is a restricted character and cannot be part of the file name.

**podman export** operates on a *container's* filesystem: it flattens the filesystem into a single-layer tarball with no image layers, history or tags. This is different from **podman save**, which archives an *image* and preserves its layers, history and tags. To archive an image instead, see **podman-save(1)**.

**podman [GLOBAL OPTIONS]**

**podman export [GLOBAL OPTIONS]**

**podman export [OPTIONS] CONTAINER**

## OPTIONS

#### **--help**, **-h**

Print usage statement

#### **--output**, **-o**

Write to a file, default is STDOUT

## EXAMPLES

Export container into specified tar ball:
```
$ podman export -o redis-container.tar 883504668ec465463bc0fe7e63d53154ac3b696ea8d7b233748918664ea90e57
```

Export container to stdout:
```
$ podman export 883504668ec465463bc0fe7e63d53154ac3b696ea8d7b233748918664ea90e57 > redis-container.tar
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-import(1)](podman-import.1.md)**

## HISTORY
August 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
