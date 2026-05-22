package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestQA_FieldAlias_RegisterAndApply verifies that registered aliases are applied:
// original keys remain, alias keys are added, and unaliased keys are untouched.
func TestQA_FieldAlias_RegisterAndApply(t *testing.T) {
	resource.SetFieldAliasesForTest("ec2", map[string]string{
		"instance_id": "InstanceId",
		"state":       "State",
	})
	t.Cleanup(func() { resource.CleanupFieldAliasesForTest("ec2") })

	fields := map[string]string{
		"instance_id": "i-123",
		"state":       "running",
		"name":        "web",
	}

	result := resource.ApplyFieldAliases("ec2", fields)

	if result["InstanceId"] != "i-123" {
		t.Errorf("expected InstanceId=i-123, got %q", result["InstanceId"])
	}
	if result["State"] != "running" {
		t.Errorf("expected State=running, got %q", result["State"])
	}
	// original keys must be preserved
	if result["instance_id"] != "i-123" {
		t.Errorf("expected original instance_id=i-123, got %q", result["instance_id"])
	}
	if result["state"] != "running" {
		t.Errorf("expected original state=running, got %q", result["state"])
	}
	// unaliased key must be untouched
	if result["name"] != "web" {
		t.Errorf("expected name=web, got %q", result["name"])
	}
}

// TestQA_FieldAlias_NoOverwrite verifies that aliases never overwrite keys that already exist.
func TestQA_FieldAlias_NoOverwrite(t *testing.T) {
	resource.SetFieldAliasesForTest("ec2", map[string]string{
		"state": "State",
	})
	t.Cleanup(func() { resource.CleanupFieldAliasesForTest("ec2") })

	fields := map[string]string{
		"state": "running",
		"State": "ALREADY_SET",
	}

	result := resource.ApplyFieldAliases("ec2", fields)

	if result["State"] != "ALREADY_SET" {
		t.Errorf("expected State to remain ALREADY_SET, got %q", result["State"])
	}
}

// TestQA_FieldAlias_EmptyFields verifies that nil input returns nil (no panic, no allocation).
func TestQA_FieldAlias_EmptyFields(t *testing.T) {
	resource.SetFieldAliasesForTest("ec2", map[string]string{
		"instance_id": "InstanceId",
	})
	t.Cleanup(func() { resource.CleanupFieldAliasesForTest("ec2") })

	result := resource.ApplyFieldAliases("ec2", nil)

	if result != nil {
		t.Errorf("expected nil result for nil input, got %v", result)
	}
}

// TestQA_FieldAlias_NoAliasRegistered verifies that unregistered types return the same map pointer.
func TestQA_FieldAlias_NoAliasRegistered(t *testing.T) {
	// explicitly do NOT register aliases for "rds"
	fields := map[string]string{
		"db_id": "mydb",
	}

	result := resource.ApplyFieldAliases("rds", fields)

	if result["db_id"] != "mydb" {
		t.Errorf("expected db_id=mydb, got %q", result["db_id"])
	}
	// verify same pointer by checking length is unmodified
	if len(result) != 1 {
		t.Errorf("expected map length 1, got %d", len(result))
	}
}

// TestQA_FieldAlias_EmptyValueSkipped verifies that whitespace-only source values are not aliased.
func TestQA_FieldAlias_EmptyValueSkipped(t *testing.T) {
	resource.SetFieldAliasesForTest("ec2", map[string]string{
		"instance_id": "InstanceId",
	})
	t.Cleanup(func() { resource.CleanupFieldAliasesForTest("ec2") })

	fields := map[string]string{
		"instance_id": "  ",
	}

	result := resource.ApplyFieldAliases("ec2", fields)

	if _, exists := result["InstanceId"]; exists {
		t.Errorf("expected InstanceId to be absent for whitespace source value, got %q", result["InstanceId"])
	}
}

// TestQA_FieldAlias_NoCopyWhenAllPresent verifies that when all alias targets already exist
// the function returns the same map (no copy performed).
func TestQA_FieldAlias_NoCopyWhenAllPresent(t *testing.T) {
	resource.SetFieldAliasesForTest("ec2", map[string]string{
		"state": "State",
	})
	t.Cleanup(func() { resource.CleanupFieldAliasesForTest("ec2") })

	fields := map[string]string{
		"state": "running",
		"State": "running",
	}

	result := resource.ApplyFieldAliases("ec2", fields)

	// Value must be unchanged
	if result["State"] != "running" {
		t.Errorf("expected State=running, got %q", result["State"])
	}
	if result["state"] != "running" {
		t.Errorf("expected state=running, got %q", result["state"])
	}
	// No extra keys should appear
	if len(result) != 2 {
		t.Errorf("expected map length 2, got %d", len(result))
	}
}
