// Package copysource parses the S3 X-Amz-Copy-Source header value shared by
// CopyObject, UploadPartCopy, and the policy engine's authorization check
// (which needs the source bucket/key before the handler runs). Kept as its
// own small package, with no dependency on either the HTTP handler or the
// policy package, so both can import it without a cycle.
package copysource

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Parse extracts bucket and key from an X-Amz-Copy-Source header value.
// Accepts both "/bucket/key" and "bucket/key" forms. The value is
// percent-decoded (clients URL-encode the source key when it contains
// spaces or other special characters), and any trailing "?versionId=..."
// query component is stripped — DirIO doesn't support object versioning yet,
// so the version is ignored rather than becoming part of the key.
func Parse(header string) (bucket, key string, err error) {
	if header == "" {
		return "", "", errors.New("empty copy source")
	}

	source := strings.TrimPrefix(header, "/")

	if i := strings.IndexByte(source, '?'); i >= 0 {
		source = source[:i]
	}

	decoded, err := url.PathUnescape(source)
	if err != nil {
		return "", "", fmt.Errorf("invalid copy source encoding: %w", err)
	}

	bucket, key, found := strings.Cut(decoded, "/")
	if !found || bucket == "" || key == "" {
		return "", "", errors.New("invalid copy source format, expected /bucket/key")
	}

	return bucket, key, nil
}
