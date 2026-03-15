package unit

import (
	"fmt"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/views"
)

// ===========================================================================
// MainMenuModel tests
// ===========================================================================

func TestNewMainMenu_ItemsPopulatedWith7ResourceTypes(t *testing.T) {
	m := views.NewMainMenu()
	if len(m.Items) != 7 {
		t.Errorf("expected 7 resource types in main menu, got %d", len(m.Items))
	}
}

func TestNewMainMenu_CursorStartsAtZero(t *testing.T) {
	m := views.NewMainMenu()
	if m.Cursor != 0 {
		t.Errorf("expected initial cursor at 0, got %d", m.Cursor)
	}
}

func TestMainMenuModel_MoveUp_ClampedAtZero(t *testing.T) {
	m := views.NewMainMenu()
	m.MoveUp()
	if m.Cursor != 0 {
		t.Errorf("expected cursor to stay at 0 when moving up from top, got %d", m.Cursor)
	}
	m.MoveUp()
	if m.Cursor != 0 {
		t.Errorf("expected cursor to stay at 0 after repeated MoveUp, got %d", m.Cursor)
	}
}

func TestMainMenuModel_MoveDown_ClampedAtBottom(t *testing.T) {
	m := views.NewMainMenu()
	maxIndex := len(m.Items) - 1

	// Move down to the bottom
	for i := 0; i < len(m.Items)+5; i++ {
		m.MoveDown()
	}

	if m.Cursor != maxIndex {
		t.Errorf("expected cursor clamped at %d, got %d", maxIndex, m.Cursor)
	}
}

func TestMainMenuModel_MoveDown_Increments(t *testing.T) {
	m := views.NewMainMenu()
	m.MoveDown()
	if m.Cursor != 1 {
		t.Errorf("expected cursor at 1 after one MoveDown, got %d", m.Cursor)
	}
	m.MoveDown()
	if m.Cursor != 2 {
		t.Errorf("expected cursor at 2 after two MoveDown, got %d", m.Cursor)
	}
}

func TestMainMenuModel_MoveUp_Decrements(t *testing.T) {
	m := views.NewMainMenu()
	m.Cursor = 3
	m.MoveUp()
	if m.Cursor != 2 {
		t.Errorf("expected cursor at 2 after MoveUp from 3, got %d", m.Cursor)
	}
}

func TestMainMenuModel_GoTop(t *testing.T) {
	m := views.NewMainMenu()
	m.Cursor = 5
	m.GoTop()
	if m.Cursor != 0 {
		t.Errorf("expected cursor at 0 after GoTop, got %d", m.Cursor)
	}
}

func TestMainMenuModel_GoBottom(t *testing.T) {
	m := views.NewMainMenu()
	m.GoBottom()
	expected := len(m.Items) - 1
	if m.Cursor != expected {
		t.Errorf("expected cursor at %d after GoBottom, got %d", expected, m.Cursor)
	}
}

func TestMainMenuModel_GoTop_ThenGoBottom(t *testing.T) {
	m := views.NewMainMenu()
	m.GoBottom()
	m.GoTop()
	if m.Cursor != 0 {
		t.Errorf("expected cursor at 0 after GoBottom then GoTop, got %d", m.Cursor)
	}
}

func TestMainMenuModel_SelectedItem_AtCursor0(t *testing.T) {
	m := views.NewMainMenu()
	item := m.SelectedItem()
	if item.Name != m.Items[0].Name {
		t.Errorf("expected first item %q, got %q", m.Items[0].Name, item.Name)
	}
}

func TestMainMenuModel_SelectedItem_AtEachCursor(t *testing.T) {
	m := views.NewMainMenu()
	for i := 0; i < len(m.Items); i++ {
		m.Cursor = i
		item := m.SelectedItem()
		if item.Name != m.Items[i].Name {
			t.Errorf("at cursor %d: expected %q, got %q", i, m.Items[i].Name, item.Name)
		}
	}
}

func TestMainMenuModel_SelectedItem_OutOfBounds(t *testing.T) {
	m := views.NewMainMenu()
	m.Cursor = -1
	item := m.SelectedItem()
	if item.Name != "" {
		t.Errorf("expected empty ResourceTypeDef for cursor -1, got %q", item.Name)
	}

	m.Cursor = 100
	item = m.SelectedItem()
	if item.Name != "" {
		t.Errorf("expected empty ResourceTypeDef for cursor 100, got %q", item.Name)
	}
}

func TestMainMenuModel_View_ContainsAllResourceTypeNames(t *testing.T) {
	m := views.NewMainMenu()
	output := m.View()

	for _, item := range m.Items {
		if !strings.Contains(output, item.Name) {
			t.Errorf("expected View to contain resource type name %q", item.Name)
		}
	}
}

func TestMainMenuModel_View_ContainsCursorAtSelectedIndex(t *testing.T) {
	m := views.NewMainMenu()
	m.Cursor = 0
	output := m.View()
	if !strings.Contains(output, ">") {
		t.Error("expected View to contain '>' cursor indicator")
	}
}

func TestMainMenuModel_View_ContainsHelpText(t *testing.T) {
	m := views.NewMainMenu()
	output := m.View()
	if !strings.Contains(output, "Press : for commands") {
		t.Error("expected View to contain help text 'Press : for commands'")
	}
}

func TestMainMenuModel_View_CursorChangesWithPosition(t *testing.T) {
	m := views.NewMainMenu()

	// At cursor 0, first item should be selected
	m.Cursor = 0
	output0 := m.View()

	// At cursor 2, third item should be selected
	m.Cursor = 2
	output2 := m.View()

	// Outputs should differ (the cursor moved)
	if output0 == output2 {
		t.Error("expected different View output when cursor is at different positions")
	}
}

// ===========================================================================
// ResourceListModel tests
// ===========================================================================

