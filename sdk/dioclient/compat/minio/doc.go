// Package minio is the madmin-go quarantine zone within the dioclient SDK.
//
// This is the ONLY package in sdk/ that may import github.com/minio/madmin-go.
// All madmin-go type usage will be migrated here from admin.go as DirIO-native
// types are introduced in sdk/dioclient/types.go to replace the madmin surface.
//
// Current status: placeholder — admin.go still imports madmin-go directly while
// the native type extraction is pending.
package minio
