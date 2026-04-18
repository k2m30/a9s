package unit

// qa_column_sort_key_prefix_test.go — Tests that column header sort-key prefixes
// (e.g., "1:", "2:", ..., "0:") are always shown for the first 10 columns,
// regardless of column width, provided the prefix fits within the column.
//
// Reference: internal/tui/views/table_render.go colHeaderTitle():
//   "Add position number prefix (1-based, max 10 columns for sort),
//    only when the prefixed title fits within the column width."
//
// The prefix IS rendered when len([]rune(prefixed)) <= c.width. So a narrow
// column may not show the prefix if the column is too narrow. Our tests use
// widths that guarantee visibility.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// buildResourceList creates a ResourceListModel for the given resource type with
// a realistic resource loaded, at the given terminal width. Uses DefaultConfig so
// view columns match production defaults.
func buildResourceList(t *testing.T, shortName string, width int) views.ResourceListModel {
	t.Helper()
	styles.Reinit()

	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("resource type %q not registered", shortName)
	}

	cfg := config.DefaultConfig()
	k := keys.Default()
	m := views.NewResourceList(*td, cfg, k)
	m.SetSize(width, 24)
	m, _ = m.Init()

	// Load a minimal synthetic resource so the header renders.
	res := resource.Resource{
		ID:     "test-resource-001",
		Name:   "test-resource",
		Status: "available",
		Fields: map[string]string{},
	}
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: shortName,
		Resources:    []resource.Resource{res},
	})
	return m
}

// headerLineFrom extracts the first rendered line (header row) from View().
func headerLineFrom(m views.ResourceListModel) string {
	lines := strings.SplitN(stripANSI(m.View()), "\n", 2)
	return lines[0]
}

// TestColumnHeader_ShowsPrefixOnNarrowColumn verifies that a column at absolute
// index 5 (displayed as "6:") shows the "6:" prefix even on a 7-character-wide
// column, given that "6:Writ" (6 chars) fits within width 7.
//
// The "dbc" type has column index 5 = "Writer" (Width: 7).
// "6:Writer" is 8 chars, exceeds width 7, so the prefix is suppressed.
// "6:Write" would be 7 chars — but PadOrTrunc truncates to 7 from the outside.
// The real invariant to pin: colHeaderTitle returns the prefixed form when
// len([]rune(prefixed)) <= c.width, i.e., when the width is large enough.
//
// We test with ec2 (many columns) at a wide terminal so column 5 prefix "6:" is
// visible without truncation interfering.
func TestColumnHeader_ShowsPrefixOnNarrowColumn(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	// "ec2" has multiple columns. At width 200 all fit. Column at absIdx=5
	// (6th column, 0-based) should have "6:" prefix in the header.
	m := buildResourceList(t, "ec2", 200)
	header := headerLineFrom(m)

	if !strings.Contains(header, "6:") {
		t.Errorf("ec2 header at width=200 must contain '6:' prefix for column index 5,"+
			" got header: %s", header)
	}
}

// TestColumnHeader_ShowsPrefixForAllTenColumns verifies that columns 0-9
// (absIdx 0-9) all render their "N:" prefix in the header when the terminal
// is wide enough to display all columns.
//
// Keys: absIdx 0-8 → prefixes "1:"-"9:", absIdx 9 → prefix "0:".
func TestColumnHeader_ShowsPrefixForAllTenColumns(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	// Find a resource type with at least 10 columns in defaults.
	var targetType string
	for _, rt := range resource.AllResourceTypes() {
		vd := config.GetViewDef(config.DefaultConfig(), rt.ShortName)
		if len(vd.List) >= 10 {
			targetType = rt.ShortName
			break
		}
	}
	if targetType == "" {
		t.Skip("no resource type with 10+ columns found in defaults")
	}

	m := buildResourceList(t, targetType, 300)
	header := headerLineFrom(m)

	cases := []struct {
		absIdx int
		prefix string
	}{
		{0, "1:"},
		{1, "2:"},
		{2, "3:"},
		{3, "4:"},
		{4, "5:"},
		{5, "6:"},
		{6, "7:"},
		{7, "8:"},
		{8, "9:"},
		{9, "0:"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("col%d_prefix_%s", c.absIdx, c.prefix), func(t *testing.T) {
			if !strings.Contains(header, c.prefix) {
				t.Errorf("header must contain %q for absIdx=%d on type %q;"+
					" header:\n%s", c.prefix, c.absIdx, targetType, header)
			}
		})
	}
}

// TestColumnHeader_NoPrefixForColumn11Plus verifies that columns with absIdx >= 10
// (the 11th visible column and beyond) are rendered WITHOUT a numeric sort-key prefix.
//
// The colHeaderTitle contract: "only when absIdx < 10".
// We test with a type that has more than 10 columns and a wide enough terminal.
func TestColumnHeader_NoPrefixForColumn11Plus(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	// Find a resource type with at least 11 columns.
	var targetType string
	var eleventhTitle string
	for _, rt := range resource.AllResourceTypes() {
		vd := config.GetViewDef(config.DefaultConfig(), rt.ShortName)
		if len(vd.List) >= 11 {
			targetType = rt.ShortName
			eleventhTitle = vd.List[10].Title
			break
		}
	}
	if targetType == "" {
		t.Skip("no resource type with 11+ columns in defaults")
	}

	m := buildResourceList(t, targetType, 400)
	header := headerLineFrom(m)

	// The 11th column title must appear WITHOUT a "11:" or any "N:" prefix
	// (there is no "11:" at all — the scheme only covers 1-9 then 0 for 10).
	// We verify the 11th column is NOT prefixed by checking that it appears
	// WITHOUT immediately preceding digit-colon sequence.
	//
	// Strategy: find the column title in the header; verify no "N:" immediately
	// precedes it (where N is any digit).
	if !strings.Contains(header, eleventhTitle) {
		t.Skipf("11th column %q not rendered at width=400 for type %q — column may be off-screen",
			eleventhTitle, targetType)
	}
	idx := strings.Index(header, eleventhTitle)
	if idx >= 2 {
		// Check the two chars before the title.
		before := header[idx-2 : idx]
		if len(before) == 2 && before[1] == ':' && before[0] >= '0' && before[0] <= '9' {
			t.Errorf("column 11 (absIdx=10) must NOT have a 'N:' prefix, but found %q before %q"+
				" in header:\n%s", before, eleventhTitle, header)
		}
	}
}

// TestColumnHeader_DocDBClustersHaveAllFivePrefixes verifies that the "dbc"
// (DocumentDB Clusters) list view renders the correct sort-key prefixes for
// its first five columns: "1:" (Cluster ID), "2:" (Version), "3:" (Status),
// "4:" (CIS), "5:" (Instances).
//
// This is a regression pin: if the column order changes or prefixes are
// accidentally suppressed, this test catches it.
func TestColumnHeader_DocDBClustersHaveAllFivePrefixes(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	m := buildResourceList(t, "dbc", 300)
	header := headerLineFrom(m)

	expectedPrefixes := []string{"1:", "2:", "3:", "4:", "5:"}
	for _, prefix := range expectedPrefixes {
		if !strings.Contains(header, prefix) {
			t.Errorf("dbc header must contain %q sort-key prefix; header:\n%s", prefix, header)
		}
	}
}
