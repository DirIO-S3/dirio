module github.com/DirIO-S3/dirio/console

go 1.26

require (
	github.com/DirIO-S3/dirio/api v0.0.0
	github.com/Oudwins/tailwind-merge-go v0.2.3
	github.com/a-h/templ v0.3.1020
	github.com/google/uuid v1.6.0
	github.com/mallardduck/teapot-router v0.15.2
)

require (
	github.com/go-chi/chi/v5 v5.3.2 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
)

replace github.com/DirIO-S3/dirio/api => ../api
