package dioclient

import "time"

// BucketInfo describes a single S3 bucket.
type BucketInfo struct {
	Name      string
	CreatedAt time.Time
}

// ObjectInfo describes a single S3 object.
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	ContentType  string
	StorageClass string
}
