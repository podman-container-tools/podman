% podman-network-exists 1

## NAME
podman\-network\-exists - Check if the given network exists

## SYNOPSIS
**podman network exists** *network*

## DESCRIPTION
**podman network exists** checks if a network exists. The **Name** or **ID**
of the network may be used as input.  Podman returns an exit code
of `0` when the network is found.  A `1` is returned otherwise. An exit code of
`125` indicates there was another issue.


## OPTIONS

#### **--help**, **-h**

Print usage statement

## EXAMPLE

Check if specified network exists (the network does actually exist):
```
$ podman network exists net1
$ echo $?
0
```

Check if nonexistent network exists:
```
$ podman network exists webbackend
$ echo $?
1
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-network(1)](podman-network.1.md)**

## HISTORY
January 2021, Originally compiled by Paul Holzinger `<paul.holzinger@web.de>`