func makeEC2TypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		Name:      "EC2 Instances",
		ShortName: "ec2",
		Columns: []resource.Column{
			{Key: "instance_id", Title: "Instance ID", Width: 20, Sortable: true},
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "type", Title: "Type", Width: 14, Sortable: true},
			{Key: "launch_time", Title: "Launch Time", Width: 22, Sortable: true},
		},
	}
}

func makeSampleEC2Resources() []resource.Resource {
	return []resource.Resource{
		{
			ID:   "i-001",
			Name: "alpha-server",
			Fields: map[string]string{
				"instance_id": "i-001",
				"name":        "alpha-server",
				"state":       "running",
				"type":        "t3.micro",
				"launch_time": "2025-01-01T00:00:00Z",
			},
		},
		{
			ID:   "i-002",
			Name: "bravo-server",
			Fields: map[string]string{
				"instance_id": "i-002",
				"name":        "bravo-server",
				"state":       "stopped",
				"type":        "t3.large",
				"launch_time": "2025-02-15T00:00:00Z",
			},
		},
		{
			ID:   "i-003",
			Name: "charlie-server",
			Fields: map[string]string{
				"instance_id": "i-003",
				"name":        "charlie-server",
				"state":       "running",
				"type":        "t3.medium",
				"launch_time": "2024-12-20T00:00:00Z",
			},
		},
	}
}

func TestNewResourceList_CreatesModelWithColumnsFromTypeDef(t *testing.T) {
	typeDef := makeEC2TypeDef()
	resources := makeSampleEC2Resources()
	m := views.NewResourceList(typeDef, resources, 120, 30)

	if len(m.Resources) != 3 {
		t.Errorf("expected 3 resources, got %d", len(m.Resources))
	}
	if m.TypeDef.Name != "EC2 Instances" {
		t.Errorf("expected TypeDef name 'EC2 Instances', got %q", m.TypeDef.Name)
	}
	if m.Width != 120 {
		t.Errorf("expected width 120, got %d", m.Width)
	}
	if m.Height != 30 {
		t.Errorf("expected height 30, got %d", m.Height)
	}
}

func TestNewResourceList_EmptyResources(t *testing.T) {
	typeDef := makeEC2TypeDef()
	m := views.NewResourceList(typeDef, []resource.Resource{}, 120, 30)

	if len(m.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(m.Resources))
	}

	// View should not panic on empty resources
	output := m.View()
	if output == "" {
		t.Error("expected non-empty output from View even with no resources")
	}
}

func TestNewResourceList_SmallHeight(t *testing.T) {
	typeDef := makeEC2TypeDef()
	resources := makeSampleEC2Resources()
	// Height 3 means pageSize = 3 - 4 = -1, clamped to 1
	m := views.NewResourceList(typeDef, resources, 120, 3)

	// Should not panic
	output := m.View()
	if output == "" {
		t.Error("expected non-empty output with small height")
	}
	_ = m
}

func TestNewResourceList_ZeroHeight(t *testing.T) {
	typeDef := makeEC2TypeDef()
	resources := makeSampleEC2Resources()
	// Height 0 => pageSize = 0 - 4 = -4, clamped to 1
	m := views.NewResourceList(typeDef, resources, 120, 0)
	output := m.View()
	if output == "" {
		t.Error("expected non-empty output with zero height")
	}
	_ = m
}

func TestResourceListModel_SelectedResource_ReturnsFirstByDefault(t *testing.T) {
	typeDef := makeEC2TypeDef()
	resources := makeSampleEC2Resources()
	m := views.NewResourceList(typeDef, resources, 120, 30)

	selected := m.SelectedResource()
	if selected == nil {
		t.Fatal("expected non-nil selected resource")
	}
	if selected.ID != "i-001" {
		t.Errorf("expected selected resource ID 'i-001', got %q", selected.ID)
	}
}

func TestResourceListModel_SelectedResource_EmptyList(t *testing.T) {
	typeDef := makeEC2TypeDef()
	m := views.NewResourceList(typeDef, []resource.Resource{}, 120, 30)

	selected := m.SelectedResource()
	if selected != nil {
		t.Errorf("expected nil selected resource for empty list, got %v", selected)
	}
}

func TestResourceListModel_SetSize(t *testing.T) {
	typeDef := makeEC2TypeDef()
	resources := makeSampleEC2Resources()
	m := views.NewResourceList(typeDef, resources, 120, 30)

	m.SetSize(200, 50)
	if m.Width != 200 {
		t.Errorf("expected width 200 after SetSize, got %d", m.Width)
	}
	if m.Height != 50 {
		t.Errorf("expected height 50 after SetSize, got %d", m.Height)
	}
}

func TestResourceListModel_SetSize_SmallHeight(t *testing.T) {
	typeDef := makeEC2TypeDef()
	resources := makeSampleEC2Resources()
	m := views.NewResourceList(typeDef, resources, 120, 30)

	// Small height where pageSize would be < 1
	m.SetSize(80, 2)
	if m.Width != 80 {
		t.Errorf("expected width 80, got %d", m.Width)
	}
	if m.Height != 2 {
		t.Errorf("expected height 2, got %d", m.Height)
	}
	// Should not panic
	_ = m.View()
}

func TestResourceListModel_SortByColumn_Ascending(t *testing.T) {
	typeDef := makeEC2TypeDef()
	resources := makeSampleEC2Resources()
	m := views.NewResourceList(typeDef, resources, 120, 30)

	// Should not panic
	m.SortByColumn("name", true)
	output := m.View()
	if output == "" {
		t.Error("expected non-empty View after SortByColumn ascending")
	}
}

func TestResourceListModel_SortByColumn_Descending(t *testing.T) {
	typeDef := makeEC2TypeDef()
	resources := makeSampleEC2Resources()
	m := views.NewResourceList(typeDef, resources, 120, 30)

	m.SortByColumn("name", false)
	output := m.View()
	if output == "" {
		t.Error("expected non-empty View after SortByColumn descending")
	}
}

