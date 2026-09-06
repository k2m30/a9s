package unit_test

// rel2_ct_candidates_test.go — rows 3 and 5: an event that names a secret by
// its full ARN cannot confirm it against the secrets list, and the fragment
// rule that lets a path-named role be confirmed has one demo witness where it
// should have two.

import (
	"context"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// rel2CTCheckerByTarget returns the ct-events checker for a target, picked by
// display name where several defs share a target type.
func rel2CTCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ct-events") {
		if def.TargetType == target && def.Checker != nil {
			return def.Checker
		}
	}
	t.Fatalf("no ct-events checker for %q", target)
	return nil
}

// rel2CTFixtureByID finds one demo ct-events row by its event id.
func rel2CTFixtureByID(t *testing.T, id string) resource.Resource {
	t.Helper()
	for _, r := range loadAllCTFixtures(t) {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no demo ct-events fixture with id %q", id)
	return resource.Resource{}
}

// rel2DemoList fetches the demo rows of a type through its registered fetcher,
// so the candidate rule is matched against the list the panel really holds.
func rel2DemoList(t *testing.T, shortName string) []resource.Resource {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s not registered", shortName)
	}
	out, err := td.Fetcher(context.Background(), demo.NewServiceClients(), "")
	if err != nil && len(out.Resources) == 0 {
		t.Fatalf("%s fetch: %v", shortName, err)
	}
	if len(out.Resources) == 0 {
		t.Fatalf("%s: no demo rows", shortName)
	}
	return out.Resources
}

// --- row 3: a secret named by its ARN --------------------------------------

// TestRel2SecretNamedByARNResolves pins the ARN candidate against the secrets
// list. A secret's id is its name, while its ARN carries the six-character
// suffix AWS appends, so an event that names the ARN can only be confirmed by
// matching the list's own ARN. Guessing the suffix away would be inventing a
// name AWS never promised.
func TestRel2SecretNamedByARNResolves(t *testing.T) {
	event := rel2CTFixtureByID(t, "evt-secrets-get-001")
	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{Resources: rel2DemoList(t, "secrets")},
	}

	result := rel2CTCheckerByTarget(t, "secrets")(context.Background(), nil, event, cache)

	if result.State() != domain.RelatedResolved {
		t.Fatalf("State = %v, want Resolved", result.State())
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1: the event names arn:...:secret:prod/database/primary-AbCdEf", result.Count())
	}
	if ids := result.ResourceIDs(); len(ids) != 1 || ids[0] != "prod/database/primary" {
		t.Errorf("ResourceIDs = %v, want [prod/database/primary]: the row navigates by the list's own id", ids)
	}
}

// TestRel2SecretNamedByNameStillResolves is the counterpart the ARN rule must
// not disturb: an event naming a secret by its plain name resolves the same
// way it always did.
func TestRel2SecretNamedByNameStillResolves(t *testing.T) {
	event := rel2CTFixtureByID(t, fixtures.CtEventPathNamedSecret)
	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{Resources: rel2DemoList(t, "secrets")},
	}

	result := rel2CTCheckerByTarget(t, "secrets")(context.Background(), nil, event, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

// TestRel2UnknownSecretARNResolvesZero pins the negative: an ARN naming a
// secret this account does not hold confirms nothing, and matching on the ARN
// field must not turn a miss into a match.
func TestRel2UnknownSecretARNResolvesZero(t *testing.T) {
	event := rel2CTFixtureByID(t, "evt-secrets-get-001")
	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{Resources: []resource.Resource{{
			ID:   "acme-unrelated-secret",
			Name: "acme-unrelated-secret",
			Fields: map[string]string{
				"arn": "arn:aws:secretsmanager:us-east-1:123456789012:secret:acme-unrelated-secret-ZzZzZz",
			},
		}}},
	}

	result := rel2CTCheckerByTarget(t, "secrets")(context.Background(), nil, event, cache)

	if result.State() != domain.RelatedResolved {
		t.Fatalf("State = %v, want Resolved", result.State())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

// --- row 5: the fragment rule needs a second witness ------------------------

// rel2SecondPathNamedRole is the demo role dev adds for this row: a second
// role living under an IAM path, so the fragment rule is exercised by more
// than the single fixture that happens to carry it today. Its event is
// evt-iam-assume-002.
const rel2SecondPathNamedRole = "acme-partner-integration-role"

// TestRel2PathNamedRoleResolvesFromItsEvent pins the shape both witnesses must
// render. An event names a role by an ARN whose resource part carries a path,
// and the id the panel navigates by is the bare role name the roles list
// holds — the fragment rule, exercised on the witness that exists today.
func TestRel2PathNamedRoleResolvesFromItsEvent(t *testing.T) {
	event := rel2CTFixtureByID(t, fixtures.CtEventPathNamedRole)
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: rel2DemoList(t, "role")},
	}

	result := rel2CTCheckerByTarget(t, "role")(context.Background(), nil, event, cache)

	if result.State() != domain.RelatedResolved {
		t.Fatalf("State = %v, want Resolved", result.State())
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

// TestRel2SecondPathNamedRoleResolvesFromItsEvent is the same shape on the
// witness dev adds. Two witnesses are what keeps the fragment rule honest: one
// fixture can be satisfied by a coincidence of that fixture's spelling.
func TestRel2SecondPathNamedRoleResolvesFromItsEvent(t *testing.T) {
	event := rel2CTFixtureByID(t, fixtures.CtEventSecondPathNamedRole)
	roles := rel2DemoList(t, "role")

	found := false
	for _, r := range roles {
		if r.ID == rel2SecondPathNamedRole {
			found = true
		}
	}
	if !found {
		t.Fatalf("demo role %q is missing; row 5 needs it and a second event naming it by a path-carrying ARN",
			rel2SecondPathNamedRole)
	}

	result := rel2CTCheckerByTarget(t, "role")(context.Background(), nil, event,
		resource.ResourceCache{"role": resource.ResourceCacheEntry{Resources: roles}})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if ids := result.ResourceIDs(); len(ids) != 1 || ids[0] != rel2SecondPathNamedRole {
		t.Errorf("ResourceIDs = %v, want [%s]", ids, rel2SecondPathNamedRole)
	}
}

// TestRel2PathNamedRoleIdIsNeverAPathFragment pins what the fragment rule may
// not do: no candidate a role event produces may be a piece of the ARN
// grammar. A row navigating to "service-role" opens nothing.
func TestRel2PathNamedRoleIdIsNeverAPathFragment(t *testing.T) {
	event := rel2CTFixtureByID(t, fixtures.CtEventPathNamedRole)
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: rel2DemoList(t, "role")},
	}

	result := rel2CTCheckerByTarget(t, "role")(context.Background(), nil, event, cache)
	for _, id := range result.ResourceIDs() {
		if strings.Contains(id, "/") || id == "role" || id == "service-role" {
			t.Errorf("resolved id %q is a fragment of the ARN grammar, not a role id", id)
		}
	}
}
