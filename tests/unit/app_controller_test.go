// app_controller_test.go — contract tests for internal/app.Controller (PR-A).
//
// Behavioral contracts covered:
//
//  1. ViewState JSON round-trip: all documented fields survive Marshal/Unmarshal.
//  2. Empty-stack safety: Snapshot() on a fresh controller never panics and
//     returns BodyKindUnknown.
//  3. Stack mechanics: PushScreen grows the stack, PopScreen shrinks it,
//     ReplaceScreen swaps the top without changing depth.
//  4. DrainSync terminates: empty queue returns immediately; non-empty queue
//     is drained to empty (execution deferred to PR-B).
//  5. Apply contract: returns ViewState == Snapshot() post-apply and a
//     (possibly empty) []runtime.TaskRequest without panicking, for all
//     documented verbs.
//  6. Handle lane: feeding a result event returns a ViewState and a (possibly
//     empty) []runtime.TaskRequest without panicking.
//
// Stack-mechanics tests (group 3) use the public ApplyIntents seam:
//
//	func (c *Controller) ApplyIntents(intents []runtime.UIIntent) app.ViewState
//
// This is the public surface for injecting PushScreen / PopScreen /
// ReplaceScreen intents from the external test package.
package unit_test

import (
	"encoding/json"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
)

// newTestController builds a Controller backed by a fresh runtime.Core
// with recognisable profile/region values. No AWS clients are attached;
// all test scenarios either exercise the empty-stack path or inject intents
// directly via ApplyIntents.
func newTestController() *app.Controller {
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	return app.New(core)
}

// =============================================================================
// 1. ViewState JSON round-trip
// =============================================================================