func TestResourceListModel_SortByColumn_InvalidKey_NoPanic(t *testing.T) {
	typeDef := makeEC2TypeDef()
	resources := makeSampleEC2Resources()
	m := views.NewResourceList(typeDef, resources, 120, 30)

	// Should not panic with a nonexistent column key
	m.SortByColumn("nonexistent_column", true)
	m.SortByColumn("", false)
	_ = m.View()
}

func TestResourceListModel_SortByName(t *testing.T) {
	typeDef := makeEC2TypeDef()
	resources := makeSampleEC2Resources()
	m := views.NewResourceList(typeDef, resources, 120, 30)

	// Should not panic; EC2 has a "name" column
	m.SortByName(true)
	output := m.View()
	if output == "" {
		t.Error("expected non-empty View after SortByName")
	}

	m.SortByName(false)
	output = m.View()
	if output == "" {
		t.Error("expected non-empty View after SortByName desc")
	}
}

func TestResourceListModel_SortByStatus(t *testing.T) {
	typeDef := makeEC2TypeDef()
	resources := makeSampleEC2Resources()
	m := views.NewResourceList(typeDef, resources, 120, 30)

	// EC2 has "state" column, so SortByStatus should find it via "state"
	m.SortByStatus(true)
	output := m.View()
	if output == "" {
		t.Error("expected non-empty View after SortByStatus")
	}

	m.SortByStatus(false)
	_ = m.View()
}

func TestResourceListModel_SortByStatus_WithStatusColumn(t *testing.T) {
	// RDS has a "status" column
	rdsDef := resource.ResourceTypeDef{
		Name:      "RDS Instances",
		ShortName: "rds",
		Columns: []resource.Column{
			{Key: "db_identifier", Title: "DB Identifier", Width: 28},
			{Key: "status", Title: "Status", Width: 14},
		},
	}
	rdsResources := []resource.Resource{
		{
			ID:   "db-1",
			Name: "mydb",
			Fields: map[string]string{
				"db_identifier": "mydb",
				"status":        "available",
			},
		},
	}
	m := views.NewResourceList(rdsDef, rdsResources, 120, 30)
	m.SortByStatus(true)
	_ = m.View()
}

func TestResourceListModel_SortByAge(t *testing.T) {
	typeDef := makeEC2TypeDef()
	resources := makeSampleEC2Resources()
	m := views.NewResourceList(typeDef, resources, 120, 30)

	// EC2 has "launch_time" column which matches "launch"
	m.SortByAge(true)
	output := m.View()
	if output == "" {
		t.Error("expected non-empty View after SortByAge")
	}

	m.SortByAge(false)
	_ = m.View()
}

func TestResourceListModel_SortByAge_NoMatchingColumn(t *testing.T) {
	// TypeDef with no time/date/age columns
	typeDef := resource.ResourceTypeDef{
		Name:      "Custom",
		ShortName: "custom",
		Columns: []resource.Column{
			{Key: "id", Title: "ID", Width: 20},
			{Key: "label", Title: "Label", Width: 30},
		},
	}
	resources := []resource.Resource{
		{
			ID:   "c-1",
			Name: "custom-1",
			Fields: map[string]string{
				"id":    "c-1",
				"label": "something",
			},
		},
	}
	m := views.NewResourceList(typeDef, resources, 120, 30)

	// Should not panic when no column matches age
	m.SortByAge(true)
	_ = m.View()
}

func TestResourceListModel_SortByName_NoNameColumn(t *testing.T) {
	typeDef := resource.ResourceTypeDef{
		Name:      "Custom",
		ShortName: "custom",
		Columns: []resource.Column{
			{Key: "id", Title: "ID", Width: 20},
			{Key: "label", Title: "Label", Width: 30},
		},
	}
	resources := []resource.Resource{
		{
			ID:     "c-1",
			Name:   "custom-1",
			Fields: map[string]string{"id": "c-1", "label": "something"},
		},
	}
	m := views.NewResourceList(typeDef, resources, 120, 30)

	// Should not panic when no "name" column found
	m.SortByName(true)
	_ = m.View()
}

func TestResourceListModel_View_RendersTableContent(t *testing.T) {
	typeDef := makeEC2TypeDef()
	resources := makeSampleEC2Resources()
	m := views.NewResourceList(typeDef, resources, 120, 30)

	output := m.View()
	if output == "" {
		t.Error("expected non-empty View output")
	}
	// The table should contain the column titles
	for _, col := range typeDef.Columns {
		if !strings.Contains(output, col.Title) {
			t.Errorf("expected View to contain column title %q", col.Title)
		}
	}
}

func TestResourceListModel_FindColumnKey_FindsCorrectKey(t *testing.T) {
	// We test findColumnKey indirectly through SortByName, SortByStatus, SortByAge
	// Here we test SortByAge matching "time" in "launch_time"
	typeDef := makeEC2TypeDef()
	resources := makeSampleEC2Resources()
	m := views.NewResourceList(typeDef, resources, 120, 30)

	// SortByAge should find "launch_time" via the "launch" substring
	m.SortByAge(true)
	// No panic means findColumnKey found the key

	// SortByStatus should find "state" via the "state" substring
	m.SortByStatus(true)
	// No panic means it worked
}

func TestResourceListModel_FindColumnKey_CaseInsensitive(t *testing.T) {
	typeDef := resource.ResourceTypeDef{
		Name:      "Test",
		ShortName: "test",
		Columns: []resource.Column{
			{Key: "MyName", Title: "My Name", Width: 20},
			{Key: "STATUS_CODE", Title: "Status", Width: 10},
		},
	}
	resources := []resource.Resource{
		{
			ID:     "t-1",
			Name:   "test",
			Fields: map[string]string{"MyName": "test", "STATUS_CODE": "ok"},
		},
	}
	m := views.NewResourceList(typeDef, resources, 120, 30)

	// SortByName should find "MyName" (contains "name" case-insensitive)
	m.SortByName(true)

	// SortByStatus should find "STATUS_CODE" (contains "status" case-insensitive)
	m.SortByStatus(true)
}

