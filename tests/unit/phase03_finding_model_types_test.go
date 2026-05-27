// phase03_finding_model_types_test.go — TDD red-light tests for PR-03a-types.
//
// These tests MUST fail to compile until the following are added:
//   - domain.FindingCode, domain.Finding, domain.AttentionDetail, domain.DetailRow
//   - Resource.Findings and Resource.AttentionDetails fields
//   - ResourceTypeDef.LifecycleKey field
//
// Spec: docs/refactor/03-finding-model.md
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestPhase03_FindingStructShape verifies that domain.Finding carries all four
// expected fields and that each field round-trips correctly.
func TestPhase03_FindingStructShape(t *testing.T) {
	f := domain.Finding{
		Code:     "ec2.impaired",
		Phrase:   "impaired",
		Severity: domain.SevBroken,
		Source:   "wave1",
	}
	if f.Code != "ec2.impaired" {
		t.Errorf("Code: got %q, want %q", f.Code, "ec2.impaired")
	}
	if f.Phrase != "impaired" {
		t.Errorf("Phrase: got %q, want %q", f.Phrase, "impaired")
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Severity: got %v, want SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPhase03_FindingCodeIsString verifies that FindingCode is a string
// alias and that string() conversion is lossless.
func TestPhase03_FindingCodeIsString(t *testing.T) {
	if string(domain.FindingCode("foo")) != "foo" {
		t.Errorf("string(FindingCode(%q)) != %q", "foo", "foo")
	}
}

// TestPhase03_AttentionDetailShape verifies that AttentionDetail holds a Rows
// slice and that the first DetailRow exposes Label, Value, and Tier.
func TestPhase03_AttentionDetailShape(t *testing.T) {
	ad := domain.AttentionDetail{
		Rows: []domain.DetailRow{
			{Label: "L", Value: "V", Tier: "!"},
		},
	}
	if len(ad.Rows) != 1 {
		t.Fatalf("len(Rows): got %d, want 1", len(ad.Rows))
	}
	r := ad.Rows[0]
	if r.Label != "L" {
		t.Errorf("Label: got %q, want %q", r.Label, "L")
	}
	if r.Value != "V" {
		t.Errorf("Value: got %q, want %q", r.Value, "V")
	}
	if r.Tier != "!" {
		t.Errorf("Tier: got %q, want %q", r.Tier, "!")
	}
}

// TestPhase03_DetailRowTierZeroValue documents the contract that an omitted
// Tier field defaults to the empty string.
func TestPhase03_DetailRowTierZeroValue(t *testing.T) {
	r := domain.DetailRow{Label: "L", Value: "V"}
	if r.Tier != "" {
		t.Errorf("Tier zero value: got %q, want %q", r.Tier, "")
	}
}

// TestPhase03_ResourceFindingsField verifies that Resource accepts both
// Findings and AttentionDetails and that the stored values are retrievable.
func TestPhase03_ResourceFindingsField(t *testing.T) {
	r := domain.Resource{
		Findings: []domain.Finding{
			{Code: "x", Phrase: "y", Severity: domain.SevWarn, Source: "wave2:ec2"},
		},
		AttentionDetails: map[domain.FindingCode]domain.AttentionDetail{
			"x": {Rows: []domain.DetailRow{{Label: "L", Value: "V"}}},
		},
	}
	if len(r.Findings) != 1 {
		t.Fatalf("len(Findings): got %d, want 1", len(r.Findings))
	}
	if r.Findings[0].Code != "x" {
		t.Errorf("Findings[0].Code: got %q, want %q", r.Findings[0].Code, "x")
	}
	if len(r.AttentionDetails) != 1 {
		t.Fatalf("len(AttentionDetails): got %d, want 1", len(r.AttentionDetails))
	}
	if r.AttentionDetails["x"].Rows[0].Label != "L" {
		t.Errorf("AttentionDetails[x].Rows[0].Label: got %q, want %q",
			r.AttentionDetails["x"].Rows[0].Label, "L")
	}
}

// TestPhase03_ResourceCoreFieldsPresent verifies the post-W1.4b.3 surface of
// domain.Resource — Status and Issues were removed in AS-1428 W1.4b.3; the
// remaining fields (ID, Name, Type, Fields, RawStruct, Findings,
// AttentionDetails) must continue to round-trip.
func TestPhase03_ResourceCoreFieldsPresent(t *testing.T) {
	r := domain.Resource{
		ID:        "i-abc123",
		Name:      "my-instance",
		Type:      "ec2",
		Fields:    map[string]string{"az": "us-east-1a"},
		RawStruct: struct{}{},
	}
	if r.ID != "i-abc123" {
		t.Errorf("ID: got %q, want %q", r.ID, "i-abc123")
	}
	if r.Name != "my-instance" {
		t.Errorf("Name: got %q, want %q", r.Name, "my-instance")
	}
	if r.Type != "ec2" {
		t.Errorf("Type: got %q, want %q", r.Type, "ec2")
	}
	if r.Fields["az"] != "us-east-1a" {
		t.Errorf("Fields[az]: got %q, want %q", r.Fields["az"], "us-east-1a")
	}
	if r.RawStruct == nil {
		t.Error("RawStruct: got nil, want non-nil")
	}
}

// TestPhase03_ResourceTypeDefLifecycleKey verifies that ResourceTypeDef
// accepts and returns the LifecycleKey field.
func TestPhase03_ResourceTypeDefLifecycleKey(t *testing.T) {
	td := resource.ResourceTypeDef{ShortName: "ec2", LifecycleKey: "state"}
	if td.LifecycleKey != "state" {
		t.Errorf("LifecycleKey: got %q, want %q", td.LifecycleKey, "state")
	}
}

// TestPhase03_LifecycleKeyZeroValueIsEmpty verifies that LifecycleKey defaults
// to the empty string when not set.
func TestPhase03_LifecycleKeyZeroValueIsEmpty(t *testing.T) {
	td := resource.ResourceTypeDef{}
	if td.LifecycleKey != "" {
		t.Errorf("LifecycleKey zero value: got %q, want %q", td.LifecycleKey, "")
	}
}