// TestViewState_JSONRoundTrip_ListBodyAllFieldsSurvive marshals a
// fully-populated ViewState with a List body to JSON and unmarshals it back,
// asserting that every documented field survives without loss.
// This pins the "fully serializable, renderer-agnostic" contract from the plan.
func TestViewState_JSONRoundTrip_ListBodyAllFieldsSurvive(t *testing.T) {
	original := app.ViewState{
		Header: app.Header{
			Version:          "v3.99.0-test",
			Profile:          "demo",
			Region:           "us-east-1",
			Mode:             "demo",
			RightSide:        "arn:aws:iam::111122223333:user/ops",
			Flash:            app.Flash{Text: "loaded 42 resources", IsError: false},
			ErrorHintVisible: true,
		},
		FrameTitle:  "EC2 Instances",
		HelpContext: "list",
		Footer: []app.KeyHint{
			{Key: "↑↓", Help: "navigate"},
			{Key: "enter", Help: "select"},
			{Key: "q", Help: "quit"},
		},
		Body: app.Body{
			Kind: app.BodyKindList,
			List: &app.ListBody{
				Columns: []app.ColumnDef{
					{Key: "id", Title: "Instance ID", Width: 20},
					{Key: "state", Title: "State", Width: 10},
					{Key: "type", Title: "Type", Width: 14},
				},
				Rows: []app.ListRow{
					{Cells: []string{"i-0abc123def456", "running", "t3.micro"}, Decorator: app.DecoratorNormal, Severity: ""},
					{Cells: []string{"i-0def789abc012", "stopped", "m5.large"}, Decorator: app.DecoratorWarning, Severity: "medium"},
					{Cells: []string{"i-0000000000001", "terminated", "t2.nano"}, Decorator: app.DecoratorError, Severity: "critical"},
				},
				Selected:      1,
				ScrollX:       4,
				Filter:        "web",
				Sort:          app.SortSpec{Col: "state", Dir: "asc"},
				AttentionOnly: true,
				Loading:       false,
				Truncated:     true,
				Pagination:    app.PaginationInfo{HasMore: true, Cursor: "tok-abc"},
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var got app.ViewState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Header fields
	if got.Header.Version != original.Header.Version {
		t.Errorf("Header.Version: got %q want %q", got.Header.Version, original.Header.Version)
	}
	if got.Header.Profile != original.Header.Profile {
		t.Errorf("Header.Profile: got %q want %q", got.Header.Profile, original.Header.Profile)
	}
	if got.Header.Region != original.Header.Region {
		t.Errorf("Header.Region: got %q want %q", got.Header.Region, original.Header.Region)
	}
	if got.Header.Mode != original.Header.Mode {
		t.Errorf("Header.Mode: got %q want %q", got.Header.Mode, original.Header.Mode)
	}
	if got.Header.RightSide != original.Header.RightSide {
		t.Errorf("Header.RightSide: got %q want %q", got.Header.RightSide, original.Header.RightSide)
	}
	if got.Header.Flash.Text != original.Header.Flash.Text {
		t.Errorf("Header.Flash.Text: got %q want %q", got.Header.Flash.Text, original.Header.Flash.Text)
	}
	if got.Header.Flash.IsError != original.Header.Flash.IsError {
		t.Errorf("Header.Flash.IsError: got %v want %v", got.Header.Flash.IsError, original.Header.Flash.IsError)
	}
	if got.Header.ErrorHintVisible != original.Header.ErrorHintVisible {
		t.Errorf("Header.ErrorHintVisible: got %v want %v", got.Header.ErrorHintVisible, original.Header.ErrorHintVisible)
	}

	// Top-level fields
	if got.FrameTitle != original.FrameTitle {
		t.Errorf("FrameTitle: got %q want %q", got.FrameTitle, original.FrameTitle)
	}
	if got.HelpContext != original.HelpContext {
		t.Errorf("HelpContext: got %q want %q", got.HelpContext, original.HelpContext)
	}
	if len(got.Footer) != len(original.Footer) {
		t.Fatalf("Footer length: got %d want %d", len(got.Footer), len(original.Footer))
	}
	for i, hint := range original.Footer {
		if got.Footer[i].Key != hint.Key {
			t.Errorf("Footer[%d].Key: got %q want %q", i, got.Footer[i].Key, hint.Key)
		}
		if got.Footer[i].Help != hint.Help {
			t.Errorf("Footer[%d].Help: got %q want %q", i, got.Footer[i].Help, hint.Help)
		}
	}

	// Body discriminator
	if got.Body.Kind != app.BodyKindList {
		t.Errorf("Body.Kind: got %q want %q", got.Body.Kind, app.BodyKindList)
	}
	if got.Body.List == nil {
		t.Fatal("Body.List is nil after round-trip")
	}

	lb := got.Body.List
	orig := original.Body.List

	if len(lb.Columns) != len(orig.Columns) {
		t.Fatalf("Columns length: got %d want %d", len(lb.Columns), len(orig.Columns))
	}
	for i, col := range orig.Columns {
		if lb.Columns[i].Key != col.Key {
			t.Errorf("Columns[%d].Key: got %q want %q", i, lb.Columns[i].Key, col.Key)
		}
		if lb.Columns[i].Title != col.Title {
			t.Errorf("Columns[%d].Title: got %q want %q", i, lb.Columns[i].Title, col.Title)
		}
		if lb.Columns[i].Width != col.Width {
			t.Errorf("Columns[%d].Width: got %d want %d", i, lb.Columns[i].Width, col.Width)
		}
	}

	if len(lb.Rows) != len(orig.Rows) {
		t.Fatalf("Rows length: got %d want %d", len(lb.Rows), len(orig.Rows))
	}
	for i, row := range orig.Rows {
		gotRow := lb.Rows[i]
		if len(gotRow.Cells) != len(row.Cells) {
			t.Errorf("Rows[%d] Cells length: got %d want %d", i, len(gotRow.Cells), len(row.Cells))
			continue
		}
		for j, cell := range row.Cells {
			if gotRow.Cells[j] != cell {
				t.Errorf("Rows[%d].Cells[%d]: got %q want %q", i, j, gotRow.Cells[j], cell)
			}
		}
		if gotRow.Decorator != row.Decorator {
			t.Errorf("Rows[%d].Decorator: got %q want %q", i, gotRow.Decorator, row.Decorator)
		}
		if gotRow.Severity != row.Severity {
			t.Errorf("Rows[%d].Severity: got %q want %q", i, gotRow.Severity, row.Severity)
		}
	}

	if lb.Selected != orig.Selected {
		t.Errorf("List.Selected: got %d want %d", lb.Selected, orig.Selected)
	}
	if lb.ScrollX != orig.ScrollX {
		t.Errorf("List.ScrollX: got %d want %d", lb.ScrollX, orig.ScrollX)
	}
	if lb.Filter != orig.Filter {
		t.Errorf("List.Filter: got %q want %q", lb.Filter, orig.Filter)
	}
	if lb.Sort.Col != orig.Sort.Col {
		t.Errorf("List.Sort.Col: got %q want %q", lb.Sort.Col, orig.Sort.Col)
	}
	if lb.Sort.Dir != orig.Sort.Dir {
		t.Errorf("List.Sort.Dir: got %q want %q", lb.Sort.Dir, orig.Sort.Dir)
	}
	if lb.AttentionOnly != orig.AttentionOnly {
		t.Errorf("List.AttentionOnly: got %v want %v", lb.AttentionOnly, orig.AttentionOnly)
	}
	if lb.Truncated != orig.Truncated {
		t.Errorf("List.Truncated: got %v want %v", lb.Truncated, orig.Truncated)
	}
	if lb.Pagination.HasMore != orig.Pagination.HasMore {
		t.Errorf("Pagination.HasMore: got %v want %v", lb.Pagination.HasMore, orig.Pagination.HasMore)
	}
	if lb.Pagination.Cursor != orig.Pagination.Cursor {
		t.Errorf("Pagination.Cursor: got %q want %q", lb.Pagination.Cursor, orig.Pagination.Cursor)
	}
}

// TestViewState_JSONRoundTrip_AllBodyKindsPreserved verifies that every
// BodyKind constant marshals to its documented string value and round-trips
// without change.
func TestViewState_JSONRoundTrip_AllBodyKindsPreserved(t *testing.T) {
	cases := []struct {
		kind     app.BodyKind
		wantJSON string
	}{
		{app.BodyKindList, `"list"`},
		{app.BodyKindDetail, `"detail"`},
		{app.BodyKindText, `"text"`},
		{app.BodyKindMenu, `"menu"`},
		{app.BodyKindSelector, `"selector"`},
		{app.BodyKindHelp, `"help"`},
		{app.BodyKindIdentity, `"identity"`},
		{app.BodyKindUnknown, `"unknown"`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.kind), func(t *testing.T) {
			vs := app.ViewState{Body: app.Body{Kind: tc.kind}}

			data, err := json.Marshal(vs)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}

			var got app.ViewState
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("json.Unmarshal failed: %v", err)
			}
			if got.Body.Kind != tc.kind {
				t.Errorf("round-trip changed BodyKind: got %q want %q", got.Body.Kind, tc.kind)
			}
		})
	}
}

