# Bucket Versioning Design

**Status:** Proposal. Not implemented. Needs review before any code.

## Context

DirIO stores each object as a plain file at `<data-dir>/<bucket>/<key>`, with object metadata in `<data-dir>/.dirio/objects/<bucket>/<key>.json`. Overwrites and deletes currently destroy the previous bytes. Object Lock works on non-versioned buckets and refuses to overwrite or delete a locked object, because there is no older version to keep. Versioning would let an overwrite keep the old bytes as a prior version, and let a delete leave a delete marker in place of the data.

Today `GetBucketVersioning`, `PutBucketVersioning` and `ListObjectVersions` return NotImplemented (`internal/http/server/routes.go`). The bucket metadata struct already has `VersioningConfigXML` and `VersioningConfigUpdatedAt`, populated by the MinIO import, but nothing reads or writes them through the API.

## Goals

- The current version of every object stays at its existing path, so `ls` and `cat` still show the current data, and existing unversioned data needs no migration.
- Prior versions live under `<data-dir>/.dirio/versions/`, outside the bucket directories, so they never appear as bucket contents.
- Behaviour follows AWS for the operations clients use: versioning states, `versionId` on GET/HEAD/DELETE, delete markers, `ListObjectVersions`.

## Non-goals (this design)

- Object Lock on versioned buckets. It is a later phase, after versioning exists.
- Lifecycle rules that expire noncurrent versions.
- Changing Object Lock behaviour on unversioned buckets.

## Proposed layout

```
<data-dir>/
├── <bucket>/<key>                          # current version bytes (unchanged)
├── .dirio/
│   ├── objects/<bucket>/<key>.json         # current version metadata (unchanged)
│   └── versions/<bucket>/...               # prior versions and delete markers
```

Each prior version has a data file and a metadata file, named by version ID. The version index (which versions exist for a key, which is current, delete-marker flag) lives in BoltDB, `.dirio/dirio.db`. The on-disk files are the source of the bytes; the index is the source of ordering.

**Open decision:** the versions directory uses the object key as a path, so keys containing `/` nest. A key `a` (with versions) and a key `a/b` then share the directory `versions/<bucket>/a/`. This is safe only if version filenames can never equal a key segment. The alternatives are:

- **A. Key as path** (`versions/<bucket>/<key>/<versionId>.data`). Readable with `ls`. Needs a rule that version filenames cannot collide with key segments. Version IDs are UUIDs, so collision is unlikely, but the rule must be enforced.
- **B. Hashed key directory** (`versions/<bucket>/<sha256(key)>/`, with `key.json` holding the real key). No collisions. Harder to read by hand.

Recommendation: A, with the filename rule written down and tested. This is for the reviewer to confirm.

## Bucket versioning states

| State | Meaning | Behaviour |
|---|---|---|
| Unversioned (default) | Never enabled | Overwrite and delete destroy data, as today. Version ID is absent. |
| Enabled | `PutBucketVersioning Enabled` | Overwrite and delete keep prior versions. Each write gets a new version ID. |
| Suspended | `PutBucketVersioning Suspended` | New writes get version ID `null` and replace the current `null` version. Existing versions are kept. |

Pre-existing objects in a bucket that becomes Enabled are treated as the version `null`. On the first overwrite after enabling, that `null` version moves into the version store.

## Write and delete semantics

All operations run under the existing per-key mutex, so a key's versions are changed by one request at a time.

**Overwrite on Enabled bucket (PutObject, CopyObject, multipart complete):**
1. Write the new bytes to staging under `.dirio-uploads/` (as today).
2. If a current version exists, move its bytes and metadata into `versions/<bucket>/...` under its version ID.
3. Commit the new bytes to `<bucket>/<key>` by rename (as today).
4. Write the new current metadata and update the BoltDB version index.

**Delete on Enabled bucket:**
1. Move the current bytes and metadata into the version store.
2. Write a delete marker record (no bytes) into the version store.
3. Update the BoltDB index so the delete marker is current.

**Delete of a specific `versionId`:** removes that version's files and index entry. If it is the current version, the next most recent version becomes current, moving its bytes back to `<bucket>/<key>`.

**Crash safety (to specify before implementation):** each step above is a rename or a single write. A crash between steps can leave the current path missing while the index still points to it, or the version store holding a file the index does not know about. Proposed recovery: a startup reconciliation pass that compares the index with the files, which DirIO can do because the bytes stay on disk. The exact ordering and the reconciliation rules need a review.

## Object Lock interaction (later phase)

Lock state stays with each version. The current version's lock stays in `.dirio/objects/...json` as it is today. A prior version's lock is kept in its version metadata file. On a versioned bucket, an overwrite of a locked current version would create a new version and leave the locked one intact, so the refusal described in `docs/CLIENTS.md` would no longer apply. Unversioned buckets keep the refusal.

## Phasing

- **Phase A: config and reads.** `GetBucketVersioning` and `PutBucketVersioning` (Enabled and Suspended). `versionId` on GET and HEAD. Storage writes that keep prior versions, with the layout and recovery rules above.
- **Phase B: delete and listing.** Delete markers, `versionId` on DELETE, `x-amz-version-id` and `x-amz-delete-marker` response headers, `ListObjectVersions`.
- **Phase C: the rest.** `versionId` on CopyObject source and UploadPartCopy, multipart and copy version handling, per-version Object Lock (later phase, see above).

Each phase ends with its own integration tests and doc updates in `docs/CLIENTS.md`.

## Code touched (estimate, not yet verified)

- `internal/persistence/storage/` for writes, deletes and version moves.
- `internal/persistence/metadata/` for the BoltDB version index and the metadata cache, which is currently keyed by bucket and key only (`metadata.go`), so it needs a version dimension.
- `internal/http/api/s3/` for `versionId` parsing and response headers.
- `internal/http/server/routes.go` for the bucket versioning and `ListObjectVersions` routes.
- `internal/policy/` for the versioned actions (`s3:GetObjectVersion`, `s3:DeleteObjectVersion`, `s3:ListBucketVersions`).

## Testing

Integration tests under `tests/integration/` in the style of `object_lock_test.go`:
- state transitions, including Suspended behaviour with `null` versions
- overwrite keeps the old bytes, readable by `versionId`
- delete marker hides the object from a plain GET, and `ListObjectVersions` shows it
- deleting a `versionId` restores the previous current version
- a crash-recovery test that simulates an interrupted overwrite and runs the reconciliation pass

## Open questions

1. Key layout: A (key as path) or B (hashed key directory)? Recommendation: A.
2. Should Enabled versioning be allowed on a bucket with lock enabled? AWS requires it. Our current decision is that lock does not force versioning, so the answer here is yes, but the ordering with Object Lock is a later phase.
