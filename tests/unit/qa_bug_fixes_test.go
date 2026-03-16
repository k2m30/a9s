package unit

import (
	"encoding/json"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/app"
	"github.com/k2m30/a9s/internal/config"
	"github.com/k2m30/a9s/internal/navigation"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/ui"
	"github.com/k2m30/a9s/internal/views"
)

func init() {
	app.InitStyles()
}

// ---------------------------------------------------------------------------
// Helper: make a tea.KeyPressMsg from a string key.
// ---------------------------------------------------------------------------

func bugKeyMsg(k string) tea.KeyPressMsg {
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc", "escape":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	default:
		if len(k) == 1 {
			return tea.KeyPressMsg{Code: rune(k[0])}
		}
		return tea.KeyPressMsg{}
	}
}

// ---------------------------------------------------------------------------
// Bug 1: Filter on main menu (T001)
// ---------------------------------------------------------------------------

func TestBug1_FilterOnMainMenu(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	state := app.NewAppState("", "")
	state.Width = 120
	state.Height = 40

	// Press '/' on main menu view — should activate filter mode
	model, _ := state.Update(bugKeyMsg("/"))
	updated := model.(app.AppState)

	if !updated.FilterMode {
		t.Fatalf("expected FilterMode to be true after pressing / on main menu, got false")
	}

	// Type "ec2"
	model, _ = updated.Update(bugKeyMsg("e"))
	updated = model.(app.AppState)
	model, _ = updated.Update(bugKeyMsg("c"))
	updated = model.(app.AppState)
	model, _ = updated.Update(bugKeyMsg("2"))
	updated = model.(app.AppState)

	if updated.Filter != "ec2" {
		t.Errorf("expected Filter = %q, got %q", "ec2", updated.Filter)
	}

	// The main menu should show only items matching "ec2"
	output := updated.View()
	if !strings.Contains(output.Content, "EC2") {
		t.Errorf("expected filtered main menu to contain EC2, got:\n%s", output.Content)
	}

	// Pressing Escape should clear filter, not exit app
	model, _ = updated.Update(bugKeyMsg("esc"))
	updated = model.(app.AppState)
	if updated.Filter != "" {
		t.Errorf("expected Filter to be cleared after Escape, got %q", updated.Filter)
	}
	if updated.CurrentView != app.MainMenuView {
		t.Errorf("expected to stay on MainMenuView after clearing filter, got %d", updated.CurrentView)
	}
}

// ---------------------------------------------------------------------------
// Bug 2: S3 back navigation preserves position (T003)
// ---------------------------------------------------------------------------

func TestBug2_S3NavigationPreservesPosition(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	state := app.NewAppState("", "")
	state.Width = 120
	state.Height = 40

	// Set up state as if we're on resource list viewing S3 buckets
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.SelectedIndex = 4

	// Create 10 mock buckets
	buckets := make([]resource.Resource, 10)
	for i := 0; i < 10; i++ {
		buckets[i] = resource.Resource{
			ID:     "bucket-" + string(rune('a'+i)),
			Name:   "bucket-" + string(rune('a'+i)),
			Fields: map[string]string{"name": "bucket-" + string(rune('a'+i))},
		}
	}
	state.Resources = buckets

	// Drill into the 5th bucket (press Enter)
	model, _ := state.Update(bugKeyMsg("enter"))
	drilled := model.(app.AppState)

	if drilled.SelectedIndex != 0 {
		t.Errorf("expected SelectedIndex=0 after drilling in, got %d", drilled.SelectedIndex)
	}

	// Press Escape to go back
	model, _ = drilled.Update(bugKeyMsg("esc"))
	restored := model.(app.AppState)

	// SelectedIndex should be restored to 4
	if restored.SelectedIndex != 4 {
		t.Errorf("expected SelectedIndex=4 after going back, got %d", restored.SelectedIndex)
	}
}

func TestBug2_NavigationStackStoresSelectedIndex(t *testing.T) {
	var stack navigation.NavigationStack

	stack.Push(navigation.ViewState{
		ViewType:     navigation.ResourceListView,
		ResourceType: "ec2",
		CursorPos:    7,
	})

	state, ok := stack.Pop()
	if !ok {
		t.Fatal("expected Pop to succeed")
	}
	if state.CursorPos != 7 {
		t.Errorf("expected CursorPos = 7, got %d", state.CursorPos)
	}
}

