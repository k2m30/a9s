package unit_test

// detail_rendering_spec007_test.go — 11 failing tests for 5 rendering divergences
// identified in Spec-007.
//
// All tests in this file are written BEFORE the coder fixes land.
// They must:
//   1. Compile cleanly against the CURRENT code
//   2. FAIL against the CURRENT code
//   3. PASS after the coder applies the fixes described in the spec
//
// Bugs under test:
//   Bug1: View() never inserts │ (U+2502) between left and right columns
//   Bug2: renderFromFieldList() sub-field renders only item.Value — no separate key label styling
//   Bug3: renderFromFieldList() never applies styles.RowSelected to cursor row
//   Bug4: NavigableField underline is always on; should be OFF when cursor is on that row
//   Bug5: No stacked layout for width 80-99 with right column registered

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers local to spec-007 tests
// ---------------------------------------------------------------------------

// make007EC2Detail creates a DetailModel for "ec2" using only Fields (no RawStruct)
// to keep the fieldList minimal and deterministic.
// The caller controls width and whether related defs are registered.
func make007EC2Detail(width, height int, cfg *config.ViewsConfig) views.DetailModel {
	res := resource.Resource{
		ID:   "i-spec007test",
		Name: "spec007-instance",
		Fields: map[string]string{
			"InstanceId":   "i-spec007test",
			"VpcId":        "vpc-spec007",
			"InstanceType": "t3.small",
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", cfg, k)
	d.SetSize(width, height)
	return d
}

// twoFieldNavConfig returns a ViewsConfig with exactly two "ec2" detail paths,
// making fieldCursor index 0 = "VpcId" (navigable) and index 1 = "InstanceType".
func twoFieldNavConfig() *config.ViewsConfig {
	return &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {
				Detail: []string{"VpcId", "InstanceType"},
			},
		},
	}
}

// make007NavDetailWithColors creates a DetailModel with colors enabled, a
// 2-field nav config, and "VpcId" registered as navigable → "vpc".
// Caller must call defer resource.UnregisterNavigableFields("ec2") and
// defer styles.Reinit().
func make007NavDetailWithColors(t *testing.T, width, height int) views.DetailModel {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })

	resource.RegisterNavigableFields("ec2", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
	})
	t.Cleanup(func() { resource.UnregisterNavigableFields("ec2") })

	return make007EC2Detail(width, height, twoFieldNavConfig())
}

// register007EC2Defs registers one RelatedDef for "ec2" and returns cleanup.
func register007EC2Defs(t *testing.T) {
	t.Helper()
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})
	t.Cleanup(func() { resource.UnregisterRelated("ec2") })
}

// ---------------------------------------------------------------------------
// Bug 1: Missing │ separator between left and right columns
// ---------------------------------------------------------------------------

// TestDetail_007_SeparatorPresent_LeftFocused verifies that when the right column
// is showing (width=120, registered defs), View() includes a │ character between
// the two columns.
// FAILS NOW: View() builds left+right with no │ in between.
// PASSES AFTER FIX: │ is inserted at the column boundary.
func TestDetail_007_SeparatorPresent_LeftFocused(t *testing.T) {
	register007EC2Defs(t)

	d := make007EC2Detail(120, 30, nil)
	view := d.View()

	if !strings.Contains(view, "RELATED") {
		t.Skip("right column not shown at width=120 with registered defs; cannot verify separator")
	}

	if !strings.Contains(view, "│") {
		t.Errorf("View() with right column showing (left focused) must contain │ (U+2502) column separator; got:\n%s", stripAnsi(view))
	}
}

// TestDetail_007_SeparatorPresent_RightFocused verifies that │ is present even
// when the right column has focus (after Tab).
// FAILS NOW: no │ in View() output at all.
// PASSES AFTER FIX: │ persists regardless of focus state.
func TestDetail_007_SeparatorPresent_RightFocused(t *testing.T) {
	register007EC2Defs(t)

	d := make007EC2Detail(120, 30, nil)
	view := d.View()

	if !strings.Contains(view, "RELATED") {
		t.Skip("right column not shown at width=120; cannot test separator with right focus")
	}

	// Tab to focus right column.
	d, _ = pressTabDetail(d)
	view = d.View()

	if !strings.Contains(view, "│") {
		t.Errorf("View() with right column focused must still contain │ (U+2502); got:\n%s", stripAnsi(view))
	}
}

