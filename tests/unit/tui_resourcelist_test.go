package unit

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ===========================================================================
// exactRelatedTargetID — indirect coverage via ResourcesLoadedMsg with
// pagination + autoOpenSingleDetail + relatedIDSet (single non-empty ID)
// ===========================================================================

func TestResourceListView_ExactRelatedTargetID_SingleID_TriggersLoadMore(t *testing.T) {
	// When: ResourceListModel has relatedIDSet = {"vol-target-123"} (exactly one non-empty ID),
	//       autoOpenSingleDetail = true, ResourcesLoadedMsg arrives with IsTruncated=true
	//       and no resources match the filter.
	// Then: exactRelatedTargetID returns ("vol-target-123", true) → LoadMoreMsg is emitted.
	k := keys.Default()
	td := resource.ResourceTypeDef{
		Name:      "EBS Volumes",
		ShortName: "ebs",
		Columns: []resource.Column{
			{Key: "volume_id", Title: "Volume ID", Width: 20},
		},
	}
	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, 20)
	m, _ = m.Init()

	m.SetRelatedIDFilter([]string{"vol-target-123"})
	m.SetAutoOpenSingleDetail(true)

	// Resources that do NOT match the filter (non-matching IDs)
	nonMatching := []resource.Resource{
		{ID: "vol-other-1", Fields: map[string]string{"volume_id": "vol-other-1"}},
		{ID: "vol-other-2", Fields: map[string]string{"volume_id": "vol-other-2"}},
	}
	var got tea.Cmd
	m, got = m.Update(messages.ResourcesLoaded{
		ResourceType: "ebs",
		Resources:    nonMatching,
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "token-page-2",
		},
	})

	if got == nil {
		t.Fatal("exactRelatedTargetID with single non-empty ID + truncated page should emit LoadMoreMsg cmd")
	}
	msg := got()
	loadMore, ok := msg.(messages.LoadMore)
	if !ok {
		t.Fatalf("expected LoadMoreMsg, got %T: %+v", msg, msg)
	}
	if loadMore.ResourceType != "ebs" {
		t.Errorf("LoadMoreMsg.ResourceType should be 'ebs'; got %q", loadMore.ResourceType)
	}
	if loadMore.ContinuationToken != "token-page-2" {
		t.Errorf("LoadMoreMsg.ContinuationToken should be 'token-page-2'; got %q", loadMore.ContinuationToken)
	}
}

func TestResourceListView_ExactRelatedTargetID_MultipleIDs_NoLoadMore(t *testing.T) {
	// When: relatedIDSet has 2 IDs, exactRelatedTargetID returns false → no LoadMoreMsg.
	k := keys.Default()
	td := resource.ResourceTypeDef{
		Name:      "EBS Volumes",
		ShortName: "ebs",
		Columns: []resource.Column{
			{Key: "volume_id", Title: "Volume ID", Width: 20},
		},
	}
	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, 20)
	m, _ = m.Init()

	m.SetRelatedIDFilter([]string{"vol-id-1", "vol-id-2"}) // 2 IDs — exactRelatedTargetID returns false
	m.SetAutoOpenSingleDetail(true)

	nonMatching := []resource.Resource{
		{ID: "vol-other-x", Fields: map[string]string{"volume_id": "vol-other-x"}},
	}
	var got tea.Cmd
	m, got = m.Update(messages.ResourcesLoaded{
		ResourceType: "ebs",
		Resources:    nonMatching,
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "token-page-2",
		},
	})

	if got != nil {
		msg := got()
		if _, isLoadMore := msg.(messages.LoadMore); isLoadMore {
			t.Error("exactRelatedTargetID with 2 IDs should NOT emit LoadMoreMsg (ambiguous target)")
		}
	}
}

func TestResourceListView_ExactRelatedTargetID_EmptyID_NoLoadMore(t *testing.T) {
	// When: relatedIDSet = {""} (one empty-string ID),
	//       exactRelatedTargetID returns ("", false) → no LoadMoreMsg.
	k := keys.Default()
	td := resource.ResourceTypeDef{
		Name:      "EBS Volumes",
		ShortName: "ebs",
		Columns: []resource.Column{
			{Key: "volume_id", Title: "Volume ID", Width: 20},
		},
	}
	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, 20)
	m, _ = m.Init()

	// SetRelatedIDFilter skips empty strings when building the set,
	// so the set ends up empty — exactRelatedTargetID returns false.
	m.SetRelatedIDFilter([]string{""})
	m.SetAutoOpenSingleDetail(true)

	nonMatching := []resource.Resource{
		{ID: "vol-other-y", Fields: map[string]string{"volume_id": "vol-other-y"}},
	}
	var got tea.Cmd
	m, got = m.Update(messages.ResourcesLoaded{
		ResourceType: "ebs",
		Resources:    nonMatching,
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "token-page-3",
		},
	})

	if got != nil {
		msg := got()
		if _, isLoadMore := msg.(messages.LoadMore); isLoadMore {
			t.Error("empty-string relatedID should NOT trigger LoadMoreMsg")
		}
	}
}

// ===========================================================================
// AS-140 — TestResourceListView_Wave2FieldUpdate_OverridesWave1FindingsPhrase
// DELETED.
//
// This test pinned the inverted priority where ApplyFieldUpdates overlaying
// Fields["status"] WINS over Findings[0].Phrase. AS-140 reverses that: the
// status column now reads phraseFromFindings(r.Findings) first (which
// aggregates Wave-1 + Wave-2 findings into a "<top> (+N)" phrase), then
// falls back to Fields[lifecycleKey]. Wave-2 enrichers no longer write
// Fields["status"] at all — both findings travel via r.Findings, and the
// renderer composes the stacked display.
//
// The new contract is covered by:
//   - tests/unit/aws_efs_issue_enrichment_test.go (Wave-1 + Wave-2 stacked
//     mount-target case — Findings populated, FieldUpdates empty).
//   - tests/unit/enrichment_rds_findings_test.go
//     (TestEnrichDBI_Wave1StoppedPlusWave2_StackedFindings_AS140).
//   - tests/unit/phase03_view_reads_test.go (extractCellValue reads Findings
//     for the list status column).
// ===========================================================================
