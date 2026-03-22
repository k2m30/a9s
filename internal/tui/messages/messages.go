package messages

import (
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ViewTarget identifies a destination view for NavigateMsg.
type ViewTarget int

const (
	TargetMainMenu ViewTarget = iota
	TargetResourceList
	TargetDetail
	TargetYAML
	TargetReveal
	TargetProfile
	TargetRegion
	TargetHelp
)

// NavigateMsg requests a view transition. The root model handles push/pop.
type NavigateMsg struct {
	Target       ViewTarget
	ResourceType string
	Resource     *resource.Resource
}

// PopViewMsg requests popping the current view from the stack.
type PopViewMsg struct{}

// ResourcesLoadedMsg is sent when AWS resources have been fetched.
type ResourcesLoadedMsg struct {
	ResourceType string
	Resources    []resource.Resource
}

// APIErrorMsg is sent when an AWS API call fails.
type APIErrorMsg struct {
	ResourceType string
	Err          error
}

// FlashMsg sets a transient message in the header right side.
type FlashMsg struct {
	Text    string
	IsError bool
}

// ClearFlashMsg is sent after the flash auto-clear timer expires.
type ClearFlashMsg struct {
	Gen int // only clear if this matches current flash generation
}

// ProfileSelectedMsg is sent when the user confirms a profile selection.
type ProfileSelectedMsg struct {
	Profile string
}

// RegionSelectedMsg is sent when the user confirms a region selection.
type RegionSelectedMsg struct {
	Region string
}

// SecretRevealedMsg is sent when a secret value has been fetched.
type SecretRevealedMsg struct {
	SecretName string
	Value      string
	Err        error
}

// CopiedMsg is sent after a successful clipboard copy.
type CopiedMsg struct {
	Content string
}

// InitConnectMsg triggers the initial AWS session setup.
type InitConnectMsg struct {
	Profile string
	Region  string
}

// ClientsReadyMsg is sent when AWS clients are initialized.
// Clients is typed as interface{} to avoid importing aws/ from the messages package.
// The root model type-asserts it to *awsclient.ServiceClients.
type ClientsReadyMsg struct {
	Clients interface{}
	Err     error
}

// LoadResourcesMsg triggers an async fetch of resources for a given type.
type LoadResourcesMsg struct {
	ResourceType string
	S3Bucket     string
	S3Prefix     string
}

// RevealSecretMsg triggers an async fetch of a secret's value.
type RevealSecretMsg struct {
	SecretName string
}

// RefreshMsg triggers a re-fetch of the current resource list.
type RefreshMsg struct{}

// S3EnterBucketMsg signals that the user selected an S3 bucket to drill into.
type S3EnterBucketMsg struct {
	BucketName string
}

// S3NavigatePrefixMsg signals navigation to a new prefix within an S3 bucket.
type S3NavigatePrefixMsg struct {
	Bucket string
	Prefix string
}

// R53EnterZoneMsg signals that the user selected a Route53 hosted zone to drill into.
type R53EnterZoneMsg struct {
	ZoneId   string
	ZoneName string
}
