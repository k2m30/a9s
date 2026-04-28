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
		ID:     "arn:aws:rds:us-east-1:000000000000:db:prod-db",
		Name:   "prod-db",
		Fields: map[string]string{"status": "stopped"},
	}
	got := td.Color(r)
	// stopped is intentional admin action, not failure — Warning, mirrors EC2 stopped.
	if got != resource.ColorWarning {
		t.Errorf("dbi Color with Fields[status]=stopped = %v, want ColorWarning", got)
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
// DynamoDB Table (ddb) Color tests — §4 phrases (lowercase, not raw AWS enums)
// ─────────────────────────────────────────────────────────────────────────────

// TestDDBColor_StatusBlank_IsColorHealthy verifies Fields["status"]="" (ACTIVE→blank) → ColorHealthy.
func TestDDBColor_StatusBlank_IsColorHealthy(t *testing.T) {
	td := resource.FindResourceType("ddb")
	r := resource.Resource{
		ID:     "arn:aws:dynamodb:us-east-1:123456789012:table/orders",
		Name:   "orders",
		Fields: map[string]string{"status": ""},
	}
	got := td.Color(r)
	if got != resource.ColorHealthy {
		t.Errorf("ddb Color with Fields[status]='' = %v, want ColorHealthy (ACTIVE maps to blank phrase)", got)
	}
}

// TestDDBColor_StatusDeleting_IsColorWarning verifies Fields["status"]="deleting" → ColorWarning.
func TestDDBColor_StatusDeleting_IsColorWarning(t *testing.T) {
	td := resource.FindResourceType("ddb")
	r := resource.Resource{
		ID:     "arn:aws:dynamodb:us-east-1:123456789012:table/old-table",
		Name:   "old-table",
		Fields: map[string]string{"status": "deleting"},
	}
	got := td.Color(r)
	if got != resource.ColorWarning {
		t.Errorf("ddb Color with Fields[status]=deleting = %v, want ColorWarning", got)
	}
}

// TestDDBColor_StatusCreating_IsColorWarning verifies Fields["status"]="creating" → ColorWarning.
func TestDDBColor_StatusCreating_IsColorWarning(t *testing.T) {
	td := resource.FindResourceType("ddb")
	r := resource.Resource{
		ID:     "arn:aws:dynamodb:us-east-1:123456789012:table/new-table",
		Name:   "new-table",
		Fields: map[string]string{"status": "creating"},
	}
	got := td.Color(r)
	if got != resource.ColorWarning {
		t.Errorf("ddb Color with Fields[status]=creating = %v, want ColorWarning", got)
	}
}

// TestDDBColor_StatusUpdating_IsColorWarning verifies Fields["status"]="updating" → ColorWarning.
func TestDDBColor_StatusUpdating_IsColorWarning(t *testing.T) {
	td := resource.FindResourceType("ddb")
	r := resource.Resource{
		ID:     "arn:aws:dynamodb:us-east-1:123456789012:table/sessions-updating",
		Name:   "sessions-updating",
		Fields: map[string]string{"status": "updating"},
	}
	got := td.Color(r)
	if got != resource.ColorWarning {
		t.Errorf("ddb Color with Fields[status]=updating = %v, want ColorWarning", got)
	}
}

// TestDDBColor_StatusArchiving_IsColorWarning verifies Fields["status"]="archiving" → ColorWarning.
func TestDDBColor_StatusArchiving_IsColorWarning(t *testing.T) {
	td := resource.FindResourceType("ddb")
	r := resource.Resource{
		ID:     "arn:aws:dynamodb:us-east-1:123456789012:table/legacy-archiving",
		Name:   "legacy-archiving",
		Fields: map[string]string{"status": "archiving"},
	}
	got := td.Color(r)
	if got != resource.ColorWarning {
		t.Errorf("ddb Color with Fields[status]=archiving = %v, want ColorWarning", got)
	}
}

// TestDDBColor_StatusKMSInaccessible_IsColorBroken verifies
// Fields["status"]="kms key inaccessible" → ColorBroken.
func TestDDBColor_StatusKMSInaccessible_IsColorBroken(t *testing.T) {
	td := resource.FindResourceType("ddb")
	r := resource.Resource{
		ID:     "arn:aws:dynamodb:us-east-1:123456789012:table/legacy-kms-lost",
		Name:   "legacy-kms-lost",
		Fields: map[string]string{"status": "kms key inaccessible"},
	}
	got := td.Color(r)
	if got != resource.ColorBroken {
		t.Errorf("ddb Color with Fields[status]='kms key inaccessible' = %v, want ColorBroken", got)
	}
}

// TestDDBColor_StatusArchivedKMSLost_IsColorBroken verifies
// Fields["status"]="archived: kms key lost" → ColorBroken.
func TestDDBColor_StatusArchivedKMSLost_IsColorBroken(t *testing.T) {
	td := resource.FindResourceType("ddb")
	r := resource.Resource{
		ID:     "arn:aws:dynamodb:us-east-1:123456789012:table/legacy-archived",
		Name:   "legacy-archived",
		Fields: map[string]string{"status": "archived: kms key lost"},
	}
	got := td.Color(r)
	if got != resource.ColorBroken {
		t.Errorf("ddb Color with Fields[status]='archived: kms key lost' = %v, want ColorBroken", got)
	}
}

// TestDDBColor_StatusPITROff_IsColorHealthy verifies that the Wave-2 enrichment
// finding summary "PITR off" (~ severity) does not degrade Color: ColorHealthy.
// The Color func reads Fields["status"], not the enrichment summary, so a table
// with ACTIVE status and a PITR-off finding must remain green.
func TestDDBColor_StatusPITROff_IsColorHealthy(t *testing.T) {
	td := resource.FindResourceType("ddb")
	r := resource.Resource{
		ID:     "arn:aws:dynamodb:us-east-1:123456789012:table/audit-pitr-off",
		Name:   "audit-pitr-off",
		Fields: map[string]string{"status": ""},
	}
	got := td.Color(r)
	if got != resource.ColorHealthy {
		t.Errorf("ddb Color for PITR-off table (ACTIVE status, blank phrase) = %v, want ColorHealthy — Wave-2 ~ finding must not affect Color", got)
	}
}

// TestDDBColor_ArchivedKMSLostWithSuffix_StripBeforeColor verifies that a status
// value that carries a BumpFindingSuffix (e.g. "archived: kms key lost (+1)")
// is still correctly classified as ColorBroken. The Color func must call
// StripFindingSuffix before lookup.
func TestDDBColor_ArchivedKMSLostWithSuffix_StripBeforeColor(t *testing.T) {
	td := resource.FindResourceType("ddb")
	r := resource.Resource{
		ID:     "arn:aws:dynamodb:us-east-1:123456789012:table/legacy-archived-plus",
		Name:   "legacy-archived-plus",
		Fields: map[string]string{"status": "archived: kms key lost (+1)"},
	}
	got := td.Color(r)
	if got != resource.ColorBroken {
		t.Errorf("ddb Color with Fields[status]='archived: kms key lost (+1)' = %v, want ColorBroken — StripFindingSuffix must be applied before color lookup", got)
	}
}

// TestDDBColor_LegacyTableStatusIsIgnored pins that the legacy "table_status"
// key is ignored by the ddb Color func. Only the canonical "status" key drives
// classification. Uses §4 phrases: blank for ACTIVE, "deleting" for DELETING.
func TestDDBColor_LegacyTableStatusIsIgnored(t *testing.T) {
	td := resource.FindResourceType("ddb")
	// "status" = "" (ACTIVE phrase → healthy); "table_status" = "deleting"
	// (warning under the old fallback). Legacy key must be ignored → ColorHealthy.
	r := resource.Resource{
		ID:   "arn:aws:dynamodb:us-east-1:123456789012:table/my-table",
		Name: "my-table",
		Fields: map[string]string{
			"status":       "",         // canonical ACTIVE phrase — wins
			"table_status": "deleting", // legacy — ignored
		},
	}
	got := td.Color(r)
	if got != resource.ColorHealthy {
		t.Errorf("ddb Color with Fields[status]='' AND Fields[table_status]=deleting = %v, want ColorHealthy — legacy key must be ignored", got)
	}
}
