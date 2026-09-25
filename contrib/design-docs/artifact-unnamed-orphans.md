# Change Request

## **Short Summary**

Handle unnamed orphan artifacts left in the local OCI artifact store after re-pulling an artifact whose content has changed. This currently breaks `podman artifact list` and makes `podman artifact rm` unreliable.

## **Objective**

When `podman artifact pull` re-pulls an artifact whose upstream content has changed (different digest), the OCI layout's `addManifest` logic strips the `org.opencontainers.image.ref.name` annotation from the old index entry and attaches it to the new one. The old manifest remains in `index.json` as an unnamed orphan. This causes two user-visible failures:

- `podman artifact list` calls `GetName()` on every artifact. An unnamed artifact returns `ErrArtifactUnnamed`, which aborts the entire listing. The user sees only `Error: artifact is unnamed` and zero output.
- `podman artifact rm` can find unnamed artifacts by digest prefix, but `Remove()` passes the empty `arty.Name` to `layout.NewReference`, which falls back to "only image in layout" mode. In a multi-artifact store this fails with `ErrMoreThanOneImage`.

Two options are proposed below. Both share a common cleanup for legacy orphans (Change C1). **After discussion, Option A/B (TODO update after votes) was chosen as the implementation approach.**

### Design context

The artifact store was intentionally designed to be simpler than the image store. It does not have the same layered complexity (image IDs, deduplication, dangling images, build cache). This simplicity is a feature. However, the current re-pull behavior creates unnamed orphans that break the store, so the re-pull path needs to be fixed.

