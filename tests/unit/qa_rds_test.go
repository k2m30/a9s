package unit

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ===========================================================================
// Helpers for RDS tests
// ===========================================================================

// rdsTypeDef returns the RDS type definition from the registry.
func rdsTypeDef() resource.ResourceTypeDef {
	td := resource.FindResourceType("dbi")
	if td == nil {
		panic("rds resource type not found")
	}
	return *td
}

// rdsLoadedModel returns a ResourceListModel loaded with fixture RDS data.
func rdsLoadedModel(t *testing.T) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rdsTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(160, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "dbi",
		Resources:    fixtureRDSInstances(),
	})
	return m
}

// rdsLoadedModelWide returns a model with a wide terminal to show all columns.
func rdsLoadedModelWide(t *testing.T) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rdsTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(200, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "dbi",
		Resources:    fixtureRDSInstances(),
	})
	return m
}

// fixtureRDSInstancesExtended adds extra instances for edge-case testing:
// a stopped instance, a creating instance (no endpoint), and a postgres instance.
func fixtureRDSInstancesExtended() []resource.Resource {
	base := fixtureRDSInstances()
	return append(base,
		resource.Resource{
			ID:     "stopped-db",
			Name:   "stopped-db",
			Status: "stopped",
			Fields: map[string]string{
				"db_identifier":  "stopped-db",
				"engine":         "mysql",
				"engine_version": "8.0.35",
				"status":         "stopped",
				"class":          "db.r5.large",
				"endpoint":       "stopped-db.abc123.us-east-1.rds.amazonaws.com",
				"multi_az":       "Yes",
			},
		},
		resource.Resource{
			ID:     "creating-db",
			Name:   "creating-db",
			Status: "creating",
			Fields: map[string]string{
				"db_identifier":  "creating-db",
				"engine":         "postgres",
				"engine_version": "16.2",
				"status":         "creating",
				"class":          "db.t3.medium",
				"endpoint":       "",
				"multi_az":       "No",
			},
		},
		resource.Resource{
			ID:     "prod-postgres-primary",
			Name:   "prod-postgres-primary",
			Status: "available",
			Fields: map[string]string{
				"db_identifier":  "prod-postgres-primary",
				"engine":         "postgres",
				"engine_version": "14.9",
				"status":         "available",
				"class":          "db.r5.xlarge",
				"endpoint":       "prod-postgres-primary.abc123.us-east-1.rds.amazonaws.com",
				"multi_az":       "Yes",
			},
		},
	)
}

// rdsExtendedModel loads a model with the extended fixture set.
func rdsExtendedModel(t *testing.T) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rdsTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(200, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "dbi",
		Resources:    fixtureRDSInstancesExtended(),
	})
	return m
}

// rdsKeyPress creates a tea.KeyPressMsg for a printable character.
func rdsKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

// ===========================================================================
// A.2 Column Layout
// ===========================================================================

func TestQA_RDS_ListColumns_AllSevenPresent(t *testing.T) {
	m := rdsLoadedModelWide(t)
	out := m.View()

	expectedHeaders := []string{
		"DB Identifier",
		"Engine",
		"Version",
		"Status",
		"Class",
		"Endpoint",
		"Multi-AZ",
	}
	for _, hdr := range expectedHeaders {
		if !strings.Contains(out, hdr) {
			t.Errorf("RDS list view missing column header %q", hdr)
		}
	}
}

func TestQA_RDS_ListColumns_CorrectOrder(t *testing.T) {
	m := rdsLoadedModelWide(t)
	out := m.View()
	plain := stripANSI(out)
	lines := strings.Split(plain, "\n")

	// The header line is the first line (line 0).
	headerLine := lines[0]

	// Verify the expected order by checking column positions.
	headers := []string{"DB Identifier", "Engine", "Version", "Status", "Class", "Endpoint", "Multi-AZ"}
	prevPos := -1
	for _, h := range headers {
		pos := strings.Index(headerLine, h)
		if pos < 0 {
			t.Errorf("column header %q not found in header line: %q", h, headerLine)
			continue
		}
		if pos <= prevPos {
			t.Errorf("column %q at position %d should be after previous column at position %d", h, pos, prevPos)
		}
		prevPos = pos
	}
}

func TestQA_RDS_ListColumns_ColumnWidths(t *testing.T) {
	// Verify that the resource type definition has the correct column widths per spec.
	td := rdsTypeDef()
	expectedWidths := map[string]int{
		"db_identifier":  28,
		"engine":         12,
		"engine_version": 10,
		"status":         14,
		"class":          16,
		"endpoint":       40,
		"multi_az":       10,
	}
	for _, col := range td.Columns {
		expected, ok := expectedWidths[col.Key]
		if !ok {
			continue
		}
		if col.Width != expected {
			t.Errorf("column %q width: expected %d, got %d", col.Key, expected, col.Width)
		}
	}
}

