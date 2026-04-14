package unit

// qa_review_fix_enricher_truncated_test.go — Tests for EnricherFunc signature
// and EnrichmentCap constant behavior.
//
// Updated for the EnricherResult return type:
//   EnricherFunc = func(ctx, clients, resources) (EnricherResult, error)

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestEnricherFuncSignatureReturnsResult verifies that EnricherFunc accepts
// functions with the new signature (ctx, clients, resources) → (EnricherResult, error).
func TestEnricherFuncSignatureReturnsResult(t *testing.T) {
	fn := awsclient.EnricherFunc(func(
		_ context.Context,
		_ *awsclient.ServiceClients,
		_ []resource.Resource,
	) (awsclient.EnricherResult, error) {
		return awsclient.EnricherResult{
			IssueCount: 3,
			Truncated:  true,
			Findings:   make(map[string]resource.EnrichmentFinding),
		}, nil
	})

	result, err := fn(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error from test enricher: %v", err)
	}
	if result.IssueCount != 3 {
		t.Errorf("IssueCount = %d, want 3", result.IssueCount)
	}
	if !result.Truncated {
		t.Error("Truncated = false, want true")
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil on success")
	}
}

// TestEnricherFuncSignatureReturnsFalseWhenNotTruncated verifies that an enricher
// can return Truncated=false when results are not capped.
func TestEnricherFuncSignatureReturnsFalseWhenNotTruncated(t *testing.T) {
	fn := awsclient.EnricherFunc(func(
		_ context.Context,
		_ *awsclient.ServiceClients,
		_ []resource.Resource,
	) (awsclient.EnricherResult, error) {
		return awsclient.EnricherResult{
			IssueCount: 0,
			Truncated:  false,
			Findings:   make(map[string]resource.EnrichmentFinding),
		}, nil
	})

	result, err := fn(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
	if result.Truncated {
		t.Error("Truncated = true, want false")
	}
}

// TestEnrichmentCapValue verifies EnrichmentCap is a positive non-zero constant.
// The exact value (50) is documented in the fix; changes should be intentional.
func TestEnrichmentCapValue(t *testing.T) {
	if awsclient.EnrichmentCap <= 0 {
		t.Errorf("EnrichmentCap must be positive, got %d", awsclient.EnrichmentCap)
	}
	if awsclient.EnrichmentCap != 50 {
		t.Errorf("EnrichmentCap expected=50, got=%d (if changed intentionally, update this test)", awsclient.EnrichmentCap)
	}
}

// TestEnrichmentCapTruncation verifies that per-resource enrichers report
// Truncated=true when the resource slice exceeds EnrichmentCap.
// This test uses a synthetic enricher that mirrors the pattern used by the
// real per-resource enrichers in enrichment.go.
func TestEnrichmentCapTruncation(t *testing.T) {
	fn := awsclient.EnricherFunc(func(
		_ context.Context,
		_ *awsclient.ServiceClients,
		resources []resource.Resource,
	) (awsclient.EnricherResult, error) {
		return awsclient.EnricherResult{
			IssueCount: 0,
			Truncated:  len(resources) > awsclient.EnrichmentCap,
			Findings:   make(map[string]resource.EnrichmentFinding),
		}, nil
	})

	tests := []struct {
		name      string
		count     int
		wantTrunc bool
	}{
		{"below cap", awsclient.EnrichmentCap - 1, false},
		{"at cap", awsclient.EnrichmentCap, false},
		{"above cap", awsclient.EnrichmentCap + 1, true},
		{"well above cap", awsclient.EnrichmentCap * 2, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resources := make([]resource.Resource, tc.count)
			result, err := fn(context.Background(), nil, resources)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Truncated != tc.wantTrunc {
				t.Errorf("len(resources)=%d: want Truncated=%v, got %v",
					tc.count, tc.wantTrunc, result.Truncated)
			}
		})
	}
}
