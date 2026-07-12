% podman-artifact-inspect 1

## NAME
podman\-artifact\-inspect - Inspect an OCI artifact

## SYNOPSIS
**podman artifact inspect** *name*

## DESCRIPTION

Inspect an artifact in the local store and output the results in JSON format.
The artifact can be referred to with either:

1. Fully qualified artifact name
2. Full or partial digest of the artifact's manifest

The inspect output includes the artifact manifest with annotations. All artifacts
automatically include a creation timestamp in the `org.opencontainers.image.created`
annotation using RFC3339Nano format, showing when the artifact was initially created.

## OPTIONS

#### **--format**, **-f**=*format*

Format the output using the given Go template.
The keys of the returned JSON can be used as the values for the --format flag (see examples below).

Valid placeholders for the Go template are listed below:

| **Placeholder**  | **Description**                        |
|------------------|----------------------------------------|
| .Artifact ...    | Artifact details (nested struct)       |
| .Digest          | Artifact digest (sha256:+64-char hash) |
| .Manifest ...    | Artifact manifest details (struct)     |
| .Name            | Artifact name (string)                 |
| .TotalSizeBytes  | Total Size of the artifact in bytes    |

## EXAMPLES

Inspect an OCI image in the local store.

```shell
$ podman artifact inspect quay.io/myartifact/mytxt:latest
{
     "Manifest": {
          "schemaVersion": 2,
          "mediaType": "application/vnd.oci.image.manifest.v1+json",
          "config": {
               "mediaType": "application/vnd.oci.empty.v1+json",
               "digest": "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
               "size": 2,
               "data": "e30="
          },
          "layers": [
               {
                    "mediaType": "text/plain; charset=utf-8",
                    "digest": "sha256:f2ca1bb6c7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2",
                    "size": 5,
                    "annotations": {
                         "org.opencontainers.image.title": "foobar.txt"
                    }
               }
          ]
     },
     "Name": "quay.io/myartifact/mytxt:latest",
     "Digest": "sha256:6c28fa07a5b0a1cee29862c1f6ea38a66df982495b14da2c052427eb628ed8c6"
}
```

Inspect artifact digest for the specified artifact:

```shell
$ podman artifact inspect quay.io/myartifact/mytxt:latest --format {{.Digest}}
sha256:6c28fa07a5b0a1cee29862c1f6ea38a66df982495b14da2c052427eb628ed8c6
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-artifact(1)](podman-artifact.1.md)**

## HISTORY
Sept 2024, Originally compiled by Brent Baude <bbaude@redhat.com>
