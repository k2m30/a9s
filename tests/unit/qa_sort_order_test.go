package unit

import (
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
				"launch_time": "2026-01-05 00:00",
			},
		},
		{
			ID: "i-002", Name: "alpha", Status: "running",
			Fields: map[string]string{
				"instance_id": "i-002", "name": "alpha", "state": "running",
				"launch_time": "2026-01-01 00:00",
			},
		},
		{
			ID: "i-003", Name: "delta", Status: "stopped",
			Fields: map[string]string{
				"instance_id": "i-003", "name": "delta", "state": "stopped",
				"launch_time": "2026-01-04 00:00",
			},
		},
		{
			ID: "i-004", Name: "bravo", Status: "pending",
			Fields: map[string]string{
				"instance_id": "i-004", "name": "bravo", "state": "pending",
				"launch_time": "2026-01-02 00:00",
			},
		},
		{
			ID: "i-005", Name: "charlie", Status: "running",
			Fields: map[string]string{
				"instance_id": "i-005", "name": "charlie", "state": "running",
				"launch_time": "2026-01-03 00:00",
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
			end := idx + len("2026-01-05 00:00")
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

	// Press 2 to sort by name ascending (column 1 = name, 1-indexed key "2")
	m, _ = m.Update(rlKeyPress("2"))

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

	// Press 2 twice: first ascending, second toggles to descending (column 1 = name)
	m, _ = m.Update(rlKeyPress("2"))
	m, _ = m.Update(rlKeyPress("2"))

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

	// Press 1 to sort by ID ascending (column 0 = instance_id, 1-indexed key "1")
	m, _ = m.Update(rlKeyPress("1"))

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

	// Press 1 twice: ascending then descending (column 0 = instance_id)
	m, _ = m.Update(rlKeyPress("1"))
	m, _ = m.Update(rlKeyPress("1"))

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

	// Press 4 to sort by age (launch_time) ascending (column 3 = launch_time, 1-indexed key "4")
	m, _ = m.Update(rlKeyPress("4"))

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

	// Press 4 twice: ascending then descending (column 3 = launch_time)
	m, _ = m.Update(rlKeyPress("4"))
	m, _ = m.Update(rlKeyPress("4"))

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
	m, _ = m.Update(rlKeyPress("2"))
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

	// First press: ascending (column 1 = name, key "2")
	m, _ = m.Update(rlKeyPress("2"))
	namesAsc := extractDataNames(m.View())

	// Second press: descending
	m, _ = m.Update(rlKeyPress("2"))
	namesDesc := extractDataNames(m.View())

	// Third press: back to ascending
	m, _ = m.Update(rlKeyPress("2"))
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

	// Sort by name descending (2 presses of "2" — column 1 = name)
	m, _ = m.Update(rlKeyPress("2"))
	m, _ = m.Update(rlKeyPress("2"))

	// Now switch to ID sort — should be ascending (column 0 = instance_id, key "1")
	m, _ = m.Update(rlKeyPress("1"))

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

	// Sort by name ascending (column 1 = name, key "2")
	m, _ = m.Update(rlKeyPress("2"))

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
	m, _ = m.Update(rlKeyPress("2"))

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

// ===========================================================================
// Bug fix: Sort by Age must work for child view fields like "last_event"
// that don't contain "time" or "date" in their key names.
// ===========================================================================

// logStreamSortTypeDef builds a log-streams-like type def for sort testing.
func logStreamSortTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		Name:      "Log Streams",
		ShortName: "log_streams",
		Columns: []resource.Column{
			{Key: "stream_name", Title: "Stream Name", Width: 48},
			{Key: "last_event", Title: "Last Event", Width: 22},
			{Key: "first_event", Title: "First Event", Width: 22},
		},
	}
}

func logStreamSortResources() []resource.Resource {
	return []resource.Resource{
		{
			ID: "stream-c", Name: "stream-c", Status: "",
			Fields: map[string]string{
				"stream_name": "stream-c", "last_event": "2026-03-22 02:47",
				"first_event": "2026-03-22 01:00",
			},
		},
		{
			ID: "stream-a", Name: "stream-a", Status: "",
			Fields: map[string]string{
				"stream_name": "stream-a", "last_event": "2026-03-20 10:00",
				"first_event": "2026-03-20 09:00",
			},
		},
		{
			ID: "stream-b", Name: "stream-b", Status: "",
			Fields: map[string]string{
				"stream_name": "stream-b", "last_event": "2026-03-21 15:30",
				"first_event": "2026-03-21 14:00",
			},
		},
	}
}

