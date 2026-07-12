% podman-network-update 1

## NAME
podman\-network\-update - Update an existing Podman network

## SYNOPSIS
**podman network update**  [*options*] *network*

## DESCRIPTION
Allow changes to existing container networks. At present, only changes to the DNS servers in use by a network is supported.

NOTE: Only supported with the netavark network backend.


## OPTIONS
#### **--dns-add**

Accepts array of DNS resolvers and add it to the existing list of resolvers configured for a network.

#### **--dns-drop**

Accepts array of DNS resolvers and removes them from the existing list of resolvers configured for a network.

## EXAMPLE

Update a network:
```
$ podman network update network1 --dns-add 8.8.8.8,1.1.1.1
```

Update a network and add/remove dns servers:
```
$ podman network update network1 --dns-drop 8.8.8.8 --dns-add 3.3.3.3
```
## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-network(1)](podman-network.1.md)**, **[podman-network-inspect(1)](podman-network-inspect.1.md)**, **[podman-network-ls(1)](podman-network-ls.1.md)**
