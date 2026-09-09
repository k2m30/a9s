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
	"sync"
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
// the "single source of truth for both renderers" contract.
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

// wave4AssertTextScreenCopy pushes screenID with lines via the blessed
// newTestController helper (see qa_controller_construction_discipline_test.go
// — a direct app.New here isn't callable from this package's
// newTextScreenController equivalent, which lives in package unit) and
// asserts CopyContent()'s content is the ANSI-stripped, newline-joined body
// with the screen-exact label (BodyKindText covers ScreenYAML, ScreenJSON,
// and ScreenErrorLog — copyContentText, core/app/copy.go — so the label must
// vary by id, not stay hardcoded to "YAML").
func wave4AssertTextScreenCopy(t *testing.T, screenID runtime.ScreenID, lines []string, wantLabel string) {
	t.Helper()
	hasANSI := false
	for _, l := range lines {
		if strings.Contains(l, "\x1b[") {
			hasANSI = true
			break
		}
	}
	if !hasANSI {
		t.Fatal("precondition: ContentLines() must contain ANSI color codes for this test to be meaningful")
	}

	c := newTestController(t)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: screenID}})
	c.EnsureTextState(lines)

	content, label := wave4AssertCopyContent(t, c)
	if strings.Contains(content, "\x1b[") {
		t.Errorf("CopyContent() content must have ANSI codes stripped, got: %q", content)
	}
	wantPlain := stripAnsi(strings.Join(lines, "\n"))
	if content != wantPlain {
		t.Errorf("CopyContent() content mismatch:\ngot:\n%q\nwant:\n%q", content, wantPlain)
	}
	if label != wantLabel {
		t.Errorf("CopyContent() label = %q, want %q", label, wantLabel)
	}
}

func wave4CopyTextResource() resource.Resource {
	return resource.Resource{
		ID:   "i-0copytext001",
		Name: "copy-text-server",
		Fields: map[string]string{
			"state":         "running",
			"instance_type": "t3.medium",
		},
	}
}

func TestCopyContent_Text_StripsANSI_JoinsLines(t *testing.T) {
	m := views.NewYAMLWithCtrl(wave4CopyTextResource(), "ec2", keys.Default(), nil)
	m.SetSize(120, 40)
	wave4AssertTextScreenCopy(t, runtime.ScreenYAML, m.ContentLines(), "Copied YAML to clipboard")
}

func TestCopyContent_JSON_StripsANSI_JoinsLines(t *testing.T) {
	m := views.NewJSONWithCtrl(wave4CopyTextResource(), "ec2", keys.Default(), nil)
	m.SetSize(120, 40)
	wave4AssertTextScreenCopy(t, runtime.ScreenJSON, m.ContentLines(), "Copied JSON to clipboard")
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

// TestCopyContent_Identity_ClearedAfterRotation: the controller caches the
// resolved identity in c.identityResult (set via SetIdentityIntent,
// core/app/intents.go, from a messages.IdentityLoaded delivery).
// handleActionSelectProfile/handleActionSelectRegion rotate the session
// (which clears Session.Identity) and must also reset
// identityLoading/identityResult/identityErrMsg, or CopyContent() on an
// already-open identity screen keeps serving the PREVIOUS profile's ARN
// across a profile/region switch until the refetch eventually lands.
//
// HandleProfileSelected's own intents are MenuClearAvailabilityIntent,
// PopSelectorIntent, and a FlashIntent — PopSelectorIntent (core/app/
// intents.go) only pops the top screen when it is a profile/region/theme
// SELECTOR, never ScreenIdentity, so the identity screen legitimately stays
// on top of the stack across ActionSelectProfile (confirmed by the
// Body.Kind precondition below) — no re-open/re-navigate is needed to
// reproduce this on the real seam.
func TestCopyContent_Identity_ClearedAfterRotation(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionOpenIdentity})

	wantARN := "arn:aws:iam::123456789012:user/copy-test-preswitch-operator"
	c.Handle(messages.IdentityLoaded{
		Identity: &awsclient.CallerIdentity{
			AccountID: "123456789012",
			Arn:       wantARN,
			UserName:  "copy-test-preswitch-operator",
		},
		Gen: 0,
	})
	if content, _ := c.CopyContent(); content != wantARN {
		t.Fatalf("precondition: CopyContent() before rotation = %q, want the loaded ARN %q", content, wantARN)
	}

	c.Apply(app.Action{Kind: app.ActionSelectProfile, Arg: "other-profile"})

	if kind := c.Snapshot().Body.Kind; kind != app.BodyKindIdentity {
		t.Fatalf("precondition: identity screen must still be on top after ActionSelectProfile (PopSelectorIntent only pops a selector screen), got Body.Kind = %q", kind)
	}

	content, label := wave4AssertCopyContent(t, c)
	if content != "" || label != "" {
		t.Errorf("CopyContent() after a profile rotation must not serve the stale pre-switch ARN: got (%q, %q), want (\"\", \"\") until the refetch lands", content, label)
	}
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

