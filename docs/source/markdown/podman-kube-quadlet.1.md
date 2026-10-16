% podman-kube-quadlet 1

## NAME
podman\-kube\-quadlet - Convert a Kubernetes Pod YAML into Quadlet unit files

## SYNOPSIS
**podman kube quadlet** [*options*] *pod-yaml*

## DESCRIPTION
**podman kube quadlet** reads a Kubernetes Pod YAML file and generates the
corresponding Quadlet unit files (`.pod`, `.container`, `.volume`, `.env`,
`.sh`) without connecting to a Podman daemon. The generated files can be placed
in a systemd user unit directory (e.g. `~/.config/containers/systemd/`) so that
**podman-systemd-generator(8)** starts and manages the pod as a systemd service.

Each container in the Pod becomes a `.container` unit that joins the `.pod` unit
via `Pod=`. Init containers are emitted before regular containers and are given
`Restart=no`. Volume sources are mapped as follows:

- `PersistentVolumeClaim` → `Volume=` (or `Mount=type=volume,subpath=...` when SubPath is set)
- `EmptyDir` → a named tmpfs volume shared across all containers in the pod
- `HostPath` → `Volume=` with `:z` SELinux relabeling
- `Image` → `Mount=type=image`

## OPTIONS

#### **--configmap**=*path*

Path to a Kubernetes ConfigMap YAML file. May be specified multiple times.
ConfigMap volumes in the Pod spec are written as read-only bind-mount directories
containing the key/value pairs.

#### **--format**=**text** | **json**

Output format. **text** (default) writes unit files to the directory given by
**--output-dir** or, if omitted, prints them to standard output. **json** emits
a JSON array where each element has `name` (filename) and `content` (unit file
text) fields; useful for programmatic consumption.

#### **--name-prefix**=*prefix*

Override the name prefix used for all generated unit files. Defaults to the Pod
metadata name.

#### **--network**=*network*

Network to attach the pod to (passed as `Network=` in the `.pod` unit).

#### **--output-dir**, **-o**=*dir*

Directory to write the generated unit files into. If omitted, files are printed
to standard output (one after another, each preceded by a `# filename` comment).

#### **--script-dir**=*dir*

Directory in which companion `.sh` scripts are written when a container's
command is a shell one-liner (`/bin/sh -c <script>`). Defaults to the same
directory as **--output-dir**.

#### **--secret**=*path*

Path to a Kubernetes Secret YAML file. May be specified multiple times.

## EXAMPLES

Convert a pod YAML and write unit files to a systemd user directory.
```
$ podman kube quadlet --output-dir ~/.config/containers/systemd/myapp pod.yaml
```

Convert and emit JSON for programmatic consumption.
```
$ podman kube quadlet --format json pod.yaml | jq '.[].name'
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-kube(1)](podman-kube.1.md)**, **[podman-kube-play(1)](podman-kube-play.1.md)**, **[podman-systemd.unit(5)](podman-systemd.unit.5.md)**

## HISTORY
June 2026, Originally compiled by Asaf Ben Natan (asafbennatan at gmail dot com)
