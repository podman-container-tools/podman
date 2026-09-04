% podman-image-scp 1

## NAME
podman-image-scp - Securely copy an image from one host to another

## SYNOPSIS
**podman image scp** [*options*] *name*[:*tag*]

## DESCRIPTION
**podman image scp** copies container images between hosts on a network. This command can copy images to the remote host or from the remote host as well as between two remote hosts.
Note: `::` is used to specify the image name depending on Podman is saving or loading. Images can also be transferred from rootful to rootless storage on the same machine without using sshd. This feature is not supported on the remote client, including Mac and Windows (excluding WSL2) machines.

This is not a direct storage-to-storage copy. The image is saved to an archive (using **podman save**), the archive file is transferred (e.g., over SSH), and then loaded on the destination. As a result, digest references to the original compressed blobs are not preserved (e.g., **podman pull** *image*@*digest* followed by **podman image scp** and then inspecting by that digest may not work). For regular workflows, using a registry (push from source, pull on destination) is often preferable.

**podman image scp [GLOBAL OPTIONS]**

**podman image** *scp [OPTIONS] NAME[:TAG] [HOSTNAME::]*

**podman image** *scp [OPTIONS] [HOSTNAME::]IMAGENAME*

**podman image** *scp [OPTIONS] [HOSTNAME::]IMAGENAME [HOSTNAME::]*

## OPTIONS

#### **--compression-format**=*algorithm*

Compress the transfer archive with *algorithm* before it is sent over the network. Allowed values are **gzip** and **zstd**. If omitted, the archive is transferred uncompressed.

Because **podman save** writes docker-archive layers uncompressed, compressing the archive typically cuts the transferred data to around half its original size or less. The receiving **podman load** detects the compression and decompresses the archive itself, so nothing has to be configured on the destination.

How much is gained depends on **--format**. An **oci-archive** keeps the layers in the compression they already have, so there is little left to compress and the transfer is barely smaller; the saving applies to the default **docker-archive**.

Compression is applied on the host that produces the archive, so that only compressed bytes cross the network. When the source is a remote host, the archive is compressed there and the matching command line compressor (**gzip** or **zstd**) must be installed on that host. When the source is local, Podman compresses the archive itself and no extra tooling is needed.

This option has no effect on a transfer between two users on the same machine, because no data crosses a network.

#### **--compression-level**=*level*

Compression level to use, **1**-**9** for **gzip** and **1**-**19** for **zstd**. If omitted, the algorithm's default is used. Requires **--compression-format**.

The accepted range for **zstd** stops at **19** rather than the **20** accepted by **podman push**, because levels above that need the compressor's *--ultra* mode when the archive is compressed on a remote host.

Note that **zstd** levels are only fully distinct when the source is a remote host, where the level is passed to the command line compressor. When the source is local, Podman compresses through the same library used elsewhere, which groups the level into four bands (**1**-**2**, **3**-**5**, **6**-**9**, **10** and above), so any level of **10** or more produces the same output.

#### **--format**=*format*

Format passed to **podman save** when creating the transfer archive. Allowed values are **oci-archive** and **docker-archive**. If omitted, **podman save** uses its default (docker-archive).

Only the **oci-archive** and **docker-archive** archive (tar) formats are supported. Directory formats (**oci-dir**, **docker-dir**) are not supported because the transfer sends a single file; the remote path does not support directory layouts.

#### **--help**, **-h**

Print usage statement

#### **--quiet**, **-q**

Suppress the output

## EXAMPLES

Copy specified image to local storage:
```
$ podman image scp alpine
Loaded image: docker.io/library/alpine:latest
```

Copy specified image from local storage to remote connection:
```
$ podman image scp alpine Fedora::/home/charliedoern/Documents/alpine
Getting image source signatures
Copying blob 72e830a4dff5 done
Copying config 85f9dc67c7 done
Writing manifest to image destination
Storing signatures
Loaded image: docker.io/library/alpine:latest
```

Copy specified image from remote connection to remote connection:
```
$ podman image scp Fedora::alpine RHEL::
Loaded image: docker.io/library/alpine:latest
```

Copy specified image via ssh to local storage:
```
$ podman image scp charliedoern@192.168.68.126:22/run/user/1000/podman/podman.sock::alpine
WARN[0000] Unknown connection name given. Please use system connection add to specify the default remote socket location
Getting image source signatures
Copying blob 9450ef9feb15 [--------------------------------------] 0.0b / 0.0b
Copying config 1f97f0559c done
Writing manifest to image destination
Storing signatures
Loaded image: docker.io/library/alpine:latest
```

Copy specified image from root account to user accounts local storage:
```
$ sudo podman image scp root@localhost::alpine username@localhost::
Copying blob e2eb06d8af82 done
Copying config 696d33ca15 done
Writing manifest to image destination
Storing signatures
Getting image source signatures
Copying blob 5eb901baf107 skipped: already exists
Copying config 696d33ca15 done
Writing manifest to image destination
Storing signatures
Loaded image: docker.io/library/alpine:latest
```

Copy specified image from root account to local storage:
```
$ sudo podman image scp root@localhost::alpine
Copying blob e2eb06d8af82 done
Copying config 696d33ca15 done
Writing manifest to image destination
Storing signatures
Getting image source signatures
Copying blob 5eb901baf107
Copying config 696d33ca15 done
Writing manifest to image destination
Storing signatures
Loaded image: docker.io/library/alpine:latest
```

Copy image to rootful storage with OCI archive format:
```
$ podman image scp --format oci-archive quay.io/fedora/fedora:43 root@localhost::
```

Copy image to remote host (uses default format when **--format** is omitted):
```
$ podman image scp alpine root@myserver::
```

Copy specified image to a remote host, compressing the transfer archive with zstd:
```
$ podman image scp --compression-format zstd alpine root@myserver::
Loaded image: docker.io/library/alpine:latest
```

Copy specified image to a remote host, trading CPU time for the smallest transfer:
```
$ podman image scp --compression-format zstd --compression-level 19 alpine root@myserver::
Loaded image: docker.io/library/alpine:latest
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-load(1)](podman-load.1.md)**, **[podman-save(1)](podman-save.1.md)**, **[podman-remote(1)](podman-remote.1.md)**, **[podman-system-connection-add(1)](podman-system-connection-add.1.md)**, **[containers.conf(5)](https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md)**, **[containers-transports(5)](https://github.com/containers/image/blob/main/docs/containers-transports.5.md)**

## HISTORY
July 2021, Originally written by Charlie Doern <cdoern@redhat.com>
