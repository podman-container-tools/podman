% podman-info 1

## NAME
podman\-info - Display Podman related system information

## SYNOPSIS
**podman info** [*options*]

**podman system info** [*options*]

## DESCRIPTION

Displays information pertinent to the host, current storage stats, configured container registries, and build of podman.
Host information includes configured CDI spec directories and resolved CDI devices when present.


## OPTIONS

#### **--format**, **-f**=*format*

Change output format to "json" or a Go template.

| **Placeholder**     | **Info pertaining to ...**              |
| ------------------- | --------------------------------------- |
| .Host ...           | ...the host on which podman is running  |
| .Plugins ...        | ...external plugins                     |
| .Registries ...     | ...configured registries                |
| .Store ...          | ...the storage driver and paths         |
| .Version ...        | ...podman version                       |

Each of the above branch out into further subfields, more than can
reasonably be enumerated in this document.

## EXAMPLES

Run `podman info` for a YAML formatted response:
```
$ podman info
host:
  arch: amd64
  buildahVersion: 1.23.0
  cgroupControllers: []
  cgroupManager: systemd
  cgroupVersion: v2
  conmon:
    package: conmon-2.0.29-2.fc34.x86_64
    path: /usr/bin/conmon
    version: 'conmon version 2.0.29, commit: '
 cpu_utilization:
   idle_percent: 96.84
   system_percent: 0.71
   user_percent: 2.45
  cpus: 8
  distribution:
    distribution: fedora
    variant: workstation
    version: "34"
  eventLogger: journald
  hostname: localhost.localdomain
  idMappings:
    gidmap:
    - container_id: 0
      host_id: 3267
      size: 1
    - container_id: 1
      host_id: 100000
      size: 65536
    uidmap:
    - container_id: 0
      host_id: 3267
      size: 1
    - container_id: 1
      host_id: 100000
      size: 65536
  kernel: 5.13.13-200.fc34.x86_64
  linkmode: dynamic
  logDriver: journald
  memFree: 1833385984
  memTotal: 16401895424
  networkBackend: netavark
  networkBackendInfo:
    backend: netavark
    dns:
      package: aardvark-dns-1.0.1-1.fc34.x86_64
      path: /usr/libexec/podman/aardvark-dns
      version: aardvark-dns 1.0.1
    package: netavark-1.0.1-1.fc34.x86_64
    path: /usr/libexec/podman/netavark
    version: netavark 1.0.1
  ociRuntime:
    name: crun
    package: crun-1.0-1.fc34.x86_64
    path: /usr/bin/crun
    version: |-
      crun version 1.0
      commit: 139dc6971e2f1d931af520188763e984d6cdfbf8
      spec: 1.0.0
      +SYSTEMD +SELINUX +APPARMOR +CAP +SECCOMP +EBPF +CRIU +YAJL
  os: linux
  rootlessPortForwarder: rootlessport
  pasta:
    executable: /usr/bin/passt
    package: passt-0^20221116.gace074c-1.fc34.x86_64
    version: |
      passt 0^20221116.gace074c-1.fc34.x86_64
      Copyright Red Hat
      GNU Affero GPL version 3 or later <https://www.gnu.org/licenses/agpl-3.0.html>
      This is free software: you are free to change and redistribute it.
      There is NO WARRANTY, to the extent permitted by law.
  remoteSocket:
    path: /run/user/3267/podman/podman.sock
  security:
    apparmorEnabled: false
    capabilities: CAP_CHOWN,CAP_DAC_OVERRIDE,CAP_FOWNER,CAP_FSETID,CAP_KILL,CAP_NET_BIND_SERVICE,CAP_SETFCAP,CAP_SETGID,CAP_SETPCAP,CAP_SETUID
    rootless: true
    seccompEnabled: true
    seccompProfilePath: /usr/share/containers/seccomp.json
    selinuxEnabled: true
  serviceIsRemote: false
  swapFree: 15687475200
  swapTotal: 16886259712
  uptime: 47h 15m 9.91s (Approximately 1.96 days)
plugins:
  log:
  - k8s-file
  - none
  - journald
  network:
  - bridge
  - macvlan
  volume:
  - local
registries:
  search:
  - registry.fedoraproject.org
  - registry.access.redhat.com
  - docker.io
  - quay.io
store:
  containerStore:
    number: 9
    paused: 0
    running: 1
    stopped: 8
  graphDriverName: overlay
  graphOptions: {}
  graphRoot: /home/dwalsh/.local/share/containers/storage
  graphRootAllocated: 510389125120
  graphRootUsed: 129170714624
  graphStatus:
    Backing Filesystem: extfs
    Native Overlay Diff: "true"
    Supports d_type: "true"
    Using metacopy: "false"
  imageCopyTmpDir: /home/dwalsh/.local/share/containers/storage/tmp
  imageStore:
    number: 5
  runRoot: /run/user/3267/containers
  transientStore: false
  volumePath: /home/dwalsh/.local/share/containers/storage/volumes
version:
  APIVersion: 4.0.0
  Built: 1631648722
  BuiltTime: Tue Sep 14 15:45:22 2021
  GitCommit: 23677f92dd83e96d2bc8f0acb611865fb8b1a56d
  GoVersion: go1.16.6
  OsArch: linux/amd64
  Version: 4.0.0
```

