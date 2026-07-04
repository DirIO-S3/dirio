package copysource

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		wantBucket string
		wantKey    string
		wantErr    bool
	}{
		{
			name:       "leading slash form",
			header:     "/mybucket/mykey.txt",
			wantBucket: "mybucket",
			wantKey:    "mykey.txt",
		},
		{
			name:       "no leading slash",
			header:     "mybucket/mykey.txt",
			wantBucket: "mybucket",
			wantKey:    "mykey.txt",
		},
		{
			name:       "nested key path",
			header:     "/mybucket/path/to/file.txt",
			wantBucket: "mybucket",
			wantKey:    "path/to/file.txt",
		},
		{
			name:       "percent-encoded space in key",
			header:     "/mybucket/my%20file.txt",
			wantBucket: "mybucket",
			wantKey:    "my file.txt",
		},
		{
			name:       "percent-encoded plus stays literal",
			header:     "/mybucket/a%2Bb.txt",
			wantBucket: "mybucket",
			wantKey:    "a+b.txt",
		},
		{
			name:       "literal plus stays literal (not query-decoded)",
			header:     "/mybucket/a+b.txt",
			wantBucket: "mybucket",
			wantKey:    "a+b.txt",
		},
		{
			name:       "versionId query component stripped",
			header:     "/mybucket/mykey.txt?versionId=abc123",
			wantBucket: "mybucket",
			wantKey:    "mykey.txt",
		},
		{
			name:    "empty header",
			header:  "",
			wantErr: true,
		},
		{
			name:    "missing key",
			header:  "/mybucket",
			wantErr: true,
		},
		{
			name:    "missing key with trailing slash",
			header:  "/mybucket/",
			wantErr: true,
		},
		{
			name:    "invalid percent-encoding",
			header:  "/mybucket/bad%zz",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, key, err := Parse(tt.header)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = (%q, %q, nil), want error", tt.header, bucket, key)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.header, err)
			}
			if bucket != tt.wantBucket || key != tt.wantKey {
				t.Fatalf("Parse(%q) = (%q, %q), want (%q, %q)", tt.header, bucket, key, tt.wantBucket, tt.wantKey)
			}
		})
	}
}
