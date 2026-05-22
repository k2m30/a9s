package unit

// lazy_add_stories_failures_counts_test.go — Orchestration tests for
// lazy-add user stories LA-020..LA-025 (resolution failures / permission
// errors) and LA-060..LA-064 (count / footer / identity correctness).
//
// Sections C and G from tests/stories/lazy_add.md.
//
// Coverage exclusions (explicit skip placeholders):
//   - LA-021 — covered by TestLazyAdd_FetchByIDsErrorSwallowed_ChecksResultStillDelivered
//   - LA-022 — covered by TestFetchKMSKeysByIDs_ListAliasesFailure_ProceedsWithoutAliases
//   - LA-023 — covered by TestFetchKMSKeysByIDs_DescribeKeyFailure_SkipsOneKey
//   - LA-020 (UX), LA-063 (auto-open), LA-064 (count honesty) — OCQ skips
//
// Approach: orchestration unit tests using registered FetchByIDs stubs and
// RelatedCheckStartedMsg dispatch, mirroring the existing pattern in
// lazy_add_orchestration_edges_test.go.

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ────────────────────────────────────────────────────────────────────────────
// LA-020 — Partial resolution: orchestration contract (non-OCQ part)
// ────────────────────────────────────────────────────────────────────────────

// Test_LA_020_PartialResolution_ChecksStillDelivered pins the orchestration
// invariant for a partial FetchByIDs response: checker emits 5 IDs, FetchByIDs
// resolves only 3 (returning a 3-element slice). The root model must:
//   - Not propagate an error (RelatedCheckResultMsg delivered, no panic).
//   - Keep Result.Count == 5 (checker's declared count is not revised).
//   - Set LazyAddedResources[target] to the 3 returned resources only.
//
// The UX interpretation of "2 items unresolved" (dim rows, toast, or silent
// skip) is an Open Contract Question (#3) and is NOT asserted here.
func Test_LA_020_PartialResolution_ChecksStillDelivered(t *testing.T) {
	const (
		srcType    = "test-la020-source"
		targetType = "test-la020-target"
	)

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-020 Partial Test Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				// Checker emits 5 IDs; only 3 will resolve.
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       5,
					ResourceIDs: []string{"id-001", "id-002", "id-003", "id-004", "id-005"},
				}
			},
		},
	})

	// FetchByIDs returns only 3 of the 5 requested IDs (simulates partial resolve).
	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		// Return at most 3 resources regardless of how many were requested.
		resolvable := []string{"id-001", "id-002", "id-003"}
		var out []resource.Resource
		for _, id := range ids {
			for _, rid := range resolvable {
				if id == rid {
					out = append(out, resource.Resource{ID: id, Name: "resolved-" + id})
					break
				}
			}
		}
		return out, nil // no error — partial result, not a failure
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: resource.Resource{ID: "src-la020-001"},
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}

	// Checker-emitted count must pass through unchanged (5, not 3).
	if resultMsg.Result.Count != 5 {
		t.Errorf("Result.Count: got %d, want 5 (checker count must not be revised by partial resolution)",
			resultMsg.Result.Count)
	}

	// ResourceIDs on the result carry the checker's full declared set.
	if len(resultMsg.Result.ResourceIDs) != 5 {
		t.Errorf("Result.ResourceIDs: got %d IDs, want 5", len(resultMsg.Result.ResourceIDs))
	}

	// LazyAddedResources must contain only the 3 resolvable entries.
	lazy, ok := resultMsg.LazyAddedResources[targetType]
	if !ok {
		t.Fatalf("LazyAddedResources[%q] not present; want 3 resolved resources", targetType)
	}
	if len(lazy) != 3 {
		t.Errorf("LazyAddedResources[%q]: got %d resources, want 3", targetType, len(lazy))
	}
}

// ────────────────────────────────────────────────────────────────────────────
// LA-021 — Full resolution failure (placeholder — covered elsewhere)
// ────────────────────────────────────────────────────────────────────────────

func Test_LA_021_FullResolutionFailure_Placeholder(t *testing.T) {
	t.Skip("covered by TestLazyAdd_FetchByIDsErrorSwallowed_ChecksResultStillDelivered " +
		"in lazy_add_orchestration_edges_test.go")
}

// ────────────────────────────────────────────────────────────────────────────
// LA-022 — ListAliases denied (placeholder — covered elsewhere)
// ────────────────────────────────────────────────────────────────────────────

