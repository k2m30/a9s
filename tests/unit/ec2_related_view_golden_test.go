package unit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// snapshotEC2RelatedView renders the live RenderDetail seam for an EC2
// resource with 4 related rows (mixed non-zero/zero counts) and the right
// column focused — the live replacement for the retired
// views.NewDetail(...).Update(RelatedCheckResult...).Update(KeyTab).View()
// chain (DetailModel.Update/View are dead; see
// specs/022-codebase-cleanup/wave3-map-detail.md).
func snapshotEC2RelatedView(t *testing.T) string {
	t.Helper()

	res := ec2StoryResource()
	c := newDetailController(t, res, "ec2")
	// InitDetailRelatedRows is what sets ds.RelatedVisible=true (mirrors real
	// navigation, core/app/navigate.go) — without it, ActionToggleFocus below
	// early-returns as a no-op (core/app/detail_cursor.go's ActionToggleFocus
	// case gates on ds.RelatedVisible) and the snapshot never actually shows
	// the right column focused. Must run before ApplyDetailRelated, since
	// InitDetailRelatedRows no-ops once RelatedRows is non-empty.
	c.InitDetailRelatedRows("ec2")
	c.ApplyDetailRelated([]app.DetailRelatedRow{
		{TargetType: "tg", DisplayName: "Target Groups", Count: 1},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Count: 2},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Count: 0},
		{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Count: 0},
	})
	// Focus right column so the highlighted row is part of the snapshot contract.
	c.Apply(app.Action{Kind: app.ActionToggleFocus})

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}
	// Assert the toggle actually landed on the right column before trusting
	// the golden byte-compare to catch a regression here: DetailBody.RelatedFocused
	// is what RenderDetail reads to pick the RowSelected style + accent column
	// separator (internal/tui/views/detail_helpers.go), and RelatedCursor==0 is
	// where detailSkipToDrillable parks the cursor on the first drillable row
	// ("tg", Count=1) starting from cursor 0. A no-op ActionToggleFocus (e.g. from
	// a missing RelatedVisible seed) would leave RelatedFocused false here.
	if !body.RelatedFocused {
		t.Fatal("expected DetailBody.RelatedFocused to be true after ActionToggleFocus — right column focus toggle was a no-op")
	}
	if body.RelatedCursor != 0 {
		t.Fatalf("expected RelatedCursor 0 (first drillable row) after ActionToggleFocus, got %d", body.RelatedCursor)
	}
	vp := viewport.New(viewport.WithWidth(120), viewport.WithHeight(30))
	m := views.NewTransientDetail(120, 30, vp)
	return m.RenderDetail(*body)
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
	tuitest.ForceColor(t)

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