// TestDetail_007_SeparatorAbsent_NoRightCol is a regression guard: when no related
// defs are registered, │ must NOT appear in View() output.
// PASSES NOW (regression guard — no right column → no separator).
// Must continue to pass after fix.
func TestDetail_007_SeparatorAbsent_NoRightCol(t *testing.T) {
	// Explicitly ensure no related defs are registered for "ec2".
	resource.UnregisterRelated("ec2")
	t.Cleanup(func() { resource.UnregisterRelated("ec2") })

	d := make007EC2Detail(120, 30, nil)
	view := d.View()

	if strings.Contains(view, "│") {
		t.Errorf("View() with no right column must NOT contain │; got:\n%s", stripAnsi(view))
	}
}

// ---------------------------------------------------------------------------
// Bug 2: Sub-field key label missing
// ---------------------------------------------------------------------------

// TestDetail_007_SubField_RendersKeyAndValue verifies that when a detail field
// produces multi-line (sub-field) output, each sub-field line's KEY portion is
// rendered with a distinct style from its VALUE portion.
//
// The current code applies only styles.DetailVal to the whole sub-field line:
//
//	line = "     " + styles.DetailVal.Render(item.Value)
//
// After fix, the key part (text before ": ") is styled with styles.DetailKey
// and the value part (text after ": ") with styles.DetailVal, producing two
// separate ANSI-styled segments per sub-field line.
//
// Test strategy (colors ON):
//   - Use a Tags field that produces multi-line YAML output.
//   - With colors ON, current code emits one DetailVal ANSI sequence per sub-field line.
//   - After fix, each sub-field line emits TWO distinct color sequences (DetailKey + DetailVal).
//   - We detect the current bug by checking for DetailKey-styled text on a sub-field line.
//   - With DetailKey color = ColDetailKey and DetailVal color = ColDetailVal (different
//     foreground codes), the presence of the DetailKey ANSI color on a sub-field line
//     distinguishes the fixed from the broken state.
//
// FAILS NOW: sub-field lines have only DetailVal ANSI, not DetailKey.
// PASSES AFTER FIX: sub-field lines contain both DetailKey and DetailVal ANSI sequences.
func TestDetail_007_SubField_RendersKeyAndValue(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })
	resource.UnregisterNavigableFields("ec2")
	resource.UnregisterRelated("ec2")
	t.Cleanup(func() {
		resource.UnregisterNavigableFields("ec2")
		resource.UnregisterRelated("ec2")
	})

	// Register a multi-line field: Tags as a YAML-like string with sub-entries.
	// We inject Tags directly into Fields as a multi-line value so ExtractFieldList
	// generates a header FieldItem + sub-field FieldItems.
	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {
				Detail: []string{"Tags"},
			},
		},
	}
	res := resource.Resource{
		ID:   "i-subfield007",
		Name: "subfield-test",
		Fields: map[string]string{
			// Multi-line value triggers header + sub-field path in ExtractFieldList.
			"Tags": "Name: web-prod\nEnv: production",
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", cfg, k)
	d.SetSize(120, 30)

	view := d.View()
	plain := stripAnsi(view)

	// Basic sanity: plain output must contain the sub-field values.
	if !strings.Contains(plain, "web-prod") {
		t.Fatalf("stripped View() must contain sub-field value %q; got:\n%s", "web-prod", plain)
	}
	if !strings.Contains(plain, "Name") {
		t.Fatalf("stripped View() must contain sub-field key %q; got:\n%s", "Name", plain)
	}

	// With colors ON, DetailKey style has a distinct ANSI foreground code.
	// Get the rendered DetailKey style by rendering a known string and extracting
	// the opening escape sequence.
	detailKeyRendered := styles.DetailKey.Render("X")
	if !strings.Contains(detailKeyRendered, "\x1b[") {
		// Colors appear to be off despite our Setenv — skip rather than false-fail.
		t.Skip("DetailKey produces no ANSI escape; color environment may not be active")
	}
	// Extract the ANSI prefix of DetailKey (everything up to and including the first 'm').
	detailKeyPrefix := ""
	if idx := strings.Index(detailKeyRendered, "m"); idx >= 0 {
		detailKeyPrefix = detailKeyRendered[:idx+1]
	}
	if detailKeyPrefix == "" {
		t.Skip("could not extract DetailKey ANSI prefix; skipping color-dependent assertion")
	}

	// Find a sub-field line in the raw view that contains the sub-field value "web-prod".
	// Each line in view corresponds to one rendered line.
	lines := strings.Split(view, "\n")
	var subFieldLine string
	for _, line := range lines {
		if strings.Contains(stripAnsi(line), "Name: web-prod") {
			subFieldLine = line
			break
		}
	}
	if subFieldLine == "" {
		// Sub-field might be split into key and value on separate styled segments.
		// Try finding the line with "Name" in it.
		for _, line := range lines {
			if strings.Contains(stripAnsi(line), "Name") && strings.Contains(stripAnsi(line), "web-prod") {
				subFieldLine = line
				break
			}
		}
	}
	if subFieldLine == "" {
		t.Fatalf("could not find sub-field line containing %q in view:\n%s", "Name: web-prod", plain)
	}

	// After fix, the sub-field line must contain DetailKey ANSI sequence (for the key part).
	// Currently it only contains DetailVal sequence.
	if !strings.Contains(subFieldLine, detailKeyPrefix) {
		t.Errorf("sub-field line must contain DetailKey ANSI style %q (separate key styling);\n  sub-field line: %q\n  full plain view:\n%s",
			detailKeyPrefix, stripAnsi(subFieldLine), plain)
	}
}

// ---------------------------------------------------------------------------
// Bug 3: Cursor highlight missing
// ---------------------------------------------------------------------------

// TestDetail_007_CursorHighlight_Row0 verifies that View() applies
// styles.RowSelected (background highlight) to the field at cursor index 0.
// FAILS NOW: renderFromFieldList() never applies RowSelected to any row.
// PASSES AFTER FIX: the row at fieldCursor gets RowSelected applied.
func TestDetail_007_CursorHighlight_Row0(t *testing.T) {
	d := make007NavDetailWithColors(t, 120, 30)

	view := d.View()

	// RowSelected with colors ON: lipgloss.NewStyle().Background(...) emits \x1b[48;
	// RowSelected with NO_COLOR=1: lipgloss.NewStyle().Reverse(true) emits \x1b[7m
	// We need colors ON (NO_COLOR="") so we get a background-color highlight.
	// Check for both true-color background (\x1b[48;) and reverse video (\x1b[7m).
	hasHighlight := strings.Contains(view, "\x1b[48;") || strings.Contains(view, "\x1b[7m")
	if !hasHighlight {
		t.Errorf("View() at cursor index 0 must contain RowSelected ANSI style (\\x1b[48; or \\x1b[7m) on the first field;\ngot stripped view:\n%s", stripAnsi(view))
	}
}

// TestDetail_007_CursorHighlight_MovesAfterJ verifies that pressing j once moves
// the highlight from index 0 to index 1, and index 0 loses the background style.
// FAILS NOW: no highlight exists at any row — impossible to "move" it.
// PASSES AFTER FIX: row 1 gains highlight, row 0 loses it.
func TestDetail_007_CursorHighlight_MovesAfterJ(t *testing.T) {
	d := make007NavDetailWithColors(t, 120, 30)

	// Press j once: cursor moves to index 1 (InstanceType).
	d, _ = pressDown(d)
	view := d.View()

	// Highlight must be present somewhere (row 1 should be highlighted).
	hasHighlight := strings.Contains(view, "\x1b[48;") || strings.Contains(view, "\x1b[7m")
	if !hasHighlight {
		t.Errorf("View() after pressing j must contain RowSelected highlight (\\x1b[48; or \\x1b[7m);\ngot stripped view:\n%s", stripAnsi(view))
	}

	// Additionally verify that the PLAIN text of line 0 (VpcId) differs from
	// line 1 (InstanceType): only one of them should have the highlight in the raw view.
	// We find the line containing "vpc-spec007" (field value at index 0) and check
	// it does NOT contain a background escape.
	lines := strings.Split(view, "\n")
	var row0Line string
	for _, line := range lines {
		if strings.Contains(stripAnsi(line), "vpc-spec007") {
			row0Line = line
			break
		}
	}
	if row0Line == "" {
		t.Fatal("could not find VpcId row in View() output; check field rendering")
	}
	// Row 0 should NOT have background highlight after j.
	row0HasBg := strings.Contains(row0Line, "\x1b[48;") || strings.Contains(row0Line, "\x1b[7m")
	if row0HasBg {
		t.Errorf("VpcId row (index 0) must NOT have background highlight after cursor moved to index 1;\n  row0Line: %q", row0Line)
	}
}

// TestDetail_007_CursorHighlight_AbsentWhenRightFocused verifies that when the
// right column is focused (after Tab), no left-column row carries RowSelected.
//
// NOTE: This test requires Bug1 (│ separator) to be fixed first, so it can
// reliably split left-column content from right-column content on each line.
// When no │ is present (Bug1 unfixed), the test skips.
//
// After Bug3 fix, this test ensures the left column loses its cursor highlight
// when the right column takes focus.
func TestDetail_007_CursorHighlight_AbsentWhenRightFocused(t *testing.T) {
	register007EC2Defs(t)
	t.Setenv("NO_COLOR", "")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })

	d := make007EC2Detail(120, 30, twoFieldNavConfig())

	view := d.View()
	if !strings.Contains(view, "RELATED") {
		t.Skip("right column not shown at width=120; cannot test focus-transfer")
	}

	// This test requires Bug1 to be fixed (│ separator present) so we can
	// isolate left-column content from right-column content.
	if !strings.Contains(view, "│") {
		t.Skip("│ separator not present (Bug1 not yet fixed); skipping left/right isolation test")
	}

	// Tab to focus right column.
	d, _ = pressTabDetail(d)
	view = d.View()

	// After Bug1 fix, each line has │ as the boundary. Check left-column content only.
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		before, _, ok := strings.Cut(line, "│")
		if !ok {
			continue
		}
		leftPart := before
		// Left column field lines must not carry background highlight when right is focused.
		if strings.Contains(stripAnsi(leftPart), "vpc-spec007") || strings.Contains(stripAnsi(leftPart), "t3.small") {
			hasLeftBg := strings.Contains(leftPart, "\x1b[48;") || strings.Contains(leftPart, "\x1b[7m")
			if hasLeftBg {
				t.Errorf("left-column line %d must NOT have background highlight when right column is focused;\n  left part: %q", i, leftPart)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Bug 4: Underline always on navigable field
// ---------------------------------------------------------------------------

// TestDetail_007_NavigableUnderline_OffWhenSelected verifies that the navigable
// VpcId field does NOT have an underline escape when the cursor is on its row
// (index 0). When selected, the full-row highlight (RowSelected) should replace
// the underline indicator.
// FAILS NOW: NavigableField (underline) is always applied regardless of cursor position.
// PASSES AFTER FIX: cursor row shows RowSelected instead of NavigableField underline.
func TestDetail_007_NavigableUnderline_OffWhenSelected(t *testing.T) {
	d := make007NavDetailWithColors(t, 120, 30)
	// Cursor is at index 0 (VpcId) by default.
	view := d.View()

	// Find the line containing the VpcId value.
	lines := strings.Split(view, "\n")
	var vpcLine string
	for _, line := range lines {
		if strings.Contains(stripAnsi(line), "vpc-spec007") {
			vpcLine = line
			break
		}
	}
	if vpcLine == "" {
		t.Fatal("could not find VpcId row (value=vpc-spec007) in View() output")
	}

	// The VpcId line (cursor row) must NOT contain underline ANSI escapes.
	// Underline = \x1b[4m or \x1b[4;
	hasUnderline := strings.Contains(vpcLine, "\x1b[4m") || strings.Contains(vpcLine, "\x1b[4;")
	if hasUnderline {
		t.Errorf("VpcId row (cursor at index 0) must NOT have underline escape when selected;\n  line: %q\n  (underline should be replaced by RowSelected highlight after fix)", vpcLine)
	}
}

// TestDetail_007_NavigableUnderline_OnWhenNotSelected verifies that the VpcId
// navigable field DOES have an underline escape when the cursor is NOT on its row.
// PASSES NOW: NavigableField style (underline) is always applied.
// Must continue to pass after fix (underline on non-cursor navigable rows).
func TestDetail_007_NavigableUnderline_OnWhenNotSelected(t *testing.T) {
	d := make007NavDetailWithColors(t, 120, 30)
	// Press j: cursor moves to index 1 (InstanceType), leaving VpcId at index 0 unselected.
	d, _ = pressDown(d)
	view := d.View()

	lines := strings.Split(view, "\n")
	var vpcLine string
	for _, line := range lines {
		if strings.Contains(stripAnsi(line), "vpc-spec007") {
			vpcLine = line
			break
		}
	}
	if vpcLine == "" {
		t.Fatal("could not find VpcId row in View() after pressing j")
	}

	// VpcId is navigable and NOT the cursor row → must have underline.
	hasUnderline := strings.Contains(vpcLine, "\x1b[4m") || strings.Contains(vpcLine, "\x1b[4;")
	if !hasUnderline {
		t.Errorf("VpcId row (non-cursor, navigable) must have underline escape (\\x1b[4m or \\x1b[4;) when cursor is elsewhere;\n  line: %q", vpcLine)
	}
}

// ---------------------------------------------------------------------------
// Bug 5: Related panel must stay visible for width 80-99
// ---------------------------------------------------------------------------

// TestDetail_007_MediumLayout_85Cols_HasRelated verifies that at width=85 with
// registered related defs, View() contains RELATED content.
func TestDetail_007_MediumLayout_85Cols_HasRelated(t *testing.T) {
	register007EC2Defs(t)

	d := make007EC2Detail(85, 30, nil)
	plain := stripAnsi(d.View())

	if !strings.Contains(plain, "RELATED") {
		t.Errorf("View() at width=85 with registered related defs must contain RELATED panel;\ngot:\n%s", plain)
	}
}

// TestDetail_007_MediumLayout_85Cols_HasSeparator verifies that medium widths
// keep the related panel visible in two-column mode.
func TestDetail_007_MediumLayout_85Cols_HasSeparator(t *testing.T) {
	register007EC2Defs(t)

	d := make007EC2Detail(85, 30, nil)
	view := d.View()

	if !strings.Contains(view, "│") {
		t.Errorf("medium layout (width=85) must contain │ column separator; got:\n%s", stripAnsi(view))
	}
}

// TestDetail_007_SideBySide_100Cols_HasSeparator verifies that at exactly width=100
// (the side-by-side threshold), View() contains │.
// FAILS NOW: View() builds left+right without inserting │.
// PASSES AFTER FIX: │ is inserted at the boundary on every line.
func TestDetail_007_SideBySide_100Cols_HasSeparator(t *testing.T) {
	register007EC2Defs(t)

	d := make007EC2Detail(100, 30, nil)
	view := d.View()

	if !strings.Contains(view, "RELATED") {
		t.Skip("right column not auto-shown at width=100; cannot verify │ separator")
	}

	if !strings.Contains(view, "│") {
		t.Errorf("View() at width=100 (side-by-side threshold) must contain │ separator;\ngot stripped view:\n%s", stripAnsi(view))
	}
}