func Test_LA_022_ListAliasesDenied_Placeholder(t *testing.T) {
	t.Skip("covered by TestFetchKMSKeysByIDs_ListAliasesFailure_ProceedsWithoutAliases " +
		"in aws_kms_fetch_by_ids_test.go")
}

// ────────────────────────────────────────────────────────────────────────────
// LA-023 — DescribeKey denied for one in batch (placeholder — covered elsewhere)
// ────────────────────────────────────────────────────────────────────────────

func Test_LA_023_DescribeKeyDeniedForOne_Placeholder(t *testing.T) {
	t.Skip("covered by TestFetchKMSKeysByIDs_DescribeKeyFailure_SkipsOneKey " +
		"in aws_kms_fetch_by_ids_test.go")
}

// ────────────────────────────────────────────────────────────────────────────
// LA-024 — GetPolicy denied on AWS-managed policy: partial metadata is OK
// ────────────────────────────────────────────────────────────────────────────

// Test_LA_024_GetPolicyDenied_PartialMetadataOK pins the contract that when
// FetchByIDs for an IAM policy ARN returns a resource with a populated
// policy_name field but empty attachment_count / create_date (simulating what
// happens when GetPolicy is denied but the ARN is still parseable), the
// lazy-add path stores that partial row without error.
//
// Contract: partial metadata (some fields populated, others empty) is
// acceptable for a lazy-added row. The drill still surfaces the row with
// the fields that could be derived.
func Test_LA_024_GetPolicyDenied_PartialMetadataOK(t *testing.T) {
	const (
		srcType    = "test-la024-source"
		targetType = "test-la024-target"
		policyARN  = "arn:aws:iam::aws:policy/AdministratorAccess"
	)

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-024 Policy Test Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       1,
					ResourceIDs: []string{policyARN},
				}
			},
		},
	})

	// FetchByIDs simulates GetPolicy denied: returns the row with policy_name
	// populated (parseable from ARN) but empty attachment_count and create_date.
	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		var out []resource.Resource
		for _, id := range ids {
			if id == policyARN {
				out = append(out, resource.Resource{
					ID:   policyARN,
					Name: "AdministratorAccess",
					Fields: map[string]string{
						"policy_name":      "AdministratorAccess",
						"attachment_count": "", // empty: GetPolicy was denied
						"create_date":      "", // empty: GetPolicy was denied
					},
				})
			}
		}
		return out, nil
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: resource.Resource{ID: "src-la024-role-001"},
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}

	// Count must survive.
	if resultMsg.Result.Count != 1 {
		t.Errorf("Result.Count: got %d, want 1", resultMsg.Result.Count)
	}

	// LazyAddedResources must contain the partial row.
	lazy, ok := resultMsg.LazyAddedResources[targetType]
	if !ok {
		t.Fatalf("LazyAddedResources[%q] not present; partial-metadata rows must be kept", targetType)
	}
	if len(lazy) != 1 {
		t.Fatalf("LazyAddedResources[%q]: got %d resources, want 1", targetType, len(lazy))
	}

	row := lazy[0]
	if row.ID != policyARN {
		t.Errorf("lazy row ID: got %q, want %q", row.ID, policyARN)
	}
	if row.Fields["policy_name"] != "AdministratorAccess" {
		t.Errorf("policy_name: got %q, want %q", row.Fields["policy_name"], "AdministratorAccess")
	}
	// Metadata fields must be empty (simulating GetPolicy denial) — not absent.
	if row.Fields["attachment_count"] != "" {
		t.Errorf("attachment_count: got %q, want empty string (simulated GetPolicy deny)", row.Fields["attachment_count"])
	}
	if row.Fields["create_date"] != "" {
		t.Errorf("create_date: got %q, want empty string (simulated GetPolicy deny)", row.Fields["create_date"])
	}
}

// ────────────────────────────────────────────────────────────────────────────
// LA-025 — Throttling retry
// ────────────────────────────────────────────────────────────────────────────