Run `podman info --format json` for a JSON formatted response:
```
$ podman info --format json
{
  "host": {
    "arch": "amd64",
    "buildahVersion": "1.23.0",
    "cgroupManager": "systemd",
    "cgroupVersion": "v2",
    "cgroupControllers": [],
    "conmon": {
      "package": "conmon-2.0.29-2.fc34.x86_64",
      "path": "/usr/bin/conmon",
      "version": "conmon version 2.0.29, commit: "
    },
    "cpus": 8,
    "distribution": {
      "distribution": "fedora",
      "version": "34"
    },
    "eventLogger": "journald",
    "hostname": "localhost.localdomain",
    "idMappings": {
      "gidmap": [
	{
	  "container_id": 0,
	  "host_id": 3267,
	  "size": 1
	},
	{
	  "container_id": 1,
	  "host_id": 100000,
	  "size": 65536
	}
      ],
      "uidmap": [
	{
	  "container_id": 0,
	  "host_id": 3267,
	  "size": 1
	},
	{
	  "container_id": 1,
	  "host_id": 100000,
	  "size": 65536
	}
      ]
    },
    "kernel": "5.13.13-200.fc34.x86_64",
    "logDriver": "journald",
    "memFree": 1785753600,
    "memTotal": 16401895424,
    "networkBackend": "netavark",
    "networkBackendInfo": {
      "backend": "netavark",
      "package": "netavark-1.0.1-1.fc34.x86_64",
      "path": "/usr/libexec/podman/netavark",
      "version": "netavark 1.0.1",
      "dns": {
        "version": "aardvark-dns 1.0.1",
        "package": "aardvark-dns-1.0.1-1.fc34.x86_64",
        "path": "/usr/libexec/podman/aardvark-dns"
      }
    },
    "ociRuntime": {
      "name": "crun",
      "package": "crun-1.0-1.fc34.x86_64",
      "path": "/usr/bin/crun",
      "version": "crun version 1.0\ncommit: 139dc6971e2f1d931af520188763e984d6cdfbf8\nspec: 1.0.0\n+SYSTEMD +SELINUX +APPARMOR +CAP +SECCOMP +EBPF +CRIU +YAJL"
    },
    "os": "linux",
    "rootlessPortForwarder": "rootlessport",
    "remoteSocket": {
      "path": "/run/user/3267/podman/podman.sock"
    },
    "serviceIsRemote": false,
    "security": {
      "apparmorEnabled": false,
      "capabilities": "CAP_CHOWN,CAP_DAC_OVERRIDE,CAP_FOWNER,CAP_FSETID,CAP_KILL,CAP_NET_BIND_SERVICE,CAP_SETFCAP,CAP_SETGID,CAP_SETPCAP,CAP_SETUID",
      "rootless": true,
      "seccompEnabled": true,
      "seccompProfilePath": "/usr/share/containers/seccomp.json",
      "selinuxEnabled": true
    },
    "pasta": {
      "executable": "/usr/bin/passt",
      "package": "passt-0^20221116.gace074c-1.fc34.x86_64",
      "version": "passt 0^20221116.gace074c-1.fc34.x86_64\nCopyright Red Hat\nGNU Affero GPL version 3 or later \u003chttps://www.gnu.org/licenses/agpl-3.0.html\u003e\nThis is free software: you are free to change and redistribute it.\nThere is NO WARRANTY, to the extent permitted by law.\n"
    },
    "swapFree": 15687475200,
    "swapTotal": 16886259712,
    "uptime": "47h 17m 29.75s (Approximately 1.96 days)",
    "linkmode": "dynamic"
  },
  "store": {
    "configFile": "/home/dwalsh/.config/containers/storage.conf",
    "containerStore": {
      "number": 9,
      "paused": 0,
      "running": 1,
      "stopped": 8
    },
    "graphDriverName": "overlay",
    "graphOptions": {

    },
    "graphRoot": "/home/dwalsh/.local/share/containers/storage",
    "graphStatus": {
      "Backing Filesystem": "extfs",
      "Native Overlay Diff": "true",
      "Supports d_type": "true",
      "Using metacopy": "false"
    },
    "imageCopyTmpDir": "/home/dwalsh/.local/share/containers/storage/tmp",
    "imageStore": {
      "number": 5
    },
    "runRoot": "/run/user/3267/containers",
    "volumePath": "/home/dwalsh/.local/share/containers/storage/volumes",
    "transientStore": false
  },
  "registries": {
    "search": [
  "registry.fedoraproject.org",
  "registry.access.redhat.com",
  "docker.io",
  "quay.io"
]
  },
  "plugins": {
    "volume": [
      "local"
    ],
    "network": [
      "bridge",
      "macvlan"
    ],
    "log": [
      "k8s-file",
      "none",
      "journald"
    ]
  },
  "version": {
    "APIVersion": "4.0.0",
    "Version": "4.0.0",
    "GoVersion": "go1.16.6",
    "GitCommit": "23677f92dd83e96d2bc8f0acb611865fb8b1a56d",
    "BuiltTime": "Tue Sep 14 15:45:22 2021",
    "Built": 1631648722,
    "OsArch": "linux/amd64"
  }
}
```