func TestQA_RDS_ListColumns_NoSeparatorBelowHeaders(t *testing.T) {
	m := rdsLoadedModelWide(t)
	out := m.View()
	plain := stripANSI(out)
	lines := strings.Split(plain, "\n")

	for _, line := range lines {
		stripped := strings.TrimSpace(line)
		if stripped == "" {
			continue
		}
		allDash := true
		for _, ch := range stripped {
			if ch != '-' && ch != '_' && ch != '=' && ch != ' ' {
				allDash = false
				break
			}
		}
		if allDash && len(stripped) > 5 {
			t.Errorf("found separator-like row in RDS list: %q", stripped)
		}
	}
}

func TestQA_RDS_ListColumns_SpaceAlignedNotPipeSeparated(t *testing.T) {
	m := rdsLoadedModelWide(t)
	out := m.View()
	plain := stripANSI(out)
	lines := strings.Split(plain, "\n")

	for _, line := range lines {
		if strings.Contains(line, "|") {
			t.Errorf("found pipe character in RDS list output: %q", line)
		}
	}
}

// ===========================================================================
// A.3 Data Mapping
// ===========================================================================

func TestQA_RDS_ListData_DBIdentifier(t *testing.T) {
	m := rdsLoadedModelWide(t)
	out := m.View()

	for _, r := range fixtureRDSInstances() {
		id := r.Fields["db_identifier"]
		if !strings.Contains(out, id) {
			t.Errorf("RDS list missing DB Identifier %q", id)
		}
	}
}

func TestQA_RDS_ListData_Engine(t *testing.T) {
	m := rdsLoadedModelWide(t)
	out := m.View()

	for _, r := range fixtureRDSInstances() {
		engine := r.Fields["engine"]
		// Engine column is 12 chars wide; long values like "aurora-postgresql" (18 chars)
		// get truncated by PadOrTrunc. Check for the first 10 chars which will be present
		// whether the value is truncated or not.
		prefix := engine
		if len(prefix) > 10 {
			prefix = engine[:10]
		}
		if !strings.Contains(out, prefix) {
			t.Errorf("RDS list missing Engine value prefix %q (full: %q)", prefix, engine)
		}
	}
}

func TestQA_RDS_ListData_Version(t *testing.T) {
	m := rdsLoadedModelWide(t)
	out := m.View()

	for _, r := range fixtureRDSInstances() {
		ver := r.Fields["engine_version"]
		if !strings.Contains(out, ver) {
			t.Errorf("RDS list missing Version %q", ver)
		}
	}
}

func TestQA_RDS_ListData_Status(t *testing.T) {
	m := rdsLoadedModelWide(t)
	out := m.View()

	for _, r := range fixtureRDSInstances() {
		status := r.Fields["status"]
		if !strings.Contains(out, status) {
			t.Errorf("RDS list missing Status %q", status)
		}
	}
}

func TestQA_RDS_ListData_Class(t *testing.T) {
	m := rdsLoadedModelWide(t)
	out := m.View()

	for _, r := range fixtureRDSInstances() {
		class := r.Fields["class"]
		if !strings.Contains(out, class) {
			t.Errorf("RDS list missing Class %q", class)
		}
	}
}

func TestQA_RDS_ListData_Endpoint(t *testing.T) {
	m := rdsLoadedModelWide(t)
	out := m.View()

	for _, r := range fixtureRDSInstances() {
		ep := r.Fields["endpoint"]
		if ep == "" {
			continue
		}
		// Endpoint may be truncated; check for the first segment.
		prefix := ep
		if len(prefix) > 30 {
			prefix = ep[:30]
		}
		if !strings.Contains(out, prefix) {
			t.Errorf("RDS list missing Endpoint prefix %q", prefix)
		}
	}
}

func TestQA_RDS_ListData_MultiAZ(t *testing.T) {
	m := rdsLoadedModelWide(t)
	out := m.View()
	plain := stripANSI(out)

	// All fixture instances have Multi-AZ "No".
	if !strings.Contains(plain, "No") {
		t.Errorf("RDS list missing Multi-AZ value 'No'")
	}
}

func TestQA_RDS_ListData_RowCount(t *testing.T) {
	m := rdsLoadedModel(t)
	title := m.FrameTitle()
	expected := "dbi(2)"
	if title != expected {
		t.Errorf("FrameTitle: expected %q, got %q", expected, title)
	}
}

// ===========================================================================
// A.4 Status Coloring
// ===========================================================================

func TestQA_RDS_StatusColor_Available(t *testing.T) {
	// RowColorStyle("available") should use ColRunning (green #9ece6a).
	style := styles.RowColorStyle("available")
	rendered := style.Render("test-available")
	if !strings.Contains(rendered, "9ece6a") {
		// If the style system is active, it should apply color.
		// In NO_COLOR mode it won't. We verify the style function returns non-nil.
		if styles.NoColorActive() {
			t.Skip("NO_COLOR is set, skipping color assertion")
		}
		// Check that the rendered output is styled (has ANSI codes).
		if rendered == "test-available" {
			t.Error("RowColorStyle('available') should apply green color styling")
		}
	}
}

