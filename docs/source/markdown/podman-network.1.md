% podman-network 1

## NAME
podman\-network - Manage Podman networks

## SYNOPSIS
**podman network** *subcommand*

## DESCRIPTION
The network command manages networks for Podman.

Podman uses [Netavark](https://github.com/containers/netavark) as the network backend.

## DNS NOTES

On networks with DNS enabled (the default, unless **--disable-dns** is used),
[aardvark-dns](https://github.com/containers/aardvark-dns) registers each
container under its name and aliases. Containers that share a pod network
namespace are registered under the pod name.

The container name is always registered. The short container ID (first 12
characters) is always registered as an alias. When **--hostname** is explicitly
set, that hostname is also registered as an alias. By default the hostname
inside the container is the short ID, so it already matches the alias. Setting
*container_name_as_hostname=true* in the `[containers]` section of
**[containers.conf(5)](https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md)**
changes the hostname only; DNS entries remain unchanged. Additional names can be set with the **alias=** option
in **--network** (e.g., **podman run --network mynet:alias=foo**),
with **--network-alias**, or with **--alias** on **podman network connect**.
Auto-generated names use an underscore between words, for example `exciting_chebyshev`.

## COMMANDS

| Command    | Man Page                                                       | Description                                                     |
| ---------- | -------------------------------------------------------------- | --------------------------------------------------------------- |
| connect    | [podman-network-connect(1)](podman-network-connect.1.md)       | Connect a container to a network                                |
| create     | [podman-network-create(1)](podman-network-create.1.md)         | Create a Podman network                                         |
| disconnect | [podman-network-disconnect(1)](podman-network-disconnect.1.md) | Disconnect a container from a network                           |
| exists     | [podman-network-exists(1)](podman-network-exists.1.md)         | Check if the given network exists                               |
| inspect    | [podman-network-inspect(1)](podman-network-inspect.1.md)       | Display the network configuration for one or more networks      |
| ls         | [podman-network-ls(1)](podman-network-ls.1.md)                 | Display a summary of networks                                   |
| prune      | [podman-network-prune(1)](podman-network-prune.1.md)           | Remove all unused networks                                      |
| reload     | [podman-network-reload(1)](podman-network-reload.1.md)         | Reload network configuration for containers                     |
| rm         | [podman-network-rm(1)](podman-network-rm.1.md)                 | Remove one or more networks                                     |
| update     | [podman-network-update(1)](podman-network-update.1.md)         | Update an existing Podman network                               |

## SUBNET NOTES
Podman requires specific default IPs and, thus, network subnets.  The default values used by Podman can be modified in the **[containers.conf(5)](https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md)** file.

### Podman network
The default bridge network (called `podman`) uses 10.88.0.0/16 as a subnet. When Podman runs as root, the `podman` network is used as default.  It is the same as adding the option `--network bridge` or `--network podman`. This subnet can be changed in **[containers.conf(5)](https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md)** under the [network] section. Set the `default_subnet` to any subnet that is free in the environment. The name of the default network can also be changed from `podman` to another name using the default network key. Note that this is only done when no containers are running.

### Pasta
Pasta by default performs no Network Address Translation (NAT) and copies the IPs from your main interface into the container namespace. If pasta cannot find an interface with the default route, it will select an interface if there is only one interface with a valid route. If you do not have a default route and several interfaces have defined routes, pasta will be unable to figure out the correct interface and it will fail to start. To specify the interface, use `-i` option to pasta. A default set of pasta options can be set in **[containers.conf(5)](https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md)** under the `[network]` section with the `pasta_options` key.

Pasta is the default rootless networking tool.

### Podman network create
When a new network is created with a `podman network create` command, and no subnet is given with the --subnet option, Podman starts picking a free subnet from 10.89.0.0/24 to 10.255.255.0/24. Use the `default_subnet_pools` option under the `[network]` section in **[containers.conf(5)](https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md)** to change the range and/or size that is assigned by default.

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-network-create(1)](podman-network-create.1.md)**, **[containers.conf(5)](https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md)**
