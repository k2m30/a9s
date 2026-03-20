package unit

import (
	"os"
	"strings"
	"testing"

	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/messages"
	"github.com/k2m30/a9s/internal/tui/styles"
	"github.com/k2m30/a9s/internal/tui/views"
)

// ===========================================================================
// Helpers for sort order tests
// ===========================================================================

// sortTestTypeDef returns an EC2-like type definition with Name, State, and
// Launch Time columns so that sorting can be verified on all three fields.
func sortTestTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		Name:      "EC2 Instances",
		ShortName: "ec2",
		Aliases:   []string{"ec2"},
		Columns: []resource.Column{
			{Key: "instance_id", Title: "Instance ID", Width: 14},
			{Key: "name", Title: "Name", Width: 20},
			{Key: "state", Title: "State", Width: 14},
			{Key: "launch_time", Title: "Launch Time", Width: 24},
		},
	}
}

// sortTestResources returns 5 resources with deliberate out-of-order names,
// mixed statuses, and distinct timestamps so that sort tests can verify the
// actual row order in the rendered output.
func sortTestResources() []resource.Resource {
	return []resource.Resource{
		{
			ID: "i-001", Name: "echo", Status: "terminated",
			Fields: map[string]string{
				"instance_id": "i-001", "name": "echo", "state": "terminated",
				"launch_time": "2026-01-05T00:00:00Z",
			},
		},
		{
			ID: "i-002", Name: "alpha", Status: "running",
			Fields: map[string]string{
				"instance_id": "i-002", "name": "alpha", "state": "running",
				"launch_time": "2026-01-01T00:00:00Z",
			},
		},
		{
			ID: "i-003", Name: "delta", Status: "stopped",
			Fields: map[string]string{
				"instance_id": "i-003", "name": "delta", "state": "stopped",
				"launch_time": "2026-01-04T00:00:00Z",
			},
		},
		{
			ID: "i-004", Name: "bravo", Status: "pending",
			Fields: map[string]string{
				"instance_id": "i-004", "name": "bravo", "state": "pending",
				"launch_time": "2026-01-02T00:00:00Z",
			},
		},
		{
			ID: "i-005", Name: "charlie", Status: "running",
			Fields: map[string]string{
				"instance_id": "i-005", "name": "charlie", "state": "running",
				"launch_time": "2026-01-03T00:00:00Z",
			},
		},
	}
}

// sortLoadedModel builds a ResourceListModel with NO_COLOR disabled so
// stripANSI is the only transform needed. Returns the model ready for
// sort key presses.
func sortLoadedModel(t *testing.T) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})

	td := sortTestTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    sortTestResources(),
	})
	return m
}

// extractDataNames parses the rendered output and returns the name column
// values in the order they appear, skipping the header row.
// It looks for known fixture names in each line.
func extractDataNames(rendered string) []string {
	knownNames := []string{"alpha", "bravo", "charlie", "delta", "echo"}
	lines := strings.Split(rendered, "\n")
	var names []string
	for _, line := range lines {
		plain := stripANSI(line)
		for _, name := range knownNames {
			if strings.Contains(plain, name) {
				names = append(names, name)
				break
			}
		}
	}
	return names
}

// extractDataStatuses parses rendered output and returns the status column
// values in the order they appear.
func extractDataStatuses(rendered string) []string {
	knownStatuses := []string{"pending", "running", "stopped", "terminated"}
	lines := strings.Split(rendered, "\n")
	var statuses []string
	for _, line := range lines {
		plain := stripANSI(line)
		// Skip the header line
		if strings.Contains(plain, "State") && strings.Contains(plain, "Name") {
			continue
		}
		for _, status := range knownStatuses {
			if strings.Contains(plain, status) {
				statuses = append(statuses, status)
				break
			}
		}
	}
	return statuses
}

// extractDataTimestamps parses rendered output and returns the launch_time
// column values in the order they appear.
func extractDataTimestamps(rendered string) []string {
	lines := strings.Split(rendered, "\n")
	var timestamps []string
	for _, line := range lines {
		plain := stripANSI(line)
		// Skip header
		if strings.Contains(plain, "Launch Time") && strings.Contains(plain, "Name") {
			continue
		}
		// Look for ISO timestamps
		if idx := strings.Index(plain, "2026-01-0"); idx >= 0 {
			// Extract the timestamp substring
			end := idx + len("2026-01-05T00:00:00Z")
			if end <= len(plain) {
				timestamps = append(timestamps, plain[idx:end])
			}
		}
	}
	return timestamps
}