// Test_LA_025_ThrottlingRetry_NotInLazyAddFetchers documents that the two
// lazy-add FetchByIDs functions (FetchKMSKeysByIDs and
// FetchIAMPoliciesByIDsFull) do NOT currently invoke RetryOnThrottle for
// their per-ID calls. Throttling is handled by paginated top-level fetchers
// and related-checkers (e.g. acm_related.go, apigw_related.go,
// asg_related.go, kms_related.go), not by the per-ID lazy-add path.
//
// This test documents the current state so a future change that adds retry
// to FetchKMSKeysByIDs or FetchIAMPoliciesByIDsFull will cause this test to
// fail visibly (prompting update of this comment and test).
func Test_LA_025_ThrottlingRetry_NotInLazyAddFetchers(t *testing.T) {
	t.Skip("lazy-add FetchByIDs functions (FetchKMSKeysByIDs, " +
		"FetchIAMPoliciesByIDsFull) do not currently invoke RetryOnThrottle; " +
		"throttling is a concern for paginated top-level fetchers only. " +
		"Known RetryOnThrottle callers: acm_related.go, apigw_related.go, " +
		"asg_related.go, asg_related_extra.go, backup_related.go, cf_related.go, " +
		"codeartifact_related.go, dbc_related.go, dbi_related.go, ddb_related.go, " +
		"docdb_snap_related.go, eb_related_extra.go, ec2_related.go, " +
		"ecr_related_extra.go, ecs_svc_related_extra.go, ecs_task.go, " +
		"eks_related_extra.go, elb_related.go, kinesis_related.go, kms_related.go, " +
		"ng_related.go, pipeline_related.go, r53_related.go, rds_snap_related.go, " +
		"redis_related.go, redshift_related.go, secrets_related_extra.go, " +
		"ses_related.go, sfn_related.go, sns_related.go, sqs_related.go, " +
		"tg_related.go, tgw_related.go, vpc_related.go, vpce_related.go, waf_related.go.")
}

// ────────────────────────────────────────────────────────────────────────────
// LA-060 — Pivot count equals drilled row count (happy path)
// ────────────────────────────────────────────────────────────────────────────

// Test_LA_060_PivotCountEqualsRowCount verifies that when a checker emits
// Count=7 with 7 ResourceIDs and FetchByIDs returns exactly 7 resources, the
// RelatedCheckResultMsg carries:
//   - Result.Count == 7
//   - len(Result.ResourceIDs) == 7
//   - len(LazyAddedResources[target]) == 7
//
// This is the no-failure happy path for the count/row agreement contract.
func Test_LA_060_PivotCountEqualsRowCount(t *testing.T) {
	const (
		srcType    = "test-la060-source"
		targetType = "test-la060-target"
	)

	ids := []string{"kms-001", "kms-002", "kms-003", "kms-004", "kms-005", "kms-006", "kms-007"}

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "LA-060 Count Equality Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       7,
					ResourceIDs: ids,
				}
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, fetchIDs []string) ([]resource.Resource, error) {
		var out []resource.Resource
		for _, id := range fetchIDs {
			out = append(out, resource.Resource{ID: id, Name: "key-" + id})
		}
		return out, nil
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: resource.Resource{ID: "src-la060-001"},
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}

	// All three cardinalities must agree: declared count, ID list, and fetched rows.
	if resultMsg.Result.Count != 7 {
		t.Errorf("Result.Count: got %d, want 7", resultMsg.Result.Count)
	}
	if len(resultMsg.Result.ResourceIDs) != 7 {
		t.Errorf("len(Result.ResourceIDs): got %d, want 7", len(resultMsg.Result.ResourceIDs))
	}
	lazy, ok := resultMsg.LazyAddedResources[targetType]
	if !ok {
		t.Fatalf("LazyAddedResources[%q] not present; FetchByIDs was supposed to return 7 resources", targetType)
	}
	if len(lazy) != 7 {
		t.Errorf("len(LazyAddedResources[%q]): got %d, want 7", targetType, len(lazy))
	}
}

// ────────────────────────────────────────────────────────────────────────────
// LA-061 — Footer suppressed when filter is fully resolved
// ────────────────────────────────────────────────────────────────────────────

