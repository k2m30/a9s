package unit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

func snapshotEC2RelatedView(t *testing.T) string {
	t.Helper()

	d, cleanup := ec2StoryDetail(t, 120, 30, true)
	t.Cleanup(cleanup)

	// Deterministic state: left column selected row + related counts loaded.
	d = deliverRelatedResult(d, "tg", 1)
	d = deliverRelatedResult(d, "asg", 2)
	d = deliverRelatedResult(d, "alarm", 0)
	d = deliverRelatedResult(d, "cfn", 0)

	// Focus right column so the highlighted row is part of the snapshot contract.
	d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	return d.View()
}

func TestGolden_EC2RelatedView_Text(t *testing.T) {
	ensureNoColor(t)

	actual := stripAnsi(snapshotEC2RelatedView(t))
	actual = strings.ReplaceAll(actual, "\r\n", "\n")

	goldenPath := filepath.Join("..", "testdata", "golden", "ec2_related_view", "view.golden.txt")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(actual), 0o644); err != nil {
			t.Fatalf("write golden file: %v", err)
		}
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden file %s: %v (run with UPDATE_GOLDEN=1 to create it)", goldenPath, err)
	}
	expectedStr := strings.ReplaceAll(string(expected), "\r\n", "\n")

	if expectedStr != actual {
		t.Fatalf("golden mismatch for EC2 related view\n--- expected (golden) ---\n%s\n--- actual ---\n%s", expectedStr, actual)
	}
}

func TestGolden_EC2RelatedView_ANSI(t *testing.T) {
	// Explicitly force color-enabled style init for ANSI snapshot.
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	actual := snapshotEC2RelatedView(t)
	actual = strings.ReplaceAll(actual, "\r\n", "\n")

	if !strings.Contains(actual, "\x1b[") {
		t.Fatalf("ANSI golden requires style escape sequences, but none were present")
	}

	goldenPath := filepath.Join("..", "testdata", "golden", "ec2_related_view", "view.ansi.golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(actual), 0o644); err != nil {
			t.Fatalf("write golden file: %v", err)
		}
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden file %s: %v (run with UPDATE_GOLDEN=1 to create it)", goldenPath, err)
	}
	expectedStr := strings.ReplaceAll(string(expected), "\r\n", "\n")

	if expectedStr != actual {
		t.Fatalf("ANSI golden mismatch for EC2 related view\n--- expected (golden) ---\n%s\n--- actual ---\n%s", expectedStr, actual)
	}
}
