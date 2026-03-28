package unit

// reveal_registry_test.go tests the reveal registry functions that will be
// added to internal/resource/registry.go as part of issue #104.
// These tests will FAIL until the coder adds RegisterRevealFetcher,
// GetRevealFetcher, UnregisterRevealFetcher, HasRevealFetcher to registry.go
// and updates secrets.go and ssm.go to register reveal fetchers in init().

import (
	"context"
	"testing"

	// Import internal/aws to trigger init() registrations for "secrets" and "ssm".
	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// TestRevealRegistry_RegisterAndGet
// ---------------------------------------------------------------------------

// TestRevealRegistry_RegisterAndGet verifies that a registered reveal fetcher
// can be retrieved with GetRevealFetcher.
func TestRevealRegistry_RegisterAndGet(t *testing.T) {
	const shortName = "test_reveal_register"
	fetcher := func(_ context.Context, _ interface{}, _ string) (string, error) {
		return "value", nil
	}

	resource.RegisterRevealFetcher(shortName, fetcher)
	t.Cleanup(func() { resource.UnregisterRevealFetcher(shortName) })

	got := resource.GetRevealFetcher(shortName)
	if got == nil {
		t.Fatal("GetRevealFetcher returned nil after RegisterRevealFetcher")
	}

	// Verify the returned function is callable and returns the expected value.
	val, err := got(context.Background(), nil, "any-id")
	if err != nil {
		t.Errorf("reveal fetcher returned unexpected error: %v", err)
	}
	if val != "value" {
		t.Errorf("reveal fetcher returned %q, want %q", val, "value")
	}
}

// ---------------------------------------------------------------------------
// TestRevealRegistry_GetUnregistered
// ---------------------------------------------------------------------------

// TestRevealRegistry_GetUnregistered verifies that GetRevealFetcher returns nil
// for a short name that has never been registered.
func TestRevealRegistry_GetUnregistered(t *testing.T) {
	got := resource.GetRevealFetcher("definitely_not_registered_reveal_xyz")
	if got != nil {
		t.Fatal("GetRevealFetcher should return nil for unregistered type, got non-nil")
	}
}

// ---------------------------------------------------------------------------
// TestRevealRegistry_HasRevealFetcher
// ---------------------------------------------------------------------------

// TestRevealRegistry_HasRevealFetcher verifies that HasRevealFetcher returns
// true for registered types and false for unregistered ones.
func TestRevealRegistry_HasRevealFetcher(t *testing.T) {
	const shortName = "test_has_reveal"
	fetcher := func(_ context.Context, _ interface{}, _ string) (string, error) {
		return "", nil
	}

	// Before registration, should be false.
	if resource.HasRevealFetcher(shortName) {
		t.Fatal("HasRevealFetcher returned true before registration")
	}

	resource.RegisterRevealFetcher(shortName, fetcher)
	t.Cleanup(func() { resource.UnregisterRevealFetcher(shortName) })

	// After registration, should be true.
	if !resource.HasRevealFetcher(shortName) {
		t.Fatal("HasRevealFetcher returned false after registration")
	}
}

// ---------------------------------------------------------------------------
// TestRevealRegistry_Unregister
// ---------------------------------------------------------------------------

// TestRevealRegistry_Unregister verifies that UnregisterRevealFetcher removes
// a previously registered fetcher.
func TestRevealRegistry_Unregister(t *testing.T) {
	const shortName = "test_unregister_reveal"
	fetcher := func(_ context.Context, _ interface{}, _ string) (string, error) {
		return "", nil
	}

	resource.RegisterRevealFetcher(shortName, fetcher)

	// Verify it was registered.
	if resource.GetRevealFetcher(shortName) == nil {
		t.Fatal("expected fetcher to be registered before unregister")
	}

	resource.UnregisterRevealFetcher(shortName)

	// After unregister, should be nil.
	if resource.GetRevealFetcher(shortName) != nil {
		t.Fatal("GetRevealFetcher should return nil after UnregisterRevealFetcher")
	}
	if resource.HasRevealFetcher(shortName) {
		t.Fatal("HasRevealFetcher should return false after UnregisterRevealFetcher")
	}
}

// ---------------------------------------------------------------------------
// TestRevealRegistry_SecretsRegistered
// ---------------------------------------------------------------------------

// TestRevealRegistry_SecretsRegistered verifies that importing internal/aws
// causes the "secrets" type to have a reveal fetcher registered via init().
func TestRevealRegistry_SecretsRegistered(t *testing.T) {
	if !resource.HasRevealFetcher("secrets") {
		t.Fatal("expected 'secrets' to have a reveal fetcher registered via init(), got none")
	}
	if resource.GetRevealFetcher("secrets") == nil {
		t.Fatal("GetRevealFetcher('secrets') returned nil, expected a non-nil fetcher")
	}
}

// ---------------------------------------------------------------------------
// TestRevealRegistry_SSMRegistered
// ---------------------------------------------------------------------------

// TestRevealRegistry_SSMRegistered verifies that importing internal/aws
// causes the "ssm" type to have a reveal fetcher registered via init().
func TestRevealRegistry_SSMRegistered(t *testing.T) {
	if !resource.HasRevealFetcher("ssm") {
		t.Fatal("expected 'ssm' to have a reveal fetcher registered via init(), got none")
	}
	if resource.GetRevealFetcher("ssm") == nil {
		t.Fatal("GetRevealFetcher('ssm') returned nil, expected a non-nil fetcher")
	}
}

// ---------------------------------------------------------------------------
// TestRevealRegistry_EC2NotRegistered
// ---------------------------------------------------------------------------

// TestRevealRegistry_EC2NotRegistered verifies that "ec2" does NOT have a
// reveal fetcher — it is not a secret-bearing resource type.
func TestRevealRegistry_EC2NotRegistered(t *testing.T) {
	if resource.HasRevealFetcher("ec2") {
		t.Fatal("'ec2' should NOT have a reveal fetcher, but one is registered")
	}
	if resource.GetRevealFetcher("ec2") != nil {
		t.Fatal("GetRevealFetcher('ec2') should return nil")
	}
}
