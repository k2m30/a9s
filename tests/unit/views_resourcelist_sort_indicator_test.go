package unit

// Tests for §6 of docs/design/ct-event-list-v2.md: sort indicator binding.
//
// History: an earlier named-sort model matched sort sentinels against any
// column whose key or title contained "time"/"event"/"date" etc. For
// ct-events, both the TIME column (key="time") and the EVENT column
// (title="EVENT", which contains "event") matched → both got the ↓/↑ glyph.
//
// Contract (§6): exactly ONE column carries the sort glyph for any active
// sort. Runtime sort is column-position-only (sortColIdx on
// ResourceListModel); there is no substring matching in the sort indicator
// path.
//
// Column headers now carry number prefixes: "1:TIME", "2:EVENT", etc.
// The sort glyph appears after the prefix+title: "2:TIME↓" (descending).

import (
	"os"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// countSortGlyphs returns the total number of ↑ or ↓ characters in s.
func countSortGlyphs(header string) int {
	return strings.Count(header, "\u2191") + strings.Count(header, "\u2193")
}

// headerLine extracts the first line of View() output.
// The header is always the first line regardless of frame borders because
// ResourceListModel.View() starts with renderHeaderRow().
func headerLine(view string) string {
	lines := strings.SplitN(view, "\n", 2)
	return lines[0]
}

// buildSortModel creates a ResourceListModel for the given resource type with
// a config-driven view, loads one synthetic resource, and applies the given
// sort column index. Uses sortAsc=false (descending) to match ct-events default.
//
// For ct-events the model must receive viewConfig so the default sort
// initialisation in NewResourceList kicks in (§6 requirement).
func buildSortModel(
	t *testing.T,
	shortName string,
	sortColIdx int,
	sortAsc bool,
) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("resource type %q not found in registry", shortName)
	}

	cfg := config.DefaultConfig()

	k := keys.Default()
	m := views.NewResourceList(*td, cfg, k)
	m.SetSize(200, 20)
	m, _ = m.Init()

	// Load a minimal synthetic resource so View() renders the header.
	res := syntheticResourceForType(shortName)
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: shortName,
		Resources:    []resource.Resource{res},
	})

	// Apply the requested sort column and direction via key presses.
	m = applySort(t, m, sortColIdx, sortAsc)

	return m
}

// applySort sets the sort column on the model by simulating key presses.
// sortColIdx is 0-based; key "1" activates column 0, key "2" activates column 1, etc.
// Key "0" activates column 9 (10th column).
//
// If sortColIdx == views.SortColNone, no key press is sent and the model is
// returned unchanged.
//
// A second key press on the same column toggles ascending/descending.
// If the direction after the first press does not match wantAsc, a second
// press is sent.
func applySort(t *testing.T, m views.ResourceListModel, sortColIdx int, wantAsc bool) views.ResourceListModel {
	t.Helper()

	if sortColIdx == views.SortColNone {
		return m
	}

	// Map 0-based column index to the digit key character.
	// Columns 0-8 → "1"-"9"; column 9 → "0".
	var keyChar string
	switch {
	case sortColIdx >= 0 && sortColIdx <= 8:
		keyChar = string(rune('1' + sortColIdx))
	case sortColIdx == 9:
		keyChar = "0"
	default:
		t.Fatalf("applySort: column index %d out of range (0-9)", sortColIdx)
	}

	msg := rlKeyPress(keyChar)

	// Press the sort key once to activate the sort column.
	m, _ = m.Update(msg)

	// Check direction — a second press on the same column toggles ascending/descending.
	_, gotAsc := m.SortState()
	if gotAsc != wantAsc {
		m, _ = m.Update(msg)
	}

	return m
}

// syntheticResourceForType returns a minimal resource.Resource sufficient to
// make View() render at least one data row for the given resource type.
// We do not invent real AWS IDs; all values are synthetic test data.
func syntheticResourceForType(shortName string) resource.Resource {
	switch shortName {
	case "ct-events":
		return resource.Resource{
			ID:   "event-test-0001",
			Name: "DescribeInstances",
			// Use ct-info (dim) so the sort test is purely about header decoration.
			Fields: map[string]string{
				"time":        "Apr 07 17:00:00",
				"event_time":  "2026-04-07T17:00:00Z",
				"_ct.verb":    "R",
				"_ct.actor":   "test-user",
				"_ct.origin":  "CLI",
				"_ct.target":  "i-0test001",
				"_ct.outcome": "OK",
			},
		}
	case "ec2":
		return resource.Resource{
			ID:     "i-0test001",
			Name:   "test-instance",
			Fields: map[string]string{
				"instance_id": "i-0test001",
				"name":        "test-instance",
				"state":       "running",
				"type":        "t3.medium",
			},
		}
	case "dbi":
		return resource.Resource{
			ID:     "test-db-instance",
			Name:   "test-db",
			Fields: map[string]string{
				"db_instance_identifier": "test-db",
				"status":                 "available",
				"engine":                 "mysql",
				"instance_class":         "db.t3.medium",
			},
		}
	default:
		return resource.Resource{
			ID:     "test-resource-001",
			Name:   "test-resource",
			Fields: map[string]string{},
		}
	}
}