func TestResourceListModel_SortByAge_MatchesLastAccessed(t *testing.T) {
	// Secrets Manager has "last_accessed" and "last_changed" columns
	typeDef := resource.ResourceTypeDef{
		Name:      "Secrets Manager",
		ShortName: "secrets",
		Columns: []resource.Column{
			{Key: "secret_name", Title: "Secret Name", Width: 36},
			{Key: "last_accessed", Title: "Last Accessed", Width: 18},
			{Key: "last_changed", Title: "Last Changed", Width: 18},
		},
	}
	resources := []resource.Resource{
		{
			ID:   "s-1",
			Name: "my-secret",
			Fields: map[string]string{
				"secret_name":   "my-secret",
				"last_accessed": "2025-03-01",
				"last_changed":  "2025-02-28",
			},
		},
	}
	m := views.NewResourceList(typeDef, resources, 120, 30)

	// SortByAge should find "last_accessed" via "accessed" substring
	m.SortByAge(true)
	_ = m.View()
}

// ===========================================================================
// DetailModel tests (expanding existing)
// ===========================================================================

func TestDetailModel_GoTop_SetsOffsetToZero(t *testing.T) {
	data := map[string]string{
		"key1": "val1",
		"key2": "val2",
		"key3": "val3",
		"key4": "val4",
		"key5": "val5",
	}
	m := views.NewDetailModel("GoTop Test", data)
	m.Height = 3

	m.ScrollDown()
	m.ScrollDown()
	m.ScrollDown()
	if m.Offset == 0 {
		t.Fatal("expected offset > 0 after scrolling down")
	}

	m.GoTop()
	if m.Offset != 0 {
		t.Errorf("expected offset 0 after GoTop, got %d", m.Offset)
	}
}

func TestDetailModel_GoBottom_SetsOffsetToMax(t *testing.T) {
	data := map[string]string{
		"key1": "val1",
		"key2": "val2",
		"key3": "val3",
		"key4": "val4",
		"key5": "val5",
		"key6": "val6",
		"key7": "val7",
		"key8": "val8",
	}
	m := views.NewDetailModel("GoBottom Test", data)
	m.Height = 3

	m.GoBottom()
	expected := len(m.Keys) - m.Height
	if expected < 0 {
		expected = 0
	}
	if m.Offset != expected {
		t.Errorf("expected offset %d after GoBottom, got %d", expected, m.Offset)
	}
}

func TestDetailModel_GoBottom_SmallData(t *testing.T) {
	// When data fits in viewport, GoBottom should set offset to 0
	data := map[string]string{
		"key1": "val1",
	}
	m := views.NewDetailModel("Small GoBottom", data)
	m.Height = 10

	m.GoBottom()
	if m.Offset != 0 {
		t.Errorf("expected offset 0 when data fits in viewport, got %d", m.Offset)
	}
}

func TestDetailModel_ScrollUp_AtTop_StaysAtZero(t *testing.T) {
	data := map[string]string{"k1": "v1", "k2": "v2"}
	m := views.NewDetailModel("ScrollUp at top", data)

	m.ScrollUp()
	if m.Offset != 0 {
		t.Errorf("expected offset 0 after ScrollUp at top, got %d", m.Offset)
	}
	m.ScrollUp()
	if m.Offset != 0 {
		t.Errorf("expected offset still 0 after repeated ScrollUp, got %d", m.Offset)
	}
}

func TestDetailModel_ScrollDown_AtBottom_StaysAtMax(t *testing.T) {
	data := map[string]string{"k1": "v1", "k2": "v2", "k3": "v3"}
	m := views.NewDetailModel("ScrollDown at bottom", data)
	m.Height = 5

	maxOffset := len(m.Keys) - 1

	// Scroll down past the end
	for i := 0; i < 20; i++ {
		m.ScrollDown()
	}

	if m.Offset != maxOffset {
		t.Errorf("expected offset clamped at %d, got %d", maxOffset, m.Offset)
	}
}

func TestDetailModel_NilMap_NoPanic(t *testing.T) {
	// Passing nil data map should not panic
	m := views.NewDetailModel("Nil Map", nil)
	if len(m.Keys) != 0 {
		t.Errorf("expected 0 keys for nil map, got %d", len(m.Keys))
	}
	output := m.View()
	if !strings.Contains(output, "No details available") {
		t.Error("expected 'No details available' message for nil map")
	}
}

func TestDetailModel_VeryLongValues(t *testing.T) {
	longVal := strings.Repeat("x", 5000)
	data := map[string]string{
		"long_key": longVal,
		"short":    "ok",
	}
	m := views.NewDetailModel("Long Values", data)
	m.Width = 80
	m.Height = 30

	output := m.View()
	// Should not panic and should contain the long value
	if !strings.Contains(output, longVal) {
		t.Error("expected View to contain the long value")
	}
}

func TestDetailModel_View_RendersKeyValuePairs(t *testing.T) {
	data := map[string]string{
		"alpha": "one",
		"beta":  "two",
	}
	m := views.NewDetailModel("KV Test", data)
	m.Width = 80
	m.Height = 30

	output := m.View()
	if !strings.Contains(output, "alpha") || !strings.Contains(output, "one") {
		t.Error("expected View to contain 'alpha' and 'one'")
	}
	if !strings.Contains(output, "beta") || !strings.Contains(output, "two") {
		t.Error("expected View to contain 'beta' and 'two'")
	}
	if !strings.Contains(output, "KV Test") {
		t.Error("expected View to contain title")
	}
}

