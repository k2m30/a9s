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

// TestRDSColor_LegacyKeyIsIgnored pins that the legacy "db_instance_status"
// field name is ignored by the Color func (#284 removed the fallback).
// Only the canonical "status" key drives classification.
func TestRDSColor_LegacyKeyIsIgnored(t *testing.T) {
	td := resource.FindResourceType("dbi")
	// "status" says available; "db_instance_status" says stopped — legacy key
	// must be ignored, so the result is ColorHealthy.
	r := resource.Resource{
		ID:   "arn:aws:rds:us-east-1:123456789012:db:prod-db",
		Name: "prod-db",
		Fields: map[string]string{
			"status":             "available", // canonical — wins
			"db_instance_status": "stopped",   // legacy — ignored
		},
	}
	got := td.Color(r)
	if got != resource.ColorHealthy {
		t.Errorf("dbi Color with Fields[status]=available AND Fields[db_instance_status]=stopped = %v, want ColorHealthy — legacy key must be ignored", got)
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

// TestDDBColor_LegacyTableStatusIsIgnored pins that the legacy "table_status"
// key is ignored by the ddb Color func (#284 removed the fallback). Only the
// canonical "status" key drives classification.
func TestDDBColor_LegacyTableStatusIsIgnored(t *testing.T) {
	td := resource.FindResourceType("ddb")
	// "status" = ACTIVE (healthy); "table_status" = DELETING (warning under
	// the old fallback). Legacy key must be ignored → ColorHealthy.
	r := resource.Resource{
		ID:   "arn:aws:dynamodb:us-east-1:123456789012:table/my-table",
		Name: "my-table",
		Fields: map[string]string{
			"status":       "ACTIVE",   // canonical — wins
			"table_status": "DELETING", // legacy — ignored
		},
	}
	got := td.Color(r)
	if got != resource.ColorHealthy {
		t.Errorf("ddb Color with Fields[status]=ACTIVE AND Fields[table_status]=DELETING = %v, want ColorHealthy — legacy key must be ignored", got)
	}
}
