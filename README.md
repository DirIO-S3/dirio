# test-assets

Orphan branch holding third-party binaries that the DirIO client tests used to download
from upstream hosts that have since gone away (dl.min.io returns 410 for mc).

Binaries are not stored in this branch. They are attached as assets to the release
tagged `test-assets-mc-RELEASE.2025-08-13T08-35-41Z`.

| Asset | Source | Build |
|---|---|---|
| mc-linux-amd64-RELEASE.2025-08-13T08-35-41Z, mc-linux-arm64-RELEASE.2025-08-13T08-35-41Z | github.com/minio/mc tag RELEASE.2025-08-13T08-35-41Z (commit 7394ce0dd2a80935aded936b09fa12cbb3cb8096) | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath` |

Verify with `sha256sum -c SHA256SUMS`.