func TestQA_RDS_StatusColor_Stopped(t *testing.T) {
	style := styles.RowColorStyle("stopped")
	rendered := style.Render("test-stopped")
	if styles.NoColorActive() {
		t.Skip("NO_COLOR is set, skipping color assertion")
	}
	if rendered == "test-stopped" {
		t.Error("RowColorStyle('stopped') should apply red color styling")
	}
}

func TestQA_RDS_StatusColor_Creating(t *testing.T) {
	style := styles.RowColorStyle("creating")
	rendered := style.Render("test-creating")
	if styles.NoColorActive() {
		t.Skip("NO_COLOR is set, skipping color assertion")
	}
	if rendered == "test-creating" {
		t.Error("RowColorStyle('creating') should apply yellow color styling")
	}
}

func TestQA_RDS_StatusColor_Modifying(t *testing.T) {
	style := styles.RowColorStyle("modifying")
	rendered := style.Render("test-modifying")
	if styles.NoColorActive() {
		t.Skip("NO_COLOR is set, skipping color assertion")
	}
	if rendered == "test-modifying" {
		t.Error("RowColorStyle('modifying') should apply yellow color styling")
	}
}

func TestQA_RDS_StatusColor_Failed(t *testing.T) {
	style := styles.RowColorStyle("failed")
	rendered := style.Render("test-failed")
	if styles.NoColorActive() {
		t.Skip("NO_COLOR is set, skipping color assertion")
	}
	if rendered == "test-failed" {
		t.Error("RowColorStyle('failed') should apply red color styling")
	}
}

func TestQA_RDS_StatusColor_AvailableAndStoppedDifferent(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	availStyle := styles.RowColorStyle("available")
	stopStyle := styles.RowColorStyle("stopped")

	availRendered := availStyle.Render("X")
	stopRendered := stopStyle.Render("X")

	if availRendered == stopRendered {
		t.Error("available and stopped should have different coloring")
	}
}

func TestQA_RDS_StatusColor_AvailableAndCreatingDifferent(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	availStyle := styles.RowColorStyle("available")
	createStyle := styles.RowColorStyle("creating")

	if availStyle.Render("X") == createStyle.Render("X") {
		t.Error("available and creating should have different coloring")
	}
}

// ===========================================================================
// A.5 Edge Cases
// ===========================================================================

func TestQA_RDS_EdgeCase_CreatingInstanceNoEndpoint(t *testing.T) {
	m := rdsExtendedModel(t)
	out := m.View()
	plain := stripANSI(out)

	// The creating instance should appear in the list.
	if !strings.Contains(plain, "creating-db") {
		t.Error("creating instance should appear in the list")
	}

	// It should NOT show "null" or "<nil>".
	lines := strings.Split(plain, "\n")
	for _, line := range lines {
		if strings.Contains(line, "creating-db") {
			if strings.Contains(line, "<nil>") || strings.Contains(line, "null") {
				t.Errorf("creating instance row should not show <nil> or null: %q", line)
			}
		}
	}
}

func TestQA_RDS_EdgeCase_MultiEngineMix(t *testing.T) {
	m := rdsExtendedModel(t)
	out := m.View()
	plain := stripANSI(out)

	// Both postgres and mysql should be present.
	if !strings.Contains(plain, "postgres") {
		t.Error("expected 'postgres' engine in multi-engine list")
	}
	if !strings.Contains(plain, "mysql") {
		t.Error("expected 'mysql' engine in multi-engine list")
	}
}

func TestQA_RDS_EdgeCase_AuroraInstancePresent(t *testing.T) {
	m := rdsLoadedModelWide(t)
	out := m.View()
	plain := stripANSI(out)

	// "aurora-postgresql" is 18 chars but the Engine column is 12 wide,
	// so it gets truncated with PadOrTrunc to 11 chars + ellipsis = "aurora-post..."
	// Check that the aurora engine prefix is present (truncated).
	if !strings.Contains(plain, "aurora-post") {
		t.Error("expected truncated aurora-postgresql engine prefix 'aurora-post' in the list")
	}
}

func TestQA_RDS_EdgeCase_EmptyList(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rdsTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(160, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "dbi",
		Resources:    []resource.Resource{},
	})

	out := m.View()
	if !strings.Contains(out, "No resources found") {
		t.Errorf("empty RDS list should show 'No resources found', got: %q", out)
	}

	title := m.FrameTitle()
	if title != "dbi(0)" {
		t.Errorf("empty RDS list FrameTitle: expected %q, got %q", "dbi(0)", title)
	}
}

