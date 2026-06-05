package dioclient

import (
	"context"
	"io"

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

// PutObject uploads r to bucket/key. size is the content length (-1 for unknown).
// minio-go automatically uses multipart when size exceeds the part size (8 MiB).
func (c *Client) PutObject(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := c.mc.PutObject(ctx, bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
		PartSize:    8 * 1024 * 1024,
	})
	return err
}

// GetObject returns a reader for the object content and its metadata.
// The caller must close the returned ReadCloser.
func (c *Client) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, ObjectInfo, error) {
	obj, err := c.mc.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	stat, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, ObjectInfo{}, err
	}
	return obj, ObjectInfo{
		Key:          stat.Key,
		Size:         stat.Size,
		LastModified: stat.LastModified,
		ETag:         stat.ETag,
		ContentType:  stat.ContentType,
		StorageClass: stat.StorageClass,
	}, nil
}

// StatObject returns metadata for bucket/key without downloading it.
func (c *Client) StatObject(ctx context.Context, bucket, key string) (ObjectInfo, error) {
	stat, err := c.mc.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{
		Key:          stat.Key,
		Size:         stat.Size,
		LastModified: stat.LastModified,
		ETag:         stat.ETag,
		ContentType:  stat.ContentType,
		StorageClass: stat.StorageClass,
	}, nil
}

// RemoveObject deletes bucket/key.
func (c *Client) RemoveObject(ctx context.Context, bucket, key string) error {
	return c.mc.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}

// CopyObject performs a server-side copy from srcBucket/srcKey to dstBucket/dstKey.
func (c *Client) CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	src := minio.CopySrcOptions{Bucket: srcBucket, Object: srcKey}
	dst := minio.CopyDestOptions{Bucket: dstBucket, Object: dstKey}
	_, err := c.mc.CopyObject(ctx, dst, src)
	return err
}
