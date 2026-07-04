package server

import (
	"io"
	"net/http"

	"github.com/mallardduck/teapot-router/pkg/teapot"

	minioHTTP "github.com/mallardduck/dirio/internal/compat/minio/http"

	dirioapi "github.com/mallardduck/dirio/internal/http/api/dirio"
	"github.com/mallardduck/dirio/internal/http/server/prof"

	"github.com/mallardduck/dirio/internal/http/api"
	"github.com/mallardduck/dirio/internal/http/auth"
	"github.com/mallardduck/dirio/internal/http/middleware"
	httpresponse "github.com/mallardduck/dirio/internal/http/response"
	"github.com/mallardduck/dirio/internal/http/server/favicon"
	"github.com/mallardduck/dirio/internal/http/server/health"
	"github.com/mallardduck/dirio/internal/http/server/metrics"
	"github.com/mallardduck/dirio/internal/http/vhost"

	"github.com/mallardduck/dirio/internal/consts"
	"github.com/mallardduck/dirio/internal/persistence/metadata"
	"github.com/mallardduck/dirio/internal/policy"
)

// RouteDependencies contains all dependencies needed for route handlers.
type RouteDependencies struct {
	// Original Deps
	auth         *auth.Authenticator
	policyEngine *policy.Engine
	metadata     *metadata.Manager      // For ownership-based authorization
	adminKeys    policy.AdminKeyChecker // Live admin key source (auth.Authenticator)
	APIHandler   *api.Handler

	// Modern Deps
	Health   health.RouteHandlers
	Metrics  metrics.RouteHandlers
	Minio    minioHTTP.RouteHandlers
	Pprof    prof.RouteHandlers
	DirioAPI dirioapi.RouteHandlers
}

// SetupRoutes configures all application routes on the provided router.
// When deps is nil, routes are registered with nil handlers (for CLI route listing).
func SetupRoutes(r *teapot.Router, deps RouteDependencies) {
	// Favicon must be at site the root for full compatibility
	r.Func().GET("/favicon.ico", favicon.HandleFavicon).Name("favicon")
	// /.dirio/* — DirIO-specific routes.
	// Dot-prefix guarantees no collision with S3 bucket names (bucket names must
	// start with a letter or digit per the S3 spec and AWS validation rules).
	r.GET(
		"/.dirio/routes",
		teapot.NewListRoutesHandler(r, &teapot.ListRoutesOptions{
			BaseURLFunc: func(r *http.Request) string {
				scheme := "https"
				if r.TLS == nil {
					scheme = "http"
				}
				return scheme + "://" + r.Host
			},
		}),
	).Name("debug.routes").Action("dirio:ListRoutes")

	// DirIO health endpoints (unauthenticated).
	// These are under /.dirio/ so they never collide with user bucket names.
	health.RegisterRoutes(r, deps.Health)

	// DirIO metrics endpoint — serves Prometheus-format OTel metrics (unauthenticated).
	metrics.RegisterRoutes(r, deps.Metrics)

	// pprof profiling endpoints — only registered when --debug is set.
	// Unauthenticated: debug mode is not intended for production use.
	prof.RegisterRoutes(r, deps.Pprof)

	// DirIO REST API — requires SigV4 authentication; no S3 policy authz or chunked encoding.
	var dirioAPIMW []func(http.Handler) http.Handler
	if deps.auth != nil {
		dirioAPIMW = []func(http.Handler) http.Handler{deps.auth.AuthMiddleware}
	}
	r.MiddlewareGroup(func(r *teapot.Router) {
		dirioapi.RegisterRoutes(r, deps.DirioAPI)
	}, dirioAPIMW...)

	// S3 API routes (authenticated + chunked encoding), path-style addressing.
	s3Deps := buildS3RouteDeps(pathStyle, deps)
	s3MW := buildS3Middleware(deps)
	r.MiddlewareGroup(func(r *teapot.Router) {
		// MinIO Admin API routes (authenticated)
		minioHTTP.RegisterRouter(r, deps.Minio)

		// Setup the S3 API routes
		setupS3Routes(r, pathStyle, s3Deps)
	}, s3MW...)

	r.Finalize()
}

