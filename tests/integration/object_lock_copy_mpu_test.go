package integration

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DirIO-S3/dirio/internal/testutil"
)

// lockHeaders returns the Object Lock write headers for a retention mode that
// runs for the next hour.
func lockHeaders(mode string) map[string]string {
	return map[string]string{
		"X-Amz-Object-Lock-Mode":              mode,
		"X-Amz-Object-Lock-Retain-Until-Date": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}
}

// copyObjectWithHeaders copies srcBucket/srcKey to dstBucket/dstKey, adding extra headers.
func copyObjectWithHeaders(t *testing.T, ts *testutil.TestServer, srcBucket, srcKey, dstBucket, dstKey string, extra map[string]string) *http.Response {
	t.Helper()
	headers := map[string]string{"X-Amz-Copy-Source": "/" + srcBucket + "/" + srcKey}
	for k, v := range extra {
		headers[k] = v
	}
	return doObjectRequest(t, ts, http.MethodPut, ts.ObjectURL(dstBucket, dstKey), nil, headers)
}

// deleteObject sends a DELETE for bucket/key and returns the response.
func deleteObject(t *testing.T, ts *testutil.TestServer, bucket, key string, extra map[string]string) *http.Response {
	t.Helper()
	return doObjectRequest(t, ts, http.MethodDelete, ts.ObjectURL(bucket, key), nil, extra)
}

func TestObjectLock_CopyObjectAppliesLockHeaders(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	createLockedBucket(t, ts, "copy-lock-bucket")
	ts.PutObject(t, "copy-lock-bucket", "src.txt", "source content")

	resp := copyObjectWithHeaders(t, ts, "copy-lock-bucket", "src.txt", "copy-lock-bucket", "dst.txt", lockHeaders("GOVERNANCE"))
	DrainAndClose(resp)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	del := deleteObject(t, ts, "copy-lock-bucket", "dst.txt", nil)
	DrainAndClose(del)
	assert.Equal(t, http.StatusForbidden, del.StatusCode, "destination of a locked copy is protected")

	// The source was not given a lock, so it can still be deleted.
	srcDel := deleteObject(t, ts, "copy-lock-bucket", "src.txt", nil)
	DrainAndClose(srcDel)
	assert.Equal(t, http.StatusNoContent, srcDel.StatusCode)
}

func TestObjectLock_CopyObjectLegalHold(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	createLockedBucket(t, ts, "copy-hold-bucket")
	ts.PutObject(t, "copy-hold-bucket", "src.txt", "source content")

	resp := copyObjectWithHeaders(t, ts, "copy-hold-bucket", "src.txt", "copy-hold-bucket", "held.txt", map[string]string{
		"X-Amz-Object-Lock-Legal-Hold": "ON",
	})
	DrainAndClose(resp)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	del := deleteObject(t, ts, "copy-hold-bucket", "held.txt", map[string]string{
		"X-Amz-Bypass-Governance-Retention": "true",
	})
	DrainAndClose(del)
	assert.Equal(t, http.StatusForbidden, del.StatusCode, "legal hold blocks delete even with bypass")
}

func TestObjectLock_CopyObjectOverCompliantDestinationIsBlocked(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	createLockedBucket(t, ts, "copy-over-bucket")
	ts.PutObject(t, "copy-over-bucket", "src.txt", "new content")
	putLockedObject(t, ts, "copy-over-bucket", "dst.txt", "COMPLIANCE", time.Now().Add(time.Hour))

	resp := copyObjectWithHeaders(t, ts, "copy-over-bucket", "src.txt", "copy-over-bucket", "dst.txt", nil)
	DrainAndClose(resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "copy cannot replace a COMPLIANCE-locked object")
}

