package unit_test

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/tests/unit"
)

// The demo shows the three policy verdicts that carry no grant to read: a
// public grant written with NotAction lists no actions, a document that does
// not parse is not inspected, and a repository whose policy read carries no
// document is not inspected, since ECR answers "no policy" with an error.
func TestT3BDev_DemoWitnessesForTheVerdictsWithoutAGrant(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()

	secrets := unit.Wave2Result(t, "secrets", clients, byType["secrets"], cache)
	publicID := t3bDevRowID(t, byType["secrets"], fixtures.SecretPublicPolicy)
	rows := secrets.AttentionDetails[publicID]["secrets.public-policy"].Rows
	if len(rows) != 1 || rows[0].Label != "Principal" {
		t.Errorf("NotAction witness %q rows = %+v, want the Principal row alone", publicID, rows)
	}
	if got := secrets.TruncatedIDs[t3bDevRowID(t, byType["secrets"], fixtures.SecretUnreadablePolicy)]; got != awsclient.CheckPolicyUnreadable {
		t.Errorf("unreadable secret mark = %q, want %q", got, awsclient.CheckPolicyUnreadable)
	}

	ecr := unit.Wave2Result(t, "ecr", clients, byType["ecr"], cache)
	if got := ecr.TruncatedIDs[t3bDevRowID(t, byType["ecr"], fixtures.ECRPolicyMissing)]; got != awsclient.CheckPolicyMissing {
		t.Errorf("repository with no policy document mark = %q, want %q", got, awsclient.CheckPolicyMissing)
	}
	if fs := ecr.Findings[t3bDevRowID(t, byType["ecr"], fixtures.ECRPolicyMissing)]; len(fs) != 0 {
		t.Errorf("repository with no policy document raised %v", fs)
	}
}

func t3bDevRowID(t *testing.T, rows []resource.Resource, name string) string {
	t.Helper()
	for _, r := range rows {
		if r.Name == name || r.ID == name {
			return r.ID
		}
	}
	t.Fatalf("no demo row named %s", name)
	return ""
}
