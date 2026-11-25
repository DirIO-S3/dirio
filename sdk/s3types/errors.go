package s3types

import (
	"encoding/xml"
	"errors"
	"net/http"
)

// ErrorResponse represents an S3 error
type ErrorResponse struct {
	XMLName          xml.Name `xml:"Error" json:"-"`
	Code             string   `xml:"Code"`
	Message          string   `xml:"Message"`
	Key              string   `xml:"Key,omitempty" json:"Key,omitempty"`
	BucketName       string   `xml:"BucketName,omitempty" json:"BucketName,omitempty"`
	Resource         string
	Region           string `xml:"Region,omitempty" json:"Region,omitempty"`
	RequestID        string `xml:"RequestId" json:"RequestId"`
	HostID           string `xml:"HostId" json:"HostId"`
	ActualObjectSize string `xml:"ActualObjectSize,omitempty" json:"ActualObjectSize,omitempty"`
	RangeRequested   string `xml:"RangeRequested,omitempty" json:"RangeRequested,omitempty"`
}

// ErrorCode represents S3 error codes
type ErrorCode int

const (
	ErrCodeNone ErrorCode = iota
	ErrCodeInternalError
	ErrCodeNoSuchBucket
	ErrCodeNoSuchKey
	ErrCodeBucketAlreadyExists
	ErrCodeBucketNotEmpty
	ErrCodeMethodNotAllowed
	ErrCodeInvalidAccessKeyID
	ErrCodeSignatureDoesNotMatch
	ErrCodeInvalidBucketName
	ErrCodeInvalidObjectKey
	ErrCodeAccessDenied
	ErrCodeNoSuchBucketPolicy
	ErrCodeMalformedPolicy
	ErrCodeInvalidRequest
	ErrCodeMalformedXML
	ErrCodeNoSuchUpload
	ErrCodeInvalidPart
	ErrCodeNotImplemented
	ErrCodePreconditionFailed
	ErrCodeInvalidBucketState
	ErrCodeObjectLockConfigurationNotFound
	ErrCodeNoSuchObjectLockConfiguration
)

