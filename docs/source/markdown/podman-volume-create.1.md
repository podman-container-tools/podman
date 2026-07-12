% podman-volume-create 1

## NAME
podman\-volume\-create - Create a new volume

## SYNOPSIS
**podman volume create** [*options*] [*name*]

## DESCRIPTION

Creates an empty volume and prepares it to be used by containers. The volume
can be created with a specific name, if a name is not given a random name is
generated. You can add metadata to the volume by using the **--label** flag and
driver options can be set using the **--opt** flag.

## OPTIONS

#### **--driver**, **-d**=*driver*

Specify the volume driver name (default **local**).
There are two drivers supported by Podman itself: **local** and **image**.

The **local** driver uses a directory on disk as the backend by default, but can also use the **mount(8)** command to mount a filesystem as the volume if **--opt** is specified.

The **image** driver uses an image as the backing store of for the volume.
An overlay filesystem is created, which allows changes to the volume to be committed as a new layer on top of the image.

Using a value other than **local** or **image**, Podman attempts to create the volume using a volume plugin with the given name.
Such plugins must be defined in the **volume_plugins** section of the **[containers.conf(5)](https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md)** configuration file.

#### **--gid**=*gid*

Set the GID that the volume will be created as. Differently than `--opt o=gid=*gid*`, the specified value is not passed to the mount operation. The specified GID will own the volume's mount point directory and affects the volume chown operation.

#### **--help**

Print usage statement

#### **--ignore**

Don't fail if the named volume already exists, instead just print the name. Note that the new options are not applied to the existing volume.

#### **--label**, **-l**=*label*

Set metadata for the volume (e.g., `--label mykey=value`). Can be used multiple times.
These labels can be viewed in `podman volume inspect` or `podman volume ls`.

#### **--opt**, **-o**=*option*

Set driver specific options.
For the default driver, **local**, this allows a volume to be configured to mount a filesystem on the host.

For the `local` driver the following options are supported: `type`, `device`, `o`, and `[no]copy`.

  - The `type` option sets the type of the filesystem to be mounted, and is equivalent to the `-t` flag to **mount(8)**.
  - The `device` option sets the device to be mounted, and is equivalent to the `device` argument to **mount(8)**.
  - The `copy` option enables copying files from the container image path where the mount is created to the newly created volume on the first run.  `copy` is the default.

The `o` option sets options for the mount, and is equivalent to the filesystem
options (also `-o`) passed to **mount(8)** with the following exceptions:

  - The `o` option supports `uid` and `gid` options to set the UID and GID of the created volume that are not normally supported by **mount(8)**.
  - The `o` option supports the `size` option to set the maximum size of the created volume, the `inodes` option to set the maximum number of inodes for the volume, and `noquota` to completely disable quota support even for tracking of disk usage.
  The `size` option is supported on the "tmpfs" and "xfs[note]" file systems.
  The `inodes` option is supported on the "xfs[note]" file systems.
  Note: xfs filesystems must be mounted with the `prjquota` flag described in the **xfs_quota(8)** man page. Podman will throw an error if they're not.
  - The `o` option supports using volume options other than the UID/GID options with the **local** driver and requires root privileges.
  - The `o` options supports the `timeout` option which allows users to set a driver specific timeout in seconds before volume creation fails. For example, **--opt=o=timeout=10** sets a driver timeout of 10 seconds.

***Note*** Do not confuse the `--opt,-o` create option with the `-o` mount option.  For example, with `podman volume create`, use `-o=o=uid=1000` *not* `-o=uid=1000`.

For the **image** driver, the only supported option is `image`, which specifies the image the volume is based on.
This option is mandatory when using the **image** driver.

When not using the **local** and **image** drivers, the given options are passed directly to the volume plugin. In this case, supported options are dictated by the plugin in question, not Podman.

#### **--uid**=*uid*

Set the UID that the volume will be created as. Differently than `--opt o=uid=*uid*`, the specified value is not passed to the mount operation. The specified UID will own the volume's mount point directory and affects the volume chown operation.

## QUOTAS

`podman volume create` uses `XFS project quota controls` for controlling the size and the number of inodes of builtin volumes. The directory used to store the volumes must be an `XFS` file system and be mounted with the `pquota` option.

Example /etc/fstab entry:
```
/dev/podman/podman-var /var xfs defaults,x-systemd.device-timeout=0,pquota 1 2
```

Podman generates project IDs for each builtin volume, but these project IDs need to be unique for the XFS file system. These project IDs by default are generated randomly, with a potential for overlap with other quotas on the same file
system.

The xfs_quota tool can be used to assign a project ID to the storage driver directory, e.g.:

