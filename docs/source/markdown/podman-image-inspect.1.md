% podman-image-inspect 1

## NAME
podman\-image\-inspect - Display an image's configuration

## SYNOPSIS
**podman image inspect** [*options*] *image* [*image* ...]

## DESCRIPTION

This displays the low-level information on images identified by name or ID. By default, this renders all results in a JSON array.  If a format is specified, the given template is executed for each result.

## OPTIONS

#### **--format**, **-f**=*format*

Format the output using the given Go template.
The keys of the returned JSON can be used as the values for the --format flag (see examples below).

Valid placeholders for the Go template are listed below:

| **Placeholder**      | **Description**                                    |
| -----------------    | ------------------                                 |
| .Annotations ...     | Annotation information included in the image       |
| .Architecture        | Architecture of software in the image              |
| .Author              | Image author                                       |
| .Comment             | Image comment                                      |
| .Config ...          | Structure with config info                         |
| .Created ...         | Image creation time (string, ISO3601)              |
| .Digest              | Image digest (sha256:+64-char hash)                |
| .GraphDriver ...     | Structure for the graph driver info                |
| .HealthCheck ...     | Structure for the health check info                |
| .History             | History information stored in image                |
| .ID                  | Image ID (full 64-char hash)                       |
| .Labels ...          | Label information included in the image            |
| .ManifestType        | Manifest type of the image                         |
| .NamesHistory        | Name history information stored in image           |
| .Os                  | Operating system of software in the image          |
| .Parent              | Parent image of the specified image                |
| .RepoDigests         | Repository digests for the image                   |
| .RepoTags            | Repository tags for the image                      |
| .RootFS ...          | Structure for the root file system info            |
| .Size                | Size of image, in bytes                            |
| .User                | Default user to execute the image as               |
| .Version             | Image Version                                      |
| .VirtualSize         | Virtual size of image, in bytes                    |

## EXAMPLE

Inspect information on the specified image:
```
$ podman image inspect fedora
[
    {
        "Id": "37e5619f4a8ca9dbc4d6c0ae7890625674a10dbcfb76201399e2aaddb40da17d",
        "Digest": "sha256:1b0d4ddd99b1a8c8a80e885aafe6034c95f266da44ead992aab388e6aa91611a",
        "RepoTags": [
            "registry.fedoraproject.org/fedora:latest"
        ],
        "RepoDigests": [
            "registry.fedoraproject.org/fedora@sha256:1b0d4ddd99b1a8c8a80e885aafe6034c95f266da44ead992aab388e6aa91611a",
            "registry.fedoraproject.org/fedora@sha256:b5290db40008aae9272ad3a6bd8070ef7ecd547c3bef014b894c327960acc582"
        ],
        "Parent": "",
        "Comment": "Created by Image Factory",
        "Created": "2021-08-09T05:48:47Z",
        "Config": {
            "Env": [
                "DISTTAG=f34container",
                "FGC=f34",
                "container=oci"
            ],
            "Cmd": [
                "/bin/bash"
            ],
            "Labels": {
                "license": "MIT",
                "name": "fedora",
                "vendor": "Fedora Project",
                "version": "34"
            }
        },
        "Version": "1.10.1",
        "Author": "",
        "Architecture": "amd64",
        "Os": "linux",
        "Size": 183852302,
        "VirtualSize": 183852302,
        "GraphDriver": {
            "Name": "overlay",
            "Data": {
                "UpperDir": "/home/dwalsh/.local/share/containers/storage/overlay/0203e243f1ca4b6bb49371ecd21363212467ec6d7d3fa9f324cd4e78cc6b5fa2/diff",
                "WorkDir": "/home/dwalsh/.local/share/containers/storage/overlay/0203e243f1ca4b6bb49371ecd21363212467ec6d7d3fa9f324cd4e78cc6b5fa2/work"
            }
        },
        "RootFS": {
            "Type": "layers",
            "Layers": [
                "sha256:0203e243f1ca4b6bb49371ecd21363212467ec6d7d3fa9f324cd4e78cc6b5fa2"
            ]
        },
        "Labels": {
            "license": "MIT",
            "name": "fedora",
            "vendor": "Fedora Project",
            "version": "34"
        },
        "Annotations": {},
        "ManifestType": "application/vnd.docker.distribution.manifest.v2+json",
        "User": "",
        "History": [
            {
                "created": "2021-08-09T05:48:47Z",
                "comment": "Created by Image Factory"
            }
        ],
        "NamesHistory": [
            "registry.fedoraproject.org/fedora:latest"
        ]
    }
]
```

Inspect image ID for the specified image:
```
$ podman image inspect --format '{{ .Id }}' fedora
37e5619f4a8ca9dbc4d6c0ae7890625674a10dbcfb76201399e2aaddb40da17d
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-image(1)](podman-image.1.md)**, **[podman-inspect(1)](podman-inspect.1.md)**

## HISTORY
Sep 2021, Originally compiled by Dan Walsh <dwalsh@redhat.com>
