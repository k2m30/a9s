// web_copy_content_test.go — pins for Controller.CopyContent(), the new
// single source of truth for copy-content resolution shared by the TUI and
// web renderers (fixes web mode's silent-no-op copy key). Mirrors the
// reference semantics of internal/tui/runtime_adapter_navigate.go's
// handleCopy (rsKindList/Detail/Text/Identity/Menu/Selector/Help/Costs), but
// asserts directly against the CONTROLLER method and the new
// ViewState.CopyText/CopyLabel snapshot fields — neither exists yet, so this
// file is expected to fail to compile until both land.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// wave4AssertCopyContent calls c.CopyContent() and cross-checks that
// Snapshot().CopyText/CopyLabel expose the exact same pair at that moment —
// the dispatch's "single source of truth for both renderers" contract.
// Every test in this file routes through this helper so the snapshot
// exposure is pinned across all 9 behaviors, not just once.
func wave4AssertCopyContent(t *testing.T, c *app.Controller) (content, label string) {
	t.Helper()
	content, label = c.CopyContent()
	snap := c.Snapshot()
	if snap.CopyText != content {
		t.Errorf("Snapshot().CopyText = %q, want %q (must match CopyContent())", snap.CopyText, content)
	}
	if snap.CopyLabel != label {
		t.Errorf("Snapshot().CopyLabel = %q, want %q (must match CopyContent())", snap.CopyLabel, label)
	}
	return content, label
}

// ===========================================================================
// 1. List screen — CopyField precedence.
//
// Every CopyField entry in the entire catalog is on a CHILD type (grepped
// core/aws/catalog_*.go: cb_builds, cb_build_logs, pipeline_stages,
// eb_rule_targets, sfn_executions, sfn_execution_history, sns_subscriptions,
// image_uri child, error_message child, message child, conditions_summary
// child, user_name child — zero top-level types set it). "sns_subscriptions"
// (CopyField: "endpoint") is used as the real CopyField-bearing type;
// resource.FindResourceType only searches the TOP-LEVEL catalog (confirmed
// by reading core/catalog/catalog.go's Find vs FindChild), so CopyContent
// must resolve CopyField via the child registry too for this to pass — a
// real bug this test is positioned to catch.
// ===========================================================================

func wave4SNSSubscriptionResource(id, endpoint string) resource.Resource {
	return resource.Resource{
		ID:   id,
		Name: "orders-topic-subscription",
		Fields: map[string]string{
			"protocol":            "https",
			"endpoint":            endpoint,
			"confirmation_status": "Confirmed",
			"owner":               "123456789012",
			"subscription_arn":    id,
			"topic_arn":           "arn:aws:sns:us-east-1:123456789012:orders-topic",
		},
	}
}

func TestCopyContent_List_UsesCopyField_WhenFieldNonEmpty(t *testing.T) {
	td := resource.GetChildType("sns_subscriptions")
	if td == nil {
		t.Fatal("sns_subscriptions child type not registered")
	}
	if td.CopyField != "endpoint" {
		t.Fatalf("precondition: sns_subscriptions.CopyField = %q, want %q — update this test if the catalog changed", td.CopyField, "endpoint")
	}

	id := "arn:aws:sns:us-east-1:123456789012:orders-topic:8f14e45f-ceea-467e-adc8-b71d1d09b1a1"
	endpoint := "https://hooks.example.com/notify/orders"
	c := wave3ChildListController(t, "sns_subscriptions")
	c.ApplyResourcesLoaded("sns_subscriptions", []resource.Resource{wave4SNSSubscriptionResource(id, endpoint)}, nil, false)

	content, label := wave4AssertCopyContent(t, c)
	if content != endpoint {
		t.Errorf("CopyContent() content = %q, want CopyField value %q", content, endpoint)
	}
	if label != "Copied: "+endpoint {
		t.Errorf("CopyContent() label = %q, want %q", label, "Copied: "+endpoint)
	}
}

