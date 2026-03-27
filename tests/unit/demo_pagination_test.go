package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/demo"
)

// ═══════════════════════════════════════════════════════════════════════════
// Demo pagination tests — written BEFORE implementation (TDD).
// These verify that demo mode returns paginated results with DemoPageSize=5.
// All tests should FAIL against stub implementations.
// ═══════════════════════════════════════════════════════════════════════════

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// smallTypes have ≤5 demo items — pagination should return all without truncation.
var smallTypes = []string{"iam-group", "waf", "eip", "iam-user", "eks"}

// largeTypes have >5 demo items — pagination should truncate at DemoPageSize.
var largeTypes = []string{"ec2", "lambda", "role", "policy", "sg", "subnet", "s3", "dbi"}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// allCount returns the total demo item count for a resource type using GetResources.
func allCount(t *testing.T, resourceType string) int {
	t.Helper()
	all, ok := demo.GetResources(resourceType)
	if !ok {
		t.Fatalf("GetResources(%q) returned ok=false; demo fixtures must exist", resourceType)
	}
	return len(all)
}

// ---------------------------------------------------------------------------
// 1. TestDemoPageSize_Constant
// ---------------------------------------------------------------------------

func TestDemoPageSize_Constant(t *testing.T) {
	if demo.DemoPageSize != 5 {
		t.Errorf("DemoPageSize = %d; want 5", demo.DemoPageSize)
	}
}

// ---------------------------------------------------------------------------
// 2. TestDemoGetResourcesPaginated_SmallTypes
// ---------------------------------------------------------------------------

