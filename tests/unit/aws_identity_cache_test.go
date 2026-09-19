// aws_identity_cache_test.go exercises core/aws/identity_cache.go through
// checkEBSBackup: accountIDFromClients and regionFromEnv are unexported, and
// ServiceClients.STS is a concrete *sts.Client with no mock injection point
// from an external test package.
package unit_test

import (
	"context"
	"os"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"

	_ "github.com/k2m30/a9s/v3/core/aws"
)

func TestIdentityCache_RegionFromEnv_EmptyWhenEnvUnset(t *testing.T) {
	orig1, has1 := os.LookupEnv("AWS_REGION")
	orig2, has2 := os.LookupEnv("AWS_DEFAULT_REGION")
	if err := os.Unsetenv("AWS_REGION"); err != nil {
		t.Fatalf("cannot unset AWS_REGION: %v", err)
	}
	if err := os.Unsetenv("AWS_DEFAULT_REGION"); err != nil {
		t.Fatalf("cannot unset AWS_DEFAULT_REGION: %v", err)
	}
	t.Cleanup(func() {
		if has1 {
			_ = os.Setenv("AWS_REGION", orig1)
		}
		if has2 {
			_ = os.Setenv("AWS_DEFAULT_REGION", orig2)
		}
	})

	// A non-nil Backup client passes the nil-client early exit; STS stays nil.
	clients := &awsclient.ServiceClients{
		Backup: newFakeBackupWithRecoveryPoints(nil),
	}
	src := resource.Resource{
		ID:     "vol-0a1b2c3d4e5f67890",
		Fields: map[string]string{},
	}

	checker := ebsCheckerByTarget(t, "backup")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	// The fake Backup client returns an empty plan list — a SUCCESSFUL fetch of a
	// zero-size target population. A fully-scanned empty population is a proven
	// zero, so the checker resolves to "(0)", not a blank/unknown row.
	if result.State() != domain.RelatedResolved {
		t.Errorf("State = %v, want RelatedResolved (0 backup plans → proven zero (0))", result.State())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
	if result.TargetType() != "backup" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "backup")
	}
}

func TestIdentityCache_RegionFromEnv_FallbackToAWSDefaultRegion(t *testing.T) {
	orig1, has1 := os.LookupEnv("AWS_REGION")
	orig2, has2 := os.LookupEnv("AWS_DEFAULT_REGION")
	if err := os.Unsetenv("AWS_REGION"); err != nil {
		t.Fatalf("cannot unset AWS_REGION: %v", err)
	}
	if err := os.Setenv("AWS_DEFAULT_REGION", "eu-west-1"); err != nil {
		t.Fatalf("cannot set AWS_DEFAULT_REGION: %v", err)
	}
	t.Cleanup(func() {
		if has1 {
			_ = os.Setenv("AWS_REGION", orig1)
		} else {
			_ = os.Unsetenv("AWS_REGION")
		}
		if has2 {
			_ = os.Setenv("AWS_DEFAULT_REGION", orig2)
		} else {
			_ = os.Unsetenv("AWS_DEFAULT_REGION")
		}
	})

	clients := &awsclient.ServiceClients{
		Backup: newFakeBackupWithRecoveryPoints(nil),
	}
	src := resource.Resource{
		ID:     "vol-0a1b2c3d4e5f67890",
		Fields: map[string]string{},
	}

	checker := ebsCheckerByTarget(t, "backup")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	// Empty plan list from a successful fetch → proven zero (0), regardless of
	// region/account resolution (there are no plans to match against).
	if result.State() != domain.RelatedResolved {
		t.Errorf("State = %v, want RelatedResolved (0 backup plans → proven zero (0))", result.State())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

// TestIdentityCache_NilClients_ReturnsMinusOne verifies that a nil clients
// argument to a checker that needs identity returns State: RelatedUnknown without panicking.
func TestIdentityCache_NilClients_ReturnsMinusOne(t *testing.T) {
	src := resource.Resource{
		ID:     "vol-0a1b2c3d4e5f67890",
		Fields: map[string]string{},
	}

	checker := ebsCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v, want RelatedUnknown (nil clients)", result.State())
	}
}

// TestIdentityCache_EmptyVolumeID_ReturnsZero verifies that an empty resource
// ID short-circuits before any identity lookup, returning Count:0.
func TestIdentityCache_EmptyVolumeID_ReturnsZero(t *testing.T) {
	src := resource.Resource{
		ID:     "",
		Fields: map[string]string{},
	}

	checker := ebsCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty volume ID)", result.Count())
	}
}
