package unit

// qa_resource_color_test.go — color refactor invariants (Stream 4).
//
// Invariants tested here:
//   #1  No presentation-layer code reads resource-type-specific fields.
//   #3  EC2 impaired/initializing promotion via Color func.
//   #4  EC2 CellDecorators["state"] parity.
//   #5  nil CellDecorators map does not panic on map read.
//   #6  ct-events ExcludeFromIssueBadge is set, and Status=="ct-danger" → ColorBroken.
//   #7  Every registered type has a non-nil Color function.
//
// These tests will fail until Stream 3 (consumer refactor) completes — that is
// expected TDD behavior. They MUST compile cleanly against Stream 1 + Stream 2 output.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ─────────────────────────────────────────────────────────────────────────────
// Invariant #1 — No presentation-layer code reads EC2-specific field names.
//
// Post-refactor, the strings "system_status" and "instance_status" must not
// appear anywhere in internal/tui/ (excluding *_test.go files). The EC2
// CellDecorators func lives in internal/resource/, which is the correct owner.
// ─────────────────────────────────────────────────────────────────────────────

func TestColorRefactor_NoEC2FieldsInPresentationLayer(t *testing.T) {
	// These files are the generic row-rendering and resource-listing code that must
	// not contain EC2-specific field names. detail_fields.go is intentionally excluded:
	// it injects per-type sub-fields for the EC2 status-checks detail section, which
	// reads system_status/instance_status from r.Fields. Relocating that injection to
	// a data-driven mechanism is a separate future refactor (not part of this change).
	targetFiles := []string{
		filepath.Join(projectRoot(t), "internal", "tui", "views", "table_render.go"),
		filepath.Join(projectRoot(t), "internal", "tui", "views", "resourcelist.go"),
		filepath.Join(projectRoot(t), "internal", "tui", "views", "resourcelist_helpers.go"),
		filepath.Join(projectRoot(t), "internal", "tui", "app_fetchers.go"),
	}
	forbidden := []string{"system_status", "instance_status"}

	var hits []string
	for _, path := range targetFiles {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue // file doesn't exist yet — skip
			}
			t.Fatalf("ReadFile(%q): %v", path, readErr)
		}
		content := string(data)
		for _, needle := range forbidden {
			if strings.Contains(content, needle) {
				rel, _ := filepath.Rel(projectRoot(t), path)
				hits = append(hits, rel+": contains "+needle)
			}
		}
	}
	for _, h := range hits {
		t.Errorf("invariant #1 violated — EC2-specific field in presentation layer: %s", h)
	}
}

// projectRoot returns the module root directory (the directory containing go.mod).
func projectRoot(t *testing.T) string {
	t.Helper()
	// Walk upward from the test file's working directory until go.mod is found.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod — cannot determine project root")
		}
		dir = parent
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Invariant #3 — EC2 Color func: impaired/initializing promotion.
// ─────────────────────────────────────────────────────────────────────────────

