package storage

import (
	"context"
	"time"

	"github.com/DirIO-S3/dirio/internal/persistence/metadata"
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

// checkObjectLock is a stub: it never blocks. Real decision logic is added later.
func checkObjectLock(meta *metadata.ObjectMetadata, now time.Time, bypassGovernance bool) error {
	return nil
}

// existingObjectMetadata is a stub: it reports no lock state for any object.
func (s *Storage) existingObjectMetadata(ctx context.Context, bucket, key string) (*metadata.ObjectMetadata, error) {
	return &metadata.ObjectMetadata{}, nil
}
