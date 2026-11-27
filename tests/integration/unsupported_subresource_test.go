package integration

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sub-resource requests for operations DirIO does not implement must get an
// explicit 501 NotImplemented. Without a registered route they fall through to
// the plain bucket or object handler (CreateBucket, ListObjects, GetObject).

func TestUnsupportedBucketSubresources_Return501(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	ts.CreateBucket(t, "sub-bucket")

	cases := []struct {
		method string
		query  string
	}{
		{http.MethodGet, "logging"},
		{http.MethodPut, "logging"},
		{http.MethodGet, "notification"},
		{http.MethodPut, "notification"},
		{http.MethodGet, "requestPayment"},
		{http.MethodPut, "requestPayment"},
		{http.MethodGet, "analytics"},
		{http.MethodPut, "analytics"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" ?"+tc.query, func(t *testing.T) {
			body := []byte("<x/>")
			if tc.method == http.MethodGet {
				body = nil
			}
			resp := doObjectRequest(t, ts, tc.method, ts.BucketURL("sub-bucket")+"?"+tc.query, body, nil)
			DrainAndClose(resp)
			assert.Equal(t, http.StatusNotImplemented, resp.StatusCode)
		})
	}
}

func TestUnsupportedObjectSubresources_Return501(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	ts.CreateBucket(t, "sub-object-bucket")
	ts.PutObject(t, "sub-object-bucket", "obj.txt", "content")

	for _, query := range []string{"attributes", "torrent"} {
		t.Run("GET ?"+query, func(t *testing.T) {
			resp := doObjectRequest(t, ts, http.MethodGet, ts.ObjectURL("sub-object-bucket", "obj.txt")+"?"+query, nil, nil)
			DrainAndClose(resp)
			assert.Equal(t, http.StatusNotImplemented, resp.StatusCode)
		})
	}
}

// TestUnsupportedSubresourcePut_DoesNotModifyBucket checks that a PUT for an
// unsupported bucket sub-resource neither creates nor changes the bucket: the
// existing object must still be readable with its original content.
func TestUnsupportedSubresourcePut_DoesNotModifyBucket(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	ts.CreateBucket(t, "keep-bucket")
	ts.PutObject(t, "keep-bucket", "keep.txt", "original")

	for _, query := range []string{"logging", "notification", "requestPayment", "analytics"} {
		t.Run("PUT ?"+query, func(t *testing.T) {
			resp := doObjectRequest(t, ts, http.MethodPut, ts.BucketURL("keep-bucket")+"?"+query, []byte("<x/>"), nil)
			DrainAndClose(resp)
			assert.Equal(t, http.StatusNotImplemented, resp.StatusCode)
		})
	}

	get := doObjectRequest(t, ts, http.MethodGet, ts.ObjectURL("keep-bucket", "keep.txt"), nil, nil)
	defer DrainAndClose(get)
	require.Equal(t, http.StatusOK, get.StatusCode, "object must survive unsupported sub-resource PUTs")
	raw, err := io.ReadAll(get.Body)
	require.NoError(t, err)
	assert.Equal(t, "original", string(raw))
}
