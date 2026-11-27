package policy

import "testing"

func TestAcceptsBypassGovernance(t *testing.T) {
	tests := map[string]bool{
		"s3:DeleteObject":          true,
		"s3:DeleteObjects":         true,
		"s3:PutObject":             true,
		"s3:CopyObject":            true,
		"s3:PutObjectRetention":    true,
		"s3:GetObject":             false,
		"s3:CreateMultipartUpload": false,
	}
	for action, want := range tests {
		if got := acceptsBypassGovernance(action); got != want {
			t.Errorf("acceptsBypassGovernance(%q) = %v, want %v", action, got, want)
		}
	}
}
