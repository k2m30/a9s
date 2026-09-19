package unit

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

func TestListProfiles_SampleFiles(t *testing.T) {
	configPath := filepath.Join("..", "testdata", "aws_profile", "config_sample")

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
	configPath := filepath.Join("..", "testdata", "aws_profile", "config_sample")

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
	configPath := filepath.Join("..", "testdata", "nonexistent_config")

	profiles, err := awsclient.ListProfiles(configPath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles, got %v", profiles)
	}
}

func TestDefaultConfigPath_EnvOverride(t *testing.T) {
	t.Setenv("AWS_CONFIG_FILE", "/custom/path/config")
	got := awsclient.DefaultConfigPath()
	if got != "/custom/path/config" {
		t.Errorf("expected /custom/path/config, got %s", got)
	}
}

func TestDefaultConfigPath_FallbackWithoutEnv(t *testing.T) {
	t.Setenv("AWS_CONFIG_FILE", "")
	got := awsclient.DefaultConfigPath()
	if got == "" {
		t.Error("expected non-empty default path")
	}
	if got == "/custom/path/config" {
		t.Error("should not return custom path when env is empty")
	}
}

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
	configPath := filepath.Join("..", "testdata", "aws_profile", "config_sample")

	region := awsclient.GetDefaultRegion(configPath, "default")
	if region != "us-east-1" {
		t.Errorf("expected region %q for default profile, got %q", "us-east-1", region)
	}

	region = awsclient.GetDefaultRegion(configPath, "dev")
	if region != "eu-west-1" {
		t.Errorf("expected region %q for dev profile, got %q", "eu-west-1", region)
	}

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
	configPath := filepath.Join("..", "testdata", "aws_profile", "config_sample")
	region := awsclient.GetDefaultRegion(configPath, "nonexistent-profile")
	if region != "us-east-1" {
		t.Errorf("expected fallback region %q for unknown profile, got %q", "us-east-1", region)
	}
}

// GetDefaultRegion resolves: AWS_REGION env > AWS_DEFAULT_REGION env >
// profile region > source_profile chain (recursive, cycle-guarded) >
// [default] region > "us-east-1".

func TestGetDefaultRegion_SourceProfileChain_SingleHop(t *testing.T) {
	configPath := filepath.Join("..", "testdata", "aws_profile", "config_source_profile_chain")

	region := awsclient.GetDefaultRegion(configPath, "role-only")
	if region != "eu-central-1" {
		t.Errorf("expected region %q resolved from source_profile chain, got %q", "eu-central-1", region)
	}
}

func TestGetDefaultRegion_SourceProfileChain_TwoHop(t *testing.T) {
	configPath := filepath.Join("..", "testdata", "aws_profile", "config_source_profile_chain")

	region := awsclient.GetDefaultRegion(configPath, "role-two-level")
	if region != "eu-central-1" {
		t.Errorf("expected region %q resolved from two-hop source_profile chain, got %q", "eu-central-1", region)
	}
}

func TestGetDefaultRegion_SourceProfileChain_CycleGuarded(t *testing.T) {
	configPath := filepath.Join("..", "testdata", "aws_profile", "config_source_profile_chain")

	done := make(chan string, 1)
	go func() {
		done <- awsclient.GetDefaultRegion(configPath, "role-cycle-a")
	}()

	select {
	case region := <-done:
		if region != "us-east-1" {
			t.Errorf("expected fallthrough to [default] region %q on source_profile cycle, got %q", "us-east-1", region)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("GetDefaultRegion did not terminate on a source_profile cycle — missing cycle guard")
	}
}

func TestGetDefaultRegion_SourceProfileChain_NoRegionAnywhere(t *testing.T) {
	configPath := filepath.Join("..", "testdata", "aws_profile", "config_source_profile_no_region_anywhere")

	region := awsclient.GetDefaultRegion(configPath, "role-no-region")
	if region != "us-east-1" {
		t.Errorf("expected fallback region %q when no region exists anywhere in the chain, got %q", "us-east-1", region)
	}
}

func TestGetDefaultRegion_EnvRegion_WinsOverProfileAndChain(t *testing.T) {
	t.Setenv("AWS_REGION", "ap-southeast-1")
	t.Setenv("AWS_DEFAULT_REGION", "")

	configPath := filepath.Join("..", "testdata", "aws_profile", "config_source_profile_chain")

	region := awsclient.GetDefaultRegion(configPath, "source-with-region")
	if region != "ap-southeast-1" {
		t.Errorf("expected AWS_REGION env %q to win over profile region, got %q", "ap-southeast-1", region)
	}
}

func TestGetDefaultRegion_EnvDefaultRegion_WinsWhenAWSRegionUnset(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "sa-east-1")

	configPath := filepath.Join("..", "testdata", "aws_profile", "config_source_profile_chain")

	region := awsclient.GetDefaultRegion(configPath, "source-with-region")
	if region != "sa-east-1" {
		t.Errorf("expected AWS_DEFAULT_REGION env %q to win over profile region, got %q", "sa-east-1", region)
	}
}

func TestGetDefaultRegion_ProfileOwnRegion_UnchangedWhenNoEnv(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")

	configPath := filepath.Join("..", "testdata", "aws_profile", "config_source_profile_chain")

	region := awsclient.GetDefaultRegion(configPath, "role-own-region")
	if region != "ap-northeast-1" {
		t.Errorf("expected profile's own region %q (not source_profile's), got %q", "ap-northeast-1", region)
	}
}

func TestGetDefaultRegion_NothingAnywhere_FallsBackToUsEast1(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")

	configPath := filepath.Join("..", "testdata", "aws_profile", "config_no_default_region")
	region := awsclient.GetDefaultRegion(configPath, "dev-with-no-region-here")
	if region != "us-east-1" {
		t.Errorf("expected fallback region %q, got %q", "us-east-1", region)
	}
}

// ini.v1 parity: key lookups (region, source_profile) are case-insensitive
// (InsensitiveKeys); section names are exact-case — do not "fix" findINISection.
func TestGetDefaultRegion_CapitalizedKeys_MainParity(t *testing.T) {
	configPath := filepath.Join("..", "testdata", "aws_profile", "config_case_insensitive_keys")

	cases := []struct {
		profile string
		want    string
	}{
		{"cap-region", "eu-central-1"},
		{"cap-source", "ap-southeast-2"},
	}
	for _, tc := range cases {
		if got := awsclient.GetDefaultRegion(configPath, tc.profile); got != tc.want {
			t.Errorf("GetDefaultRegion(%q) = %q, want %q", tc.profile, got, tc.want)
		}
	}
}
