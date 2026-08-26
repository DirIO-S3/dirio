package logging

import (
	"net/http"
	"strings"
)

// sensitiveHeaders lists header names whose values must never be logged verbatim,
// since they carry credentials, signatures, or session tokens.
var sensitiveHeaders = map[string]bool{
	"Authorization":        true,
	"Cookie":               true,
	"Set-Cookie":           true,
	"X-Amz-Security-Token": true,
	"X-Amz-Signature":      true,
	"X-Amz-Credential":     true,
	"X-Api-Key":            true,
	"Proxy-Authorization":  true,
}

// RedactHeaders returns a copy of headers safe for logging, with sensitive
// values replaced by RedactString. Use this instead of logging http.Header
// directly, since it commonly carries Authorization/cookie credentials.
func RedactHeaders(h http.Header) http.Header {
	redacted := make(http.Header, len(h))
	for k, values := range h {
		if sensitiveHeaders[http.CanonicalHeaderKey(k)] {
			redacted[k] = []string{RedactString(strings.Join(values, ","))}
			continue
		}
		redacted[k] = values
	}
	return redacted
}

// RedactString masks a secret-bearing string for logging, keeping only enough
// of it (a short prefix) to correlate log lines during debugging without
// exposing the value itself.
func RedactString(s string) string {
	const keep = 4
	if s == "" {
		return ""
	}
	if len(s) <= keep {
		return "[redacted]"
	}
	return s[:keep] + "...[redacted]"
}
