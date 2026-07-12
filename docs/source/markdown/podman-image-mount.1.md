% podman-image-mount 1

## NAME
podman\-image\-mount - Mount an image's root filesystem

## SYNOPSIS
**podman image mount** [*options*] [*image* ...]

## DESCRIPTION
Mounts the specified images' root file system in a location which can be
accessed from the host, and returns its location.

The `podman image mount` command without any arguments lists all of the
currently mounted images.

Rootless mode only supports mounting VFS driver, unless podman is run in a user namespace.
Use the `podman unshare` command to enter the user namespace. All other storage drivers fail to mount.

## RETURN VALUE
The location of the mounted file system.  On error an empty string and errno is
returned.

## OPTIONS

#### **--all**, **-a**

Mount all images.

#### **--format**=*format*

Print the mounted images in specified format (json).

## EXAMPLES

Mount multiple images. Note: In rootless mode, image mounting works only after executing the podman unshare command to enter the user namespace.
```
podman image mount fedora ubi8-init
/var/lib/containers/storage/overlay/f3ac502d97b5681989dff84dfedc8354239bcecbdc2692f9a639f4e080a02364/merged
/var/lib/containers/storage/overlay/0ff7d7ca68bed1ace424f9df154d2dd7b5a125c19d887f17653cbcd5b6e30ba1/merged
```

List mounted images:
```
podman image mount
registry.fedoraproject.org/fedora:latest /var/lib/containers/storage/overlay/f3ac502d97b5681989dff84dfedc8354239bcecbdc2692f9a639f4e080a02364/merged
registry.access.redhat.com/ubi8-init:latest /var/lib/containers/storage/overlay/0ff7d7ca68bed1ace424f9df154d2dd7b5a125c19d887f17653cbcd5b6e30ba1/merged
```

List mounted images in JSON format:
```
podman image mount --format json
[
 {
  "id": "00ff39a8bf19f810a7e641f7eb3ddc47635913a19c4996debd91fafb6b379069",
  "Names": [
   "sha256:58de585a231aca14a511347bc85b912a6f000159b49bc2b0582032911e5d3a6c"
  ],
  "Repositories": [
   "registry.fedoraproject.org/fedora:latest"
  ],
  "mountpoint": "/var/lib/containers/storage/overlay/0ccfac04663bbe8813b5f24502ee0b7371ce5bf3c5adeb12e4258d191c2cf7bc/merged"
 },
 {
  "id": "bcc2dc9a261774ad25a15e07bb515f9b77424266abf2a1252ec7bcfed1dd0ac2",
  "Names": [
   "sha256:d5f260b2e51b3ee9d05de1c31d261efc9af28e7d2d47cedf054c496d71424d63"
  ],
  "Repositories": [
   "registry.access.redhat.com/ubi8-init:latest"
  ],
  "mountpoint": "/var/lib/containers/storage/overlay/d66b58e3391ea8ce4c81316c72e22b332618f2a28b461a32ed673e8998cdaeb8/merged"
 }
]
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-image(1)](podman-image.1.md)**, **[podman-image-unmount(1)](podman-image-unmount.1.md)**, **[podman-unshare(1)](podman-unshare.1.md)**, **mount(8)**