// SetupVHostRoutes configures the S3 data-plane routes for virtual-hosted-style
// addressing on a dedicated router. Only called when CanonicalDomain is set.
//
// Unlike SetupRoutes, this does not register /.dirio/*, /minio/admin/*, or any
// other non-bucket-scoped routes — AWS's vhost-style endpoints are
// S3-data-plane-only, and control-plane traffic always goes through the path
// router (bare canonical domain or an IP).
func SetupVHostRoutes(r *teapot.Router, deps RouteDependencies, canonicalDomain string) {
	style := vhostStyle(canonicalDomain)
	s3Deps := buildS3RouteDeps(style, deps)
	s3MW := buildS3Middleware(deps)
	r.MiddlewareGroup(func(r *teapot.Router) {
		setupS3Routes(r, style, s3Deps)
	}, s3MW...)

	r.Finalize()
}

// buildS3Middleware returns the S3 API middleware chain (auth, policy authz,
// chunked encoding). Identical regardless of addressing style. Returns nil
// when deps.auth is unset (CLI route-listing stub case).
func buildS3Middleware(deps RouteDependencies) []func(http.Handler) http.Handler {
	if deps.auth == nil {
		return nil
	}

	authzConfig := &policy.AuthorizationConfig{
		Engine:    deps.policyEngine,
		Metadata:  deps.metadata,
		AdminKeys: deps.adminKeys,
	}

	return []func(http.Handler) http.Handler{
		deps.auth.AuthMiddleware,
		policy.AuthorizationMiddleware(authzConfig),
		middleware.ChunkedEncoding(func(r io.Reader) io.Reader {
			return auth.NewChunkedReader(r)
		}),
	}
}

// buildS3RouteDeps wraps S3 handlers for the given routeStyle. Returns nil
// when no S3 handler is configured (CLI route-listing stub case).
func buildS3RouteDeps(style routeStyle, deps RouteDependencies) *s3RouteDeps {
	if deps.APIHandler == nil || deps.APIHandler.S3Handler == nil {
		return nil
	}

	h := deps.APIHandler.S3Handler
	return &s3RouteDeps{
		listBuckets:             h.ListBuckets,
		headBucket:              bucket(style, h.HeadBucket),
		createBucket:            bucket(style, h.CreateBucket),
		listObjects:             bucket(style, h.ListObjects),
		deleteBucket:            bucket(style, h.DeleteBucket),
		postObject:              bucket(style, h.PostObject),
		listObjectsV2:           bucket(style, h.ListObjectsV2),
		getBucketLocation:       bucket(style, h.GetBucketLocation),
		getBucketPolicy:         bucket(style, h.GetBucketPolicy),
		putBucketPolicy:         bucket(style, h.PutBucketPolicy),
		delBucketPolicy:         bucket(style, h.DeleteBucketPolicy),
		getBucketVersioning:     httpresponse.NotImplemented,
		putBucketVersioning:     httpresponse.NotImplemented,
		getBucketACL:            httpresponse.NotImplemented,
		putBucketACL:            httpresponse.NotImplemented,
		getBucketCors:           httpresponse.NotImplemented,
		putBucketCors:           httpresponse.NotImplemented,
		listObjectVersions:      httpresponse.NotImplemented,
		listMultipartUploads:    httpresponse.NotImplemented,
		deleteObjects:           bucket(style, h.DeleteObjects),
		headObject:              object(style, h.HeadObject),
		putObject:               object(style, h.PutObject),
		copyObject:              object(style, h.CopyObject),
		getObject:               object(style, h.GetObject),
		deleteObject:            object(style, h.DeleteObject),
		getObjectACL:            httpresponse.NotImplemented,
		putObjectACL:            httpresponse.NotImplemented,
		getObjectTagging:        object(style, h.GetObjectTagging),
		putObjectTagging:        object(style, h.PutObjectTagging),
		multipartCreate:         object(style, h.CreateMultipartUpload),
		multipartUploadPart:     object(style, h.UploadPart),
		multipartUploadPartCopy: object(style, h.UploadPartCopy),
		multipartComplete:       object(style, h.CompleteMultipartUpload),
		multipartAbort:          object(style, h.AbortMultipartUpload),
		multipartListParts:      object(style, h.ListParts),
	}
}

// routeStyle parameterizes S3 route registration by addressing style
// (path-style vs virtual-hosted-style). The two differ only in (a) whether
// the path pattern has a {bucket} segment, and (b) how the bucket value is
// read out of the request — handler signatures and business logic never see
// which style resolved their bucket/key strings.
type routeStyle struct {
	bucketSegment string // "/{bucket}" for path-style, "" for vhost-style
	resolveBucket func(r *http.Request) string
}

