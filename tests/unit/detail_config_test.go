package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/internal/fieldpath"
	"github.com/k2m30/a9s/internal/views"
)

// ---------------------------------------------------------------------------
// Test struct definitions (local to test — no AWS SDK dependency)
// ---------------------------------------------------------------------------

type detailTestState struct {
	Name string `json:"name"`
	Code int32  `json:"code"`
}

type detailTestResource struct {
	InstanceID string           `json:"instanceId"`
	State      *detailTestState `json:"state"`
	FieldA     string           `json:"a"`
	FieldB     string           `json:"b"`
	FieldC     string           `json:"c"`
}

// ---------------------------------------------------------------------------
// T028 — Detail view renders configured fields IN ORDER (not alphabetical)
// ---------------------------------------------------------------------------

func TestConfigDetail_RendersFieldsInConfiguredOrder(t *testing.T) {
	obj := detailTestResource{
		FieldA: "value-a",
		FieldB: "value-b",
		FieldC: "value-c",
	}
	// Configure reverse-alphabetical order: c, a, b
	paths := []string{"c", "a", "b"}

	m := views.NewConfigDetailModel("Order Test", obj, paths)
	m.Width = 80
	m.Height = 30

	output := m.View()

	// Find the positions of each path label in the output
	posC := strings.Index(output, "c")
	posA := strings.Index(output, "a")
	posB := strings.Index(output, "b")

	if posC < 0 || posA < 0 || posB < 0 {
		t.Fatalf("expected all three fields in output, got:\n%s", output)
	}

	// Find positions using the value lines to avoid ambiguity with title text
	posValC := strings.Index(output, "value-c")
	posValA := strings.Index(output, "value-a")
	posValB := strings.Index(output, "value-b")

	if posValC < 0 || posValA < 0 || posValB < 0 {
		t.Fatalf("expected all three values in output, got:\n%s", output)
	}

	// C should appear before A, and A before B
	if posValC >= posValA {
		t.Errorf("expected value-c before value-a, but posC=%d posA=%d\noutput:\n%s", posValC, posValA, output)
	}
	if posValA >= posValB {
		t.Errorf("expected value-a before value-b, but posA=%d posB=%d\noutput:\n%s", posValA, posValB, output)
	}
}

// ---------------------------------------------------------------------------
// T029 — Detail view renders scalar path as key : value
// ---------------------------------------------------------------------------

func TestConfigDetail_ScalarRendersAsKeyValue(t *testing.T) {
	obj := detailTestResource{
		InstanceID: "i-123",
	}
	paths := []string{"instanceId"}

	m := views.NewConfigDetailModel("Scalar Test", obj, paths)
	m.Width = 80
	m.Height = 30

	output := m.View()

	// The output should contain instanceId and i-123 on the same line
	lines := strings.Split(output, "\n")
	found := false
	for _, line := range lines {
		if strings.Contains(line, "instanceId") && strings.Contains(line, "i-123") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a line with both 'instanceId' and 'i-123', got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// T030 — Detail view renders nested object as indented YAML subtree
// ---------------------------------------------------------------------------

func TestConfigDetail_NestedStructRendersAsYAMLSubtree(t *testing.T) {
	obj := detailTestResource{
		State: &detailTestState{Name: "running", Code: 16},
	}
	paths := []string{"state"}

	m := views.NewConfigDetailModel("Nested Test", obj, paths)
	m.Width = 80
	m.Height = 30

	output := m.View()

	// Verify the subtree header for "state" exists
	if !strings.Contains(output, "state") {
		t.Errorf("expected 'state' label in output, got:\n%s", output)
	}

	// The nested struct should be rendered as YAML with name and code fields
	subtree := fieldpath.ExtractSubtree(obj, "state")
	if !strings.Contains(subtree, "name") || !strings.Contains(subtree, "running") {
		t.Fatalf("ExtractSubtree sanity check failed: %q", subtree)
	}

	if !strings.Contains(output, "name") || !strings.Contains(output, "running") {
		t.Errorf("expected YAML output to contain 'name: running', got:\n%s", output)
	}
	if !strings.Contains(output, "code") || !strings.Contains(output, "16") {
		t.Errorf("expected YAML output to contain 'code: 16', got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// T031 — Detail view falls back to legacy behavior when no config detail paths
// ---------------------------------------------------------------------------

func TestConfigDetail_FallbackToLegacyWithEmptyPaths(t *testing.T) {
	data := map[string]string{
		"zebra": "z-value",
		"apple": "a-value",
		"mango": "m-value",
	}

	// Use old constructor with no detail paths
	m := views.NewDetailModel("Legacy Test", data)
	m.Width = 80
	m.Height = 30

	output := m.View()

	// Keys should be sorted alphabetically
	posApple := strings.Index(output, "apple")
	posMango := strings.Index(output, "mango")
	posZebra := strings.Index(output, "zebra")

	if posApple < 0 || posMango < 0 || posZebra < 0 {
		t.Fatalf("expected all three keys in output, got:\n%s", output)
	}

	if posApple >= posMango {
		t.Errorf("expected 'apple' before 'mango', got posApple=%d posMango=%d", posApple, posMango)
	}
	if posMango >= posZebra {
		t.Errorf("expected 'mango' before 'zebra', got posMango=%d posZebra=%d", posMango, posZebra)
	}
}

func TestConfigDetail_FallbackWhenDetailPathsNilAndRawStructNil(t *testing.T) {
	// Create a config detail model with nil struct and nil paths —
	// should behave like the legacy model (no panic, empty output).
	m := views.NewConfigDetailModel("Empty Config Test", nil, nil)
	m.Width = 80
	m.Height = 30

	output := m.View()

	if !strings.Contains(output, "Empty Config Test") {
		t.Errorf("expected title in output, got:\n%s", output)
	}
}
