package s3

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/DirIO-S3/dirio/internal/consts"
	"github.com/DirIO-S3/dirio/internal/http/middleware"
	"github.com/DirIO-S3/dirio/internal/http/response"
	"github.com/DirIO-S3/dirio/sdk/s3types"
)

// GetBucketObjectLockConfiguration handles GET /{bucket}?object-lock
func (h *HTTPHandler) GetBucketObjectLockConfiguration(w http.ResponseWriter, r *http.Request, bucket string) {
	w.WriteHeader(http.StatusOK)
}

// PutBucketObjectLockConfiguration handles PUT /{bucket}?object-lock
func (h *HTTPHandler) PutBucketObjectLockConfiguration(w http.ResponseWriter, r *http.Request, bucket string) {
	w.WriteHeader(http.StatusOK)
}

// GetObjectRetention handles GET /{bucket}/{key}?retention
func (h *HTTPHandler) GetObjectRetention(w http.ResponseWriter, r *http.Request, bucket, key string) {
	w.WriteHeader(http.StatusOK)
}

// PutObjectRetention handles PUT /{bucket}/{key}?retention
func (h *HTTPHandler) PutObjectRetention(w http.ResponseWriter, r *http.Request, bucket, key string) {
	w.WriteHeader(http.StatusOK)
}

// GetObjectLegalHold handles GET /{bucket}/{key}?legal-hold
func (h *HTTPHandler) GetObjectLegalHold(w http.ResponseWriter, r *http.Request, bucket, key string) {
	w.WriteHeader(http.StatusOK)
}

// PutObjectLegalHold handles PUT /{bucket}/{key}?legal-hold
func (h *HTTPHandler) PutObjectLegalHold(w http.ResponseWriter, r *http.Request, bucket, key string) {
	w.WriteHeader(http.StatusOK)
}

// objectLockWriteHeaders parses the Object Lock headers sent with PutObject and
// CopyObject. A missing retain-until header yields nil.
func objectLockWriteHeaders(r *http.Request) (mode string, retainUntil *time.Time, legalHold string, err error) {
	mode = r.Header.Get(consts.HeaderObjectLockMode)
	legalHold = r.Header.Get(consts.HeaderObjectLockLegalHold)

	if raw := r.Header.Get(consts.HeaderObjectLockRetainUntilDate); raw != "" {
		t, parseErr := time.Parse(time.RFC3339, raw)
		if parseErr != nil {
			return "", nil, "", s3types.ErrInvalidObjectLockRequest
		}
		retainUntil = &t
	}
	return mode, retainUntil, legalHold, nil
}

// bypassGovernanceRequested reports whether the request asks to bypass GOVERNANCE retention.
// The authorization middleware has already verified the caller may do so.
func bypassGovernanceRequested(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get(consts.HeaderBypassGovernanceRetention), "true")
}

// isObjectLockError reports whether err is an Object Lock error that has a
// specific S3 error response.
func isObjectLockError(err error) bool {
	return errors.Is(err, s3types.ErrObjectLocked) ||
		errors.Is(err, s3types.ErrObjectLockNotEnabled) ||
		errors.Is(err, s3types.ErrObjectLockConfigNotFound) ||
		errors.Is(err, s3types.ErrNoSuchObjectLockConfiguration) ||
		errors.Is(err, s3types.ErrInvalidObjectLockRequest)
}

// writeObjectLockError maps an Object Lock service error to its S3 error response.
func writeObjectLockError(w http.ResponseWriter, r *http.Request, err error) {
	requestID := middleware.GetRequestID(r.Context())
	switch {
	case errors.Is(err, s3types.ErrBucketNotFound):
		respondError(r.Context(), w, requestID, err, s3types.ErrCodeNoSuchBucket, "bucket not found", response.SetErrAsMessage(err))
	case errors.Is(err, s3types.ErrObjectNotFound):
		respondError(r.Context(), w, requestID, err, s3types.ErrCodeNoSuchKey, "object not found", response.SetErrAsMessage(err))
	case errors.Is(err, s3types.ErrObjectLocked):
		respondError(r.Context(), w, requestID, err, s3types.ErrCodeAccessDenied, "access denied because object protected by object lock", response.SetErrAsMessage(err))
	case errors.Is(err, s3types.ErrObjectLockConfigNotFound):
		respondError(r.Context(), w, requestID, err, s3types.ErrCodeObjectLockConfigurationNotFound, "object lock configuration does not exist", response.SetErrAsMessage(err))
	case errors.Is(err, s3types.ErrNoSuchObjectLockConfiguration):
		respondError(r.Context(), w, requestID, err, s3types.ErrCodeNoSuchObjectLockConfiguration, "the specified object does not have an object lock configuration", response.SetErrAsMessage(err))
	case errors.Is(err, s3types.ErrObjectLockNotEnabled), errors.Is(err, s3types.ErrInvalidObjectLockRequest):
		respondError(r.Context(), w, requestID, err, s3types.ErrCodeInvalidRequest, "invalid object lock request", response.SetErrAsMessage(err))
	default:
		respondError(r.Context(), w, requestID, err, s3types.ErrCodeInternalError, "object lock operation failed", response.SetErrAsMessage(err))
	}
}