// ===========================================================================
// TestSortIndicator_ExactlyOnePerSort
//
// For each (resource-type, sort-column-index) pair: assert the rendered header
// contains exactly the expected number of sort glyphs (↑ or ↓), and when
// wantOnColumn is non-empty, assert that the glyph is attached to that column
// title specifically — not any other.
//
// ct-events + column 1 (TIME) is the regression case for the §6 double-glyph bug:
// with the bug both TIME and EVENT carry the glyph (count=2).
// After the fix, count=1 and the glyph is on 2:TIME only.
//
// Column headers use number prefixes: "1:V", "2:TIME", "3:ACTOR", etc.
// The sort glyph appears after the full prefix+title: "2:TIME↓".
// ===========================================================================

func TestSortIndicator_ExactlyOnePerSort(t *testing.T) {
	cases := []struct {
		name         string
		typeName     string
		sortColIdx   int // 0-based column index; views.SortColNone (-1) = no sort
		sortAsc      bool
		wantCount    int
		wantOnColumn string // non-empty: assert the glyph appears immediately after this text
	}{
		// ct-events + col 1 (TIME) descending: BUG CASE — should be 1 glyph on 2:TIME, not 2.
		// The config-driven ct-events column order is: V[0], TIME[1], ACTOR[2], ORIGIN[3],
		// EVENT[4], TARGET[5], OUTCOME[6]. Key "2" selects column 1 (TIME).
		// This test FAILS against HEAD until the P3 coder ships the sortColIdx fix.
		{
			name:         "ct-events_TIME_col1_desc",
			typeName:     "ct-events",
			sortColIdx:   1, // TIME column (0-based index 1, key "2")
			sortAsc:      false,
			wantCount:    1,
			wantOnColumn: "2:TIME", // header prefix + title; glyph appended → "2:TIME↓"
		},
		// ct-events + col 1 (TIME) ascending: same fix, different glyph direction.
		{
			name:         "ct-events_TIME_col1_asc",
			typeName:     "ct-events",
			sortColIdx:   1,
			sortAsc:      true,
			wantCount:    1,
			wantOnColumn: "2:TIME",
		},
		// ec2 + sort by column — verify no regression on a non-ct resource.
		// ec2 default config columns include a launch_time-like column; check count only.
		{
			name:         "ec2_col0",
			typeName:     "ec2",
			sortColIdx:   0,
			sortAsc:      false,
			wantCount:    1,
			wantOnColumn: "", // check count only; ec2 col 0 varies by config
		},
		// dbi + sort by col 0: exactly one glyph.
		{
			name:         "dbi_col0",
			typeName:     "dbi",
			sortColIdx:   0,
			sortAsc:      false,
			wantCount:    1,
			wantOnColumn: "",
		},
		// ct-events + col 4 (EVENT): regression for the legacy substring-match
		// bug. Sorting by the EVENT column must place the glyph on 5:EVENT and
		// nowhere else — not on the TIME column which also contains "time" in
		// its key.
		{
			name:         "ct-events_EVENT_col4",
			typeName:     "ct-events",
			sortColIdx:   4, // EVENT column (0-based index 4, key "5")
			sortAsc:      true,
			wantCount:    1,
			wantOnColumn: "5:EVENT",
		},
		// ec2 + col 1: verify exactly one glyph on the second column.
		{
			name:         "ec2_col1",
			typeName:     "ec2",
			sortColIdx:   1,
			sortAsc:      true,
			wantCount:    1,
			wantOnColumn: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := buildSortModel(t, tc.typeName, tc.sortColIdx, tc.sortAsc)

			view := m.View()
			if view == "No resources found" || strings.HasPrefix(view, "Loading") {
				t.Fatalf("model did not render a data view; got: %q", view)
			}

			// Header is the first line of View() output.
			hdr := headerLine(view)
			// Strip ANSI before counting so lipgloss rendering doesn't interfere.
			plainHdr := stripANSI(hdr)

			got := countSortGlyphs(plainHdr)
			if got != tc.wantCount {
				t.Errorf(
					"sort glyph count = %d, want %d\n"+
						"  header (plain): %q\n"+
						"  sortColIdx=%d asc=%v type=%s\n"+
						"  bug: colHeaderTitle substring-matches isAgeKey against column title,\n"+
						"  causing both TIME and EVENT columns to get the glyph on ct-events",
					got, tc.wantCount, plainHdr, tc.sortColIdx, tc.sortAsc, tc.typeName,
				)
			}

			// If we expect a specific column to carry the glyph, verify it.
			if tc.wantCount == 1 && tc.wantOnColumn != "" {
				// The glyph should appear immediately after the column prefix+title text.
				wantSubstr := tc.wantOnColumn + "\u2193"
				if tc.sortAsc {
					wantSubstr = tc.wantOnColumn + "\u2191"
				}
				if !strings.Contains(plainHdr, wantSubstr) {
					t.Errorf(
						"sort glyph not on expected column\n"+
							"  want glyph immediately after %q (i.e., substring %q)\n"+
							"  header (plain): %q\n"+
							"  sortColIdx=%d asc=%v type=%s\n"+
							"  bug: glyph may be on wrong column (e.g. EVENT instead of TIME)",
						tc.wantOnColumn, wantSubstr, plainHdr, tc.sortColIdx, tc.sortAsc, tc.typeName,
					)
				}
			}
		})
	}
}