// ---------------------------------------------------------------------------
// Bug 3: Y key produces YAML not JSON (T005)
// ---------------------------------------------------------------------------

func TestBug3_YKeyProducesYAML(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	state := app.NewAppState("", "")
	state.Width = 120
	state.Height = 40
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"

	type FakeStruct struct {
		Name   string
		Status string
	}
	raw := FakeStruct{Name: "test-instance", Status: "running"}

	rawJSON, _ := json.MarshalIndent(raw, "", "  ")
	state.Resources = []resource.Resource{
		{
			ID:        "i-123",
			Name:      "test-instance",
			RawJSON:   string(rawJSON),
			RawStruct: raw,
			Fields:    map[string]string{},
		},
	}

	// Press 'y' to open YAML view
	model, _ := state.Update(bugKeyMsg("y"))
	updated := model.(app.AppState)

	if updated.CurrentView != app.JSONView {
		t.Fatalf("expected to switch to JSONView, got %d", updated.CurrentView)
	}

	content := updated.JSONData.Content
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		t.Errorf("expected YAML output but got JSON-like content:\n%s", content)
	}

	if !strings.Contains(content, "Name:") && !strings.Contains(content, "name:") {
		t.Errorf("expected YAML key-value format with 'Name:', got:\n%s", content)
	}
}

// ---------------------------------------------------------------------------
// Bug 4: Context-aware copy (T008)
// ---------------------------------------------------------------------------

func TestBug4_CopyInListView_CopiesID(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	state := app.NewAppState("", "")
	state.Width = 120
	state.Height = 40
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Resources = []resource.Resource{
		{ID: "i-abc123", Name: "web-server", Fields: map[string]string{}},
	}

	model, _ := state.Update(bugKeyMsg("c"))
	updated := model.(app.AppState)

	if !strings.Contains(updated.StatusMessage, "i-abc123") {
		t.Errorf("expected status message to contain resource ID 'i-abc123', got %q", updated.StatusMessage)
	}
}

func TestBug4_CopyInDetailView_CopiesDetailContent(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	state := app.NewAppState("", "")
	state.Width = 120
	state.Height = 40
	state.CurrentView = app.DetailView
	state.Detail = views.NewDetailModel("test-instance", map[string]string{
		"Name": "test-instance",
		"ID":   "i-abc123",
	})

	// Press 'c' in detail view
	model, _ := state.Update(bugKeyMsg("c"))
	updated := model.(app.AppState)

	// Should confirm copy happened
	if !strings.Contains(strings.ToLower(updated.StatusMessage), "copied") &&
		!strings.Contains(strings.ToLower(updated.StatusMessage), "copy") {
		t.Errorf("expected copy confirmation in status, got %q", updated.StatusMessage)
	}
}

// ---------------------------------------------------------------------------
// Bug 5: Status bar at bottom (T010)
// ---------------------------------------------------------------------------

func TestBug5_StatusBarAtBottom(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	state := app.NewAppState("", "")
	state.Width = 80
	state.Height = 40

	output := state.View()
	lines := strings.Split(output.Content, "\n")

	if len(lines) < state.Height {
		t.Errorf("expected output to fill terminal height %d, got %d lines", state.Height, len(lines))
	}
}

// ---------------------------------------------------------------------------
// Bug 6: Status clear on navigation (T024)
// ---------------------------------------------------------------------------

func TestBug6_StatusClearsOnNavigation(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	state := app.NewAppState("", "")
	state.Width = 120
	state.Height = 40
	state.StatusMessage = "Some old status"
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Resources = []resource.Resource{
		{
			ID:     "i-123",
			Name:   "test",
			Fields: map[string]string{},
			DetailData: map[string]string{
				"Name": "test",
			},
		},
	}

	model, _ := state.Update(bugKeyMsg("d"))
	updated := model.(app.AppState)

	if updated.StatusMessage != "" {
		t.Errorf("expected StatusMessage to be cleared on navigation, got %q", updated.StatusMessage)
	}
}

