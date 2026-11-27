# Design Philosophy

DirIO is built on a simple principle: **objects should be files**.

## Core Tenets

### 1. Filesystem is the Source of Truth

Objects live at `<data-dir>/<bucket>/<key>` (for example `data/photos/2026/cat.jpg`). That's where your object bytes are, and a plain `cat` or `ls` shows them.

`<data-dir>/.dirio/` holds state that is **not** all derivable from the bucket directories: bucket config (owner, policy, Object Lock configuration), per-object metadata (content type, custom metadata, Object Lock retention and legal hold), IAM users, groups, policies and service accounts, the bbolt index (`.dirio/dirio.db`), and config and keyring files. Deleting `.dirio/` keeps your object bytes but loses that state. Back up `.dirio/` together with the bucket directories.

### 2. No Encoding, No Chunking

When you PUT an object, it's written to disk as-is. When you GET it, you read that file. No base64, no multipart reassembly, no deduplication tricks.

This makes debugging trivial: `ls` and `cat` work as expected.

### 3. Minimal Metadata

Metadata lives in `.dirio/`, mostly as JSON files, plus a bbolt index. Structure (abridged):

```
.dirio/
├── buckets/
│   └── mybucket.json      # Per-bucket: owner, policy, Object Lock config, versioning config
├── objects/
│   └── mybucket/
│       └── path/to/object.json   # Per-object: content type, custom metadata, lock state
├── iam/                   # Users, groups, policies, service accounts (one JSON file each)
├── dirio.db               # bbolt index
├── config.json
└── .import-state          # MinIO import tracking
```

Object metadata is stored per object, so ETag and content type are not recomputed from the filesystem on each request.

### 4. Import, Don't Migrate

When you point DirIO at MinIO data, we don't "migrate" it. We import the metadata into `.dirio/` and leave `.minio.sys/` alone.

Why? Because you might need to switch back. Or debug. Or compare. The cost is a few extra kilobytes.

### 5. Single Binary, No Dependencies

DirIO is one Go binary. No Redis, no PostgreSQL, no external services. Just:
- The binary
- A data directory
- TCP port 9000

## Non-Goals

### We Will Not Support

- **Distributed mode**: Use actual S3 or MinIO if you need clustering
- **Versioning (for now)**: Not implemented. A design is planned, with the current version kept at the original path and prior versions under `.dirio/versions/`. See [docs/design/VERSIONING.md](design/VERSIONING.md).
- **Advanced IAM**: Groups, policies and service accounts exist; AWS IAM API compatibility does not (see [README](../README.md)).

### Durability — Filesystem First, Sidecar EC as Fallback

The preferred durability path is to use a filesystem that handles it natively — ZFS, BTRFS, or hardware RAID. DirIO doesn't need to replicate what ZFS already does better.

For environments where that isn't possible (ext4/XFS JBODs, NTFS, locked-down NAS firmware), a **Sidecar EC driver** is planned for V2. It generates parity shards on separate disks using Reed-Solomon coding via `klauspost/reedsolomon` (pure Go). The data file on the primary disk always remains a complete, intact copy of the object — no stripe splitting, no lock-in. See [docs/design/STORAGE-ARCHITECTURE.md](design/STORAGE-ARCHITECTURE.md) for the V2 spec.

### We Will Not Optimize For

- **Billions of objects**: If you have that scale, use a real object store
- **Sub-millisecond latency**: This is for homelabs, not production SaaS
- **Maximum throughput**: Filesystem I/O is the bottleneck by design

## Architecture Decisions

### Why Go?

- MinIO is Go, so we can reference their code
- Single static binary for easy deployment
- Good stdlib for HTTP and filesystem I/O
- Fast enough for homelab workloads

### Why JSON for Metadata?

- Human-readable for debugging
- Easy to edit manually if needed
- Go stdlib marshaling is fine for our scale
- We're not optimizing for metadata read speed

### Why Not SQLite?

SQLite would add dependency complexity. Object and bucket metadata stays as JSON files, and an embedded bbolt index (`.dirio/dirio.db`, pure Go) handles lookups. SQLite remains an option if we hit performance limits.

### Why msgpack for MinIO Import?

MinIO stores bucket metadata as msgpack (`.metadata.bin`). We need to decode it once during import. After that, we never touch msgpack again—our format is JSON.

## Implementation Principles

### Fail Loud

If something goes wrong, log it clearly and return an error. Don't silently degrade or make assumptions.

Example: If we can't read bucket metadata, return 500, don't pretend the bucket doesn't exist.

### Idempotent Operations

PUT the same object twice? Same result. Import MinIO data twice? Same result. Creating an existing bucket? Error, not silent success.

### Filesystem Writes are Scary

Always write to a temp file first, then atomically rename. Never truncate/overwrite in place.

```go
// Good
tmp := filepath.Join(dir, ".tmp-"+uuid.New().String())
ioutil.WriteFile(tmp, data, 0644)
os.Rename(tmp, finalPath)

// Bad
os.OpenFile(finalPath, os.O_TRUNC|os.O_WRONLY, 0644)
```

### No Global State

Everything should be configurable and testable. Pass dependencies explicitly, don't rely on `init()` functions or global variables.

## Testing Strategy

### Unit Tests

Test each package in isolation. Mock filesystem operations if needed.

### Integration Tests

Spin up a real server, use AWS CLI to test operations end-to-end.

### Migration Tests

Keep a snapshot of MinIO data in `testdata/` and verify import works correctly.

## Performance Expectations

For a typical homelab NAS (4-core CPU, spinning disks):

- **Small objects (<1MB)**: 50-200 req/sec
- **Large objects (>100MB)**: Limited by disk I/O, ~100-500 MB/sec
- **List operations**: 1000-10000 objects/sec

These are rough estimates. Real performance depends on disk speed, CPU, and workload.

## Future Work

Still open:

- **Bucket versioning**: Requested. Design in [docs/design/VERSIONING.md](design/VERSIONING.md). Implementation waits for design review.
- **Object Lock on versioned buckets**: Depends on versioning. Object Lock currently works on non-versioned buckets only.

Done (kept for reference): multipart uploads, pre-signed URLs, range requests, object tagging, Object Lock on non-versioned buckets.

Things we probably won't add:

- **S3 Select**: Too complex for our use case
- **Glacier storage tiers**: Filesystem is already "one tier"
- **Cross-region replication**: Not applicable to single-node
- **Lambda triggers**: Out of scope entirely

## Contributing Philosophy

If you're adding a feature, ask:

1. Does this make the common case simpler?
2. Can it be done without external dependencies?
3. Is the code easy to understand?
4. Does it align with "filesystem-first"?

If yes to all four, it's probably a good fit.
