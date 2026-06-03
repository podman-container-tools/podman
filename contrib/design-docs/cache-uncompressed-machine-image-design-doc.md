# Change Request

## **Short Summary**

Speed up `podman machine init` by keeping a decompressed base image in the cache directory (created at pull time), so subsequent inits copy/clone the base instead of re-decompressing. Introduce `podman machine rm --cache` to handle cleanups and cache rotation.

## **Objective**

`podman machine init` is slow because reasons, one of which is that the VM disk image is decompressed on every invocation, even when the compressed image is already cached locally. When a user does `podman machine rm` + `podman machine init`, the decompressed disk is destroyed and recreated from scratch.

This proposal eliminates redundant decompression by caching the decompressed base image alongside the compressed blob, drastically reducing warm init times. Also introduces future improvements like a `podman machine pull` command that could further improve the init times, or at least move the logic to a phase of initialization where the user might expect longer times, so during pull.

## **Detailed Description:**

### High-level approach

- Decompress the image at pull time and save it next to the compressed blob in the cache directory.
- Add a new `podman machine rm --cache` flag to allow explicit cache pair removal.
- Add an `ImageDigest` field to `MachineConfig` to track provenance and enable refcounting.

```
Registry (quay.io/podman/machine-os:5.4)
    |
    | pull (identified by artifact digest)
    v
Compressed blob: cache/{digest}.{format}.zst
    |
    | decompress (at init time)
    v
Decompressed base: cache/{digest}.{format}
    |
    | copy/clone (at init time)
    v
Per-machine image: {datadir}/{name}-{arch}.{format}
```

Cache is now a pair of compressed and decompressed files.

- `cache/{digest}.{format}.zst`: Compressed blob pulled from registry
- `cache/{digest}.{format}`: Decompressed base, ready to copy

### Behavior: Old vs New

#### `podman machine init`

Here the logic is simple. During init decompress cache and save it as a separate file in `cache` dir. If no compressed file found, pull and decompress. If no decompressed file found, decompress only.


#### `podman machine rm`

Deletes machine and init, preserves cache. Use `--cache` to wipe cache pair.

| Command                           | Image   | Ignition | Cache pair                                                |
| --------------------------------- | ------- | -------- | --------------------------------------------------------- |
| `machine rm` (current)            | Deleted | Deleted  | Kept                                                      |
| `machine rm` (proposed)           | Deleted | Deleted  | Kept                                                      |
| `machine rm --cache` (new)        | Deleted | Deleted  | Deleted (unless refcount > 0; use `--force` to override)  |
| `machine rm --cache` (VM gone)    | N/A     | N/A      | All orphan cache files listed and deleted on confirmation |
| `machine rm --save-image`         | Kept    | Deleted  | Kept                                                      |
| `machine rm --save-ignition`      | Deleted | Kept     | Kept                                                      |
| `machine rm --cache --save-image` | Kept    | Deleted  | Deleted (unless refcount > 0; use `--force` to override)  |
| `machine reset`                   | Deleted | Deleted  | Deleted                                                   |

When `--cache` is used, a new confirmation prompt shows cache files in a dedicated section:

```
$ podman machine rm --cache
The following files will be deleted:

podman-machine-default.json
podman-machine-default.sock
...

The following cache files will be deleted:

91d1e51d...qcow2.zst
91d1e51d...qcow2
Are you sure you want to continue? [y/N] y
```

When the machine does not exist but `--cache` is specified, cache files are still listed and removed:

```
$ podman machine rm --cache
podman-machine-default: VM does not exist

The following cache files will be deleted:

91d1e51d...qcow2.zst
91d1e51d...qcow2
Are you sure you want to continue? [y/N] y
```

If no cache files exist, the output is:

```
$ podman machine rm --cache
podman-machine-default: VM does not exist
No cache files to remove.
```

#### Cache rotation (during `podman machine init`)

