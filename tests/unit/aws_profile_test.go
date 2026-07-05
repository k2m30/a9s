package unit

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// T032 - Test profile enumeration
// ---------------------------------------------------------------------------

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
	configPath := filepath.Join("..", "testdata", "aws_profile", "config_sample")

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
	configPath := filepath.Join("..", "testdata", "aws_profile", "config_sample")
	region := awsclient.GetDefaultRegion(configPath, "nonexistent-profile")
	if region != "us-east-1" {
		t.Errorf("expected fallback region %q for unknown profile, got %q", "us-east-1", region)
	}
}

// ---------------------------------------------------------------------------
// DEF-14/D11 Cause A — GetDefaultRegion static resolution chain:
// AWS_REGION env > AWS_DEFAULT_REGION env > profile region >
// source_profile chain (recursive, cycle-guarded) > [default] region >
// "us-east-1" fallback.
//
// Today GetDefaultRegion only reads the requested profile's own "region"
// key and falls straight to "us-east-1" otherwise — it never consults env
// vars, never follows source_profile, and never falls back to [default]'s
// region. Every sub-test below except (e) is RED against that behavior.
// ---------------------------------------------------------------------------

// TestGetDefaultRegion_SourceProfileChain_SingleHop verifies (a): a profile
// with only role_arn+source_profile (no own region) resolves the region
// from its source_profile. RED today: GetDefaultRegion has no source_profile
// awareness at all, so it returns the "us-east-1" fallback instead of
// "eu-central-1".
func TestGetDefaultRegion_SourceProfileChain_SingleHop(t *testing.T) {
	configPath := filepath.Join("..", "testdata", "aws_profile", "config_source_profile_chain")

	region := awsclient.GetDefaultRegion(configPath, "role-only")
	if region != "eu-central-1" {
		t.Errorf("expected region %q resolved from source_profile chain, got %q", "eu-central-1", region)
	}
}

// TestGetDefaultRegion_SourceProfileChain_TwoHop verifies (b): a
// source_profile chain two levels deep (role-two-level -> role-only ->
// source-with-region) still resolves to the region at the end of the
// chain.
func TestGetDefaultRegion_SourceProfileChain_TwoHop(t *testing.T) {
	configPath := filepath.Join("..", "testdata", "aws_profile", "config_source_profile_chain")

	region := awsclient.GetDefaultRegion(configPath, "role-two-level")
	if region != "eu-central-1" {
		t.Errorf("expected region %q resolved from two-hop source_profile chain, got %q", "eu-central-1", region)
	}
}

// TestGetDefaultRegion_SourceProfileChain_CycleGuarded verifies (c): a
// source_profile cycle (role-cycle-a <-> role-cycle-b) does not hang the
// resolver — it must terminate and fall through to [default]'s region
// (us-east-1, from config_source_profile_chain's [default] section).
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

// TestGetDefaultRegion_SourceProfileChain_NoRegionAnywhere verifies (f) via
// the source_profile path specifically: when neither the profile, its
// source_profile, nor [default] carry a region, GetDefaultRegion still
// falls through to the hardcoded "us-east-1" fallback (not empty, not a
// panic).
func TestGetDefaultRegion_SourceProfileChain_NoRegionAnywhere(t *testing.T) {
	configPath := filepath.Join("..", "testdata", "aws_profile", "config_source_profile_no_region_anywhere")

	region := awsclient.GetDefaultRegion(configPath, "role-no-region")
	if region != "us-east-1" {
		t.Errorf("expected fallback region %q when no region exists anywhere in the chain, got %q", "us-east-1", region)
	}
}

// TestGetDefaultRegion_EnvRegion_WinsOverProfileAndChain verifies (d):
// AWS_REGION set in the environment takes precedence over any profile or
// source_profile region — even when the requested profile has its own
// explicit region on disk. RED today: GetDefaultRegion never reads any env
// var, so it returns the profile's on-disk region ("eu-central-1")
// instead of the env-supplied "ap-southeast-1".
func TestGetDefaultRegion_EnvRegion_WinsOverProfileAndChain(t *testing.T) {
	t.Setenv("AWS_REGION", "ap-southeast-1")
	t.Setenv("AWS_DEFAULT_REGION", "")

	configPath := filepath.Join("..", "testdata", "aws_profile", "config_source_profile_chain")

	region := awsclient.GetDefaultRegion(configPath, "source-with-region")
	if region != "ap-southeast-1" {
		t.Errorf("expected AWS_REGION env %q to win over profile region, got %q", "ap-southeast-1", region)
	}
}

// TestGetDefaultRegion_EnvDefaultRegion_WinsWhenAWSRegionUnset verifies the
// AWS_DEFAULT_REGION fallback ordering: with AWS_REGION unset but
// AWS_DEFAULT_REGION set, the env value still wins over the profile's own
// on-disk region.
func TestGetDefaultRegion_EnvDefaultRegion_WinsWhenAWSRegionUnset(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "sa-east-1")

	configPath := filepath.Join("..", "testdata", "aws_profile", "config_source_profile_chain")

	region := awsclient.GetDefaultRegion(configPath, "source-with-region")
	if region != "sa-east-1" {
		t.Errorf("expected AWS_DEFAULT_REGION env %q to win over profile region, got %q", "sa-east-1", region)
	}
}

// TestGetDefaultRegion_ProfileOwnRegion_UnchangedWhenNoEnv verifies (e): the
// existing green behavior is preserved — a profile with its own region key
// (even one that ALSO carries role_arn+source_profile) resolves to its own
// region, not the source_profile's, when no env var is set.
func TestGetDefaultRegion_ProfileOwnRegion_UnchangedWhenNoEnv(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")

	configPath := filepath.Join("..", "testdata", "aws_profile", "config_source_profile_chain")

	region := awsclient.GetDefaultRegion(configPath, "role-own-region")
	if region != "ap-northeast-1" {
		t.Errorf("expected profile's own region %q (not source_profile's), got %q", "ap-northeast-1", region)
	}
}

// TestGetDefaultRegion_NothingAnywhere_FallsBackToUsEast1 verifies (f): a
// profile that does not exist, in a config file with no [default] region
// and no env vars set, falls back to "us-east-1" — the pre-existing
// TestGetDefaultRegion_MissingFile / _UnknownProfile behavior must survive
// the new resolution chain unchanged.
func TestGetDefaultRegion_NothingAnywhere_FallsBackToUsEast1(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")

	configPath := filepath.Join("..", "testdata", "aws_profile", "config_no_default_region")
	region := awsclient.GetDefaultRegion(configPath, "dev-with-no-region-here")
	if region != "us-east-1" {
		t.Errorf("expected fallback region %q, got %q", "us-east-1", region)
	}
}
