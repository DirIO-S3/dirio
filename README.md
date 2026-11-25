# test-assets

Orphan branch holding third-party binaries that the DirIO client tests used to download
from upstream hosts that have since gone away (dl.min.io returns 410 for mc).

Binaries are not stored in this branch. They are attached as assets to the release
tagged `test-assets-mc-RELEASE.2025-08-13T08-35-41Z`.

| Asset | Source | Build |
|---|---|---|
| mc-linux-amd64-RELEASE.2025-08-13T08-35-41Z, mc-linux-arm64-RELEASE.2025-08-13T08-35-41Z | github.com/minio/mc tag RELEASE.2025-08-13T08-35-41Z (commit 7394ce0dd2a80935aded936b09fa12cbb3cb8096) | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath` |

Verify with `sha256sum -c SHA256SUMS`.

## MinIO server RELEASE.2022-10-24T18-35-07Z

Server build used for migration/compatibility fixtures (QUICKSTART.md names RELEASE.2022-10-24T18-35-07Z as the compatibility bound).

| Asset | Source | Build |
|---|---|---|
| minio-linux-amd64-RELEASE.2022-10-24T18-35-07Z, minio-linux-arm64-RELEASE.2022-10-24T18-35-07Z | github.com/minio/minio tag RELEASE.2022-10-24T18-35-07Z (commit fc6c7949727ec261cd57fbdb02fa7575d0fd8e61) | `CGO_ENABLED=0 GOOS=linux GOARCH=<arch> go build -trimpath` with `-X cmd.Version/ReleaseTag/CommitID` |

Release: `test-assets-minio-RELEASE.2022-10-24T18-35-07Z`.
