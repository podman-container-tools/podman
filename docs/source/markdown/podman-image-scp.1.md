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

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-load(1)](podman-load.1.md)**, **[podman-save(1)](podman-save.1.md)**, **[podman-remote(1)](podman-remote.1.md)**, **[podman-system-connection-add(1)](podman-system-connection-add.1.md)**, **[containers.conf(5)](https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md)**, **[containers-transports(5)](https://github.com/containers/image/blob/main/docs/containers-transports.5.md)**

## HISTORY
July 2021, Originally written by Charlie Doern <cdoern@redhat.com>