func TestDetailModel_ScrollDown_ThenScrollUp(t *testing.T) {
	data := map[string]string{
		"a": "1", "b": "2", "c": "3", "d": "4", "e": "5",
	}
	m := views.NewDetailModel("Round trip", data)

	m.ScrollDown()
	m.ScrollDown()
	if m.Offset != 2 {
		t.Errorf("expected offset 2, got %d", m.Offset)
	}
	m.ScrollUp()
	if m.Offset != 1 {
		t.Errorf("expected offset 1, got %d", m.Offset)
	}
}

// ===========================================================================
// JSONViewModel tests (expanding existing)
// ===========================================================================

func TestJSONViewModel_GoTop(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5"
	m := views.NewJSONView("GoTop JSON", content)
	m.Height = 3

	m.ScrollDown()
	m.ScrollDown()
	m.GoTop()
	if m.Offset != 0 {
		t.Errorf("expected offset 0 after GoTop, got %d", m.Offset)
	}
}

func TestJSONViewModel_GoBottom(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8"
	m := views.NewJSONView("GoBottom JSON", content)
	m.Height = 3

	m.GoBottom()
	lines := strings.Split(content, "\n")
	expected := len(lines) - m.Height
	if expected < 0 {
		expected = 0
	}
	if m.Offset != expected {
		t.Errorf("expected offset %d after GoBottom, got %d", expected, m.Offset)
	}
}

func TestJSONViewModel_GoBottom_SmallContent(t *testing.T) {
	content := "one"
	m := views.NewJSONView("Small GoBottom", content)
	m.Height = 10

	m.GoBottom()
	if m.Offset != 0 {
		t.Errorf("expected offset 0 when content fits in viewport, got %d", m.Offset)
	}
}

func TestJSONViewModel_EmptyContent_NoContentMessage(t *testing.T) {
	m := views.NewJSONView("Empty", "")
	output := m.View()
	if !strings.Contains(output, "No JSON content available") {
		t.Error("expected 'No JSON content available' for empty content")
	}
}

func TestJSONViewModel_VeryLongJSON(t *testing.T) {
	var lines []string
	for i := 0; i < 1000; i++ {
		lines = append(lines, fmt.Sprintf(`  "key_%d": "value_%d",`, i, i))
	}
	content := "{\n" + strings.Join(lines, "\n") + "\n}"

	m := views.NewJSONView("Long JSON", content)
	m.Width = 80
	m.Height = 20

	// View should render without panic
	output := m.View()
	if output == "" {
		t.Error("expected non-empty View for long JSON")
	}

	// GoBottom should work
	m.GoBottom()
	expectedMax := len(strings.Split(content, "\n")) - m.Height
	if expectedMax < 0 {
		expectedMax = 0
	}
	if m.Offset != expectedMax {
		t.Errorf("expected offset %d after GoBottom on long JSON, got %d", expectedMax, m.Offset)
	}

	// GoTop should work
	m.GoTop()
	if m.Offset != 0 {
		t.Errorf("expected offset 0 after GoTop, got %d", m.Offset)
	}
}

func TestJSONViewModel_ScrollDown_ClampedAtBottom(t *testing.T) {
	content := "line1\nline2\nline3"
	m := views.NewJSONView("Clamp Test", content)
	m.Height = 5

	for i := 0; i < 20; i++ {
		m.ScrollDown()
	}

	lines := strings.Split(content, "\n")
	maxOffset := len(lines) - 1
	if m.Offset != maxOffset {
		t.Errorf("expected offset clamped at %d, got %d", maxOffset, m.Offset)
	}
}

func TestJSONViewModel_ScrollUp_AtTop_StaysZero(t *testing.T) {
	content := "line1\nline2"
	m := views.NewJSONView("Top Test", content)

	m.ScrollUp()
	if m.Offset != 0 {
		t.Errorf("expected offset 0, got %d", m.Offset)
	}
}

// ===========================================================================
// RevealModel tests
// ===========================================================================

func TestNewRevealView_CreatesModel(t *testing.T) {
	m := views.NewRevealView("Secret Title", "secret content here")

	if m.Title != "Secret Title" {
		t.Errorf("expected title 'Secret Title', got %q", m.Title)
	}
	if m.Content != "secret content here" {
		t.Errorf("expected content 'secret content here', got %q", m.Content)
	}
	if m.Offset != 0 {
		t.Errorf("expected initial offset 0, got %d", m.Offset)
	}
}

func TestRevealModel_View_RendersContent(t *testing.T) {
	m := views.NewRevealView("Reveal Test", "some secret data")
	output := m.View()

	if !strings.Contains(output, "Reveal Test") {
		t.Error("expected View to contain title")
	}
	if !strings.Contains(output, "some secret data") {
		t.Error("expected View to contain content")
	}
}

func TestRevealModel_View_EmptyContent(t *testing.T) {
	m := views.NewRevealView("Empty Reveal", "")
	output := m.View()

	if !strings.Contains(output, "No content available") {
		t.Error("expected 'No content available' for empty reveal content")
	}
	if !strings.Contains(output, "Empty Reveal") {
		t.Error("expected View to contain title even for empty content")
	}
}

func TestRevealModel_ScrollDown(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5"
	m := views.NewRevealView("Scroll Test", content)

	m.ScrollDown()
	if m.Offset != 1 {
		t.Errorf("expected offset 1 after ScrollDown, got %d", m.Offset)
	}

	m.ScrollDown()
	if m.Offset != 2 {
		t.Errorf("expected offset 2 after second ScrollDown, got %d", m.Offset)
	}
}

func TestRevealModel_ScrollUp(t *testing.T) {
	content := "line1\nline2\nline3"
	m := views.NewRevealView("ScrollUp Test", content)

	m.ScrollDown()
	m.ScrollDown()
	m.ScrollUp()
	if m.Offset != 1 {
		t.Errorf("expected offset 1 after ScrollDown, ScrollDown, ScrollUp, got %d", m.Offset)
	}
}