func TestQA_RDS_EdgeCase_MultiAZBooleanDisplay(t *testing.T) {
	m := rdsExtendedModel(t)
	out := m.View()
	plain := stripANSI(out)

	// Fixture data has "No" and "Yes" as multi_az values.
	if !strings.Contains(plain, "No") && !strings.Contains(plain, "false") {
		t.Error("expected Multi-AZ column to display 'No' or 'false'")
	}
}

// ===========================================================================
// A.6 Frame Title
// ===========================================================================

func TestQA_RDS_FrameTitle_ShowsCount(t *testing.T) {
	m := rdsLoadedModel(t)
	title := m.FrameTitle()
	if title != "dbi(2)" {
		t.Errorf("FrameTitle: expected %q, got %q", "dbi(2)", title)
	}
}

func TestQA_RDS_FrameTitle_FilteredCount(t *testing.T) {
	m := rdsLoadedModel(t)
	m.SetFilter("dbc")

	title := m.FrameTitle()
	if title != "dbi(1/2)" {
		t.Errorf("FrameTitle with filter: expected %q, got %q", "dbi(1/2)", title)
	}
}

func TestQA_RDS_FrameTitle_ClearedFilter(t *testing.T) {
	m := rdsLoadedModel(t)
	m.SetFilter("dbc")
	m.SetFilter("")

	title := m.FrameTitle()
	if title != "dbi(2)" {
		t.Errorf("FrameTitle after clearing filter: expected %q, got %q", "dbi(2)", title)
	}
}

// ===========================================================================
// A.7 Sorting
// ===========================================================================

func TestQA_RDS_Sort_ByNameAscending(t *testing.T) {
	m := rdsExtendedModel(t)

	// Press 'N' for sort by name ascending.
	m, _ = m.Update(rdsKeyPress("N"))
	out := m.View()
	plain := stripANSI(out)

	// Name sort works on r.Name but the sort indicator only shows on columns
	// whose key/title contains "name". For RDS, "DB Identifier" does not contain
	// "name", so no indicator is shown. Instead, verify the data is actually sorted.
	posCreating := strings.Index(plain, "creating-db")
	posDocdb := strings.Index(plain, "test-docdb-1")
	if posCreating >= 0 && posDocdb >= 0 && posCreating > posDocdb {
		t.Error("expected creating-db before test-docdb-1 in ascending name sort")
	}
}

func TestQA_RDS_Sort_ByNameDescending(t *testing.T) {
	m := rdsExtendedModel(t)

	// Press 'N' twice for descending.
	m, _ = m.Update(rdsKeyPress("N"))
	m, _ = m.Update(rdsKeyPress("N"))
	out := m.View()
	plain := stripANSI(out)

	// Verify descending order: stopped-db should come before creating-db.
	posStopped := strings.Index(plain, "stopped-db")
	posCreating := strings.Index(plain, "creating-db")
	if posStopped >= 0 && posCreating >= 0 && posStopped > posCreating {
		t.Error("expected stopped-db before creating-db in descending name sort")
	}
}

func TestQA_RDS_Sort_ByID(t *testing.T) {
	m := rdsExtendedModel(t)

	// Press 'I' for sort by ID.
	m, _ = m.Update(rdsKeyPress("I"))
	out := m.View()

	// DB Identifier column key is "db_identifier" which contains "id",
	// so the sort indicator should appear on that column.
	if !strings.Contains(out, "\u2191") && !strings.Contains(out, "\u2193") {
		t.Error("expected sort indicator after pressing I")
	}
}

func TestQA_RDS_Sort_IndicatorOnlyOneColumn(t *testing.T) {
	m := rdsExtendedModel(t)

	// Sort by ID (which matches the "DB Identifier" column key containing "id").
	m, _ = m.Update(rdsKeyPress("I"))
	out := m.View()
	plain := stripANSI(out)
	lines := strings.Split(plain, "\n")

	headerLine := lines[0]
	arrowCount := strings.Count(headerLine, "\u2191") + strings.Count(headerLine, "\u2193")
	if arrowCount != 1 {
		t.Errorf("expected exactly 1 sort arrow in header, got %d in: %q", arrowCount, headerLine)
	}
}

// ===========================================================================
// A.8 Filtering
// ===========================================================================

func TestQA_RDS_Filter_ByPartialName(t *testing.T) {
	m := rdsExtendedModel(t)
	m.SetFilter("prod")

	out := m.View()
	plain := stripANSI(out)

	if !strings.Contains(plain, "prod-postgres-primary") {
		t.Error("filter 'prod' should show prod-postgres-primary")
	}
	if strings.Contains(plain, "stopped-db") {
		t.Error("filter 'prod' should NOT show stopped-db")
	}
}

func TestQA_RDS_Filter_ByEngine(t *testing.T) {
	m := rdsExtendedModel(t)
	m.SetFilter("postgres")

	out := m.View()
	plain := stripANSI(out)

	if !strings.Contains(plain, "prod-postgres-primary") {
		t.Error("filter 'postgres' should match prod-postgres-primary")
	}
}

