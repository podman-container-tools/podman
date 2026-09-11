# Change Request

## **Short Summary**

Speed up `podman machine init` by caching the uncompressed base image so subsequent inits copy instead of re-decompressing. Introduce `podman machine cache` for cache inspection and cleanup.

## **Objective**

`podman machine init` decompresses the VM disk image on every invocation, even when the image is already cached locally. A `podman machine rm` + `podman machine init` cycle destroys and recreates the disk from scratch. This proposal eliminates redundant decompression by caching the uncompressed image at pull time.

## **Detailed Description:**

### High-level approach

- Download and decompress the image at pull time; store only the uncompressed result in the cache directory (discard the compressed blob).
- On `podman machine init`, copy the cached base image to the per-machine path.
- Add a new `podman machine cache` subcommand for cache inspection and cleanup.

```
Registry (quay.io/podman/machine-os:5.4)
    |
    | pull + decompress
    v
cache/{digest}.{format}        (uncompressed base, ready to copy)
    |
    | copy
    v
{datadir}/{name}.{format}      (per-machine disk)
```

### `podman machine init` behavior

On cache hit (uncompressed base exists for the requested image): copy the cached file to the per-machine path. On cache miss: pull from registry, decompress, store in cache, then copy to per-machine path.

### `podman machine rm` behavior

No change. `podman machine rm` deletes the per-machine disk and config but does not touch the cache. Since each machine holds an independent copy, removing a machine has no effect on the cached base image or other machines.

### `podman machine cache` (new subcommand)

Output follows the same tabular style as `podman machine list` (uppercase headers, `--format` for Go templates).

| Command | Effect |
| --- | --- |
| `podman machine cache` | List cached base images (digest, format, size) |
| `podman machine cache --remove` | Delete all cached base images after confirmation |
| `podman machine cache --format` | Format output using a Go template |

Example output:

```
$ podman machine cache
DIGEST          FORMAT  SIZE
91d1e51d...     raw     4.2 GiB
```

```
$ podman machine cache --remove
The following cache files will be deleted:

DIGEST          FORMAT  SIZE
91d1e51d...     raw     4.2 GiB

Are you sure you want to continue? [y/N] y
```

### Cache rotation (during `podman machine init`)

When a cache miss occurs (new version pulled), old cached files are removed after the new image is downloaded and decompressed. This keeps at most one base image in the cache.

### Copy strategy

Init performs a full copy of the cached base image to the per-machine path. This ensures predictable `podman machine start` performance with no CoW overhead on first boot.

## **Use cases**

- **Repeated init/rm cycle**: A developer destroying and recreating machines during testing gets fast `init` times (seconds instead of minutes) since decompression is skipped.
- **Disk space management**: `podman machine cache --remove` gives users explicit control to reclaim cache space.

## **Target Podman Release**

After Podman 6

## **Link(s)**

- [RUN-4473](https://redhat.atlassian.net/browse/RUN-4473) — Jira tracking issue

## **Stakeholders**

- [x] Podman Users
- [x] Podman Developers
- [ ] Buildah Users
- [ ] Buildah Developers
- [ ] Skopeo Users
- [ ] Skopeo Developers
- [x] Podman Desktop
- [ ] CRI-O
- [ ] Storage library
- [ ] Image library
- [ ] Common library
- [ ] Netavark and aardvark-dns

## ** Assignee(s) **

@inknos

## **Impacts**

### **CLI**

- New `podman machine cache` subcommand for viewing and removing cached base images.
- No changes to `podman machine init` or `podman machine rm` CLI interfaces; the optimization is transparent to users.

### **Libpod**

- No config schema changes required. The cache is a filesystem-level optimization with no new fields in `MachineConfig`.
