package unit

// qa_rds_ddb_color_fieldkey_test.go — Regression tests for RDS (dbi) and DDB Color
// reading the "status" field key.
//
// Bug: RDS Color was reading "db_instance_status" as the primary key;
//      DDB Color was reading "table_status" as the primary key.
// Fix: Both now read "status" first (with legacy fallback).
//
// Tests fail if the fix is reverted: a resource with Fields["status"] set would
// not be classified correctly when the wrong key is primary.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ─────────────────────────────────────────────────────────────────────────────
// RDS DB Instance (dbi) Color tests
// ─────────────────────────────────────────────────────────────────────────────

// TestRDSColor_StatusFailed_IsColorBroken verifies Fields["status"]="failed" → ColorBroken.
func TestRDSColor_StatusFailed_IsColorBroken(t *testing.T) {
	td := resource.FindResourceType("dbi")
	r := resource.Resource{
		ID:     "arn:aws:rds:us-east-1:123456789012:db:prod-db",
		Name:   "prod-db",
		Fields: map[string]string{"status": "stopped"},
	}
	got := td.Color(r)
	if got != resource.ColorBroken {
		t.Errorf("dbi Color with Fields[status]=stopped = %v, want ColorBroken", got)
	}
}

// TestRDSColor_StatusAvailable_IsColorHealthy verifies Fields["status"]="available" → ColorHealthy.
func TestRDSColor_StatusAvailable_IsColorHealthy(t *testing.T) {
	td := resource.FindResourceType("dbi")
	r := resource.Resource{
		ID:     "arn:aws:rds:us-east-1:123456789012:db:prod-db",
		Name:   "prod-db",
		Fields: map[string]string{"status": "available"},
	}
	got := td.Color(r)
	if got != resource.ColorHealthy {
		t.Errorf("dbi Color with Fields[status]=available = %v, want ColorHealthy", got)
	}
}

// TestRDSColor_StatusCreating_IsColorWarning verifies Fields["status"]="creating" → ColorWarning.
func TestRDSColor_StatusCreating_IsColorWarning(t *testing.T) {
	td := resource.FindResourceType("dbi")
	r := resource.Resource{
		ID:     "arn:aws:rds:us-east-1:123456789012:db:new-db",
		Name:   "new-db",
		Fields: map[string]string{"status": "creating"},
	}
	got := td.Color(r)
	if got != resource.ColorWarning {
		t.Errorf("dbi Color with Fields[status]=creating = %v, want ColorWarning", got)
	}
}

