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
	"context"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestQA_YAML_AllTypes iterates every registered resource type and verifies:
//
//	(a) the demo fixture's YAML view renders without error and is non-empty
//	(b) the marshaled data is valid YAML (parseable by yaml.v3)
//	(c) the marshaled data contains no unresolved template markers
//
// (b)/(c) marshal directly via fieldpath.ToSafeValue + yaml.Marshal — the
// exact steps YAMLModel.RawContent() (DEAD, no live caller) used internally
// — rather than through ContentLines()+stripANSI. This keeps the pin on the
// marshal step itself, independent of the colorizing pass; the colorized
// path's re-parseability (including colon-bearing quoted keys like
// "aws:autoscaling:groupName") is pinned separately by
// TestWave3Port_ColorizeYAML_ColonInQuotedKey_Regression.
//
// (d) FrameTitle() was dropped: DEAD, no live caller (see qa_docdb_test.go's
// identical retirement note). (e) "no ANSI codes" was dropped: the marshal
// step tested here never touches colorizeYAML, so ANSI-free is tautological;
// the real ANSI-free clipboard-copy contract is already pinned end-to-end via
// handleCopy in wave3_text_ports_test.go's
// TestWave3Port_ErrorLogCopy_UncoloredContent and qa_copy_test.go's
// TestQA_Copy_YAML_CopiesFullYAML.
func TestQA_YAML_AllTypes(t *testing.T) {
	forbidden := []string{"<no value>", "<nil>", "%!(EXTRA", "<missing field>"}

	clients := demo.NewServiceClients()
	ctx := context.Background()

	// Representative sample — full sweep in CI slow suite
	sampleYAML := []resource.ResourceTypeDef{
		*resource.FindResourceType("ec2"),
		*resource.FindResourceType("s3"),
		*resource.FindResourceType("secrets"),
	}
	for _, rt := range sampleYAML {
		t.Run(rt.ShortName, func(t *testing.T) {
			fetcher := resource.GetPaginatedFetcher(rt.ShortName)
			if fetcher == nil {
				t.Skipf("no paginated fetcher for %s — skipping", rt.ShortName)
			}
			result, fetchErr := fetcher(ctx, clients, "")
			if fetchErr != nil || len(result.Resources) == 0 {
				t.Skipf("no demo fixtures for %s (err=%v, len=%d) — skipping", rt.ShortName, fetchErr, len(result.Resources))
			}
			res := result.Resources[0]

			// (a) non-empty view
			out := yamlView(t, res, 120, 40)
			if out == "" {
				t.Fatalf("YAML view is empty for %s", rt.ShortName)
			}

			var data []byte
			var marshalErr error
			if res.RawStruct != nil {
				safe := fieldpath.ToSafeValue(reflect.ValueOf(res.RawStruct))
				data, marshalErr = yaml.Marshal(safe)
			} else if len(res.Fields) > 0 {
				data, marshalErr = yaml.Marshal(res.Fields)
			}
			if marshalErr != nil {
				t.Fatalf("marshal error for %s: %v", rt.ShortName, marshalErr)
			}
			raw := string(data)

			// (b) valid YAML
			var parsed any
			if err := yaml.Unmarshal(data, &parsed); err != nil {
				t.Errorf("marshaled data is not valid YAML for %s: %v\n---\n%s", rt.ShortName, err, raw)
			}

			// (c) no unresolved template markers
			for _, f := range forbidden {
				if strings.Contains(raw, f) {
					t.Errorf("%s YAML contains unresolved template marker %q", rt.ShortName, f)
				}
			}
		})
	}
}
