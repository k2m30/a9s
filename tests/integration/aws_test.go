//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

func skipIfNoAWSConfig(t *testing.T) {
	t.Helper()
	configPath := awsclient.DefaultConfigPath()

	if _, err := os.Stat(configPath); err != nil {
		t.Skip("no AWS config file found; skipping real AWS test")
	}
}

func TestQA_170_InvalidProfileNameError(t *testing.T) {
	_, err := awsclient.NewAWSSessionContext(context.Background(), "this-profile-definitely-does-not-exist-xyz123", "us-east-1")
	// The AWS SDK may or may not error depending on configuration.
	if err != nil {
		t.Logf("NewAWSSession with invalid profile returned error (expected): %v", err)
	} else {
		t.Log("NewAWSSession with invalid profile succeeded (SDK may defer auth check)")
	}
}

func TestQA_171_RegionNoServiceSupport(t *testing.T) {
	skipIfNoAWSConfig(t)

	cfg, err := awsclient.NewAWSSessionContext(context.Background(), "", "af-south-1")
	if err != nil {
		t.Skipf("could not create AWS session: %v", err)
	}
	clients := awsclient.CreateServiceClients(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err = awsclient.FetchEKSClustersPage(ctx, clients, "")
	if err != nil {
		t.Logf("FetchEKSClusters in af-south-1 returned error (may be expected): %v", err)
	} else {
		t.Log("FetchEKSClusters in af-south-1 succeeded")
	}
}

func TestQA_173_InitConnectMsgFailure(t *testing.T) {
	origConfig := os.Getenv("AWS_CONFIG_FILE")
	origCreds := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	t.Setenv("AWS_CONFIG_FILE", "/nonexistent/path/config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent/path/credentials")
	defer func() {
		os.Setenv("AWS_CONFIG_FILE", origConfig)
		os.Setenv("AWS_SHARED_CREDENTIALS_FILE", origCreds)
	}()

	_, err := awsclient.NewAWSSessionContext(context.Background(), "nonexistent-profile", "us-east-1")
	if err != nil {
		t.Logf("NewAWSSession failed as expected: %v", err)
	} else {
		t.Log("NewAWSSession succeeded even with missing config (SDK defaults used)")
	}
}

func TestQA_074_SSOExpiredToken(t *testing.T) {
	if os.Getenv("A9S_TEST_SSO_PROFILE") == "" {
		t.Skip("set A9S_TEST_SSO_PROFILE to an SSO profile name to test expired token behavior")
	}

	ssoProfile := os.Getenv("A9S_TEST_SSO_PROFILE")
	cfg, err := awsclient.NewAWSSessionContext(context.Background(), ssoProfile, "us-east-1")
	if err != nil {
		t.Logf("NewAWSSession for SSO profile %q failed: %v", ssoProfile, err)
		return
	}

	clients := awsclient.CreateServiceClients(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err = awsclient.FetchEC2InstancesPage(ctx, clients.EC2, "")
	if err != nil {
		t.Logf("FetchEC2InstancesPage with SSO profile returned error: %v", err)
	} else {
		t.Log("FetchEC2InstancesPage with SSO profile succeeded (token may be valid)")
	}
}

func TestQA_200_S3ListingGlobalRegardlessOfRegion(t *testing.T) {
	skipIfNoAWSConfig(t)

	// S3 ListBuckets is a global operation: every region returns the same buckets.
	cfg1, err := awsclient.NewAWSSessionContext(context.Background(), "", "us-east-1")
	if err != nil {
		t.Skipf("could not create AWS session for us-east-1: %v", err)
	}
	cfg2, err := awsclient.NewAWSSessionContext(context.Background(), "", "eu-west-1")
	if err != nil {
		t.Skipf("could not create AWS session for eu-west-1: %v", err)
	}

	clients1 := awsclient.CreateServiceClients(cfg1)
	clients2 := awsclient.CreateServiceClients(cfg2)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result1, err := awsclient.FetchS3BucketsPageWithNotifications(ctx, clients1.S3, nil, "")
	if err != nil {
		t.Skipf("could not fetch S3 buckets from us-east-1: %v", err)
	}
	buckets1 := result1.Resources

	result2, err := awsclient.FetchS3BucketsPageWithNotifications(ctx, clients2.S3, nil, "")
	if err != nil {
		t.Skipf("could not fetch S3 buckets from eu-west-1: %v", err)
	}
	buckets2 := result2.Resources

	if len(buckets1) != len(buckets2) {
		t.Errorf("S3 bucket count differs between regions: us-east-1=%d, eu-west-1=%d", len(buckets1), len(buckets2))
	} else {
		t.Logf("S3 bucket count matches across regions: %d buckets", len(buckets1))
	}

	names1 := make(map[string]bool)
	for _, b := range buckets1 {
		names1[b.Name] = true
	}

	for _, b := range buckets2 {
		if !names1[b.Name] {
			t.Errorf("bucket %q found in eu-west-1 but not in us-east-1", b.Name)
		}
	}
}

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

func TestIntegration_NewAWSSessionDefaultProfile(t *testing.T) {
	skipIfNoAWSConfig(t)

	cfg, err := awsclient.NewAWSSessionContext(context.Background(), "", "us-east-1")
	if err != nil {
		t.Fatalf("NewAWSSession with default profile failed: %v", err)
	}
	if cfg.Region != "us-east-1" {
		t.Errorf("expected region 'us-east-1', got %q", cfg.Region)
	}
}

func TestIntegration_FetchEC2Instances(t *testing.T) {
	skipIfNoAWSConfig(t)

	cfg, err := awsclient.NewAWSSessionContext(context.Background(), "", "us-east-1")
	if err != nil {
		t.Skipf("could not create AWS session: %v", err)
	}
	clients := awsclient.CreateServiceClients(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := awsclient.FetchEC2InstancesPage(ctx, clients.EC2, "")
	if err != nil {
		t.Logf("FetchEC2InstancesPage returned error (may be auth): %v", err)
		return
	}
	t.Logf("FetchEC2InstancesPage returned %d instances", len(result.Resources))
}

func TestIntegration_FetchS3Buckets(t *testing.T) {
	skipIfNoAWSConfig(t)

	cfg, err := awsclient.NewAWSSessionContext(context.Background(), "", "us-east-1")
	if err != nil {
		t.Skipf("could not create AWS session: %v", err)
	}
	clients := awsclient.CreateServiceClients(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := awsclient.FetchS3BucketsPageWithNotifications(ctx, clients.S3, nil, "")
	if err != nil {
		t.Logf("FetchS3BucketsPageWithNotifications returned error (may be auth): %v", err)
		return
	}
	t.Logf("FetchS3BucketsPageWithNotifications returned %d buckets", len(result.Resources))
}
