package dioclient

import (
	"context"

	"github.com/minio/minio-go/v7"
)

// ListBuckets returns all buckets visible to the configured credentials.
func (c *Client) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	buckets, err := c.mc.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]BucketInfo, len(buckets))
	for i, b := range buckets {
		out[i] = BucketInfo{Name: b.Name, CreatedAt: b.CreationDate}
	}
	return out, nil
}

// ListObjects streams the objects in bucket with the given prefix. When
// recursive is false a "/" delimiter is used and common prefixes (virtual
// directories) are returned as ObjectInfo entries with Size == -1. The
// returned channel is closed when all results have been sent or ctx is
// cancelled; check ObjectInfo.Err for per-object errors.
func (c *Client) ListObjects(ctx context.Context, bucket, prefix string, recursive bool) <-chan ObjectInfo {
	out := make(chan ObjectInfo)

	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: recursive,
	}

	go func() {
		defer close(out)
		for obj := range c.mc.ListObjects(ctx, bucket, opts) {
			if obj.Err != nil {
				select {
				case out <- ObjectInfo{Key: obj.Key, Size: -1}:
				case <-ctx.Done():
				}
				return
			}
			info := ObjectInfo{
				Key:          obj.Key,
				Size:         obj.Size,
				LastModified: obj.LastModified,
				ETag:         obj.ETag,
				ContentType:  obj.ContentType,
				StorageClass: obj.StorageClass,
			}
			select {
			case out <- info:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}
