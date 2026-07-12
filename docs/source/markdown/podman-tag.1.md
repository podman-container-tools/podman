% podman-tag 1

## NAME
podman\-tag - Add an additional name to a local image

## SYNOPSIS
**podman tag** *image*[:*tag*] [*target-name*[:*tag*]...] [*options*]

**podman image tag** *image*[:*tag*] [*target-name*[:*tag*]...] [*options*]

## DESCRIPTION
Assigns a new image name to an existing image.  A full name refers to the entire
image name, including the optional *tag* after the `:`.  If there is no *tag*
provided, then Podman defaults to `latest` for both the *image* and the
*target-name*.

## OPTIONS

#### **--help**, **-h**

Print usage statement

## EXAMPLES

Tag specified image with an image name defaulting to :latest.
```
$ podman tag 0e3bbc2 fedora:latest
```

Tag specified image with fully specified image name.
```
$ podman tag httpd myregistryhost:5000/fedora/httpd:v2
```

Tag specified image with multiple tags.
```
$ podman tag mymariadb mycontainerregistry.io/namespace/mariadb:10 mycontainerregistry.io/namespace/mariadb:10.11 mycontainerregistry.io/namespace/mariadb:10.11.12
```


## SEE ALSO
**[podman(1)](podman.1.md)**

## HISTORY
December 2019, Update description to refer to 'name' instead of 'alias' by Sascha Grunert <sgrunert@suse.com>
July 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
