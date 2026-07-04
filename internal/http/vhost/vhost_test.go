package vhost

import "testing"

func TestBucketFromHost(t *testing.T) {
	tests := []struct {
		name            string
		host            string
		canonicalDomain string
		wantBucket      string
		wantOK          bool
	}{
		{
			name:            "bare canonical domain is path-style",
			host:            "s3.example.com",
			canonicalDomain: "s3.example.com",
			wantBucket:      "",
			wantOK:          false,
		},
		{
			name:            "simple subdomain resolves bucket",
			host:            "mybucket.s3.example.com",
			canonicalDomain: "s3.example.com",
			wantBucket:      "mybucket",
			wantOK:          true,
		},
		{
			name:            "IP host is path-style",
			host:            "192.168.1.10",
			canonicalDomain: "s3.example.com",
			wantBucket:      "",
			wantOK:          false,
		},
		{
			name:            "port suffix is stripped before matching",
			host:            "mybucket.s3.example.com:9000",
			canonicalDomain: "s3.example.com",
			wantBucket:      "mybucket",
			wantOK:          true,
		},
		{
			name:            "IPv6 host with port is path-style",
			host:            "[::1]:9000",
			canonicalDomain: "s3.example.com",
			wantBucket:      "",
			wantOK:          false,
		},
		{
			name:            "dotted bucket name still resolves (documented TLS limitation)",
			host:            "my.dotted.bucket.s3.example.com",
			canonicalDomain: "s3.example.com",
			wantBucket:      "my.dotted.bucket",
			wantOK:          true,
		},
		{
			name:            "empty canonical domain disables vhost resolution",
			host:            "mybucket.s3.example.com",
			canonicalDomain: "",
			wantBucket:      "",
			wantOK:          false,
		},
		{
			name:            "mixed-case host normalizes to lowercase bucket",
			host:            "MyBucket.S3.Example.Com",
			canonicalDomain: "s3.example.com",
			wantBucket:      "mybucket",
			wantOK:          true,
		},
		{
			name:            "unrelated host is path-style",
			host:            "example.org",
			canonicalDomain: "s3.example.com",
			wantBucket:      "",
			wantOK:          false,
		},
		{
			name:            "host that merely contains canonical domain as substring, not suffix",
			host:            "s3.example.com.evil.com",
			canonicalDomain: "s3.example.com",
			wantBucket:      "",
			wantOK:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, ok := BucketFromHost(tt.host, tt.canonicalDomain)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if bucket != tt.wantBucket {
				t.Fatalf("bucket = %q, want %q", bucket, tt.wantBucket)
			}
		})
	}
}
