package unit

// Tests for §6 of docs/design/ct-event-list-v2.md: sort indicator binding.
//
// Bug: colHeaderTitle matches SortAge against any column whose key or title
// contains "time"/"event"/"date" etc. For ct-events, both the TIME column
// (key="time") and the EVENT column (title="EVENT", which contains "event")
// match isAgeKey → both get the ↓/↑ glyph.
//
// Contract (§6): exactly ONE column carries the sort glyph for any active sort.
// The fix is to bind the indicator to one explicit column via sortColKey on
// ResourceListModel instead of substring matching in colHeaderTitle.
//
// These tests compile against HEAD but the ct-events SortAge test WILL FAIL
// until the P3 coder ships the sortColKey field fix.

import (
	"os"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
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
// sort field. Uses SortAsc=false (descending) to match ct-events default.
//
// For ct-events the model must receive viewConfig so the default SortAge
// initialisation in NewResourceList kicks in (§6 requirement).
func buildSortModel(
	t *testing.T,
	shortName string,
	sort views.SortField,
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
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: shortName,
		Resources:    []resource.Resource{res},
	})

	// Override sort to the requested field and direction.
	// We do this via the WithSort setter; if that is not exposed, we simulate
	// the key press that activates it. Use the WithSort method (added by P3
	// coder) when available; fall back to key-press simulation.
	m = applySort(t, m, sort, sortAsc)

	return m
}

// applySort sets the sort field on the model. Uses the public SortState()
// accessor to verify the sort was applied. The model's sort is set by
// the P3 coder's WithSort setter; until that ships we simulate via key presses.
//
// Key bindings (keys.go):
//   SortByName = "N"
//   SortByID   = "I"
//   SortByAge  = "A"
//
// If the current sort already matches the target field, the direction may be
// toggled by a second key press; we handle that case explicitly.
func applySort(t *testing.T, m views.ResourceListModel, sort views.SortField, wantAsc bool) views.ResourceListModel {
	t.Helper()

	if sort == views.SortNone {
		return m
	}

	// Map sort field to the key character.
	var keyChar string
	switch sort {
	case views.SortName:
		keyChar = "N"
	case views.SortID:
		keyChar = "I"
	case views.SortAge:
		keyChar = "A"
	default:
		t.Fatalf("applySort: unhandled SortField %v", sort)
	}

	msg := rlKeyPress(keyChar)

	// Press the sort key once to activate the sort field.
	m, _ = m.Update(msg)

	// Check direction — a second press toggles ascending/descending.
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
			Status: "ct-info",
			Fields: map[string]string{
				"time":         "Apr 07 17:00:00",
				"event_time":   "2026-04-07T17:00:00Z",
				"_ct.verb":     "R",
				"_ct.actor":    "test-user",
				"_ct.origin":   "CLI",
				"_ct.target":   "i-0test001",
				"_ct.outcome":  "OK",
			},
		}
	case "ec2":
		return resource.Resource{
			ID:     "i-0test001",
			Name:   "test-instance",
			Status: "running",
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
			Status: "available",
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
			Status: "available",
			Fields: map[string]string{},
		}
	}
}

// ===========================================================================
// TestSortIndicator_ExactlyOnePerSort
//
// For each (resource-type, sort-field) pair: assert the rendered header
// contains exactly the expected number of sort glyphs (↑ or ↓), and when
// wantOnColumn is non-empty, assert that the glyph is attached to that column
// title specifically — not any other.
//
// ct-events + SortAge is the regression case for the §6 double-glyph bug:
// with the bug both TIME and EVENT carry the glyph (count=2).
// After the fix, count=1 and the glyph is on TIME only.
// ===========================================================================

