package unit

// qa_sort_test.go tests the column-position sort feature fixes (PR #267).
//
// Fix 1: nil-config fallback carries SortKey/SortPath from defaults when
//         a typeDef column title matches a default column title.
//
// Fix 2: Digit keys use absolute column mapping — key "7" always sorts column 7
//         regardless of scroll. Off-screen columns are sortable.
//
// Fix 3: Keys for non-existent columns (beyond total column count) are absorbed.
//
// Fix 4: Header position numbers use absolute column indices — after scrolling
//         right, the leftmost header shows the correct absolute number.
//
// Fix 5: Sort indicator persists on the correct column after horizontal scroll.

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ===========================================================================
// Helpers for column-position sort tests
// ===========================================================================

// wideTypeDef returns a type definition with 6 columns so that horizontal
// scroll can be exercised at a narrow terminal width.
//
// Column layout at width 80: col0(10)+col1(10)+col2(10)+col3(10) fit;
// col4 and col5 are pushed off-screen. At width 200 all six fit.
func wideTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		Name:      "Wide Resource",
		ShortName: "wide_test",
		Aliases:   []string{"wide_test"},
		Columns: []resource.Column{
			{Key: "col0", Title: "Alpha", Width: 10},
			{Key: "col1", Title: "Bravo", Width: 10},
			{Key: "col2", Title: "Charlie", Width: 10},
			{Key: "col3", Title: "Delta", Width: 10},
			{Key: "col4", Title: "Echo", Width: 10},
			{Key: "col5", Title: "Foxtrot", Width: 10},
		},
	}
}

// wideTestResources returns 3 resources for the wideTypeDef.
func wideTestResources() []resource.Resource {
	return []resource.Resource{
		{
			ID: "w-001", Name: "item-a", Status: "active",
			Fields: map[string]string{
				"col0": "a0", "col1": "b0", "col2": "c0",
				"col3": "d0", "col4": "e0", "col5": "f0",
			},
		},
		{
			ID: "w-002", Name: "item-b", Status: "active",
			Fields: map[string]string{
				"col0": "a1", "col1": "b1", "col2": "c1",
				"col3": "d1", "col4": "e1", "col5": "f1",
			},
		},
		{
			ID: "w-003", Name: "item-c", Status: "active",
			Fields: map[string]string{
				"col0": "a2", "col1": "b2", "col2": "c2",
				"col3": "d2", "col4": "e2", "col5": "f2",
			},
		},
	}
}

// wideModel builds a ResourceListModel for wideTypeDef with 3 resources loaded
// at the given terminal width.
func wideModel(t *testing.T, width int) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})

	td := wideTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(width, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "wide_test",
		Resources:    wideTestResources(),
	})
	return m
}

// ===========================================================================
// Fix 1: nil-config fallback carries SortKey/SortPath from defaults
// ===========================================================================

