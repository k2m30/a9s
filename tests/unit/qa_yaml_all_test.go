package unit

// qa_yaml_all_test.go — table-driven YAML sweep over every registered resource type.
// Replaces the boilerplate ViewContainsFields / FrameTitle / RawContentUncolored
// tests that were duplicated across:
//   - qa_yaml_ec2_family_test.go
//   - qa_yaml_services_test.go
//   - qa_yaml_v220_test.go (boilerplate portion)
//
// Unique tests (CloudTrail JSON expansion, scroll, wrap, edge cases) are kept
// in their original files. CT event JSON uniqueness lives in qa_yaml_unique_test.go.

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestQA_YAML_AllTypes iterates every registered resource type and verifies:
//
//	(a) the demo fixture's YAML view renders without error and is non-empty
//	(b) RawContent() is valid YAML (parseable by yaml.v3)
//	(c) RawContent() contains no unresolved template markers
//	(d) FrameTitle() contains "yaml"
//	(e) RawContent() contains no ANSI escape codes
func TestQA_YAML_AllTypes(t *testing.T) {
	forbidden := []string{"<no value>", "<nil>", "%!(EXTRA", "<missing field>"}

	for _, rt := range resource.AllResourceTypes() {
		rt := rt
		t.Run(rt.ShortName, func(t *testing.T) {
			fixtures, ok := demo.GetResources(rt.ShortName)
			if !ok || len(fixtures) == 0 {
				t.Skipf("no demo fixtures for %s — skipping", rt.ShortName)
			}
			res := fixtures[0]

			// (a) non-empty view
			out := yamlView(t, res, 120, 40)
			if out == "" {
				t.Fatalf("YAML view is empty for %s", rt.ShortName)
			}

			m := yamlModel(res, 120, 40)
			raw := m.RawContent()

			// (b) valid YAML
			var parsed any
			if err := yaml.Unmarshal([]byte(raw), &parsed); err != nil {
				t.Errorf("RawContent() is not valid YAML for %s: %v\n---\n%s", rt.ShortName, err, raw)
			}

			// (c) no unresolved template markers
			for _, f := range forbidden {
				if strings.Contains(raw, f) {
					t.Errorf("%s YAML contains unresolved template marker %q", rt.ShortName, f)
				}
			}

			// (d) FrameTitle contains "yaml"
			title := m.FrameTitle()
			if !strings.Contains(title, "yaml") {
				t.Errorf("%s FrameTitle() = %q, want 'yaml' in title", rt.ShortName, title)
			}

			// (e) RawContent has no ANSI codes
			if strings.Contains(raw, "\x1b[") {
				t.Errorf("%s RawContent() contains ANSI codes — must be plain for clipboard copy", rt.ShortName)
			}
		})
	}
}
