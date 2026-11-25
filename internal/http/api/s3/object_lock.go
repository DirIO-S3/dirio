package s3

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mallardduck/go-http-helpers/pkg/headers"

	"github.com/DirIO-S3/dirio/internal/consts"
	"github.com/DirIO-S3/dirio/internal/http/middleware"
	"github.com/DirIO-S3/dirio/internal/http/response"
	"github.com/DirIO-S3/dirio/internal/service/s3"
	"github.com/DirIO-S3/dirio/sdk/s3types"
)

// GetBucketObjectLockConfiguration handles GET /{bucket}?object-lock
func (h *HTTPHandler) GetBucketObjectLockConfiguration(w http.ResponseWriter, r *http.Request, bucket string) {
	cfg, err := h.s3Service.GetBucketObjectLockConfiguration(r.Context(), bucket)
	if err != nil {
		writeObjectLockError(w, r, err)
		return
	}

	w.Header().Set(headers.ContentType, "application/xml")
	if writeErr := WriteXMLResponse(w, http.StatusOK, cfg); writeErr != nil {
		s3Logger(r.Context()).With("err", writeErr).Warn("failed to write object lock configuration XML response")
	}
}

// PutBucketObjectLockConfiguration handles PUT /{bucket}?object-lock
func (h *HTTPHandler) PutBucketObjectLockConfiguration(w http.ResponseWriter, r *http.Request, bucket string) {
	requestID := middleware.GetRequestID(r.Context())

	body, err := io.ReadAll(r.Body)
	if err != nil {
		_ = WriteErrorResponse(w, requestID, s3types.ErrCodeInvalidRequest, response.SetErrAsMessage(err))
		return
	}

	var cfg s3types.ObjectLockConfiguration
	if err := xml.Unmarshal(body, &cfg); err != nil {
		_ = WriteErrorResponse(w, requestID, s3types.ErrCodeMalformedXML, response.SetErrAsMessage(err))
		return
	}

	if err := h.s3Service.PutBucketObjectLockConfiguration(r.Context(), bucket, &cfg); err != nil {
		// Setting a lock configuration on a bucket without Object Lock is an
		// invalid bucket state, not an invalid request.
		if errors.Is(err, s3types.ErrObjectLockNotEnabled) {
			respondError(r.Context(), w, requestID, err, s3types.ErrCodeInvalidBucketState, "bucket is not configured for object lock", response.SetErrAsMessage(err))
			return
		}
		writeObjectLockError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetObjectRetention handles GET /{bucket}/{key}?retention
func (h *HTTPHandler) GetObjectRetention(w http.ResponseWriter, r *http.Request, bucket, key string) {
	retention, err := h.s3Service.GetObjectRetention(r.Context(), bucket, key)
	if err != nil {
		writeObjectLockError(w, r, err)
		return
	}

	resp := s3types.ObjectRetentionConfiguration{
		Mode:            retention.Mode,
		RetainUntilDate: retention.RetainUntil,
	}
	w.Header().Set(headers.ContentType, "application/xml")
	if writeErr := WriteXMLResponse(w, http.StatusOK, resp); writeErr != nil {
		s3Logger(r.Context()).With("err", writeErr).Warn("failed to write object retention XML response")
	}
}

// PutObjectRetention handles PUT /{bucket}/{key}?retention
func (h *HTTPHandler) PutObjectRetention(w http.ResponseWriter, r *http.Request, bucket, key string) {
	requestID := middleware.GetRequestID(r.Context())

	body, err := io.ReadAll(r.Body)
	if err != nil {
		_ = WriteErrorResponse(w, requestID, s3types.ErrCodeInvalidRequest, response.SetErrAsMessage(err))
		return
	}

	var retention s3types.ObjectRetentionConfiguration
	if err := xml.Unmarshal(body, &retention); err != nil {
		_ = WriteErrorResponse(w, requestID, s3types.ErrCodeMalformedXML, response.SetErrAsMessage(err))
		return
	}

	err = h.s3Service.PutObjectRetention(r.Context(), &s3.PutObjectRetentionRequest{
		Bucket:                    bucket,
		Key:                       key,
		Mode:                      retention.Mode,
		RetainUntil:               retention.RetainUntilDate,
		BypassGovernanceRetention: bypassGovernanceRequested(r),
	})
	if err != nil {
		writeObjectLockError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetObjectLegalHold handles GET /{bucket}/{key}?legal-hold
func (h *HTTPHandler) GetObjectLegalHold(w http.ResponseWriter, r *http.Request, bucket, key string) {
	onHold, err := h.s3Service.GetObjectLegalHold(r.Context(), bucket, key)
	if err != nil {
		writeObjectLockError(w, r, err)
		return
	}

	status := s3types.LegalHoldStatusOff
	if onHold {
		status = s3types.LegalHoldStatusOn
	}
	resp := s3types.ObjectLegalHoldConfiguration{Status: status}
	w.Header().Set(headers.ContentType, "application/xml")
	if writeErr := WriteXMLResponse(w, http.StatusOK, resp); writeErr != nil {
		s3Logger(r.Context()).With("err", writeErr).Warn("failed to write object legal hold XML response")
	}
}

// PutObjectLegalHold handles PUT /{bucket}/{key}?legal-hold
func (h *HTTPHandler) PutObjectLegalHold(w http.ResponseWriter, r *http.Request, bucket, key string) {
	requestID := middleware.GetRequestID(r.Context())

	body, err := io.ReadAll(r.Body)
	if err != nil {
		_ = WriteErrorResponse(w, requestID, s3types.ErrCodeInvalidRequest, response.SetErrAsMessage(err))
		return
	}

	var legalHold s3types.ObjectLegalHoldConfiguration
	if err := xml.Unmarshal(body, &legalHold); err != nil {
		_ = WriteErrorResponse(w, requestID, s3types.ErrCodeMalformedXML, response.SetErrAsMessage(err))
		return
	}

	err = h.s3Service.PutObjectLegalHold(r.Context(), &s3.PutObjectLegalHoldRequest{
		Bucket: bucket,
		Key:    key,
		Status: legalHold.Status,
	})
	if err != nil {
		writeObjectLockError(w, r, err)
		return
	}

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