// ===========================================================================
// 8. Concurrency — CopyContent() must be safe to call from multiple
// goroutines. CopyContent() (core/app/copy.go) takes only c.mu.RLock(), and
// its detail branch calls buildDetailBody -> buildDetailFieldItems, which —
// whenever ds.Resource.AttentionDetails is already a non-nil map — merges
// ds.AttentionDetails into it; that merge must not MUTATE a map shared
// across every call, the same class of builder-mutation snapshot.go's own
// doc comment says requires the WRITE lock ("buildListBody ... may populate
// ListState.bodyMemo on a cache miss"). Concurrent CopyContent() calls on a
// detail screen with pre-populated AttentionDetails must not race.
// ===========================================================================

// wave4ConcurrentCopyResource carries a non-nil, non-empty AttentionDetails
// map (set on the resource itself, not just delivered later via
// ApplyDetailFinding) so ensureDetailState seeds ds.Resource.AttentionDetails
// as a non-nil map AND ds.AttentionDetails (a clone) as non-empty — the exact
// precondition that makes buildDetailFieldItems's "if r.AttentionDetails ==
// nil { r.AttentionDetails = make(...) }" guard skip allocating a fresh map,
// so every call's maps.Copy writes into the SAME shared map instead.
func wave4ConcurrentCopyResource() resource.Resource {
	code := domain.FindingCode("concurrent.test.finding")
	return resource.Resource{
		ID:   "res-detail-004",
		Name: "detail-concurrent-test",
		AttentionDetails: map[domain.FindingCode]domain.AttentionDetail{
			code: {Rows: []domain.DetailRow{{Label: "Action", Value: "reboot"}}},
		},
	}
}

// TestCopyContent_ConcurrentCallsAreSerialized hammers CopyContent() from
// several goroutines on the same detail screen. Each goroutine writes only
// to its own disjoint slice range (no race in the harness itself) so any
// race reported by `go test -race` is exclusively inside CopyContent()'s own
// locking. Asserts nothing beyond "no panic and every call agrees" — the
// race detector, not this assertion, is the oracle for the locking bug.
func TestCopyContent_ConcurrentCallsAreSerialized(t *testing.T) {
	c := newDetailController(t, wave4ConcurrentCopyResource(), "copytest-detail-concurrent")

	const goroutines = 8
	const iterations = 50

	type copyResult struct{ content, label string }
	results := make([][]copyResult, goroutines)
	for g := range results {
		results[g] = make([]copyResult, iterations)
	}

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			for i := range iterations {
				content, label := c.CopyContent()
				results[g][i] = copyResult{content, label}
			}
		}(g)
	}
	wg.Wait()

	want := results[0][0]
	for g := range results {
		for i, got := range results[g] {
			if got != want {
				t.Errorf("CopyContent() result diverged across concurrent calls: goroutine %d iter %d = %+v, want %+v", g, i, got, want)
			}
		}
	}
}
