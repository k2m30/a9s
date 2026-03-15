package app

import "github.com/k2m30/a9s/internal/resource"

// ResourcesLoadedMsg is sent when resources have been successfully fetched from AWS.
type ResourcesLoadedMsg struct {
	ResourceType string
	Resources    []resource.Resource
}

// APIErrorMsg is sent when an AWS API call fails.
type APIErrorMsg struct {
	Err          error
	ResourceType string
}

// ProfileSwitchedMsg is sent when the user switches to a different AWS profile.
type ProfileSwitchedMsg struct {
	Profile string
	Region  string
}

// RegionSwitchedMsg is sent when the user switches to a different AWS region.
type RegionSwitchedMsg struct {
	Region string
}

// StatusMsg is sent to display a status message in the status bar.
type StatusMsg struct {
	Text    string
	IsError bool
}

// SecretRevealedMsg is sent when a secret value has been fetched from AWS.
type SecretRevealedMsg struct {
	SecretName string
	Value      string
	Err        error
}

// ClearErrorMsg is sent after a timeout to auto-clear error messages.
type ClearErrorMsg struct{}
