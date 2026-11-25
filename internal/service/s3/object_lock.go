package s3

import (
	"context"
	"time"

	"github.com/DirIO-S3/dirio/internal/persistence/metadata"
	"github.com/DirIO-S3/dirio/internal/persistence/storage"
	"github.com/DirIO-S3/dirio/sdk/s3types"
)

// PutObjectRetentionRequest sets the Object Lock retention on an existing object
type PutObjectRetentionRequest struct {
	Bucket                    string
	Key                       string
	Mode                      string
	RetainUntil               time.Time
	BypassGovernanceRetention bool // Authorized bypass of GOVERNANCE retention
}

// PutObjectLegalHoldRequest sets the Object Lock legal hold on an existing object
type PutObjectLegalHoldRequest struct {
	Bucket string
	Key    string
	Status string // s3types.LegalHoldStatusOn or s3types.LegalHoldStatusOff
}

// createBucketObjectLock is a stub.
func (s *Service) createBucketObjectLock(ctx context.Context, bucket string) error {
	return nil
}

// PutBucketObjectLockConfiguration is a stub.
func (s *Service) PutBucketObjectLockConfiguration(ctx context.Context, bucket string, cfg *s3types.ObjectLockConfiguration) error {
	return nil
}

// GetBucketObjectLockConfiguration is a stub.
func (s *Service) GetBucketObjectLockConfiguration(ctx context.Context, bucket string) (*s3types.ObjectLockConfiguration, error) {
	return &s3types.ObjectLockConfiguration{}, nil
}

// putOptionsFor is a stub: it resolves no lock state for any write.
func (s *Service) putOptionsFor(ctx context.Context, bucket, mode string, retainUntil *time.Time, legalHold string, bypass bool) (storage.PutOptions, error) {
	return storage.PutOptions{}, nil
}

// PutObjectRetention is a stub.
func (s *Service) PutObjectRetention(ctx context.Context, req *PutObjectRetentionRequest) error {
	return nil
}

// GetObjectRetention is a stub.
func (s *Service) GetObjectRetention(ctx context.Context, bucket, key string) (*metadata.ObjectRetention, error) {
	return &metadata.ObjectRetention{}, nil
}

// PutObjectLegalHold is a stub.
func (s *Service) PutObjectLegalHold(ctx context.Context, req *PutObjectLegalHoldRequest) error {
	return nil
}

// GetObjectLegalHold is a stub.
func (s *Service) GetObjectLegalHold(ctx context.Context, bucket, key string) (bool, error) {
	return false, nil
}