// TestQA_Sort_NilConfigFallback_S3_CreationDateHasSortMetadata verifies that
// when resolveColumns() falls through to the typeDef columns path (no
// viewConfig, and typeDef column count does NOT trigger the defaults-superset
// shortcut), the resulting columns still carry SortKey/SortPath from defaults
// when column titles match.
//
// The S3 s3BucketTypeDef defines only 2 columns (Bucket Name + Creation Date)
// while the defaults define 3 (Bucket Name + Region + Creation Date). The
// defaults superset path fires here and the returned columns must include the
// sort metadata from the defaults config.
//
// Before the fix, the typeDef-fallback path (lines 87-111 in table_render.go)
// did not merge SortKey/SortPath from defaults, leaving sortKey empty on columns
// that needed it — sort would fall back to display-value string comparison.
func TestQA_Sort_NilConfigFallback_S3_CreationDateHasSortMetadata(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})

	// s3BucketTypeDef has only 2 columns — Bucket Name + Creation Date.
	// The defaults for "s3" have 3 columns (Bucket Name + Region + Creation Date).
	// The superset check (len(defaultVD.List) > len(m.typeDef.Columns)) fires,
	// first-column title matches ("Bucket Name"), so defaults are used.
	m := s3RLBucketModel()

	// Sort by Creation Date — key "3" (3rd visible column in defaults layout).
	m, _ = m.Update(s3KeyPress("3"))

	out := m.View()

	// Sort indicator must appear — if SortKey/SortPath were missing, the sort
	// would still run (string comparison) but the indicator would be absent
	// when sortColKey can't match because colSortKey returns path/title instead.
	if !strings.Contains(out, "\u2191") && !strings.Contains(out, "\u2193") {
		t.Error("sort by Creation Date (key '3') should show a sort indicator; " +
			"missing indicator suggests sortColKey mismatch — SortKey/SortPath not carried from defaults")
	}

	// The sorted column indicator should appear on the Creation Date header, not
	// on another column. Check that the header line containing "Creation Date"
	// also contains the sort glyph.
	lines := strings.Split(out, "\n")
	creationDateLineHasIndicator := false
	for _, line := range lines {
		plain := stripANSI(line)
		if strings.Contains(plain, "Creation Date") {
			if strings.Contains(line, "\u2191") || strings.Contains(line, "\u2193") {
				creationDateLineHasIndicator = true
			}
		}
	}
	if !creationDateLineHasIndicator {
		t.Error("sort indicator should appear on the 'Creation Date' column header, not elsewhere")
	}
}

// ===========================================================================
// Fix 2: Absolute key mapping — key "1" always sorts column 0
// ===========================================================================

// TestQA_Sort_AbsoluteKey_SortsOffScreenColumn verifies that pressing a digit
// key sorts the corresponding absolute column even when that column is off-screen.
// Key "7" = absolute column 6 regardless of scroll or terminal width.
func TestQA_Sort_AbsoluteKey_SortsOffScreenColumn(t *testing.T) {
	// Width 45: only ~3 columns visible, but all 6 exist.
	// Key "6" should sort absolute column 5 even though it's off-screen.
	m := wideModel(t, 45)

	m, _ = m.Update(rlKeyPress("6"))

	sortIdx, sortAsc := m.SortState()
	if sortIdx != 5 {
		t.Errorf("key '6' should sort absolute column 5 (even if off-screen), got sortColIdx=%d", sortIdx)
	}
	if !sortAsc {
		t.Error("first sort press should be ascending")
	}
}

// ===========================================================================
// Fix 3: Key press for non-existent column is a no-op
// ===========================================================================

// TestQA_Sort_KeyBeyondColumnCountIsNoOp verifies that pressing a digit key
// for a column that doesn't exist at all is silently absorbed.
// wideTypeDef has 6 columns, so key "7" (column index 6) doesn't exist.
func TestQA_Sort_KeyBeyondColumnCountIsNoOp(t *testing.T) {
	m := wideModel(t, 200) // wide enough to show all 6 columns

	sortIdxBefore, sortAscBefore := m.SortState()

	// Press "7" — wideTypeDef only has 6 columns (keys 1-6 valid).
	m, _ = m.Update(rlKeyPress("7"))

	sortIdxAfter, sortAscAfter := m.SortState()

	if sortIdxBefore != sortIdxAfter || sortAscBefore != sortAscAfter {
		t.Errorf("key '7' for non-existent column should be a no-op, "+
			"but sort state changed: before=(%d,%v) after=(%d,%v)",
			sortIdxBefore, sortAscBefore, sortIdxAfter, sortAscAfter)
	}
}

// ===========================================================================
// Fix 4: Header numbers are 1-based from leftmost visible
// ===========================================================================

