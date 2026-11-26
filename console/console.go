// Package console provides the embedded DirIO web admin console.
// It imports only consoleapi/ and the standard library — never internal/.
// Handlers render HTML server-side via templ components by calling consoleapi.API directly.
// This makes it straightforward to extract into its own module later if needed.
//
// UI tooling:
//   - go generate runs templ to recompile .templ → _templ.go
//   - make tailwind-build regenerates console/static/style.css via the Tailwind v4 standalone CLI
//   - make vendor-htmx re-downloads htmx.min.js to console/static/

//go:generate templ generate

package console

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/mallardduck/teapot-router/pkg/teapot"

	consoleapi "github.com/DirIO-S3/dirio/api"
	consoleauth "github.com/DirIO-S3/dirio/console/auth"
	"github.com/DirIO-S3/dirio/console/handlers"
	"github.com/DirIO-S3/dirio/console/middleware"
	"github.com/DirIO-S3/dirio/console/ui"
)

//go:embed static
var staticFiles embed.FS

// New returns an http.Handler that serves the admin console.
// s3Router is used purely for URL generation via named routes; it is not invoked.
// adminAuth validates admin credentials at login time.
// basePath is the URL prefix the console is reachable under: "/dirio/ui" in
// single-port mode, "" in dedicated-port mode (console owns its own listener root).
// trustedProxies is a comma-separated list of IPs/CIDRs (e.g.
// "10.0.0.0/8,192.168.1.5") identifying reverse proxies/ingress controllers
// allowed to report request metadata via Forwarded-family headers; pass ""
// if the console is not deployed behind a trusted TLS-terminating proxy.
func New(api consoleapi.API, s3Router ui.S3Router, adminAuth consoleauth.AdminAuth, version, basePath, trustedProxies string) *teapot.Router {
	ui.AppVersion = version
	ui.BasePath = basePath
	trustedProxyList, err := consoleauth.ParseTrustedProxies(trustedProxies)
	if err != nil {
		panic("console: invalid trusted proxies: " + err.Error())
	}
	sessions, err := consoleauth.NewSession(basePath, trustedProxyList)
	if err != nil {
		panic("console: failed to create session manager: " + err.Error())
	}

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("console: failed to create static sub-filesystem: " + err.Error())
	}

	h := handlers.New(api, s3Router, adminAuth, sessions)

	// UI Console-specific router
	consoleTeapot := teapot.New()

	// TODO: make this conditional and only included when on dedicated port
	consoleTeapot.GET("/.internal/routes", teapot.NewListRoutesHandler(consoleTeapot, nil)).Name("routes")

	// Public routes — accessible without a session (login page + static assets).
	consoleTeapot.GET("/login", http.HandlerFunc(h.LoginPage)).Name("login")
	consoleTeapot.POST("/login", http.HandlerFunc(h.LoginSubmit)).Name("login")
	consoleTeapot.POST("/logout", http.HandlerFunc(h.Logout)).Name("logout")
	consoleTeapot.GET(
		"/static/{AssetUrl:.*}",
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			prefix := strings.SplitAfter(r.URL.Path, "/static/")[0]
			http.StripPrefix(prefix, http.FileServer(http.FS(staticFS))).ServeHTTP(rw, r)
		}),
	).Name("asset")

	// Protected routes — all require a valid session.
	consoleTeapot.MiddlewareGroup(func(r *teapot.Router) {
		r.GET("/", http.HandlerFunc(h.Dashboard)).Name("dashboard")
		r.GET("/users", http.HandlerFunc(h.Users)).Name("users")
		r.POST("/users", http.HandlerFunc(h.UserCreate)).Name("users.create")
		r.POST("/users/{uuid}/delete", http.HandlerFunc(h.UserDelete)).Name("users.delete")
		r.POST("/users/{uuid}/status", http.HandlerFunc(h.UserSetStatus)).Name("users.status")
		r.POST("/users/{uuid}/secret", http.HandlerFunc(h.UserUpdateSecret)).Name("users.secret.update")
		r.POST("/users/{uuid}/secret/rotate", http.HandlerFunc(h.UserRotateSecret)).Name("users.secret.rotate")
		r.GET("/users/{uuid}/secret", http.HandlerFunc(h.UserRevealSecret)).Name("users.secret.reveal")
		r.GET("/groups", http.HandlerFunc(h.Groups)).Name("groups")
		r.POST("/groups", http.HandlerFunc(h.GroupCreate)).Name("groups.create")
		r.GET("/groups/{group}", http.HandlerFunc(h.GroupDetail)).Name("groups.detail")
		r.POST("/groups/{group}/delete", http.HandlerFunc(h.GroupDelete)).Name("groups.delete")
		r.POST("/groups/{group}/members", http.HandlerFunc(h.GroupAddMember)).Name("groups.members.add")
		r.POST("/groups/{group}/members/remove", http.HandlerFunc(h.GroupRemoveMember)).Name("groups.members.remove")
		r.POST("/groups/{group}/policies", http.HandlerFunc(h.GroupAttachPolicy)).Name("groups.policies.attach")
		r.POST("/groups/{group}/policies/detach", http.HandlerFunc(h.GroupDetachPolicy)).Name("groups.policies.detach")
		r.POST("/groups/{group}/status", http.HandlerFunc(h.GroupSetStatus)).Name("groups.status")
		r.GET("/service-accounts", http.HandlerFunc(h.ServiceAccounts)).Name("service-accounts")
		r.POST("/service-accounts", http.HandlerFunc(h.ServiceAccountCreate)).Name("service-accounts.create")
		r.POST("/service-accounts/{uuid}/delete", http.HandlerFunc(h.ServiceAccountDelete)).Name("service-accounts.delete")
		r.POST("/service-accounts/{uuid}/status", http.HandlerFunc(h.ServiceAccountSetStatus)).Name("service-accounts.status")
		r.POST("/service-accounts/{uuid}/secret", http.HandlerFunc(h.ServiceAccountUpdateSecret)).Name("service-accounts.secret.update")
		r.GET("/service-accounts/{uuid}/secret", http.HandlerFunc(h.ServiceAccountRevealSecret)).Name("service-accounts.secret.reveal")
		r.GET("/policies", http.HandlerFunc(h.Policies)).Name("policies")
		r.GET("/buckets", http.HandlerFunc(h.Buckets)).Name("buckets")
		r.POST("/buckets", http.HandlerFunc(h.BucketCreate)).Name("buckets.create")
		r.POST("/buckets/{bucket}/delete", http.HandlerFunc(h.BucketDelete)).Name("buckets.delete")
		r.GET("/buckets/{bucket}", http.HandlerFunc(h.BucketDetail)).Name("buckets.detail")
		r.POST("/buckets/{bucket}/policy", http.HandlerFunc(h.BucketPolicySet)).Name("buckets.policy.set")
		r.POST("/buckets/{bucket}/ownership", http.HandlerFunc(h.BucketTransferOwnership)).Name("buckets.ownership.transfer")
		r.GET("/simulate", http.HandlerFunc(h.Simulate)).Name("simulate")
		r.POST("/simulate", http.HandlerFunc(h.Simulate)).Name("simulate")
		r.GET("/toasts", http.HandlerFunc(h.Toasts)).Name("toasts")
	}, middleware.RequireAdminSession(sessions))

	return consoleTeapot
}
