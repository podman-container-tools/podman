% podman-load 1

## NAME
podman\-load - Load image(s) from tar archives, directories, or URLs into container storage

## SYNOPSIS
**podman load** [*options*]

**podman image load** [*options*]

## DESCRIPTION
**podman load** loads a saved image(s) into local container storage from tar archives, directories, or URLs pointing to an tar archive. By default, **podman load** reads from stdin, to load from any of the other input sources use the `--input` option.

**podman load** operates on *images*: it restores an archive created by **podman save** as the same image, preserving its layers, history and tags. This is different from **podman import**, which builds a *new* image from a root-filesystem tarball (such as one produced by **podman export**) with no image history. To import a container's filesystem instead, see **podman-import(1)**.

The local client further supports loading an **oci-dir** or a **docker-dir** as created with **podman save** (1).

The **quiet** option suppresses the progress output when set.

Note: `:` is a restricted character and cannot be part of the file name.

## OPTIONS

#### **--help**, **-h**

Print usage statement

#### **--input**, **-i**=*input*

Load the specified input file or URL instead of reading from stdin. The input can be one of:

- A local tar archive (uncompressed or compressed)

- A local OCI or Docker directory layout

- A URL to a tar archive (not available on macOS and Windows)

NOTE: Use the environment variable `TMPDIR` to change the temporary storage location of container images. Podman defaults to use `/var/tmp`.

#### **--quiet**, **-q**

Suppress the progress output

## EXAMPLES

Create an image from a compressed tar file, without showing progress.
```
$ podman load --quiet -i fedora.tar.gz
```

Create an image from the archive.tar file pulled from a URL, without showing progress.
```
$ podman load -q -i https://server.com/archive.tar
```

Create an image from a directory created with either podman-save `--format oci-dir` or `--format docker-dir`
```
$ podman load -i fedora/
```

Create an image from stdin using bash redirection from a tar file.
```
$ podman load < fedora.tar
Getting image source signatures
Copying blob sha256:5bef08742407efd622d243692b79ba0055383bbce12900324f75e56f589aedb0
 0 B / 4.03 MB [---------------------------------------------------------------]
Copying config sha256:7328f6f8b41890597575cbaadc884e7386ae0acc53b747401ebce5cf0d624560
 0 B / 1.48 KB [---------------------------------------------------------------]
Writing manifest to image destination
Storing signatures
Loaded image:  registry.fedoraproject.org/fedora:latest
```

Create an image from stdin using a pipe.
```
$ cat fedora.tar | podman load
Getting image source signatures
Copying blob sha256:5bef08742407efd622d243692b79ba0055383bbce12900324f75e56f589aedb0
 0 B / 4.03 MB [---------------------------------------------------------------]
Copying config sha256:7328f6f8b41890597575cbaadc884e7386ae0acc53b747401ebce5cf0d624560
 0 B / 1.48 KB [---------------------------------------------------------------]
Writing manifest to image destination
Storing signatures
Loaded image:  registry.fedoraproject.org/fedora:latest
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-save(1)](podman-save.1.md)**

## HISTORY
July 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
