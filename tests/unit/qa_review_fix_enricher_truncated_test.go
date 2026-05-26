package unit

// qa_review_fix_enricher_truncated_test.go — Tests for IssueEnricherFunc signature
// and EnrichmentCap constant behavior.
//
// Updated for the IssueEnricherResult return type:
//   IssueEnricherFunc = func(ctx, clients, resources, cache) (IssueEnricherResult, error)

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestIssueEnricherFuncSignatureReturnsResult verifies that IssueEnricherFunc accepts
// functions with the new signature (ctx, clients, resources, cache) → (IssueEnricherResult, error).
func TestIssueEnricherFuncSignatureReturnsResult(t *testing.T) {
	fn := awsclient.IssueEnricherFunc(func(
		_ context.Context,
		_ *awsclient.ServiceClients,
		_ []resource.Resource,
		_ resource.ResourceCache,
	) (awsclient.IssueEnricherResult, error) {
		return awsclient.IssueEnricherResult{
			IssueCount: 3,
			Truncated:  true,
			Findings:   make(map[string]domain.Finding),
		}, nil
	})

	result, err := fn(context.Background(), nil, nil, nil)
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

// TestIssueEnricherFuncSignatureReturnsFalseWhenNotTruncated verifies that an enricher
// can return Truncated=false when results are not capped.
func TestIssueEnricherFuncSignatureReturnsFalseWhenNotTruncated(t *testing.T) {
	fn := awsclient.IssueEnricherFunc(func(
		_ context.Context,
		_ *awsclient.ServiceClients,
		_ []resource.Resource,
		_ resource.ResourceCache,
	) (awsclient.IssueEnricherResult, error) {
		return awsclient.IssueEnricherResult{
			IssueCount: 0,
			Truncated:  false,
			Findings:   make(map[string]domain.Finding),
		}, nil
	})

	result, err := fn(context.Background(), nil, nil, nil)
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
func TestEnrichmentCapValue(t *testing.T) {
	if awsclient.EnrichmentCap <= 0 {
		t.Errorf("EnrichmentCap must be positive, got %d", awsclient.EnrichmentCap)
	}
}

// TestEnrichmentCapTruncation verifies that per-resource enrichers report
// Truncated=true when the resource slice exceeds EnrichmentCap.
// This test uses a synthetic enricher that mirrors the pattern used by the
// real per-resource enrichers in *_issue_enrichment.go.
func TestEnrichmentCapTruncation(t *testing.T) {
	fn := awsclient.IssueEnricherFunc(func(
		_ context.Context,
		_ *awsclient.ServiceClients,
		resources []resource.Resource,
		_ resource.ResourceCache,
	) (awsclient.IssueEnricherResult, error) {
		return awsclient.IssueEnricherResult{
			IssueCount: 0,
			Truncated:  len(resources) > awsclient.EnrichmentCap,
			Findings:   make(map[string]domain.Finding),
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
			result, err := fn(context.Background(), nil, resources, nil)
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
