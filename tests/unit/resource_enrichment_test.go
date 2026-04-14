package unit

// Tests for internal/resource/enrichment.go
// Verifies: struct shape, zero-value behavior, usability as map value.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestEnrichmentFinding_FieldValues(t *testing.T) {
	f := resource.EnrichmentFinding{
		Severity: "!",
		Summary:  "db storage full",
	}
	if f.Severity != "!" {
		t.Errorf("Severity = %q, want %q", f.Severity, "!")
	}
	if f.Summary != "db storage full" {
		t.Errorf("Summary = %q, want %q", f.Summary, "db storage full")
	}
}

func TestEnrichmentFinding_ZeroValue(t *testing.T) {
	var f resource.EnrichmentFinding
	if f.Severity != "" {
		t.Errorf("zero-value Severity = %q, want empty string", f.Severity)
	}
	if f.Summary != "" {
		t.Errorf("zero-value Summary = %q, want empty string", f.Summary)
	}
}

func TestEnrichmentFinding_UsableAsMapValue(t *testing.T) {
	m := map[string]resource.EnrichmentFinding{
		"i-0aaa111111111111a": {Severity: "!", Summary: "instance impaired"},
		"i-0bbb222222222222b": {Severity: "~", Summary: "pending maintenance"},
	}

	f1, ok := m["i-0aaa111111111111a"]
	if !ok {
		t.Fatal("lookup for i-0aaa111111111111a failed")
	}
	if f1.Severity != "!" {
		t.Errorf("Severity = %q, want %q", f1.Severity, "!")
	}
	if f1.Summary != "instance impaired" {
		t.Errorf("Summary = %q, want %q", f1.Summary, "instance impaired")
	}

	f2, ok := m["i-0bbb222222222222b"]
	if !ok {
		t.Fatal("lookup for i-0bbb222222222222b failed")
	}
	if f2.Severity != "~" {
		t.Errorf("Severity = %q, want %q", f2.Severity, "~")
	}
	if f2.Summary != "pending maintenance" {
		t.Errorf("Summary = %q, want %q", f2.Summary, "pending maintenance")
	}
}

func TestEnrichmentFinding_SeverityValues(t *testing.T) {
	// "!" = broken/degraded (contributes to menu badge)
	// "~" = scheduled/informational (excluded from menu badge)
	cases := []struct {
		severity string
		summary  string
	}{
		{"!", "latest build FAILED (2026-04-13)"},
		{"~", "pending maintenance: system-update"},
		{"", ""},
	}
	for _, tc := range cases {
		f := resource.EnrichmentFinding{Severity: tc.severity, Summary: tc.summary}
		if f.Severity != tc.severity {
			t.Errorf("Severity = %q, want %q", f.Severity, tc.severity)
		}
		if f.Summary != tc.summary {
			t.Errorf("Summary = %q, want %q", f.Summary, tc.summary)
		}
	}
}
