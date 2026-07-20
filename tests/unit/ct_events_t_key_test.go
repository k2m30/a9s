package unit

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ═══════════════════════════════════════════════════════════════════════════
// "t" key navigation tests — issue #247
// ═══════════════════════════════════════════════════════════════════════════

// ctEventsEC2Resource returns a test EC2 resource with an ARN field.
func ctEventsEC2Resource() resource.Resource {
	return resource.Resource{
		ID:   "i-test",
		Name: "test-instance",
		Fields: map[string]string{
			"arn": "arn:aws:ec2:us-east-1:000000000000:instance/i-test",
		},
	}
}

// ctEventsLoadedEC2List returns a ResourceListModel for "ec2" with one
// resource that has Fields["arn"] populated.
func ctEventsLoadedEC2List(t *testing.T) views.ResourceListModel {
	t.Helper()
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("resource type 'ec2' not registered")
	}
	k := keys.Default()
	m := views.NewResourceList(*td, nil, k)
	m.SetSize(80, 24)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    []resource.Resource{ctEventsEC2Resource()},
	})
	return m
}

// TestResourceList_TKey_EmitsRelatedNavigateMsg verifies that pressing "t"
// on a ResourceListModel emits a RelatedNavigateMsg with TargetType "ct-events"
// and a FetchFilter containing "ResourceName" keyed to the ARN.
func TestResourceList_TKey_EmitsRelatedNavigateMsg(t *testing.T) {
	m := ctEventsLoadedEC2List(t)

	_, cmd := m.Update(tea.KeyPressMsg{Code: -1, Text: "t"})
	if cmd == nil {
		t.Fatal("pressing 't' on a loaded ResourceListModel must return a non-nil cmd")
	}

	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigate)
	if !ok {
		t.Fatalf("pressing 't' must emit RelatedNavigateMsg; got %T", msg)
	}
	if nav.TargetType != "ct-events" {
		t.Errorf("RelatedNavigateMsg.TargetType = %q, want %q", nav.TargetType, "ct-events")
	}
	// EC2 CloudTrailKey is "ResourceName:ID" — filter uses res.ID, not Fields["arn"]
	wantID := "i-test"
	if nav.FetchFilter["ResourceName"] != wantID {
		t.Errorf("FetchFilter[ResourceName] = %q, want %q", nav.FetchFilter["ResourceName"], wantID)
	}
}

// TestResourceList_TKey_NoopWhenEmpty verifies that pressing "t" when the
// resource list is empty returns nil (no-op).
func TestResourceList_TKey_NoopWhenEmpty(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("resource type 'ec2' not registered")
	}
	k := keys.Default()
	m := views.NewResourceList(*td, nil, k)
	m.SetSize(80, 24)
	m, _ = m.Init()

	_, cmd := m.Update(tea.KeyPressMsg{Code: -1, Text: "t"})
	if cmd != nil {
		t.Errorf("pressing 't' on empty list must return nil cmd, got non-nil")
	}
}

// TestResourceList_TKey_IAMUser_UsesUsername verifies that pressing "t" on an
// IAM user resource emits a RelatedNavigateMsg with FetchFilter["Username"].
func TestResourceList_TKey_IAMUser_UsesUsername(t *testing.T) {
	td := resource.FindResourceType("iam-user")
	if td == nil {
		t.Fatal("resource type 'iam-user' not registered")
	}
	k := keys.Default()
	m := views.NewResourceList(*td, nil, k)
	m.SetSize(80, 24)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: "iam-user",
		Resources: []resource.Resource{
			{
				ID:   "test-user",
				Name: "test-user",
				Fields: map[string]string{
					"user_name": "test-user",
				},
			},
		},
	})

	_, cmd := m.Update(tea.KeyPressMsg{Code: -1, Text: "t"})
	if cmd == nil {
		t.Fatal("pressing 't' on IAM user must return a non-nil cmd")
	}

	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigate)
	if !ok {
		t.Fatalf("pressing 't' on IAM user must emit RelatedNavigateMsg; got %T", msg)
	}
	if nav.FetchFilter["Username"] != "test-user" {
		t.Errorf("FetchFilter[Username] = %q, want %q", nav.FetchFilter["Username"], "test-user")
	}
}

// TestResourceList_TKey_NoopOnCtEventsList verifies that pressing "t" while
// viewing the ct-events list is a no-op (returns nil cmd).
func TestResourceList_TKey_NoopOnCtEventsList(t *testing.T) {
	td := resource.FindResourceType("ct-events")
	if td == nil {
		t.Fatal("ct-events type not found")
	}
	rl := views.NewResourceList(*td, nil, keys.Default())
	rl.SetSize(120, 40)
	// Load one event
	rl, _ = rl.Update(messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources: []resource.Resource{{
			ID:     "evt-001",
			Name:   "DescribeInstances",
			Fields: map[string]string{"event_name": "DescribeInstances"},
		}},
	})
	_, cmd := rl.Update(tea.KeyPressMsg{Code: -1, Text: "t"})
	if cmd != nil {
		t.Fatal("t key should be no-op on ct-events list")
	}
}

