package unit_test

// rel2_census_test.go — the census row 4 closed on, as a gate, and the
// false-match attack on the ARN candidate row 3 added.

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestRel2EveryPivotTargetHasAFetcher is the census that closed row 4. The
// cold-cache Unknown is reachable only when a pivot names a target type
// nothing can fetch; today none does, which is why the state has no demo
// witness. Registering such a pivot would reopen that hole silently, so the
// census is a gate rather than a paragraph.
func TestRel2EveryPivotTargetHasAFetcher(t *testing.T) {
	var missing []string
	for _, td := range resource.AllResourceTypes() {
		for _, def := range td.Related {
			if def.TargetType == "" {
				continue
			}
			if resource.GetPaginatedFetcher(def.TargetType) == nil {
				missing = append(missing, td.ShortName+" → "+def.TargetType)
			}
		}
	}
	if len(missing) > 0 {
		t.Errorf("pivots whose target type has no fetcher, so the panel can only say \"?\": %v", missing)
	}
}

// rel2SecretsCacheWith builds a secrets list from id/arn pairs.
func rel2SecretsCacheWith(pairs map[string]string) resource.ResourceCache {
	rows := make([]resource.Resource, 0, len(pairs))
	for id, arn := range pairs {
		rows = append(rows, resource.Resource{
			ID: id, Name: id, Fields: map[string]string{"arn": arn},
		})
	}
	return resource.ResourceCache{"secrets": resource.ResourceCacheEntry{Resources: rows}}
}

// TestRel2SecretARNIsNotMatchedByPrefixOrSuffix attacks the ARN candidate: two
// secrets whose ARNs contain one another as a prefix and a suffix must not
// both answer for the event. A row that matched on containment would offer a
// pivot to a secret the event never named.
func TestRel2SecretARNIsNotMatchedByPrefixOrSuffix(t *testing.T) {
	const named = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/primary-AbCdEf"
	cache := rel2SecretsCacheWith(map[string]string{
		"prod/database/primary":       named,
		"prod/database/primary-extra": named + "-extra",
		"database/primary-shadow":     "arn:aws:secretsmanager:us-east-1:123456789012:secret:database/primary-AbCdEf",
	})

	event := rel2CTFixtureByID(t, "evt-secrets-get-001")
	result := rel2CTCheckerByTarget(t, "secrets")(context.Background(), nil, event, cache)

	if result.State() != domain.RelatedResolved {
		t.Fatalf("State = %v, want Resolved", result.State())
	}
	if ids := result.ResourceIDs(); len(ids) != 1 || ids[0] != "prod/database/primary" {
		t.Errorf("ResourceIDs = %v, want exactly [prod/database/primary]", ids)
	}
}

// TestRel2SecretARNDoesNotOutrankAnExactIDMatch pins the candidate order the
// row asked for: the ARN is offered last, so a list row whose id the event
// named directly still wins. Putting the ARN first is what broke the cfn and
// lambda candidate rules.
func TestRel2SecretARNDoesNotOutrankAnExactIDMatch(t *testing.T) {
	const named = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/primary-AbCdEf"
	// One row holds the id the ARN's resource part spells; another holds the
	// ARN itself. The id candidate comes first, so it is the one that answers.
	cache := rel2SecretsCacheWith(map[string]string{
		"prod/database/primary-AbCdEf": "arn:aws:secretsmanager:us-east-1:123456789012:secret:other-XxXxXx",
		"prod/database/primary":        named,
	})

	event := rel2CTFixtureByID(t, "evt-secrets-get-001")
	result := rel2CTCheckerByTarget(t, "secrets")(context.Background(), nil, event, cache)

	if ids := result.ResourceIDs(); len(ids) != 1 || ids[0] != "prod/database/primary-AbCdEf" {
		t.Errorf("ResourceIDs = %v, want [prod/database/primary-AbCdEf]: the id candidate is offered before the ARN", ids)
	}
}