// pathStyle resolves the bucket from the "/{bucket}" URL segment.
var pathStyle = routeStyle{
	bucketSegment: "/{bucket}",
	resolveBucket: func(r *http.Request) string {
		return teapot.URLParam(r, "bucket")
	},
}

// vhostStyle resolves the bucket from the leftmost label of the Host header,
// given the configured canonical domain.
func vhostStyle(canonicalDomain string) routeStyle {
	return routeStyle{
		bucketSegment: "",
		resolveBucket: func(r *http.Request) string {
			b, _ := vhost.BucketFromHost(r.Host, canonicalDomain)
			return b
		},
	}
}

// bucket wraps an S3 bucket-level handler, resolving the bucket per style.
// It also applies S3 bucket name validation middleware against that same
// resolved value.
func bucket(style routeStyle, fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	// Create the base handler that extracts parameters
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, style.resolveBucket(r))
	})

	// Apply validation middleware
	validated := middleware.ValidateS3BucketNameMiddleware(
		style.resolveBucket,
		api.WriteErrorResponse,
	)(baseHandler)

	return validated.ServeHTTP
}

// object wraps an S3 object-level handler, resolving the bucket per style
// and the key from the {key} URL segment (present under both styles). It
// also applies S3 bucket name and key validation middleware.
func object(style routeStyle, fn func(http.ResponseWriter, *http.Request, string, string)) http.HandlerFunc {
	// Create the base handler that extracts parameters
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, style.resolveBucket(r), teapot.URLParam(r, "key"))
	})

	// Apply bucket name validation middleware first
	validated := middleware.ValidateS3BucketNameMiddleware(
		style.resolveBucket,
		api.WriteErrorResponse,
	)(baseHandler)

	// Then apply key validation middleware
	validated = middleware.ValidateS3KeyMiddleware(
		func(r *http.Request) string { return teapot.URLParam(r, "key") },
		api.WriteErrorResponse,
	)(validated)

	return validated.ServeHTTP
}

type s3RouteDeps struct {
	// Service
	listBuckets http.HandlerFunc
	// Bucket — direct routes (become fallbacks when query routes are added)
	headBucket   http.HandlerFunc
	createBucket http.HandlerFunc
	listObjects  http.HandlerFunc
	deleteBucket http.HandlerFunc
	postObject   http.HandlerFunc
	// Bucket — query-dispatched operations
	listObjectsV2        http.HandlerFunc
	getBucketLocation    http.HandlerFunc
	getBucketPolicy      http.HandlerFunc
	putBucketPolicy      http.HandlerFunc
	delBucketPolicy      http.HandlerFunc
	getBucketVersioning  http.HandlerFunc
	putBucketVersioning  http.HandlerFunc
	getBucketACL         http.HandlerFunc
	putBucketACL         http.HandlerFunc
	getBucketCors        http.HandlerFunc
	putBucketCors        http.HandlerFunc
	listObjectVersions   http.HandlerFunc
	listMultipartUploads http.HandlerFunc
	deleteObjects        http.HandlerFunc
	// Object — direct routes (use {key:.*} to capture entire path including slashes)
	headObject   http.HandlerFunc
	putObject    http.HandlerFunc
	copyObject   http.HandlerFunc
	getObject    http.HandlerFunc
	deleteObject http.HandlerFunc
	// Object — query-dispatched operations
	getObjectACL     http.HandlerFunc
	putObjectACL     http.HandlerFunc
	getObjectTagging http.HandlerFunc
	putObjectTagging http.HandlerFunc
	// Multipart upload operations
	multipartCreate         http.HandlerFunc
	multipartUploadPart     http.HandlerFunc
	multipartUploadPartCopy http.HandlerFunc
	multipartComplete       http.HandlerFunc
	multipartAbort          http.HandlerFunc
	multipartListParts      http.HandlerFunc
}

