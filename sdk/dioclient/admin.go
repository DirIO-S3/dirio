package dioclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ctxKey int

const ctxUseV1API ctxKey = iota

// WithV1API returns a context that tells InfoCannedPolicy to use the legacy V1 API
// instead of V2. Pass this when connecting to older MinIO or pre-fix DirIO builds.
func WithV1API(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxUseV1API, true)
}

// AdminClient is a connected DirIO/MinIO admin client. It is safe for concurrent use.
type AdminClient struct {
	mc *madmin.AdminClient
}

// NewAdminClient creates an AdminClient for the given Config using the MinIO
// admin API. The endpoint in cfg is parsed to extract the host and TLS flag.
// No network calls are made until the first operation.
func NewAdminClient(cfg Config) (*AdminClient, error) {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://localhost:9000"
	}

	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("dioclient/admin: invalid endpoint %q: %w", cfg.Endpoint, err)
	}

	secure := u.Scheme == "https"
	mc, err := madmin.NewWithOptions(u.Host, &madmin.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, fmt.Errorf("dioclient/admin: %w", err)
	}

	return &AdminClient{mc: mc}, nil
}

// --- Service accounts ---

// ListServiceAccounts returns service accounts for the given user.
// Pass user="" to list service accounts for the authenticated user.
func (a *AdminClient) ListServiceAccounts(ctx context.Context, user string) (madmin.ListServiceAccountsResp, error) {
	return a.mc.ListServiceAccounts(ctx, user)
}

// AddServiceAccount creates a new service account and returns its credentials.
func (a *AdminClient) AddServiceAccount(ctx context.Context, opts madmin.AddServiceAccountReq) (madmin.Credentials, error) {
	return a.mc.AddServiceAccount(ctx, opts)
}

// InfoServiceAccount returns metadata for a service account by access key.
func (a *AdminClient) InfoServiceAccount(ctx context.Context, accessKey string) (madmin.InfoServiceAccountResp, error) {
	return a.mc.InfoServiceAccount(ctx, accessKey)
}

// UpdateServiceAccount modifies an existing service account.
func (a *AdminClient) UpdateServiceAccount(ctx context.Context, accessKey string, opts madmin.UpdateServiceAccountReq) error {
	return a.mc.UpdateServiceAccount(ctx, accessKey, opts)
}

// DeleteServiceAccount removes a service account.
func (a *AdminClient) DeleteServiceAccount(ctx context.Context, accessKey string) error {
	return a.mc.DeleteServiceAccount(ctx, accessKey)
}

// --- IAM users ---

// ListUsers returns all IAM users.
func (a *AdminClient) ListUsers(ctx context.Context) (map[string]madmin.UserInfo, error) {
	return a.mc.ListUsers(ctx)
}

// AddUser creates a new IAM user.
func (a *AdminClient) AddUser(ctx context.Context, accessKey, secretKey string) error {
	return a.mc.AddUser(ctx, accessKey, secretKey)
}

// RemoveUser deletes an IAM user.
func (a *AdminClient) RemoveUser(ctx context.Context, accessKey string) error {
	return a.mc.RemoveUser(ctx, accessKey)
}

// GetUserInfo returns info for an IAM user.
func (a *AdminClient) GetUserInfo(ctx context.Context, accessKey string) (madmin.UserInfo, error) {
	return a.mc.GetUserInfo(ctx, accessKey)
}

// SetUserStatus enables or disables an IAM user.
func (a *AdminClient) SetUserStatus(ctx context.Context, accessKey string, status madmin.AccountStatus) error {
	return a.mc.SetUserStatus(ctx, accessKey, status)
}

// --- IAM policies ---

// ListCannedPolicies returns all named policies as raw JSON.
func (a *AdminClient) ListCannedPolicies(ctx context.Context) (map[string]json.RawMessage, error) {
	return a.mc.ListCannedPolicies(ctx)
}

// AddCannedPolicy creates or replaces a named policy.
func (a *AdminClient) AddCannedPolicy(ctx context.Context, name string, policyJSON []byte) error {
	return a.mc.AddCannedPolicy(ctx, name, policyJSON)
}

// InfoCannedPolicy returns metadata and raw JSON for a named policy.
// By default it uses the V2 API (includes timestamps and PolicyName).
// Wrap ctx with WithV1API to use the legacy V1 API instead (for older servers).
func (a *AdminClient) InfoCannedPolicy(ctx context.Context, name string) (*madmin.PolicyInfo, error) {
	if ctx.Value(ctxUseV1API) == true {
		raw, err := a.mc.InfoCannedPolicy(ctx, name) //nolint:staticcheck // deprecated V1 API used intentionally for legacy server compatibility
		if err != nil {
			return nil, err
		}
		return &madmin.PolicyInfo{PolicyName: name, Policy: json.RawMessage(raw)}, nil
	}
	return a.mc.InfoCannedPolicyV2(ctx, name)
}

// DeleteCannedPolicy removes a named IAM policy.
func (a *AdminClient) DeleteCannedPolicy(ctx context.Context, name string) error {
	return a.mc.RemoveCannedPolicy(ctx, name)
}

// AttachPolicy attaches a named policy to a user or group.
func (a *AdminClient) AttachPolicy(ctx context.Context, req madmin.PolicyAssociationReq) (madmin.PolicyAssociationResp, error) {
	return a.mc.AttachPolicy(ctx, req)
}

// DetachPolicy detaches a named policy from a user or group.
func (a *AdminClient) DetachPolicy(ctx context.Context, req madmin.PolicyAssociationReq) (madmin.PolicyAssociationResp, error) {
	return a.mc.DetachPolicy(ctx, req)
}