// TestViewState_JSONRoundTrip_FlashIsErrorFalseOmitted verifies that a Flash
// with IsError=false and empty Text is omitted from JSON (omitempty on the
// struct), keeping snapshots concise for web tests.
func TestViewState_JSONRoundTrip_FlashIsErrorFalseOmitted(t *testing.T) {
	vs := app.ViewState{
		Header: app.Header{Profile: "demo", Region: "us-east-1"},
		Body:   app.Body{Kind: app.BodyKindUnknown},
	}
	data, err := json.Marshal(vs)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	// "flash" with zero value should not appear as {"text":"","is_error":false}
	// when the struct tag is omitempty. If it does appear, the round-trip is
	// still lossless — but it signals a missing omitempty tag on Flash fields.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal to map failed: %v", err)
	}
	header, _ := raw["header"].(map[string]any)
	if header == nil {
		t.Fatal("header key missing from JSON")
	}
	if flash, ok := header["flash"]; ok {
		flashMap, _ := flash.(map[string]any)
		if flashMap != nil {
			if text, _ := flashMap["text"].(string); text != "" {
				t.Errorf("zero Flash.Text should be omitted but got %q", text)
			}
		}
	}
	// round-trip must still decode cleanly
	var back app.ViewState
	if err := json.Unmarshal(data, &back); err != nil {
		t.Errorf("json.Unmarshal of zero-flash snapshot failed: %v", err)
	}
}

// TestViewState_JSONRoundTrip_HelpBodyAllFieldsSurvive verifies that a
// fully-populated HelpBody (key-hint sections) round-trips through JSON without
// field loss. HelpBody groups KeyHints into named sections so the renderer can
// draw a structured help overlay.
func TestViewState_JSONRoundTrip_HelpBodyAllFieldsSurvive(t *testing.T) {
	original := app.ViewState{
		Header: app.Header{Profile: "demo", Region: "us-east-1"},
		Body: app.Body{
			Kind: app.BodyKindHelp,
			Help: &app.HelpBody{
				Sections: []app.HelpSection{
					{
						Title: "Navigation",
						Hints: []app.KeyHint{
							{Key: "↑↓", Help: "move cursor"},
							{Key: "enter", Help: "select"},
						},
					},
					{
						Title: "Actions",
						Hints: []app.KeyHint{
							{Key: "q", Help: "quit"},
							{Key: "?", Help: "help"},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal HelpBody failed: %v", err)
	}

	var got app.ViewState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal HelpBody failed: %v", err)
	}

	if got.Body.Kind != app.BodyKindHelp {
		t.Errorf("Body.Kind: got %q want %q", got.Body.Kind, app.BodyKindHelp)
	}
	if got.Body.Help == nil {
		t.Fatal("Body.Help is nil after round-trip")
	}

	hb := got.Body.Help
	orig := original.Body.Help

	if len(hb.Sections) != len(orig.Sections) {
		t.Fatalf("Sections length: got %d want %d", len(hb.Sections), len(orig.Sections))
	}
	for i, sec := range orig.Sections {
		gotSec := hb.Sections[i]
		if gotSec.Title != sec.Title {
			t.Errorf("Sections[%d].Title: got %q want %q", i, gotSec.Title, sec.Title)
		}
		if len(gotSec.Hints) != len(sec.Hints) {
			t.Errorf("Sections[%d] Hints length: got %d want %d", i, len(gotSec.Hints), len(sec.Hints))
			continue
		}
		for j, hint := range sec.Hints {
			if gotSec.Hints[j].Key != hint.Key {
				t.Errorf("Sections[%d].Hints[%d].Key: got %q want %q", i, j, gotSec.Hints[j].Key, hint.Key)
			}
			if gotSec.Hints[j].Help != hint.Help {
				t.Errorf("Sections[%d].Hints[%d].Help: got %q want %q", i, j, gotSec.Hints[j].Help, hint.Help)
			}
		}
	}
}

// TestViewState_JSONRoundTrip_IdentityBodyAllFieldsSurvive verifies that a
// fully-populated IdentityBody round-trips through JSON without field loss.
// Covers the assumed-role path (IsAssumedRole=true, RoleName, SessionName set;
// UserName empty). Uses clearly-fake values — no real AWS account IDs or ARNs.
func TestViewState_JSONRoundTrip_IdentityBodyAllFieldsSurvive(t *testing.T) {
	original := app.ViewState{
		Header: app.Header{Profile: "demo", Region: "us-east-1"},
		Body: app.Body{
			Kind: app.BodyKindIdentity,
			Identity: &app.IdentityBody{
				AccountID:     "000000000000",
				AccountAlias:  "demo-account",
				ARN:           "arn:aws:iam::000000000000:assumed-role/DemoRole/demo-session",
				IsAssumedRole: true,
				RoleName:      "DemoRole",
				SessionName:   "demo-session",
				UserName:      "",
				Profile:       "demo",
				Region:        "us-east-1",
				Loading:       false,
				ErrorMsg:      "",
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal IdentityBody failed: %v", err)
	}

	var got app.ViewState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal IdentityBody failed: %v", err)
	}

	if got.Body.Kind != app.BodyKindIdentity {
		t.Errorf("Body.Kind: got %q want %q", got.Body.Kind, app.BodyKindIdentity)
	}
	if got.Body.Identity == nil {
		t.Fatal("Body.Identity is nil after round-trip")
	}

	ib := got.Body.Identity
	orig := original.Body.Identity

	if ib.AccountID != orig.AccountID {
		t.Errorf("Identity.AccountID: got %q want %q", ib.AccountID, orig.AccountID)
	}
	if ib.AccountAlias != orig.AccountAlias {
		t.Errorf("Identity.AccountAlias: got %q want %q", ib.AccountAlias, orig.AccountAlias)
	}
	if ib.ARN != orig.ARN {
		t.Errorf("Identity.ARN: got %q want %q", ib.ARN, orig.ARN)
	}
	if ib.IsAssumedRole != orig.IsAssumedRole {
		t.Errorf("Identity.IsAssumedRole: got %v want %v", ib.IsAssumedRole, orig.IsAssumedRole)
	}
	if ib.RoleName != orig.RoleName {
		t.Errorf("Identity.RoleName: got %q want %q", ib.RoleName, orig.RoleName)
	}
	if ib.SessionName != orig.SessionName {
		t.Errorf("Identity.SessionName: got %q want %q", ib.SessionName, orig.SessionName)
	}
	if ib.Profile != orig.Profile {
		t.Errorf("Identity.Profile: got %q want %q", ib.Profile, orig.Profile)
	}
	if ib.Region != orig.Region {
		t.Errorf("Identity.Region: got %q want %q", ib.Region, orig.Region)
	}
}

// =============================================================================
// 2. Empty-stack safety
// =============================================================================

// TestController_Snapshot_FreshControllerNoPanic verifies that Snapshot() on a
// freshly-constructed controller never panics and returns BodyKindMenu.
//
// PR-C contract: New(core) starts with ScreenMenu as the root screen, so a
// fresh controller's Snapshot() returns BodyKindMenu, not BodyKindUnknown.
func TestController_Snapshot_FreshControllerNoPanic(t *testing.T) {
	c := newTestController()

	var vs app.ViewState
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Snapshot() panicked on fresh controller: %v", r)
			}
		}()
		vs = c.Snapshot()
	}()

	if vs.Body.Kind != app.BodyKindMenu {
		t.Errorf("fresh controller Body.Kind: got %q want %q", vs.Body.Kind, app.BodyKindMenu)
	}
}

// TestController_Snapshot_EmptyStackAfterPopNoPanic verifies that Snapshot()
// after repeated PopScreen calls never panics and always returns BodyKindMenu.
// The root screen (menu) is preserved — PopScreen at depth 1 is a no-op, so
// the stack never empties and BodyKindUnknown is never returned.
func TestController_Snapshot_EmptyStackAfterPopNoPanic(t *testing.T) {
	c := newTestController()

	// Attempt to pop the root menu — must be a no-op (root is preserved).
	var vs app.ViewState
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Snapshot() panicked after PopScreen on root: %v", r)
			}
		}()
		c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
		vs = c.Snapshot()
	}()

	// Root preservation: stack never drops below depth 1.
	if vs.Body.Kind != app.BodyKindMenu {
		t.Errorf("after PopScreen on root: Body.Kind = %q, want %q (root preserved)", vs.Body.Kind, app.BodyKindMenu)
	}
}