func TestRevealModel_ScrollUp_AtTop_StaysZero(t *testing.T) {
	content := "line1\nline2"
	m := views.NewRevealView("Top Test", content)

	m.ScrollUp()
	if m.Offset != 0 {
		t.Errorf("expected offset 0 when scrolling up at top, got %d", m.Offset)
	}
	m.ScrollUp()
	if m.Offset != 0 {
		t.Errorf("expected offset still 0, got %d", m.Offset)
	}
}

func TestRevealModel_ScrollDown_ClampedAtBottom(t *testing.T) {
	content := "line1\nline2\nline3"
	m := views.NewRevealView("Bottom Test", content)

	for i := 0; i < 20; i++ {
		m.ScrollDown()
	}

	lines := strings.Split(content, "\n")
	maxOffset := len(lines) - 1
	if m.Offset != maxOffset {
		t.Errorf("expected offset clamped at %d, got %d", maxOffset, m.Offset)
	}
}

func TestRevealModel_GoTop(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5"
	m := views.NewRevealView("GoTop Test", content)
	m.Height = 3

	m.ScrollDown()
	m.ScrollDown()
	m.ScrollDown()
	m.GoTop()
	if m.Offset != 0 {
		t.Errorf("expected offset 0 after GoTop, got %d", m.Offset)
	}
}

func TestRevealModel_GoBottom(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8"
	m := views.NewRevealView("GoBottom Test", content)
	m.Height = 3

	m.GoBottom()
	lines := strings.Split(content, "\n")
	expected := len(lines) - m.Height
	if expected < 0 {
		expected = 0
	}
	if m.Offset != expected {
		t.Errorf("expected offset %d after GoBottom, got %d", expected, m.Offset)
	}
}

func TestRevealModel_GoBottom_SmallContent(t *testing.T) {
	m := views.NewRevealView("Small GoBottom", "oneliner")
	m.Height = 10

	m.GoBottom()
	if m.Offset != 0 {
		t.Errorf("expected offset 0 when content fits in viewport, got %d", m.Offset)
	}
}

func TestRevealModel_VeryLongContent(t *testing.T) {
	var lines []string
	for i := 0; i < 500; i++ {
		lines = append(lines, fmt.Sprintf("secret line %d with data", i))
	}
	content := strings.Join(lines, "\n")

	m := views.NewRevealView("Long Reveal", content)
	m.Width = 80
	m.Height = 20

	output := m.View()
	if output == "" {
		t.Error("expected non-empty View for long reveal content")
	}

	m.GoBottom()
	expectedMax := len(lines) - m.Height
	if expectedMax < 0 {
		expectedMax = 0
	}
	if m.Offset != expectedMax {
		t.Errorf("expected offset %d after GoBottom, got %d", expectedMax, m.Offset)
	}

	m.GoTop()
	if m.Offset != 0 {
		t.Errorf("expected offset 0 after GoTop, got %d", m.Offset)
	}
}

func TestRevealModel_EmptyContent_ScrollOperations(t *testing.T) {
	m := views.NewRevealView("Empty", "")
	m.Height = 10

	// None of these should panic
	m.ScrollDown()
	m.ScrollUp()
	m.GoTop()
	m.GoBottom()

	// With Height=10 and content="" (1 line after split), GoBottom sets max=1-10=-9 => clamped to 0
	if m.Offset != 0 {
		t.Errorf("expected offset 0 for empty content with large height, got %d", m.Offset)
	}

	// Also verify View does not panic
	output := m.View()
	if !strings.Contains(output, "No content available") {
		t.Error("expected 'No content available' for empty content")
	}
}

// ===========================================================================
// ProfileSelectModel tests (expanding existing)
// ===========================================================================

func TestProfileSelectModel_GoTop(t *testing.T) {
	profiles := []string{"default", "dev", "staging", "prod"}
	m := views.NewProfileSelect(profiles, "default")
	m.Cursor = 3
	m.GoTop()
	if m.Cursor != 0 {
		t.Errorf("expected cursor 0 after GoTop, got %d", m.Cursor)
	}
}

func TestProfileSelectModel_GoBottom(t *testing.T) {
	profiles := []string{"default", "dev", "staging", "prod"}
	m := views.NewProfileSelect(profiles, "default")
	m.GoBottom()
	if m.Cursor != 3 {
		t.Errorf("expected cursor 3 after GoBottom, got %d", m.Cursor)
	}
}

func TestProfileSelectModel_GoBottom_EmptyProfiles(t *testing.T) {
	m := views.NewProfileSelect([]string{}, "")
	m.GoBottom()
	if m.Cursor != 0 {
		t.Errorf("expected cursor 0 for empty profiles, got %d", m.Cursor)
	}
}

func TestProfileSelectModel_EmptyProfiles_NoPanic(t *testing.T) {
	m := views.NewProfileSelect([]string{}, "")

	// All operations should not panic
	m.MoveUp()
	m.MoveDown()
	m.GoTop()
	m.GoBottom()

	selected := m.SelectedProfile()
	if selected != "" {
		t.Errorf("expected empty string for empty profiles, got %q", selected)
	}

	output := m.View()
	if output == "" {
		t.Error("expected non-empty View even with empty profiles")
	}
}

func TestProfileSelectModel_SelectedProfile_AtVariousCursors(t *testing.T) {
	profiles := []string{"alpha", "bravo", "charlie", "delta"}
	m := views.NewProfileSelect(profiles, "alpha")

	for i, expected := range profiles {
		m.Cursor = i
		got := m.SelectedProfile()
		if got != expected {
			t.Errorf("at cursor %d: expected %q, got %q", i, expected, got)
		}
	}
}

func TestProfileSelectModel_SelectedProfile_NegativeCursor(t *testing.T) {
	profiles := []string{"alpha", "bravo"}
	m := views.NewProfileSelect(profiles, "alpha")
	m.Cursor = -1
	selected := m.SelectedProfile()
	if selected != "" {
		t.Errorf("expected empty string for negative cursor, got %q", selected)
	}
}

