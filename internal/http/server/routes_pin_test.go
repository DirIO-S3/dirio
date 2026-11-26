package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mallardduck/teapot-router/pkg/teapot"

	dirioapi "github.com/DirIO-S3/dirio/internal/http/api/dirio"
	"github.com/DirIO-S3/dirio/internal/http/server/stubs"
)

// routeTableGolden pins the full route table (method, pattern, name, action,
// query and header matchers) so router upgrades and refactors cannot silently
// change what is served or how it is named.
const routeTableGolden = "testdata/route_table.golden"

func renderRouteTable(routes []teapot.RouteInfo) string {
	var b strings.Builder
	for _, rt := range routes {
		var query, header []string
		for _, q := range rt.QueryParams {
			query = append(query, q.Key+"="+q.Value)
		}
		for _, h := range rt.HeaderParams {
			header = append(header, h.Key+"="+h.Value)
		}
		fmt.Fprintf(&b, "%-7s %-60s name=%-40s action=%-40s query=[%s] header=[%s]\n",
			rt.Method, rt.Pattern, rt.Name, rt.Action,
			strings.Join(query, ","), strings.Join(header, ","))
	}
	return b.String()
}

// noopDirioAPI satisfies dirioapi.RouteHandlers with no-op handlers. The
// `routes` CLI does not wire DirioAPI, so this keeps the test self-contained.
type noopDirioAPI struct{}

var _ dirioapi.RouteHandlers = noopDirioAPI{}

func (noopDirioAPI) HandleGetBucketOwner() http.Handler          { return teapot.NoopHandler }
func (noopDirioAPI) HandleTransferBucketOwner() http.Handler     { return teapot.NoopHandler }
func (noopDirioAPI) HandleGetObjectOwner() http.Handler          { return teapot.NoopHandler }
func (noopDirioAPI) HandleSimulate() http.Handler                { return teapot.NoopHandler }
func (noopDirioAPI) HandleGetEffectivePermissions() http.Handler { return teapot.NoopHandler }

func TestRouteTableMatchesGolden(t *testing.T) {
	r := teapot.New()
	stub := stubs.StubHandler{}
	SetupRoutes(r, RouteDependencies{
		Health:   stub,
		Metrics:  stub,
		Minio:    stub,
		Pprof:    stub,
		DirioAPI: noopDirioAPI{},
	})

	got := renderRouteTable(r.Routes())
	if got == "" {
		t.Fatal("route table is empty")
	}

	path := filepath.FromSlash(routeTableGolden)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Skipf("golden file written to %s", path)
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file (run with UPDATE_GOLDEN=1 to create): %v", err)
	}
	if got != string(want) {
		t.Errorf("route table changed.\n--- want (%s)\n+++ got\n%s", path, diffLines(string(want), got))
	}
}

// diffLines returns a minimal line-by-line diff for readable failures.
func diffLines(want, got string) string {
	wl := strings.Split(want, "\n")
	gl := strings.Split(got, "\n")
	var b strings.Builder
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			fmt.Fprintf(&b, "line %d:\n- %s\n+ %s\n", i+1, w, g)
		}
	}
	return b.String()
}