#### Extracting the list of container registries with a Go template

If shell completion is enabled, type `podman info --format={{.` and then press `[TAB]` twice.

```
$ podman info --format={{.
{{.Host.         {{.Plugins.      {{.Registries}}  {{.Store.        {{.Version.
```

Press `R` `[TAB]` `[ENTER]` to print the registries information.

```
$ podman info -f {{.Registries}}
map[search:[registry.fedoraproject.org registry.access.redhat.com docker.io quay.io]]
$
```

The output still contains a map and an array. The map value can be extracted with

```
$ podman info -f '{{index .Registries "search"}}'
[registry.fedoraproject.org registry.access.redhat.com docker.io quay.io]
```

The array can be printed as one entry per line

```
$ podman info -f '{{range index .Registries "search"}}{{.}}\n{{end}}'
registry.fedoraproject.org
registry.access.redhat.com
docker.io
quay.io

```

#### Extracting the list of container registries from JSON with jq

The command-line JSON processor [__jq__](https://stedolan.github.io/jq/) can be used to extract the list
of container registries.

```
$ podman info -f json | jq '.registries["search"]'
[
  "registry.fedoraproject.org",
  "registry.access.redhat.com",
  "docker.io",
  "quay.io"
]
```

The array can be printed as one entry per line

```
$ podman info -f json | jq -r '.registries["search"] | .[]'
registry.fedoraproject.org
registry.access.redhat.com
docker.io
quay.io
```

Note, the Go template struct fields start with upper case. When running `podman info` or `podman info --format=json`, the same names start with lower case.

## SEE ALSO
**[podman(1)](podman.1.md)**, **[containers-registries.conf(5)](https://github.com/containers/image/blob/main/docs/containers-registries.conf.5.md)**, **[containers-storage.conf(5)](https://github.com/containers/storage/blob/main/docs/containers-storage.conf.5.md)**
