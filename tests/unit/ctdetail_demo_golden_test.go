package unit_test

// ctdetail_demo_golden_test.go — golden snapshot test for ct-events detail view.
//
// Case A: Karpenter ec2:DescribeInstances (R, ct-info)
// Uses demo fixture "e-a1b2c3d4" from internal/demo/fixtures_monitoring.go.
//
// Case B: SSO Console TerminateInstances with MFA
// Uses demo fixture "e-b2c3d4e5" from internal/demo/fixtures_monitoring.go.
//
// Case C: IAMUser s3:PutObject AccessDenied, ERROR hoisted
// Uses demo fixture "e-c3d4e5f6" from internal/demo/fixtures_monitoring.go.
//
// Case E: Root PutBucketPolicy (W, ct-warning)
// Uses demo fixture "e-e5f6a7b8" from internal/demo/fixtures_monitoring.go.
//
// Case F: IRSA GetObject WebIdentityUser
// Uses demo fixture "e-f6a7b8c9" from internal/demo/fixtures_monitoring.go.
//
// Case G: Cross-account PutObject
// Uses demo fixture "e-a7b8c9d0" from internal/demo/fixtures_monitoring.go.
//
// Case H: Insight ApiCallRateInsight, no ACTOR
// Uses demo fixture "e-b8c9d0e1" from internal/demo/fixtures_monitoring.go.
//
// Case I: NetworkActivity VPCE deny
// Uses demo fixture "e-c9d0e1f2" from internal/demo/fixtures_monitoring.go.
//
// Generation:
//
//	UPDATE_GOLDEN=1 go test ./tests/unit -run TestCTDetailDemoGolden_CaseA -v
//	UPDATE_GOLDEN=1 go test ./tests/unit -run TestCTDetailDemoGolden_CaseB -v
//	UPDATE_GOLDEN=1 go test ./tests/unit -run TestCTDetailDemoGolden_CaseC -v
//	UPDATE_GOLDEN=1 go test ./tests/unit -run TestCTDetailDemoGolden_CaseE -v
//	UPDATE_GOLDEN=1 go test ./tests/unit -run TestCTDetailDemoGolden_CaseF -v
//	UPDATE_GOLDEN=1 go test ./tests/unit -run TestCTDetailDemoGolden_CaseG -v
//	UPDATE_GOLDEN=1 go test ./tests/unit -run TestCTDetailDemoGolden_CaseH -v
//	UPDATE_GOLDEN=1 go test ./tests/unit -run TestCTDetailDemoGolden_CaseI -v
//
// Verification:
//
//	go test ./tests/unit -run TestCTDetailDemoGolden_CaseA -v
//	go test ./tests/unit -run TestCTDetailDemoGolden_CaseB -v
//	go test ./tests/unit -run TestCTDetailDemoGolden_CaseC -v
//	go test ./tests/unit -run TestCTDetailDemoGolden_CaseE -v
//	go test ./tests/unit -run TestCTDetailDemoGolden_CaseF -v
//	go test ./tests/unit -run TestCTDetailDemoGolden_CaseG -v
//	go test ./tests/unit -run TestCTDetailDemoGolden_CaseH -v
//	go test ./tests/unit -run TestCTDetailDemoGolden_CaseI -v

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/demo"
)

// TestCTDetailDemoGolden_CaseA renders the ct-events detail view for fixture
// "e-a1b2c3d4" (Karpenter DescribeInstances, ct-info) at size 180×40 and
// compares the ANSI-stripped output against a golden file.
//
// First run: generate the golden file with UPDATE_GOLDEN=1.
// Subsequent runs: fail if the output changes.
func TestCTDetailDemoGolden_CaseA(t *testing.T) {
	ensureNoColor(t)

	// Load the demo fixture for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Find fixture "e-a1b2c3d4" (Case A — Karpenter DescribeInstances).
	var caseAIdx int = -1
	for i, r := range resources {
		if r.ID == "e-a1b2c3d4" {
			caseAIdx = i
			break
		}
	}
	if caseAIdx == -1 {
		t.Fatal("demo fixture \"e-a1b2c3d4\" not found in ct-events fixtures")
	}
	res := resources[caseAIdx]

	// Build the detail model at the specified size.
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)
	m.SetSize(180, 40)

	// Render and strip ANSI codes for deterministic comparison.
	actual := stripAnsi(m.View())
	actual = strings.ReplaceAll(actual, "\r\n", "\n")

	goldenPath := filepath.Join("..", "testdata", "golden", "ctdetail_demo_case_a.txt")

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
		t.Fatalf("golden mismatch for ct-events detail Case A\n--- expected (golden) ---\n%s\n--- actual ---\n%s", expectedStr, actual)
	}
}

