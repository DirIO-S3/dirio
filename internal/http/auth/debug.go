package auth

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/DirIO-S3/dirio/internal/consts"
	"github.com/DirIO-S3/dirio/internal/logging"
)

// canonicalSensitiveLine matches a "header-name:value" line within a SigV4
// canonical request/string-to-sign for a header whose value must not be
// logged verbatim (e.g. a security token or a client-signed cookie/auth header).
var canonicalSensitiveLine = regexp.MustCompile(`(?im)^(x-amz-security-token|x-amz-signature|authorization|cookie):.*$`)

// redactCanonicalText redacts sensitive header lines from a canonical
// request or string-to-sign while preserving the rest of the structure,
// so the debug output stays useful for diagnosing signature mismatches.
func redactCanonicalText(s string) string {
	return canonicalSensitiveLine.ReplaceAllStringFunc(s, func(line string) string {
		name, _, found := strings.Cut(line, ":")
		if !found {
			return logging.RedactString(line)
		}
		return name + ":" + logging.RedactString(line)
	})
}

// DebugVerifySignature is like VerifySignature but logs verbose debug information
// about each step of the signing process, at debug level, with credential and
// signature material redacted before logging. This keeps the output useful for
// diagnosing signature mismatches without leaking secrets into log aggregators
// that may be less tightly access-controlled than the server itself.
func DebugVerifySignature(r *http.Request, secretKey string) error {
	log := logging.Component("auth.debug")

	authHeader := r.Header.Get(authorizationHeader)
	creds, err := ParseAuthorizationHeader(authHeader)
	if err != nil {
		log.Debug("Failed to parse auth header", "error", err)
		return err
	}

	log.Debug("Parsed credentials",
		"access_key", creds.AccessKey,
		"region", creds.Region,
		"signed_headers", creds.SignedHeaders,
	)
	log.Debug("Client signature", "value", logging.RedactString(creds.Signature))

	// Get timestamp from X-Amz-Date header
	dateStr := r.Header.Get(dateHeader)
	if dateStr == "" {
		log.Debug("Missing X-Amz-Date header")
		return ErrMissingDateHeader
	}
	log.Debug("X-Amz-Date", "value", dateStr)

	timestamp, err := time.Parse(iso8601TimeFormat, dateStr)
	if err != nil {
		log.Debug("Failed to parse timestamp", "error", err)
		return fmt.Errorf("%w: %v", ErrInvalidDateFormat, err)
	}

	// Get payload hash from header
	payloadHash := r.Header.Get(consts.HeaderContentSHA256)
	if payloadHash == "" {
		payloadHash = consts.ContentSHA256Unsigned
	}
	log.Debug("Payload hash", "value", payloadHash)

	// Build canonical request
	canonicalRequest := BuildCanonicalRequest(r, creds.SignedHeaders, payloadHash)
	log.Debug("Canonical request", "value", redactCanonicalText(canonicalRequest))

	// Build string to sign
	stringToSign := BuildStringToSign(timestamp, creds.Region, canonicalRequest)
	log.Debug("String to sign", "value", redactCanonicalText(stringToSign))

	// Compute expected signature
	expectedSignature := ComputeSignature(secretKey, timestamp, creds.Region, stringToSign)
	log.Debug("Expected signature", "value", logging.RedactString(expectedSignature))

	// Compare signatures
	if expectedSignature != creds.Signature {
		log.Debug("Signature mismatch",
			"expected", logging.RedactString(expectedSignature),
			"got", logging.RedactString(creds.Signature),
		)
		return ErrSignatureMismatch
	}

	log.Debug("Signature verified successfully")
	return nil
}
