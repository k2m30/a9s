package unit_test

// detail_navigable_test.go — tests for navigable field rendering and Enter-key
// navigation in the detail view (T016).
//
// Navigable fields are registered via resource.RegisterNavigableFields.
// When a field is navigable the detail view renders it with an underline style
// and pressing Enter while the cursor is on that field emits a
// messages.RelatedNavigateMsg.

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// testNavEC2 is a minimal stand-in for an EC2 instance used only in navigable
// field tests. It provides the fields the ViewDef paths reference.
type testNavEC2 struct {
	VpcId        *string
	InstanceType string
}

// navViewConfig returns a ViewsConfig with a single "ec2" entry whose Detail
// paths are limited to the two fields we care about in these tests.
// Keeping the list short ensures that fieldCursor == 0 points directly to the
// "VpcId" field (which is at index 0 in the resulting fieldList).
func navViewConfig() *config.ViewsConfig {
	return &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {
				Detail: []config.DetailField{{Path: "VpcId"}, {Path: "InstanceType"}},
			},
		},
	}
}

// makeNavEC2Resource builds a resource.Resource with a VpcId pointer and a
// snake_case Fields map so both extraction paths are exercised.
func makeNavEC2Resource() resource.Resource {
	vpcID := "vpc-test123"
	return resource.Resource{
		ID:   "i-test123",
		Name: "test-instance",
		Fields: map[string]string{
			"vpc_id": "vpc-test123",
			"type":   "t3.medium",
		},
		RawStruct: &testNavEC2{
			VpcId:        &vpcID,
			InstanceType: "t3.medium",
		},
	}
}

// makeNavDetail creates a DetailModel with navigable fields registered, sets
// its size, and returns it ready for View() and Update().
// The caller is responsible for deregistering navigable fields via defer.
func makeNavDetail(width, height int) views.DetailModel {
	res := makeNavEC2Resource()
	k := keys.Default()
	d := views.NewDetail(res, "ec2", navViewConfig(), k)
	d.SetSize(width, height)
	return d
}

// ---------------------------------------------------------------------------
// Test 1 — navigable field rendered with underline escape
// ---------------------------------------------------------------------------

func TestDetail_NavigableField_HighlightedInView(t *testing.T) {
	// Colours must be ON so the underline escape is emitted.
	// Explicitly unset NO_COLOR and reinitialize styles.
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })

	resource.RegisterNavigableFields("ec2", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
	})
	defer resource.UnregisterNavigableFields("ec2")

	d := makeNavDetail(140, 30)

	// Move cursor off VpcId (row 0) → row 1, so VpcId is NOT under cursor.
	// Per spec-007 Bug4, navigable underline is suppressed when cursor is on the row.
	d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	view := d.View()

	// VpcId is navigable and cursor is NOT on it → NavigableField style (underline + accent colour).
	// Lipgloss v2 encodes underline as attribute "4" combined with other codes,
	// so the escape looks like \x1b[4;... rather than a standalone \x1b[4m.
	// We check for the two forms that indicate underline is active.
	hasUnderline := strings.Contains(view, "\x1b[4;") || strings.Contains(view, "\x1b[4m")
	if !hasUnderline {
		t.Errorf("navigable field VpcId should be rendered with underline (\\x1b[4;... or \\x1b[4m), got:\n%s", view)
	}

	// The value must be present in the stripped (ANSI-free) view.
	// Lipgloss v2 may render underlined text char-by-char with ANSI resets,
	// so the literal value won't appear as a contiguous substring in raw output.
	plain := stripAnsi(view)
	if !strings.Contains(plain, "vpc-test123") {
		t.Errorf("navigable field value \"vpc-test123\" must appear in stripped view output, got:\n%s", plain)
	}

	// InstanceType is non-navigable — strip all ANSI from view and verify
	// "InstanceType" appears (the field renders without underline on its key
	// portion when we strip; underline ANSI is only around the navigable key).
	if !strings.Contains(plain, "InstanceType") {
		t.Errorf("non-navigable field InstanceType must appear in stripped view")
	}
}

// ---------------------------------------------------------------------------
// Test 2 — Enter on a navigable field emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

func TestDetail_NavigableField_EnterEmitsNavigateMsg(t *testing.T) {
	resource.RegisterNavigableFields("ec2", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
	})
	defer resource.UnregisterNavigableFields("ec2")

	// Use a narrow-enough width that the right column is NOT auto-shown (< 100).
	// That prevents rightColAutoShown, so the right column is not focused and
	// Enter falls through to navigable-field handling.
	d := makeNavDetail(80, 30)

	// fieldCursor starts at 0. With navViewConfig(), the first item in fieldList
	// is "VpcId" (navigable). Pressing Enter should emit RelatedNavigateMsg.
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Enter on navigable field should return a non-nil cmd")
	}

	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("cmd() should produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "vpc" {
		t.Errorf("RelatedNavigateMsg.TargetType: want %q, got %q", "vpc", nav.TargetType)
	}
	if !strings.Contains(nav.TargetID, "vpc-test123") {
		t.Errorf("RelatedNavigateMsg.TargetID should contain \"vpc-test123\", got %q", nav.TargetID)
	}
	if nav.SourceType != "ec2" {
		t.Errorf("RelatedNavigateMsg.SourceType: want %q, got %q", "ec2", nav.SourceType)
	}
}

// ---------------------------------------------------------------------------
// Test 3 — No NavigableFields registered → Enter is a no-op
// ---------------------------------------------------------------------------

func TestDetail_NonNavigableField_EnterIsNoop(t *testing.T) {
	// Deliberately do NOT register navigable fields for "ec2".
	// VpcId is present in the config but has no navigable registration.
	d := makeNavDetail(80, 30)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil {
		t.Errorf("Enter on non-navigable field should return nil cmd, got a non-nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Test 4 — nil viewConfig → Enter is a no-op (fieldList never built)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// IsFieldNavigable unit tests (resource.IsFieldNavigable)
// ---------------------------------------------------------------------------

func TestIsFieldNavigable_MatchFound(t *testing.T) {
	resource.RegisterNavigableFields("ec2", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "SubnetId", TargetType: "subnet"},
	})
	defer resource.UnregisterNavigableFields("ec2")

	f := resource.IsFieldNavigable("ec2", "VpcId")
	if f == nil {
		t.Fatal("IsFieldNavigable: expected non-nil for registered field VpcId")
	}
	if f.TargetType != "vpc" {
		t.Errorf("IsFieldNavigable: TargetType: want %q, got %q", "vpc", f.TargetType)
	}
}

func TestIsFieldNavigable_NoMatch(t *testing.T) {
	resource.RegisterNavigableFields("ec2", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
	})
	defer resource.UnregisterNavigableFields("ec2")

	f := resource.IsFieldNavigable("ec2", "SubnetId")
	if f != nil {
		t.Errorf("IsFieldNavigable: expected nil for unregistered field SubnetId, got %+v", f)
	}
}

func TestIsFieldNavigable_UnknownType(t *testing.T) {
	f := resource.IsFieldNavigable("rds", "VpcId")
	if f != nil {
		t.Errorf("IsFieldNavigable: expected nil for unregistered type, got %+v", f)
	}
}

func TestDetail_NoFieldList_EnterIsNoop(t *testing.T) {
	res := makeNavEC2Resource()
	k := keys.Default()
	// Pass nil viewConfig so buildFieldList sets fieldList = nil.
	d := views.NewDetail(res, "ec2", nil, k)
	d.SetSize(80, 30)

	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil {
		t.Errorf("Enter with nil viewConfig should return nil cmd, got a non-nil cmd")
	}
}