// ---------------------------------------------------------------------------
// Bug 7: Breadcrumbs without "main" prefix (T026)
// ---------------------------------------------------------------------------

func TestBug7_BreadcrumbsNoMainPrefix(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	// On main menu, breadcrumbs show "main"
	state := app.NewAppState("", "")
	state.Width = 120
	state.Height = 40
	if len(state.Breadcrumbs) != 1 || state.Breadcrumbs[0] != "main" {
		t.Errorf("expected breadcrumbs = [main] on main menu, got %v", state.Breadcrumbs)
	}

	// Navigate to EC2 via command
	state.CommandMode = true
	state.CommandText = "ec2"
	model, _ := state.Update(bugKeyMsg("enter"))
	updated := model.(app.AppState)

	// Breadcrumbs should NOT start with "main" when on resource list
	if len(updated.Breadcrumbs) > 0 && updated.Breadcrumbs[0] == "main" {
		t.Errorf("expected breadcrumbs to NOT start with 'main' on resource list view, got %v", updated.Breadcrumbs)
	}
}

// ---------------------------------------------------------------------------
// Bug 8: Detail view horizontal scroll (T012)
// ---------------------------------------------------------------------------

func TestBug8_DetailViewHorizontalScroll(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	state := app.NewAppState("", "")
	state.Width = 40
	state.Height = 30
	state.CurrentView = app.DetailView
	state.Detail = views.NewDetailModel("test", map[string]string{
		"LongField": strings.Repeat("x", 200),
	})
	state.Detail.Width = 40
	state.Detail.Height = 30

	model, _ := state.Update(bugKeyMsg("l"))
	updated := model.(app.AppState)

	if updated.HScrollOffset <= 0 {
		t.Errorf("expected HScrollOffset > 0 after pressing right in detail view, got %d", updated.HScrollOffset)
	}

	model, _ = updated.Update(bugKeyMsg("h"))
	updated = model.(app.AppState)

	if updated.HScrollOffset < 0 {
		t.Errorf("expected HScrollOffset >= 0 after pressing left, got %d", updated.HScrollOffset)
	}
}

func TestBug8_JSONViewHorizontalScroll(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	state := app.NewAppState("", "")
	state.Width = 40
	state.Height = 30
	state.CurrentView = app.JSONView
	state.JSONData = views.NewJSONView("test", strings.Repeat("x", 200))

	model, _ := state.Update(bugKeyMsg("l"))
	updated := model.(app.AppState)

	if updated.HScrollOffset <= 0 {
		t.Errorf("expected HScrollOffset > 0 after pressing right in JSON view, got %d", updated.HScrollOffset)
	}
}

// ---------------------------------------------------------------------------
// Bug 9: Horizontal scroll clamped (T014)
// ---------------------------------------------------------------------------

func TestBug9_HorizontalScrollClamped(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	state := app.NewAppState("", "")
	state.Width = 80
	state.Height = 40
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Resources = []resource.Resource{
		{
			ID:   "i-123",
			Name: "test",
			Fields: map[string]string{
				"instance_id": "i-123",
				"name":        "test",
				"state":       "running",
			},
		},
	}

	var model tea.Model = state
	for i := 0; i < 100; i++ {
		model, _ = model.(app.AppState).Update(bugKeyMsg("l"))
	}
	updated := model.(app.AppState)
	maxOffset := updated.HScrollOffset

	model, _ = updated.Update(bugKeyMsg("l"))
	afterExtra := model.(app.AppState)

	if afterExtra.HScrollOffset > maxOffset {
		t.Errorf("expected scroll to be clamped at %d, but got %d", maxOffset, afterExtra.HScrollOffset)
	}

	model, _ = afterExtra.Update(bugKeyMsg("h"))
	afterLeft := model.(app.AppState)

	if afterLeft.HScrollOffset >= afterExtra.HScrollOffset {
		t.Errorf("expected HScrollOffset to decrease after pressing left from max, went from %d to %d",
			afterExtra.HScrollOffset, afterLeft.HScrollOffset)
	}
}

// ---------------------------------------------------------------------------
// Bug 10: Scroll reset on navigation (T028)
// ---------------------------------------------------------------------------