// extractDataIDs parses the rendered output and returns the instance ID
// values in the order they appear, skipping the header row.
func extractDataIDs(rendered string) []string {
	knownIDs := []string{"i-001", "i-002", "i-003", "i-004", "i-005"}
	lines := strings.Split(rendered, "\n")
	var ids []string
	for _, line := range lines {
		plain := stripANSI(line)
		// Skip the header line
		if strings.Contains(plain, "Instance ID") && strings.Contains(plain, "Name") {
			continue
		}
		for _, id := range knownIDs {
			if strings.Contains(plain, id) {
				ids = append(ids, id)
				break
			}
		}
	}
	return ids
}

// ===========================================================================
// Issue 1: Sort by Name tests — verify actual data order
// ===========================================================================

func TestQA_SortOrder_NameAscending(t *testing.T) {
	m := sortLoadedModel(t)

	// Press N to sort by name ascending
	m, _ = m.Update(rlKeyPress("N"))

	rendered := m.View()
	names := extractDataNames(rendered)

	expected := []string{"alpha", "bravo", "charlie", "delta", "echo"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d names, got %d: %v", len(expected), len(names), names)
	}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("position %d: expected %q, got %q (full order: %v)", i, name, names[i], names)
		}
	}
}

func TestQA_SortOrder_NameDescending(t *testing.T) {
	m := sortLoadedModel(t)

	// Press N twice: first ascending, second toggles to descending
	m, _ = m.Update(rlKeyPress("N"))
	m, _ = m.Update(rlKeyPress("N"))

	rendered := m.View()
	names := extractDataNames(rendered)

	expected := []string{"echo", "delta", "charlie", "bravo", "alpha"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d names, got %d: %v", len(expected), len(names), names)
	}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("position %d: expected %q, got %q (full order: %v)", i, name, names[i], names)
		}
	}
}

// ===========================================================================
// Sort by ID tests — verify actual data order
// ===========================================================================

func TestQA_SortOrder_IDAscending(t *testing.T) {
	m := sortLoadedModel(t)

	// Press I to sort by ID ascending
	m, _ = m.Update(rlKeyPress("I"))

	rendered := m.View()
	ids := extractDataIDs(rendered)

	expected := []string{"i-001", "i-002", "i-003", "i-004", "i-005"}
	if len(ids) != len(expected) {
		t.Fatalf("expected %d IDs, got %d: %v", len(expected), len(ids), ids)
	}
	for i, id := range expected {
		if ids[i] != id {
			t.Errorf("position %d: expected %q, got %q (full order: %v)", i, id, ids[i], ids)
		}
	}
}

func TestQA_SortOrder_IDDescending(t *testing.T) {
	m := sortLoadedModel(t)

	// Press I twice: ascending then descending
	m, _ = m.Update(rlKeyPress("I"))
	m, _ = m.Update(rlKeyPress("I"))

	rendered := m.View()
	ids := extractDataIDs(rendered)

	expected := []string{"i-005", "i-004", "i-003", "i-002", "i-001"}
	if len(ids) != len(expected) {
		t.Fatalf("expected %d IDs, got %d: %v", len(expected), len(ids), ids)
	}
	for i, id := range expected {
		if ids[i] != id {
			t.Errorf("position %d: expected %q, got %q (full order: %v)", i, id, ids[i], ids)
		}
	}
}

// ===========================================================================
// Sort by Age tests — verify actual data order
// ===========================================================================

func TestQA_SortOrder_AgeAscending(t *testing.T) {
	m := sortLoadedModel(t)

	// Press A to sort by age (launch_time) ascending
	m, _ = m.Update(rlKeyPress("A"))

	rendered := m.View()
	timestamps := extractDataTimestamps(rendered)

	if len(timestamps) != 5 {
		t.Fatalf("expected 5 timestamps, got %d: %v", len(timestamps), timestamps)
	}

	// Ascending: earliest first
	for i := 0; i < len(timestamps)-1; i++ {
		if timestamps[i] > timestamps[i+1] {
			t.Errorf("timestamp at position %d (%q) should be <= position %d (%q); full order: %v",
				i, timestamps[i], i+1, timestamps[i+1], timestamps)
		}
	}
}

