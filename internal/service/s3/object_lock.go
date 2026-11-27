package s3

import (
	"context"
	"encoding/xml"
	"errors"
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

// ============================================================================
// Bucket Object Lock Configuration
// ============================================================================

// createBucketObjectLock marks a freshly created bucket as Object Lock enabled.
// Object Lock can only be enabled at bucket creation.
func (s *Service) createBucketObjectLock(ctx context.Context, bucket string) error {
	enabledXML, err := xml.Marshal(s3types.ObjectLockConfiguration{
		ObjectLockEnabled: s3types.ObjectLockEnabledValue,
	})
	if err != nil {
		return err
	}
	return s.metadataManager.SetBucketObjectLockConfig(ctx, bucket, string(enabledXML))
}

// PutBucketObjectLockConfiguration sets the default retention for a bucket that
// has Object Lock enabled.
func (s *Service) PutBucketObjectLockConfiguration(ctx context.Context, bucket string, cfg *s3types.ObjectLockConfiguration) error {
	if cfg.ObjectLockEnabled != s3types.ObjectLockEnabledValue {
		return s3types.ErrInvalidObjectLockRequest
	}
	if cfg.Rule != nil {
		if err := validateDefaultRetention(cfg.Rule.DefaultRetention); err != nil {
			return err
		}
	}

	_, enabled, err := s.bucketObjectLock(ctx, bucket)
	if err != nil {
		return err
	}
	if !enabled {
		return s3types.ErrObjectLockNotEnabled
	}

	// Store only the normalized fields, not whatever the client sent.
	storedXML, err := xml.Marshal(s3types.ObjectLockConfiguration{
		ObjectLockEnabled: s3types.ObjectLockEnabledValue,
		Rule:              cfg.Rule,
	})
	if err != nil {
		return err
	}
	return s.metadataManager.SetBucketObjectLockConfig(ctx, bucket, string(storedXML))
}

// GetBucketObjectLockConfiguration returns the bucket's Object Lock configuration.
// Returns s3types.ErrObjectLockConfigNotFound when the bucket does not have Object Lock enabled.
func (s *Service) GetBucketObjectLockConfiguration(ctx context.Context, bucket string) (*s3types.ObjectLockConfiguration, error) {
	cfg, enabled, err := s.bucketObjectLock(ctx, bucket)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, s3types.ErrObjectLockConfigNotFound
	}
	return &cfg, nil
}

// bucketObjectLock returns the Object Lock configuration of a bucket and whether
// Object Lock is enabled for it. The configuration is only meaningful when enabled
// is true. Returns s3types.ErrBucketNotFound when the bucket does not exist.
func (s *Service) bucketObjectLock(ctx context.Context, bucket string) (s3types.ObjectLockConfiguration, bool, error) {
	var cfg s3types.ObjectLockConfiguration

	meta, err := s.GetBucket(ctx, bucket)
	if err != nil {
		return cfg, false, err
	}
	if meta.ObjectLockConfigXML == "" {
		return cfg, false, nil
	}

	if err := xml.Unmarshal([]byte(meta.ObjectLockConfigXML), &cfg); err != nil {
		return cfg, false, err
	}
	return cfg, cfg.ObjectLockEnabled == s3types.ObjectLockEnabledValue, nil
}

// validateDefaultRetention checks a bucket default retention rule.
func validateDefaultRetention(d *s3types.DefaultRetention) error {
	if d == nil {
		return s3types.ErrInvalidObjectLockRequest
	}
	if !validRetentionMode(d.Mode) {
		return s3types.ErrInvalidObjectLockRequest
	}
	// Exactly one of Days or Years, and it must be positive.
	if (d.Days == nil) == (d.Years == nil) {
		return s3types.ErrInvalidObjectLockRequest
	}
	if (d.Days != nil && *d.Days <= 0) || (d.Years != nil && *d.Years <= 0) {
		return s3types.ErrInvalidObjectLockRequest
	}
	return nil
}