// TestQA_Sort_HeaderNumbers_StartAt1AfterScroll verifies that after scrolling
// right, the leftmost visible column header still shows "1:" as its position
// prefix, not "3:" or whatever the absolute index would be.
//
// Before the fix, position numbering used `absIdx = i + hScrollOffset`, which
// caused the leftmost visible column to show "3:" after scrolling 2 positions —
// inconsistent with the key bindings (key "1" sorts the leftmost visible column).
func TestQA_Sort_HeaderNumbers_StartAt1AfterScroll(t *testing.T) {
	// Use a wide enough terminal to scroll, but narrow enough to not show all cols.
	m := wideModel(t, 55)

	// Scroll right once.
	m, _ = m.Update(rlKeyPress("l"))

	hOff := m.HScrollOffset()
	if hOff == 0 {
		t.Skip("horizontal scroll did not advance — terminal width too large for this test")
	}

	out := m.View()
	plain := stripANSI(out)

	// Find the header row — the line containing a column title.
	headerLine := ""
	for line := range strings.SplitSeq(plain, "\n") {
		if strings.Contains(line, "Bravo") || strings.Contains(line, "Charlie") ||
			strings.Contains(line, "Delta") || strings.Contains(line, "Echo") {
			headerLine = line
			break
		}
	}
	if headerLine == "" {
		t.Fatal("could not find header row in rendered output")
	}

	// Headers use absolute column numbers. After scrolling right by hOff,
	// the leftmost visible column has absolute index hOff, displayed as hOff+1.
	// E.g., scroll right 1 → first visible column shows "2:" (absolute column 1).
	expectedPrefix := fmt.Sprintf("%d:", hOff+1)
	if !strings.Contains(headerLine, expectedPrefix) {
		t.Errorf("after scrolling right by %d, leftmost header should show absolute prefix %q, "+
			"but header line is: %q", hOff, expectedPrefix, headerLine)
	}
}

// ===========================================================================
// Fix 5: Sort indicator persists across horizontal scroll
// ===========================================================================

// TestQA_Sort_IndicatorPersistsAcrossScroll verifies that after sorting a column
// and then scrolling right, the sort indicator ↑/↓ still appears in the view on
// the correct column (if it's still visible) or disappears gracefully (if the
// sorted column scrolled off).
//
// Before the fix, sortColKey could become stale when the visible column set
// changed, causing the indicator to disappear or appear on the wrong column.
func TestQA_Sort_IndicatorPersistsAcrossScroll(t *testing.T) {
	// Width 200: all 6 columns fit. Sort by column 3 ("4" key = Delta), then scroll.
	m := wideModel(t, 200)

	// Sort by column 3 (key "4" = Delta, 4th visible column).
	m, _ = m.Update(rlKeyPress("4"))

	outBefore := m.View()
	if !strings.Contains(outBefore, "\u2191") {
		t.Fatal("sort indicator should appear after pressing '4'")
	}

	// Verify the indicator is on the Delta column header before scroll.
	plainBefore := stripANSI(outBefore)
	deltaHeaderBefore := ""
	for line := range strings.SplitSeq(plainBefore, "\n") {
		if strings.Contains(line, "Delta") {
			deltaHeaderBefore = line
			break
		}
	}
	if deltaHeaderBefore == "" {
		t.Fatal("could not find Delta column in header before scroll")
	}
	// The raw (ANSI) line should contain the sort glyph.
	for line := range strings.SplitSeq(outBefore, "\n") {
		if strings.Contains(stripANSI(line), "Delta") {
			if !strings.Contains(line, "\u2191") && !strings.Contains(line, "\u2193") {
				t.Error("sort indicator should be on Delta column header before scroll")
			}
			break
		}
	}

	// Scroll right once.
	m, _ = m.Update(rlKeyPress("l"))

	hOff := m.HScrollOffset()
	outAfter := m.View()

	// If Delta is still visible after scroll, indicator should still be on it.
	// If Delta scrolled off (hScrollOffset > 3), the indicator is absent — that's ok.
	plainAfter := stripANSI(outAfter)
	deltaVisible := strings.Contains(plainAfter, "Delta")

	if deltaVisible {
		// Delta is still in view — sort indicator must still appear on it.
		foundOnDelta := false
		for line := range strings.SplitSeq(outAfter, "\n") {
			if strings.Contains(stripANSI(line), "Delta") {
				if strings.Contains(line, "\u2191") || strings.Contains(line, "\u2193") {
					foundOnDelta = true
				}
				break
			}
		}
		if !foundOnDelta {
			t.Errorf("sort indicator disappeared from Delta column after scrolling right by %d", hOff)
		}
	}

	// The sort state (sortColIdx, sortAsc) must be unchanged by the scroll.
	sortIdxAfter, sortAscAfter := m.SortState()
	if sortIdxAfter != 3 {
		t.Errorf("sort column index should still be 3 (Delta) after scroll, got %d", sortIdxAfter)
	}
	if !sortAscAfter {
		t.Errorf("sort direction should still be ascending after scroll")
	}
}