func TestDemoGetResourcesPaginated_SmallTypes(t *testing.T) {
	for _, rt := range smallTypes {
		t.Run(rt, func(t *testing.T) {
			total := allCount(t, rt)
			if total > demo.DemoPageSize {
				t.Skipf("type %q has %d items (> DemoPageSize %d); not a small type", rt, total, demo.DemoPageSize)
			}

			result, ok := demo.GetResourcesPaginated(rt)
			if !ok {
				t.Fatalf("GetResourcesPaginated(%q) returned ok=false; expected true for known type", rt)
			}

			// Should return all items since total <= DemoPageSize.
			if got := len(result.Resources); got != total {
				t.Errorf("len(Resources) = %d; want %d (all items)", got, total)
			}

			// Pagination metadata must be set.
			if result.Pagination == nil {
				t.Fatal("Pagination is nil; expected non-nil PaginationMeta")
			}
			if result.Pagination.IsTruncated {
				t.Error("IsTruncated = true; want false for type with ≤5 items")
			}
			if result.Pagination.PageSize != total {
				t.Errorf("PageSize = %d; want %d", result.Pagination.PageSize, total)
			}
			if result.Pagination.TotalHint != total {
				t.Errorf("TotalHint = %d; want %d", result.Pagination.TotalHint, total)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 3. TestDemoGetResourcesPaginated_LargeTypes
// ---------------------------------------------------------------------------

func TestDemoGetResourcesPaginated_LargeTypes(t *testing.T) {
	for _, rt := range largeTypes {
		t.Run(rt, func(t *testing.T) {
			total := allCount(t, rt)
			if total <= demo.DemoPageSize {
				t.Skipf("type %q has %d items (≤ DemoPageSize %d); not a large type", rt, total, demo.DemoPageSize)
			}

			result, ok := demo.GetResourcesPaginated(rt)
			if !ok {
				t.Fatalf("GetResourcesPaginated(%q) returned ok=false; expected true for known type", rt)
			}

			// First page must be exactly DemoPageSize items.
			if got := len(result.Resources); got != demo.DemoPageSize {
				t.Errorf("len(Resources) = %d; want %d (DemoPageSize)", got, demo.DemoPageSize)
			}

			// Pagination metadata must indicate truncation.
			if result.Pagination == nil {
				t.Fatal("Pagination is nil; expected non-nil PaginationMeta")
			}
			if !result.Pagination.IsTruncated {
				t.Error("IsTruncated = false; want true for type with >5 items")
			}
			if result.Pagination.PageSize != demo.DemoPageSize {
				t.Errorf("PageSize = %d; want %d", result.Pagination.PageSize, demo.DemoPageSize)
			}
			if result.Pagination.TotalHint != total {
				t.Errorf("TotalHint = %d; want %d", result.Pagination.TotalHint, total)
			}

			// First page must contain the FIRST DemoPageSize items in order.
			all, _ := demo.GetResources(rt)
			for i := 0; i < demo.DemoPageSize; i++ {
				if result.Resources[i].ID != all[i].ID {
					t.Errorf("Resources[%d].ID = %q; want %q", i, result.Resources[i].ID, all[i].ID)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 4. TestDemoGetMoreResources_ReturnsRemainder
// ---------------------------------------------------------------------------

func TestDemoGetMoreResources_ReturnsRemainder(t *testing.T) {
	for _, rt := range largeTypes {
		t.Run(rt, func(t *testing.T) {
			total := allCount(t, rt)
			if total <= demo.DemoPageSize {
				t.Skipf("type %q has ≤ DemoPageSize items; skip", rt)
			}

			// Must call GetResourcesPaginated first to establish state.
			_, ok := demo.GetResourcesPaginated(rt)
			if !ok {
				t.Fatalf("GetResourcesPaginated(%q) returned ok=false", rt)
			}

			// GetMoreResources should return the remaining items.
			remainder, ok := demo.GetMoreResources(rt)
			if !ok {
				t.Fatalf("GetMoreResources(%q) returned ok=false; expected remaining items", rt)
			}

			expectedRemaining := total - demo.DemoPageSize
			if got := len(remainder.Resources); got != expectedRemaining {
				t.Errorf("len(Resources) = %d; want %d remaining", got, expectedRemaining)
			}

			// Remainder should not be truncated (all leftover items returned).
			if remainder.Pagination == nil {
				t.Fatal("Pagination is nil; expected non-nil PaginationMeta")
			}
			if remainder.Pagination.IsTruncated {
				t.Error("IsTruncated = true; want false for final page")
			}

			// Verify remainder contains the correct items in order.
			all, _ := demo.GetResources(rt)
			for i, r := range remainder.Resources {
				expectedIdx := demo.DemoPageSize + i
				if r.ID != all[expectedIdx].ID {
					t.Errorf("Resources[%d].ID = %q; want %q (index %d from full list)",
						i, r.ID, all[expectedIdx].ID, expectedIdx)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 5. TestDemoGetMoreResources_SecondCallEmpty
// ---------------------------------------------------------------------------

func TestDemoGetMoreResources_SecondCallEmpty(t *testing.T) {
	for _, rt := range largeTypes {
		t.Run(rt, func(t *testing.T) {
			total := allCount(t, rt)
			if total <= demo.DemoPageSize {
				t.Skipf("type %q has ≤ DemoPageSize items; skip", rt)
			}

			// First: paginated fetch.
			_, ok := demo.GetResourcesPaginated(rt)
			if !ok {
				t.Fatalf("GetResourcesPaginated(%q) returned ok=false", rt)
			}

			// Second: get remainder.
			_, ok = demo.GetMoreResources(rt)
			if !ok {
				t.Fatalf("first GetMoreResources(%q) returned ok=false", rt)
			}

			// Third: second call should return ok=false (no more data).
			result, ok := demo.GetMoreResources(rt)
			if ok {
				t.Errorf("second GetMoreResources(%q) returned ok=true with %d items; want ok=false",
					rt, len(result.Resources))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 6. TestDemoGetMoreResources_WithoutPaginatedCall
// ---------------------------------------------------------------------------

func TestDemoGetMoreResources_WithoutPaginatedCall(t *testing.T) {
	// Calling GetMoreResources without a preceding GetResourcesPaginated
	// should return ok=false — there's no pending page.
	for _, rt := range largeTypes {
		t.Run(rt, func(t *testing.T) {
			result, ok := demo.GetMoreResources(rt)
			if ok {
				t.Errorf("GetMoreResources(%q) without prior pagination returned ok=true with %d items; want ok=false",
					rt, len(result.Resources))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 7. TestDemoGetResourcesPaginated_ExactPageSize
// ---------------------------------------------------------------------------

func TestDemoGetResourcesPaginated_ExactPageSize(t *testing.T) {
	// Find or verify a type with exactly DemoPageSize items, or use a known small type
	// and check the boundary condition.
	for _, rt := range smallTypes {
		total := allCount(t, rt)
		if total != demo.DemoPageSize {
			continue
		}

		// Found a type with exactly DemoPageSize items.
		t.Run(rt+"_exact", func(t *testing.T) {
			result, ok := demo.GetResourcesPaginated(rt)
			if !ok {
				t.Fatalf("GetResourcesPaginated(%q) returned ok=false", rt)
			}
			if got := len(result.Resources); got != demo.DemoPageSize {
				t.Errorf("len(Resources) = %d; want %d", got, demo.DemoPageSize)
			}
			if result.Pagination == nil {
				t.Fatal("Pagination is nil")
			}
			// Exactly at page size means NOT truncated (no items beyond).
			if result.Pagination.IsTruncated {
				t.Error("IsTruncated = true; want false for exactly DemoPageSize items")
			}

			// GetMoreResources should return ok=false.
			_, moreOK := demo.GetMoreResources(rt)
			if moreOK {
				t.Error("GetMoreResources returned ok=true for type at exact page size; want ok=false")
			}
		})
		return // tested one, that's enough for this boundary
	}

	// If no existing type has exactly 5 items, test with a type that has fewer
	// and verify no truncation.
	t.Run("boundary_no_exact_type", func(t *testing.T) {
		// Use the first small type as a proxy: fewer than page size still means no truncation.
		rt := smallTypes[0]
		total := allCount(t, rt)
		result, ok := demo.GetResourcesPaginated(rt)
		if !ok {
			t.Fatalf("GetResourcesPaginated(%q) returned ok=false", rt)
		}
		if got := len(result.Resources); got != total {
			t.Errorf("len(Resources) = %d; want %d", got, total)
		}
		if result.Pagination == nil {
			t.Fatal("Pagination is nil")
		}
		if result.Pagination.IsTruncated {
			t.Errorf("IsTruncated = true; want false for %d items (< DemoPageSize %d)", total, demo.DemoPageSize)
		}
	})
}

// ---------------------------------------------------------------------------
// 8. TestDemoGetResourcesPaginated_UnknownType
// ---------------------------------------------------------------------------

func TestDemoGetResourcesPaginated_UnknownType(t *testing.T) {
	_, ok := demo.GetResourcesPaginated("nonexistent-type-xyz")
	if ok {
		t.Error("GetResourcesPaginated(\"nonexistent-type-xyz\") returned ok=true; want false")
	}
}

// ---------------------------------------------------------------------------
// 9. TestDemoGetMoreResources_UnknownType
// ---------------------------------------------------------------------------

func TestDemoGetMoreResources_UnknownType(t *testing.T) {
	_, ok := demo.GetMoreResources("nonexistent-type-xyz")
	if ok {
		t.Error("GetMoreResources(\"nonexistent-type-xyz\") returned ok=true; want false")
	}
}

// ---------------------------------------------------------------------------
// 10. TestDemoGetResourcesPaginated_AllTypes
// ---------------------------------------------------------------------------

func TestDemoGetResourcesPaginated_AllTypes(t *testing.T) {
	// Every demo resource type must work with paginated fetch.
	// This ensures no type is missed during implementation.
	allDemoTypes := []string{
		"ec2", "lambda", "s3", "dbi", "redis", "dbc", "eks", "ng",
		"secrets", "ssm", "kms", "vpc", "sg", "subnet", "elb", "tg",
		"role", "policy", "iam-user", "iam-group", "waf",
		"r53", "cf", "acm", "apigw",
		"ecs", "ecs-svc", "ecs-task",
		"alarm", "logs", "trail",
		"sqs", "sns", "sns-sub", "eb-rule", "kinesis", "msk", "sfn",
		"cfn", "ecr", "codeartifact", "pipeline", "cb",
		"ddb", "opensearch", "redshift", "efs",
		"asg", "eb",
		"nat", "igw", "eip", "rtb",
		"glue", "athena",
		"backup", "ses",
		"rds-snap", "docdb-snap",
		"vpce", "tgw", "eni",
	}

	for _, rt := range allDemoTypes {
		t.Run(rt, func(t *testing.T) {
			// Verify the type exists in demo.
			all, ok := demo.GetResources(rt)
			if !ok {
				t.Fatalf("GetResources(%q) returned ok=false; demo fixtures must exist", rt)
			}
			total := len(all)

			result, pOK := demo.GetResourcesPaginated(rt)
			if !pOK {
				t.Fatalf("GetResourcesPaginated(%q) returned ok=false; must work for all demo types", rt)
			}

			if result.Pagination == nil {
				t.Fatal("Pagination is nil; expected non-nil PaginationMeta")
			}

			if total <= demo.DemoPageSize {
				// Small type: should return all items, not truncated.
				if got := len(result.Resources); got != total {
					t.Errorf("len(Resources) = %d; want %d (all items, small type)", got, total)
				}
				if result.Pagination.IsTruncated {
					t.Error("IsTruncated = true; want false for small type")
				}
			} else {
				// Large type: should return DemoPageSize items, truncated.
				if got := len(result.Resources); got != demo.DemoPageSize {
					t.Errorf("len(Resources) = %d; want %d (DemoPageSize, large type)", got, demo.DemoPageSize)
				}
				if !result.Pagination.IsTruncated {
					t.Error("IsTruncated = false; want true for large type")
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 11. TestDemoGetResourcesPaginated_TotalItems_Consistency
// ---------------------------------------------------------------------------

func TestDemoGetResourcesPaginated_TotalItems_Consistency(t *testing.T) {
	// For large types, verify paginated first page + GetMoreResources = all items.
	for _, rt := range largeTypes {
		t.Run(rt, func(t *testing.T) {
			total := allCount(t, rt)
			if total <= demo.DemoPageSize {
				t.Skipf("type %q has ≤ DemoPageSize items; skip", rt)
			}

			page1, ok := demo.GetResourcesPaginated(rt)
			if !ok {
				t.Fatalf("GetResourcesPaginated(%q) returned ok=false", rt)
			}

			page2, ok := demo.GetMoreResources(rt)
			if !ok {
				t.Fatalf("GetMoreResources(%q) returned ok=false", rt)
			}

			combined := len(page1.Resources) + len(page2.Resources)
			if combined != total {
				t.Errorf("page1 (%d) + page2 (%d) = %d; want %d (total from GetResources)",
					len(page1.Resources), len(page2.Resources), combined, total)
			}

			// Verify no duplicate IDs between pages.
			seen := make(map[string]bool, combined)
			for _, r := range page1.Resources {
				seen[r.ID] = true
			}
			for _, r := range page2.Resources {
				if seen[r.ID] {
					t.Errorf("duplicate ID %q found across page1 and page2", r.ID)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 12. TestDemoGetChildResourcesPaginated_LargeChild
// ---------------------------------------------------------------------------

func TestDemoGetChildResourcesPaginated_LargeChild(t *testing.T) {
	// role_policies has 7 items (>5), which makes it a good candidate.
	// s3_objects and others may also qualify.
	childTypes := []struct {
		childType string
		parentCtx map[string]string
	}{
		{
			childType: "role_policies",
			parentCtx: map[string]string{"role_name": "acme-eks-node-role"},
		},
	}

	for _, ct := range childTypes {
		t.Run(ct.childType, func(t *testing.T) {
			// First verify this child type has >5 items via GetChildResources.
			all, ok := demo.GetChildResources(ct.childType, ct.parentCtx)
			if !ok {
				t.Fatalf("GetChildResources(%q) returned ok=false", ct.childType)
			}
			total := len(all)
			if total <= demo.DemoPageSize {
				t.Skipf("child type %q has %d items (≤ DemoPageSize); skip", ct.childType, total)
			}

			// Paginated first page.
			result, pOK := demo.GetChildResourcesPaginated(ct.childType, ct.parentCtx)
			if !pOK {
				t.Fatalf("GetChildResourcesPaginated(%q) returned ok=false; expected true", ct.childType)
			}

			if got := len(result.Resources); got != demo.DemoPageSize {
				t.Errorf("len(Resources) = %d; want %d (DemoPageSize)", got, demo.DemoPageSize)
			}
			if result.Pagination == nil {
				t.Fatal("Pagination is nil; expected non-nil PaginationMeta")
			}
			if !result.Pagination.IsTruncated {
				t.Error("IsTruncated = false; want true")
			}
			if result.Pagination.TotalHint != total {
				t.Errorf("TotalHint = %d; want %d", result.Pagination.TotalHint, total)
			}

			// Get remainder.
			remainder, rOK := demo.GetMoreChildResources(ct.childType, ct.parentCtx)
			if !rOK {
				t.Fatalf("GetMoreChildResources(%q) returned ok=false; expected remaining items", ct.childType)
			}

			expectedRemaining := total - demo.DemoPageSize
			if got := len(remainder.Resources); got != expectedRemaining {
				t.Errorf("len(remainder.Resources) = %d; want %d", got, expectedRemaining)
			}
			if remainder.Pagination == nil {
				t.Fatal("remainder.Pagination is nil")
			}
			if remainder.Pagination.IsTruncated {
				t.Error("remainder IsTruncated = true; want false for final page")
			}

			// Combined must equal total.
			combined := len(result.Resources) + len(remainder.Resources)
			if combined != total {
				t.Errorf("page1 (%d) + page2 (%d) = %d; want %d",
					len(result.Resources), len(remainder.Resources), combined, total)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 13. TestDemoGetChildResourcesPaginated_SmallChild
// ---------------------------------------------------------------------------

func TestDemoGetChildResourcesPaginated_SmallChild(t *testing.T) {
	// iam_group_members has 4 items (≤5) — should return all without truncation.
	childType := "iam_group_members"
	parentCtx := map[string]string{"group_name": "admins"}

	all, ok := demo.GetChildResources(childType, parentCtx)
	if !ok {
		t.Fatalf("GetChildResources(%q) returned ok=false", childType)
	}
	total := len(all)
	if total > demo.DemoPageSize {
		t.Skipf("child type %q has %d items (> DemoPageSize); not a small child", childType, total)
	}

	result, pOK := demo.GetChildResourcesPaginated(childType, parentCtx)
	if !pOK {
		t.Fatalf("GetChildResourcesPaginated(%q) returned ok=false; expected true", childType)
	}

	if got := len(result.Resources); got != total {
		t.Errorf("len(Resources) = %d; want %d (all items)", got, total)
	}
	if result.Pagination == nil {
		t.Fatal("Pagination is nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("IsTruncated = true; want false for small child type")
	}
}

// ---------------------------------------------------------------------------
// 14. TestDemoGetChildResourcesPaginated_UnknownType
// ---------------------------------------------------------------------------

func TestDemoGetChildResourcesPaginated_UnknownType(t *testing.T) {
	_, ok := demo.GetChildResourcesPaginated("nonexistent_child_xyz", map[string]string{})
	if ok {
		t.Error("GetChildResourcesPaginated for unknown type returned ok=true; want false")
	}
}

// ---------------------------------------------------------------------------
// 15. TestDemoGetMoreChildResources_SecondCallEmpty
// ---------------------------------------------------------------------------

func TestDemoGetMoreChildResources_SecondCallEmpty(t *testing.T) {
	childType := "role_policies"
	parentCtx := map[string]string{"role_name": "acme-eks-node-role"}

	all, ok := demo.GetChildResources(childType, parentCtx)
	if !ok {
		t.Fatalf("GetChildResources(%q) returned ok=false", childType)
	}
	if len(all) <= demo.DemoPageSize {
		t.Skip("role_policies has ≤ DemoPageSize items; skip")
	}

	// Fetch first page.
	_, ok = demo.GetChildResourcesPaginated(childType, parentCtx)
	if !ok {
		t.Fatalf("GetChildResourcesPaginated(%q) returned ok=false", childType)
	}

	// Fetch remainder.
	_, ok = demo.GetMoreChildResources(childType, parentCtx)
	if !ok {
		t.Fatalf("first GetMoreChildResources(%q) returned ok=false", childType)
	}

	// Second call should return ok=false.
	result, ok := demo.GetMoreChildResources(childType, parentCtx)
	if ok {
		t.Errorf("second GetMoreChildResources(%q) returned ok=true with %d items; want ok=false",
			childType, len(result.Resources))
	}
}

// ---------------------------------------------------------------------------
// 16. TestDemoGetMoreChildResources_WithoutPaginatedCall
// ---------------------------------------------------------------------------

func TestDemoGetMoreChildResources_WithoutPaginatedCall(t *testing.T) {
	result, ok := demo.GetMoreChildResources("role_policies", map[string]string{"role_name": "test"})
	if ok {
		t.Errorf("GetMoreChildResources without prior pagination returned ok=true with %d items; want ok=false",
			len(result.Resources))
	}
}

// ---------------------------------------------------------------------------
// 17. TestDemoGetResourcesPaginated_Idempotent
// ---------------------------------------------------------------------------

func TestDemoGetResourcesPaginated_Idempotent(t *testing.T) {
	// Calling GetResourcesPaginated twice should return the same first page
	// (it resets pagination state).
	rt := "ec2"
	total := allCount(t, rt)
	if total <= demo.DemoPageSize {
		t.Skip("ec2 has too few items")
	}

	result1, ok := demo.GetResourcesPaginated(rt)
	if !ok {
		t.Fatalf("first GetResourcesPaginated(%q) returned ok=false", rt)
	}

	result2, ok := demo.GetResourcesPaginated(rt)
	if !ok {
		t.Fatalf("second GetResourcesPaginated(%q) returned ok=false", rt)
	}

	if len(result1.Resources) != len(result2.Resources) {
		t.Errorf("result1 has %d items, result2 has %d; want same count",
			len(result1.Resources), len(result2.Resources))
	}

	for i := range result1.Resources {
		if i >= len(result2.Resources) {
			break
		}
		if result1.Resources[i].ID != result2.Resources[i].ID {
			t.Errorf("Resources[%d].ID: first=%q, second=%q; want identical",
				i, result1.Resources[i].ID, result2.Resources[i].ID)
			break
		}
	}
}

// ---------------------------------------------------------------------------
// 18. TestDemoGetResourcesPaginated_ResourceFieldsPreserved
// ---------------------------------------------------------------------------

func TestDemoGetResourcesPaginated_ResourceFieldsPreserved(t *testing.T) {
	// Paginated results must have identical resource data as non-paginated.
	for _, rt := range largeTypes {
		t.Run(rt, func(t *testing.T) {
			all, ok := demo.GetResources(rt)
			if !ok || len(all) <= demo.DemoPageSize {
				t.Skip("skip: not available or too few items")
			}

			result, ok := demo.GetResourcesPaginated(rt)
			if !ok {
				t.Fatalf("GetResourcesPaginated(%q) returned ok=false", rt)
			}

			for i, r := range result.Resources {
				if r.ID != all[i].ID {
					t.Errorf("Resources[%d].ID = %q; want %q", i, r.ID, all[i].ID)
				}
				if r.Name != all[i].Name {
					t.Errorf("Resources[%d].Name = %q; want %q", i, r.Name, all[i].Name)
				}
				if r.Status != all[i].Status {
					t.Errorf("Resources[%d].Status = %q; want %q", i, r.Status, all[i].Status)
				}
				if r.RawStruct == nil && all[i].RawStruct != nil {
					t.Errorf("Resources[%d].RawStruct is nil; want non-nil", i)
				}
			}
		})
	}
}
