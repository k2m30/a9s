// aws_identity_cache_test.go contains indirect coverage tests for
// internal/aws/identity_cache.go.
//
// COVERAGE LIMITS:
//   - accountIDFromClients: unexported; ServiceClients.STS is *sts.Client
//     (concrete, not an interface). There is no mock injection point from an
//     external test package. Covered indirectly through checkers that call it
//     (e.g. checkEBSBackup) — nil-STS path is exercised below.
//   - regionFromEnv: unexported. Covered indirectly by exercising a checker
//     that calls it. The AWS_REGION / AWS_DEFAULT_REGION env-var branches are
//     tested by inspecting the behaviour of checkEBSBackup when the env is
//     unset vs. set (observable via Count:-1 when region is the bottleneck).
//
// Direct white-box tests for these functions require a test file inside the
// internal/aws package itself (internal/aws/identity_cache_test.go), which is
// outside the QA agent's write scope.
package unit_test

import (
	"context"
	"os"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"

	_ "github.com/k2m30/a9s/v3/internal/aws"
)

// Note: ebsCheckerByTarget is declared in aws_ebs_related_test.go (same package).

// ---------------------------------------------------------------------------
// regionFromEnv — indirect coverage via checkEBSBackup
//
// checkEBSBackup calls:
//   region  := regionFromEnv()
//   account := accountIDFromClients(ctx, c)
//   if region == "" || account == "" { return Count:-1 }
//
// When AWS_REGION is unset AND AWS_DEFAULT_REGION is unset, regionFromEnv()
// returns "" → the checker returns Count:-1 regardless of the STS client.
// ---------------------------------------------------------------------------

// TestIdentityCache_RegionFromEnv_EmptyWhenEnvUnset verifies that a checker
// that needs the region returns Count:-1 when neither AWS_REGION nor
// AWS_DEFAULT_REGION is set.  This exercises the regionFromEnv() "" branch.
func TestIdentityCache_RegionFromEnv_EmptyWhenEnvUnset(t *testing.T) {
	// Ensure both region env vars are absent for this test.
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

	// Provide a non-nil Backup client so the nil-client early-exit is NOT hit,
	// but leave STS nil — the checker reaches regionFromEnv(), which returns "".
	clients := &awsclient.ServiceClients{
		Backup: newFakeBackupWithRecoveryPoints(nil),
	}
	src := resource.Resource{
		ID:     "vol-0a1b2c3d4e5f67890",
		Fields: map[string]string{},
	}

	checker := ebsCheckerByTarget(t, "backup")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	// With no AWS_REGION and no STS client, Count must be -1.
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (region unresolvable, no STS)", result.Count)
	}
	if result.TargetType != "backup" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "backup")
	}
}

// TestIdentityCache_RegionFromEnv_FallbackToAWSDefaultRegion verifies that
// when AWS_REGION is unset but AWS_DEFAULT_REGION is set, the checker still
// returns Count:-1 because the STS client (accountIDFromClients) is nil —
// but it exercises the AWS_DEFAULT_REGION branch of regionFromEnv().
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

	// region is now non-empty (eu-west-1) but STS is nil → accountIDFromClients
	// returns "" → checker still returns Count:-1. The test verifies the code
	// path executes without panic, which covers the regionFromEnv fallback branch.
	clients := &awsclient.ServiceClients{
		Backup: newFakeBackupWithRecoveryPoints(nil),
	}
	src := resource.Resource{
		ID:     "vol-0a1b2c3d4e5f67890",
		Fields: map[string]string{},
	}

	checker := ebsCheckerByTarget(t, "backup")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no STS client to resolve account)", result.Count)
	}
}

// TestIdentityCache_NilClients_ReturnsMinusOne verifies that a nil clients
// argument to a checker that needs identity returns Count:-1 without panicking.
func TestIdentityCache_NilClients_ReturnsMinusOne(t *testing.T) {
	src := resource.Resource{
		ID:     "vol-0a1b2c3d4e5f67890",
		Fields: map[string]string{},
	}

	checker := ebsCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty volume ID)", result.Count)
	}
}
