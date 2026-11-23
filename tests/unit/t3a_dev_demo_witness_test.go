package unit_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// Each demo witness of the fixed-value rule renders its verdict: a grant
// whose condition still matches anyone (a wildcard bucket name, a /0 range
// with a non-zero base) shows the public-policy finding in the list Status
// cell, with a Deny that did not close it (the data-perimeter Deny, which
// spares every AWS service principal) as a "Deny when" row; a grant scoped
// to one role's sessions, to mount-target clients, or fenced for both caller
// classes by the two-statement organization perimeter shows none.
func TestT3a_Dev_DemoWitnessesRenderTheirVerdict(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()
	cases := []struct {
		short, id, code, phrase string
		public                  bool
		denyWhen                string
	}{
		{"sqs", fixtures.SQSPublicPolicy, "sqs.public-policy", "queue policy open to anyone", true, ""},
		{"sqs", fixtures.SQSOrgPerimeterPolicy, "sqs.public-policy", "", false, ""},
		{"sns", fixtures.SNSPublicPolicyARN, "sns.public-policy", "topic policy open to anyone", true, "BoolIfExists aws:PrincipalIsAWSService false and StringNotEquals aws:SourceVpce vpce-0acme1234567890ab"},
		{"sns", fixtures.SNSRoleSessionsPolicyARN, "sns.public-policy", "", false, ""},
		{"efs", fixtures.EFSMountTargetOnlyPolicy, "efs.public-policy", "", false, ""},
	}
	for _, c := range cases {
		t.Run(c.short+"/"+c.id, func(t *testing.T) {
			td := *resource.FindResourceType(c.short)
			rows := mergeWave2Findings(t, td, byType[c.short], cache, clients)
			i := slices.IndexFunc(rows, func(r resource.Resource) bool { return r.ID == c.id || r.Name == c.id })
			if i < 0 {
				t.Fatalf("no %s demo row %q", c.short, c.id)
			}
			has := slices.ContainsFunc(rows[i].Findings, func(f domain.Finding) bool { return string(f.Code) == c.code })
			if has != c.public {
				t.Errorf("%s finding present = %v, want %v; findings %+v", c.code, has, c.public, rows[i].Findings)
			}
			if !c.public {
				return
			}
			if c.denyWhen != "" {
				ad := rows[i].AttentionDetails[domain.FindingCode(c.code)]
				if !slices.ContainsFunc(ad.Rows, func(r domain.DetailRow) bool { return r.Label == "Deny when" && r.Value == c.denyWhen }) {
					t.Errorf("Attention rows = %+v, want Deny when %q", ad.Rows, c.denyWhen)
				}
			}
			cell, ok := listStatusCellFor(t, td, rows, rows[i].ID)
			if !ok || !strings.Contains(cell, c.phrase) {
				t.Errorf("Status cell = %q (found %v), want it to contain %q", cell, ok, c.phrase)
			}
		})
	}
}
