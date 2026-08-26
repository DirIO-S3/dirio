// Package path provides filesystem abstraction and security for DirIO.
//
// This package wraps go-billy to provide:
// - Chroot-based filesystem isolation (via go-billy's helper/chroot)
// - Path traversal attack prevention
// - Scoped filesystem access for buckets and metadata
//
// Design principles:
// - Filesystem-level security only (path traversal, null bytes, absolute paths)
// - NO S3 validation (that belongs in API handlers)
// - Generic and reusable for any filesystem access
package path

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/chroot"
	"github.com/go-git/go-billy/v5/osfs"

	"github.com/DirIO-S3/dirio/internal/consts"
)

var (
	// ErrInvalidPath indicates a path failed security validation
	ErrInvalidPath = errors.New("invalid path: security violation")

	// ErrPathTraversal indicates an attempt to escape the filesystem boundary
	ErrPathTraversal = errors.New("path traversal detected")

	// ErrAbsolutePath indicates an absolute path was provided where relative is required
	ErrAbsolutePath = errors.New("absolute paths not allowed")

	// ErrNullByte indicates a null byte in the path
	ErrNullByte = errors.New("null byte in path")
)

// NewRootFS creates a chroot-protected filesystem at the specified data directory.
// This is the root filesystem for all DirIO operations.
//
// The filesystem is isolated to dataDir using chroot, preventing any access
// outside this directory tree.
func NewRootFS(dataDir string) (billy.Filesystem, error) {
	// Ensure dataDir is an absolute path
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve data directory: %w", err)
	}

	// osfs.New defaults to go-billy's ChrootOS, which already wraps the OS
	// filesystem in helper/chroot for us, so this is chroot-protected as-is.
	return osfs.New(absDataDir), nil
}

// NewMinIOFS creates a read-only filesystem scoped to the MinIO metadata directory.
// This is used for importing existing MinIO data.
//
// Returns an error if the MinIO directory doesn't exist.
func NewMinIOFS(rootFS billy.Filesystem) (billy.Filesystem, error) {
	// Verify MinIO directory exists
	info, err := rootFS.Stat(consts.MinioMetadataDir)
	if err != nil {
		return nil, fmt.Errorf("MinIO metadata directory not found: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("MinIO metadata path is not a directory")
	}

	// Create a chrooted, read-only filesystem for the MinIO directory
	readOnlyRootFS := ReadOnlyFS{rootFS}
	return chroot.New(readOnlyRootFS, consts.MinioMetadataDir), nil
}

// NewMetadataFS creates a read/write filesystem scoped to the DirIO metadata directory.
// This is used for storing DirIO's own metadata (bucket info, policies, etc.)
//
// Creates the metadata directory if it doesn't exist.
func NewMetadataFS(rootFS billy.Filesystem) (billy.Filesystem, error) {
	// Ensure metadata directory exists
	if err := rootFS.MkdirAll(consts.DirIOMetadataDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// Create a scoped filesystem for the metadata directory
	return chroot.New(rootFS, consts.DirIOMetadataDir), nil
}

// NewBucketFS creates a filesystem scoped to a specific bucket directory.
// This ensures bucket operations cannot access other buckets or system directories.
//
// The bucket parameter should be the bucket name (already validated by API layer).
// This function only validates path safety, not S3 naming rules.
//
// Creates the bucket directory if it doesn't exist.
func NewBucketFS(rootFS billy.Filesystem, bucket string) (billy.Filesystem, error) {
	// Validate the bucket name for path safety (not S3 compliance)
	if err := ValidatePathSafe(bucket); err != nil {
		return nil, fmt.Errorf("bucket name failed path security check: %w", err)
	}

	// Ensure bucket directory exists
	if err := rootFS.MkdirAll(bucket, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create bucket directory: %w", err)
	}

	// Create a scoped filesystem for the bucket
	return chroot.New(rootFS, bucket), nil
}

// NewUploadStagingFS creates a filesystem scoped to the upload staging area for a bucket.
// All in-progress write state (PutObject temp files, multipart parts) lives here,
// completely outside the bucket directory, so it can never appear in S3 listings.
//
// Creates the staging directory if it doesn't exist.
func NewUploadStagingFS(rootFS billy.Filesystem, bucket string) (billy.Filesystem, error) {
	if err := ValidatePathSafe(bucket); err != nil {
		return nil, fmt.Errorf("bucket name failed path security check: %w", err)
	}
	stagingPath := filepath.Join(consts.DirIOUploadsDir, bucket)
	if err := rootFS.MkdirAll(stagingPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create upload staging directory: %w", err)
	}
	return chroot.New(rootFS, stagingPath), nil
}

// ValidatePathSafe checks if a path is safe from a filesystem security perspective.
// This function checks for:
// - Path traversal attempts (../)
// - Absolute paths
// - Null bytes
// - Empty paths
//
// This does NOT validate S3 naming rules - that's the API layer's responsibility.
func ValidatePathSafe(path string) error {
	if path == "" {
		return fmt.Errorf("%w: empty path", ErrInvalidPath)
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("%w: %w", ErrInvalidPath, ErrNullByte)
	}

	// Check for absolute paths
	if filepath.IsAbs(path) {
		return fmt.Errorf("%w: %w", ErrInvalidPath, ErrAbsolutePath)
	}

	// Check for paths starting with /
	if strings.HasPrefix(path, "/") {
		return fmt.Errorf("%w: %w", ErrInvalidPath, ErrAbsolutePath)
	}

	// Check for path traversal attempts in original path
	// Look for .. as a path component (not just substring like my..file)
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		if part == ".." {
			return fmt.Errorf("%w: %w", ErrInvalidPath, ErrPathTraversal)
		}
	}

	return nil
}

// CleanPath sanitizes a path for safe filesystem access.
// Returns the cleaned path or an error if the path is unsafe.
//
// This uses filepath.Clean to normalize the path, then validates it.
// Always returns forward slashes for consistency with S3 conventions.
func CleanPath(path string) (string, error) {
	// Clean the path (OS-native separators)
	cleaned := filepath.Clean(path)

	// Convert to forward slashes for S3 consistency (cross-platform)
	cleaned = filepath.ToSlash(cleaned)

	// Validate the cleaned path
	if err := ValidatePathSafe(cleaned); err != nil {
		return "", err
	}

	return cleaned, nil
}
