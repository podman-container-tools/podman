% podman-image-tree 1

## NAME
podman\-image\-tree - Print layer hierarchy of an image in a tree format

## SYNOPSIS
**podman image tree** [*options*] *image:tag*|*image-id*


## DESCRIPTION
Prints layer hierarchy of an image in a tree format.
If no *tag* is provided, Podman defaults to `latest` for the *image*.
Layers are indicated with image tags as `Top Layer of`, when the tag is known locally.
## OPTIONS

#### **--help**, **-h**

Print usage statement

#### **--whatrequires**

Show all child images and layers of the specified image

## EXAMPLES

List image tree information on specified image:
```
$ podman image tree docker.io/library/wordpress
Image ID: 6e880d17852f
Tags:    [docker.io/library/wordpress:latest]
Size:    429.9MB
Image Layers
├──  ID: 3c816b4ead84 Size: 58.47MB
├──  ID: e39dad2af72e Size: 3.584kB
├──  ID: b2d6a702383c Size: 213.6MB
├──  ID: 94609408badd Size: 3.584kB
├──  ID: f4dddbf86725 Size: 43.04MB
├──  ID: 8f695df43a4c Size: 11.78kB
├──  ID: c29d67bf8461 Size: 9.728kB
├──  ID: 23f4315918f8 Size:  7.68kB
├──  ID: d082f93a18b3 Size: 13.51MB
├──  ID: 7ea8bedcac69 Size: 4.096kB
├──  ID: dc3bbf7b3dc0 Size: 57.53MB
├──  ID: fdbbc6404531 Size: 11.78kB
├──  ID: 8d24785437c6 Size: 4.608kB
├──  ID: 80715f9e8880 Size: 4.608kB Top Layer of: [docker.io/library/php:7.2-apache]
├──  ID: c93cbcd6437e Size: 3.573MB
├──  ID: dece674f3cd1 Size: 4.608kB
├──  ID: 834f4497afda Size: 7.168kB
├──  ID: bfe2ce1263f8 Size: 40.06MB
└──  ID: 748e99b214cf Size: 11.78kB Top Layer of: [docker.io/library/wordpress:latest]
```

Show all child images and layers of the specified image:
```
$ podman image tree ae96a4ad4f3f --whatrequires
Image ID: ae96a4ad4f3f
Tags:    [docker.io/library/ruby:latest]
Size:    894.2MB
Image Layers
└──  ID: 9c92106221c7 Size:  2.56kB Top Layer of: [docker.io/library/ruby:latest]
 ├──  ID: 1b90f2b80ba0 Size: 3.584kB
 │   ├──  ID: 42b7d43ae61c Size: 169.5MB
 │   ├──  ID: 26dc8ba99ec3 Size: 2.048kB
 │   ├──  ID: b4f822db8d95 Size: 3.957MB
 │   ├──  ID: 044e9616ef8a Size: 164.7MB
 │   ├──  ID: bf94b940200d Size: 11.75MB
 │   ├──  ID: 4938e71bfb3b Size: 8.532MB
 │   └──  ID: f513034bf553 Size: 1.141MB
 ├──  ID: 1e55901c3ea9 Size: 3.584kB
 ├──  ID: b62835a63f51 Size: 169.5MB
 ├──  ID: 9f4e8857f3fd Size: 2.048kB
 ├──  ID: c3b392020e8f Size: 3.957MB
 ├──  ID: 880163026a0a Size: 164.8MB
 ├──  ID: 8c78b2b14643 Size: 11.75MB
 ├──  ID: 830370cfa182 Size: 8.532MB
 └──  ID: 567fd7b7bd38 Size: 1.141MB Top Layer of: [docker.io/circleci/ruby:latest]

```


## SEE ALSO
**[podman(1)](podman.1.md)**

## HISTORY
Feb 2019, Originally compiled by Kunal Kushwaha `<kushwaha_kunal_v7@lab.ntt.co.jp>`
