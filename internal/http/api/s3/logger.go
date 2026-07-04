package s3

import (
	"context"
	"log/slog"

	"github.com/mallardduck/dirio/internal/logging"
)

func s3Logger(ctx context.Context) *slog.Logger {
	return logging.ComponentWithContext(ctx, "s3")
}
