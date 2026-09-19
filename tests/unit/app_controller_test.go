package unit_test

import (
	"encoding/json"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// newTestController builds a Controller backed by a fresh runtime.Core
// with recognisable profile/region values. No AWS clients are attached;
// all test scenarios either exercise the empty-stack path or inject intents
// directly via ApplyIntents.
//
// Redirects A9S_CONFIG_FOLDER to a fresh t.TempDir() and registers
// t.Cleanup(c.Close) — in that order, so t.TempDir()'s own RemoveAll
// cleanup (registered by the FIRST TempDir() call in a test, per Go's
// testing package) runs AFTER Close via t.Cleanup's LIFO ordering. Any test
// in this file that drives a ResourcesLoaded/AvailabilityChecked event
// through the returned Controller can queue an async availability-cache
// save (queueAvailabilitySave); without this ordering the writer goroutine
// can still be running cache.Store.SaveType when RemoveAll fires and the
// directory is not empty when it is removed (see
// app_availsave_tempdir_cleanup_race_test.go for the mechanism).
func newTestController(t *testing.T) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	return c
}

// newTestControllerForProfile is newTestController for a test whose subject is
// the disk cache. The cache directory is keyed by the profile/region pair, so
// a test that saves a type file and reads it back needs a pair of its own or
// it reads a sibling subtest's rows. The registry is seeded with every
// resource type, since such a test opens a real list.
//
// Same TempDir-before-Close ordering as newTestController, and for the same
// reason — see its doc comment.
func newTestControllerForProfile(t *testing.T, profile, region string) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	c := newBlessedController(t, runtime.Bootstrap(profile, region, resource.AllResourceTypes()))
	t.Cleanup(c.Close)
	return c
}

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
					{Cells: []string{"i-0abc123def456", "running", "t3.micro"}, Severity: ""},
					{Cells: []string{"i-0def789abc012", "stopped", "m5.large"}, Severity: "medium"},
					{Cells: []string{"i-0000000000001", "terminated", "t2.nano"}, Severity: "critical"},
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
	var back app.ViewState
	if err := json.Unmarshal(data, &back); err != nil {
		t.Errorf("json.Unmarshal of zero-flash snapshot failed: %v", err)
	}
}

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

// New(core) starts with ScreenMenu as the root screen.
func TestController_Snapshot_FreshControllerNoPanic(t *testing.T) {
	c := newTestController(t)

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

// PopScreen at depth 1 is a no-op: the root menu is never popped.
func TestController_Snapshot_EmptyStackAfterPopNoPanic(t *testing.T) {
	c := newTestController(t)

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

	if vs.Body.Kind != app.BodyKindMenu {
		t.Errorf("after PopScreen on root: Body.Kind = %q, want %q (root preserved)", vs.Body.Kind, app.BodyKindMenu)
	}
}

func TestController_Snapshot_EmptyStackCarriesProfileAndRegion(t *testing.T) {
	c := newTestController(t)
	vs := c.Snapshot()

	if vs.Header.Profile != "demo" {
		t.Errorf("Header.Profile: got %q want %q", vs.Header.Profile, "demo")
	}
	if vs.Header.Region != "us-east-1" {
		t.Errorf("Header.Region: got %q want %q", vs.Header.Region, "us-east-1")
	}
}

func TestController_Stack_PushGrowsStackAndSetsBodyKind(t *testing.T) {
	c := newTestController(t)

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

func TestController_Stack_PushChildListBodyKindList(t *testing.T) {
	c := newTestController(t)

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

func TestController_Stack_PushRevealBodyKindDetail(t *testing.T) {
	c := newTestController(t)

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

func TestController_Stack_PopShrinksStack(t *testing.T) {
	c := newTestController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenProfileSelector,
			Context: runtime.ScreenContext{},
		},
	})
	if c.Snapshot().Body.Kind != app.BodyKindSelector {
		t.Fatalf("precondition: expected BodyKindSelector after push, got %q", c.Snapshot().Body.Kind)
	}

	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})

	vs := c.Snapshot()
	if vs.Body.Kind != app.BodyKindMenu {
		t.Errorf("after PopScreen: expected menu root (BodyKindMenu), got %q", vs.Body.Kind)
	}
}

func TestController_Stack_PopOnEmptyStackNoPanic(t *testing.T) {
	c := newTestController(t)

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

func TestController_Stack_ReplaceSwapsTopWithoutChangingDepth(t *testing.T) {
	c := newTestController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenChildList,
			Context: runtime.ScreenContext{ResourceType: "rds"},
		},
	})
	if c.Snapshot().Body.Kind != app.BodyKindList {
		t.Fatalf("precondition: expected BodyKindList after push, got %q", c.Snapshot().Body.Kind)
	}

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

	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	afterPop := c.Snapshot()
	if afterPop.Body.Kind != app.BodyKindMenu {
		t.Errorf("after pop-post-replace: expected BodyKindMenu (menu root at depth-1), got %q — ReplaceScreen must not have grown the stack", afterPop.Body.Kind)
	}
}