func validRetentionMode(mode string) bool {
	return mode == s3types.RetentionModeGovernance || mode == s3types.RetentionModeCompliance
}

// ============================================================================
// Write-time Object Lock resolution
// ============================================================================

// hasExplicitLock reports whether any Object Lock write setting was supplied.
func hasExplicitLock(mode string, retainUntil *time.Time, legalHold string) bool {
	return mode != "" || retainUntil != nil || legalHold != ""
}

// explicitPutOptions converts explicit Object Lock settings into storage options
// without re-checking them against the clock or the bucket. Use it only for
// settings that putOptionsFor has already validated, such as those recorded when
// a multipart upload was initiated, which may be older than the retain-until date.
func explicitPutOptions(mode string, retainUntil *time.Time, legalHold string) storage.PutOptions {
	var opts storage.PutOptions
	opts.LegalHold = legalHold == s3types.LegalHoldStatusOn
	if mode != "" && retainUntil != nil {
		opts.Retention = &metadata.ObjectRetention{Mode: mode, RetainUntil: *retainUntil}
	}
	return opts
}

// putOptionsFor resolves the Object Lock state a newly written object should get.
// Explicit request settings take precedence; otherwise the bucket's default
// retention applies. Explicit settings on a bucket without Object Lock are rejected.
func (s *Service) putOptionsFor(ctx context.Context, bucket, mode string, retainUntil *time.Time, legalHold string, bypass bool) (storage.PutOptions, error) {
	opts := storage.PutOptions{BypassGovernance: bypass}

	cfg, enabled, err := s.bucketObjectLock(ctx, bucket)
	if err != nil {
		return storage.PutOptions{}, err
	}
	hasExplicit := hasExplicitLock(mode, retainUntil, legalHold)

	if !enabled {
		if hasExplicit {
			return storage.PutOptions{}, s3types.ErrInvalidObjectLockRequest
		}
		return opts, nil
	}

	now := time.Now()
	switch legalHold {
	case "", s3types.LegalHoldStatusOff:
	case s3types.LegalHoldStatusOn:
		opts.LegalHold = true
	default:
		return storage.PutOptions{}, s3types.ErrInvalidObjectLockRequest
	}

	switch {
	case mode != "" || retainUntil != nil:
		// Mode and retain-until must be supplied together.
		if mode == "" || retainUntil == nil || !validRetentionMode(mode) || !retainUntil.After(now) {
			return storage.PutOptions{}, s3types.ErrInvalidObjectLockRequest
		}
		opts.Retention = &metadata.ObjectRetention{Mode: mode, RetainUntil: *retainUntil}
	case cfg.Rule != nil && cfg.Rule.DefaultRetention != nil:
		opts.Retention = defaultRetentionUntil(now, cfg.Rule.DefaultRetention)
	}

	return opts, nil
}

// defaultRetentionUntil computes the retain-until time for a bucket default rule.
// validateDefaultRetention guarantees exactly one of Days or Years is set.
func defaultRetentionUntil(now time.Time, d *s3types.DefaultRetention) *metadata.ObjectRetention {
	retainUntil := now
	if d.Days != nil {
		retainUntil = now.AddDate(0, 0, *d.Days)
	} else if d.Years != nil {
		retainUntil = now.AddDate(*d.Years, 0, 0)
	}
	return &metadata.ObjectRetention{Mode: d.Mode, RetainUntil: retainUntil}
}

// ============================================================================
// Object Retention and Legal Hold
// ============================================================================

