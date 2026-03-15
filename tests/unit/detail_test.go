package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/internal/views"
)

// ---------------------------------------------------------------------------
// T039 - Test detail view rendering
// ---------------------------------------------------------------------------

func TestDetailModel_RendersFiveEntries(t *testing.T) {
	data := map[string]string{
		"instance_id": "i-0001",
		"name":        "web-server",
		"state":       "running",
		"type":        "t3.micro",
		"private_ip":  "10.0.0.1",
	}

	m := views.NewDetailModel("EC2 Instance Detail", data)
	m.Width = 80
	m.Height = 30

	output := m.View()

	// Verify all keys and values are present
	for k, v := range data {
		if !strings.Contains(output, k) {
			t.Errorf("expected output to contain key %q", k)
		}
		if !strings.Contains(output, v) {
			t.Errorf("expected output to contain value %q", v)
		}
	}
}

func TestDetailModel_EmptyDetailData(t *testing.T) {
	data := map[string]string{}

	m := views.NewDetailModel("Empty Detail", data)
	m.Width = 80
	m.Height = 30

	output := m.View()

	// Should still render without panic
	if output == "" {
		t.Error("expected non-empty output even with empty data")
	}
	// Should contain the title
	if !strings.Contains(output, "Empty Detail") {
		t.Error("expected output to contain the title 'Empty Detail'")
	}
}

func TestDetailModel_KeysSortedAlphabetically(t *testing.T) {
	data := map[string]string{
		"zebra":    "z-value",
		"apple":    "a-value",
		"mango":    "m-value",
	}

	m := views.NewDetailModel("Sorted Test", data)

	// Keys should be sorted alphabetically
	if len(m.Keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(m.Keys))
	}
	if m.Keys[0] != "apple" {
		t.Errorf("expected first key to be 'apple', got %q", m.Keys[0])
	}
	if m.Keys[1] != "mango" {
		t.Errorf("expected second key to be 'mango', got %q", m.Keys[1])
	}
	if m.Keys[2] != "zebra" {
		t.Errorf("expected third key to be 'zebra', got %q", m.Keys[2])
	}
}

func TestDetailModel_ScrollUpDown(t *testing.T) {
	data := map[string]string{
		"key1": "val1",
		"key2": "val2",
		"key3": "val3",
		"key4": "val4",
		"key5": "val5",
	}

	m := views.NewDetailModel("Scroll Test", data)
	m.Width = 80
	m.Height = 5

	if m.Offset != 0 {
		t.Errorf("expected initial offset 0, got %d", m.Offset)
	}

	m.ScrollDown()
	if m.Offset != 1 {
		t.Errorf("expected offset 1 after ScrollDown, got %d", m.Offset)
	}

	m.ScrollUp()
	if m.Offset != 0 {
		t.Errorf("expected offset 0 after ScrollUp, got %d", m.Offset)
	}

	// ScrollUp at top should stay at 0
	m.ScrollUp()
	if m.Offset != 0 {
		t.Errorf("expected offset 0 after ScrollUp at top, got %d", m.Offset)
	}
}
