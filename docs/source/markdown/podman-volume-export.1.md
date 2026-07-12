% podman-volume-export 1

## NAME
podman\-volume\-export - Export volume to external tar

## SYNOPSIS
**podman volume export** [*options*] *volume*

## DESCRIPTION

**podman volume export** exports the contents of a podman volume and saves it as a tarball
on the local machine. **podman volume export** writes to STDOUT by default and can be
redirected to a file using the `--output` flag.

**podman volume export [OPTIONS] VOLUME**

## OPTIONS

#### **--help**

Print usage statement

#### **--output**, **-o**=*file*

Write to a file, default is STDOUT

## EXAMPLES

Export named volume content into the specified file.
```
$ podman volume export myvol --output myvol.tar

```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-volume(1)](podman-volume.1.md)**, **[podman-volume-import(1)](podman-volume-import.1.md)**
