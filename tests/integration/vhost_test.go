package integration

import (
	"io"
	"net/http"
	"testing"

	"github.com/mallardduck/dirio/internal/testutil"
)

const testCanonicalDomain = "s3.vhost.test"

// TestVHostRoundTrip_PutPathGetVHost verifies an object PUT via path-style
// addressing can be read back via virtual-hosted-style addressing.
func TestVHostRoundTrip_PutPathGetVHost(t *testing.T) {
	ts := testutil.NewWithCanonicalDomain(t, testCanonicalDomain)
	ts.CreateBucket(t, "vhost-bucket-a")
	ts.PutObject(t, "vhost-bucket-a", "hello.txt", "hello from path-style")

	req, err := ts.NewVHostRequest(http.MethodGet, "vhost-bucket-a."+testCanonicalDomain, "/hello.txt", nil)
	if err != nil {
		t.Fatalf("build vhost request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("vhost GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("vhost GET status = %d, body: %s", resp.StatusCode, body)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello from path-style" {
		t.Fatalf("body = %q, want %q", body, "hello from path-style")
	}
}

// TestVHostRoundTrip_PutVHostGetPath verifies the reverse: PUT via
// virtual-hosted-style, GET back via path-style.
func TestVHostRoundTrip_PutVHostGetPath(t *testing.T) {
	ts := testutil.NewWithCanonicalDomain(t, testCanonicalDomain)
	ts.CreateBucket(t, "vhost-bucket-b")

	content := "hello from vhost-style"
	putReq, err := ts.NewVHostRequest(http.MethodPut, "vhost-bucket-b."+testCanonicalDomain, "/greeting.txt", []byte(content))
	if err != nil {
		t.Fatalf("build vhost PUT request: %v", err)
	}
	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatalf("vhost PUT: %v", err)
	}
	testutil.DrainAndClose(putResp)
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("vhost PUT status = %d", putResp.StatusCode)
	}

	getReq, err := ts.NewRequest(http.MethodGet, ts.ObjectURL("vhost-bucket-b", "greeting.txt"), nil)
	if err != nil {
		t.Fatalf("build path GET request: %v", err)
	}
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatalf("path GET: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		t.Fatalf("path GET status = %d, body: %s", getResp.StatusCode, body)
	}
	body, _ := io.ReadAll(getResp.Body)
	if string(body) != content {
		t.Fatalf("body = %q, want %q", body, content)
	}
}

// TestVHostDoesNotServeControlPlaneRoutes verifies /.dirio/* and
// /minio/admin/* are not served by the vhost router even when a vhost Host
// header happens to be present — control-plane traffic only ever goes
// through the path router.
func TestVHostDoesNotServeControlPlaneRoutes(t *testing.T) {
	ts := testutil.NewWithCanonicalDomain(t, testCanonicalDomain)
	ts.CreateBucket(t, "vhost-bucket-c")

	paths := []string{"/.dirio/routes", "/minio/admin/v3/info"}
	for _, p := range paths {
		req, err := ts.NewVHostRequest(http.MethodGet, "vhost-bucket-c."+testCanonicalDomain, p, nil)
		if err != nil {
			t.Fatalf("build vhost request for %s: %v", p, err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("vhost request %s: %v", p, err)
		}
		testutil.DrainAndClose(resp)
		if resp.StatusCode == http.StatusOK {
			t.Fatalf("path %s served 200 via vhost router — expected it to be unreachable", p)
		}
	}
}

// TestVHostMalformedBucketLabel verifies a malformed bucket label carried in
// Host still goes through S3 bucket-name validation rather than reaching
// handlers unvalidated.
func TestVHostMalformedBucketLabel(t *testing.T) {
	ts := testutil.NewWithCanonicalDomain(t, testCanonicalDomain)

	// "ab" is below the 3-character minimum bucket name length.
	req, err := ts.NewVHostRequest(http.MethodGet, "ab."+testCanonicalDomain, "/", nil)
	if err != nil {
		t.Fatalf("build vhost request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("vhost request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 400, body: %s", resp.StatusCode, body)
	}
}

// TestPathStyleUnaffectedByCanonicalDomain verifies bare-IP / non-vhost Host
// requests still work exactly as path-style, even with CanonicalDomain set.
func TestPathStyleUnaffectedByCanonicalDomain(t *testing.T) {
	ts := testutil.NewWithCanonicalDomain(t, testCanonicalDomain)
	ts.CreateBucket(t, "vhost-bucket-d")
	ts.PutObject(t, "vhost-bucket-d", "k.txt", "still path-style")

	req, err := ts.NewRequest(http.MethodGet, ts.ObjectURL("vhost-bucket-d", "k.txt"), nil)
	if err != nil {
		t.Fatalf("build path request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("path GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body: %s", resp.StatusCode, body)
	}
}
