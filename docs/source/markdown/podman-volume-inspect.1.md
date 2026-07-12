% podman-volume-inspect 1

## NAME
podman\-volume\-inspect - Get detailed information on one or more volumes

## SYNOPSIS
**podman volume inspect** [*options*] *volume* [...]

## DESCRIPTION

Display detailed information on one or more volumes. The output can be formatted using
the **--format** flag and a Go template. To get detailed information about all the
existing volumes, use the **--all** flag.
Volumes can be queried individually by providing their full name or a unique partial name.


## OPTIONS

#### **--all**, **-a**

Inspect all volumes.

#### **--format**, **-f**=*format*

Format volume output using Go template

Valid placeholders for the Go template are listed below:

| **Placeholder**     | **Description**                                                             |
| ------------------- | --------------------------------------------------------------------------- |
| .Anonymous          | Indicates whether volume is anonymous                                       |
| .CreatedAt ...      | Volume creation time                                                        |
| .Driver             | Volume driver                                                               |
| .GID                | GID the volume was created with                                             |
| .Labels ...         | Label information associated with the volume                                |
| .LockNumber         | Number of the volume's Libpod lock                                          |
| .MountCount         | Number of times the volume is mounted                                       |
| .Mountpoint         | Source of volume mount point                                                |
| .Name               | Volume name                                                                 |
| .NeedsChown         | Indicates volume will be chowned on next use                                |
| .NeedsCopyUp        | Indicates data at the destination will be copied into the volume on next use|
| .Options ...        | Volume options                                                              |
| .Scope              | Volume scope                                                                |
| .Status ...         | Status of the volume                                                        |
| .StorageID          | StorageID of the volume                                                     |
| .Timeout            | Timeout of the volume                                                       |
| .UID                | UID the volume was created with                                             |

#### **--help**

Print usage statement


## EXAMPLES

Inspect named volume.
```
$ podman volume inspect myvol
[
     {
          "Name": "myvol",
          "Driver": "local",
          "Mountpoint": "/home/myusername/.local/share/containers/storage/volumes/myvol/_data",
          "CreatedAt": "2023-03-13T16:26:48.423069028-04:00",
          "Labels": {},
          "Scope": "local",
          "Options": {},
          "MountCount": 0,
          "NeedsCopyUp": true,
          "NeedsChown": true
     }
]
```

Inspect all volumes.
```
$ podman volume inspect --all
[
     {
          "Name": "myvol",
          "Driver": "local",
          "Mountpoint": "/home/myusername/.local/share/containers/storage/volumes/myvol/_data",
          "CreatedAt": "2023-03-13T16:26:48.423069028-04:00",
          "Labels": {},
          "Scope": "local",
          "Options": {},
          "MountCount": 0,
          "NeedsCopyUp": true,
          "NeedsChown": true
     }
]
```

Inspect named volume and display its Driver and Scope field.
```
$ podman volume inspect --format "{{.Driver}} {{.Scope}}" myvol
local local
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-volume(1)](podman-volume.1.md)**, **[podman-inspect(1)](podman-inspect.1.md)**

## HISTORY
November 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
