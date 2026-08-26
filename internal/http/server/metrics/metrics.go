package metrics

import (
	"net/http"

	"github.com/mallardduck/teapot-router/pkg/teapot"

	"github.com/DirIO-S3/dirio/internal/telemetry"
)

var _ RouteHandlers = (*Handler)(nil)

type Handler struct {
	telemetry *telemetry.Provider
}

func (h Handler) HandlePrometheus() http.Handler {
	if h.telemetry == nil {
		return teapot.NoopHandler
	}
	return h.telemetry.PrometheusHTTP
}

func New(tel *telemetry.Provider) Handler {
	return Handler{telemetry: tel}
}
