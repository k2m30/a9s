package unit

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// A partial FetchByIDs response keeps the checker's declared Count and
// lazy-adds only the resources that resolved.
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
				return resource.KnownRelated(targetType, []string{"id-001", "id-002", "id-003", "id-004", "id-005"}, false)
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
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

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	_, batchCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &resource.Resource{ID: "src-la020-001"},
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}

	if resultMsg.Result.Count() != 5 {
		t.Errorf("Result.Count: got %d, want 5 (checker count must not be revised by partial resolution)",
			resultMsg.Result.Count())
	}

	if len(resultMsg.Result.ResourceIDs()) != 5 {
		t.Errorf("Result.ResourceIDs(): got %d IDs, want 5", len(resultMsg.Result.ResourceIDs()))
	}

	lazy, ok := resultMsg.LazyAddedResources[targetType]
	if !ok {
		t.Fatalf("LazyAddedResources[%q] not present; want 3 resolved resources", targetType)
	}
	if len(lazy) != 3 {
		t.Errorf("LazyAddedResources[%q]: got %d resources, want 3", targetType, len(lazy))
	}
}

func Test_LA_021_FullResolutionFailure_Placeholder(t *testing.T) {
	t.Skip("covered by TestLazyAdd_FetchByIDsErrorSwallowed_ChecksResultStillDelivered " +
		"in lazy_add_orchestration_edges_test.go")
}

func Test_LA_022_ListAliasesDenied_Placeholder(t *testing.T) {
	t.Skip("covered by TestFetchKMSKeysByIDs_ListAliasesFailure_ProceedsWithoutAliases " +
		"in aws_kms_fetch_by_ids_test.go")
}

func Test_LA_023_DescribeKeyDeniedForOne_Placeholder(t *testing.T) {
	t.Skip("covered by TestFetchKMSKeysByIDs_DescribeKeyFailure_SkipsOneKey " +
		"in aws_kms_fetch_by_ids_test.go")
}

// When GetPolicy is denied the policy name is still derivable from the ARN;
// a lazy-added row with partial metadata is kept.
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
				return resource.KnownRelated(targetType, []string{policyARN}, false)
			},
		},
	})

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

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	_, batchCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &resource.Resource{ID: "src-la024-role-001"},
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}

	if resultMsg.Result.Count() != 1 {
		t.Errorf("Result.Count: got %d, want 1", resultMsg.Result.Count())
	}

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
	if row.Fields["attachment_count"] != "" {
		t.Errorf("attachment_count: got %q, want empty string (simulated GetPolicy deny)", row.Fields["attachment_count"])
	}
	if row.Fields["create_date"] != "" {
		t.Errorf("create_date: got %q, want empty string (simulated GetPolicy deny)", row.Fields["create_date"])
	}
}

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
				return resource.KnownRelated(targetType, ids, false)
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

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	_, batchCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &resource.Resource{ID: "src-la060-001"},
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}

	if resultMsg.Result.Count() != 7 {
		t.Errorf("Result.Count: got %d, want 7", resultMsg.Result.Count())
	}
	if len(resultMsg.Result.ResourceIDs()) != 7 {
		t.Errorf("len(Result.ResourceIDs()): got %d, want 7", len(resultMsg.Result.ResourceIDs()))
	}
	lazy, ok := resultMsg.LazyAddedResources[targetType]
	if !ok {
		t.Fatalf("LazyAddedResources[%q] not present; FetchByIDs was supposed to return 7 resources", targetType)
	}
	if len(lazy) != 7 {
		t.Errorf("len(LazyAddedResources[%q]): got %d, want 7", targetType, len(lazy))
	}
}

// A related drill whose IDs all resolve from a truncated cache shows no
// "load more" footer: upstream truncation does not apply to a fully-resolved
// ID filter.
func Test_LA_061_FooterSuppressed_WhenAllRelatedIDsResolved(t *testing.T) {
	const targetType = "ec2" // real registered type so FindResourceType works

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	ids := []string{"i-la061-001", "i-la061-002", "i-la061-003"}
	resources := []resource.Resource{
		{ID: "i-la061-001", Name: "instance-001"},
		{ID: "i-la061-002", Name: "instance-002"},
		{ID: "i-la061-003", Name: "instance-003"},
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: targetType,
		Resources:    resources,
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "some-token"},
	})

	m, _ = rootApplyMsg(m, messages.RelatedNavigate{
		TargetType: targetType,
		RelatedIDs: ids,
		SourceResource: resource.Resource{
			ID:   "rds-la061-src",
			Name: "my-db",
		},
		SourceType: "rds",
	})

	content := rootViewContent(m)
	if strings.Contains(content, "load more") {
		t.Errorf("View contains 'load more' after fully-resolved RelatedIDs filter — "+
			"IsTruncated should have been stripped; content snippet: %q",
			truncateContentSnippet(content, 300))
	}
}

func Test_LA_062_FooterSuppressed_UpstreamTruncatedDrillResolved(t *testing.T) {
	const targetType = "kms" // real registered type

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	ids := []string{"kms-la062-001", "kms-la062-002"}
	resources := []resource.Resource{
		{ID: "kms-la062-001", Name: "alias/first-key"},
		{ID: "kms-la062-002", Name: "alias/second-key"},
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: targetType,
		Resources:    resources,
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "truncated-token"},
	})

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

func Test_LA_063_SingleIDAutoOpen_OCQ(t *testing.T) {
	t.Skip("OCQ#5 — spec does not pin single-ID auto-open behavior; " +
		"a9s may open the lone target's detail directly or land on a one-row list. " +
		"Pin observed behavior once the design question is resolved.")
}

func Test_LA_064_CountHonestyUnderPartialFailure_OCQ(t *testing.T) {
	t.Skip("OCQ#6 — spec does not pin whether the related panel count should " +
		"reflect 'emitted by checker' or 'resolvable right now'. " +
		"These diverge when permission errors partially block FetchByIDs. " +
		"Pin once the design question is resolved. " +
		"Partial behavior for the resolvable subset is covered by Test_LA_020.")
}

// truncateContentSnippet trims a rendered TUI content string to at most maxLen
// bytes for use in error messages, avoiding test output floods.
func truncateContentSnippet(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
