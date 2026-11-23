package unit

// The IssueEnricherFunc signature,
// func(ctx, clients, resources, cache) (IssueEnricherResult, error), and
// EnrichmentCap behaviour.

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// IssueEnricherFunc accepts func(ctx, clients, resources, cache) →
// (IssueEnricherResult, error).
func TestIssueEnricherFuncSignatureReturnsResult(t *testing.T) {
	fn := awsclient.IssueEnricherFunc(func(
		_ context.Context,
		_ *awsclient.ServiceClients,
		_ []resource.Resource,
		_ resource.ResourceCache,
	) (awsclient.IssueEnricherResult, error) {
		return awsclient.IssueEnricherResult{
			Findings: make(map[string][]domain.Finding),
		}, nil
	})

	result, err := fn(context.Background(), nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error from test enricher: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil on success")
	}
}

// TestEnrichmentCapValue verifies EnrichmentCap is a positive non-zero constant.
func TestEnrichmentCapValue(t *testing.T) {
	if awsclient.EnrichmentCap <= 0 {
		t.Errorf("EnrichmentCap must be positive, got %d", awsclient.EnrichmentCap)
	}
}
