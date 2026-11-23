package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// The demo is where an operator first meets a new finding, so a cross-account
// grant must render there as the row's own Status and colour, not only exist
// as a code a unit test raises.
func TestT3B_DemoWitnessForEachCrossAccountCode(t *testing.T) {
	bench := policyBench(t)
	for _, short := range []string{"kms", "sqs", "sns", "lambda", "ecr", "efs", "codeartifact"} {
		t.Run(short, func(t *testing.T) {
			code := domain.FindingCode(short + ".cross-account-policy")
			td := resource.FindResourceType(short)
			rows := bench[short]
			var witness *resource.Resource
			for i := range rows {
				if top, ok := domain.TopFinding(rows[i].Findings); ok && top.Code == code {
					witness = &rows[i]
					break
				}
			}
			if witness == nil {
				t.Fatalf("no demo %s row whose top finding is %s", short, code)
			}
			top, _ := domain.TopFinding(witness.Findings)
			cell, ok := listStatusCellFor(t, *td, rows, witness.ID)
			if !ok {
				t.Fatalf("no list row for witness %s", witness.ID)
			}
			if got, _, _ := strings.Cut(cell, " (+"); got != catalog.Phrase(code) {
				t.Errorf("witness %s Status cell = %q, want %q", witness.ID, cell, catalog.Phrase(code))
			}
			if got, want := td.ResolveColor(*witness), w5ColorOfSeverity(top.Severity); got != want {
				t.Errorf("witness %s colour = %v, want %v (the colour of its top finding)", witness.ID, got, want)
			}
		})
	}
}

// A policy a9s cannot read shows on its Wave 1 row as the type's
// "<noun> policy not inspected" Status, and the demo carries one such row per
// type so the rendering can be seen.
func TestT3B_DemoWitnessForAnUnknownPolicyVerdict(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	for _, short := range []string{"opensearch", "role", "vpce"} {
		td := resource.FindResourceType(short)
		code := td.Policy.Unread
		rows := byType[short]
		found := false
		for _, r := range rows {
			if top, ok := domain.TopFinding(r.Findings); !ok || top.Code != code {
				continue
			}
			cell, _ := listStatusCellFor(t, *td, rows, r.ID)
			if got, _, _ := strings.Cut(cell, " (+"); got == catalog.Phrase(code) {
				found = true
			}
		}
		if !found {
			t.Errorf("no demo %s row whose Status cell reads %q", short, catalog.Phrase(code))
		}
	}
}