func TestQA_RDS_Filter_NoMatches(t *testing.T) {
	m := rdsLoadedModel(t)
	m.SetFilter("zzz_nonexistent_zzz")

	out := m.View()
	if !strings.Contains(out, "No resources found") {
		t.Error("filter with no matches should show 'No resources found'")
	}

	title := m.FrameTitle()
	if !strings.Contains(title, "0/2") {
		t.Errorf("filter with no matches: FrameTitle should contain '0/2', got %q", title)
	}
}

func TestQA_RDS_Filter_CaseInsensitive(t *testing.T) {
	m := rdsExtendedModel(t)
	m.SetFilter("POSTGRES")

	out := m.View()
	plain := stripANSI(out)

	if !strings.Contains(plain, "postgres") {
		t.Error("case-insensitive filter 'POSTGRES' should match postgres instances")
	}
}

func TestQA_RDS_Filter_AcrossAllColumns(t *testing.T) {
	m := rdsExtendedModel(t)
	m.SetFilter("db.t3")

	out := m.View()
	plain := stripANSI(out)

	// db.t3.medium class instances should match.
	if !strings.Contains(plain, "db.t3") {
		t.Error("filter 'db.t3' should match instances with db.t3.medium class")
	}
}

func TestQA_RDS_Filter_ByStatus(t *testing.T) {
	m := rdsExtendedModel(t)
	m.SetFilter("stopped")

	out := m.View()
	plain := stripANSI(out)

	if !strings.Contains(plain, "stopped-db") {
		t.Error("filter 'stopped' should show stopped-db")
	}
}

// ===========================================================================
// A.9 Keyboard Navigation
// ===========================================================================

func TestQA_RDS_Navigation_CursorDown(t *testing.T) {
	m := rdsLoadedModel(t)

	// Initially cursor is at 0. Press j to move down.
	m, _ = m.Update(rdsKeyPress("j"))

	selected := m.SelectedResource()
	if selected == nil {
		t.Fatal("expected a selected resource after cursor down")
	}
	// Second fixture is "test-rds-1".
	if selected.ID != "test-rds-1" {
		t.Errorf("after j, expected selected ID %q, got %q", "test-rds-1", selected.ID)
	}
}

func TestQA_RDS_Navigation_CursorUp(t *testing.T) {
	m := rdsLoadedModel(t)

	// Move down first, then up.
	m, _ = m.Update(rdsKeyPress("j"))
	m, _ = m.Update(rdsKeyPress("k"))

	selected := m.SelectedResource()
	if selected == nil {
		t.Fatal("expected a selected resource after cursor up")
	}
	if selected.ID != "test-docdb-1" {
		t.Errorf("after j then k, expected selected ID %q, got %q", "test-docdb-1", selected.ID)
	}
}

func TestQA_RDS_Navigation_JumpToBottom(t *testing.T) {
	m := rdsLoadedModel(t)
	m, _ = m.Update(rdsKeyPress("G"))

	selected := m.SelectedResource()
	if selected == nil {
		t.Fatal("expected a selected resource after jump to bottom")
	}
	// Last fixture is "test-rds-1".
	if selected.ID != "test-rds-1" {
		t.Errorf("after G, expected last resource, got %q", selected.ID)
	}
}

func TestQA_RDS_Navigation_JumpToTop(t *testing.T) {
	m := rdsLoadedModel(t)
	m, _ = m.Update(rdsKeyPress("G"))
	m, _ = m.Update(rdsKeyPress("g"))

	selected := m.SelectedResource()
	if selected == nil {
		t.Fatal("expected a selected resource after jump to top")
	}
	if selected.ID != "test-docdb-1" {
		t.Errorf("after G then g, expected first resource, got %q", selected.ID)
	}
}

func TestQA_RDS_Navigation_EnterOpensDetail(t *testing.T) {
	m := rdsLoadedModel(t)

	// Press Enter — should return a cmd that produces NavigateMsg.
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on RDS list should return a command")
	}

	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("Enter should produce NavigateMsg, got %T", msg)
	}
	if nav.Target != messages.TargetDetail {
		t.Errorf("Enter should navigate to Detail, got target %d", nav.Target)
	}
	if nav.Resource == nil {
		t.Error("NavigateMsg.Resource should not be nil")
	}
}

func TestQA_RDS_Navigation_DOpensDetail(t *testing.T) {
	m := rdsLoadedModel(t)

	_, cmd := m.Update(rdsKeyPress("d"))
	if cmd == nil {
		t.Fatal("'d' on RDS list should return a command")
	}

	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("'d' should produce NavigateMsg, got %T", msg)
	}
	if nav.Target != messages.TargetDetail {
		t.Errorf("'d' should navigate to Detail, got target %d", nav.Target)
	}
}

