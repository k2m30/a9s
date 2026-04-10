package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ═══════════════════════════════════════════════════════════════════════════
// "t" key navigation tests — issue #247
// ═══════════════════════════════════════════════════════════════════════════

// ctEventsEC2Resource returns a test EC2 resource with an ARN field.
func ctEventsEC2Resource() resource.Resource {
	return resource.Resource{
		ID:     "i-test",
		Name:   "test-instance",
		Status: "running",
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
	m, _ = m.Update(messages.ResourcesLoadedMsg{
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
	nav, ok := msg.(messages.RelatedNavigateMsg)
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
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "iam-user",
		Resources: []resource.Resource{
			{
				ID:     "test-user",
				Name:   "test-user",
				Status: "active",
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
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("pressing 't' on IAM user must emit RelatedNavigateMsg; got %T", msg)
	}
	if nav.FetchFilter["Username"] != "test-user" {
		t.Errorf("FetchFilter[Username] = %q, want %q", nav.FetchFilter["Username"], "test-user")
	}
}

// TestDetail_TKey_EmitsRelatedNavigateMsg verifies that pressing "t" in a
// DetailModel emits a RelatedNavigateMsg with TargetType "ct-events".
func TestDetail_TKey_EmitsRelatedNavigateMsg(t *testing.T) {
	res := ctEventsEC2Resource()
	m := views.NewDetail(res, "ec2", nil, keys.Default())
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyPressMsg{Code: -1, Text: "t"})
	if cmd == nil {
		t.Fatal("pressing 't' in DetailModel must return a non-nil cmd")
	}

	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("pressing 't' in DetailModel must emit RelatedNavigateMsg; got %T", msg)
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

// TestYAML_TKey_EmitsRelatedNavigateMsg verifies that pressing "t" in a
// YAMLModel emits a RelatedNavigateMsg with TargetType "ct-events".
func TestYAML_TKey_EmitsRelatedNavigateMsg(t *testing.T) {
	res := ctEventsEC2Resource()
	m := views.NewYAML(res, "ec2", keys.Default())
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyPressMsg{Code: -1, Text: "t"})
	if cmd == nil {
		t.Fatal("pressing 't' in YAMLModel must return a non-nil cmd")
	}

	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("pressing 't' in YAMLModel must emit RelatedNavigateMsg; got %T", msg)
	}
	if nav.TargetType != "ct-events" {
		t.Errorf("RelatedNavigateMsg.TargetType = %q, want %q", nav.TargetType, "ct-events")
	}
	// EC2 CloudTrailKey is "ResourceName:ID" — filter uses res.ID
	wantID := "i-test"
	if nav.FetchFilter["ResourceName"] != wantID {
		t.Errorf("FetchFilter[ResourceName] = %q, want %q", nav.FetchFilter["ResourceName"], wantID)
	}
}

// TestResourceList_TKey_NoHintOnCtEventsList verifies that the "t" (CloudTrail)
// hint does not appear in BottomHints() when the list is already showing ct-events.
func TestResourceList_TKey_NoHintOnCtEventsList(t *testing.T) {
	td := resource.FindResourceType("ct-events")
	if td == nil {
		t.Fatal("ct-events type not found")
	}
	rl := views.NewResourceList(*td, nil, keys.Default())
	hints := rl.BottomHints()
	for _, h := range hints {
		if h.Key == "t" {
			t.Fatal("t hint should not appear on ct-events list")
		}
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
	rl, _ = rl.Update(messages.ResourcesLoadedMsg{
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
		rl, _ = rl.Update(messages.ResourcesLoadedMsg{
			ResourceType: "ec2",
			Resources:    []resource.Resource{res},
		})
		_, cmd := rl.Update(tea.KeyPressMsg{Code: -1, Text: "t"})
		if cmd == nil {
			t.Fatal("ResourceList: t key returned nil cmd")
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("ResourceList: expected RelatedNavigateMsg, got %T", msg)
		}
		if nav.TargetType != "ct-events" {
			t.Errorf("ResourceList: expected ct-events, got %s", nav.TargetType)
		}
	})

	t.Run("Detail_LeftCol", func(t *testing.T) {
		d := views.NewDetail(res, "ec2", nil, k)
		d.SetSize(80, 40) // narrow — no right col
		_, cmd := d.Update(tea.KeyPressMsg{Code: -1, Text: "t"})
		if cmd == nil {
			t.Fatal("Detail (left col): t key returned nil cmd")
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("Detail (left col): expected RelatedNavigateMsg, got %T", msg)
		}
		if nav.TargetType != "ct-events" {
			t.Errorf("Detail (left col): expected ct-events, got %s", nav.TargetType)
		}
	})

	t.Run("Detail_RightColFocused", func(t *testing.T) {
		d := views.NewDetail(res, "ec2", nil, k)
		d.SetSize(120, 40)
		if !strings.Contains(d.View(), "RELATED") {
			t.Skip("right column not auto-shown at width=120; skipping right-col focus subtest")
		}
		// r → r: auto-shown → explicitly visible. Then Tab: focus right col.
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "r"})
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "r"})
		d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		_, cmd := d.Update(tea.KeyPressMsg{Code: -1, Text: "t"})
		if cmd == nil {
			t.Fatal("Detail (right col focused): t key returned nil cmd")
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("Detail (right col focused): expected RelatedNavigateMsg, got %T", msg)
		}
		if nav.TargetType != "ct-events" {
			t.Errorf("Detail (right col focused): expected ct-events, got %s", nav.TargetType)
		}
	})

	t.Run("YAML", func(t *testing.T) {
		y := views.NewYAML(res, "ec2", k)
		y.SetSize(80, 40)
		_, cmd := y.Update(tea.KeyPressMsg{Code: -1, Text: "t"})
		if cmd == nil {
			t.Fatal("YAML: t key returned nil cmd")
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigateMsg)
		if !ok {
			t.Fatalf("YAML: expected RelatedNavigateMsg, got %T", msg)
		}
		if nav.TargetType != "ct-events" {
			t.Errorf("YAML: expected ct-events, got %s", nav.TargetType)
		}
	})
}