func TestCopyContent_List_FallsBackToResourceID_WhenCopyFieldEmptyOnRow(t *testing.T) {
	td := resource.GetChildType("sns_subscriptions")
	if td == nil {
		t.Fatal("sns_subscriptions child type not registered")
	}

	id := "arn:aws:sns:us-east-1:123456789012:orders-topic:11111111-2222-3333-4444-555555555555"
	c := wave3ChildListController(t, "sns_subscriptions")
	c.ApplyResourcesLoaded("sns_subscriptions", []resource.Resource{wave4SNSSubscriptionResource(id, "")}, nil, false)

	content, label := wave4AssertCopyContent(t, c)
	if content != id {
		t.Errorf("CopyContent() content = %q, want resource ID %q (CopyField %q empty on this row)", content, id, td.CopyField)
	}
	if label != "Copied: "+id {
		t.Errorf("CopyContent() label = %q, want %q", label, "Copied: "+id)
	}
}

func TestCopyContent_List_FallsBackToResourceID_WhenTypeHasNoCopyField(t *testing.T) {
	if td := resource.FindResourceType("ec2"); td != nil && td.CopyField != "" {
		t.Fatalf("precondition: ec2.CopyField = %q, want empty — update this test if the catalog changed", td.CopyField)
	}

	id := "i-0a1b2c3d4e5f67890"
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", []resource.Resource{
		{
			ID:   id,
			Name: "prod-web-01",
			Fields: map[string]string{
				"instance_id":   id,
				"state":         "running",
				"instance_type": "m5.large",
				"private_ip":    "10.0.4.15",
			},
		},
	}, nil, false)

	content, label := wave4AssertCopyContent(t, c)
	if content != id {
		t.Errorf("CopyContent() content = %q, want resource ID %q (ec2 has no CopyField)", content, id)
	}
	if label != "Copied: "+id {
		t.Errorf("CopyContent() label = %q, want %q", label, "Copied: "+id)
	}
}

func TestCopyContent_List_EmptySelection_ReturnsEmpty(t *testing.T) {
	c := openListController(t, "ec2")
	// No ApplyResourcesLoaded call — list is empty, ListSelected() has nothing.

	content, label := wave4AssertCopyContent(t, c)
	if content != "" || label != "" {
		t.Errorf("CopyContent() on an empty list = (%q, %q), want (\"\", \"\")", content, label)
	}
}

// ===========================================================================
// 2. Detail screen — related panel focused.
// ===========================================================================

func TestCopyContent_Detail_RelatedFocused_UsesSelectedRowDisplayName(t *testing.T) {
	res := resource.Resource{
		ID:   "i-0abc123def456789a",
		Name: "prod-backend-01",
		Fields: map[string]string{
			"instance_id": "i-0abc123def456789a",
			"state":       "running",
		},
	}
	c := newDetailController(t, res, "ec2")
	wantName := "sg-0a1b2c3d4e5f67890"
	c.ApplyDetailRelated([]app.DetailRelatedRow{
		{TargetType: "sg", DisplayName: wantName, State: domain.RelatedResolved, Count: 2},
	})
	// ApplyDetailRelated only replaces RelatedRows; RelatedVisible is a
	// separate flag (mirrors DetailState.RelatedVisible) that ActionToggleFocus
	// gates on (core/app/detail_cursor.go) — set it via the same seam
	// detail_ports_test.go uses before exercising Tab focus.
	c.SetDetailRelatedVisible(true, true)
	c.Apply(app.Action{Kind: app.ActionToggleFocus})

	row, ok := c.SelectedRelatedRow()
	if !ok || row.DisplayName != wantName {
		t.Fatalf("precondition: SelectedRelatedRow() = (%+v, %v), want DisplayName %q selected", row, ok, wantName)
	}

	content, label := wave4AssertCopyContent(t, c)
	if content != wantName {
		t.Errorf("CopyContent() content = %q, want selected related row's DisplayName %q", content, wantName)
	}
	if label != "Copied: "+wantName {
		t.Errorf("CopyContent() label = %q, want %q", label, "Copied: "+wantName)
	}
}

