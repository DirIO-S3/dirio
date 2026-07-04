package integration

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCopyRequest builds a signed PUT request for bucket/key with the given
// X-Amz-Copy-Source header (and any extra headers) set before signing.
func newCopyRequest(t *testing.T, ts *TestServer, bucket, key, copySource string, extraHeaders map[string]string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, ts.ObjectURL(bucket, key), http.NoBody)
	require.NoError(t, err)
	req.Header.Set("X-Amz-Copy-Source", copySource)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	ts.SignRequest(req, nil)
	return req
}

func TestCopyObject_Basic(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()

	ts.CreateBucket(t, "src-bucket")
	ts.CreateBucket(t, "dst-bucket")
	ts.PutObject(t, "src-bucket", "source.txt", "original content")

	req := newCopyRequest(t, ts, "dst-bucket", "dest.txt", "/src-bucket/source.txt", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))

	getReq, err := ts.NewRequest(http.MethodGet, ts.ObjectURL("dst-bucket", "dest.txt"), nil)
	require.NoError(t, err)
	getResp, err := http.DefaultClient.Do(getReq)
	require.NoError(t, err)
	defer getResp.Body.Close()
	require.Equal(t, http.StatusOK, getResp.StatusCode)
	gotBody, _ := io.ReadAll(getResp.Body)
	assert.Equal(t, "original content", string(gotBody))
}

