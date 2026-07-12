% podman-network-rm 1

## NAME
podman\-network\-rm - Remove one or more networks

## SYNOPSIS
**podman network rm** [*options*] [*network...*]

## DESCRIPTION
Delete one or more Podman networks.

## OPTIONS
#### **--force**, **-f**

The `force` option removes all containers that use the named network. If the container is
running, the container is stopped and removed.

#### **--ignore**, **-i**

Ignore the error when a specified network does not exist.
Other failures, including a network being in use, are still returned.

#### **--time**, **-t**=*seconds*

Seconds to wait before forcibly stopping the running containers that are using the specified network. The --force option must be specified to use the --time option. Use -1 for infinite wait.

## EXAMPLES

Delete specified network:
```
# podman network rm podman9
podman9
```

Delete specified network and all containers associated with the network:
```
# podman network rm -f fred
fred
```

Delete specified network and all containers associated with the network after waiting up to 15 seconds:
```
# podman network rm --force --time 15 fred
fred
```

## Exit Status
  **0**   All specified networks removed

  **1**   One of the specified networks did not exist, and no other failures

  **2**   The network is in use by a container or a Pod

  **125** The command fails for any other reason

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-network(1)](podman-network.1.md)**

## HISTORY
August 2019, Originally compiled by Brent Baude <bbaude@redhat.com>
