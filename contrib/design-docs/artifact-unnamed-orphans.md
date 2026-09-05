# Change Request

## **Short Summary**

Handle unnamed orphan artifacts left in the local OCI artifact store after re-pulling an artifact whose content has changed. This currently breaks `podman artifact list` and makes `podman artifact rm` unreliable.

## **Objective**

When `podman artifact pull` re-pulls an artifact whose upstream content has changed (different digest), the OCI layout's `addManifest` logic strips the `org.opencontainers.image.ref.name` annotation from the old index entry and attaches it to the new one. The old manifest remains in `index.json` as an unnamed orphan. This causes two user-visible failures:

- `podman artifact list` calls `GetName()` on every artifact. An unnamed artifact returns `ErrArtifactUnnamed`, which aborts the entire listing. The user sees only `Error: artifact is unnamed` and zero output.
- `podman artifact rm` can find unnamed artifacts by digest prefix, but `Remove()` passes the empty `arty.Name` to `layout.NewReference`, which falls back to "only image in layout" mode. In a multi-artifact store this fails with `ErrMoreThanOneImage`.

The objective is to:

1. Prevent orphans on re-pull by requiring an explicit `--replace` flag to overwrite an existing artifact (consistent with `podman artifact add`).
2. Automatically clean up existing unnamed orphans when the artifact store is initialized.

## **Detailed Description:**

### How orphans are created on re-pull

The c/image OCI layout destination's `addManifest` function handles name collisions by stripping the annotation from the old entry:

1. User pulls `quay.io/foo:v1` (digest A). The index gets entry `{digest:A, name:"quay.io/foo:v1"}`.
2. Upstream pushes new content to `quay.io/foo:v1`.
3. User pulls `quay.io/foo:v1` again (digest B). `addManifest` strips the name from entry A, appends entry `{digest:B, name:"quay.io/foo:v1"}`. Entry A remains as an unnamed orphan.

When the digest is the same (no upstream change), `addManifest` reuses the existing entry, so no orphan is created.

### Change 1: Error on duplicate pull, add `--replace` flag

**Files:** `go.podman.io/common/pkg/libartifact/store.go` (`Pull()` method), `cmd/podman/artifact/pull.go`

Currently `Pull()` silently overwrites existing artifacts via c/image's `addManifest`, which strips the name from the old entry and creates an unnamed orphan. Instead, `Pull()` should check whether an artifact with the same name already exists and error with `ErrArtifactAlreadyExists`. When `--replace` is passed, the existing artifact is deleted before pulling the new one, consistent with `podman artifact add --replace`.

`--replace` should also succeed when the artifact does not exist yet, so scripts can always use `pull --replace` without checking first.

```go
func (as *ArtifactStore) Pull(ctx context.Context, ref ArtifactReference, opts PullOptions) (_ digest.Digest, pullErr error) {
    as.lock.Lock()
    defer as.lock.Unlock()

    _, lookupErr := as.lookupArtifactLocked(ctx, ref.ToArtifactStoreReference())

    switch {
    case lookupErr == nil && !opts.Replace:
        return "", fmt.Errorf("%s: %w", ref.String(), libartTypes.ErrArtifactAlreadyExists)
    case lookupErr == nil && opts.Replace:
        ir, err := layout.NewReference(as.storePath, ref.String())
        if err != nil {
            return "", err
        }
        if err := ir.DeleteImage(ctx, as.SystemContext); err != nil {
            return "", err
        }
    case !errors.Is(lookupErr, libartTypes.ErrArtifactNotExist):
        return "", lookupErr
    }

    // ... proceed with copy ...
}
```

### Change 2: Clean up orphans on store initialization

**File:** `go.podman.io/common/pkg/libartifact/store.go`, `NewArtifactStore()` function

Call `removeUnnamedArtifacts()` at the end of `NewArtifactStore()`, after the lock is acquired and the index file is verified. This handles orphans that already exist from older Podman versions or crashes that occurred before Changes 1 and 2 were in place.

The artifact store is initialized once per Podman invocation via `sync.OnceValues` in `libpod/runtime.go`, so this cleanup runs at most once per command.

