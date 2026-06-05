// Package dioclient provides an importable client library for DirIO servers.
// It wraps the S3 API (via minio-go) and the DirIO-specific REST API
// (/.dirio/api/v1/), presenting a single interface for all client operations.
//
// The package has no dependency on internal/ server packages and is safe to
// import in third-party tools.
package dioclient

import (
	"fmt"
	"net/url"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config holds the connection parameters for a single DirIO server.
type Config struct {
	// Endpoint is the S3 API base URL, e.g. "http://localhost:9000".
	Endpoint string
	// AccessKey is the S3 access key (or IAM user access key).
	AccessKey string
	// SecretKey is the corresponding secret key.
	SecretKey string
	// Region defaults to "us-east-1" when empty.
	Region string
}

// Client is a connected DirIO client. It is safe for concurrent use.
type Client struct {
	mc     *minio.Client
	cfg    Config
	secure bool
}

// New creates a Client for the given Config. It does not make any network
// calls; connection errors surface on the first operation.
func New(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://localhost:9000"
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("dioclient: invalid endpoint %q: %w", cfg.Endpoint, err)
	}

	secure := u.Scheme == "https"
	host := u.Host

	mc, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: secure,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("dioclient: %w", err)
	}

	return &Client{mc: mc, cfg: cfg, secure: secure}, nil
}
