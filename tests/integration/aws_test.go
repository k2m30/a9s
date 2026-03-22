//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// skipIfNoAWSConfig skips the test if the user has no AWS configuration available.
func skipIfNoAWSConfig(t *testing.T) {
	t.Helper()
	configPath := awsclient.DefaultConfigPath()

	if _, err := os.Stat(configPath); err != nil {
		t.Skip("no AWS config file found; skipping real AWS test")
	}
}

// QA-170: Invalid profile name error on startup
func TestQA_170_InvalidProfileNameError(t *testing.T) {
	// This test verifies that using an invalid/nonexistent profile name
	// results in a meaningful error rather than a panic.
	_, err := awsclient.NewAWSSession("this-profile-definitely-does-not-exist-xyz123", "us-east-1")
	// The AWS SDK may or may not error depending on configuration.
	// The key assertion is that it does not panic.
	if err != nil {
		t.Logf("NewAWSSession with invalid profile returned error (expected): %v", err)
	} else {
		t.Log("NewAWSSession with invalid profile succeeded (SDK may defer auth check)")
	}
}

// QA-171: Region with no support for service
func TestQA_171_RegionNoServiceSupport(t *testing.T) {
	skipIfNoAWSConfig(t)

	// Use a real session but try to list EKS clusters in a region that may
	// have limited service availability. The test verifies no panic occurs.
	cfg, err := awsclient.NewAWSSession("", "af-south-1")
	if err != nil {
		t.Skipf("could not create AWS session: %v", err)
	}
	clients := awsclient.CreateServiceClients(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Attempt to fetch EKS clusters -- may return an error for unsupported regions,
	// but should not panic.
	_, err = awsclient.FetchEKSClusters(ctx, clients.EKS, clients.EKS)
	if err != nil {
		t.Logf("FetchEKSClusters in af-south-1 returned error (may be expected): %v", err)
	} else {
		t.Log("FetchEKSClusters in af-south-1 succeeded")
	}
}

// QA-173: InitConnectMsg failure on startup
func TestQA_173_InitConnectMsgFailure(t *testing.T) {
	// This tests the path where NewAWSSession fails during InitConnectMsg processing.
	// We point at a nonexistent config to simulate failure.
	origConfig := os.Getenv("AWS_CONFIG_FILE")
	origCreds := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	t.Setenv("AWS_CONFIG_FILE", "/nonexistent/path/config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent/path/credentials")
	defer func() {
		os.Setenv("AWS_CONFIG_FILE", origConfig)
		os.Setenv("AWS_SHARED_CREDENTIALS_FILE", origCreds)
	}()

	// NewAWSSession with a profile that doesn't exist in the (nonexistent) config
	_, err := awsclient.NewAWSSession("nonexistent-profile", "us-east-1")
	// Should not panic, may or may not error
	if err != nil {
		t.Logf("NewAWSSession failed as expected: %v", err)
	} else {
		t.Log("NewAWSSession succeeded even with missing config (SDK defaults used)")
	}
}

// QA-074: Switch to profile with SSO (expired token)
func TestQA_074_SSOExpiredToken(t *testing.T) {
	if os.Getenv("A9S_TEST_SSO_PROFILE") == "" {
		t.Skip("set A9S_TEST_SSO_PROFILE to an SSO profile name to test expired token behavior")
	}

	ssoProfile := os.Getenv("A9S_TEST_SSO_PROFILE")
	cfg, err := awsclient.NewAWSSession(ssoProfile, "us-east-1")
	if err != nil {
		t.Logf("NewAWSSession for SSO profile %q failed: %v", ssoProfile, err)
		return
	}

	clients := awsclient.CreateServiceClients(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Try to fetch EC2 instances -- if SSO token is expired, this should return
	// an error containing "ExpiredToken" or similar.
	_, err = awsclient.FetchEC2Instances(ctx, clients.EC2)
	if err != nil {
		t.Logf("FetchEC2Instances with SSO profile returned error: %v", err)
	} else {
		t.Log("FetchEC2Instances with SSO profile succeeded (token may be valid)")
	}
}

// QA-200: S3 listing is global regardless of region
func TestQA_200_S3ListingGlobalRegardlessOfRegion(t *testing.T) {
	skipIfNoAWSConfig(t)

	// Create sessions in two different regions and compare S3 bucket lists.
	// S3 ListBuckets is a global operation, so both should return the same buckets.
	cfg1, err := awsclient.NewAWSSession("", "us-east-1")
	if err != nil {
		t.Skipf("could not create AWS session for us-east-1: %v", err)
	}
	cfg2, err := awsclient.NewAWSSession("", "eu-west-1")
	if err != nil {
		t.Skipf("could not create AWS session for eu-west-1: %v", err)
	}

	clients1 := awsclient.CreateServiceClients(cfg1)
	clients2 := awsclient.CreateServiceClients(cfg2)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	buckets1, err := awsclient.FetchS3Buckets(ctx, clients1.S3)
	if err != nil {
		t.Skipf("could not fetch S3 buckets from us-east-1: %v", err)
	}

	buckets2, err := awsclient.FetchS3Buckets(ctx, clients2.S3)
	if err != nil {
		t.Skipf("could not fetch S3 buckets from eu-west-1: %v", err)
	}

	// Both regions should return the same number of buckets
	if len(buckets1) != len(buckets2) {
		t.Errorf("S3 bucket count differs between regions: us-east-1=%d, eu-west-1=%d", len(buckets1), len(buckets2))
	} else {
		t.Logf("S3 bucket count matches across regions: %d buckets", len(buckets1))
	}

	// Build a set of bucket names from region 1
	names1 := make(map[string]bool)
	for _, b := range buckets1 {
		names1[b.Name] = true
	}

	// Check that all buckets from region 2 also appear in region 1
	for _, b := range buckets2 {
		if !names1[b.Name] {
			t.Errorf("bucket %q found in eu-west-1 but not in us-east-1", b.Name)
		}
	}
}

// Test: ListProfiles returns non-empty list from real ~/.aws/config
func TestIntegration_ListProfilesReal(t *testing.T) {
	skipIfNoAWSConfig(t)

	profiles, err := awsclient.ListProfiles(awsclient.DefaultConfigPath())
	if err != nil {
		t.Fatalf("ListProfiles failed: %v", err)
	}
	if len(profiles) == 0 {
		t.Error("expected at least one AWS profile from real config")
	}
	t.Logf("found %d profiles: %v", len(profiles), profiles)
}

// Test: NewAWSSession with default profile succeeds
func TestIntegration_NewAWSSessionDefaultProfile(t *testing.T) {
	skipIfNoAWSConfig(t)

	cfg, err := awsclient.NewAWSSession("", "us-east-1")
	if err != nil {
		t.Fatalf("NewAWSSession with default profile failed: %v", err)
	}
	if cfg.Region != "us-east-1" {
		t.Errorf("expected region 'us-east-1', got %q", cfg.Region)
	}
}

// Test: FetchEC2Instances with real client (may return 0 instances, just verify no panic)
func TestIntegration_FetchEC2Instances(t *testing.T) {
	skipIfNoAWSConfig(t)

	cfg, err := awsclient.NewAWSSession("", "us-east-1")
	if err != nil {
		t.Skipf("could not create AWS session: %v", err)
	}
	clients := awsclient.CreateServiceClients(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resources, err := awsclient.FetchEC2Instances(ctx, clients.EC2)
	if err != nil {
		t.Logf("FetchEC2Instances returned error (may be auth): %v", err)
		return
	}
	t.Logf("FetchEC2Instances returned %d instances", len(resources))
}

// Test: FetchS3Buckets with real client
func TestIntegration_FetchS3Buckets(t *testing.T) {
	skipIfNoAWSConfig(t)

	cfg, err := awsclient.NewAWSSession("", "us-east-1")
	if err != nil {
		t.Skipf("could not create AWS session: %v", err)
	}
	clients := awsclient.CreateServiceClients(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resources, err := awsclient.FetchS3Buckets(ctx, clients.S3)
	if err != nil {
		t.Logf("FetchS3Buckets returned error (may be auth): %v", err)
		return
	}
	t.Logf("FetchS3Buckets returned %d buckets", len(resources))
}
