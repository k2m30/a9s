package unit

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestEnricherFuncSignatureReturnsTruncated verifies that EnricherFunc accepts
// functions with the signature (ctx, clients, resources) → (int, bool, error)
// and that the bool (truncated flag) can be returned.
// This is the fix from issue #196 bug 5: signature changed to include the truncated bool.
func TestEnricherFuncSignatureReturnsTruncated(t *testing.T) {
	// A minimal enricher that always reports 3 issues and truncated=true.
	fn := awsclient.EnricherFunc(func(
		_ context.Context,
		_ *awsclient.ServiceClients,
		_ []resource.Resource,
	) (int, bool, error) {
		return 3, true, nil
	})

	count, truncated, err := fn(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error from test enricher: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count=3, got %d", count)
	}
	if !truncated {
		t.Error("expected truncated=true, got false")
	}
}

// TestEnricherFuncSignatureReturnsFalseWhenNotTruncated verifies that an enricher
// can return truncated=false when results are not capped.
func TestEnricherFuncSignatureReturnsFalseWhenNotTruncated(t *testing.T) {
	fn := awsclient.EnricherFunc(func(
		_ context.Context,
		_ *awsclient.ServiceClients,
		_ []resource.Resource,
	) (int, bool, error) {
		return 0, false, nil
	})

	count, truncated, err := fn(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count=0, got %d", count)
	}
	if truncated {
		t.Error("expected truncated=false, got true")
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
// truncated=true when the resource slice exceeds EnrichmentCap.
// This test uses a synthetic enricher that mirrors the pattern used by the
// real per-resource enrichers in enrichment.go.
func TestEnrichmentCapTruncation(t *testing.T) {
	// Simulate the per-resource enricher cap check:
	//   return issues, len(resources) > EnrichmentCap, nil
	fn := awsclient.EnricherFunc(func(
		_ context.Context,
		_ *awsclient.ServiceClients,
		resources []resource.Resource,
	) (int, bool, error) {
		return 0, len(resources) > awsclient.EnrichmentCap, nil
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
			_, truncated, err := fn(context.Background(), nil, resources)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if truncated != tc.wantTrunc {
				t.Errorf("len(resources)=%d: want truncated=%v, got %v",
					tc.count, tc.wantTrunc, truncated)
			}
		})
	}
}