func TestQA_SortOrder_LogStreams_AgeSortWorks(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})

	td := logStreamSortTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(200, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "log_streams",
		Resources:    logStreamSortResources(),
	})

	// Press 2 to sort by age ascending (column 1 = last_event, 1-indexed key "2")
	m, _ = m.Update(rlKeyPress("2"))

	rendered := m.View()
	plain := stripANSI(rendered)

	// Verify that sort by age actually reorders:
	// stream-a (2026-03-20) should come before stream-b (2026-03-21)
	// which should come before stream-c (2026-03-22) in ascending order
	idxA := strings.Index(plain, "stream-a")
	idxB := strings.Index(plain, "stream-b")
	idxC := strings.Index(plain, "stream-c")

	if idxA < 0 || idxB < 0 || idxC < 0 {
		t.Fatalf("could not find all stream names in rendered output:\n%s", plain)
	}

	if idxA > idxB || idxB > idxC {
		t.Errorf("age ascending sort failed for log streams: stream-a@%d, stream-b@%d, stream-c@%d — fields like 'last_event' must be recognized as time fields",
			idxA, idxB, idxC)
	}
}

// ===========================================================================
// Bug fix: Age sort must use the FIRST time-related column in column order,
// not a random field from the Fields map (which has non-deterministic iteration).
// ===========================================================================

// multiTimeFieldTypeDef builds a type def with two time columns to expose
// non-deterministic field selection in getAgeField.
// Columns: name(20), creation_date(24), modified_date(24).
// Both "creation_date" and "modified_date" match the "date" keyword in
// getAgeField, so if the implementation iterates the Fields map (non-
// deterministic), it may pick either column. The fix must use column
// definition order instead.
func multiTimeFieldTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		Name:      "Multi Time Fields",
		ShortName: "multi_time",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 20},
			{Key: "creation_date", Title: "Creation Date", Width: 24},
			{Key: "modified_date", Title: "Modified Date", Width: 24},
		},
	}
}

// multiTimeFieldResources returns 3 resources where creation_date and
// modified_date produce different sort orders, so the test can detect
// which column is actually used.
//
// By creation_date ascending: alpha, charlie, bravo
// By modified_date ascending: bravo, charlie, alpha
func multiTimeFieldResources() []resource.Resource {
	return []resource.Resource{
		{
			ID: "r-001", Name: "alpha", Status: "",
			Fields: map[string]string{
				"name":          "alpha",
				"creation_date": "2026-01-01 00:00",
				"modified_date": "2026-03-01 00:00",
			},
		},
		{
			ID: "r-002", Name: "bravo", Status: "",
			Fields: map[string]string{
				"name":          "bravo",
				"creation_date": "2026-01-03 00:00",
				"modified_date": "2026-02-01 00:00",
			},
		},
		{
			ID: "r-003", Name: "charlie", Status: "",
			Fields: map[string]string{
				"name":          "charlie",
				"creation_date": "2026-01-02 00:00",
				"modified_date": "2026-02-15 00:00",
			},
		},
	}
}

// TestQA_SortOrder_AgeDeterministic_MultipleTimeFields verifies that when a
// resource type has multiple time-related columns, age sort deterministically
// uses the FIRST time column in column definition order (creation_date), not
// an arbitrary one from the Fields map (which would non-deterministically
// select modified_date on some Go map iterations).
//
// Both "creation_date" and "modified_date" contain the "date" keyword that
// getAgeField matches, so a map-iterating implementation may pick either.
//
// Expected ascending order by creation_date: alpha, charlie, bravo.
// If modified_date were used instead: bravo, charlie, alpha — a different order.
func TestQA_SortOrder_AgeDeterministic_MultipleTimeFields(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})

	td := multiTimeFieldTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "multi_time",
		Resources:    multiTimeFieldResources(),
	})

	// Press 2 to sort by age ascending — column 1 = creation_date (first time column)
	m, _ = m.Update(rlKeyPress("2"))

	rendered := m.View()
	plain := stripANSI(rendered)

	idxAlpha := strings.Index(plain, "alpha")
	idxBravo := strings.Index(plain, "bravo")
	idxCharlie := strings.Index(plain, "charlie")

	if idxAlpha < 0 || idxBravo < 0 || idxCharlie < 0 {
		t.Fatalf("could not find all resource names in rendered output:\n%s", plain)
	}

	// creation_date ascending order: alpha (2026-01-01) < charlie (2026-01-02) < bravo (2026-01-03)
	if idxAlpha > idxCharlie || idxCharlie > idxBravo {
		t.Errorf(
			"age sort must use first time column (creation_date), not modified_date: "+
				"got alpha@%d charlie@%d bravo@%d — "+
				"expected alpha before charlie before bravo (creation_date order); "+
				"if modified_date were used, order would be bravo, charlie, alpha",
			idxAlpha, idxCharlie, idxBravo,
		)
	}
}