func TestController_Stack_ReplaceOnRootSwapsScreen(t *testing.T) {
	c := newTestController(t)

	if c.Snapshot().Body.Kind != app.BodyKindMenu {
		t.Fatalf("precondition: expected BodyKindMenu (root), got %q", c.Snapshot().Body.Kind)
	}

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

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenHelp, Context: runtime.ScreenContext{}},
	})
	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	afterPop := c.Snapshot()
	if afterPop.Body.Kind != app.BodyKindDetail {
		t.Errorf("after push+pop-post-replace: expected BodyKindDetail (replacement at depth 1), got %q — ReplaceScreen must not have grown the stack", afterPop.Body.Kind)
	}
}

func TestController_Stack_MultiPushPreservesDepth(t *testing.T) {
	c := newTestController(t)

	pushes := []runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenChildList, Context: runtime.ScreenContext{ResourceType: "ec2"}},
		runtime.PushScreen{ID: runtime.ScreenProfileSelector, Context: runtime.ScreenContext{}},
		runtime.PushScreen{ID: runtime.ScreenReveal, Context: runtime.ScreenContext{ResourceType: "secrets"}},
	}
	c.ApplyIntents(pushes)

	vs := c.Snapshot()
	if vs.Body.Kind != app.BodyKindDetail {
		t.Errorf("after 3 pushes: top should be Reveal (BodyKindDetail), got %q", vs.Body.Kind)
	}

	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	if c.Snapshot().Body.Kind != app.BodyKindSelector {
		t.Errorf("after pop 1: expected BodyKindSelector, got %q", c.Snapshot().Body.Kind)
	}

	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	if c.Snapshot().Body.Kind != app.BodyKindList {
		t.Errorf("after pop 2: expected BodyKindList, got %q", c.Snapshot().Body.Kind)
	}

	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	if c.Snapshot().Body.Kind != app.BodyKindMenu {
		t.Errorf("after pop 3: expected BodyKindMenu (menu root), got %q", c.Snapshot().Body.Kind)
	}

	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	if c.Snapshot().Body.Kind != app.BodyKindMenu {
		t.Errorf("after pop 4: expected BodyKindMenu (root preserved, no-op pop), got %q", c.Snapshot().Body.Kind)
	}
}

func TestController_Stack_ApplyIntentsReturnedViewStateMatchesSnapshot(t *testing.T) {
	c := newTestController(t)

	returned := c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenChildList,
			Context: runtime.ScreenContext{ResourceType: "lambda"},
		},
	})
	snap := c.Snapshot()

	assertViewStateEqualsSnapshot(t, "ApplyIntents return vs Snapshot", returned, snap)
}

// DrainSync runs in a goroutine so the test's -timeout catches a hang.
func TestDrainSync_EmptyPendingReturnsImmediately(t *testing.T) {
	c := newTestController(t)

	done := make(chan struct{}, 1)
	go func() {
		app.DrainSync(c, nil)
		done <- struct{}{}
	}()

	<-done
}

func TestDrainSync_AfterSeededPendingApplyIsShapeCorrect(t *testing.T) {
	c := newTestController(t)

	_, tasks := c.Apply(app.Action{Kind: app.ActionMoveDown})
	app.DrainSync(c, tasks)

	vs, subsequent := c.Apply(app.Action{Kind: app.ActionMoveDown})
	if vs.Body.Kind == "" {
		t.Error("Apply after DrainSync returned ViewState with empty BodyKind")
	}
	// MoveDown on a menu never enqueues background tasks regardless of prior DrainSync state.
	if len(subsequent) != 0 {
		t.Errorf("Apply(MoveDown) after DrainSync returned %d tasks, want 0", len(subsequent))
	}
}

