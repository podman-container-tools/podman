% podman-artifact-extract 1

## NAME
podman\-artifact\-extract - Extract an OCI artifact to a local path

## SYNOPSIS
**podman artifact extract** *artifact* *target*

## DESCRIPTION

Extract the blobs of an OCI artifact to a local file or directory.

If the target path is a file or does not exist, the artifact must either consist
of one blob (layer) or if it has multiple blobs (layers) then the **--digest** or
**--title** option must be used to select only a single blob. If the file already
exists it will be overwritten.

If the target is a directory (it must exist), all blobs will be copied to the
target directory. As the target file name the value from the `org.opencontainers.image.title`
annotation is used. If the annotation is missing, the target file name will be the
digest of the blob (with `:` replaced by `-` in the name).
If the target file already exists in the directory, it will be overwritten.

## OPTIONS

#### **--digest**=**digest**

When extracting blobs from the artifact only use the one with the specified digest.
If the target is a directory then the digest is always used as file name instead even
when the title annotation exists on the blob.
Conflicts with **--title**.

#### **--title**=**title**

When extracting blobs from the artifact only use the one with the specified title.
It looks for the `org.opencontainers.image.title` annotation and compares that
against the given title.
Conflicts with **--digest**.

## EXAMPLES

Extract an artifact with a single blob

```
$ podman artifact extract quay.io/artifact/foobar1:test /tmp/myfile
```

Extract an artifact with multiple blobs

```
$ podman artifact extract quay.io/artifact/foobar2:test /tmp/mydir
$ ls /tmp/mydir
CONTRIBUTING.md  README.md
```

Extract only a single blob from an artifact with multiple blobs

```
$ podman artifact extract --title README.md quay.io/artifact/foobar2:test /tmp/mydir
$ ls /tmp/mydir
README.md
```
Or using the digest instead of the title
```
$ podman artifact extract --digest sha256:c0594e012b17fd9e6548355ceb571a79613f7bb988d7d883f112513601ac6e9a quay.io/artifact/foobar2:test /tmp/mydir
$ ls /tmp/mydir
README.md
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-artifact(1)](podman-artifact.1.md)**

## HISTORY
Feb 2025, Originally compiled by Paul Holzinger <pholzing@redhat.com>