// setupS3Routes registers S3 API routes for the given addressing style.
// Direct routes are registered first — they become fallbacks when
// query-dispatched routes are added to the same method+pattern via the
// router's auto-promotion logic.
//
// Under path-style, style.bucketSegment is "/{bucket}" so bucketPattern is
// "/{bucket}" and objectPattern is "/{bucket}/{key:.*}", same as before this
// was parameterized. Under vhost-style, style.bucketSegment is "" — the
// bucket comes from Host, not the path — so bucketPattern collapses to "/"
// and objectPattern to "/{key:.*}". The service-level ListBuckets route
// (bare "/") has no vhost equivalent — a vhost request always resolves to a
// specific bucket — so it is only registered for path-style.
func setupS3Routes(r *teapot.Router, style routeStyle, deps *s3RouteDeps) {
	if deps == nil {
		deps = &s3RouteDeps{}
	}

	bucketPattern := style.bucketSegment
	if bucketPattern == "" {
		bucketPattern = "/"
	} else {
		// Service — only meaningful when the path can distinguish "no bucket"
		// from "bucket at root", i.e. path-style only.
		r.GET("/", deps.listBuckets).Name("index").Action("s3:ListBuckets")
	}
	objectPattern := style.bucketSegment + "/{key:.*}"

	// Bucket — direct routes (become fallbacks when query routes are added)
	r.HEAD(bucketPattern, deps.headBucket).Name("buckets.head").Action("s3:HeadBucket")
	r.PUT(bucketPattern, deps.createBucket).Name("buckets.store").Action("s3:CreateBucket")
	r.GET(bucketPattern, deps.listObjects).Name("buckets.show").Action("s3:ListObjects")
	r.DELETE(bucketPattern, deps.deleteBucket).Name("buckets.destroy").Action("s3:DeleteBucket")

	// POST Policy Uploads (browser-based form upload via multipart/form-data)
	// Credentials are embedded in the form body — auth middleware handles authentication,
	// authz middleware skips (no Action), and the handler validates policy conditions.
	// Spec: https://docs.aws.amazon.com/AmazonS3/latest/API/RESTObjectPOST.html
	r.POST(bucketPattern, deps.postObject).Name("buckets.post-policy-upload")

	// Query-based bucket operations
	// ListObjectsV2 (preferred over v1)
	r.QueryGET(bucketPattern, deps.listObjectsV2).QueryValue("list-type", "2").Name("buckets.listv2").Action("s3:ListObjectsV2")

	// Bucket configuration endpoints
	r.QueryGET(bucketPattern, deps.getBucketLocation).Query("location").Name("buckets.location").Action("s3:GetBucketLocation")
	r.QueryGET(bucketPattern, deps.getBucketVersioning).Query("versioning").Name("buckets.versioning.show").Action("s3:GetBucketVersioning")
	r.QueryPUT(bucketPattern, deps.putBucketVersioning).Query("versioning").Name("buckets.versioning.store").Action("s3:PutBucketVersioning")
	r.QueryGET(bucketPattern, deps.getBucketACL).Query("acl").Name("buckets.acl.show").Action("s3:GetBucketAcl")
	r.QueryPUT(bucketPattern, deps.putBucketACL).Query("acl").Name("buckets.acl.store").Action("s3:PutBucketAcl")

	// Bucket policy endpoints
	r.QueryGET(bucketPattern, deps.getBucketPolicy).Query("policy").Name("buckets.policy.show").Action("s3:GetBucketPolicy")
	r.QueryPUT(bucketPattern, deps.putBucketPolicy).Query("policy").Name("buckets.policy.store").Action("s3:PutBucketPolicy")
	r.QueryDELETE(bucketPattern, deps.delBucketPolicy).Query("policy").Name("buckets.policy.destroy").Action("s3:DeleteBucketPolicy")

	// Bucket CORS endpoints
	r.QueryGET(bucketPattern, deps.getBucketCors).Query("cors").Name("buckets.cors.show").Action("s3:GetBucketCors")
	r.QueryPUT(bucketPattern, deps.putBucketCors).Query("cors").Name("buckets.cors.store").Action("s3:PutBucketCors")

	// Bucket lifecycle configuration
	// Note: Legacy GetBucketLifecycle/PutBucketLifecycle share the same path and query param
	//       as the modern *Configuration variants; one route per method covers both.
	r.Func().QueryGET(bucketPattern, httpresponse.NotImplemented).Query("lifecycle").Name("bucket.get-lifecycle-configuration").Action("s3:GetBucketLifecycleConfiguration")
	r.Func().QueryPUT(bucketPattern, httpresponse.NotImplemented).Query("lifecycle").Name("bucket.put-lifecycle-configuration").Action("s3:PutBucketLifecycleConfiguration")

	// Public access block
	r.Func().QueryGET(bucketPattern, httpresponse.NotImplemented).Query("publicAccessBlock").Name("bucket.get-public-access-block").Action("s3:GetPublicAccessBlock")
	r.Func().QueryPUT(bucketPattern, httpresponse.NotImplemented).Query("publicAccessBlock").Name("bucket.put-public-access-block").Action("s3:PutPublicAccessBlock")

	// Object lock configuration
	r.Func().QueryPUT(bucketPattern, httpresponse.NotImplemented).Query("object-lock").Name("bucket.put-object-lock-configuration").Action("s3:PutObjectLockConfiguration")

	// List object versions (for versioned buckets)
	r.QueryGET(bucketPattern, deps.listObjectVersions).Query("versions").Name("buckets.versions").Action("s3:ListObjectVersions")

	// List multipart uploads in bucket
	r.QueryGET(bucketPattern, deps.listMultipartUploads).Query("uploads").Name("buckets.uploads").Action("s3:ListMultipartUploads")

	// Bulk delete objects
	r.QueryPOST(bucketPattern, deps.deleteObjects).Query("delete").Name("buckets.delete-objects").Action("s3:DeleteObjects")

	// ==================== OBJECT OPERATIONS ====================
	r.GET(objectPattern, deps.getObject).Name("objects.show").Action("s3:GetObject")
	r.DELETE(objectPattern, deps.deleteObject).Name("objects.destroy").Action("s3:DeleteObject")
	r.HEAD(objectPattern, deps.headObject).Name("objects.head").Action("s3:HeadObject")

	// PUT {bucketPattern}/{key} dispatches on X-Amz-Copy-Source header.
	// UploadPart / UploadPartCopy also live here: same method+path, and header
	// presence distinguishes the copy variant.  The remaining QueryPUT routes
	// below (acl, tagging, …) are added to this same dispatcher automatically.
	// PUT dispatcher
	// TODO(Phase 3.2 #4): Implement CopyObject handler (currently httpresponse.NotImplemented)
	//   - Parse X-Amz-Copy-Source header (bucket/key format)
	//   - Policy engine already supports dual permission checks (source read + dest write)
	//   - Copy object metadata, content-type, and custom metadata
	//   - Handle copy-if-* conditional headers (If-Match, If-None-Match, If-Modified-Since, If-Unmodified-Since)
	//   - Test: aws s3 cp s3://bucket/src.txt s3://bucket/dest.txt
	//   - See policy/middleware.go:169 for multi-resource action handling
	r.Dispatch("PUT", objectPattern, func(d *teapot.DispatchBuilder, m teapot.Matchers) {
		// Query-based operations must come before default
		d.When(m.QueryExists("partNumber"), m.QueryExists("uploadId"), m.HeaderExists(consts.HeaderCopySource)).Do(deps.multipartUploadPartCopy).Name("multipart.upload-part-copy").Action("s3:UploadPartCopy")
		d.When(m.QueryExists("partNumber"), m.QueryExists("uploadId")).Do(deps.multipartUploadPart).Name("multipart.upload-part").Action("s3:UploadPart")
		d.When(m.QueryExists("acl")).Do(deps.putObjectACL).Name("objects.acl.store").Action("s3:PutObjectAcl")
		d.When(m.QueryExists("tagging")).Do(deps.putObjectTagging).Name("objects.tagging.store").Action("s3:PutObjectTagging")

		// Header-based copy operation
		d.When(m.HeaderExists(consts.HeaderCopySource)).Do(deps.copyObject).Name("object.copy").Action("s3:CopyObject")

		// Default: regular PUT object
		d.Default(deps.putObject).Name("object.put").Action("s3:PutObject")
	})

	// Query-based object operations
	r.QueryGET(objectPattern, deps.getObjectACL).Query("acl").Name("objects.acl.show").Action("s3:GetObjectAcl")

	// Object tagging
	r.QueryGET(objectPattern, deps.getObjectTagging).Query("tagging").Name("objects.tagging.show").Action("s3:GetObjectTagging")

	// Multipart upload operations
	r.QueryPOST(objectPattern, deps.multipartCreate).Query("uploads").Name("multipart.create").Action("s3:CreateMultipartUpload")
	r.QueryPOST(objectPattern, deps.multipartComplete).Query("uploadId").Name("multipart.complete").Action("s3:CompleteMultipartUpload")
	r.QueryDELETE(objectPattern, deps.multipartAbort).Query("uploadId").Name("multipart.abort").Action("s3:AbortMultipartUpload")
	r.QueryGET(objectPattern, deps.multipartListParts).Query("uploadId").Name("multipart.list-parts").Action("s3:ListParts")
}