func TestQA_RDS_Navigation_YOpensYAML(t *testing.T) {
	m := rdsLoadedModel(t)

	_, cmd := m.Update(rdsKeyPress("y"))
	if cmd == nil {
		t.Fatal("'y' on RDS list should return a command")
	}

	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("'y' should produce NavigateMsg, got %T", msg)
	}
	if nav.Target != messages.TargetYAML {
		t.Errorf("'y' should navigate to YAML, got target %d", nav.Target)
	}
}

// ===========================================================================
// A.10 Loading State
// ===========================================================================

func TestQA_RDS_LoadingSpinner(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rdsTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	out := m.View()
	if !strings.Contains(out, "Loading") {
		t.Errorf("RDS list should show 'Loading' before data arrives, got: %q", out)
	}
}

// ===========================================================================
// B. RDS Detail View
// ===========================================================================

func TestQA_RDS_Detail_ContainsAllFields(t *testing.T) {
	k := keys.Default()
	res := fixtureRDSInstances()[0]
	m := views.NewDetail(res, "dbi", nil, k)
	m.SetSize(120, 30)

	out := m.View()
	if out == "Initializing..." || out == "" {
		t.Fatal("Detail view should not be empty or initializing after SetSize")
	}

	// Fields map keys should appear in the detail.
	for fieldKey, fieldVal := range res.Fields {
		if !strings.Contains(out, fieldKey) {
			t.Errorf("Detail missing field key %q", fieldKey)
		}
		if fieldVal != "" && !strings.Contains(out, fieldVal) {
			// Value might be truncated, check first 20 chars.
			prefix := fieldVal
			if len(prefix) > 20 {
				prefix = fieldVal[:20]
			}
			if !strings.Contains(out, prefix) {
				t.Errorf("Detail missing field value %q (prefix %q)", fieldVal, prefix)
			}
		}
	}
}

func TestQA_RDS_Detail_FrameTitle(t *testing.T) {
	k := keys.Default()
	res := fixtureRDSInstances()[0]
	m := views.NewDetail(res, "dbi", nil, k)

	title := m.FrameTitle()
	if title != "test-docdb-1" {
		t.Errorf("Detail FrameTitle: expected %q, got %q", "test-docdb-1", title)
	}
}

func TestQA_RDS_Detail_EndpointField(t *testing.T) {
	k := keys.Default()
	res := fixtureRDSInstances()[0]
	m := views.NewDetail(res, "dbi", nil, k)
	m.SetSize(120, 30)

	out := m.View()
	endpointAddr := res.Fields["endpoint"]
	if !strings.Contains(out, endpointAddr[:20]) {
		t.Errorf("Detail should show endpoint address %q", endpointAddr)
	}
}

func TestQA_RDS_Detail_WithRawStruct_AllDetailPaths(t *testing.T) {
	k := keys.Default()

	// Use an RDS-only config to avoid non-deterministic map iteration
	// over all resource ViewDefs in renderFromConfig.
	rdsViewDef := config.DefaultViewDef("dbi")
	viewCfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"dbi": rdsViewDef,
		},
	}

	dbIdentifier := "prod-db-01"
	engine := "mysql"
	engineVersion := "8.0.35"
	status := "available"
	class := "db.r5.large"
	storageType := "gp3"
	az := "us-east-1a"

	rawStruct := rdstypes.DBInstance{
		DBInstanceIdentifier: &dbIdentifier,
		Engine:               &engine,
		EngineVersion:        &engineVersion,
		DBInstanceStatus:     &status,
		DBInstanceClass:      &class,
		MultiAZ:              boolPtr(true),
		AllocatedStorage:     int32Val(100),
		StorageType:          &storageType,
		AvailabilityZone:     &az,
		Endpoint: &rdstypes.Endpoint{
			Address: strPointer("prod-db-01.abc123.us-east-1.rds.amazonaws.com"),
			Port:    int32Val(3306),
		},
	}

	res := resource.Resource{
		ID:        "prod-db-01",
		Name:      "prod-db-01",
		Status:    "available",
		RawStruct: &rawStruct,
		Fields:    map[string]string{},
	}

	m := views.NewDetail(res, "dbi", viewCfg, k)
	m.SetSize(120, 30)

	out := m.View()
	plain := stripANSI(out)

	// Verify all detail fields from config are present.
	expectedValues := []string{
		"prod-db-01",
		"mysql",
		"8.0.35",
		"available",
		"db.r5.large",
		"gp3",
		"us-east-1a",
	}
	for _, val := range expectedValues {
		if !strings.Contains(plain, val) {
			t.Errorf("Detail with RawStruct missing value %q", val)
		}
	}

	// Endpoint should show as nested with Address and Port.
	if !strings.Contains(plain, "prod-db-01.abc123.us-east-1.rds.amazonaws.com") {
		t.Error("Detail should show Endpoint.Address")
	}
	if !strings.Contains(plain, "3306") {
		t.Error("Detail should show Endpoint Port 3306")
	}
}

