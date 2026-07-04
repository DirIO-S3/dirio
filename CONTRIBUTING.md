# Contributing to DirIO

Thanks for considering contributing!

## Setup

This project uses [go-task](https://taskfile.dev) for build automation. Install it once:

```bash
go install github.com/go-task/task/v3/cmd/task@latest
```

Then clone and build:

```bash
git clone https://github.com/mallardduck/dirio.git
cd dirio
go mod tidy
task build
```

Run `task --list` to see all available tasks.

### Multi-module workspace

This repo is a multi-module workspace (`.`, `api/`, `common/`, `console/`, `sdk/`). The root
`go.mod` depends on the others via pinned versions, not local paths, so changes you make in
`api/` or `sdk/` won't be picked up by the root module until you either bump those pins or use
a local Go workspace.

Run this once per clone so cross-module edits build and your IDE resolves them locally:

```bash
task workspace-init
```

This creates a local, uncommitted `go.work` (from `go.work.example`). We deliberately don't
commit `go.work` itself: with it checked in, CI and any `go build`/`go vet ./...` at the repo
root would silently use the local module directories instead of the versions actually pinned
in `go.mod` — masking real version-mismatch bugs that downstream consumers of these modules
would hit.

## Making Changes

1. **Pick a task** from [TODO.md](TODO.md) or fix a bug
2. **Create a branch**: `git checkout -b fix-thing`
3. **Make your changes**
4. **Test**: `task test`
5. **Format**: `task fmt`
6. **Commit**: `git commit -m "Fix thing"`
7. **Push**: `git push origin fix-thing`
8. **Open PR**

## Guidelines

## Guidelines

**Code:**
- Follow standard Go conventions
- Keep functions small
- Avoid global state
- Pass dependencies explicitly

**Tests:**
- Add tests for new features
- Don't break existing tests
- Test edge cases

**Commits:**
- One logical change per commit
- Clear commit messages
- No "WIP" or "fix" commits in PRs

**PRs:**
- Describe what and why
- Link related issues
- Keep changes focused

## What to Work On

Check [TODO.md](TODO.md) for tasks.

**Good first issues:**
- Add tests for existing code
- Improve error messages
- Fix documentation typos
- Add examples

**Bigger projects:**
- Implement missing S3 operations
- Optimize performance
- Improve metrics/monitoring

## Testing

```bash
# Unit tests
go test ./...

# Run server locally
./dirio-server --data-dir ./testdata

# Test with AWS CLI
aws --endpoint-url http://localhost:9000 s3 ls
```

## Questions?

Open an issue or start a discussion.