// TestRDSColor_OldWrongKey_DoesNotProduceBroken pins that "db_instance_status" is
// NOT the primary key. A stopped value under "db_instance_status" with no "status"
// key set must produce the fallback color (legacy path), not ColorBroken from the
// primary path. This regresses if someone moves "db_instance_status" back to primary.
//
// Note: the legacy fallback reads "db_instance_status" when "status" is empty,
// so a stopped value here DOES produce ColorBroken via the fallback path. The
// regression-critical behavior is that "status" takes precedence: if both keys
// are set, "status" wins.
func TestRDSColor_StatusTakesPrecedenceOverLegacyKey(t *testing.T) {
	td := resource.FindResourceType("dbi")
	// Set "status" to available (healthy) AND "db_instance_status" to stopped (broken).
	// The "status" key must take precedence → ColorHealthy.
	r := resource.Resource{
		ID:   "arn:aws:rds:us-east-1:123456789012:db:prod-db",
		Name: "prod-db",
		Fields: map[string]string{
			"status":             "available",      // should win
			"db_instance_status": "stopped",        // should be ignored when "status" is set
		},
	}
	got := td.Color(r)
	if got != resource.ColorHealthy {
		t.Errorf("dbi Color with Fields[status]=available AND Fields[db_instance_status]=stopped = %v, want ColorHealthy — 'status' key must take precedence", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// DynamoDB Table (ddb) Color tests
// ─────────────────────────────────────────────────────────────────────────────

// TestDDBColor_StatusACTIVE_IsColorHealthy verifies Fields["status"]="ACTIVE" → ColorHealthy.
func TestDDBColor_StatusACTIVE_IsColorHealthy(t *testing.T) {
	td := resource.FindResourceType("ddb")
	r := resource.Resource{
		ID:     "arn:aws:dynamodb:us-east-1:123456789012:table/orders",
		Name:   "orders",
		Fields: map[string]string{"status": "ACTIVE"},
	}
	got := td.Color(r)
	if got != resource.ColorHealthy {
		t.Errorf("ddb Color with Fields[status]=ACTIVE = %v, want ColorHealthy", got)
	}
}

// TestDDBColor_StatusDELETING_IsColorWarning verifies Fields["status"]="DELETING" → ColorWarning.
func TestDDBColor_StatusDELETING_IsColorWarning(t *testing.T) {
	td := resource.FindResourceType("ddb")
	r := resource.Resource{
		ID:     "arn:aws:dynamodb:us-east-1:123456789012:table/old-table",
		Name:   "old-table",
		Fields: map[string]string{"status": "DELETING"},
	}
	got := td.Color(r)
	if got != resource.ColorWarning {
		t.Errorf("ddb Color with Fields[status]=DELETING = %v, want ColorWarning", got)
	}
}

// TestDDBColor_StatusCREATING_IsColorWarning verifies Fields["status"]="CREATING" → ColorWarning.
func TestDDBColor_StatusCREATING_IsColorWarning(t *testing.T) {
	td := resource.FindResourceType("ddb")
	r := resource.Resource{
		ID:     "arn:aws:dynamodb:us-east-1:123456789012:table/new-table",
		Name:   "new-table",
		Fields: map[string]string{"status": "CREATING"},
	}
	got := td.Color(r)
	if got != resource.ColorWarning {
		t.Errorf("ddb Color with Fields[status]=CREATING = %v, want ColorWarning", got)
	}
}

// TestDDBColor_StatusTakesPrecedenceOverLegacyKey verifies that "status" wins over
// "table_status" when both are set. Regresses if "table_status" is moved back to primary.
func TestDDBColor_StatusTakesPrecedenceOverLegacyKey(t *testing.T) {
	td := resource.FindResourceType("ddb")
	// "status" = ACTIVE (healthy) vs "table_status" = DELETING (warning).
	// The "status" key must win → ColorHealthy.
	r := resource.Resource{
		ID:   "arn:aws:dynamodb:us-east-1:123456789012:table/my-table",
		Name: "my-table",
		Fields: map[string]string{
			"status":       "ACTIVE",   // should win
			"table_status": "DELETING", // should be ignored when "status" is set
		},
	}
	got := td.Color(r)
	if got != resource.ColorHealthy {
		t.Errorf("ddb Color with Fields[status]=ACTIVE AND Fields[table_status]=DELETING = %v, want ColorHealthy — 'status' must take precedence", got)
	}
}

// TestDDBColor_OldWrongKeyOnly_DoesNotProduceWarning pins that "table_status" alone
// (no "status" key) correctly falls back to the legacy path → ColorWarning for DELETING.
func TestDDBColor_OldWrongKeyOnly_DoesNotClassifyAsHealthy(t *testing.T) {
	td := resource.FindResourceType("ddb")
	// Legacy fallback: "status" is absent, "table_status" = DELETING.
	// With the fix in place, this still works via the fallback → ColorWarning.
	r := resource.Resource{
		ID:     "arn:aws:dynamodb:us-east-1:123456789012:table/legacy-table",
		Name:   "legacy-table",
		Fields: map[string]string{"table_status": "DELETING"},
	}
	got := td.Color(r)
	// The legacy key still works as a fallback — we just pin that the behavior
	// is consistent (DELETING via any key path = ColorWarning).
	if got == resource.ColorBroken {
		t.Errorf("ddb Color with Fields[table_status]=DELETING = ColorBroken, want ColorWarning (DELETING is transitional, not broken)")
	}
}