// TestCTDetailDemoGolden_CaseB renders the ct-events detail view for fixture
// "e-b2c3d4e5" (SSO Console TerminateInstances with MFA) at size 180×40 and
// compares the ANSI-stripped output against a golden file.
//
// First run: generate the golden file with UPDATE_GOLDEN=1.
// Subsequent runs: fail if the output changes.
func TestCTDetailDemoGolden_CaseB(t *testing.T) {
	ensureNoColor(t)

	// Load the demo fixture for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Find fixture "e-b2c3d4e5" (Case B — SSO Console TerminateInstances with MFA).
	var caseBIdx int = -1
	for i, r := range resources {
		if r.ID == "e-b2c3d4e5" {
			caseBIdx = i
			break
		}
	}
	if caseBIdx == -1 {
		t.Fatal("demo fixture \"e-b2c3d4e5\" not found in ct-events fixtures")
	}
	res := resources[caseBIdx]

	// Build the detail model at the specified size.
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)
	m.SetSize(180, 40)

	// Render and strip ANSI codes for deterministic comparison.
	actual := stripAnsi(m.View())
	actual = strings.ReplaceAll(actual, "\r\n", "\n")

	goldenPath := filepath.Join("..", "testdata", "golden", "ctdetail_demo_case_b.txt")

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
		t.Fatalf("golden mismatch for ct-events detail Case B\n--- expected (golden) ---\n%s\n--- actual ---\n%s", expectedStr, actual)
	}
}

// TestCTDetailDemoGolden_CaseE renders the ct-events detail view for fixture
// "e-e5f6a7b8" (Root PutBucketPolicy, ct-warning) at size 180×40 and
// compares the ANSI-stripped output against a golden file.
//
// First run: generate the golden file with UPDATE_GOLDEN=1.
// Subsequent runs: fail if the output changes.
func TestCTDetailDemoGolden_CaseE(t *testing.T) {
	ensureNoColor(t)

	// Load the demo fixture for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Find fixture "e-e5f6a7b8" (Case E — Root PutBucketPolicy).
	var caseEIdx int = -1
	for i, r := range resources {
		if r.ID == "e-e5f6a7b8" {
			caseEIdx = i
			break
		}
	}
	if caseEIdx == -1 {
		t.Fatal("demo fixture \"e-e5f6a7b8\" not found in ct-events fixtures")
	}
	res := resources[caseEIdx]

	// Build the detail model at the specified size.
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)
	m.SetSize(180, 40)

	// Render and strip ANSI codes for deterministic comparison.
	actual := stripAnsi(m.View())
	actual = strings.ReplaceAll(actual, "\r\n", "\n")

	goldenPath := filepath.Join("..", "testdata", "golden", "ctdetail_demo_case_e.txt")

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
		t.Fatalf("golden mismatch for ct-events detail Case E\n--- expected (golden) ---\n%s\n--- actual ---\n%s", expectedStr, actual)
	}
}

// TestCTDetailDemoGolden_CaseF renders the ct-events detail view for fixture
// "e-f6a7b8c9" (IRSA GetObject WebIdentityUser) at size 180×40 and compares
// the ANSI-stripped output against a golden file.
//
// First run: generate the golden file with UPDATE_GOLDEN=1.
// Subsequent runs: fail if the output changes.
func TestCTDetailDemoGolden_CaseF(t *testing.T) {
	ensureNoColor(t)

	// Load the demo fixture for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Find fixture "e-f6a7b8c9" (Case F — IRSA GetObject WebIdentityUser).
	var caseFIdx int = -1
	for i, r := range resources {
		if r.ID == "e-f6a7b8c9" {
			caseFIdx = i
			break
		}
	}
	if caseFIdx == -1 {
		t.Fatal("demo fixture \"e-f6a7b8c9\" not found in ct-events fixtures")
	}
	res := resources[caseFIdx]

	// Build the detail model at the specified size.
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)
	m.SetSize(180, 40)

	// Render and strip ANSI codes for deterministic comparison.
	actual := stripAnsi(m.View())
	actual = strings.ReplaceAll(actual, "\r\n", "\n")

	goldenPath := filepath.Join("..", "testdata", "golden", "ctdetail_demo_case_f.txt")

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
		t.Fatalf("golden mismatch for ct-events detail Case F\n--- expected (golden) ---\n%s\n--- actual ---\n%s", expectedStr, actual)
	}
}

// TestCTDetailDemoGolden_CaseG renders the ct-events detail view for fixture
// "e-a7b8c9d0" (cross-account PutObject) at size 180×40 and compares the
// ANSI-stripped output against a golden file.
//
// First run: generate the golden file with UPDATE_GOLDEN=1.
// Subsequent runs: fail if the output changes.
func TestCTDetailDemoGolden_CaseG(t *testing.T) {
	ensureNoColor(t)

	// Load the demo fixture for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Find fixture "e-a7b8c9d0" (Case G — cross-account PutObject).
	var caseGIdx int = -1
	for i, r := range resources {
		if r.ID == "e-a7b8c9d0" {
			caseGIdx = i
			break
		}
	}
	if caseGIdx == -1 {
		t.Fatal("demo fixture \"e-a7b8c9d0\" not found in ct-events fixtures")
	}
	res := resources[caseGIdx]

	// Build the detail model at the specified size.
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)
	m.SetSize(180, 40)

	// Render and strip ANSI codes for deterministic comparison.
	actual := stripAnsi(m.View())
	actual = strings.ReplaceAll(actual, "\r\n", "\n")

	goldenPath := filepath.Join("..", "testdata", "golden", "ctdetail_demo_case_g.txt")

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
		t.Fatalf("golden mismatch for ct-events detail Case G\n--- expected (golden) ---\n%s\n--- actual ---\n%s", expectedStr, actual)
	}
}