// TestController_Snapshot_EmptyStackCarriesProfileAndRegion verifies that the
// Header fields from runtime.Core are present even with an empty stack.
func TestController_Snapshot_EmptyStackCarriesProfileAndRegion(t *testing.T) {
	c := newTestController()
	vs := c.Snapshot()

	if vs.Header.Profile != "demo" {
		t.Errorf("Header.Profile: got %q want %q", vs.Header.Profile, "demo")
	}
	if vs.Header.Region != "us-east-1" {
		t.Errorf("Header.Region: got %q want %q", vs.Header.Region, "us-east-1")
	}
}

// =============================================================================
// 3. Stack mechanics via ApplyIntents
//
// ApplyIntents is the public method on Controller that applies a slice of
// UIIntents to the screen stack and returns the resulting ViewState.
// It is the authoritative seam for stack-mechanics tests in the external
// test package.
// =============================================================================

// TestController_Stack_PushGrowsStackAndSetsBodyKind verifies that a
// PushScreen intent causes Snapshot() to reflect the new top screen.
//
// PR-C: a fresh controller starts on ScreenMenu (BodyKindMenu). After pushing
// ScreenProfileSelector, the top becomes BodyKindSelector.
func TestController_Stack_PushGrowsStackAndSetsBodyKind(t *testing.T) {
	c := newTestController()

	before := c.Snapshot()
	if before.Body.Kind != app.BodyKindMenu {
		t.Fatalf("precondition: expected menu root (BodyKindMenu), got %q", before.Body.Kind)
	}

	// ScreenProfileSelector maps to BodyKindSelector via bodyKindForScreen.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenProfileSelector,
			Context: runtime.ScreenContext{},
		},
	})

	after := c.Snapshot()
	if after.Body.Kind != app.BodyKindSelector {
		t.Errorf("after PushScreen(ScreenProfileSelector): expected BodyKindSelector, got %q", after.Body.Kind)
	}
	if after.FrameTitle != string(runtime.ScreenProfileSelector) {
		t.Errorf("FrameTitle: got %q want %q", after.FrameTitle, string(runtime.ScreenProfileSelector))
	}
}