```go
func NewArtifactStore(storePath string, sc *types.SystemContext) (*ArtifactStore, error) {
    // ... existing initialization ...

    if err := removeUnnamedArtifacts(context.TODO(), artifactStore); err != nil {
        return nil, err
    }

    return artifactStore, nil
}
```

### Helper: `removeUnnamedArtifacts()`

```go
func removeUnnamedArtifacts(ctx context.Context, as *ArtifactStore) error {
    for {
        lrs, err := layout.List(as.storePath)
        if err != nil {
            return err
        }
        found := false
        for _, l := range lrs {
            if _, ok := l.ManifestDescriptor.Annotations[specV1.AnnotationRefName]; ok {
                continue
            }
            if err := l.Reference.DeleteImage(ctx, as.SystemContext); err != nil {
                return err
            }
            found = true
            break
        }
        if !found {
            return nil
        }
    }
}
```

## **Use cases**

### Re-pulling an updated artifact

A user pulls `quay.io/my-team/model:latest`. The upstream team pushes a new version. The user runs `podman artifact pull quay.io/my-team/model:latest` again. Today, the old version remains as an unnamed orphan and `podman artifact ls` fails. After this fix, the pull errors with `artifact already exists`. The user runs `podman artifact pull --replace quay.io/my-team/model:latest`, which cleanly replaces the old artifact.


## **Target Podman Release**

No hard deadline. The changes are in containers/common (libartifact) and will be vendored into Podman after merging upstream.

## **Link(s)**

- [Issue #28033: Podman artifact pull leaves the previous local artifact around, without a name](https://github.com/podman-container-tools/podman/issues/28033)

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
- [x] Common library
- [ ] Netavark and aardvark-dns

## **Assignee(s)**

- jrodak @Honny1

## **Impacts**

### **CLI**

A new `--replace` flag is added to `podman artifact pull`. `--replace` succeeds even when the artifact does not exist yet (so scripts can always use `pull --replace`):

```
$ podman artifact pull quay.io/foo/bar:latest
Error: quay.io/foo/bar:latest: artifact already exists

$ podman artifact pull --replace quay.io/foo/bar:latest
Pulling quay.io/foo/bar:latest...
```

This is consistent with the existing `--replace` flag on `podman artifact add`.

### **Libpod**

No changes to core container management logic.

### **Others**

Store-level changes are in `go.podman.io/common/pkg/libartifact/`. After the containers/common PR is merged, Podman needs a vendor bump to pick up the fix. The `--replace` flag is added in `cmd/podman/artifact/pull.go`.

The `cleanupAfterAppend()` function can be simplified to use the new `removeUnnamedArtifacts()` helper, since it is a strict subset of the same logic. `cleanupAfterAppend` removes unnamed entries matching a specific digest, whereas the new helper removes all unnamed entries.

## **Test Descriptions (Optional):**

### Unit tests (containers/common)

- **`removeUnnamedArtifacts`**: Create an OCI layout with multiple unnamed entries. Call `removeUnnamedArtifacts`. Verify only named entries remain, blobs for orphans are cleaned up, and index re-listing after each deletion handles shifted indices correctly.
- **Store init cleanup**: Create an OCI layout with unnamed orphan entries, then call `NewArtifactStore`. Verify the orphans are removed after initialization.
- **`Pull` duplicate error**: Pull an artifact, then pull the same name again without `--replace`. Verify it returns `ErrArtifactAlreadyExists`.
- **`Pull` with replace**: Pull an artifact, then pull the same name with `Replace: true`. Verify the old artifact is removed, the new one is stored, and no unnamed entries remain.
- **`Pull` replace when not exists**: Call pull with `Replace: true` when artifact does not exist yet. Verify it succeeds normally.
- **`Pull` unexpected lookupErr**: Simulate a corrupted store that returns an unexpected error from `lookupArtifactLocked`. Verify the error is propagated, not silently ignored.

### e2e tests (containers/podman)

- **Pull duplicate error**: Pull an artifact, then pull the same name again. Verify the command exits with an error.
- **Pull with `--replace`**: Pull an artifact, push a new version to a local registry, pull again with `--replace`. Verify `podman artifact ls` shows only the new version.

No changes to CI images are required.