// ===========================================================================
// 3. Detail screen — related NOT focused, FieldCursor on a field row.
// ===========================================================================

// TestCopyContent_Detail_FieldCursor_UsesFieldValue uses an unregistered
// synthetic resource type (no td.Project override) with EXACTLY ONE Fields
// entry so the generic flat-alphabetical projector produces a single,
// unambiguous field row at FieldCursor's default index 0.
func TestCopyContent_Detail_FieldCursor_UsesFieldValue(t *testing.T) {
	wantValue := "arn:aws:iam::123456789012:role/deploy-role"
	res := resource.Resource{
		ID:     "res-detail-001",
		Name:   "detail-field-cursor-test",
		Fields: map[string]string{"account_arn": wantValue},
	}
	c := newDetailController(t, res, "copytest-detail-fieldcursor")

	body := c.Snapshot().Body.Detail
	if body == nil || len(body.Fields) != 1 || body.Fields[0].Value != wantValue {
		t.Fatalf("precondition: expected exactly 1 detail field row with Value %q, got: %+v", wantValue, body)
	}

	content, label := wave4AssertCopyContent(t, c)
	if content != wantValue {
		t.Errorf("CopyContent() content = %q, want focused field's Value %q", content, wantValue)
	}
	if label != "Copied: "+wantValue {
		t.Errorf("CopyContent() label = %q, want %q", label, "Copied: "+wantValue)
	}
}

// TestCopyContent_Detail_FieldCursor_DashValue_CopiesDash pins the reachable
// reality of a Fields entry whose map value is empty: core/fieldpath (frozen)
// substitutes the display placeholder "-" for an empty value before the item
// ever reaches domainItemToFieldItemDetail, so no projected row's Value is
// ever the literal empty string — the projected row instead carries
// Value:"-", and the live handleCopy/CopyContent path copies that dash
// verbatim (TUI parity), not the Key.
func TestCopyContent_Detail_FieldCursor_DashValue_CopiesDash(t *testing.T) {
	res := resource.Resource{
		ID:     "res-detail-002",
		Name:   "detail-dash-value-test",
		Fields: map[string]string{"launch_time": ""},
	}
	c := newDetailController(t, res, "copytest-detail-dashvalue")

	body := c.Snapshot().Body.Detail
	if body == nil || len(body.Fields) != 1 || body.Fields[0].Value != "-" {
		t.Fatalf("precondition: expected exactly 1 detail field row with the fieldpath dash placeholder Value \"-\", got: %+v", body)
	}

	content, label := wave4AssertCopyContent(t, c)
	if content != "-" {
		t.Errorf("CopyContent() content = %q, want the projected dash placeholder %q", content, "-")
	}
	if label != "Copied: -" {
		t.Errorf("CopyContent() label = %q, want %q", label, "Copied: -")
	}
}

// ===========================================================================
// 4. Detail screen — fallback to raw YAML when no usable field is focused.
//
// An unregistered synthetic type with NO Fields map produces a genuinely
// EMPTY detail Fields slice regardless of RawStruct (confirmed by reading
// core/semantics/projection/generic.go's buildItems: with no view config the
// flat-rendering branch reads only r.Fields, never r.RawStruct, and the
// ID/Name-synthesis branch is skipped whenever RawStruct is non-nil) — the
// FieldCursor-in-range branch of the reference handleCopy logic can never
// fire, forcing the YAML fallback. RawStruct is a real SDK-typed fixture
// (core/demo/fixtures) so RawYAMLFromResource has real data to marshal.
// ===========================================================================

