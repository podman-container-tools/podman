% podman-pod-prune 1

## NAME
podman-pod-prune - Remove all stopped pods and their containers

## SYNOPSIS
**podman pod prune** [*options*]

## DESCRIPTION
**podman pod prune** removes all stopped pods and their containers from local storage.

## OPTIONS

#### **--force**, **-f**
Force removal of all running pods and their containers. The default is false.

## EXAMPLES

Remove all stopped pods and their containers from local storage.
```
$ sudo podman pod prune
WARNING! This will remove all stopped/exited pods..
Are you sure you want to continue? [y/N] y
22b8813332948064b6566370088c5e0230eeaf15a58b1c5646859fd9fc364fe7
2afb26869fe5beab979c234afb75c7506063cd4655b1a73557c9d583ff1aebe9
49161ad2a722cf18722f0e17199a9e840703a17d1158cdeda502b6d54080f674
5ca429f37fb83a9f54eea89e3a9102b7780a6e6ae5f132db0672da551d862c4a
6bb06573787efb8b0675bc88ebf8361f1a56d3ac7922d1a6436d8f59ffd955f1
```

Force removal of all running pods and their containers.
```
$ sudo podman pod prune --force
22b8813332948064b6566370088c5e0230eeaf15a58b1c5646859fd9fc364fe7
2afb26869fe5beab979c234afb75c7506063cd4655b1a73557c9d583ff1aebe9
49161ad2a722cf18722f0e17199a9e840703a17d1158cdeda502b6d54080f674
5ca429f37fb83a9f54eea89e3a9102b7780a6e6ae5f132db0672da551d862c4a
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-pod(1)](podman-pod.1.md)**

## HISTORY
April 2019, Originally compiled by Peter Hunt (pehunt at redhat dot com)
