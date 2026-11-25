package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/DirIO-S3/dirio/internal/persistence/metadata"
	"github.com/DirIO-S3/dirio/sdk/s3types"
)

func TestCheckObjectLock(t *testing.T) {
	now := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name   string
		meta   *metadata.ObjectMetadata
		bypass bool
		want   error
	}{
		{name: "no metadata is unprotected", meta: nil, want: nil},
		{name: "no lock state is unprotected", meta: &metadata.ObjectMetadata{}, want: nil},
		{
			name: "expired governance is unprotected",
			meta: &metadata.ObjectMetadata{Retention: &metadata.ObjectRetention{Mode: s3types.RetentionModeGovernance, RetainUntil: past}},
			want: nil,
		},
		{
			name: "active governance blocks",
			meta: &metadata.ObjectMetadata{Retention: &metadata.ObjectRetention{Mode: s3types.RetentionModeGovernance, RetainUntil: future}},
			want: ErrObjectLocked,
		},
		{
			name:   "active governance allows bypass",
			meta:   &metadata.ObjectMetadata{Retention: &metadata.ObjectRetention{Mode: s3types.RetentionModeGovernance, RetainUntil: future}},
			bypass: true,
			want:   nil,
		},
		{
			name: "active compliance blocks",
			meta: &metadata.ObjectMetadata{Retention: &metadata.ObjectRetention{Mode: s3types.RetentionModeCompliance, RetainUntil: future}},
			want: ErrObjectLocked,
		},
		{
			name:   "active compliance ignores bypass",
			meta:   &metadata.ObjectMetadata{Retention: &metadata.ObjectRetention{Mode: s3types.RetentionModeCompliance, RetainUntil: future}},
			bypass: true,
			want:   ErrObjectLocked,
		},
		{
			name: "expired compliance is unprotected",
			meta: &metadata.ObjectMetadata{Retention: &metadata.ObjectRetention{Mode: s3types.RetentionModeCompliance, RetainUntil: past}},
			want: nil,
		},
		{
			name:   "legal hold blocks even with bypass",
			meta:   &metadata.ObjectMetadata{LegalHold: true},
			bypass: true,
			want:   ErrObjectLocked,
		},
		{
			name: "legal hold blocks with expired retention",
			meta: &metadata.ObjectMetadata{
				LegalHold: true,
				Retention: &metadata.ObjectRetention{Mode: s3types.RetentionModeGovernance, RetainUntil: past},
			},
			want: ErrObjectLocked,
		},
		{
			name: "unknown mode fails closed",
			meta: &metadata.ObjectMetadata{Retention: &metadata.ObjectRetention{Mode: "BOGUS", RetainUntil: future}},
			want: ErrObjectLocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkObjectLock(tt.meta, now, tt.bypass)
			if tt.want == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.want)
		})
	}
}