```
echo 100000:/var/lib/containers/storage/overlay >> /etc/projects
echo 200000:/var/lib/containers/storage/volumes >> /etc/projects
echo storage:100000 >> /etc/projid
echo volumes:200000 >> /etc/projid
xfs_quota -x -c 'project -s storage volumes' /<xfs mount point>
```

In the example above we are configuring the overlay storage driver for newly
created containers as well as volumes to use project IDs with a **start offset**.
All containers are assigned larger project IDs (e.g. >= 100000).
All volume assigned project IDs larger project IDs starting with 200000.
This prevents xfs_quota management conflicts with containers/storage.

## EXAMPLES

Create empty volume.
```
$ podman volume create
```

Create empty named volume.
```
$ podman volume create myvol
```

Create empty named volume with specified label.
```
$ podman volume create --label foo=bar myvol
```

Create tmpfs named volume with specified size and mount options.
```
# podman volume create --opt device=tmpfs --opt type=tmpfs --opt o=size=2M,nodev,noexec myvol
```

Create tmpfs named volume testvol with specified options.
```
# podman volume create --opt device=tmpfs --opt type=tmpfs --opt o=uid=1000,gid=1000 testvol
```

Create volume overriding the owner UID and GID.
```
# podman volume create --uid 1000 --gid 1000 myvol
```

Create image named volume using the specified local image in containers/storage.
```
# podman volume create --driver image --opt image=fedora:latest fedoraVol
```

## MOUNT EXAMPLES

`podman volume create` allows the `type`, `device`, and `o` options to be passed to `mount(8)` when using the `local` driver.

## [s3fs-fuse](https://github.com/s3fs-fuse/s3fs-fuse)

[s3fs-fuse](https://github.com/s3fs-fuse/s3fs-fuse) or just `s3fs`, is a [fuse](https://github.com/libfuse/libfuse) filesystem that allows s3 prefixes to be mounted as filesystem mounts.

**Installing:**
```shell
$ doas dnf install s3fs-fuse
```

**Simple usage:**
```shell
$ s3fs --help
$ s3fs -o use_xattr,endpoint=aq-central-1 bucket:/prefix /mnt
```

**Equivalent through `mount(8)`**
```shell
$ mount -t fuse.s3fs -o use_xattr,endpoint=aq-central-1 bucket:/prefix /mnt
```

**Equivalent through `podman volume create`**
```shell
$ podman volume create s3fs-fuse-volume -o type=fuse.s3fs -o device=bucket:/prefix -o o=use_xattr,endpoint=aq-central-1
```

**The volume can then be mounted in a container with**
```shell
$ podman run -v s3fs-fuse-volume:/s3:z --rm -it fedora:latest
```

Please see the available [options](https://github.com/s3fs-fuse/s3fs-fuse/wiki/Fuse-Over-Amazon#options) on their wiki.

### Using with other container users

The above example works because the volume is mounted as the host user and inside the container `root` is mapped to the user in the host.

If the mount is accessed by a different user inside the container, a "Permission denied" error will be raised.

```shell
$ podman run --user bin:bin -v s3fs-fuse-volume:/s3:z,U --rm -it fedora:latest
$ ls /s3
# ls: /s3: Permission denied
```

In FUSE-land, mounts are protected for the user who mounted them; specify the `allow_other` mount option if other users need access.
> Note: This will remove the normal fuse [security measures](https://github.com/libfuse/libfuse/wiki/FAQ#why-dont-other-users-have-access-to-the-mounted-filesystem) on the mount point, after which, the normal filesystem permissions will have to protect it

```shell
$ podman volume create s3fs-fuse-other-volume -o type=fuse.s3fs -o device=bucket:/prefix -o o=allow_other,use_xattr,endpoint=aq-central-1
$ podman run --user bin:bin -v s3fs-fuse-volume:/s3:z,U --rm -it fedora:latest
$ ls /s3
```

### The Prefix must exist

`s3fs` will fail to mount if the prefix does not exist in the bucket.

Create a s3-directory by putting an empty object at the desired `prefix/` key
```shell
$ aws s3api put-object --bucket bucket --key prefix/
```

If performance is the priority, please check out the more performant [goofys](https://github.com/kahing/goofys).

> FUSE filesystems exist for [Google Cloud Storage](https://github.com/GoogleCloudPlatform/gcsfuse) and [Azure Blob Storage](https://github.com/Azure/azure-storage-fuse)


## SEE ALSO
**[podman(1)](podman.1.md)**, **[containers.conf(5)](https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md)**, **[podman-volume(1)](podman-volume.1.md)**, **mount(8)**, **xfs_quota(8)**, **xfs_quota(8)**, **projects(5)**, **projid(5)**

## HISTORY
January 2020, updated with information on volume plugins by Matthew Heon <mheon@redhat.com>
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