// TestCTDetailDemoGolden_CaseC renders the ct-events detail view for fixture
// "e-c3d4e5f6" (IAMUser s3:PutObject AccessDenied, ERROR hoisted) at size 180×40
// and compares the ANSI-stripped output against a golden file.
//
// First run: generate the golden file with UPDATE_GOLDEN=1.
// Subsequent runs: fail if the output changes.
func TestCTDetailDemoGolden_CaseC(t *testing.T) {
	ensureNoColor(t)

	// Load the demo fixture for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Find fixture "e-c3d4e5f6" (Case C — IAMUser PutObject AccessDenied).
	var caseCIdx int = -1
	for i, r := range resources {
		if r.ID == "e-c3d4e5f6" {
			caseCIdx = i
			break
		}
	}
	if caseCIdx == -1 {
		t.Fatal("demo fixture \"e-c3d4e5f6\" not found in ct-events fixtures")
	}
	res := resources[caseCIdx]

	// Build the detail model at the specified size.
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)
	m.SetSize(180, 40)

	// Render and strip ANSI codes for deterministic comparison.
	actual := stripAnsi(m.View())
	actual = strings.ReplaceAll(actual, "\r\n", "\n")

	goldenPath := filepath.Join("..", "testdata", "golden", "ctdetail_demo_case_c.txt")

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
		t.Fatalf("golden mismatch for ct-events detail Case C\n--- expected (golden) ---\n%s\n--- actual ---\n%s", expectedStr, actual)
	}
}

// TestCTDetailDemoGolden_CaseH renders the ct-events detail view for fixture
// "e-b8c9d0e1" (Insight ApiCallRateInsight, no ACTOR, ct-info) at size 180×40 and
// compares the ANSI-stripped output against a golden file.
//
// This case exercises the Insight event category path in the detail renderer,
// where no userIdentity/sessionIssuer is present and the ACTOR section must be
// omitted or rendered as empty.
//
// First run: generate the golden file with UPDATE_GOLDEN=1.
// Subsequent runs: fail if the output changes.
func TestCTDetailDemoGolden_CaseH(t *testing.T) {
	ensureNoColor(t)

	// Load the demo fixture for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Find fixture "e-b8c9d0e1" (Case H — Insight ApiCallRateInsight).
	var caseHIdx int = -1
	for i, r := range resources {
		if r.ID == "e-b8c9d0e1" {
			caseHIdx = i
			break
		}
	}
	if caseHIdx == -1 {
		t.Fatal("demo fixture \"e-b8c9d0e1\" not found in ct-events fixtures")
	}
	res := resources[caseHIdx]

	// Build the detail model at the specified size.
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)
	m.SetSize(180, 40)

	// Render and strip ANSI codes for deterministic comparison.
	actual := stripAnsi(m.View())
	actual = strings.ReplaceAll(actual, "\r\n", "\n")

	goldenPath := filepath.Join("..", "testdata", "golden", "ctdetail_demo_case_h.txt")

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
		t.Fatalf("golden mismatch for ct-events detail Case H\n--- expected (golden) ---\n%s\n--- actual ---\n%s", expectedStr, actual)
	}
}

// TestCTDetailDemoGolden_CaseI renders the ct-events detail view for fixture
// "e-c9d0e1f2" (NetworkActivity VPCE deny) at size 180×40 and compares the
// ANSI-stripped output against a golden file.
//
// First run: generate the golden file with UPDATE_GOLDEN=1.
// Subsequent runs: fail if the output changes.
func TestCTDetailDemoGolden_CaseI(t *testing.T) {
	ensureNoColor(t)

	// Load the demo fixture for ct-events.
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Find fixture "e-c9d0e1f2" (Case I — NetworkActivity VPCE deny).
	var caseIIdx int = -1
	for i, r := range resources {
		if r.ID == "e-c9d0e1f2" {
			caseIIdx = i
			break
		}
	}
	if caseIIdx == -1 {
		t.Fatal("demo fixture \"e-c9d0e1f2\" not found in ct-events fixtures")
	}
	res := resources[caseIIdx]

	// Build the detail model at the specified size.
	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)
	m.SetSize(180, 40)

	// Render and strip ANSI codes for deterministic comparison.
	actual := stripAnsi(m.View())
	actual = strings.ReplaceAll(actual, "\r\n", "\n")

	goldenPath := filepath.Join("..", "testdata", "golden", "ctdetail_demo_case_i.txt")

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
		t.Fatalf("golden mismatch for ct-events detail Case I\n--- expected (golden) ---\n%s\n--- actual ---\n%s", expectedStr, actual)
	}
}