// TestTKey_WorksFromAllViews verifies that pressing "t" emits RelatedNavigateMsg
// with TargetType "ct-events" from every applicable view type.
// Regression guard: catches "t doesn't work on some screens."
func TestTKey_WorksFromAllViews(t *testing.T) {
	// Use realistic EC2 fields — no explicit "arn" field; t key must work via res.ID fallback.
	res := resource.Resource{
		ID:     "i-test",
		Name:   "test-instance",
		Fields: map[string]string{"instance_id": "i-test", "state": "running"},
	}

	k := keys.Default()

	t.Run("ResourceList", func(t *testing.T) {
		td := resource.FindResourceType("ec2")
		if td == nil {
			t.Fatal("ec2 type not found")
		}
		rl := views.NewResourceList(*td, nil, k)
		rl.SetSize(120, 40)
		rl, _ = rl.Update(messages.ResourcesLoaded{
			ResourceType: "ec2",
			Resources:    []resource.Resource{res},
		})
		_, cmd := rl.Update(tea.KeyPressMsg{Code: -1, Text: "t"})
		if cmd == nil {
			t.Fatal("ResourceList: t key returned nil cmd")
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigate)
		if !ok {
			t.Fatalf("ResourceList: expected RelatedNavigateMsg, got %T", msg)
		}
		if nav.TargetType != "ct-events" {
			t.Errorf("ResourceList: expected ct-events, got %s", nav.TargetType)
		}
	})

	// Detail_LeftCol / Detail_RightColFocused subtests (dead views.DetailModel
	// Update/View, 022-codebase-cleanup wave 3) removed — no port needed:
	// Test_ActionCloudTrail_Detail_DispatchesCtEventsFetchFiltered
	// (detail_ports_test.go) already pins the same contract (ActionCloudTrail
	// on a detail screen dispatches a KindFetchFiltered task scoped to
	// "ct-events") more precisely, against the live controller Apply path,
	// independent of left/right-column focus (a renderer-only concern the
	// controller-level action does not branch on).

	// The YAML case (views.NewYAML+Update, both DEAD per
	// specs/022-codebase-cleanup/wave3-map-text.md) is retired here: it's
	// ported onto the live text-screen seam as
	// text_ports_test.go's TestPort_YAML_TKey_LiveCTEventsNavigate.
}

// TestResourceList_TKey_SuppressedOnChildList verifies that on a child
// resource list (parentContext != nil), pressing "t" is a no-op. The
// corresponding footer-hint suppression is pinned separately by
// list_ports_test.go's TestWave3ListFooterHints_CloudTrailTKey_
// GatedByParentContext (dead ResourceListModel.BottomHints() removed here).
func TestResourceList_TKey_SuppressedOnChildList(t *testing.T) {
	td := resource.GetChildType("s3_objects")
	if td == nil {
		t.Skip("s3_objects child type not registered")
	}
	rl := views.NewChildResourceList(*td, map[string]string{"bucket": "my-bucket"}, "my-bucket", nil, keys.Default())

	// Key should be no-op
	rl.SetSize(120, 40)
	rl, _ = rl.Update(messages.ResourcesLoaded{
		ResourceType: "s3_objects",
		Resources: []resource.Resource{{
			ID: "file.txt", Name: "file.txt", Fields: map[string]string{},
		}},
	})
	_, cmd := rl.Update(tea.KeyPressMsg{Code: -1, Text: "t"})
	if cmd != nil {
		t.Fatal("t key should be no-op on child resource list")
	}
}

func TestMainMenu_TKey_Noop(t *testing.T) {
	k := keys.Default()
	m := views.NewMainMenu(k)
	m.SetSize(120, 40)
	_, cmd := m.Update(tea.KeyPressMsg{Code: -1, Text: "t"})
	if cmd != nil {
		t.Fatal("t key should be no-op on main menu")
	}
}

// TestBottomHints_MainMenu_NoCloudTrail (dead views.MainMenuModel.BottomHints)
// removed — no live-seam port needed: the menu's live footer-hint source,
// app.MenuFooterHintsFor (core/app/viewstate.go), is a fixed two-entry
// literal ("ctrl+z", "ctrl+r") that can never contain a "t" hint, so the
// no-CloudTrail-hint-on-menu invariant is now structurally guaranteed rather
// than a runtime behavior worth pinning. See app_footer_hints_test.go for
// MenuFooterHintsFor's live coverage.