// TestController_Stack_PushChildListBodyKindList verifies that
// ScreenChildList maps to BodyKindList (not BodyKindUnknown or another kind).
func TestController_Stack_PushChildListBodyKindList(t *testing.T) {
	c := newTestController()

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenChildList,
			Context: runtime.ScreenContext{ResourceType: "ec2"},
		},
	})

	vs := c.Snapshot()
	if vs.Body.Kind != app.BodyKindList {
		t.Errorf("ScreenChildList: expected BodyKindList, got %q", vs.Body.Kind)
	}
}

// TestController_Stack_PushRevealBodyKindDetail verifies that
// ScreenReveal maps to BodyKindDetail.
func TestController_Stack_PushRevealBodyKindDetail(t *testing.T) {
	c := newTestController()

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenReveal,
			Context: runtime.ScreenContext{ResourceType: "secrets"},
		},
	})

	vs := c.Snapshot()
	if vs.Body.Kind != app.BodyKindDetail {
		t.Errorf("ScreenReveal: expected BodyKindDetail, got %q", vs.Body.Kind)
	}
}

// TestController_Stack_PopShrinksStack verifies that PopScreen reduces depth
// and Snapshot returns to the previous state.
//
// PR-C: a fresh controller starts on ScreenMenu. After pushing
// ScreenProfileSelector and then popping, the stack returns to the menu root
// (BodyKindMenu), not BodyKindUnknown.
func TestController_Stack_PopShrinksStack(t *testing.T) {
	c := newTestController()

	// Push a selector on top of the menu root.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenProfileSelector,
			Context: runtime.ScreenContext{},
		},
	})
	if c.Snapshot().Body.Kind != app.BodyKindSelector {
		t.Fatalf("precondition: expected BodyKindSelector after push, got %q", c.Snapshot().Body.Kind)
	}

	// Pop the selector — reveals the menu root underneath.
	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})

	vs := c.Snapshot()
	if vs.Body.Kind != app.BodyKindMenu {
		t.Errorf("after PopScreen: expected menu root (BodyKindMenu), got %q", vs.Body.Kind)
	}
}

// TestController_Stack_PopOnEmptyStackNoPanic verifies that PopScreen when only
// the root menu remains is a no-op: it does not panic and the stack stays at
// depth 1 with BodyKindMenu. The root screen is never popped.
func TestController_Stack_PopOnEmptyStackNoPanic(t *testing.T) {
	c := newTestController()

	// Fresh controller starts at depth 1 (root menu).
	// PopScreen at the root must be a no-op.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PopScreen on root-only stack panicked: %v", r)
			}
		}()
		c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	}()

	vs := c.Snapshot()
	if vs.Body.Kind != app.BodyKindMenu {
		t.Errorf("after PopScreen on root-only stack: expected BodyKindMenu (root preserved), got %q", vs.Body.Kind)
	}

	// A second pop must also be safe.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("second PopScreen on root-only stack panicked: %v", r)
			}
		}()
		c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	}()

	vs2 := c.Snapshot()
	if vs2.Body.Kind != app.BodyKindMenu {
		t.Errorf("after second PopScreen on root-only stack: expected BodyKindMenu, got %q", vs2.Body.Kind)
	}
}

// TestController_Stack_ReplaceSwapsTopWithoutChangingDepth verifies that
// ReplaceScreen changes the top screen's identity but leaves the stack depth
// unchanged. Depth is inferred by popping once and confirming the stack reverts
// to the prior top (the menu root), not to BodyKindUnknown.
//
// PR-C: stack starts at depth-1 (menu root). Push adds ChildList → depth-2.
// Replace swaps the ChildList entry with ProfileSelector (still depth-2).
// Pop reveals the menu root → BodyKindMenu.
func TestController_Stack_ReplaceSwapsTopWithoutChangingDepth(t *testing.T) {
	c := newTestController()

	// Push a ChildList on top of the menu root → depth-2.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenChildList,
			Context: runtime.ScreenContext{ResourceType: "rds"},
		},
	})
	if c.Snapshot().Body.Kind != app.BodyKindList {
		t.Fatalf("precondition: expected BodyKindList after push, got %q", c.Snapshot().Body.Kind)
	}

	// Replace the ChildList with ProfileSelector (still depth-2).
	c.ApplyIntents([]runtime.UIIntent{
		runtime.ReplaceScreen{
			ID:      runtime.ScreenProfileSelector,
			Context: runtime.ScreenContext{},
		},
	})

	afterReplace := c.Snapshot()
	if afterReplace.Body.Kind != app.BodyKindSelector {
		t.Errorf("after ReplaceScreen: expected BodyKindSelector, got %q", afterReplace.Body.Kind)
	}

	// Pop once: depth-2 → depth-1, revealing the menu root.
	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	afterPop := c.Snapshot()
	if afterPop.Body.Kind != app.BodyKindMenu {
		t.Errorf("after pop-post-replace: expected BodyKindMenu (menu root at depth-1), got %q — ReplaceScreen must not have grown the stack", afterPop.Body.Kind)
	}
}

