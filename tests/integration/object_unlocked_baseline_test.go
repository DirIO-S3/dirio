package integration

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin existing behavior for buckets without Object Lock. They must
// keep passing as Object Lock is added: unlocked objects are still deleted and
// overwritten normally.

// baselineRequest sends a signed request with an optional body.
func baselineRequest(t *testing.T, ts *TestServer, method, url string, body []byte, headers map[string]string) *http.Response {
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

func TestUnlockedBucket_DeleteRemovesObject(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	ts.CreateBucket(t, "plain-delete")
	ts.PutObject(t, "plain-delete", "gone.txt", "bye")

	del := baselineRequest(t, ts, http.MethodDelete, ts.ObjectURL("plain-delete", "gone.txt"), nil, nil)
	DrainAndClose(del)
	assert.Equal(t, http.StatusNoContent, del.StatusCode)

	head := baselineRequest(t, ts, http.MethodHead, ts.ObjectURL("plain-delete", "gone.txt"), nil, nil)
	DrainAndClose(head)
	assert.Equal(t, http.StatusNotFound, head.StatusCode)
}

func TestUnlockedBucket_OverwriteReplacesContent(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	ts.CreateBucket(t, "plain-overwrite")
	ts.PutObject(t, "plain-overwrite", "doc.txt", "v1")

	put := baselineRequest(t, ts, http.MethodPut, ts.ObjectURL("plain-overwrite", "doc.txt"), []byte("v2"), nil)
	DrainAndClose(put)
	require.Equal(t, http.StatusOK, put.StatusCode)

	get := baselineRequest(t, ts, http.MethodGet, ts.ObjectURL("plain-overwrite", "doc.txt"), nil, nil)
	defer DrainAndClose(get)
	require.Equal(t, http.StatusOK, get.StatusCode)
	body, err := io.ReadAll(get.Body)
	require.NoError(t, err)
	assert.Equal(t, "v2", string(body))
}

func TestUnlockedBucket_MultiDeleteDeletesAllKeys(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	ts.CreateBucket(t, "plain-multi")
	ts.PutObject(t, "plain-multi", "a.txt", "a")
	ts.PutObject(t, "plain-multi", "b.txt", "b")

	body := []byte(`<Delete><Object><Key>a.txt</Key></Object><Object><Key>b.txt</Key></Object></Delete>`)
	resp := baselineRequest(t, ts, http.MethodPost, ts.BucketURL("plain-multi")+"?delete", body, nil)
	raw, err := io.ReadAll(resp.Body)
	DrainAndClose(resp)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotContains(t, string(raw), "<Error>")
	assert.Equal(t, 2, strings.Count(string(raw), "<Deleted>"))
}