// TestQA_SortOrder_AgeUsesFirstColumnMatch verifies deterministic column
// selection when the first time-related column is named "started" and a
// second time column is named "last_event". The sort must use "started".
//
// Expected ascending order by started: foo, bar.
// If last_event were used: bar, foo — reversed.
func TestQA_SortOrder_AgeUsesFirstColumnMatch(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})

	td := resource.ResourceTypeDef{
		Name:      "Started First Type",
		ShortName: "started_first",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 20},
			{Key: "started", Title: "Started", Width: 24},
			{Key: "last_event", Title: "Last Event", Width: 24},
		},
	}

	resources := []resource.Resource{
		{
			ID: "s-001", Name: "foo", Status: "",
			Fields: map[string]string{
				"name":       "foo",
				"started":    "2026-01-01 00:00",
				"last_event": "2026-03-01 00:00",
			},
		},
		{
			ID: "s-002", Name: "bar", Status: "",
			Fields: map[string]string{
				"name":       "bar",
				"started":    "2026-01-02 00:00",
				"last_event": "2026-02-01 00:00",
			},
		},
	}

	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "started_first",
		Resources:    resources,
	})

	// Press 2 to sort by age ascending — column 1 = started (first time column)
	m, _ = m.Update(rlKeyPress("2"))

	rendered := m.View()
	plain := stripANSI(rendered)

	idxFoo := strings.Index(plain, "foo")
	idxBar := strings.Index(plain, "bar")

	if idxFoo < 0 || idxBar < 0 {
		t.Fatalf("could not find resource names in rendered output:\n%s", plain)
	}

	// started ascending order: foo (2026-01-01) before bar (2026-01-02)
	if idxFoo > idxBar {
		t.Errorf(
			"age sort must use first time column (started), not last_event: "+
				"got foo@%d bar@%d — expected foo before bar (started order); "+
				"if last_event were used, bar would appear before foo",
			idxFoo, idxBar,
		)
	}
}

// ===========================================================================
// Issue 208: Data-driven list title via ResourceTypeDef.ListTitle
// ===========================================================================

// TestQA_ListTitle_UsedInFrameTitle verifies that when ResourceTypeDef.ListTitle
// is set, FrameTitle() uses it as the base name instead of ShortName.
//
// When ListTitle = "alarms" and ShortName = "alarm", the frame title should
// start with "alarms(" after resources are loaded (format is "name(count)").
func TestQA_ListTitle_UsedInFrameTitle(t *testing.T) {
	td := resource.ResourceTypeDef{
		Name:      "CloudWatch Alarms",
		ShortName: "alarm",
		ListTitle: "alarms",
		Columns: []resource.Column{
			{Key: "alarm_name", Title: "Alarm Name", Width: 36},
		},
	}

	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "alarm",
		Resources: []resource.Resource{
			{
				ID:     "a1",
				Name:   "cpu-high",
				Status: "OK",
				Fields: map[string]string{"alarm_name": "cpu-high"},
			},
		},
	})

	title := m.FrameTitle()

	if !strings.HasPrefix(title, "alarms(") {
		t.Errorf(
			"FrameTitle() should use ListTitle as base when set: got %q, expected prefix %q; "+
				"ShortName=%q must NOT be used when ListTitle=%q is non-empty",
			title, "alarms(", "alarm", "alarms",
		)
	}

	if strings.HasPrefix(title, "alarm(") {
		t.Errorf(
			"FrameTitle() must NOT use ShortName %q when ListTitle %q is set: got %q",
			"alarm", "alarms", title,
		)
	}
}

// TestQA_ListTitle_FallsBackToShortName verifies that when ResourceTypeDef.ListTitle
// is empty (zero value), FrameTitle() falls back to ShortName as the base name.
//
// When ListTitle = "" and ShortName = "ec2", the frame title should start
// with "ec2(" after resources are loaded.
func TestQA_ListTitle_FallsBackToShortName(t *testing.T) {
	td := resource.ResourceTypeDef{
		Name:      "EC2 Instances",
		ShortName: "ec2",
		// ListTitle intentionally omitted (zero value "")
		Columns: []resource.Column{
			{Key: "instance_id", Title: "Instance ID", Width: 14},
		},
	}

	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{
				ID:     "i-abc123",
				Name:   "my-server",
				Status: "running",
				Fields: map[string]string{"instance_id": "i-abc123"},
			},
		},
	})

	title := m.FrameTitle()

	if !strings.HasPrefix(title, "ec2(") {
		t.Errorf(
			"FrameTitle() should fall back to ShortName when ListTitle is empty: got %q, expected prefix %q",
			title, "ec2(",
		)
	}
}