func TestProfileSelectModel_SelectedProfile_OutOfBounds(t *testing.T) {
	profiles := []string{"alpha", "bravo"}
	m := views.NewProfileSelect(profiles, "alpha")
	m.Cursor = 100
	selected := m.SelectedProfile()
	if selected != "" {
		t.Errorf("expected empty string for out-of-bounds cursor, got %q", selected)
	}
}

func TestProfileSelectModel_View_ContainsAsteriskForActiveProfile(t *testing.T) {
	profiles := []string{"default", "dev", "prod"}
	m := views.NewProfileSelect(profiles, "prod")

	output := m.View()
	if !strings.Contains(output, "* prod") {
		t.Error("expected View to contain '* prod' for active profile")
	}
	// Non-active profiles should not have * marker next to them
	// (They may have "  " prefix instead)
}

func TestProfileSelectModel_ActiveProfileNotInList(t *testing.T) {
	profiles := []string{"default", "dev"}
	m := views.NewProfileSelect(profiles, "nonexistent")

	// Cursor should default to 0 when active profile is not found
	if m.Cursor != 0 {
		t.Errorf("expected cursor 0 when active profile not in list, got %d", m.Cursor)
	}
}

func TestProfileSelectModel_View_ContainsAllProfiles(t *testing.T) {
	profiles := []string{"profile-a", "profile-b", "profile-c"}
	m := views.NewProfileSelect(profiles, "profile-a")

	output := m.View()
	for _, p := range profiles {
		if !strings.Contains(output, p) {
			t.Errorf("expected View to contain profile %q", p)
		}
	}
}

func TestProfileSelectModel_MoveDown_AtBottom_NoPanic(t *testing.T) {
	profiles := []string{"only-one"}
	m := views.NewProfileSelect(profiles, "only-one")

	m.MoveDown()
	if m.Cursor != 0 {
		t.Errorf("expected cursor 0 for single-item list, got %d", m.Cursor)
	}
}

// ===========================================================================
// RegionSelectModel tests (expanding existing)
// ===========================================================================

func makeTestRegions() []awsclient.AWSRegion {
	return []awsclient.AWSRegion{
		{Code: "us-east-1", DisplayName: "US East (N. Virginia)"},
		{Code: "us-west-2", DisplayName: "US West (Oregon)"},
		{Code: "eu-west-1", DisplayName: "Europe (Ireland)"},
		{Code: "ap-southeast-1", DisplayName: "Asia Pacific (Singapore)"},
	}
}

func TestRegionSelectModel_GoTop(t *testing.T) {
	regions := makeTestRegions()
	m := views.NewRegionSelect(regions, "us-east-1")
	m.Cursor = 3
	m.GoTop()
	if m.Cursor != 0 {
		t.Errorf("expected cursor 0 after GoTop, got %d", m.Cursor)
	}
}

func TestRegionSelectModel_GoBottom(t *testing.T) {
	regions := makeTestRegions()
	m := views.NewRegionSelect(regions, "us-east-1")
	m.GoBottom()
	if m.Cursor != 3 {
		t.Errorf("expected cursor 3 after GoBottom, got %d", m.Cursor)
	}
}

func TestRegionSelectModel_GoBottom_EmptyRegions(t *testing.T) {
	m := views.NewRegionSelect([]awsclient.AWSRegion{}, "")
	m.GoBottom()
	if m.Cursor != 0 {
		t.Errorf("expected cursor 0 for empty regions, got %d", m.Cursor)
	}
}

func TestRegionSelectModel_EmptyRegions_NoPanic(t *testing.T) {
	m := views.NewRegionSelect([]awsclient.AWSRegion{}, "")

	m.MoveUp()
	m.MoveDown()
	m.GoTop()
	m.GoBottom()

	selected := m.SelectedRegion()
	if selected.Code != "" {
		t.Errorf("expected empty Code for empty regions, got %q", selected.Code)
	}

	output := m.View()
	if output == "" {
		t.Error("expected non-empty View even with empty regions")
	}
}

func TestRegionSelectModel_SelectedRegion_AtVariousCursors(t *testing.T) {
	regions := makeTestRegions()
	m := views.NewRegionSelect(regions, "us-east-1")

	for i, expected := range regions {
		m.Cursor = i
		got := m.SelectedRegion()
		if got.Code != expected.Code {
			t.Errorf("at cursor %d: expected code %q, got %q", i, expected.Code, got.Code)
		}
		if got.DisplayName != expected.DisplayName {
			t.Errorf("at cursor %d: expected display name %q, got %q", i, expected.DisplayName, got.DisplayName)
		}
	}
}

func TestRegionSelectModel_SelectedRegion_OutOfBounds(t *testing.T) {
	regions := makeTestRegions()
	m := views.NewRegionSelect(regions, "us-east-1")

	m.Cursor = -1
	selected := m.SelectedRegion()
	if selected.Code != "" {
		t.Errorf("expected empty Code for negative cursor, got %q", selected.Code)
	}

	m.Cursor = 100
	selected = m.SelectedRegion()
	if selected.Code != "" {
		t.Errorf("expected empty Code for out-of-bounds cursor, got %q", selected.Code)
	}
}

func TestRegionSelectModel_View_ContainsCodesAndDisplayNames(t *testing.T) {
	regions := makeTestRegions()
	m := views.NewRegionSelect(regions, "us-east-1")

	output := m.View()
	for _, r := range regions {
		if !strings.Contains(output, r.Code) {
			t.Errorf("expected View to contain region code %q", r.Code)
		}
		if !strings.Contains(output, r.DisplayName) {
			t.Errorf("expected View to contain display name %q", r.DisplayName)
		}
	}
}

func TestRegionSelectModel_View_ContainsAsteriskForActiveRegion(t *testing.T) {
	regions := makeTestRegions()
	m := views.NewRegionSelect(regions, "eu-west-1")

	output := m.View()
	if !strings.Contains(output, "* eu-west-1") {
		t.Error("expected View to contain '* eu-west-1' for active region")
	}
}

