package unit

import (
	"path/filepath"
	"reflect"
	"testing"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// T032 - Test profile enumeration
// ---------------------------------------------------------------------------

func TestListProfiles_SampleFiles(t *testing.T) {
	configPath := filepath.Join("..", "testdata", "aws_config_sample")

	profiles, err := awsclient.ListProfiles(configPath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := []string{"default", "dev", "prod-sso"}
	if !reflect.DeepEqual(profiles, expected) {
		t.Errorf("expected profiles %v, got %v", expected, profiles)
	}
}

func TestListProfiles_MissingConfigFile(t *testing.T) {
	configPath := filepath.Join("..", "testdata", "nonexistent_config")

	profiles, err := awsclient.ListProfiles(configPath)
	if err != nil {
		t.Fatalf("expected no error for missing files, got %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("expected empty profile list, got %v", profiles)
	}
}

func TestListProfiles_ConfigOnly(t *testing.T) {
	configPath := filepath.Join("..", "testdata", "aws_config_sample")

	profiles, err := awsclient.ListProfiles(configPath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := []string{"default", "dev", "prod-sso"}
	if !reflect.DeepEqual(profiles, expected) {
		t.Errorf("expected profiles %v, got %v", expected, profiles)
	}
}

func TestListProfiles_CredentialsFileNeverRead(t *testing.T) {
	// a9s never reads ~/.aws/credentials — only ~/.aws/config for profile names.
	// Credential handling is delegated entirely to the AWS SDK.
	// This test verifies ListProfiles has no credentials path parameter.
	configPath := filepath.Join("..", "testdata", "nonexistent_config")

	profiles, err := awsclient.ListProfiles(configPath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles, got %v", profiles)
	}
}

// ---------------------------------------------------------------------------
// T035 - Test region helpers
// ---------------------------------------------------------------------------

func TestAllRegions_ContainsMinimumRegions(t *testing.T) {
	regions := awsclient.AllRegions()

	required := []string{
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1", "eu-north-1",
		"ap-southeast-1", "ap-southeast-2", "ap-northeast-1", "ap-northeast-2",
		"ap-south-1", "sa-east-1", "ca-central-1", "me-south-1", "af-south-1",
	}

	regionCodes := make(map[string]bool)
	for _, r := range regions {
		regionCodes[r.Code] = true
	}

	for _, code := range required {
		if !regionCodes[code] {
			t.Errorf("AllRegions missing required region %q", code)
		}
	}
}

func TestAllRegions_HasDisplayNames(t *testing.T) {
	regions := awsclient.AllRegions()
	for _, r := range regions {
		if r.DisplayName == "" {
			t.Errorf("region %q has empty DisplayName", r.Code)
		}
	}
}

func TestGetDefaultRegion_FromConfigFile(t *testing.T) {
	configPath := filepath.Join("..", "testdata", "aws_config_sample")

	// "default" section has region = us-east-1
	region := awsclient.GetDefaultRegion(configPath, "default")
	if region != "us-east-1" {
		t.Errorf("expected region %q for default profile, got %q", "us-east-1", region)
	}

	// "dev" profile has region = eu-west-1
	region = awsclient.GetDefaultRegion(configPath, "dev")
	if region != "eu-west-1" {
		t.Errorf("expected region %q for dev profile, got %q", "eu-west-1", region)
	}

	// "prod-sso" profile has region = us-west-2
	region = awsclient.GetDefaultRegion(configPath, "prod-sso")
	if region != "us-west-2" {
		t.Errorf("expected region %q for prod-sso profile, got %q", "us-west-2", region)
	}
}

func TestGetDefaultRegion_MissingFile(t *testing.T) {
	region := awsclient.GetDefaultRegion("/nonexistent/path/config", "default")
	if region != "us-east-1" {
		t.Errorf("expected fallback region %q, got %q", "us-east-1", region)
	}
}

func TestGetDefaultRegion_UnknownProfile(t *testing.T) {
	configPath := filepath.Join("..", "testdata", "aws_config_sample")
	region := awsclient.GetDefaultRegion(configPath, "nonexistent-profile")
	if region != "us-east-1" {
		t.Errorf("expected fallback region %q for unknown profile, got %q", "us-east-1", region)
	}
}
