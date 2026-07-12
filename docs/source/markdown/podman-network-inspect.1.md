% podman-network-inspect 1

## NAME
podman\-network\-inspect - Display the network configuration for one or more networks

## SYNOPSIS
**podman network inspect** [*options*] *network* [*network* ...]

## DESCRIPTION
Display the (JSON format) network configuration.

## OPTIONS
#### **--format**, **-f**=*format*

Pretty-print networks to JSON or using a Go template.

| **Placeholder**    | **Description**                           |
|--------------------|-------------------------------------------|
| .Containers ...    | Running containers on this network.       |
| .Created ...       | Timestamp when the network was created    |
| .DNSEnabled        | Network has dns enabled (boolean)         |
| .Driver            | Network driver                            |
| .ID                | Network ID                                |
| .Internal          | Network is internal (boolean)             |
| .IPAMOptions ...   | Network ipam options                      |
| .IPv6Enabled       | Network has ipv6 subnet (boolean)         |
| .Labels ...        | Network labels                            |
| .Name              | Network name                              |
| .Network ...       | Nested Network type                       |
| .NetworkDNSServers | Array of DNS servers used in this network |
| .NetworkInterface  | Name of the network interface on the host |
| .Options ...       | Network options                           |
| .Routes            | List of static routes for this network    |
| .Subnets           | List of subnets on this network           |

## EXAMPLE

Inspect the default podman network.

```
$ podman network inspect podman
[
    {
        "name": "podman",
        "id": "2f259bab93aaaaa2542ba43ef33eb990d0999ee1b9924b557b7be53c0b7a1bb9",
        "driver": "bridge",
        "network_interface": "podman0",
        "created": "2021-06-03T12:04:33.088567413+02:00",
        "subnets": [
            {
                "subnet": "10.88.0.0/16",
                "gateway": "10.88.0.1"
            }
        ],
        "ipv6_enabled": false,
        "internal": false,
        "dns_enabled": false,
        "ipam_options": {
            "driver": "host-local"
        }
    }
]
```

Show the subnet and gateway for a network.

```
$ podman network inspect podman --format "{{range .Subnets}}Subnet: {{.Subnet}} Gateway: {{.Gateway}}{{end}}"
Subnet: 10.88.0.0/16 Gateway: 10.88.0.1
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-network(1)](podman-network.1.md)**, **[podman-network-ls(1)](podman-network-ls.1.md)**, **[podman-network-create(1)](podman-network-create.1.md)**

## HISTORY
August 2021, Updated with the new network format by Paul Holzinger <pholzing@redhat.com>

August 2019, Originally compiled by Brent Baude <bbaude@redhat.com>
