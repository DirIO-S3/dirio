// Package urlbuilder wraps teapot-router's urlbuilder.Builder to add
// virtual-hosted-style URL generation. It is a thin, local wrapper rather
// than an upstream extension (see docs/design/VHOST-ROUTING.md open decision
// #1) — DirIO doesn't currently vendor/replace teapot-router locally, so
// extending it isn't practical without an upstream release cycle.
//
// Whichever style a generated URL uses mirrors the *inbound* request's
// style: if the client already reached us via vhost, it gets vhost URLs
// back, otherwise path-style — keeping presigned URLs, Location headers, and
// CopyObject sources consistent with however the client is already talking
// to us.
package urlbuilder

import (
	"fmt"
	"net/http"

	upstream "github.com/mallardduck/teapot-router/pkg/urlbuilder"

	"github.com/mallardduck/dirio/internal/http/vhost"
)

// Builder generates URLs for S3 API responses, choosing path-style or
// vhost-style per request.
type Builder struct {
	canonicalDomain string
	pathBuilder     *upstream.Builder
}

// New creates a new Builder. If canonicalDomain is empty, URLs are always
// built path-style from the request Host header (byte-for-byte the same as
// upstream's Builder, since vhost resolution is disabled).
func New(canonicalDomain string) *Builder {
	return &Builder{
		canonicalDomain: canonicalDomain,
		pathBuilder:     upstream.New(canonicalDomain),
	}
}

// BucketURL generates a URL for bucket operations, mirroring whether the
// inbound request itself was vhost- or path-style.
func (b *Builder) BucketURL(r *http.Request, bucket string) string {
	if _, ok := vhost.BucketFromHost(r.Host, b.canonicalDomain); ok {
		return fmt.Sprintf("https://%s.%s", bucket, b.canonicalDomain)
	}
	return b.pathBuilder.BucketURL(r, bucket)
}

// ObjectURL generates a URL for object operations, mirroring whether the
// inbound request itself was vhost- or path-style.
func (b *Builder) ObjectURL(r *http.Request, bucket, key string) string {
	if _, ok := vhost.BucketFromHost(r.Host, b.canonicalDomain); ok {
		return fmt.Sprintf("https://%s.%s/%s", bucket, b.canonicalDomain, key)
	}
	return b.pathBuilder.ObjectURL(r, bucket, key)
}