func TestBug10_ScrollResetOnNavigation(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	state := app.NewAppState("", "")
	state.Width = 80
	state.Height = 40
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.HScrollOffset = 20

	state.CommandMode = true
	state.CommandText = "rds"
	model, _ := state.Update(bugKeyMsg("enter"))
	updated := model.(app.AppState)

	if updated.HScrollOffset != 0 {
		t.Errorf("expected HScrollOffset to reset to 0 on navigation, got %d", updated.HScrollOffset)
	}
}

// ---------------------------------------------------------------------------
// Bug 11: Context-sensitive help (T030)
// ---------------------------------------------------------------------------

func TestBug11_ContextSensitiveHelp(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	// List view help
	listHelp := ui.NewHelpModel()
	listHelp.Width = 80
	listHelp.Height = 40
	listHelp.ViewType = ui.ListViewHelp
	listView := listHelp.View()

	// Detail view help
	detailHelp := ui.NewHelpModel()
	detailHelp.Width = 80
	detailHelp.Height = 40
	detailHelp.ViewType = ui.DetailViewHelp
	detailView := detailHelp.View()

	if !strings.Contains(listView, "filter") {
		t.Errorf("expected list view help to contain 'filter', got:\n%s", listView)
	}

	if !strings.Contains(detailView, "wrap") {
		t.Errorf("expected detail view help to contain 'wrap', got:\n%s", detailView)
	}

	if strings.Contains(detailView, "sort") || strings.Contains(detailView, "by name") {
		t.Errorf("expected detail view help to NOT contain sorting keys, got:\n%s", detailView)
	}
}

// ---------------------------------------------------------------------------
// Bug 12: Header styling (T032)
// ---------------------------------------------------------------------------

func TestBug12_HeaderStyling(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	app.Version = "0.5.0"
	state := app.NewAppState("test-profile", "us-east-1")
	state.Width = 80
	state.Height = 40

	output := state.View()
	lines := strings.Split(output.Content, "\n")

	if len(lines) == 0 {
		t.Fatal("expected at least one line of output")
	}

	headerLine := lines[0]

	if !strings.Contains(headerLine, "test-profile") {
		t.Errorf("expected header to contain profile name, got: %s", headerLine)
	}

	profileIdx := strings.Index(headerLine, "test-profile")
	versionIdx := strings.Index(headerLine, "0.5.0")
	if versionIdx < 0 {
		t.Errorf("expected header to contain version, got: %s", headerLine)
	} else if versionIdx < profileIdx {
		t.Errorf("expected version to appear after profile (right side), but profile at %d, version at %d",
			profileIdx, versionIdx)
	}
}

// ---------------------------------------------------------------------------
// Bug 13: Detail view improvements (T016, T017, T018)
// ---------------------------------------------------------------------------

func TestBug13_DetailTitleNoSuffix(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	state := app.NewAppState("", "")
	state.Width = 120
	state.Height = 40
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Resources = []resource.Resource{
		{
			ID:         "i-123",
			Name:       "my-instance",
			DetailData: map[string]string{"Name": "my-instance", "ID": "i-123"},
			Fields:     map[string]string{},
		},
	}

	model, _ := state.Update(bugKeyMsg("d"))
	updated := model.(app.AppState)

	if strings.Contains(updated.Detail.Title, " - Detail") {
		t.Errorf("expected detail title to NOT contain ' - Detail', got %q", updated.Detail.Title)
	}
}

func TestBug13_ConfigDetailTitleNoSuffix(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	type FakeStruct struct {
		Name   string
		Status string
	}
	state := app.NewAppState("", "")
	state.Width = 120
	state.Height = 40
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Resources = []resource.Resource{
		{
			ID:        "i-123",
			Name:      "my-instance",
			RawStruct: FakeStruct{Name: "my-instance", Status: "running"},
			Fields:    map[string]string{},
		},
	}

	model, _ := state.Update(bugKeyMsg("d"))
	updated := model.(app.AppState)

	if strings.Contains(updated.Detail.Title, " - Detail") {
		t.Errorf("expected config detail title to NOT contain ' - Detail', got %q", updated.Detail.Title)
	}
}