func TestCopyContent_Detail_FallbackToRawYAML_WhenNoUsableField(t *testing.T) {
	inst := fixtures.NewEC2Fixtures().Reservations[0].Instances[0]
	res := resource.Resource{ID: "res-detail-003", Name: "detail-yaml-fallback-test", RawStruct: inst}
	c := newDetailController(t, res, "copytest-detail-yamlfallback")

	body := c.Snapshot().Body.Detail
	if body == nil || len(body.Fields) != 0 {
		t.Fatalf("precondition: expected zero detail Fields rows for an unregistered type with no Fields/Findings, got %d: %+v", len(body.Fields), body)
	}
	wantYAML := views.RawYAMLFromResource(c.GetDetailResource())
	if wantYAML == "" {
		t.Fatal("precondition: views.RawYAMLFromResource produced empty output for a non-empty resource")
	}

	content, label := wave4AssertCopyContent(t, c)
	if content != wantYAML {
		t.Errorf("CopyContent() content does not match RawYAMLFromResource(GetDetailResource()):\ngot:\n%s\nwant:\n%s", content, wantYAML)
	}
	if label != "Copied detail to clipboard" {
		t.Errorf("CopyContent() label = %q, want %q", label, "Copied detail to clipboard")
	}
}

// ===========================================================================
// 5. Text screen (YAML/JSON) — ANSI-stripped, newline-joined content.
// ===========================================================================

func TestCopyContent_Text_StripsANSI_JoinsLines(t *testing.T) {
	res := resource.Resource{
		ID:   "i-0copytext001",
		Name: "copy-text-server",
		Fields: map[string]string{
			"state":         "running",
			"instance_type": "t3.medium",
		},
	}
	m := views.NewYAMLWithCtrl(res, "ec2", keys.Default(), nil)
	m.SetSize(120, 40)
	lines := m.ContentLines()

	hasANSI := false
	for _, l := range lines {
		if strings.Contains(l, "\x1b[") {
			hasANSI = true
			break
		}
	}
	if !hasANSI {
		t.Fatal("precondition: YAML ContentLines() must contain ANSI color codes for this test to be meaningful")
	}

	// Route construction through the blessed newTestController helper (see
	// qa_controller_construction_discipline_test.go) instead of a direct
	// app.New — it already pairs t.TempDir()/t.Cleanup(c.Close) correctly;
	// text_ports_test.go's newTextScreenController does the same
	// PushScreen+EnsureTextState arrangement but lives in package unit, not
	// unit_test, so it isn't callable from here.
	c := newTestController(t)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenYAML}})
	c.EnsureTextState(lines)

	content, label := wave4AssertCopyContent(t, c)
	if strings.Contains(content, "\x1b[") {
		t.Errorf("CopyContent() content must have ANSI codes stripped, got: %q", content)
	}
	wantPlain := stripAnsi(strings.Join(lines, "\n"))
	if content != wantPlain {
		t.Errorf("CopyContent() content mismatch:\ngot:\n%q\nwant:\n%q", content, wantPlain)
	}
	if label != "Copied YAML to clipboard" {
		t.Errorf("CopyContent() label = %q, want %q", label, "Copied YAML to clipboard")
	}
}

// ===========================================================================
// 6. Identity screen.
// ===========================================================================

func TestCopyContent_Identity_LoadedWithARN_ReturnsARNAndBangLabel(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionOpenIdentity})

	wantARN := "arn:aws:iam::123456789012:user/copy-test-operator"
	c.Handle(messages.IdentityLoaded{
		Identity: &awsclient.CallerIdentity{
			AccountID: "123456789012",
			Arn:       wantARN,
			UserName:  "copy-test-operator",
		},
		Gen: 0,
	})

	content, label := wave4AssertCopyContent(t, c)
	if content != wantARN {
		t.Errorf("CopyContent() content = %q, want identity ARN %q", content, wantARN)
	}
	if label != "Copied!" {
		t.Errorf("CopyContent() label = %q, want %q", label, "Copied!")
	}
}