func TestSortIndicator_ExactlyOnePerSort(t *testing.T) {
	cases := []struct {
		name         string
		typeName     string
		sort         views.SortField
		sortAsc      bool
		wantCount    int
		wantOnColumn string // non-empty: assert the glyph appears immediately after this text
	}{
		// ct-events + SortAge: BUG CASE — should be 1 glyph on TIME, not 2 (TIME+EVENT).
		// This test FAILS against HEAD until the P3 coder ships the sortColKey fix.
		{
			name: "ct-events_Age_desc",
			typeName: "ct-events",
			sort:     views.SortAge,
			sortAsc:  false,
			wantCount:    1,
			wantOnColumn: "TIME",
		},
		// ct-events + SortAge ascending: same fix, different glyph direction.
		{
			name: "ct-events_Age_asc",
			typeName: "ct-events",
			sort:     views.SortAge,
			sortAsc:  true,
			wantCount:    1,
			wantOnColumn: "TIME",
		},
		// ec2 + SortAge: verify no regression on a non-ct resource.
		{
			name: "ec2_Age",
			typeName: "ec2",
			sort:     views.SortAge,
			sortAsc:  false,
			wantCount:    1,
			wantOnColumn: "", // ec2 has no standard "age" column in defaults; check count only
		},
		// rds + SortAge: dbi has no age-like column in defaults, so no glyph renders.
		// This proves the sortColKey=="" branch produces zero glyphs (not a fallback).
		{
			name: "rds_Age",
			typeName: "dbi",
			sort:     views.SortAge,
			sortAsc:  false,
			wantCount:    0,
			wantOnColumn: "",
		},
		// ct-events + SortName: ct-events has no "name" column in its layout (§8).
		// isAgeKey does not match name-related columns, so SortName → 0 glyphs.
		// (After the fix this becomes an exact-key match; no column has key="name".)
		{
			name: "ct-events_Name",
			typeName: "ct-events",
			sort:     views.SortName,
			sortAsc:  true,
			wantCount:    0,
			wantOnColumn: "",
		},
		// ec2 + SortID: verify exactly one glyph on the ID-related column.
		{
			name: "ec2_ID",
			typeName: "ec2",
			sort:     views.SortID,
			sortAsc:  true,
			wantCount:    1,
			wantOnColumn: "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m := buildSortModel(t, tc.typeName, tc.sort, tc.sortAsc)

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
						"  sort=%v asc=%v type=%s\n"+
						"  bug: colHeaderTitle substring-matches isAgeKey against column title,\n"+
						"  causing both TIME and EVENT columns to get the glyph on ct-events",
					got, tc.wantCount, plainHdr, tc.sort, tc.sortAsc, tc.typeName,
				)
			}

			// If we expect a specific column to carry the glyph, verify it.
			if tc.wantCount == 1 && tc.wantOnColumn != "" {
				// The glyph should appear immediately after the column title text.
				wantSubstr := tc.wantOnColumn + "\u2193"
				if tc.sortAsc {
					wantSubstr = tc.wantOnColumn + "\u2191"
				}
				if !strings.Contains(plainHdr, wantSubstr) {
					t.Errorf(
						"sort glyph not on expected column\n"+
							"  want glyph immediately after %q (i.e., substring %q)\n"+
							"  header (plain): %q\n"+
							"  sort=%v asc=%v type=%s\n"+
							"  bug: glyph may be on wrong column (e.g. EVENT instead of TIME)",
						tc.wantOnColumn, wantSubstr, plainHdr, tc.sort, tc.sortAsc, tc.typeName,
					)
				}
			}
		})
	}
}

// ===========================================================================
// TestSortIndicator_NoGlyphWhenSortNone
//
// When sort is SortNone (default for most resource types), no glyph appears.
// ===========================================================================

func TestSortIndicator_NoGlyphWhenSortNone(t *testing.T) {
	for _, shortName := range []string{"ec2", "dbi"} {
		shortName := shortName
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
			m, _ = m.Update(messages.ResourcesLoadedMsg{
				ResourceType: shortName,
				Resources:    []resource.Resource{res},
			})

			// Do NOT apply any sort — defaults to SortNone for ec2/dbi.
			sf, _ := m.SortState()
			if sf != views.SortNone {
				t.Skipf("resource type %q initialises with non-None sort (%v), skipping SortNone test", shortName, sf)
			}

			view := m.View()
			hdr := headerLine(view)
			plainHdr := stripANSI(hdr)

			if got := countSortGlyphs(plainHdr); got != 0 {
				t.Errorf(
					"expected 0 sort glyphs when sort=SortNone, got %d; header: %q",
					got, plainHdr,
				)
			}
		})
	}
}

// ===========================================================================
// TestSortIndicator_CTEvents_SortAge_OnlyTIMEColumn
//
// Explicit regression test: after the P3 fix, the ↓ glyph must NOT appear
// anywhere near the word "EVENT" in the ct-events header when sorting by age.
//
// This test is the most precise catch for the §6 double-glyph bug.
// It will FAIL against HEAD (current code decorates both TIME and EVENT).
// ===========================================================================

func TestSortIndicator_CTEvents_SortAge_OnlyTIMEColumn(t *testing.T) {
	m := buildSortModel(t, "ct-events", views.SortAge, false)

	view := m.View()
	if strings.HasPrefix(view, "Loading") || view == "No resources found" {
		t.Fatalf("model did not render; got: %q", view)
	}

	hdr := headerLine(view)
	plainHdr := stripANSI(hdr)

	// The EVENT column title must NOT carry a sort glyph.
	if strings.Contains(plainHdr, "EVENT\u2193") || strings.Contains(plainHdr, "EVENT\u2191") {
		t.Errorf(
			"sort glyph incorrectly appears on EVENT column\n"+
				"  header: %q\n"+
				"  bug: isAgeKey(\"EVENT\") returns true because \"event\" is in its substring list;\n"+
				"  colHeaderTitle decorates both TIME and EVENT when sorting by age.\n"+
				"  fix: use exact sortColKey comparison instead of isAgeKey(c.title).",
			plainHdr,
		)
	}

	// The TIME column title MUST carry the ↓ glyph (descending = newest first).
	if !strings.Contains(plainHdr, "TIME\u2193") {
		t.Errorf(
			"sort glyph missing from TIME column\n"+
				"  header: %q\n"+
				"  want: TIME column to carry ↓ glyph when SortAge descending",
			plainHdr,
		)
	}
}

// stripANSI is defined in tests/unit/helpers_test.go (package unit).
// rlKeyPress is defined in tests/unit/tui_resourcelist_test.go (package unit).
// config.DefaultConfig() is used directly (configForType lives in package unit_test
// and is not visible to package unit).