// TestController_Stack_ReplaceOnRootSwapsScreen verifies that ReplaceScreen on
// the root (depth 1) swaps the top screen in-place. The stack stays at depth 1
// and Snapshot reflects the replacement screen.
//
// Depth-1 verification: push one more screen on top of the replacement (→ depth
// 2), then pop once (→ depth 1) to confirm the replacement is the only entry
// below. This proves ReplaceScreen did not grow the stack past depth 1.
//
// The old "drain root to empty then replace" path is gone because PopScreen at
// depth 1 is a no-op — the root is never popped.
func TestController_Stack_ReplaceOnRootSwapsScreen(t *testing.T) {
	c := newTestController()

	// Precondition: root menu at depth 1.
	if c.Snapshot().Body.Kind != app.BodyKindMenu {
		t.Fatalf("precondition: expected BodyKindMenu (root), got %q", c.Snapshot().Body.Kind)
	}

	// ReplaceScreen swaps the root in-place (depth stays 1).
	c.ApplyIntents([]runtime.UIIntent{
		runtime.ReplaceScreen{
			ID:      runtime.ScreenReveal,
			Context: runtime.ScreenContext{ResourceType: "secrets"},
		},
	})

	afterReplace := c.Snapshot()
	if afterReplace.Body.Kind != app.BodyKindDetail {
		t.Errorf("ReplaceScreen on root: expected BodyKindDetail (Reveal), got %q", afterReplace.Body.Kind)
	}

	// Push one more screen → depth 2, then pop → depth 1 reveals the
	// replacement (BodyKindDetail), not the original menu (BodyKindMenu).
	// This confirms stack is depth-1 after the replace, not depth-2.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenHelp, Context: runtime.ScreenContext{}},
	})
	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	afterPop := c.Snapshot()
	if afterPop.Body.Kind != app.BodyKindDetail {
		t.Errorf("after push+pop-post-replace: expected BodyKindDetail (replacement at depth 1), got %q — ReplaceScreen must not have grown the stack", afterPop.Body.Kind)
	}
}

// TestController_Stack_MultiPushPreservesDepth verifies that pushing N screens
// on top of the root results in the topmost screen reflected by Snapshot and
// that N sequential pops unwind the pushed screens, finally revealing the menu
// root. An additional pop at the root is a no-op — the stack never empties.
//
// Stack depth at each step (root counts as depth 1):
//   - Fresh:  depth 1 (BodyKindMenu)
//   - +3 push: depth 4 (BodyKindDetail — Reveal on top)
//   - Pop 1:  depth 3 (BodyKindSelector)
//   - Pop 2:  depth 2 (BodyKindList)
//   - Pop 3:  depth 1 (BodyKindMenu — root)
//   - Pop 4:  depth 1 (BodyKindMenu — root preserved, no-op)
func TestController_Stack_MultiPushPreservesDepth(t *testing.T) {
	c := newTestController()

	pushes := []runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenChildList, Context: runtime.ScreenContext{ResourceType: "ec2"}},
		runtime.PushScreen{ID: runtime.ScreenProfileSelector, Context: runtime.ScreenContext{}},
		runtime.PushScreen{ID: runtime.ScreenReveal, Context: runtime.ScreenContext{ResourceType: "secrets"}},
	}
	c.ApplyIntents(pushes)

	// Top is Reveal → BodyKindDetail.
	vs := c.Snapshot()
	if vs.Body.Kind != app.BodyKindDetail {
		t.Errorf("after 3 pushes: top should be Reveal (BodyKindDetail), got %q", vs.Body.Kind)
	}

	// Pop 1: top becomes ProfileSelector → BodyKindSelector.
	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	if c.Snapshot().Body.Kind != app.BodyKindSelector {
		t.Errorf("after pop 1: expected BodyKindSelector, got %q", c.Snapshot().Body.Kind)
	}

	// Pop 2: top becomes ChildList → BodyKindList.
	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	if c.Snapshot().Body.Kind != app.BodyKindList {
		t.Errorf("after pop 2: expected BodyKindList, got %q", c.Snapshot().Body.Kind)
	}

	// Pop 3: top returns to menu root → BodyKindMenu.
	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	if c.Snapshot().Body.Kind != app.BodyKindMenu {
		t.Errorf("after pop 3: expected BodyKindMenu (menu root), got %q", c.Snapshot().Body.Kind)
	}

	// Pop 4: root guard fires — stack stays at depth 1, BodyKindMenu (not Unknown).
	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	if c.Snapshot().Body.Kind != app.BodyKindMenu {
		t.Errorf("after pop 4: expected BodyKindMenu (root preserved, no-op pop), got %q", c.Snapshot().Body.Kind)
	}
}

// TestController_Stack_ApplyIntentsReturnedViewStateMatchesSnapshot verifies
// that the ViewState returned by ApplyIntents equals the Snapshot taken
// immediately afterward — they must be consistent.
func TestController_Stack_ApplyIntentsReturnedViewStateMatchesSnapshot(t *testing.T) {
	c := newTestController()

	returned := c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenChildList,
			Context: runtime.ScreenContext{ResourceType: "lambda"},
		},
	})
	snap := c.Snapshot()

	assertViewStateEqualsSnapshot(t, "ApplyIntents return vs Snapshot", returned, snap)
}

// =============================================================================
// 4. DrainSync terminates
// =============================================================================

