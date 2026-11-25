package storage

import (
	"context"
	"errors"
	"time"

	"github.com/DirIO-S3/dirio/internal/persistence/metadata"
	"github.com/DirIO-S3/dirio/sdk/s3types"
)

// PutOptions carries Object Lock settings applied while an object is written.
type PutOptions struct {
	// Retention is stored on the object. Nil means no retention.
	Retention *metadata.ObjectRetention
	// LegalHold is stored on the object.
	LegalHold bool
	// BypassGovernance allows replacing an existing object that is under
	// GOVERNANCE retention. The caller must have verified the bypass permission.
	BypassGovernance bool
}

// checkObjectLock returns ErrObjectLocked when an existing object with the given
// metadata must not be deleted or replaced at time now. A nil meta means the
// object has no lock state and is unprotected.
func checkObjectLock(meta *metadata.ObjectMetadata, now time.Time, bypassGovernance bool) error {
	if meta == nil {
		return nil
	}
	if meta.LegalHold {
		return ErrObjectLocked
	}

	retention := meta.Retention
	if retention == nil || !now.Before(retention.RetainUntil) {
		// No retention, or retention has expired.
		return nil
	}

	switch retention.Mode {
	case s3types.RetentionModeGovernance:
		if bypassGovernance {
			return nil
		}
		return ErrObjectLocked
	case s3types.RetentionModeCompliance:
		return ErrObjectLocked
	default:
		// Unknown mode: fail closed.
		return ErrObjectLocked
	}
}

// existingObjectMetadata returns the stored metadata for an object. An object with
// no metadata file has no lock state, so an empty value is returned for it. Any
// other read error is returned so that the caller fails closed rather than
// treating a locked object as unlocked.
func (s *Storage) existingObjectMetadata(ctx context.Context, bucket, key string) (*metadata.ObjectMetadata, error) {
	meta, err := s.metadataManager.GetObjectMetadata(ctx, bucket, key)
	if errors.Is(err, metadata.ErrObjectMetadataNotFound) {
		return &metadata.ObjectMetadata{}, nil
	}
	return meta, err
}