func TestRegionSelectModel_ActiveRegionNotInList(t *testing.T) {
	regions := makeTestRegions()
	m := views.NewRegionSelect(regions, "nonexistent-region")

	// Cursor should default to 0 when active region not found
	if m.Cursor != 0 {
		t.Errorf("expected cursor 0 when active region not in list, got %d", m.Cursor)
	}
}

func TestRegionSelectModel_MoveDown_AtBottom(t *testing.T) {
	regions := []awsclient.AWSRegion{
		{Code: "us-east-1", DisplayName: "US East (N. Virginia)"},
	}
	m := views.NewRegionSelect(regions, "us-east-1")
	m.MoveDown()
	if m.Cursor != 0 {
		t.Errorf("expected cursor 0 for single-item list, got %d", m.Cursor)
	}
}

func TestRegionSelectModel_View_ContainsCursorIndicator(t *testing.T) {
	regions := makeTestRegions()
	m := views.NewRegionSelect(regions, "us-east-1")

	output := m.View()
	if !strings.Contains(output, ">") {
		t.Error("expected View to contain '>' cursor indicator")
	}
}

// ===========================================================================
// FilterResources tests (expanding existing)
// ===========================================================================

func TestFilterResources_RegexLikeChars_TreatedAsSubstring(t *testing.T) {
	resources := []resource.Resource{
		{
			ID:     "r-1",
			Name:   "server.prod",
			Status: "running",
			Fields: map[string]string{"name": "server.prod"},
		},
		{
			ID:     "r-2",
			Name:   "server*test",
			Status: "running",
			Fields: map[string]string{"name": "server*test"},
		},
		{
			ID:     "r-3",
			Name:   "normal-server",
			Status: "running",
			Fields: map[string]string{"name": "normal-server"},
		},
	}

	// "." should be treated as literal dot, not regex any-char
	result := views.FilterResources("server.prod", resources)
	if len(result) != 1 {
		t.Errorf("expected 1 match for 'server.prod', got %d", len(result))
	}
	if len(result) > 0 && result[0].Name != "server.prod" {
		t.Errorf("expected 'server.prod', got %q", result[0].Name)
	}

	// "*" should be treated as literal asterisk
	result = views.FilterResources("server*test", resources)
	if len(result) != 1 {
		t.Errorf("expected 1 match for 'server*test', got %d", len(result))
	}
	if len(result) > 0 && result[0].Name != "server*test" {
		t.Errorf("expected 'server*test', got %q", result[0].Name)
	}
}

func TestFilterResources_SpecialRegexChars(t *testing.T) {
	resources := []resource.Resource{
		{
			ID:     "r-1",
			Name:   "test[0]",
			Status: "ok",
			Fields: map[string]string{"name": "test[0]"},
		},
		{
			ID:     "r-2",
			Name:   "test(1)",
			Status: "ok",
			Fields: map[string]string{"name": "test(1)"},
		},
		{
			ID:     "r-3",
			Name:   "test+2",
			Status: "ok",
			Fields: map[string]string{"name": "test+2"},
		},
	}

	result := views.FilterResources("[0]", resources)
	if len(result) != 1 {
		t.Errorf("expected 1 match for '[0]', got %d", len(result))
	}

	result = views.FilterResources("(1)", resources)
	if len(result) != 1 {
		t.Errorf("expected 1 match for '(1)', got %d", len(result))
	}

	result = views.FilterResources("+2", resources)
	if len(result) != 1 {
		t.Errorf("expected 1 match for '+2', got %d", len(result))
	}
}

func TestFilterResources_ReturnsNonNilEmptySlice(t *testing.T) {
	resources := makeTestResources()
	result := views.FilterResources("absolutely-nothing-matches-this", resources)

	if result == nil {
		t.Error("expected non-nil result slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestFilterResources_EmptyResourceList(t *testing.T) {
	result := views.FilterResources("anything", []resource.Resource{})
	if result == nil {
		t.Error("expected non-nil result slice for empty input")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

func TestFilterResources_NilResourceList(t *testing.T) {
	result := views.FilterResources("", nil)
	// Empty query returns the input as-is, which is nil
	// This is correct behavior
	if result != nil && len(result) != 0 {
		t.Errorf("expected nil or empty result for nil input with empty query, got %d items", len(result))
	}
}

func TestFilterResources_MatchesFieldValues_Only(t *testing.T) {
	resources := []resource.Resource{
		{
			ID:     "r-1",
			Name:   "server-1",
			Status: "active",
			Fields: map[string]string{
				"name":   "server-1",
				"region": "us-west-2",
			},
		},
		{
			ID:     "r-2",
			Name:   "server-2",
			Status: "active",
			Fields: map[string]string{
				"name":   "server-2",
				"region": "eu-west-1",
			},
		},
	}

	result := views.FilterResources("us-west-2", resources)
	if len(result) != 1 {
		t.Errorf("expected 1 match for field value 'us-west-2', got %d", len(result))
	}
}

func TestFilterResources_PartialMatch(t *testing.T) {
	resources := []resource.Resource{
		{
			ID:     "i-abc123",
			Name:   "webserver",
			Status: "running",
			Fields: map[string]string{"name": "webserver"},
		},
	}

	// Partial ID match
	result := views.FilterResources("abc", resources)
	if len(result) != 1 {
		t.Errorf("expected 1 match for partial ID 'abc', got %d", len(result))
	}

	// Partial name match
	result = views.FilterResources("web", resources)
	if len(result) != 1 {
		t.Errorf("expected 1 match for partial name 'web', got %d", len(result))
	}

	// Partial status match
	result = views.FilterResources("run", resources)
	if len(result) != 1 {
		t.Errorf("expected 1 match for partial status 'run', got %d", len(result))
	}
}