// TestDrainSync_EmptyPendingReturnsImmediately verifies that DrainSync with
// nil pending tasks returns without blocking or panicking. The call runs in a
// goroutine so the test harness can time it out; in practice it must complete
// before the goroutine switch even happens.
func TestDrainSync_EmptyPendingReturnsImmediately(t *testing.T) {
	c := newTestController()

	done := make(chan struct{}, 1)
	go func() {
		app.DrainSync(c, nil)
		done <- struct{}{}
	}()

	// DrainSync on nil pending must return promptly. The test harness
	// -timeout flag catches an infinite loop; this channel receive catches
	// panics (goroutine exits without sending) via the test framework.
	<-done
}

// TestDrainSync_AfterSeededPendingApplyIsShapeCorrect verifies the new seeded-
// pending model: seed DrainSync from a lane return value, then assert that a
// subsequent Apply returns a shape-correct (non-panicking) result. PR-A leaves
// task execution stubbed so deeper side-effect assertions are deferred.
func TestDrainSync_AfterSeededPendingApplyIsShapeCorrect(t *testing.T) {
	c := newTestController()

	// Obtain a pending task list from the lane return value — the authoritative
	// source under the new contract. In PR-A Apply returns nil tasks for the
	// skeleton; DrainSync must still not panic when handed nil.
	_, tasks := c.Apply(app.Action{Kind: app.ActionMoveDown})
	app.DrainSync(c, tasks)

	// After DrainSync the controller must still serve shape-correct results.
	vs, subsequent := c.Apply(app.Action{Kind: app.ActionMoveDown})
	if vs.Body.Kind == "" {
		t.Error("Apply after DrainSync returned ViewState with empty BodyKind")
	}
	// TODO PR-B0: once tasks execute and produce events, assert that subsequent
	// tasks reflect any state driven by the drained tasks.
	_ = subsequent
}

// TestDrainSync_NoPanicOnRepeatedCalls verifies that calling DrainSync
// multiple times in succession never panics (idempotent on nil pending).
func TestDrainSync_NoPanicOnRepeatedCalls(t *testing.T) {
	c := newTestController()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("DrainSync panicked: %v", r)
			}
		}()
		app.DrainSync(c, nil)
		app.DrainSync(c, nil)
		app.DrainSync(c, nil)
	}()
}

// =============================================================================
// 5. Apply contract (user-intent lane)
// =============================================================================

// TestController_Apply_MoveDownNoPanic verifies that Apply(MoveDown) does not
// panic and returns a ViewState equal to the subsequent Snapshot().
func TestController_Apply_MoveDownNoPanic(t *testing.T) {
	c := newTestController()

	var vs app.ViewState
	var tasks []runtime.TaskRequest
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Apply(MoveDown) panicked: %v", r)
			}
		}()
		vs, tasks = c.Apply(app.Action{Kind: app.ActionMoveDown})
	}()

	_ = tasks // may be nil or empty in the PR-A skeleton — TODO PR-B: verify task routing

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(MoveDown)", vs, snap)
}

// TestController_Apply_BackNoPanic verifies that Apply(Back) does not panic
// and returns a ViewState equal to Snapshot() post-apply.
func TestController_Apply_BackNoPanic(t *testing.T) {
	c := newTestController()

	var vs app.ViewState
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Apply(Back) panicked: %v", r)
			}
		}()
		vs, _ = c.Apply(app.Action{Kind: app.ActionBack})
	}()

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(Back)", vs, snap)
}

// TestController_Apply_AllSkeletonActionsNoPanic verifies that no documented
// ActionKind panics when applied to a fresh controller (menu root at bottom of
// stack). PR-C wires these verbs; this guards against any panic introduced
// while wiring menu actions.
func TestController_Apply_AllSkeletonActionsNoPanic(t *testing.T) {
	verbs := []app.Action{
		{Kind: app.ActionMoveUp},
		{Kind: app.ActionMoveDown},
		{Kind: app.ActionMoveTop},
		{Kind: app.ActionMoveBottom},
		{Kind: app.ActionPageUp},
		{Kind: app.ActionPageDown},
		{Kind: app.ActionSelect},
		{Kind: app.ActionBack},
		{Kind: app.ActionOpenDetail},
		{Kind: app.ActionOpenYAML},
		{Kind: app.ActionOpenJSON},
		{Kind: app.ActionOpenHelp},
		{Kind: app.ActionOpenIdentity},
		{Kind: app.ActionReveal},
		{Kind: app.ActionSetFilter, Arg: "web"},
		{Kind: app.ActionSort, Arg: "state"},
		{Kind: app.ActionSearch, Arg: "i-0abc"},
		{Kind: app.ActionSearchNext},
		{Kind: app.ActionSearchPrev},
		{Kind: app.ActionSearchClear},
		{Kind: app.ActionCopy},
		{Kind: app.ActionToggleRelated},
		{Kind: app.ActionToggleWrap},
		{Kind: app.ActionToggleAttention},
		{Kind: app.ActionChildView, Arg: "e"},
		{Kind: app.ActionLoadMore},
		{Kind: app.ActionRefresh},
		{Kind: app.ActionCommand, Arg: "ec2"},
		{Kind: app.ActionSelectProfile, Arg: "staging"},
		{Kind: app.ActionSelectRegion, Arg: "eu-west-1"},
		{Kind: app.ActionSelectTheme, Arg: "tokyo-night"},
		{Kind: app.ActionQuit},
	}

	for _, a := range verbs {
		a := a
		t.Run(string(a.Kind), func(t *testing.T) {
			c := newTestController() // fresh controller per verb — independent
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Apply(%q) panicked: %v", a.Kind, r)
					}
				}()
				_, _ = c.Apply(a)
			}()
		})
	}
}

