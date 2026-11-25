package s3types

import (
	"encoding/xml"
	"time"
)

// Object Lock retention modes, as used in the S3 API.
const (
	RetentionModeGovernance = "GOVERNANCE"
	RetentionModeCompliance = "COMPLIANCE"
)

// Legal hold status values, as used in the S3 API.
const (
	LegalHoldStatusOn  = "ON"
	LegalHoldStatusOff = "OFF"
)

// ObjectLockEnabledValue is the only value S3 accepts for ObjectLockEnabled.
const ObjectLockEnabledValue = "Enabled"

// ObjectLockConfiguration is the bucket-level Object Lock configuration
// (PUT/GET /{bucket}?object-lock).
type ObjectLockConfiguration struct {
	XMLName           xml.Name        `xml:"ObjectLockConfiguration"`
	Xmlns             string          `xml:"xmlns,attr,omitempty"`
	ObjectLockEnabled string          `xml:"ObjectLockEnabled,omitempty"`
	Rule              *ObjectLockRule `xml:"Rule,omitempty"`
}

// ObjectLockRule wraps the default retention applied to new objects.
type ObjectLockRule struct {
	DefaultRetention *DefaultRetention `xml:"DefaultRetention,omitempty"`
}

// DefaultRetention is the bucket default retention. Exactly one of Days or
// Years must be set.
type DefaultRetention struct {
	Mode  string `xml:"Mode"`
	Days  *int   `xml:"Days,omitempty"`
	Years *int   `xml:"Years,omitempty"`
}

// ObjectRetentionConfiguration is the per-object retention
// (PUT/GET /{bucket}/{key}?retention).
type ObjectRetentionConfiguration struct {
	XMLName         xml.Name  `xml:"Retention"`
	Xmlns           string    `xml:"xmlns,attr,omitempty"`
	Mode            string    `xml:"Mode"`
	RetainUntilDate time.Time `xml:"RetainUntilDate"`
}

// ObjectLegalHoldConfiguration is the per-object legal hold
// (PUT/GET /{bucket}/{key}?legal-hold).
type ObjectLegalHoldConfiguration struct {
	XMLName xml.Name `xml:"LegalHold"`
	Xmlns   string   `xml:"xmlns,attr,omitempty"`
	Status  string   `xml:"Status"`
}
