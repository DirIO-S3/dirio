//go:build docker

package clients_test

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const ContainerRepo = "dirio-test-local"

// imageBuildMu serializes image builds and container creation. Parallel tests share
// image tags (e.g. dirio-test-local:mc-latest), and a concurrent build of the same tag
// can replace the image between another test's build and its container create,
// which fails with "No such image".
var imageBuildMu sync.Mutex

// startClientContainer builds the image (if needed) and creates the container while
// holding imageBuildMu, then starts it outside the lock so tests still run in parallel.
func startClientContainer(ctx context.Context, req testcontainers.ContainerRequest) (testcontainers.Container, error) {
	imageBuildMu.Lock()
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          false,
	})
	imageBuildMu.Unlock()
	if err != nil {
		return nil, err
	}
	if err := ctr.Start(ctx); err != nil {
		_ = ctr.Terminate(ctx)
		return nil, err
	}
	return ctr, nil
}

// hostGatewayHosts returns the extra host mapping needed so containers can reach
// the test server running on the host.
//
// On Linux (including GitHub Actions), host.docker.internal is not defined in
// containers by default — it requires an explicit --add-host entry pointing at
// the Docker bridge gateway. On macOS/Windows Docker Desktop the name is already
// injected by the daemon, so no override is needed.
func hostGatewayHosts() []string {
	if runtime.GOOS == "linux" {
		return []string{"host.docker.internal:host-gateway"}
	}
	return nil
}

func setGateways(config *container.HostConfig) {
	if extraGateways := hostGatewayHosts(); extraGateways != nil {
		config.ExtraHosts = append(config.ExtraHosts, extraGateways...)
	}
}

// AwsClientContainer returns a container request for the AWS CLI client
// with the pre-built image. The Docker image is built once and cached for reuse.
func AwsClientContainer(envMap map[string]string) testcontainers.ContainerRequest {
	return testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    ".",
			Dockerfile: "docker/Dockerfile-aws",
			Repo:       ContainerRepo,
			Tag:        "aws",
			KeepImage:  true, // Keep the image for reuse across tests
		},
		Env:                envMap,
		HostConfigModifier: setGateways,
		Entrypoint:         []string{"./awscli.sh"},
		WaitingFor:         wait.ForExit().WithExitTimeout(3 * time.Minute),
	}
}

// MinioClientContainer returns a container request for the MinIO mc client
// using the latest mc binary. The Docker image is built once and cached for reuse.
func MinioClientContainer(envMap map[string]string) testcontainers.ContainerRequest {
	return testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    ".",
			Dockerfile: "docker/Dockerfile-minio",
			Repo:       ContainerRepo,
			Tag:        "mc-latest",
			KeepImage:  true, // Keep the image for reuse across tests
		},
		Env:                envMap,
		HostConfigModifier: setGateways,
		Cmd:                []string{"./mc.sh"},
		WaitingFor:         wait.ForExit().WithExitTimeout(3 * time.Minute),
	}
}

// MinioVersionedClientContainer returns a container request for a specific pinned
// mc release. The release string must match a tag on https://github.com/minio/mc/releases,
// e.g. "RELEASE.2025-08-13T08-35-41Z". Each version gets its own cached image so
// multiple pinned versions can coexist in the same test run.
func MinioVersionedClientContainer(release string, envMap map[string]string) testcontainers.ContainerRequest {
	return testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    ".",
			Dockerfile: "docker/Dockerfile-minio-versioned",
			Repo:       ContainerRepo,
			Tag:        "mc-" + release,
			BuildArgs:  map[string]*string{"MC_RELEASE": &release},
			KeepImage:  true, // Cache per version so CI doesn't rebuild on every run
		},
		Env:                envMap,
		HostConfigModifier: setGateways,
		Cmd:                []string{"./mc.sh"},
		WaitingFor:         wait.ForExit().WithExitTimeout(3 * time.Minute),
	}
}

// Boto3ClientContainer returns a container request for the boto3 client
// with the pre-built image. The Docker image is built once and cached for reuse.
func Boto3ClientContainer(envMap map[string]string) testcontainers.ContainerRequest {
	return testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    ".",
			Dockerfile: "docker/Dockerfile-boto3",
			Repo:       ContainerRepo,
			Tag:        "boto3",
			KeepImage:  true, // Keep the image for reuse across tests
		},
		Env:                envMap,
		HostConfigModifier: setGateways,
		Cmd:                []string{"-c", "python3 ./boto3cli.py"},
		WaitingFor:         wait.ForExit().WithExitTimeout(2 * time.Minute),
	}
}
