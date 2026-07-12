% podman-machine-inspect 1

## NAME
podman\-machine\-inspect - Inspect one or more virtual machines

## SYNOPSIS
**podman machine inspect** [*options*] [*name*] ...

## DESCRIPTION

Inspect one or more virtual machines

Obtain greater detail about Podman virtual machines. More than one virtual machine can be
inspected at once.

The default machine name is `podman-machine-default`. If a machine name is not specified as an argument,
then `podman-machine-default` will be inspected.

Rootless only.

## OPTIONS
#### **--format**

Print results with a Go template.

| **Placeholder**     | **Description**                                                       |
| ------------------- | --------------------------------------------------------------------- |
| .ConfigDir ...      | Machine configuration directory location                                   |
| .ConnectionInfo ... | Machine connection information                                        |
| .Created ...        | Machine creation time (string, ISO3601)                               |
| .LastUp ...         | Time when machine was last booted                                     |
| .Name               | Name of the machine                                                   |
| .Resources ...      | Resources used by the machine                                         |
| .Rootful            | Whether the machine prefers rootful or rootless container execution   |
| .Rosetta            | Whether this machine uses Rosetta                               |
| .SSHConfig ...      | SSH configuration info for communicating with machine                 |
| .State              | Machine state                                                         |
| .UserModeNetworking | Whether this machine uses user-mode networking                        |

#### **--help**

Print usage statement.

## EXAMPLES

Inspect the specified Podman machine.
```
$ podman machine inspect podman-machine-default
[
     {
          "ConfigDir": {
               "Path": "/Users/jacksparrow/.config/containers/podman/machine/applehv"
          },
          "ConnectionInfo": {
               "PodmanSocket": {
                    "Path": "/var/folders/9r/n3056v597wv2cq8s2j80bdnw0000gn/T/podman/podman-machine-default-api.sock"
               },
               "PodmanPipe": null
          },
          "Created": "2025-02-11T14:12:48.231836+05:30",
          "LastUp": "2025-08-12T19:31:19.391294+05:30",
          "Name": "podman-machine-default",
          "Resources": {
               "CPUs": 6,
               "DiskSize": 100,
               "Memory": 6144,
               "USBs": []
          },
          "SSHConfig": {
               "IdentityPath": "/Users/jacksparrow/.local/share/containers/podman/machine/machine",
               "Port": 53298,
               "RemoteUsername": "core"
          },
          "State": "running",
          "UserModeNetworking": true,
          "Rootful": false,
          "Rosetta": true
     }
]
```

Show machine name and state:
```
$ podman machine inspect --format "{{.Name}}\t{{.State}}"
podman-machine-default running
```

Show machine resource information:
```
$ podman machine inspect --format "Machine: {{.Name}}\nCPUs: {{.Resources.CPUs}}\nMemory: {{.Resources.Memory}} bytes\nDisk: {{.Resources.DiskSize}} bytes"
Machine: podman-machine-default
CPUs: 6
Memory: 6144 bytes
Disk: 100 bytes
```

Show machine configuration details:
```
$ podman machine inspect --format "{{.Name}}: {{.State}} (Rootful: {{.Rootful}}, User Networking: {{.UserModeNetworking}})"
podman-machine-default: running (Rootful: false, User Networking: true)
```

Show machine uptime information:
```
$ podman machine inspect --format "Created: {{.Created}}\nLast Up: {{.LastUp}}\nState: {{.State}}"
Created: 2025-02-11 14:12:48.231836 +0000 UTC
Last Up: 2025-08-12 19:31:19.391294 +0000 UTC
State: running
```

Show connection information:
```
$ podman machine inspect --format "Socket: {{.ConnectionInfo.PodmanSocket}}\nConfig Dir: {{.ConfigDir}}"
Socket: {/var/folders/9r/n3056v597wv2cq8s2j80bdnw0000gn/T/podman/podman-machine-default-api.sock <nil>}
Config Dir: {/Users/jacksparrow/.config/containers/podman/machine/applehv <nil>}
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
April 2022, Originally compiled by Brent Baude <bbaude@redhat.com>