func TestCopyContent_Identity_NoOp(t *testing.T) {
	t.Run("still loading (no IdentityLoaded delivered)", func(t *testing.T) {
		c := newTestController(t)
		c.Apply(app.Action{Kind: app.ActionOpenIdentity})

		if !c.Snapshot().Body.Identity.Loading {
			t.Fatal("precondition: identity screen must still be in the Loading state before any IdentityLoaded event")
		}

		content, label := wave4AssertCopyContent(t, c)
		if content != "" || label != "" {
			t.Errorf("CopyContent() while identity is loading = (%q, %q), want (\"\", \"\")", content, label)
		}
	})

	t.Run("loaded with empty ARN", func(t *testing.T) {
		c := newTestController(t)
		c.Apply(app.Action{Kind: app.ActionOpenIdentity})
		c.Handle(messages.IdentityLoaded{
			Identity: &awsclient.CallerIdentity{
				AccountID: "123456789012",
				Arn:       "",
				UserName:  "copy-test-operator-blank-arn",
			},
			Gen: 0,
		})

		body := c.Snapshot().Body.Identity
		if body == nil || body.Loading || body.ARN != "" {
			t.Fatalf("precondition: identity must be loaded with an empty ARN, got: %+v", body)
		}

		content, label := wave4AssertCopyContent(t, c)
		if content != "" || label != "" {
			t.Errorf("CopyContent() with an empty ARN = (%q, %q), want (\"\", \"\")", content, label)
		}
	})
}

// ===========================================================================
// 7. Menu / selector / help / costs screens — always a no-op.
// ===========================================================================

func TestCopyContent_NoOpScreens_ReturnEmpty(t *testing.T) {
	t.Run("menu", func(t *testing.T) {
		c := newTestController(t)
		if c.Snapshot().Body.Kind != app.BodyKindMenu {
			t.Fatalf("precondition: fresh controller must start on the menu screen, got Body.Kind = %q", c.Snapshot().Body.Kind)
		}
		content, label := wave4AssertCopyContent(t, c)
		if content != "" || label != "" {
			t.Errorf("CopyContent() on the menu screen = (%q, %q), want (\"\", \"\")", content, label)
		}
	})

	t.Run("selector", func(t *testing.T) {
		c := newTestController(t)
		c.Apply(app.Action{Kind: app.ActionCommand, Arg: "region"})
		if c.Snapshot().Body.Kind != app.BodyKindSelector {
			t.Fatalf("precondition: Command(region) must push a selector screen, got Body.Kind = %q", c.Snapshot().Body.Kind)
		}
		content, label := wave4AssertCopyContent(t, c)
		if content != "" || label != "" {
			t.Errorf("CopyContent() on the region selector screen = (%q, %q), want (\"\", \"\")", content, label)
		}
	})

	t.Run("help", func(t *testing.T) {
		c := newTestController(t)
		c.Apply(app.Action{Kind: app.ActionOpenHelp})
		if c.Snapshot().Body.Kind != app.BodyKindHelp {
			t.Fatalf("precondition: OpenHelp must push the help screen, got Body.Kind = %q", c.Snapshot().Body.Kind)
		}
		content, label := wave4AssertCopyContent(t, c)
		if content != "" || label != "" {
			t.Errorf("CopyContent() on the help screen = (%q, %q), want (\"\", \"\")", content, label)
		}
	})

	t.Run("costs", func(t *testing.T) {
		c := newTestController(t)
		c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
		if c.Snapshot().Body.Kind != app.BodyKindCosts {
			t.Fatalf("precondition: PushScreen(ScreenCosts) must report Body.Kind = %q, got %q", app.BodyKindCosts, c.Snapshot().Body.Kind)
		}
		content, label := wave4AssertCopyContent(t, c)
		if content != "" || label != "" {
			t.Errorf("CopyContent() on the costs screen = (%q, %q), want (\"\", \"\")", content, label)
		}
	})
}
