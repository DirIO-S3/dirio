package urlbuilder

import (
	"net/http/httptest"
	"testing"
)

func TestBucketURL(t *testing.T) {
	tests := []struct {
		name            string
		canonicalDomain string
		host            string
		bucket          string
		want            string
	}{
		{
			name:            "no canonical domain builds path-style from request host",
			canonicalDomain: "",
			host:            "localhost:9000",
			bucket:          "mybucket",
			want:            "http://localhost:9000/mybucket",
		},
		{
			name:            "canonical domain set, path-style inbound request stays path-style",
			canonicalDomain: "s3.example.com",
			host:            "s3.example.com",
			bucket:          "mybucket",
			want:            "https://s3.example.com/mybucket",
		},
		{
			name:            "canonical domain set, vhost inbound request mirrors vhost-style",
			canonicalDomain: "s3.example.com",
			host:            "mybucket.s3.example.com",
			bucket:          "mybucket",
			want:            "https://mybucket.s3.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.canonicalDomain)
			req := httptest.NewRequest("GET", "http://"+tt.host+"/", nil)
			req.Host = tt.host

			got := b.BucketURL(req, tt.bucket)
			if got != tt.want {
				t.Fatalf("BucketURL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestObjectURL(t *testing.T) {
	tests := []struct {
		name            string
		canonicalDomain string
		host            string
		bucket          string
		key             string
		want            string
	}{
		{
			name:            "no canonical domain builds path-style from request host",
			canonicalDomain: "",
			host:            "localhost:9000",
			bucket:          "mybucket",
			key:             "path/to/key.txt",
			want:            "http://localhost:9000/mybucket/path/to/key.txt",
		},
		{
			name:            "vhost inbound request mirrors vhost-style",
			canonicalDomain: "s3.example.com",
			host:            "mybucket.s3.example.com",
			bucket:          "mybucket",
			key:             "path/to/key.txt",
			want:            "https://mybucket.s3.example.com/path/to/key.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.canonicalDomain)
			req := httptest.NewRequest("GET", "http://"+tt.host+"/", nil)
			req.Host = tt.host

			got := b.ObjectURL(req, tt.bucket, tt.key)
			if got != tt.want {
				t.Fatalf("ObjectURL = %q, want %q", got, tt.want)
			}
		})
	}
}