func TestQA_RDS_Detail_CreatingInstanceNoEndpoint(t *testing.T) {
	k := keys.Default()
	viewCfg := config.DefaultConfig()

	dbIdentifier := "creating-db"
	status := "creating"

	rawStruct := rdstypes.DBInstance{
		DBInstanceIdentifier: &dbIdentifier,
		DBInstanceStatus:     &status,
		// Endpoint is nil during creation.
	}

	res := resource.Resource{
		ID:        "creating-db",
		Name:      "creating-db",
		Status:    "creating",
		RawStruct: &rawStruct,
		Fields:    map[string]string{},
	}

	m := views.NewDetail(res, "dbi", viewCfg, k)
	m.SetSize(120, 30)

	// Should not panic.
	out := m.View()
	plain := stripANSI(out)

	if strings.Contains(plain, "<nil>") {
		t.Error("Detail for creating instance should not show '<nil>'")
	}
	if !strings.Contains(plain, "creating-db") {
		t.Error("Detail for creating instance should show 'creating-db'")
	}
}

func TestQA_RDS_Detail_SwitchToYAML(t *testing.T) {
	k := keys.Default()
	res := fixtureRDSInstances()[0]
	m := views.NewDetail(res, "dbi", nil, k)
	m.SetSize(120, 30)

	_, cmd := m.Update(rdsKeyPress("y"))
	if cmd == nil {
		t.Fatal("'y' in detail view should return a command to navigate to YAML")
	}

	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("'y' should produce NavigateMsg, got %T", msg)
	}
	if nav.Target != messages.TargetYAML {
		t.Errorf("'y' in detail should navigate to YAML, got target %d", nav.Target)
	}
}

// ===========================================================================
// C. RDS YAML View
// ===========================================================================

func TestQA_RDS_YAML_ContainsFieldKeys(t *testing.T) {
	k := keys.Default()
	res := fixtureRDSInstances()[0]
	m := views.NewYAML(res, k)
	m.SetSize(120, 40)

	out := m.View()
	if out == "Initializing..." || out == "" {
		t.Fatal("YAML view should not be empty or initializing after SetSize")
	}

	// Check that field keys from the resource appear in the YAML.
	for key := range res.Fields {
		if !strings.Contains(out, key) {
			t.Errorf("YAML view missing key %q", key)
		}
	}
}

func TestQA_RDS_YAML_ContainsFieldValues(t *testing.T) {
	k := keys.Default()
	res := fixtureRDSInstances()[0]
	m := views.NewYAML(res, k)
	m.SetSize(120, 40)

	out := m.View()
	for _, val := range res.Fields {
		if val == "" {
			continue
		}
		if !strings.Contains(out, val) {
			t.Errorf("YAML view missing value %q", val)
		}
	}
}

func TestQA_RDS_YAML_FrameTitle(t *testing.T) {
	k := keys.Default()
	res := fixtureRDSInstances()[0]
	m := views.NewYAML(res, k)

	title := m.FrameTitle()
	expected := "test-docdb-1 yaml"
	if title != expected {
		t.Errorf("YAML FrameTitle: expected %q, got %q", expected, title)
	}
}

func TestQA_RDS_YAML_RawContentNonEmpty(t *testing.T) {
	k := keys.Default()
	res := fixtureRDSInstances()[0]
	m := views.NewYAML(res, k)

	raw := m.RawContent()
	if raw == "" {
		t.Error("YAML RawContent should not be empty")
	}
}

func TestQA_RDS_YAML_WithRawStruct(t *testing.T) {
	k := keys.Default()

	dbIdentifier := "test-yaml-db"
	engine := "postgres"
	engineVersion := "14.9"
	status := "available"
	class := "db.r5.large"
	storageType := "gp3"
	az := "us-east-1a"

	rawStruct := rdstypes.DBInstance{
		DBInstanceIdentifier: &dbIdentifier,
		Engine:               &engine,
		EngineVersion:        &engineVersion,
		DBInstanceStatus:     &status,
		DBInstanceClass:      &class,
		MultiAZ:              boolPtr(true),
		AllocatedStorage:     int32Val(200),
		StorageType:          &storageType,
		AvailabilityZone:     &az,
		Endpoint: &rdstypes.Endpoint{
			Address: strPointer("test-yaml-db.abc123.us-east-1.rds.amazonaws.com"),
			Port:    int32Val(5432),
		},
	}

	res := resource.Resource{
		ID:        "test-yaml-db",
		Name:      "test-yaml-db",
		Status:    "available",
		RawStruct: &rawStruct,
		Fields:    map[string]string{},
	}

	m := views.NewYAML(res, k)
	m.SetSize(120, 50)

	out := m.View()
	plain := stripANSI(out)

	expectedValues := []string{
		"test-yaml-db",
		"postgres",
		"14.9",
		"available",
		"db.r5.large",
		"gp3",
		"us-east-1a",
		"5432",
	}
	for _, val := range expectedValues {
		if !strings.Contains(plain, val) {
			t.Errorf("YAML with RawStruct missing value %q", val)
		}
	}

	// Endpoint should be a nested object with Address and Port.
	if !strings.Contains(plain, "test-yaml-db.abc123.us-east-1.rds.amazonaws.com") {
		t.Error("YAML should contain Endpoint Address")
	}
}

