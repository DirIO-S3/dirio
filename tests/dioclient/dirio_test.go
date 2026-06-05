package dioclient_test

import (
	"testing"

	"github.com/mallardduck/dirio/internal/testutil"
)

// TestServer is a type alias so test helpers can reference testutil.TestServer
// without importing the full package name everywhere.
type TestServer = testutil.TestServer

func TestListBuckets_DirIO(t *testing.T) {
	ts := testutil.New(t)
	mc := minioSeedClient(t, ts.BaseURL, ts.AccessKey, ts.SecretKey, false)
	client := newClient(t, ts.BaseURL, ts.AccessKey, ts.SecretKey)
	runListBuckets(t, client, mc)
}

func TestListObjectsFlat_DirIO(t *testing.T) {
	ts := testutil.New(t)
	mc := minioSeedClient(t, ts.BaseURL, ts.AccessKey, ts.SecretKey, false)
	client := newClient(t, ts.BaseURL, ts.AccessKey, ts.SecretKey)
	runListObjectsFlat(t, client, mc)
}

func TestListObjectsWithPrefix_DirIO(t *testing.T) {
	ts := testutil.New(t)
	mc := minioSeedClient(t, ts.BaseURL, ts.AccessKey, ts.SecretKey, false)
	client := newClient(t, ts.BaseURL, ts.AccessKey, ts.SecretKey)
	runListObjectsWithPrefix(t, client, mc)
}

func TestListObjectsRecursiveVsDelimited_DirIO(t *testing.T) {
	ts := testutil.New(t)
	mc := minioSeedClient(t, ts.BaseURL, ts.AccessKey, ts.SecretKey, false)
	client := newClient(t, ts.BaseURL, ts.AccessKey, ts.SecretKey)
	runListObjectsRecursiveVsDelimited(t, client, mc)
}
