package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/internal/views"
)

// ---------------------------------------------------------------------------
// T040 - Test JSON view rendering
// ---------------------------------------------------------------------------

func TestJSONViewModel_RendersJSONContent(t *testing.T) {
	jsonContent := `{"key": "value", "nested": {"a": 1}}`

	m := views.NewJSONView("EC2 Instance JSON", jsonContent)
	m.Width = 80
	m.Height = 30

	output := m.View()

	// Should contain the JSON content
	if !strings.Contains(output, "key") {
		t.Error("expected output to contain 'key'")
	}
	if !strings.Contains(output, "value") {
		t.Error("expected output to contain 'value'")
	}
	if !strings.Contains(output, "nested") {
		t.Error("expected output to contain 'nested'")
	}
}

func TestJSONViewModel_EmptyContent(t *testing.T) {
	m := views.NewJSONView("Empty JSON", "")
	m.Width = 80
	m.Height = 30

	output := m.View()

	if output == "" {
		t.Error("expected non-empty output even with empty content")
	}
	if !strings.Contains(output, "Empty JSON") {
		t.Error("expected output to contain the title 'Empty JSON'")
	}
}

func TestJSONViewModel_ScrollUpDown(t *testing.T) {
	jsonContent := `{
  "key1": "value1",
  "key2": "value2",
  "key3": "value3",
  "key4": "value4",
  "key5": "value5"
}`

	m := views.NewJSONView("Scroll Test", jsonContent)
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