func TestBug13_WrapToggle(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	state := app.NewAppState("", "")
	state.Width = 40
	state.Height = 30
	state.CurrentView = app.DetailView
	state.Detail = views.NewDetailModel("test", map[string]string{
		"Key": strings.Repeat("x", 200),
	})
	state.Detail.Width = 40
	state.Detail.Height = 30
	state.HScrollOffset = 10

	model, _ := state.Update(bugKeyMsg("w"))
	updated := model.(app.AppState)

	if !updated.Detail.WrapEnabled {
		t.Errorf("expected WrapEnabled=true after pressing 'w', got false")
	}

	if updated.HScrollOffset != 0 {
		t.Errorf("expected HScrollOffset=0 when wrap is on, got %d", updated.HScrollOffset)
	}
}

func TestBug13_YAMLKeyValueFormat(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	detail := views.NewDetailModel("test", map[string]string{
		"Name":   "test-instance",
		"Status": "running",
	})
	detail.Width = 80
	detail.Height = 40

	output := detail.View()

	// Check for "Key: value" format (colon right after key, no padding)
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Name") && strings.Contains(trimmed, ":") {
			colonIdx := strings.Index(trimmed, ":")
			nameEnd := len("Name")
			if colonIdx > nameEnd+1 {
				t.Errorf("expected 'Key: value' format, got padded format: %q", trimmed)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Bug 14: Resource count in breadcrumbs (T034)
// ---------------------------------------------------------------------------

func TestBug14_ResourceCountInBreadcrumbs(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	state := app.NewAppState("", "")
	state.Width = 120
	state.Height = 40
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.S3Bucket = "my-bucket"

	objects := make([]resource.Resource, 139)
	for i := 0; i < 139; i++ {
		objects[i] = resource.Resource{
			ID:     "obj-" + string(rune('a'+i%26)),
			Name:   "obj-" + string(rune('a'+i%26)),
			Fields: map[string]string{},
		}
	}
	state.Resources = objects

	// Simulate resources loaded to trigger breadcrumb update
	model, _ := state.Update(app.ResourcesLoadedMsg{
		ResourceType: "s3",
		Resources:    objects,
	})
	state = model.(app.AppState)

	output := state.View()
	body := output.Content

	if !strings.Contains(body, "(139)") {
		t.Errorf("expected breadcrumbs to contain '(139)', got:\n%s", body)
	}

	// Should not have duplicate title line
	lines := strings.Split(body, "\n")
	titleCount := 0
	for _, line := range lines {
		if strings.Contains(line, "my-bucket") && strings.Contains(line, "139") {
			titleCount++
		}
	}
	if titleCount > 1 {
		t.Errorf("expected count to appear only once (in breadcrumbs), but found %d occurrences", titleCount)
	}
}

// ---------------------------------------------------------------------------
// Bug 15: Config column widths respected (T022)
// ---------------------------------------------------------------------------

func TestBug15_ConfigColumnWidthsRespected(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"s3": {
				List: []config.ListColumn{
					{Title: "Bucket Name", Path: "Name", Width: 60},
					{Title: "Creation Date", Path: "CreationDate", Width: 22},
				},
			},
		},
	}

	state := app.NewAppState("", "")
	state.Width = 120
	state.Height = 40
	state.ViewConfig = cfg
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.Resources = []resource.Resource{
		{
			ID:        "short-bucket",
			Name:      "short-bucket",
			RawStruct: struct{ Name string }{Name: "short-bucket"},
			Fields:    map[string]string{},
		},
	}

	output := state.View()
	body := output.Content
	lines := strings.Split(body, "\n")

	// Find the separator line (contains consecutive dashes)
	found := false
	for _, line := range lines {
		if strings.Contains(line, "---") {
			// The first field of the separator should be >= 60 dashes
			trimmed := strings.TrimLeft(line, " >")
			dashFields := strings.Fields(trimmed)
			if len(dashFields) > 0 {
				bucketDashes := dashFields[0]
				if len(bucketDashes) >= 60 {
					found = true
				} else {
					t.Errorf("expected Bucket Name column width >= 60, got separator width %d in line: %q", len(bucketDashes), line)
				}
			}
			break
		}
	}
	if !found {
		t.Errorf("did not find separator line with expected width in output:\n%s", body)
	}
}
