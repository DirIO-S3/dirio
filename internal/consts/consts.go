package consts

import (
	"github.com/google/uuid"
)

const (
	DefaultBucketLocation = "us-east-1"
)

// AWS Signature V4 Headers
const (
	// HeaderContentSHA256 is the AWS SigV4 header for payload hash
	HeaderContentSHA256 = "X-Amz-Content-Sha256"

	// HeaderDate is the AWS SigV4 header for request timestamp
	HeaderDate = "X-Amz-Date"

	// HeaderCopySource is the S3 header for copy operations
	HeaderCopySource = "X-Amz-Copy-Source"

	// HeaderCopySourceIfMatch fails CopyObject with 412 unless the source
	// object's ETag matches.
	HeaderCopySourceIfMatch = "X-Amz-Copy-Source-If-Match"

	// HeaderCopySourceIfNoneMatch fails CopyObject with 412 if the source
	// object's ETag matches.
	HeaderCopySourceIfNoneMatch = "X-Amz-Copy-Source-If-None-Match"

	// HeaderCopySourceIfModifiedSince fails CopyObject with 412 unless the
	// source object was modified after the given time.
	HeaderCopySourceIfModifiedSince = "X-Amz-Copy-Source-If-Modified-Since"

	// HeaderCopySourceIfUnmodifiedSince fails CopyObject with 412 if the
	// source object was modified after the given time.
	HeaderCopySourceIfUnmodifiedSince = "X-Amz-Copy-Source-If-Unmodified-Since"

	// HeaderMetadataDirective selects whether CopyObject copies the source
	// object's metadata ("COPY", the default) or replaces it with the
	// metadata headers on the copy request itself ("REPLACE").
	HeaderMetadataDirective = "X-Amz-Metadata-Directive"

	// MetadataDirectiveReplace is the HeaderMetadataDirective value that
	// requests destination metadata be taken from the request instead of
	// copied from the source.
	MetadataDirectiveReplace = "REPLACE"

	// HeaderBucketRegion is the S3 header for bucket region
	HeaderBucketRegion = "x-amz-bucket-region"

	// ContentSHA256Streaming is the value for chunked transfer encoding
	ContentSHA256Streaming = "STREAMING-AWS4-HMAC-SHA256-PAYLOAD"

	// ContentSHA256Unsigned is the value for unsigned payloads
	ContentSHA256Unsigned = "UNSIGNED-PAYLOAD"
)

const (
	AdminUUIDString = "badfc0de-fadd-fc0f-fee0-000dadbeef00"
)

var (
	AdminUUID uuid.UUID = uuid.MustParse(AdminUUIDString)
)

const (
	DirIOMetadataDir = ".dirio"
	DirIOUploadsDir  = ".dirio-uploads"
	MinioMetadataDir = ".minio.sys"
)