func TestObjectLock_CopyObjectHeadersRejectedOnUnlockedBucket(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	ts.CreateBucket(t, "copy-unlocked-bucket")
	ts.PutObject(t, "copy-unlocked-bucket", "src.txt", "source content")

	resp := copyObjectWithHeaders(t, ts, "copy-unlocked-bucket", "src.txt", "copy-unlocked-bucket", "dst.txt", lockHeaders("GOVERNANCE"))
	DrainAndClose(resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// initiateMultipart starts a multipart upload with the given extra headers and returns its UploadId.
func initiateMultipart(t *testing.T, ts *testutil.TestServer, bucket, key string, extra map[string]string) (*http.Response, string) {
	t.Helper()
	resp := doObjectRequest(t, ts, http.MethodPost, ts.ObjectURL(bucket, key)+"?uploads", nil, extra)
	if resp.StatusCode != http.StatusOK {
		return resp, ""
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	return resp, extractXMLValue(string(body), "UploadId")
}

// completeSinglePartUpload uploads one part and completes the upload.
func completeSinglePartUpload(t *testing.T, ts *testutil.TestServer, bucket, key, uploadID string) {
	t.Helper()
	content := []byte(strings.Repeat("m", 1024))
	partURL := fmt.Sprintf("%s?partNumber=1&uploadId=%s", ts.ObjectURL(bucket, key), uploadID)
	partReq, err := http.NewRequest(http.MethodPut, partURL, strings.NewReader(string(content)))
	require.NoError(t, err)
	partReq.ContentLength = int64(len(content))
	ts.SignRequest(partReq, content)
	partResp, err := http.DefaultClient.Do(partReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, partResp.StatusCode)
	etag := partResp.Header.Get("ETag")
	DrainAndClose(partResp)

	completeXML := fmt.Sprintf(`<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>%s</ETag></Part></CompleteMultipartUpload>`, etag)
	complete := doObjectRequest(t, ts, http.MethodPost, fmt.Sprintf("%s?uploadId=%s", ts.ObjectURL(bucket, key), uploadID), []byte(completeXML), nil)
	DrainAndClose(complete)
	require.Equal(t, http.StatusOK, complete.StatusCode)
}

func TestObjectLock_MultipartUploadAppliesLockHeaders(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	createLockedBucket(t, ts, "mpu-lock-bucket")

	init, uploadID := initiateMultipart(t, ts, "mpu-lock-bucket", "big.bin", lockHeaders("COMPLIANCE"))
	DrainAndClose(init)
	require.Equal(t, http.StatusOK, init.StatusCode)
	require.NotEmpty(t, uploadID)

	completeSinglePartUpload(t, ts, "mpu-lock-bucket", "big.bin", uploadID)

	del := deleteObject(t, ts, "mpu-lock-bucket", "big.bin", nil)
	DrainAndClose(del)
	assert.Equal(t, http.StatusForbidden, del.StatusCode, "multipart object carries the lock set at initiate")
}

func TestObjectLock_MultipartUploadLegalHold(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	createLockedBucket(t, ts, "mpu-hold-bucket")

	init, uploadID := initiateMultipart(t, ts, "mpu-hold-bucket", "held.bin", map[string]string{
		"X-Amz-Object-Lock-Legal-Hold": "ON",
	})
	DrainAndClose(init)
	require.Equal(t, http.StatusOK, init.StatusCode)

	completeSinglePartUpload(t, ts, "mpu-hold-bucket", "held.bin", uploadID)

	del := deleteObject(t, ts, "mpu-hold-bucket", "held.bin", nil)
	DrainAndClose(del)
	assert.Equal(t, http.StatusForbidden, del.StatusCode)
}

func TestObjectLock_MultipartUploadUsesBucketDefaultWithoutHeaders(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	createLockedBucket(t, ts, "mpu-default-bucket")

	cfg := []byte(`<ObjectLockConfiguration><ObjectLockEnabled>Enabled</ObjectLockEnabled><Rule><DefaultRetention><Mode>GOVERNANCE</Mode><Days>1</Days></DefaultRetention></Rule></ObjectLockConfiguration>`)
	put := doObjectRequest(t, ts, http.MethodPut, ts.BucketURL("mpu-default-bucket")+"?object-lock", cfg, nil)
	DrainAndClose(put)
	require.Equal(t, http.StatusOK, put.StatusCode)

	init, uploadID := initiateMultipart(t, ts, "mpu-default-bucket", "auto.bin", nil)
	DrainAndClose(init)
	require.Equal(t, http.StatusOK, init.StatusCode)

	completeSinglePartUpload(t, ts, "mpu-default-bucket", "auto.bin", uploadID)

	del := deleteObject(t, ts, "mpu-default-bucket", "auto.bin", nil)
	DrainAndClose(del)
	assert.Equal(t, http.StatusForbidden, del.StatusCode, "bucket default retention applies to multipart objects")
}

func TestObjectLock_MultipartInitiateRejectsLockOnUnlockedBucket(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	ts.CreateBucket(t, "mpu-unlocked-bucket")

	init, _ := initiateMultipart(t, ts, "mpu-unlocked-bucket", "x.bin", lockHeaders("GOVERNANCE"))
	DrainAndClose(init)
	assert.Equal(t, http.StatusBadRequest, init.StatusCode)
}

// copyPolicyDoc grants the copy actions on bucket, plus the bypass permission when bypass is true.
func copyPolicyDoc(bucket string, bypass bool) string {
	actions := `"s3:GetObject", "s3:PutObject", "s3:CopyObject"`
	if bypass {
		actions += `, "s3:BypassGovernanceRetention"`
	}
	return fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Action": [%s],
				"Resource": ["arn:aws:s3:::%s/*"]
			}
		]
	}`, actions, bucket)
}

// TestObjectLock_CopyObjectBypassRequiresPermission checks that the bypass header
// on CopyObject is honored only for callers holding s3:BypassGovernanceRetention.
func TestObjectLock_CopyObjectBypassRequiresPermission(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Cleanup()
	const bucket = "copy-bypass-bucket"
	createLockedBucket(t, ts, bucket)
	ts.PutObject(t, bucket, "src.txt", "replacement")
	putLockedObject(t, ts, bucket, "dst.txt", "GOVERNANCE", time.Now().Add(time.Hour))

	bypassHeader := map[string]string{
		"X-Amz-Copy-Source":                 "/" + bucket + "/src.txt",
		"X-Amz-Bypass-Governance-Retention": "true",
	}
	copyAs := func(accessKey, secretKey string) *http.Response {
		req, err := http.NewRequest(http.MethodPut, ts.ObjectURL(bucket, "dst.txt"), http.NoBody)
		require.NoError(t, err)
		for k, v := range bypassHeader {
			req.Header.Set(k, v)
		}
		SignRequestWithCredentials(req, nil, accessKey, secretKey)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		return resp
	}

	// Copy user: may copy, but holds no bypass permission.
	createIAMUser(t, ts, "copynobypass", "copynobypasssecret")
	updateGroupMembers(t, ts, "copy-nobypass-group", []string{"copynobypass"}, false)
	createIAMPolicy(t, ts, "copy-nobypass-policy", copyPolicyDoc(bucket, false))
	attachGroupPolicy(t, ts, "copy-nobypass-policy", "copy-nobypass-group")

	denied := copyAs("copynobypass", "copynobypasssecret")
	DrainAndClose(denied)
	assert.Equal(t, http.StatusForbidden, denied.StatusCode, "bypass header without s3:BypassGovernanceRetention must be denied")

	// Control: a caller with the bypass permission can replace the object.
	createIAMUser(t, ts, "copybypass", "copybypasssecret")
	updateGroupMembers(t, ts, "copy-bypass-group", []string{"copybypass"}, false)
	createIAMPolicy(t, ts, "copy-bypass-policy", copyPolicyDoc(bucket, true))
	attachGroupPolicy(t, ts, "copy-bypass-policy", "copy-bypass-group")

	allowed := copyAs("copybypass", "copybypasssecret")
	DrainAndClose(allowed)
	assert.Equal(t, http.StatusOK, allowed.StatusCode, "caller with s3:BypassGovernanceRetention may bypass")
}