| Step                  | Current                            | Proposed                                                               |
| --------------------- | ---------------------------------- | ---------------------------------------------------------------------- |
| Trigger               | Cache miss (new version available) | Same                                                                   |
| Snapshot              | `os.ReadDir(cache/)` before pull   | Same                                                                   |
| Pull new              | Download new `.zst`                | Same                                                                   |
| Clean old             | Wipe all snapshotted files         | Same (now also wipes old decompressed base since it lives in same dir) |
| New decompressed base | N/A                                | Created after new `.zst` is pulled (before old files are cleaned)      |

#### `podman machine reset`

|        | Current                                  | Proposed         |
| ------ | ---------------------------------------- | ---------------- |
| Effect | Wipes entire data dir, config dir, cache | Same (no change) |

### Config change

Add `ImageDigest` field to `MachineConfig`:

```go
type MachineConfig struct {
    // ... existing fields ...
    ImageDigest string `json:"ImageDigest,omitempty"`
}
```

Set during init from the resolved OCI artifact digest. Used by:

- `rm`: to locate cache files (`cache/{digest}.*`) and do refcount check
- `init`: to verify cached base is current

### Provider considerations

| Provider | Format | Reflink support                | Benefit                                                                      |
| -------- | ------ | ------------------------------ | ---------------------------------------------------------------------------- |
| AppleHV  | .raw   | APFS clonefile (near-instant)  | High                                                                         |
| LibKrun  | .raw   | APFS clonefile                 | High                                                                         |
| HyperV   | .vhdx  | NTFS: no reflink, regular copy | High (copy still faster than decompress)                                     |
| QEMU     | .qcow2 | btrfs/xfs reflink              | Medium (qcow2 is smaller)                                                    |
| WSL      | .tar   | N/A (used for wsl --import)    | Medium (caches decompressed tarball, skips download+decompress on re-import) |

### Key implementation files

| File                                    | Change                                                                                                      |
| --------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `pkg/machine/ocipull/ociartifact.go`    | After pull+unpack, decompress to cache (not per-machine path). On cache hit, return decompressed base path. |
| `pkg/machine/shim/host.go`              | Init: copy/clone decompressed base to `mc.ImagePath`. Rm: add cache deletion with refcount check.           |
| `pkg/machine/shim/diskpull/diskpull.go` | Route to copy-from-cache when decompressed base exists                                                      |
| `pkg/machine/vmconfigs/config.go`       | Add `ImageDigest` field                                                                                     |
| `pkg/machine/vmconfigs/machine.go`      | `Remove()`: add cache pair deletion logic with refcount                                                     |
| `cmd/podman/machine/rm.go`              | Add `--cache` flag                                                                                          |
| `pkg/machine/config.go`                 | Add `Cache` (or `RemoveCache`) to `RemoveOptions`                                                           |

## **Use cases**

- **Repeated init/rm cycle**: A developer that would destroy and recreate machines during testing would benefit for short `init` times. With the decompressed base cached, `podman machine init` completes in few seconds.
- **Better cache management**: A user has a better control to reclaim disk space and manage machine files with a dedicated `--cache` flag

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

- New `--cache` flag on `podman machine rm` to force deletion of the cache pair (compressed + decompressed).
- Confirmation prompt updated to show cache files in a dedicated section when `--cache` is used.
- No changes to `podman machine init` CLI interface; the optimization is transparent.

### **Libpod**

- New `ImageDigest` field in `MachineConfig` to track image provenance.
- Add `Cache` field to `RemoveOptions` to support `--cache`.

## **Further Description (Optional):**

### Future improvements

- Add `podman machine pull` command that will pull and decompress the cache. Options could be:
  - `podman machine pull` — new command
  - `podman machine pull --no-decompress-cache` — pull only (if default is to pull and decompress)
  - `podman machine pull --decompress-cache` — pull and decompress (if default is pull only)

## **Test Descriptions (Optional):**

<!-- How will this feature be tested? -->
