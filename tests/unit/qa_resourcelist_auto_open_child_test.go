package unit

// qa_resourcelist_auto_open_child_test.go — reveal test for the s3
// auto-open-single bug.
//
// Bug: when a related-panel pivot resolves to exactly one target row
// (e.g. "Access Log Bucket (1)" for s3), `autoOpenSingleDetail` fires on
// ResourcesLoadedMsg and emits NavigateMsg{TargetDetail} regardless of
// whether the target type normally opens a CHILD view on Enter. For s3,
// Enter on the list is wired to the `s3_objects` child view — so the
// auto-skip hijacks the operator past the objects entry point and drops
// them on the bucket detail instead. The operator lost access to the
// very thing they pivoted for (the log files in the destination bucket).
//
// Contract: auto-open-single must mirror what Enter would do on the
// target list. If the type registers a child under Key="enter" (without
// a DrillCondition that vetoes this row), the auto-open must dispatch
// EnterChildViewMsg, not NavigateMsg{TargetDetail}.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// TestResourceList_AutoOpenSingle_PrefersEnterChildOverDetail asserts that
// when the type has Children[Key="enter"] and a single-row filter resolves
// after related-nav, the auto-open dispatches EnterChildViewMsg with the
// child type and resolved context — not NavigateMsg{TargetDetail}.
func TestResourceList_AutoOpenSingle_PrefersEnterChildOverDetail(t *testing.T) {
	k := keys.Default()
	td := resource.ResourceTypeDef{
		Name:      "S3 Buckets",
		ShortName: "s3",
		Columns: []resource.Column{
			{Key: "name", Title: "Bucket Name", Width: 40},
		},
		Children: []resource.ChildViewDef{{
			ChildType:      "s3_objects",
			Key:            "enter",
			ContextKeys:    map[string]string{"bucket": "ID"},
			DisplayNameKey: "bucket",
		}},
	}
	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, 20)
	m, _ = m.Init()

	targetBucket := "a9s-demo-logs"
	m.SetRelatedIDFilter([]string{targetBucket})
	m.SetAutoOpenSingleDetail(true)

	matching := []resource.Resource{
		{ID: targetBucket, Name: targetBucket, Fields: map[string]string{"name": targetBucket}},
	}
	_, got := m.Update(messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    matching,
	})
	if got == nil {
		t.Fatal("expected a cmd after single-row auto-open; got nil")
	}
	msg := got()

	// The auto-skip must land on the Enter-child view, not the generic
	// detail — otherwise pivoting via the related panel strands the
	// operator on bucket metadata when they were headed for bucket contents.
	enterChild, ok := msg.(messages.EnterChildView)
	if !ok {
		t.Fatalf("expected EnterChildViewMsg for a type with Children[Key=\"enter\"]; got %T: %+v",
			msg, msg)
	}
	if enterChild.ChildType != "s3_objects" {
		t.Errorf("ChildType = %q, want %q", enterChild.ChildType, "s3_objects")
	}
	if enterChild.ParentContext["bucket"] != targetBucket {
		t.Errorf("ParentContext[bucket] = %q, want %q",
			enterChild.ParentContext["bucket"], targetBucket)
	}
}

// TestResourceList_AutoOpenSingle_NoEnterChild_StillOpensDetail guards the
// existing behavior for types that do NOT register an Enter-child (e.g.
// ebs, kms) — auto-open must still go to the generic detail view.
func TestResourceList_AutoOpenSingle_NoEnterChild_StillOpensDetail(t *testing.T) {
	k := keys.Default()
	td := resource.ResourceTypeDef{
		Name:      "EBS Volumes",
		ShortName: "ebs",
		Columns: []resource.Column{
			{Key: "volume_id", Title: "Volume ID", Width: 20},
		},
		// No Children — the default path remains the generic detail view.
	}
	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, 20)
	m, _ = m.Init()

	m.SetRelatedIDFilter([]string{"vol-123"})
	m.SetAutoOpenSingleDetail(true)

	matching := []resource.Resource{
		{ID: "vol-123", Fields: map[string]string{"volume_id": "vol-123"}},
	}
	_, got := m.Update(messages.ResourcesLoaded{
		ResourceType: "ebs",
		Resources:    matching,
	})
	if got == nil {
		t.Fatal("expected a cmd after single-row auto-open; got nil")
	}
	msg := got()

	nav, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("expected NavigateMsg (no Enter-child registered); got %T: %+v", msg, msg)
	}
	if nav.Target != messages.TargetDetail {
		t.Errorf("Target = %v, want TargetDetail", nav.Target)
	}
}