func TestCopyObject_NoLeadingSlashSourceForm(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()

	ts.CreateBucket(t, "src-bucket")
	ts.PutObject(t, "src-bucket", "source.txt", "content")

	req := newCopyRequest(t, ts, "src-bucket", "dest.txt", "src-bucket/source.txt", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCopyObject_URLEncodedSourceKey(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()

	ts.CreateBucket(t, "src-bucket")
	ts.PutObject(t, "src-bucket", "my file.txt", "spaced key content")

	req := newCopyRequest(t, ts, "src-bucket", "dest.txt", "/src-bucket/my%20file.txt", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, string(body))
}

func TestCopyObject_SourceNotFound(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()

	ts.CreateBucket(t, "src-bucket")
	ts.CreateBucket(t, "dst-bucket")

	req := newCopyRequest(t, ts, "dst-bucket", "dest.txt", "/src-bucket/does-not-exist.txt", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestCopyObject_MetadataDirectiveCopyPreservesSourceMetadata(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()

	ts.CreateBucket(t, "src-bucket")

	putReq, err := http.NewRequest(http.MethodPut, ts.ObjectURL("src-bucket", "source.txt"), strings.NewReader("hello"))
	require.NoError(t, err)
	putReq.ContentLength = 5
	putReq.Header.Set("Content-Type", "text/x-custom")
	putReq.Header.Set("X-Amz-Meta-Foo", "bar")
	ts.SignRequest(putReq, []byte("hello"))
	putResp, err := http.DefaultClient.Do(putReq)
	require.NoError(t, err)
	DrainAndClose(putResp)
	require.Equal(t, http.StatusOK, putResp.StatusCode)

	// Default directive is COPY: no metadata headers sent, source metadata carries over.
	req := newCopyRequest(t, ts, "src-bucket", "dest.txt", "/src-bucket/source.txt", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	headReq, err := ts.NewRequest(http.MethodHead, ts.ObjectURL("src-bucket", "dest.txt"), nil)
	require.NoError(t, err)
	headResp, err := http.DefaultClient.Do(headReq)
	require.NoError(t, err)
	defer headResp.Body.Close()
	assert.Equal(t, "text/x-custom", headResp.Header.Get("Content-Type"))
	assert.Equal(t, "bar", headResp.Header.Get("X-Amz-Meta-Foo"))
}

func TestCopyObject_MetadataDirectiveReplaceUsesNewMetadata(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()

	ts.CreateBucket(t, "src-bucket")

	putReq, err := http.NewRequest(http.MethodPut, ts.ObjectURL("src-bucket", "source.txt"), strings.NewReader("hello"))
	require.NoError(t, err)
	putReq.ContentLength = 5
	putReq.Header.Set("Content-Type", "text/x-custom")
	putReq.Header.Set("X-Amz-Meta-Foo", "bar")
	ts.SignRequest(putReq, []byte("hello"))
	putResp, err := http.DefaultClient.Do(putReq)
	require.NoError(t, err)
	DrainAndClose(putResp)
	require.Equal(t, http.StatusOK, putResp.StatusCode)

	req := newCopyRequest(t, ts, "src-bucket", "dest.txt", "/src-bucket/source.txt", map[string]string{
		"X-Amz-Metadata-Directive": "REPLACE",
		"Content-Type":             "application/json",
		"X-Amz-Meta-Foo":           "baz",
	})
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))

	headReq, err := ts.NewRequest(http.MethodHead, ts.ObjectURL("src-bucket", "dest.txt"), nil)
	require.NoError(t, err)
	headResp, err := http.DefaultClient.Do(headReq)
	require.NoError(t, err)
	defer headResp.Body.Close()
	assert.Equal(t, "application/json", headResp.Header.Get("Content-Type"))
	assert.Equal(t, "baz", headResp.Header.Get("X-Amz-Meta-Foo"))
}

func TestCopyObject_IfMatchConditions(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()

	ts.CreateBucket(t, "src-bucket")
	ts.PutObject(t, "src-bucket", "source.txt", "content")

	headReq, err := ts.NewRequest(http.MethodHead, ts.ObjectURL("src-bucket", "source.txt"), nil)
	require.NoError(t, err)
	headResp, err := http.DefaultClient.Do(headReq)
	require.NoError(t, err)
	sourceETag := headResp.Header.Get("ETag")
	DrainAndClose(headResp)
	require.NotEmpty(t, sourceETag)

	t.Run("If-Match with correct ETag succeeds", func(t *testing.T) {
		req := newCopyRequest(t, ts, "src-bucket", "dest-match-ok.txt", "/src-bucket/source.txt", map[string]string{
			"X-Amz-Copy-Source-If-Match": sourceETag,
		})
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("If-Match with wrong ETag fails with 412", func(t *testing.T) {
		req := newCopyRequest(t, ts, "src-bucket", "dest-match-fail.txt", "/src-bucket/source.txt", map[string]string{
			"X-Amz-Copy-Source-If-Match": `"deadbeefdeadbeefdeadbeefdeadbeef"`,
		})
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)
	})

	t.Run("If-None-Match with matching ETag fails with 412", func(t *testing.T) {
		req := newCopyRequest(t, ts, "src-bucket", "dest-none-match-fail.txt", "/src-bucket/source.txt", map[string]string{
			"X-Amz-Copy-Source-If-None-Match": sourceETag,
		})
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)
	})

	t.Run("If-None-Match with different ETag succeeds", func(t *testing.T) {
		req := newCopyRequest(t, ts, "src-bucket", "dest-none-match-ok.txt", "/src-bucket/source.txt", map[string]string{
			"X-Amz-Copy-Source-If-None-Match": `"deadbeefdeadbeefdeadbeefdeadbeef"`,
		})
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestCopyObject_ModifiedSinceConditions(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()

	ts.CreateBucket(t, "src-bucket")
	ts.PutObject(t, "src-bucket", "source.txt", "content")

	future := time.Now().Add(1 * time.Hour).UTC().Format(http.TimeFormat)
	past := time.Now().Add(-1 * time.Hour).UTC().Format(http.TimeFormat)

	t.Run("If-Unmodified-Since in the future succeeds", func(t *testing.T) {
		req := newCopyRequest(t, ts, "src-bucket", "dest-unmod-ok.txt", "/src-bucket/source.txt", map[string]string{
			"X-Amz-Copy-Source-If-Unmodified-Since": future,
		})
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("If-Unmodified-Since in the past fails with 412", func(t *testing.T) {
		req := newCopyRequest(t, ts, "src-bucket", "dest-unmod-fail.txt", "/src-bucket/source.txt", map[string]string{
			"X-Amz-Copy-Source-If-Unmodified-Since": past,
		})
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)
	})

	t.Run("If-Modified-Since in the past succeeds", func(t *testing.T) {
		req := newCopyRequest(t, ts, "src-bucket", "dest-mod-ok.txt", "/src-bucket/source.txt", map[string]string{
			"X-Amz-Copy-Source-If-Modified-Since": past,
		})
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("If-Modified-Since in the future fails with 412", func(t *testing.T) {
		req := newCopyRequest(t, ts, "src-bucket", "dest-mod-fail.txt", "/src-bucket/source.txt", map[string]string{
			"X-Amz-Copy-Source-If-Modified-Since": future,
		})
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)
	})
}