var (
	errorCodeString = map[ErrorCode]string{
		ErrCodeNone:                            "",
		ErrCodeInternalError:                   "InternalError",
		ErrCodeNoSuchBucket:                    "NoSuchBucket",
		ErrCodeNoSuchKey:                       "NoSuchKey",
		ErrCodeBucketAlreadyExists:             "BucketAlreadyExists",
		ErrCodeBucketNotEmpty:                  "BucketNotEmpty",
		ErrCodeMethodNotAllowed:                "MethodNotAllowed",
		ErrCodeInvalidAccessKeyID:              "InvalidAccessKeyId",
		ErrCodeSignatureDoesNotMatch:           "SignatureDoesNotMatch",
		ErrCodeInvalidBucketName:               "InvalidBucketName",
		ErrCodeInvalidObjectKey:                "KeyTooLongError",
		ErrCodeAccessDenied:                    "AccessDenied",
		ErrCodeNoSuchBucketPolicy:              "NoSuchBucketPolicy",
		ErrCodeMalformedPolicy:                 "MalformedPolicy",
		ErrCodeInvalidRequest:                  "InvalidRequest",
		ErrCodeMalformedXML:                    "MalformedXML",
		ErrCodeNoSuchUpload:                    "NoSuchUpload",
		ErrCodeInvalidPart:                     "InvalidPart",
		ErrCodeNotImplemented:                  "NotImplemented",
		ErrCodePreconditionFailed:              "PreconditionFailed",
		ErrCodeInvalidBucketState:              "InvalidBucketState",
		ErrCodeObjectLockConfigurationNotFound: "ObjectLockConfigurationNotFoundError",
		ErrCodeNoSuchObjectLockConfiguration:   "NoSuchObjectLockConfiguration",
	}

	errorCodeDescription = map[ErrorCode]string{
		ErrCodeNone:                            "",
		ErrCodeInternalError:                   "We encountered an internal error. Please try again.",
		ErrCodeNoSuchBucket:                    "The specified bucket does not exist.",
		ErrCodeNoSuchKey:                       "The specified key does not exist.",
		ErrCodeBucketAlreadyExists:             "The requested bucket name is not available.",
		ErrCodeBucketNotEmpty:                  "The bucket you tried to delete is not empty.",
		ErrCodeMethodNotAllowed:                "The specified method is not allowed against this resource.",
		ErrCodeInvalidAccessKeyID:              "The AWS access key ID you provided does not exist in our records.",
		ErrCodeSignatureDoesNotMatch:           "The request signature we calculated does not match the signature you provided.",
		ErrCodeInvalidBucketName:               "The specified bucket is not valid.",
		ErrCodeInvalidObjectKey:                "Your key is too long or contains invalid characters.",
		ErrCodeAccessDenied:                    "Access Denied",
		ErrCodeNoSuchBucketPolicy:              "The bucket policy does not exist",
		ErrCodeMalformedPolicy:                 "Policy has invalid resource",
		ErrCodeInvalidRequest:                  "Invalid request",
		ErrCodeMalformedXML:                    "The XML provided was not well-formed or did not validate against our published schema",
		ErrCodeNoSuchUpload:                    "The specified multipart upload does not exist.",
		ErrCodeInvalidPart:                     "One or more of the specified parts could not be found.",
		ErrCodeNotImplemented:                  "A header you provided implies functionality that is not implemented.",
		ErrCodePreconditionFailed:              "At least one of the pre-conditions you specified did not hold.",
		ErrCodeInvalidBucketState:              "The request is not valid with the current state of the bucket.",
		ErrCodeObjectLockConfigurationNotFound: "Object Lock configuration does not exist for this bucket",
		ErrCodeNoSuchObjectLockConfiguration:   "The specified object does not have a ObjectLock configuration",
	}

	errCodeStatusMap = map[ErrorCode]int{
		ErrCodeNoSuchBucket:                    http.StatusNotFound,
		ErrCodeNoSuchKey:                       http.StatusNotFound,
		ErrCodeBucketAlreadyExists:             http.StatusConflict,
		ErrCodeBucketNotEmpty:                  http.StatusConflict,
		ErrCodeMethodNotAllowed:                http.StatusMethodNotAllowed,
		ErrCodeInvalidAccessKeyID:              http.StatusForbidden,
		ErrCodeSignatureDoesNotMatch:           http.StatusForbidden,
		ErrCodeAccessDenied:                    http.StatusForbidden,
		ErrCodeInvalidBucketName:               http.StatusBadRequest,
		ErrCodeInvalidObjectKey:                http.StatusBadRequest,
		ErrCodeMalformedPolicy:                 http.StatusBadRequest,
		ErrCodeInvalidRequest:                  http.StatusBadRequest,
		ErrCodeMalformedXML:                    http.StatusBadRequest,
		ErrCodeNoSuchBucketPolicy:              http.StatusNotFound,
		ErrCodeNoSuchUpload:                    http.StatusNotFound,
		ErrCodeInvalidPart:                     http.StatusBadRequest,
		ErrCodeNotImplemented:                  http.StatusNotImplemented,
		ErrCodeInternalError:                   http.StatusInternalServerError,
		ErrCodePreconditionFailed:              http.StatusPreconditionFailed,
		ErrCodeInvalidBucketState:              http.StatusConflict,
		ErrCodeObjectLockConfigurationNotFound: http.StatusNotFound,
		ErrCodeNoSuchObjectLockConfiguration:   http.StatusNotFound,
	}
)

// String returns the string representation of error code
func (e ErrorCode) String() string {
	if val, ok := errorCodeString[e]; ok {
		return val
	}

	return "InternalError"
}

// Description returns the error description
func (e ErrorCode) Description() string {
	if val, ok := errorCodeDescription[e]; ok {
		return val
	}

	return "Internal error"
}

// HTTPStatus returns the HTTP status code for the error
func (e ErrorCode) HTTPStatus() int {
	if val, ok := errCodeStatusMap[e]; ok {
		return val
	}
	return http.StatusInternalServerError
}

// ============================================================================
// Internal S3 Errors (shared across storage, service, and API layers)
// ============================================================================

var (
	// Bucket errors
	ErrBucketNotFound      = errors.New("bucket not found")
	ErrBucketAlreadyExists = errors.New("bucket already exists")
	ErrBucketNotEmpty      = errors.New("bucket not empty")
	ErrInvalidBucketName   = errors.New("invalid bucket name")
	ErrNoSuchBucketPolicy  = errors.New("bucket policy does not exist")

	// Object errors
	ErrObjectNotFound   = errors.New("object not found")
	ErrInvalidObjectKey = errors.New("invalid object key")

	// Object Lock errors
	ErrObjectLocked                  = errors.New("access denied because object protected by object lock")
	ErrObjectLockNotEnabled          = errors.New("bucket is not configured for object lock")
	ErrObjectLockConfigNotFound      = errors.New("object lock configuration does not exist")
	ErrNoSuchObjectLockConfiguration = errors.New("the specified object does not have an object lock configuration")
	ErrInvalidObjectLockRequest      = errors.New("invalid object lock request")
)
