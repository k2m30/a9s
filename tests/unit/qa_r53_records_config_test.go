package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
)

// ===========================================================================
// GAP 2: R53 Records Config Defaults (mirrors s3_objects config tests)
// ===========================================================================

func TestConfigDefaultViewDef_R53Records(t *testing.T) {
	vd := config.DefaultViewDef("r53_records")
	if len(vd.List) != 4 {
		t.Fatalf("expected 4 list columns for r53_records default, got %d", len(vd.List))
	}

	wantCols := []struct {
		title string
		width int
	}{
		{"Name", 40},
		{"Type", 8},
		{"TTL", 8},
		{"Values", 50},
	}
	for i, want := range wantCols {
		got := vd.List[i]
		if got.Title != want.title || got.Width != want.width {
			t.Errorf("r53_records default List[%d] = {%q, %d}, want {%q, %d}",
				i, got.Title, got.Width,
				want.title, want.width)
		}
	}
}

func TestConfigDefaultViewDef_R53Records_DetailPaths(t *testing.T) {
	vd := config.DefaultViewDef("r53_records")
	if len(vd.Detail) == 0 {
		t.Fatal("r53_records default should have non-empty Detail paths")
	}

	// Verify key detail fields are present
	wantPaths := []string{"Name", "Type", "TTL", "ResourceRecords", "AliasTarget"}
	for _, want := range wantPaths {
		found := false
		for _, p := range vd.Detail {
			if p == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("r53_records default Detail missing path %q", want)
		}
	}
}

func TestConfigYAMLParsing_R53Records(t *testing.T) {
	paths := []string{testdataPath("views_r53_records.yaml")}
	cfg, err := config.LoadFrom(paths)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	r53rec, ok := cfg.Views["r53_records"]
	if !ok {
		t.Fatal("missing r53_records view definition in parsed YAML")
	}
	if len(r53rec.List) != 4 {
		t.Fatalf("r53_records: expected 4 list columns, got %d", len(r53rec.List))
	}

	// The YAML file uses custom widths different from defaults
	wantCols := []struct {
		title string
		width int
	}{
		{"Name", 50},
		{"Type", 10},
		{"TTL", 10},
		{"Values", 60},
	}
	for i, want := range wantCols {
		got := r53rec.List[i]
		if got.Title != want.title || got.Width != want.width {
			t.Errorf("r53_records YAML List[%d] = {%q, %d}, want {%q, %d}",
				i, got.Title, got.Width,
				want.title, want.width)
		}
	}
}

func TestGetViewDef_R53Records_NilConfig(t *testing.T) {
	// With nil config, should fall back to defaults
	vd := config.GetViewDef(nil, "r53_records")
	if len(vd.List) != 4 {
		t.Fatalf("expected 4 default r53_records columns with nil config, got %d", len(vd.List))
	}
	if vd.List[0].Title != "Name" {
		t.Errorf("expected first column title 'Name', got %q", vd.List[0].Title)
	}
}

func TestGetViewDef_R53Records_FromConfig(t *testing.T) {
	paths := []string{testdataPath("views_r53_records.yaml")}
	cfg, err := config.LoadFrom(paths)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}

	vd := config.GetViewDef(cfg, "r53_records")
	if len(vd.List) != 4 {
		t.Fatalf("expected 4 r53_records columns from config, got %d", len(vd.List))
	}
	// Config has width 50 for Name, defaults have 40
	if vd.List[0].Width != 50 {
		t.Errorf("expected Name width 50 from config override, got %d", vd.List[0].Width)
	}
}

func TestGetViewDef_R53Records_PartialConfig_FallsBackToDefaults(t *testing.T) {
	// Config has s3 but not r53_records -- should fall back to defaults
	paths := []string{testdataPath("views_partial.yaml")}
	cfg, err := config.LoadFrom(paths)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}

	vd := config.GetViewDef(cfg, "r53_records")
	if len(vd.List) != 4 {
		t.Fatalf("expected 4 default r53_records columns (not in partial config), got %d", len(vd.List))
	}
	// Should be default widths
	if vd.List[0].Width != 40 {
		t.Errorf("expected default Name width 40, got %d", vd.List[0].Width)
	}
}