func TestQA_SortOrder_AgeDescending(t *testing.T) {
	m := sortLoadedModel(t)

	// Press A twice: ascending then descending
	m, _ = m.Update(rlKeyPress("A"))
	m, _ = m.Update(rlKeyPress("A"))

	rendered := m.View()
	timestamps := extractDataTimestamps(rendered)

	if len(timestamps) != 5 {
		t.Fatalf("expected 5 timestamps, got %d: %v", len(timestamps), timestamps)
	}

	// Descending: latest first
	for i := 0; i < len(timestamps)-1; i++ {
		if timestamps[i] < timestamps[i+1] {
			t.Errorf("timestamp at position %d (%q) should be >= position %d (%q); full order: %v",
				i, timestamps[i], i+1, timestamps[i+1], timestamps)
		}
	}
}

// ===========================================================================
// Sort preserves all data (no rows lost or duplicated after sort)
// ===========================================================================

func TestQA_SortOrder_PreservesAllRows(t *testing.T) {
	m := sortLoadedModel(t)

	// Get names before sort
	namesBefore := extractDataNames(m.View())

	// Sort ascending, then descending, then by different field
	m, _ = m.Update(rlKeyPress("N"))
	namesAfterSort := extractDataNames(m.View())

	if len(namesBefore) != len(namesAfterSort) {
		t.Fatalf("sort changed row count: before=%d, after=%d", len(namesBefore), len(namesAfterSort))
	}

	// Verify all original names are still present (just reordered)
	nameSet := make(map[string]bool)
	for _, n := range namesAfterSort {
		nameSet[n] = true
	}
	for _, n := range namesBefore {
		if !nameSet[n] {
			t.Errorf("name %q present before sort but missing after sort", n)
		}
	}
}

// ===========================================================================
// Sort toggle cycle: pressing same key 3 times returns to ascending
// ===========================================================================

func TestQA_SortOrder_ToggleCycle(t *testing.T) {
	m := sortLoadedModel(t)

	// First press: ascending
	m, _ = m.Update(rlKeyPress("N"))
	namesAsc := extractDataNames(m.View())

	// Second press: descending
	m, _ = m.Update(rlKeyPress("N"))
	namesDesc := extractDataNames(m.View())

	// Third press: back to ascending
	m, _ = m.Update(rlKeyPress("N"))
	namesAscAgain := extractDataNames(m.View())

	// Asc and desc should be different
	if strings.Join(namesAsc, ",") == strings.Join(namesDesc, ",") {
		t.Error("ascending and descending sort should produce different orders")
	}

	// First ascending and third press (ascending again) should match
	if strings.Join(namesAsc, ",") != strings.Join(namesAscAgain, ",") {
		t.Errorf("third press should restore ascending order: first=%v, third=%v", namesAsc, namesAscAgain)
	}
}

// ===========================================================================
// Switching sort field resets to ascending of the new field
// ===========================================================================

func TestQA_SortOrder_SwitchFieldResetsToAscending(t *testing.T) {
	m := sortLoadedModel(t)

	// Sort by name descending (2 presses)
	m, _ = m.Update(rlKeyPress("N"))
	m, _ = m.Update(rlKeyPress("N"))

	// Now switch to ID sort — should be ascending
	m, _ = m.Update(rlKeyPress("I"))

	rendered := m.View()
	ids := extractDataIDs(rendered)

	if len(ids) < 2 {
		t.Fatal("expected at least 2 ID entries")
	}

	// IDs should be in ascending order
	for i := 0; i < len(ids)-1; i++ {
		if ids[i] > ids[i+1] {
			t.Errorf("after switching to ID sort, should be ascending: got %v", ids)
			break
		}
	}
}

// ===========================================================================
// Sort indicator matches actual sort direction
// ===========================================================================

func TestQA_SortOrder_IndicatorMatchesActualOrder(t *testing.T) {
	m := sortLoadedModel(t)

	// Sort by name ascending
	m, _ = m.Update(rlKeyPress("N"))

	rendered := m.View()
	plain := stripANSI(rendered)
	names := extractDataNames(rendered)

	// Ascending indicator should be present
	if !strings.Contains(plain, "\u2191") {
		t.Error("ascending sort should show up-arrow indicator")
	}

	// And data should actually be ascending
	if len(names) >= 2 && names[0] != "alpha" {
		t.Errorf("ascending sort indicator shown but data not ascending: first name = %q", names[0])
	}

	// Toggle to descending
	m, _ = m.Update(rlKeyPress("N"))

	rendered = m.View()
	plain = stripANSI(rendered)
	names = extractDataNames(rendered)

	if !strings.Contains(plain, "\u2193") {
		t.Error("descending sort should show down-arrow indicator")
	}

	if len(names) >= 2 && names[0] != "echo" {
		t.Errorf("descending sort indicator shown but data not descending: first name = %q", names[0])
	}
}