func TestQA_RDS_YAML_CreatingInstanceNoEndpoint(t *testing.T) {
	k := keys.Default()

	dbIdentifier := "yaml-creating-db"
	status := "creating"

	rawStruct := rdstypes.DBInstance{
		DBInstanceIdentifier: &dbIdentifier,
		DBInstanceStatus:     &status,
	}

	res := resource.Resource{
		ID:        "yaml-creating-db",
		Name:      "yaml-creating-db",
		Status:    "creating",
		RawStruct: &rawStruct,
		Fields:    map[string]string{},
	}

	m := views.NewYAML(res, k)
	m.SetSize(120, 40)

	// Should not panic.
	out := m.View()
	plain := stripANSI(out)

	if strings.Contains(plain, "<nil>") {
		t.Error("YAML for creating instance should not show '<nil>'")
	}
}

func TestQA_RDS_YAML_SyntaxColoring(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	k := keys.Default()
	res := fixtureRDSInstances()[0]
	m := views.NewYAML(res, k)
	m.SetSize(120, 40)

	out := m.View()

	// The raw view should contain ANSI sequences (color codes).
	if out == stripANSI(out) {
		t.Error("YAML view should have ANSI color codes when NO_COLOR is not set")
	}
}

// ===========================================================================
// D. Cross-View Interactions (integrated with root model)
// ===========================================================================

func TestQA_RDS_CrossView_ListToDetailAndBack(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to RDS list.
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "dbi",
	})

	// Load RDS data.
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "dbi",
		Resources:    fixtureRDSInstances(),
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "dbi(2)") {
		t.Errorf("expected frame title 'rds(2)', got: %s", plain[:min(200, len(plain))])
	}

	// Press Enter to go to detail.
	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "test-docdb-1") {
		t.Errorf("expected detail view for test-docdb-1, got: %s", plain[:min(200, len(plain))])
	}

	// Press Esc to go back to list.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "dbi") {
		t.Errorf("expected RDS list after Esc, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_RDS_CrossView_ListToYAMLAndBack(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to RDS list.
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "dbi",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "dbi",
		Resources:    fixtureRDSInstances(),
	})

	// Press 'y' to go to YAML.
	m, cmd := rootApplyMsg(m, rootKeyPress("y"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "yaml") {
		t.Errorf("expected YAML view, got: %s", plain[:min(200, len(plain))])
	}

	// Press Esc to go back to list.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "dbi") {
		t.Errorf("expected RDS list after Esc from YAML, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_RDS_CrossView_CommandModeNavigation(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to RDS via command mode.
	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, r := range "dbi" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal("command ':dbi' should return a command")
	}
}

func TestQA_RDS_CrossView_FilterHeaderDisplay(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to RDS list.
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "dbi",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "dbi",
		Resources:    fixtureRDSInstances(),
	})

	// Press '/' to enter filter mode.
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "dbc" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/dbc") {
		t.Errorf("header should show '/dbc' during filter, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_RDS_CrossView_EscFromListReturnsToMainMenu(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to RDS.
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "dbi",
	})

	// Press Esc.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "resource-types") {
		t.Errorf("Esc from RDS list should return to main menu, got: %s", plain[:min(200, len(plain))])
	}
}

// ===========================================================================
// Horizontal scroll
// ===========================================================================

func TestQA_RDS_HorizontalScroll(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rdsTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(60, 20) // Narrow to force some columns off-screen.
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "dbi",
		Resources:    fixtureRDSInstances(),
	})

	outBefore := m.View()

	// Scroll right.
	m, _ = m.Update(rdsKeyPress("l"))
	outAfter := m.View()

	if outBefore == outAfter {
		t.Error("horizontal scroll should change the visible output")
	}
}

// ===========================================================================
// Config-driven columns
// ===========================================================================

func TestQA_RDS_ConfigDrivenColumns(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rdsTypeDef()
	k := keys.Default()

	cfg := config.DefaultConfig()
	m := views.NewResourceList(td, cfg, k)
	m.SetSize(200, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "dbi",
		Resources:    fixtureRDSInstances(),
	})

	out := m.View()

	// With default config, the same 7 headers should be present.
	for _, hdr := range []string{"DB Identifier", "Engine", "Version", "Status", "Class", "Endpoint", "Multi-AZ"} {
		if !strings.Contains(out, hdr) {
			t.Errorf("config-driven RDS list missing column header %q", hdr)
		}
	}
}

// ===========================================================================
// Small helper functions (local to this file to avoid conflicts)
// ===========================================================================

func strPointer(s string) *string { return &s }
func boolPtr(b bool) *bool        { return &b }
func int32Val(i int32) *int32     { return &i }