[ORAS (OCI Registry As Storage)](https://oras.land/docs/concepts/artifact/) defines OCI artifacts as content-addressable objects referenced by a **tag** or a **digest**. Tags are mutable pointers; digests are immutable content hashes. When a tag is pushed to a new digest, the old digest remains in the registry, accessible by its `repo@sha256:...` reference. Cleanup of untagged content is left to garbage collection or explicit user action. The Podman artifact store does not need to follow this model, but it provides useful context for understanding Option B.

## **Detailed Description:**

### How orphans are created on re-pull

The c/image OCI layout destination's `addManifest` function handles name collisions by stripping the annotation from the old entry:

1. User pulls `quay.io/foo:v1` (digest A). The index gets entry `{digest:A, name:"quay.io/foo:v1"}`.
2. Upstream pushes new content to `quay.io/foo:v1`.
3. User pulls `quay.io/foo:v1` again (digest B). `addManifest` strips the name from entry A, appends entry `{digest:B, name:"quay.io/foo:v1"}`. Entry A remains as an unnamed orphan.

When the digest is the same (no upstream change), `addManifest` reuses the existing entry, so no orphan is created.

---

## Option A: Require `--replace` flag

Error when pulling an artifact that already exists locally. Add a `--replace` flag to explicitly delete the old artifact before pulling the new one, consistent with `podman artifact add --replace`.

### A1: Error on duplicate pull, add `--replace` flag

**Files:** `go.podman.io/common/pkg/libartifact/store.go` (`Pull()` method), `cmd/podman/artifact/pull.go`

`Pull()` checks whether an artifact with the same name already exists. If it does, it returns `ErrArtifactAlreadyExists`. When `--replace` is passed, the existing artifact is deleted before pulling the new one. `--replace` also succeeds when the artifact does not exist yet, so scripts can always use `pull --replace`.

Before deleting, `Pull()` must check whether the artifact is currently mounted into any container. If it is, the replace must fail with a clear error (e.g., `"cannot replace: artifact is in use by container X"`). This is analogous to how `podman rmi` refuses to delete images used by running containers.

```go
func (as *ArtifactStore) Pull(ctx context.Context, ref ArtifactReference, opts PullOptions) (_ digest.Digest, pullErr error) {
    as.lock.Lock()
    defer as.lock.Unlock()

    existing, lookupErr := as.lookupArtifactLocked(ctx, ref.ToArtifactStoreReference())

    switch {
    case lookupErr == nil && !opts.Replace:
        return "", fmt.Errorf("%s: %w", ref.String(), libartTypes.ErrArtifactAlreadyExists)
    case lookupErr == nil && opts.Replace:
        // Guard: refuse to replace an artifact that is mounted in a container
        if err := as.checkNotMounted(ctx, existing); err != nil {
            return "", err
        }
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

**User experience:**

```
$ podman artifact pull quay.io/foo/bar:latest
# ... first pull succeeds ...

$ podman artifact pull quay.io/foo/bar:latest
Error: quay.io/foo/bar:latest: artifact already exists

$ podman artifact pull --replace quay.io/foo/bar:latest
Pulling quay.io/foo/bar:latest...

$ podman artifact ls
REPOSITORY          TAG      DIGEST        SIZE
quay.io/foo/bar     latest   ccc333ddd444  12MB
```

**Pros:**
- No dangling artifacts, store stays clean and simple.
- Consistent with `podman artifact add --replace` and other create commands (`podman create --name`, `podman network create`, `podman volume create`, `podman pod create`) that error on existing resources unless `--replace` is given.
- Simple implementation.

**Cons:**
- Breaks users who script `podman artifact pull` without `--replace`. Scripts and automations would most likely always use `--replace`.
- Not consistent with `podman pull` (images), which silently moves the tag.
- Old content is gone after replace, no way to rollback.
- The `--replace` flag name may be misleading on `pull`: users could interpret it as "replace the tag only," but it also deletes the old artifact data. Alternative names: `--overwrite` (unambiguously implies data replacement, no existing Podman precedent so no confusion with other flags) or `--force` (widely understood but in Podman `--force` is used for removal commands, not pull/create).

---

## Option B: Tag-stripping (image store model)

Silently retag old entries with a digest reference after pull, preserving the repository name. This follows the same model as `podman pull` for images: when a tag moves, the old content stays with a digest-only reference.

### B1: Retag orphaned entries after pull

**File:** `go.podman.io/common/pkg/libartifact/store.go`, `Pull()` method

After `copyArtifact` completes, c/image's `addManifest` has already stripped the name from any old entry with the same tag. The fix scans `index.json` for unnamed entries and sets their `AnnotationRefName` to a digest reference (`repo@sha256:...`), preserving the repository name from the pull reference.

This is consistent with how `podman artifact pull repo@sha256:...` already stores artifacts today. The result is an entry that has a repository name but no tag.

```go
func (as *ArtifactStore) Pull(ctx context.Context, ref ArtifactReference, opts libimage.CopyOptions) (_ digest.Digest, pullErr error) {
    srcRef, err := docker.NewReference(ref.ref)
    if err != nil {
        return "", err
    }
    artifactDigest, err := as.withLockedLayout(ref.String(), func(localRef types.ImageReference) (digest.Digest, error) {
        return as.copyArtifact(ctx, srcRef, localRef, opts)
    })
    if err != nil {
        return "", err
    }

    // Retag any unnamed orphans with a digest reference,
    // preserving the repository name from the pull ref.
    if err := as.retagUnnamedEntries(ref); err != nil {
        return "", err
    }

    // ... event ...
    return artifactDigest, nil
}
```

The `retagUnnamedEntries` helper reads `index.json`, finds entries without `AnnotationRefName`, sets the annotation to `repo@sha256:<digest>`, then writes `index.json` back:

```go
func (as *ArtifactStore) retagUnnamedEntries(ref ArtifactReference) error {
    indexPath := as.indexPath()
    rawData, err := os.ReadFile(indexPath)
    if err != nil {
        return err
    }
    var index specV1.Index
    if err := json.Unmarshal(rawData, &index); err != nil {
        return err
    }

    repo := reference.TrimNamed(ref.ref).String()
    modified := false
    for i, desc := range index.Manifests {
        if _, ok := desc.Annotations[specV1.AnnotationRefName]; ok {
            continue
        }
        digestRef := repo + "@" + desc.Digest.String()
        if index.Manifests[i].Annotations == nil {
            index.Manifests[i].Annotations = make(map[string]string)
        }
        index.Manifests[i].Annotations[specV1.AnnotationRefName] = digestRef
        modified = true
    }

    if !modified {
        return nil
    }

    rawData, err = json.Marshal(&index)
    if err != nil {
        return err
    }
    return os.WriteFile(indexPath, rawData, 0o644)
}
```

**User experience:**

```
$ podman artifact pull quay.io/foo/bar:latest
# ... first pull ...

$ podman artifact pull quay.io/foo/bar:latest
# ... upstream changed, tag moves to new digest ...

$ podman artifact ls
REPOSITORY          TAG      DIGEST        CREATED        SIZE
quay.io/foo/bar     latest   ccc333ddd444  1 second ago   12MB
quay.io/foo/bar              aaa111bbb222  5 minutes ago  10MB

$ podman artifact rm aaa111bbb222
aaa111bbb222
```

**Pros:**
- Consistent with `podman pull` (images).
- No breaking change for scripts, pull always succeeds.
- Old content preserved, user can rollback or inspect.
- No new flags to learn.

**Cons:**
- Moves the artifact store closer to the image store model, which was intentionally avoided. This introduces complexity (dangling artifacts, pruning) that the artifact store was designed to not have.
- Untagged artifacts accumulate over time, need future `podman artifact prune`.
- Slightly more complex implementation (direct `index.json` editing).
- Users who pull `repo:tag` probably want exactly that, not `repo@digest`. They would accumulate old versions they likely do not want.

### B2: Handle untagged artifacts in `artifact list`

**File:** `cmd/podman/artifact/list.go`

When parsing a digest reference like `repo@sha256:...`, `reference.Parse` returns a `Named` reference where `named.Name()` gives the repository and the reference is not `Tagged`, so tag stays empty. This already works.

For legacy unnamed entries (from before any fix), handle `ErrArtifactUnnamed` gracefully:

```go
artifactName, err := lr.Artifact.GetName()
if err != nil {
    if !errors.Is(err, libartTypes.ErrArtifactUnnamed) {
        return err
    }
    repoName = "<none>"
    tag = "<none>"
} else {
    // ... existing reference parsing ...
}
```

### B3: Fix `Remove()` for untagged artifacts

**File:** `go.podman.io/common/pkg/libartifact/store.go`, `Remove()` method

After B1, old artifacts have a name (`repo@sha256:digest`), so `Remove()` works via the digested lookup path in `lookupArtifactLocked`. For legacy unnamed entries, `Remove()` falls back to finding the entry by digest in `layout.List`:

```go
func (as *ArtifactStore) layoutRefForArtifact(ctx context.Context, arty *Artifact) (types.ImageReference, error) {
    if arty.Name != "" {
        return layout.NewReference(as.storePath, arty.Name)
    }
    // Fallback for unnamed entries: find by digest in the layout
    lrs, err := layout.List(as.storePath)
    if err != nil {
        return nil, err
    }
    for _, l := range lrs {
        if l.ManifestDescriptor.Digest.String() == arty.Digest.String() {
            return l.Reference, nil
        }
    }
    return nil, fmt.Errorf("%s: %w", arty.Digest, libartTypes.ErrArtifactNotExist)
}
```

### Future work: `podman artifact prune`

With Option B, old artifact digests accumulate over time. A future `podman artifact prune` command should be added to remove untagged artifacts collectively, analogous to `podman image prune`. This is not part of this change.

---

## Common Change: Clean up existing unnamed orphans on store initialization (C1)

**File:** `go.podman.io/common/pkg/libartifact/store.go`, `NewArtifactStore()` function

Both options need this. Unnamed orphans that already exist from older Podman versions cannot be retagged (their original repository name is unknown). Delete them on store initialization.

After deleting one entry, numerical indices shift and remaining references become stale. The helper re-lists after each deletion:

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

---

## Common concern: Mounted artifacts

Both options must handle the case where an artifact is currently mounted into a container via `podman artifact mount`. In Option A, `--replace` must refuse to delete a mounted artifact. In Option B, this is less critical since the old artifact stays, but interactions between mounted artifacts and their references still need consideration (e.g., what happens when an artifact mounted by tag is re-pulled and the tag moves to a new digest).

---

## **Use cases**

### Re-pulling an updated artifact (Option A)

A user pulls `quay.io/my-team/model:latest`. The upstream team pushes a new version. The user runs `podman artifact pull quay.io/my-team/model:latest` again. The pull errors with `artifact already exists`. The user runs `podman artifact pull --replace quay.io/my-team/model:latest`, which cleanly replaces the old artifact.

### Re-pulling an updated artifact (Option B)

A user pulls `quay.io/my-team/model:latest`. The upstream team pushes a new version. The user runs `podman artifact pull quay.io/my-team/model:latest` again. The tag moves to the new digest. The old digest stays in the store with the repository name but no tag. `podman artifact ls` shows both. The user can remove the old one with `podman artifact rm <digest-prefix>`.

### Pulling by digest (both options)

A user pulls `quay.io/my-team/model@sha256:abc123...`. The artifact is stored with repository name `quay.io/my-team/model` and no tag. This is the same state as an artifact that lost its tag in Option B. Both are handled identically.

## **Target Podman Release**

No hard deadline. The changes span containers/common (libartifact) and containers/podman (CLI). After the containers/common PR is merged, Podman needs a vendor bump.

## **Link(s)**

- [Issue #28033: Podman artifact pull leaves the previous local artifact around, without a name](https://github.com/podman-container-tools/podman/issues/28033)
- [OCI Artifact Specification (ORAS)](https://oras.land/docs/concepts/artifact/)

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

**Option A:** A new `--replace` flag is added to `podman artifact pull`. Without it, re-pulling an existing artifact errors. This is consistent with `podman artifact add --replace` and other Podman create commands that require `--replace` to overwrite existing resources.

**Option B:** No new flags. `podman artifact pull` silently retags old entries when the tag moves to a new digest. `podman artifact ls` shows old artifacts with repository name and empty tag. `podman artifact rm` works by digest prefix.

### **Libpod**

No changes to core container management logic.

### **Others**

Store-level changes are in `go.podman.io/common/pkg/libartifact/`. CLI changes are in `cmd/podman/artifact/` (pull.go for Option A, list.go for Option B). After the containers/common PR is merged, Podman needs a vendor bump.

## **Test Descriptions (Optional):**

### Unit tests (containers/common)

**Common:**
- **`removeUnnamedArtifacts`**: Create an OCI layout with multiple unnamed entries. Call `removeUnnamedArtifacts`. Verify entries are deleted one at a time with re-listing after each deletion.
- **Store init cleanup**: Create an OCI layout with unnamed orphan entries, then call `NewArtifactStore`. Verify the orphans are removed.

**Option A:**
- **`Pull` duplicate error**: Pull an artifact, then pull the same name again without `--replace`. Verify it returns `ErrArtifactAlreadyExists`.
- **`Pull` with replace**: Pull an artifact, then pull the same name with `Replace: true`. Verify the old artifact is removed, the new one is stored, and no unnamed entries remain.
- **`Pull` replace when not exists**: Call pull with `Replace: true` when artifact does not exist yet. Verify it succeeds normally.
- **`Pull` replace mounted artifact**: Pull an artifact, mount it, then pull with `Replace: true`. Verify it errors with a "in use" message.

**Option B:**
- **`retagUnnamedEntries`**: Create an OCI layout with unnamed entries. Call `retagUnnamedEntries`. Verify entries get `repo@sha256:digest` as their `AnnotationRefName`.
- **`Pull` re-pull with changed content**: Pull artifact A under name `repo:tag`. Pull again with different content. Verify old entry is retagged to `repo@sha256:<old-digest>` and new entry has `repo:tag`.
- **`Pull` re-pull same content**: Pull artifact A under name `repo:tag`. Pull again with same content. Verify no orphans or duplicates.
- **`Remove` by digest reference**: Look up an artifact by `repo@sha256:digest`. Call `Remove`. Verify it succeeds.
- **`Remove` by digest prefix**: Look up an untagged artifact by hex prefix. Call `Remove`. Verify it succeeds.

### e2e tests (containers/podman)

**Option A:**
- **Pull duplicate error**: Pull an artifact, then pull the same name again. Verify the command exits with an error.
- **Pull with `--replace`**: Pull an artifact, push a new version to a local registry, pull again with `--replace`. Verify `podman artifact ls` shows only the new version.

**Option B:**
- **Pull re-pull**: Pull an artifact, push a new version to a local registry, pull again. Verify `podman artifact ls` shows both entries (new with tag, old without tag).
- **Remove by digest**: After re-pull, remove the old untagged entry by digest prefix. Verify it is removed and the tagged entry remains.
- **List with untagged artifacts**: Verify `podman artifact ls` displays untagged artifacts correctly with repository name and empty tag.

No changes to CI images are required.