func TestColorRefactor_EC2Color_ImpairedPromotion(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 resource type not found in registry")
	}
	if td.Color == nil {
		t.Fatal("ec2 Color func is nil — invariant #7 violated")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "system_status=impaired → ColorBroken",
			fields: map[string]string{"state": "running", "system_status": "impaired"},
			want:   resource.ColorBroken,
		},
		{
			name:   "instance_status=impaired → ColorBroken",
			fields: map[string]string{"state": "running", "instance_status": "impaired"},
			want:   resource.ColorBroken,
		},
		{
			name:   "instance_status=initializing → ColorWarning",
			fields: map[string]string{"state": "running", "instance_status": "initializing"},
			want:   resource.ColorWarning,
		},
		{
			name:   "system_status=initializing → ColorWarning",
			fields: map[string]string{"state": "running", "system_status": "initializing"},
			want:   resource.ColorWarning,
		},
		{
			name:   "both ok → ColorHealthy",
			fields: map[string]string{"state": "running", "system_status": "ok", "instance_status": "ok"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "state=running, no status fields → ColorHealthy",
			fields: map[string]string{"state": "running"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "state=stopped → ColorBroken",
			fields: map[string]string{"state": "stopped"},
			want:   resource.ColorBroken,
		},
		{
			name:   "state=stopping → ColorBroken",
			fields: map[string]string{"state": "stopping"},
			want:   resource.ColorBroken,
		},
		{
			name:   "state=pending → ColorWarning",
			fields: map[string]string{"state": "pending"},
			want:   resource.ColorWarning,
		},
		{
			name:   "state=terminated → ColorDim",
			fields: map[string]string{"state": "terminated"},
			want:   resource.ColorDim,
		},
		{
			name:   "state=shutting-down → ColorDim",
			fields: map[string]string{"state": "shutting-down"},
			want:   resource.ColorDim,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r := resource.Resource{
				ID:     "i-0abc1234567",
				Name:   "test-instance",
				Status: tc.fields["state"],
				Fields: tc.fields,
			}
			got := td.Color(r)
			if got != tc.want {
				t.Errorf("ec2.Color(%v) = %v, want %v", tc.fields, got, tc.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Invariant #4 — EC2 CellDecorators["state"] parity.
// ─────────────────────────────────────────────────────────────────────────────

func TestColorRefactor_EC2CellDecorator_StateParity(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 resource type not found in registry")
	}
	dec, ok := td.CellDecorators["state"]
	if !ok || dec == nil {
		t.Fatal(`ec2.CellDecorators["state"] missing or nil — invariant #4 violated`)
	}

	cases := []struct {
		name   string
		fields map[string]string
		value  string
		want   string
	}{
		{
			name:   "system_status=impaired → ! running",
			fields: map[string]string{"system_status": "impaired"},
			value:  "running",
			want:   "! running",
		},
		{
			name:   "instance_status=impaired → ! running",
			fields: map[string]string{"instance_status": "impaired"},
			value:  "running",
			want:   "! running",
		},
		{
			name:   "instance_status=initializing → ~ running",
			fields: map[string]string{"instance_status": "initializing"},
			value:  "running",
			want:   "~ running",
		},
		{
			name:   "system_status=initializing → ~ running",
			fields: map[string]string{"system_status": "initializing"},
			value:  "running",
			want:   "~ running",
		},
		{
			name:   "both ok → running unchanged",
			fields: map[string]string{"system_status": "ok", "instance_status": "ok"},
			value:  "running",
			want:   "running",
		},
		{
			name:   "no status fields → running unchanged",
			fields: map[string]string{},
			value:  "running",
			want:   "running",
		},
		{
			// The decorator only fires when the cell value is "running".
			// For non-running values, the prefix is not added regardless of
			// system/instance status — the cell value is returned unchanged.
			name:   "state=stopped → value unchanged (decorator only prefixes running)",
			fields: map[string]string{"system_status": "impaired"},
			value:  "stopped",
			want:   "stopped",
		},
		{
			name:   "system_status=impaired has priority over initializing",
			fields: map[string]string{"system_status": "impaired", "instance_status": "initializing"},
			value:  "running",
			want:   "! running",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r := resource.Resource{
				ID:     "i-0abc1234567",
				Name:   "test-instance",
				Status: "running",
				Fields: tc.fields,
			}
			got := dec(r, tc.value)
			if got != tc.want {
				t.Errorf("ec2.CellDecorators[\"state\"]({fields=%v}, %q) = %q, want %q",
					tc.fields, tc.value, got, tc.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Invariant #5 — nil CellDecorators map does not panic.
//
// Go's zero-value map read returns the zero value ("", false) — no explicit nil
// guard is needed at the dispatch site. This test confirms that for all types
// that don't declare decorators, accessing td.CellDecorators["any_key"] is safe.
// ─────────────────────────────────────────────────────────────────────────────

func TestColorRefactor_NilCellDecorators_Safe(t *testing.T) {
	for _, td := range resource.AllResourceTypes() {
		td := td
		t.Run(td.ShortName, func(t *testing.T) {
			if td.CellDecorators == nil {
				// Go nil-map read: must not panic, must return nil func.
				got := td.CellDecorators["any_key"]
				if got != nil {
					t.Errorf("%s: nil CellDecorators map returned non-nil for key \"any_key\" — unexpected",
						td.ShortName)
				}
				// Double-check with a realistic column key.
				got2 := td.CellDecorators["state"]
				if got2 != nil {
					t.Errorf("%s: nil CellDecorators map returned non-nil for key \"state\" — unexpected",
						td.ShortName)
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Invariant #6 — ct-events ExcludeFromIssueBadge + Color behavior.
// ─────────────────────────────────────────────────────────────────────────────

func TestColorRefactor_CtEvents_ExcludeFromIssueBadge(t *testing.T) {
	td := resource.FindResourceType("ct-events")
	if td == nil {
		t.Fatal("ct-events resource type not found in registry")
	}

	// ExcludeFromIssueBadge must be true.
	if !td.ExcludeFromIssueBadge {
		t.Error("ct-events: ExcludeFromIssueBadge must be true — event severity != resource health")
	}

	// Color func must be present.
	if td.Color == nil {
		t.Fatal("ct-events: Color func is nil — invariant #7 violated")
	}

	colorCases := []struct {
		status string
		want   resource.Color
	}{
		{"ct-danger", resource.ColorBroken},
		{"ct-attention", resource.ColorWarning},
		{"ct-info", resource.ColorDim},
		{"", resource.ColorDim},
	}

	for _, tc := range colorCases {
		tc := tc
		t.Run("status="+tc.status, func(t *testing.T) {
			r := resource.Resource{
				ID:     "evt-0001",
				Name:   "PutObject",
				Status: tc.status,
			}
			got := td.Color(r)
			if got != tc.want {
				t.Errorf("ct-events.Color({Status=%q}) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}

	// Specifically: ct-danger → ColorBroken contributes to ctrl+z visibility
	// (IsIssue == true) but ExcludeFromIssueBadge keeps it out of the badge.
	dangerR := resource.Resource{ID: "evt-0002", Name: "DeleteBucket", Status: "ct-danger"}
	dangerColor := td.Color(dangerR)
	if !dangerColor.IsIssue() {
		t.Errorf("ct-events ct-danger row: Color.IsIssue() must be true (visible after ctrl+z); got Color=%v", dangerColor)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Invariant #7 — Every registered type has a non-nil Color function.
// ─────────────────────────────────────────────────────────────────────────────

func TestColorRefactor_AllTypes_NonNilColorFunc(t *testing.T) {
	for _, td := range resource.AllResourceTypes() {
		td := td
		t.Run(td.ShortName, func(t *testing.T) {
			if td.Color == nil {
				t.Errorf("%s (%q): Color func is nil — REQUIRED for all registered types", td.ShortName, td.Name)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Additional: IsIssue semantics of Color constants.
// ─────────────────────────────────────────────────────────────────────────────

func TestColor_IsIssue(t *testing.T) {
	cases := []struct {
		color resource.Color
		want  bool
	}{
		{resource.ColorHealthy, false},
		{resource.ColorWarning, true},
		{resource.ColorBroken, true},
		{resource.ColorDim, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run("", func(t *testing.T) {
			if got := tc.color.IsIssue(); got != tc.want {
				t.Errorf("Color(%d).IsIssue() = %v, want %v", tc.color, got, tc.want)
			}
		})
	}
}
