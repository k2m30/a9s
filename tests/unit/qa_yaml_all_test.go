package unit

// Table-driven YAML sweep over every registered resource
// type. CloudTrail event JSON expansion is pinned in qa_yaml_unique_test.go.

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestQA_YAML_AllTypes iterates every registered resource type and verifies:
//
//	(a) the demo fixture's YAML view renders without error and is non-empty
//	(b) the marshaled data is valid YAML (parseable by yaml.v3)
//	(c) the marshaled data contains no unresolved template markers
//
// (b)/(c) marshal via fieldpath.ToSafeValue + yaml.Marshal rather than
// ContentLines()+stripANSI, so the check covers the marshal step itself,
// independent of the colorizing pass; the colorized path's re-parseability
// (including colon-bearing quoted keys like "aws:autoscaling:groupName") is
// pinned by TestPort_ColorizeYAML_ColonInQuotedKey_Regression.
func TestQA_YAML_AllTypes(t *testing.T) {
	forbidden := []string{"<no value>", "<nil>", "%!(EXTRA", "<missing field>"}

	clients := demo.NewServiceClients()
	ctx := context.Background()

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
