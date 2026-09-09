package unit_test

// w27_related_fetch_partial_failure_test.go — task w27, dev round 2's
// production fix: FetchRelatedTarget (core/aws/related_fetch.go:47-54) used
// to discard every row a fetcher returned whenever it also carried a
// per-item error, so one denied node group corrupted the eks Auto Scaling
// pivot for every cluster sharing the "ng" cache — exactly the outcome
// task w27's finding-1 demo witnesses would have triggered widely. Rows
// beside an error are now a truncated answer, not a dropped one.

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	unit "github.com/k2m30/a9s/v3/tests/unit"
)

// TestFetchRelatedTarget_RowsBesideError_YieldsTruncatedRows pins the fix
// directly: a paginated fetcher returning rows AND an error must hand back
// those rows with the truncation flag set and no error, not nil-with-error.
func TestFetchRelatedTarget_RowsBesideError_YieldsTruncatedRows(t *testing.T) {
	const target = "test-partial-failure-target"
	partialErr := context.DeadlineExceeded
	resource.SetPaginatedForTest(target, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources: []resource.Resource{{ID: "kept-row-1"}, {ID: "kept-row-2"}},
		}, partialErr
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest(target) })

	resources, truncated, err := awsclient.FetchRelatedTarget(context.Background(), nil, resource.ResourceCache{}, target)

	if err != nil {
		t.Errorf("err = %v, want nil (a per-item failure beside rows is a truncated answer, not an error)", err)
	}
	if !truncated {
		t.Error("truncated = false, want true")
	}
	if len(resources) != 2 {
		t.Fatalf("resources = %v, want the 2 rows the fetcher returned kept, not dropped", resources)
	}
}

// TestFetchRelatedTarget_RowsLessErrorOnly_StaysAnError is the control: a
// fetcher returning NO rows and an error must still surface the error — only
// rows-beside-an-error becomes a truncated answer.
func TestFetchRelatedTarget_RowsLessErrorOnly_StaysAnError(t *testing.T) {
	const target = "test-total-failure-target"
	wantErr := context.DeadlineExceeded
	resource.SetPaginatedForTest(target, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{}, wantErr
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest(target) })

	resources, truncated, err := awsclient.FetchRelatedTarget(context.Background(), nil, resource.ResourceCache{}, target)

	if err != wantErr {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
	if truncated {
		t.Error("truncated = true, want false on a total failure")
	}
	if resources != nil {
		t.Errorf("resources = %v, want nil on a total failure", resources)
	}
}

// TestEKSASGPivot_DeniedNodeGroupOnOneCluster_DoesNotCorruptSiblingCluster
// pins the real consumer this fix repairs: acme-prod carries a node group
// DescribeNodegroup denies (task w27 finding-1's witness), and acme-staging
// shares the same "ng" fetch/cache but has no denial at all. Before the fix,
// the denied node group's error propagated through the shared cache and
// every cluster's Auto Scaling pivot rendered ErrorRelated. After it,
// acme-staging resolves cleanly and acme-prod resolves with its own
// (truncated) answer rather than an error.
func TestEKSASGPivot_DeniedNodeGroupOnOneCluster_DoesNotCorruptSiblingCluster(t *testing.T) {
	clients := demo.NewServiceClients()

	eksTD := catalog.FindAny("eks")
	if eksTD == nil {
		t.Fatal("no catalog entry for eks")
	}
	clusters, ok := unit.DrainFixtures(t, *eksTD, clients)
	if !ok {
		t.Fatal("eks has no Fetcher")
	}

	// Populate the "ng" cache exactly as a real render pass would: one shared
	// fetch, reused by every cluster's Auto Scaling pivot.
	ngResources, ngTruncated, err := awsclient.FetchRelatedTarget(context.Background(), clients, resource.ResourceCache{}, "ng")
	if err != nil {
		t.Fatalf("FetchRelatedTarget(ng) unexpected error: %v", err)
	}
	if !ngTruncated {
		t.Fatal("FetchRelatedTarget(ng) truncated = false, want true — the demo fixtures plant a denied node group on acme-prod")
	}
	cache := resource.ResourceCache{"ng": {Resources: ngResources, IsTruncated: ngTruncated}}

	checker := eksCheckerByTarget(t, "asg")

	prod := mustFindEKSCluster(t, clusters, "acme-prod")
	staging := mustFindEKSCluster(t, clusters, "acme-staging")

	prodResult := checker(context.Background(), clients, prod, cache)
	if prodResult.State() == domain.RelatedError {
		t.Errorf("acme-prod (has the denied node group) asg pivot = RelatedError, want it to resolve from the node groups that did describe")
	}

	stagingResult := checker(context.Background(), clients, staging, cache)
	if stagingResult.State() == domain.RelatedError {
		t.Errorf("acme-staging (no denial at all) asg pivot = RelatedError — the denied node group on a DIFFERENT cluster must not corrupt this one")
	}
}

func mustFindEKSCluster(t *testing.T, clusters []resource.Resource, id string) resource.Resource {
	t.Helper()
	for _, c := range clusters {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no demo eks cluster %q", id)
	return resource.Resource{}
}