func TestDrainSync_NoPanicOnRepeatedCalls(t *testing.T) {
	c := newTestController(t)

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

func TestController_Apply_MoveDownNoPanic(t *testing.T) {
	c := newTestController(t)

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

	// MoveDown on a menu produces no tasks (no background work triggered by cursor moves).
	if len(tasks) != 0 {
		t.Errorf("Apply(MoveDown) returned %d tasks, want 0 — cursor moves must not enqueue background tasks", len(tasks))
	}

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(MoveDown)", vs, snap)
}

func TestController_Apply_BackNoPanic(t *testing.T) {
	c := newTestController(t)

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
			c := newTestController(t) // fresh controller per verb — independent
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

func TestController_Apply_ReturnedViewStateEqualsSnapshotWithNonEmptyStack(t *testing.T) {
	c := newTestController(t)

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

// IdentityError is the cheapest constructable GenStamped event: its Err field
// is a string, and Gen=0 is accepted (AcceptZeroGen=true), so the staleness
// guard does not short-circuit it.
func TestController_Handle_IdentityErrorNoPanic(t *testing.T) {
	c := newTestController(t)

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

	// IdentityError is handled by Controller.Handle (sets identityErrMsg) and
	// produces no follow-up tasks — the result lane requires no background work.
	if len(tasks) != 0 {
		t.Errorf("Handle(IdentityError) returned %d tasks, want 0 — error events must not enqueue follow-up tasks", len(tasks))
	}

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Handle(IdentityError)", vs, snap)
}

// AvailabilityChecked has AcceptZeroGen=false, so Gen=0 is treated as stale and
// HandleEvent short-circuits to nil, nil.
func TestController_Handle_AvailabilityCheckedNoPanic(t *testing.T) {
	c := newTestController(t)

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

// messages.IdentityError (GenStamped, AcceptZeroGen=true) reaches the dispatch
// switch rather than being dropped.
func TestController_Handle_ReturnedViewStateEqualsSnapshot(t *testing.T) {
	c := newTestController(t)

	vs, _ := c.Handle(messages.IdentityError{Err: "ec2 loaded", Gen: 0})
	snap := c.Snapshot()

	assertViewStateEqualsSnapshot(t, "Handle return vs Snapshot", vs, snap)
}

// IdentityLoaded accepts Gen=0 (AcceptZeroGen=true).
func TestController_Handle_IdentityLoadedNoPanic(t *testing.T) {
	c := newTestController(t)

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Handle(IdentityLoaded{nil}) panicked: %v", r)
			}
		}()
		_, _ = c.Handle(messages.IdentityLoaded{Identity: nil, Gen: 0})
	}()

	// Push ScreenIdentity so Snapshot().Body.Identity is populated.
	_, _ = c.Apply(app.Action{Kind: app.ActionOpenIdentity})

	// IdentityLoaded.Identity is type-asserted to *awsclient.CallerIdentity in
	// Core.HandleIdentityLoaded; passing the domain mirror type yields nil/nil.
	wantARN := "arn:aws:iam::111122223333:user/test-operator"
	_, _ = c.Handle(messages.IdentityLoaded{
		Identity: &awsclient.CallerIdentity{
			AccountID: "111122223333",
			Arn:       wantARN,
			UserName:  "test-operator",
		},
		Gen: 0,
	})

	snap := c.Snapshot()
	if snap.Body.Identity == nil {
		t.Fatal("Snapshot().Body.Identity is nil after Handle(IdentityLoaded) with ScreenIdentity on stack")
	}
	if snap.Body.Identity.ARN != wantARN {
		t.Errorf("Snapshot().Body.Identity.ARN = %q, want %q", snap.Body.Identity.ARN, wantARN)
	}
	if snap.Body.Identity.AccountID != "111122223333" {
		t.Errorf("Snapshot().Body.Identity.AccountID = %q, want %q", snap.Body.Identity.AccountID, "111122223333")
	}
}

// (*Controller).consoleURL (core/app/snapshot.go) returns ("", false) whenever
// consolelink.Resolve produces a URL that fails consolelink.Valid, the same
// check openBrowserCmd runs before exec'ing a URL. A well-formed ec2 selection
// passes through to ec2's regional deep link (core/aws/catalog_compute.go).
func TestController_Snapshot_ConsoleURL_ValidListSelectionCarriesOnlyValidURL(t *testing.T) {
	c := newTestController(t)

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{
		{ID: "i-0a1b2c3d4e5f60001", Name: "web-prod-01", Type: "ec2"},
	}, nil, false)

	snap := c.Snapshot()
	if snap.ConsoleURL == "" {
		t.Fatal("Snapshot().ConsoleURL is empty for a normal ec2 list selection")
	}
	if !consolelink.Valid(snap.ConsoleURL) {
		t.Errorf("Snapshot().ConsoleURL = %q fails consolelink.Valid", snap.ConsoleURL)
	}
	want := "https://us-east-1.console.aws.amazon.com/ec2/home?region=us-east-1#InstanceDetails:instanceId=i-0a1b2c3d4e5f60001"
	if snap.ConsoleURL != want {
		t.Errorf("Snapshot().ConsoleURL = %q, want %q", snap.ConsoleURL, want)
	}
}

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
