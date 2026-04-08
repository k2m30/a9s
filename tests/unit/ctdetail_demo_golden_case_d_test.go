package unit_test

// ctdetail_demo_golden_case_d_test.go — golden snapshot test for ct-events detail view,
// Case D: kms:RotateKey AwsServiceEvent.
//
// Uses demo fixture "e-d4e5f6a7" from internal/demo/fixtures_monitoring.go.
//
// Generation:
//
//	UPDATE_GOLDEN=1 go test ./tests/unit -run TestCTDetailDemoGolden_CaseD -v
//
// Verification:
//
//	go test ./tests/unit -run TestCTDetailDemoGolden_CaseD -v

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/demo"
)

// TestCTDetailDemoGolden_CaseD renders the ct-events detail view for fixture
// "e-d4e5f6a7" (kms:RotateKey AwsServiceEvent) at size 180x40 and compares
// the ANSI-stripped output against a golden file.
//
// First run: generate the golden file with UPDATE_GOLDEN=1.
// Subsequent runs: fail if the output changes.
func TestCTDetailDemoGolden_CaseD(t *testing.T) {
	ensureNoColor(t)

	// Load the demo fixture for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Find fixture "e-d4e5f6a7" (Case D -- kms:RotateKey AwsServiceEvent).
	var caseDIdx int = -1
	for i, r := range resources {
		if r.ID == "e-d4e5f6a7" {
			caseDIdx = i
			break
		}
	}
	if caseDIdx == -1 {
		t.Fatal("demo fixture \"e-d4e5f6a7\" not found in ct-events fixtures")
	}
	res := resources[caseDIdx]

	// Build the detail model at the specified size.
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)
	m.SetSize(180, 40)

	// Render and strip ANSI codes for deterministic comparison.
	actual := stripAnsi(m.View())
	actual = strings.ReplaceAll(actual, "\r\n", "\n")

	goldenPath := filepath.Join("..", "testdata", "golden", "ctdetail_demo_case_d.txt")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(actual), 0o644); err != nil {
			t.Fatalf("write golden file: %v", err)
		}
		t.Logf("golden file written to %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden file %s: %v (run with UPDATE_GOLDEN=1 to create it)", goldenPath, err)
	}
	expectedStr := strings.ReplaceAll(string(expected), "\r\n", "\n")

	if expectedStr != actual {
		t.Fatalf("golden mismatch for ct-events detail Case D\n--- expected (golden) ---\n%s\n--- actual ---\n%s", expectedStr, actual)
	}
}
