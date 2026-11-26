package dioclient

import (
	compatminio "github.com/DirIO-S3/dirio/sdk/dioclient/compat/minio"
)

// S3 types — defined in compat/minio, re-exported as type aliases so callers
// never import the compat package directly.
type (
	BucketInfo = compatminio.BucketInfo
	ObjectInfo = compatminio.ObjectInfo
)

// Admin and IAM types — defined in compat/minio, re-exported as type aliases so
// callers never need to import the compat package directly. When DirIO-native admin
// types replace madmin-go, only this file and compat/minio need to change.

type AccountStatus = compatminio.AccountStatus

const (
	AccountEnabled  = compatminio.AccountEnabled
	AccountDisabled = compatminio.AccountDisabled
)

type (
	ServiceAccountInfo      = compatminio.ServiceAccountInfo
	ServiceAccountsListResp = compatminio.ServiceAccountsListResp
	AddServiceAccountReq    = compatminio.AddServiceAccountReq
	Credentials             = compatminio.Credentials
	ServiceAccountInfoResp  = compatminio.ServiceAccountInfoResp
	UpdateServiceAccountReq = compatminio.UpdateServiceAccountReq
	UserInfo                = compatminio.UserInfo
	PolicyInfo              = compatminio.PolicyInfo
	PolicyAssociationReq    = compatminio.PolicyAssociationReq
	PolicyAssociationResp   = compatminio.PolicyAssociationResp
)