// ===========================================================================
// TestSortIndicator_NoGlyphWhenUnsorted
//
// When no sort key has been pressed (default for most resource types), no
// glyph appears. views.SortColNone (-1) is the initial sortColIdx value.
// ===========================================================================

func TestSortIndicator_NoGlyphWhenUnsorted(t *testing.T) {
	for _, shortName := range []string{"ec2", "dbi"} {
		t.Run(shortName, func(t *testing.T) {
			os.Unsetenv("NO_COLOR")
			styles.Reinit()

			td := resource.FindResourceType(shortName)
			if td == nil {
				t.Fatalf("resource type %q not found in registry", shortName)
			}

			cfg := config.DefaultConfig()
			k := keys.Default()
			m := views.NewResourceList(*td, cfg, k)
			m.SetSize(200, 20)
			m, _ = m.Init()
			res := syntheticResourceForType(shortName)
			m, _ = m.Update(messages.ResourcesLoaded{
				ResourceType: shortName,
				Resources:    []resource.Resource{res},
			})

			// Do NOT apply any sort — defaults to SortColNone for ec2/dbi.
			colIdx, _ := m.SortState()
			if colIdx != views.SortColNone {
				t.Skipf("resource type %q initialises with non-None sort (col %d), skipping unsorted test", shortName, colIdx)
			}

			view := m.View()
			hdr := headerLine(view)
			plainHdr := stripANSI(hdr)

			if got := countSortGlyphs(plainHdr); got != 0 {
				t.Errorf(
					"expected 0 sort glyphs when sortColIdx=SortColNone, got %d; header: %q",
					got, plainHdr,
				)
			}
		})
	}
}

// ===========================================================================
// TestSortIndicator_CTEvents_TimeSort_OnlyTIMEColumn
//
// Explicit regression: the ↓ glyph must NOT appear anywhere near the word
// "EVENT" in the ct-events header when sorting by the TIME column
// (column index 1, key "2").
//
// "Sort by time" means pressing "2" to activate column 1 (TIME). The header
// renders as "2:TIME↓". This is the most precise catch for the §6
// double-glyph bug from the legacy substring-match era.
// ===========================================================================

func TestSortIndicator_CTEvents_TimeSort_OnlyTIMEColumn(t *testing.T) {
	// ct-events config column order: V[0], TIME[1], ACTOR[2], ORIGIN[3], EVENT[4], ...
	// Sort by column 1 (TIME) descending.
	m := buildSortModel(t, "ct-events", 1, false)

	view := m.View()
	if strings.HasPrefix(view, "Loading") || view == "No resources found" {
		t.Fatalf("model did not render; got: %q", view)
	}

	hdr := headerLine(view)
	plainHdr := stripANSI(hdr)

	// The EVENT column (rendered as "5:EVENT") must NOT carry a sort glyph.
	if strings.Contains(plainHdr, "EVENT\u2193") || strings.Contains(plainHdr, "EVENT\u2191") {
		t.Errorf(
			"sort glyph incorrectly appears on EVENT column\n"+
				"  header: %q\n"+
				"  regression: legacy substring matching would decorate any column whose\n"+
				"  key or title contained \"event\"/\"time\"/etc. when sorting by the time\n"+
				"  column. Current code uses exact sortColIdx comparison — do not\n"+
				"  reintroduce title-substring matching in the sort-indicator path.",
			plainHdr,
		)
	}

	// The TIME column (rendered as "2:TIME") MUST carry the ↓ glyph (descending = newest first).
	if !strings.Contains(plainHdr, "2:TIME\u2193") {
		t.Errorf(
			"sort glyph missing from TIME column\n"+
				"  header: %q\n"+
				"  want: 2:TIME column to carry ↓ glyph when sorting column 1 descending",
			plainHdr,
		)
	}
}

// stripANSI is defined in tests/unit/helpers_test.go (package unit).
// rlKeyPress is defined in tests/unit/tui_resourcelist_test.go (package unit).
// config.DefaultConfig() is used directly (configForType lives in package unit_test
// and is not visible to package unit).
