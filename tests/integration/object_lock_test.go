package integration

import (
	"encoding/xml"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DirIO-S3/dirio/internal/testutil"
)

// createLockedBucket creates a bucket with Object Lock enabled at creation time.
func createLockedBucket(t *testing.T, ts *testutil.TestServer, bucket string) {
	t.Helper()
	req, err := ts.NewRequest(http.MethodPut, ts.BucketURL(bucket), nil)
	require.NoError(t, err)
	req.Header.Set("X-Amz-Bucket-Object-Lock-Enabled", "true")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer DrainAndClose(resp)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// doObjectRequest sends a signed request with optional extra headers and returns the response.
func doObjectRequest(t *testing.T, ts *testutil.TestServer, method, url string, body []byte, headers map[string]string) *http.Response {
	t.Helper()
	req, err := ts.NewRequest(method, url, body)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// putLockedObject writes an object with explicit lock headers.
func putLockedObject(t *testing.T, ts *testutil.TestServer, bucket, key, mode string, retainUntil time.Time) {
	t.Helper()
	resp := doObjectRequest(t, ts, http.MethodPut, ts.ObjectURL(bucket, key), []byte("locked"), map[string]string{
		"X-Amz-Object-Lock-Mode":              mode,
		"X-Amz-Object-Lock-Retain-Until-Date": retainUntil.UTC().Format(time.RFC3339),
	})
	defer DrainAndClose(resp)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestObjectLock_BucketConfiguration(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()

	ts.CreateBucket(t, "plain-bucket")
	createLockedBucket(t, ts, "lock-bucket")

	// A locked bucket reports its configuration
	resp := doObjectRequest(t, ts, http.MethodGet, ts.BucketURL("lock-bucket")+"?object-lock", nil, nil)
	defer DrainAndClose(resp)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "<ObjectLockEnabled>Enabled</ObjectLockEnabled>")

	// A plain bucket has no lock configuration
	plain := doObjectRequest(t, ts, http.MethodGet, ts.BucketURL("plain-bucket")+"?object-lock", nil, nil)
	defer DrainAndClose(plain)
	assert.Equal(t, http.StatusNotFound, plain.StatusCode)

	// Setting a default retention on a plain bucket is rejected
	putBody := []byte(`<ObjectLockConfiguration><ObjectLockEnabled>Enabled</ObjectLockEnabled><Rule><DefaultRetention><Mode>GOVERNANCE</Mode><Days>1</Days></DefaultRetention></Rule></ObjectLockConfiguration>`)
	put := doObjectRequest(t, ts, http.MethodPut, ts.BucketURL("plain-bucket")+"?object-lock", putBody, nil)
	defer DrainAndClose(put)
	assert.Equal(t, http.StatusConflict, put.StatusCode)
}

func TestObjectLock_GovernanceBlocksDeleteUntilBypassed(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	createLockedBucket(t, ts, "gov-bucket")

	putLockedObject(t, ts, "gov-bucket", "doc.txt", "GOVERNANCE", time.Now().Add(time.Hour))

	del := doObjectRequest(t, ts, http.MethodDelete, ts.ObjectURL("gov-bucket", "doc.txt"), nil, nil)
	DrainAndClose(del)
	assert.Equal(t, http.StatusForbidden, del.StatusCode, "governance-locked object must not be deleted")

	bypass := doObjectRequest(t, ts, http.MethodDelete, ts.ObjectURL("gov-bucket", "doc.txt"), nil, map[string]string{
		"X-Amz-Bypass-Governance-Retention": "true",
	})
	DrainAndClose(bypass)
	assert.Equal(t, http.StatusNoContent, bypass.StatusCode, "admin with bypass may delete governance-locked object")

	head := doObjectRequest(t, ts, http.MethodHead, ts.ObjectURL("gov-bucket", "doc.txt"), nil, nil)
	DrainAndClose(head)
	assert.Equal(t, http.StatusNotFound, head.StatusCode)
}

func TestObjectLock_ComplianceCannotBeBypassed(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	createLockedBucket(t, ts, "comp-bucket")

	putLockedObject(t, ts, "comp-bucket", "record.txt", "COMPLIANCE", time.Now().Add(time.Hour))

	resp := doObjectRequest(t, ts, http.MethodDelete, ts.ObjectURL("comp-bucket", "record.txt"), nil, map[string]string{
		"X-Amz-Bypass-Governance-Retention": "true",
	})
	DrainAndClose(resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "compliance retention cannot be bypassed")
}

func TestObjectLock_LegalHoldBlocksDeleteAndOverwrite(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	createLockedBucket(t, ts, "hold-bucket")

	ts.PutObject(t, "hold-bucket", "held.txt", "original")

	holdOn := []byte(`<LegalHold><Status>ON</Status></LegalHold>`)
	put := doObjectRequest(t, ts, http.MethodPut, ts.ObjectURL("hold-bucket", "held.txt")+"?legal-hold", holdOn, nil)
	DrainAndClose(put)
	require.Equal(t, http.StatusOK, put.StatusCode)

	del := doObjectRequest(t, ts, http.MethodDelete, ts.ObjectURL("hold-bucket", "held.txt"), nil, nil)
	DrainAndClose(del)
	assert.Equal(t, http.StatusForbidden, del.StatusCode, "legal hold blocks delete")

	overwrite := doObjectRequest(t, ts, http.MethodPut, ts.ObjectURL("hold-bucket", "held.txt"), []byte("replaced"), nil)
	DrainAndClose(overwrite)
	assert.Equal(t, http.StatusForbidden, overwrite.StatusCode, "legal hold blocks overwrite")

	holdOff := []byte(`<LegalHold><Status>OFF</Status></LegalHold>`)
	clearHold := doObjectRequest(t, ts, http.MethodPut, ts.ObjectURL("hold-bucket", "held.txt")+"?legal-hold", holdOff, nil)
	DrainAndClose(clearHold)
	require.Equal(t, http.StatusOK, clearHold.StatusCode)

	del2 := doObjectRequest(t, ts, http.MethodDelete, ts.ObjectURL("hold-bucket", "held.txt"), nil, nil)
	DrainAndClose(del2)
	assert.Equal(t, http.StatusNoContent, del2.StatusCode, "delete allowed once hold is released")
}

func TestObjectLock_DefaultRetentionAppliesToNewObjects(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	createLockedBucket(t, ts, "default-bucket")

	cfg := []byte(`<ObjectLockConfiguration><ObjectLockEnabled>Enabled</ObjectLockEnabled><Rule><DefaultRetention><Mode>GOVERNANCE</Mode><Days>1</Days></DefaultRetention></Rule></ObjectLockConfiguration>`)
	put := doObjectRequest(t, ts, http.MethodPut, ts.BucketURL("default-bucket")+"?object-lock", cfg, nil)
	DrainAndClose(put)
	require.Equal(t, http.StatusOK, put.StatusCode)

	ts.PutObject(t, "default-bucket", "auto.txt", "no explicit lock")

	del := doObjectRequest(t, ts, http.MethodDelete, ts.ObjectURL("default-bucket", "auto.txt"), nil, nil)
	DrainAndClose(del)
	assert.Equal(t, http.StatusForbidden, del.StatusCode, "default retention protects object written without lock headers")
}

func TestObjectLock_MultiDeleteReportsLockedKeys(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	createLockedBucket(t, ts, "multi-bucket")

	ts.PutObject(t, "multi-bucket", "free.txt", "free")
	putLockedObject(t, ts, "multi-bucket", "locked.txt", "GOVERNANCE", time.Now().Add(time.Hour))

	body := []byte(`<Delete><Object><Key>free.txt</Key></Object><Object><Key>locked.txt</Key></Object></Delete>`)
	resp := doObjectRequest(t, ts, http.MethodPost, ts.BucketURL("multi-bucket")+"?delete", body, nil)
	defer DrainAndClose(resp)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result struct {
		Deleted []struct {
			Key string `xml:"Key"`
		} `xml:"Deleted"`
		Errors []struct {
			Key  string `xml:"Key"`
			Code string `xml:"Code"`
		} `xml:"Error"`
	}
	require.NoError(t, xml.Unmarshal(raw, &result))

	require.Len(t, result.Deleted, 1)
	assert.Equal(t, "free.txt", result.Deleted[0].Key)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "locked.txt", result.Errors[0].Key)
	assert.Equal(t, "AccessDenied", result.Errors[0].Code)
}

func TestObjectLock_ComplianceRetentionCannotBeShortened(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	createLockedBucket(t, ts, "shorten-bucket")

	putLockedObject(t, ts, "shorten-bucket", "fixed.txt", "COMPLIANCE", time.Now().Add(2*time.Hour))

	shorter := []byte(`<Retention><Mode>COMPLIANCE</Mode><RetainUntilDate>` +
		time.Now().Add(time.Hour).UTC().Format(time.RFC3339) + `</RetainUntilDate></Retention>`)
	resp := doObjectRequest(t, ts, http.MethodPut, ts.ObjectURL("shorten-bucket", "fixed.txt")+"?retention", shorter, nil)
	DrainAndClose(resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "compliance retention cannot be shortened")

	get := doObjectRequest(t, ts, http.MethodGet, ts.ObjectURL("shorten-bucket", "fixed.txt")+"?retention", nil, nil)
	defer DrainAndClose(get)
	require.Equal(t, http.StatusOK, get.StatusCode)
	raw, err := io.ReadAll(get.Body)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "<Mode>COMPLIANCE</Mode>")
}

func TestObjectLock_HeadersRejectedOnUnlockedBucket(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	ts.CreateBucket(t, "unlocked-bucket")

	resp := doObjectRequest(t, ts, http.MethodPut, ts.ObjectURL("unlocked-bucket", "x.txt"), []byte("x"), map[string]string{
		"X-Amz-Object-Lock-Mode":              "GOVERNANCE",
		"X-Amz-Object-Lock-Retain-Until-Date": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	})
	DrainAndClose(resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