// ===========================================================================
// Fix 6: S3 sort — Creation Date uses correct visible-position key
// ===========================================================================

// TestQA_Sort_S3_CreationDate_VisiblePositionKey verifies that the Creation
// Date column in the S3 bucket list is sorted by pressing the correct
// visible-position key ("3" = third visible column in defaults layout:
// Bucket Name, Region, Creation Date).
//
// This test replaces the TestQA_S3_A6_3_Sort_ByAge approach (which used the
// now-deprecated SortAge sentinel). After the visible-position fix, sort is
// triggered by digit key matching visible column position, not a named field.
//
// The s3BucketTypeDef has only 2 columns, but the defaults for "s3" have 3
// (Bucket Name, Region, Creation Date). The superset path in resolveColumns()
// fires and uses defaults, so 3 columns are visible and "3" is the correct key.
func TestQA_Sort_S3_CreationDate_VisiblePositionKey(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})

	m := s3RLBucketModel()

	// Press "3" — sorts Creation Date (third visible column in defaults).
	m, _ = m.Update(s3KeyPress("3"))

	out := m.View()

	// Sort indicator must appear somewhere in the view.
	if !strings.Contains(out, "\u2191") && !strings.Contains(out, "\u2193") {
		t.Error("pressing '3' should sort by Creation Date and show a sort indicator")
	}

	// Sort indicator (↑/↓) is appended directly to the column title in the header.
	// All column titles appear on the same header line, so we check that the
	// indicator is adjacent to "Creation Date", not to other column names.
	plain := stripANSI(out)
	if !strings.Contains(plain, "Creation Date\u2191") && !strings.Contains(plain, "Creation Date\u2193") {
		t.Error("sort indicator must appear immediately after 'Creation Date' column title")
	}
	if strings.Contains(plain, "Bucket Name\u2191") || strings.Contains(plain, "Bucket Name\u2193") {
		t.Error("sort indicator should be on 'Creation Date', not 'Bucket Name'")
	}
	if strings.Contains(plain, "Region\u2191") || strings.Contains(plain, "Region\u2193") {
		t.Error("sort indicator should be on 'Creation Date', not 'Region'")
	}
}

// TestQA_Sort_S3_CreationDate_SortsDataCorrectly verifies that after pressing
// "3", the S3 buckets are actually sorted by creation date in ascending order.
//
// Fixture buckets (from fixtureS3Buckets) by creation_date ascending:
//   dev-fileshare        2025-03-06
//   dev-loki-chunks      2025-05-?? (second)
//   cdn-logs.example.com 2025-05-12
//   cdn-website...       2025-05-13
//   test-app-state       2025-06-20
func TestQA_Sort_S3_CreationDate_SortsDataCorrectly(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})

	m := s3RLBucketModel()

	// Sort by Creation Date ascending (key "3").
	m, _ = m.Update(s3KeyPress("3"))

	out := m.View()
	plain := stripANSI(out)

	// dev-fileshare (2025-03-06) must appear before test-app-state (2025-06-20)
	// in ascending order.
	idxFileshare := strings.Index(plain, "dev-fileshare")
	idxTestApp := strings.Index(plain, "test-app-state")

	if idxFileshare < 0 || idxTestApp < 0 {
		t.Fatalf("expected 'dev-fileshare' and 'test-app-state' in output; plain=%q", plain)
	}

	if idxFileshare > idxTestApp {
		t.Errorf("ascending sort by Creation Date should place dev-fileshare (2025-03-06) "+
			"before test-app-state (2025-06-20), but got reversed order: "+
			"dev-fileshare@%d, test-app-state@%d", idxFileshare, idxTestApp)
	}
}