// PutObjectRetention sets the retention on an existing object.
// Per S3 semantics, COMPLIANCE retention cannot be shortened or changed, and
// shortening GOVERNANCE retention requires BypassGovernanceRetention.
func (s *Service) PutObjectRetention(ctx context.Context, req *PutObjectRetentionRequest) error {
	_, enabled, err := s.bucketObjectLock(ctx, req.Bucket)
	if err != nil {
		return err
	}
	if !enabled {
		return s3types.ErrObjectLockNotEnabled
	}
	now := time.Now()
	if !validRetentionMode(req.Mode) || !req.RetainUntil.After(now) {
		return s3types.ErrInvalidObjectLockRequest
	}

	meta, err := s.loadObjectMetadataForUpdate(ctx, req.Bucket, req.Key)
	if err != nil {
		return err
	}

	if current := meta.Retention; current != nil && now.Before(current.RetainUntil) {
		shortened := req.RetainUntil.Before(current.RetainUntil)
		switch current.Mode {
		case s3types.RetentionModeCompliance:
			if shortened || req.Mode != current.Mode {
				return s3types.ErrObjectLocked
			}
		case s3types.RetentionModeGovernance:
			if shortened && !req.BypassGovernanceRetention {
				return s3types.ErrObjectLocked
			}
		}
	}

	meta.Retention = &metadata.ObjectRetention{Mode: req.Mode, RetainUntil: req.RetainUntil}
	return s.metadataManager.PutObjectMetadata(ctx, req.Bucket, req.Key, meta)
}

// GetObjectRetention returns the retention on an object.
// Returns s3types.ErrNoSuchObjectLockConfiguration when the object has no retention.
func (s *Service) GetObjectRetention(ctx context.Context, bucket, key string) (*metadata.ObjectRetention, error) {
	_, enabled, err := s.bucketObjectLock(ctx, bucket)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, s3types.ErrObjectLockNotEnabled
	}

	meta, err := s.loadObjectMetadataForUpdate(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	if meta.Retention == nil {
		return nil, s3types.ErrNoSuchObjectLockConfiguration
	}
	return meta.Retention, nil
}

// PutObjectLegalHold sets or clears the legal hold on an existing object.
// Changing the legal hold does not require the bypass permission.
func (s *Service) PutObjectLegalHold(ctx context.Context, req *PutObjectLegalHoldRequest) error {
	_, enabled, err := s.bucketObjectLock(ctx, req.Bucket)
	if err != nil {
		return err
	}
	if !enabled {
		return s3types.ErrObjectLockNotEnabled
	}
	if req.Status != s3types.LegalHoldStatusOn && req.Status != s3types.LegalHoldStatusOff {
		return s3types.ErrInvalidObjectLockRequest
	}

	meta, err := s.loadObjectMetadataForUpdate(ctx, req.Bucket, req.Key)
	if err != nil {
		return err
	}
	meta.LegalHold = req.Status == s3types.LegalHoldStatusOn
	return s.metadataManager.PutObjectMetadata(ctx, req.Bucket, req.Key, meta)
}

// GetObjectLegalHold reports whether an object is under legal hold.
func (s *Service) GetObjectLegalHold(ctx context.Context, bucket, key string) (bool, error) {
	_, enabled, err := s.bucketObjectLock(ctx, bucket)
	if err != nil {
		return false, err
	}
	if !enabled {
		return false, s3types.ErrObjectLockNotEnabled
	}

	meta, err := s.loadObjectMetadataForUpdate(ctx, bucket, key)
	if err != nil {
		return false, err
	}
	return meta.LegalHold, nil
}

// loadObjectMetadataForUpdate returns a private copy of an object's metadata that
// the caller can modify and save. The copy is required because the metadata
// layer returns cached pointers that must not be mutated in place.
func (s *Service) loadObjectMetadataForUpdate(ctx context.Context, bucket, key string) (*metadata.ObjectMetadata, error) {
	meta, err := s.diskStorage.GetObjectMetadata(ctx, bucket, key)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrNoSuchKey):
			return nil, s3types.ErrObjectNotFound
		case errors.Is(err, storage.ErrNoSuchBucket):
			return nil, s3types.ErrBucketNotFound
		}
		return nil, err
	}
	cp := *meta
	return &cp, nil
}