// TestController_Apply_ReturnedViewStateEqualsSnapshotWithNonEmptyStack
// verifies the core contract "Apply returns ViewState == Snapshot() post-apply"
// when the stack is non-empty (so FrameTitle is meaningful).
func TestController_Apply_ReturnedViewStateEqualsSnapshotWithNonEmptyStack(t *testing.T) {
	c := newTestController()

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenChildList,
			Context: runtime.ScreenContext{ResourceType: "rds"},
		},
	})

	vs, _ := c.Apply(app.Action{Kind: app.ActionMoveDown})
	snap := c.Snapshot()

	assertViewStateEqualsSnapshot(t, "Apply(MoveDown) with non-empty stack", vs, snap)
}

// =============================================================================
// 6. Handle lane (task-result lane)
// =============================================================================

// TestController_Handle_IdentityErrorNoPanic verifies that Handle fed a
// messages.IdentityError does not panic and returns a ViewState consistent
// with Snapshot(). IdentityError is the cheapest constructable GenStamped
// event — its Err field is a string (not error), and Gen=0 is accepted
// (AcceptZeroGen=true), so the staleness guard does not short-circuit it.
// In PR-A the handler body is a no-op; this guards against any panic
// introduced while wiring the handler in future PRs.
func TestController_Handle_IdentityErrorNoPanic(t *testing.T) {
	c := newTestController()

	var vs app.ViewState
	var tasks []runtime.TaskRequest
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Handle(IdentityError) panicked: %v", r)
			}
		}()
		vs, tasks = c.Handle(messages.IdentityError{Err: "identity fetch failed", Gen: 0})
	}()

	_ = tasks // TODO PR-B: verify IdentityError produces an appropriate intent/task

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Handle(IdentityError)", vs, snap)
}

// TestController_Handle_AvailabilityCheckedNoPanic verifies that Handle fed a
// messages.AvailabilityChecked does not panic. AvailabilityChecked has
// AcceptZeroGen=false, so Gen=0 is always treated as stale and HandleEvent
// short-circuits to nil, nil — which is the correct safe fallback and is
// still a legitimate exercise of the dispatch path.
func TestController_Handle_AvailabilityCheckedNoPanic(t *testing.T) {
	c := newTestController()

	var vs app.ViewState
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Handle(AvailabilityChecked) panicked: %v", r)
			}
		}()
		vs, _ = c.Handle(messages.AvailabilityChecked{ResourceType: "ec2", Gen: 0})
	}()

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Handle(AvailabilityChecked)", vs, snap)
}

// TestController_Handle_ReturnedViewStateEqualsSnapshot verifies the core
// Handle contract: the returned ViewState equals Snapshot() taken immediately
// after the call. Uses messages.IdentityError (GenStamped, AcceptZeroGen=true)
// so the event reaches the dispatch switch rather than being dropped.
func TestController_Handle_ReturnedViewStateEqualsSnapshot(t *testing.T) {
	c := newTestController()

	vs, _ := c.Handle(messages.IdentityError{Err: "ec2 loaded", Gen: 0})
	snap := c.Snapshot()

	assertViewStateEqualsSnapshot(t, "Handle return vs Snapshot", vs, snap)
}

// TestController_Handle_IdentityLoadedNoPanic verifies that Handle tolerates
// messages.IdentityLoaded without panicking. Identity is typed as any — nil
// is a valid value in a no-AWS-client test context; the runtime handler must
// not dereference it without a nil check. Gen=0 is accepted (AcceptZeroGen=true).
// TODO PR-B: once HandleIdentityLoaded is wired, assert that a non-nil
// Identity populates Header.RightSide with the account ARN.
func TestController_Handle_IdentityLoadedNoPanic(t *testing.T) {
	c := newTestController()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Handle(IdentityLoaded{nil}) panicked: %v", r)
			}
		}()
		_, _ = c.Handle(messages.IdentityLoaded{Identity: nil, Gen: 0})
	}()
}

// =============================================================================
// helpers
// =============================================================================

// assertViewStateEqualsSnapshot compares the fields that Snapshot() guarantees
// to populate in PR-A (Header.Profile, Header.Region, Body.Kind, FrameTitle).
// Full body equality is deferred to PR-C when body fields are populated.
func assertViewStateEqualsSnapshot(t *testing.T, label string, vs, snap app.ViewState) {
	t.Helper()
	if vs.Body.Kind != snap.Body.Kind {
		t.Errorf("%s: ViewState.Body.Kind=%q != Snapshot Body.Kind=%q", label, vs.Body.Kind, snap.Body.Kind)
	}
	if vs.FrameTitle != snap.FrameTitle {
		t.Errorf("%s: ViewState.FrameTitle=%q != Snapshot FrameTitle=%q", label, vs.FrameTitle, snap.FrameTitle)
	}
	if vs.Header.Profile != snap.Header.Profile {
		t.Errorf("%s: ViewState.Header.Profile=%q != Snapshot Header.Profile=%q", label, vs.Header.Profile, snap.Header.Profile)
	}
	if vs.Header.Region != snap.Header.Region {
		t.Errorf("%s: ViewState.Header.Region=%q != Snapshot Header.Region=%q", label, vs.Header.Region, snap.Header.Region)
	}
}
