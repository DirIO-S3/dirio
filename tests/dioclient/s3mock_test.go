//go:build docker

package dioclient_test

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	flociImage     = "floci/floci:latest"
	flociPort      = "4566/tcp"
	flociAccessKey = "test"
	flociSecretKey = "test"
)

// startFloci spins up a Floci container (drop-in LocalStack replacement) and returns
// the S3 endpoint URL. The container is terminated automatically when the test ends.
func startFloci(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        flociImage,
		ExposedPorts: []string{flociPort},
		WaitingFor:   wait.ForListeningPort(flociPort),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("S3Mock (Floci): could not start container: %v — is Docker available?", err)
		return ""
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("S3Mock (Floci): get container host: %v", err)
	}
	mappedPort, err := container.MappedPort(ctx, flociPort)
	if err != nil {
		t.Fatalf("S3Mock (Floci): get mapped port: %v", err)
	}

	return "http://" + host + ":" + mappedPort.Port()
}

func TestListBuckets_S3Mock(t *testing.T) {
	endpoint := startFloci(t)
	mc := minioSeedClient(t, endpoint, flociAccessKey, flociSecretKey, false)
	client := newClient(t, endpoint, flociAccessKey, flociSecretKey)
	runListBuckets(t, client, mc)
}

func TestListObjectsFlat_S3Mock(t *testing.T) {
	endpoint := startFloci(t)
	mc := minioSeedClient(t, endpoint, flociAccessKey, flociSecretKey, false)
	client := newClient(t, endpoint, flociAccessKey, flociSecretKey)
	runListObjectsFlat(t, client, mc)
}

func TestListObjectsWithPrefix_S3Mock(t *testing.T) {
	endpoint := startFloci(t)
	mc := minioSeedClient(t, endpoint, flociAccessKey, flociSecretKey, false)
	client := newClient(t, endpoint, flociAccessKey, flociSecretKey)
	runListObjectsWithPrefix(t, client, mc)
}

func TestListObjectsRecursiveVsDelimited_S3Mock(t *testing.T) {
	endpoint := startFloci(t)
	mc := minioSeedClient(t, endpoint, flociAccessKey, flociSecretKey, false)
	client := newClient(t, endpoint, flociAccessKey, flociSecretKey)
	runListObjectsRecursiveVsDelimited(t, client, mc)
}
