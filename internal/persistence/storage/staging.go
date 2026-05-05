package storage

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/google/uuid"

	"github.com/mallardduck/dirio/internal/consts"
	"github.com/mallardduck/dirio/internal/persistence/path"
)

type stagingManager struct {
	rootFS billy.Filesystem
	log    *slog.Logger
}

// stageObject creates a temp file under .dirio-uploads/<bucket>/<uuid>.
// Returns the open file and its rootFS-relative path (used for commit/abort).
func (m *stagingManager) stageObject(bucket string) (billy.File, string, error) {
	stagingFS, err := path.NewUploadStagingFS(m.rootFS, bucket)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get staging filesystem: %w", err)
	}
	tmpName := uuid.New().String()
	tmpFile, err := stagingFS.Create(tmpName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create staged temp file: %w", err)
	}
	stagedPath := filepath.Join(consts.DirIOUploadsDir, bucket, tmpName)
	return tmpFile, stagedPath, nil
}

// commitObject atomically renames the staged file to its final bucket path.
// Both staging and bucket directories live under the same rootFS, so the
// cross-directory rename is a cheap metadata-only operation on the same device.
func (m *stagingManager) commitObject(stagedPath, bucket, objectPath string) error {
	finalPath := filepath.Join(bucket, objectPath)
	if err := m.rootFS.Rename(stagedPath, finalPath); err != nil {
		return fmt.Errorf("failed to commit staged object: %w", err)
	}
	return nil
}

// abortObject removes a staged temp file on write failure or context cancellation.
func (m *stagingManager) abortObject(stagedPath string) {
	if err := m.rootFS.Remove(stagedPath); err != nil {
		m.log.Warn("failed to remove staged temp file", "path", stagedPath, "error", err)
	}
}

// getUploadStagingFS returns a billy.Filesystem scoped to .dirio-uploads/<bucket>.
// Multipart operations within this FS use the uploadID as the top-level subdirectory.
func (m *stagingManager) getUploadStagingFS(bucket string) (billy.Filesystem, error) {
	return path.NewUploadStagingFS(m.rootFS, bucket)
}

// cleanupMultipartUpload removes all staging state for a multipart uploadID.
func (m *stagingManager) cleanupMultipartUpload(bucket, uploadID string) error {
	stagingFS, err := path.NewUploadStagingFS(m.rootFS, bucket)
	if err != nil {
		return fmt.Errorf("failed to get staging filesystem: %w", err)
	}
	return util.RemoveAll(stagingFS, uploadID)
}

// cleanup sweeps .dirio-uploads/ on startup and removes all orphaned staging state
// left over from a previous crash. Called once during Storage initialization.
func (m *stagingManager) cleanup() {
	buckets, err := m.rootFS.ReadDir(consts.DirIOUploadsDir)
	if err != nil {
		return // staging root doesn't exist yet — nothing to sweep
	}
	for _, b := range buckets {
		stagingPath := filepath.Join(consts.DirIOUploadsDir, b.Name())
		entries, err := m.rootFS.ReadDir(stagingPath)
		if err != nil {
			continue
		}
		for _, e := range entries {
			entryPath := filepath.Join(stagingPath, e.Name())
			if err := util.RemoveAll(m.rootFS, entryPath); err != nil {
				m.log.Warn("failed to remove orphaned staging entry", "path", entryPath, "error", err)
			} else {
				m.log.Info("removed orphaned staging entry", "path", entryPath)
			}
		}
	}
}
