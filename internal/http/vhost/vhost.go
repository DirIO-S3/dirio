// Package vhost resolves S3 bucket names from virtual-hosted-style Host
// headers. It is the only place that knows how to translate between a Host
// header and a bucket name — route registration and handlers stay unaware
// of which addressing style produced the bucket they were given.
package vhost

import (
	"net"
	"strings"
)

// BucketFromHost extracts the bucket name from a Host header, given the
// configured canonical domain. Returns ok=false if host does not have a
// canonical-domain subdomain (i.e. this is not a vhost-style request):
// canonicalDomain is empty, host is exactly the bare canonical domain
// (path-style), or host doesn't end in "."+canonicalDomain.
//
// Only r.Host should ever be passed in here — never a forwarded header —
// since routing must key off the same value SigV4 signs.
func BucketFromHost(host, canonicalDomain string) (bucket string, ok bool) {
	if canonicalDomain == "" {
		return "", false
	}

	// Strip a port suffix, if present.
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	host = strings.ToLower(host)
	canonicalDomain = strings.ToLower(canonicalDomain)

	if host == canonicalDomain {
		return "", false
	}

	suffix := "." + canonicalDomain
	if !strings.HasSuffix(host, suffix) {
		return "", false
	}

	bucket = strings.TrimSuffix(host, suffix)
	if bucket == "" {
		return "", false
	}

	return bucket, true
}