func TestResourceList_TKey_SuppressedOnChildList(t *testing.T) {
	// Child lists (parentContext != nil) should suppress t key hint and treat key as no-op.
	td := resource.GetChildType("s3_objects")
	if td == nil {
		t.Skip("s3_objects child type not registered")
	}
	rl := views.NewChildResourceList(*td, map[string]string{"bucket": "my-bucket"}, "my-bucket", nil, keys.Default())

	// Hint should be absent
	hints := rl.BottomHints()
	for _, h := range hints {
		if h.Key == "t" {
			t.Fatal("t hint should not appear on child resource list")
		}
	}

	// Key should be no-op
	rl.SetSize(120, 40)
	rl, _ = rl.Update(messages.ResourcesLoadedMsg{
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

// TestYAML_TKey_SuppressedForChildType_ViaDetail verifies that when YAML is
// opened from a child detail view (e.g. dbi_events), the YAML view receives
// the child resourceType and suppresses the "t" key hint and action.
func TestYAML_TKey_SuppressedForChildType_ViaDetail(t *testing.T) {
	td := resource.GetChildType("dbi_events")
	if td == nil {
		t.Skip("dbi_events child type not registered")
	}
	res := resource.Resource{
		ID:     "evt-001",
		Name:   "CreateDBInstance",
		Fields: map[string]string{"event_name": "CreateDBInstance"},
	}
	y := views.NewYAML(res, "dbi_events", keys.Default())
	y.SetSize(80, 40)

	// t key should be no-op (child type suppressed)
	_, cmd := y.Update(tea.KeyPressMsg{Code: -1, Text: "t"})
	if cmd != nil {
		t.Fatal("t key should be suppressed in YAML for child resource type dbi_events")
	}

	// t hint should be absent
	hints := y.BottomHints()
	for _, h := range hints {
		if h.Key == "t" {
			t.Fatal("t hint should not appear in YAML for child resource type")
		}
	}
}

// ---------------------------------------------------------------------------
// Main menu: t key and hint suppression
// ---------------------------------------------------------------------------

// TestMainMenu_TKey_Noop verifies that pressing t on the main menu is a no-op.
func TestMainMenu_TKey_Noop(t *testing.T) {
	k := keys.Default()
	m := views.NewMainMenu(k)
	m.SetSize(120, 40)
	_, cmd := m.Update(tea.KeyPressMsg{Code: -1, Text: "t"})
	if cmd != nil {
		t.Fatal("t key should be no-op on main menu")
	}
}

// TestBottomHints_MainMenu_NoCloudTrail verifies that the t hint is absent from
// the main menu status bar (CloudTrail is only meaningful on resource views).
func TestBottomHints_MainMenu_NoCloudTrail(t *testing.T) {
	k := keys.Default()
	m := views.NewMainMenu(k)
	m.SetSize(120, 40)
	hints := m.BottomHints()
	for _, h := range hints {
		if h.Key == "t" {
			t.Fatal("t hint should not appear on main menu")
		}
	}
}