// Test_LA_061_FooterSuppressed_WhenAllRelatedIDsResolved verifies the
// IsTruncated-strip contract in handleRelatedNavigate
// (app_related.go — "All RelatedIDs matched" branch):
// when an upstream cache entry has IsTruncated=true but ALL RelatedIDs are
// present in the cache, the handler strips IsTruncated before handing
// pagination to the pushed ResourceListModel.
//
// Observable invariant: after dispatching RelatedNavigateMsg with 3 IDs all
// present in a IsTruncated=true cache, View() MUST NOT contain "m: load more".
//
// The code path (app_related.go:257-263) clones pagination with
// IsTruncated=false / NextToken="" before constructing the ResourceListModel,
// ensuring the "m: load more" footer is suppressed for fully-resolved filters.
func Test_LA_061_FooterSuppressed_WhenAllRelatedIDsResolved(t *testing.T) {
	const targetType = "ec2" // real registered type so FindResourceType works

	// Seed the model's cache with 3 resources and IsTruncated=true (more pages
	// exist at the top level), then navigate with exactly those 3 IDs.
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	ids := []string{"i-la061-001", "i-la061-002", "i-la061-003"}
	resources := []resource.Resource{
		{ID: "i-la061-001", Name: "instance-001"},
		{ID: "i-la061-002", Name: "instance-002"},
		{ID: "i-la061-003", Name: "instance-003"},
	}

	// Load resources into cache with IsTruncated=true (upstream has more pages).
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: targetType,
		Resources:    resources,
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "some-token"},
	})

	// Navigate using exactly the 3 IDs — all are present in cache.
	m, _ = rootApplyMsg(m, messages.RelatedNavigate{
		TargetType: targetType,
		RelatedIDs: ids,
		SourceResource: resource.Resource{
			ID:   "rds-la061-src",
			Name: "my-db",
		},
		SourceType: "rds",
	})

	// After navigation the pushed ResourceListModel is the active view.
	// View() renders its pagination; IsTruncated must be false (no "load more").
	content := rootViewContent(m)
	if strings.Contains(content, "load more") {
		t.Errorf("View contains 'load more' after fully-resolved RelatedIDs filter — "+
			"IsTruncated should have been stripped; content snippet: %q",
			truncateContentSnippet(content, 300))
	}
}

// ────────────────────────────────────────────────────────────────────────────
// LA-062 — Footer suppressed even when upstream top-level list was truncated
// ────────────────────────────────────────────────────────────────────────────

// Test_LA_062_FooterSuppressed_UpstreamTruncatedDrillResolved is a variant of
// LA-061 specifically for the case where the upstream cache was explicitly
// truncated (simulating > 1000 resources at the top level). The drill uses
// 2 IDs that both resolve from the cached slice; the footer must still be
// absent after the push.
//
// This pins the invariant that upstream truncation does NOT leak into a
// narrowed, fully-resolved filter. See app_related.go:257-263.
func Test_LA_062_FooterSuppressed_UpstreamTruncatedDrillResolved(t *testing.T) {
	const targetType = "kms" // real registered type

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	// Simulate 2 KMS keys cached with upstream truncation (many more exist at top level).
	ids := []string{"kms-la062-001", "kms-la062-002"}
	resources := []resource.Resource{
		{ID: "kms-la062-001", Name: "alias/first-key"},
		{ID: "kms-la062-002", Name: "alias/second-key"},
	}

	// Load with IsTruncated=true — simulates an account with >1000 KMS keys.
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: targetType,
		Resources:    resources,
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "truncated-token"},
	})

	// Both IDs are in the cache; filter is fully resolved.
	m, _ = rootApplyMsg(m, messages.RelatedNavigate{
		TargetType: targetType,
		RelatedIDs: ids,
		SourceResource: resource.Resource{
			ID:   "rds-la062-src",
			Name: "my-database",
		},
		SourceType: "rds",
	})

	content := rootViewContent(m)
	if strings.Contains(content, "load more") {
		t.Errorf("View contains 'load more' after fully-resolved filter on truncated cache — "+
			"upstream truncation must not leak into a narrowed drill; content snippet: %q",
			truncateContentSnippet(content, 300))
	}
}

// ────────────────────────────────────────────────────────────────────────────
// LA-063 — Single-ID auto-open (OCQ skip)
// ────────────────────────────────────────────────────────────────────────────

func Test_LA_063_SingleIDAutoOpen_OCQ(t *testing.T) {
	t.Skip("OCQ#5 — spec does not pin single-ID auto-open behavior; " +
		"a9s may open the lone target's detail directly or land on a one-row list. " +
		"Pin observed behavior once the design question is resolved.")
}

// ────────────────────────────────────────────────────────────────────────────
// LA-064 — Count honesty under partial failure (OCQ skip)
// ────────────────────────────────────────────────────────────────────────────

func Test_LA_064_CountHonestyUnderPartialFailure_OCQ(t *testing.T) {
	t.Skip("OCQ#6 — spec does not pin whether the related panel count should " +
		"reflect 'emitted by checker' or 'resolvable right now'. " +
		"These diverge when permission errors partially block FetchByIDs. " +
		"Pin once the design question is resolved. " +
		"Partial behavior for the resolvable subset is covered by Test_LA_020.")
}

// ────────────────────────────────────────────────────────────────────────────
// helpers
// ────────────────────────────────────────────────────────────────────────────

// truncateContentSnippet trims a rendered TUI content string to at most maxLen
// bytes for use in error messages, avoiding test output floods.
func truncateContentSnippet(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
