package unit_test

// rel2_id_answerable_test.go — the other half of the warm-cache rule: a
// checker that can still answer from the row's own ID or Fields must not
// throw that answer away just because the row carries no RawStruct.
//
// The answer-side rule (unreadZero) is right, but six checkers never reach it:
// they return early on res.RawStruct == nil, before the filter they would have
// used is even computed. For these the filter does not come from the struct.
// ec2Identity sets instanceID = res.ID before it looks at the struct at all,
// and secretIdentifiers falls back to Fields["arn"] and res.Name. So the row
// carries everything the scan needs and the panel is shown "?" for a count
// that was fully determined.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// rel2CheckerFor returns the registered checker for a (source, target) pair.
func rel2CheckerFor(t *testing.T, sourceType, targetType string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated(sourceType) {
		if def.TargetType == targetType && def.Checker != nil {
			return def.Checker
		}
	}
	t.Fatalf("no %s→%s checker registered", sourceType, targetType)
	return nil
}

// rel2DemoCacheFor fetches the named types from the demo fakes into one cache.
func rel2DemoCacheFor(t *testing.T, shortNames ...string) resource.ResourceCache {
	t.Helper()
	cache := resource.ResourceCache{}
	for _, name := range shortNames {
		cache[name] = resource.ResourceCacheEntry{Resources: rel2DemoList(t, name)}
	}
	return cache
}

// TestRel2IDAnswerableCheckersDoNotDiscardTheirCount pins the six checkers
// whose filter survives a cache replay. The proof is the bare struct: an
// ec2types.Instance with no InstanceId leaves ec2Identity returning res.ID,
// and an smtypes.SecretListEntry with no ARN leaves secretIdentifiers falling
// back to Fields["arn"] — exactly the values a nil RawStruct produces. So the
// scan sees identical inputs in both runs, and any difference in the answer
// comes from the early nil guard alone, not from anything the struct supplied.
func TestRel2IDAnswerableCheckersDoNotDiscardTheirCount(t *testing.T) {
	clients := demo.NewServiceClients()
	cache := rel2DemoCacheFor(t, "ec2", "asg", "tg", "eip", "alarm", "secrets", "eb", "ecs-task")

	cases := []struct {
		source, target string
		rowID          string
		// bare builds a struct of the row's own type carrying only what the
		// row's own ID and Fields already supply, so it gives the scan nothing
		// a nil RawStruct would not.
		bare func(resource.Resource) any
	}{
		// tg reads the VPC too, and ec2Identity takes it from Fields only when
		// the assertion fails — so the struct has to carry the same value for
		// the two runs to be comparable.
		{"ec2", "tg", "i-0a1b2c3d4e5f60001", func(r resource.Resource) any {
			return ec2types.Instance{VpcId: aws.String(r.Fields["vpc_id"])}
		}},
		{"ec2", "asg", "i-0a1b2c3d4e5f60001", func(resource.Resource) any { return ec2types.Instance{} }},
		{"ec2", "alarm", "i-0a1b2c3d4e5f60001", func(resource.Resource) any { return ec2types.Instance{} }},
		{"ec2", "eip", "i-0a1b2c3d4e5f60001", func(resource.Resource) any { return ec2types.Instance{} }},
		{"secrets", "eb", "prod/database/primary", func(resource.Resource) any { return smtypes.SecretListEntry{} }},
		{"secrets", "ecs-task", "prod/database/primary", func(resource.Resource) any { return smtypes.SecretListEntry{} }},
	}

	for _, tc := range cases {
		t.Run(tc.source+"→"+tc.target, func(t *testing.T) {
			var row resource.Resource
			for _, r := range rel2DemoList(t, tc.source) {
				if r.ID == tc.rowID {
					row = r
				}
			}
			if row.ID == "" {
				t.Fatalf("%s row %q is not in the demo list", tc.source, tc.rowID)
			}
			checker := rel2CheckerFor(t, tc.source, tc.target)

			stripped := row
			stripped.RawStruct = tc.bare(row)
			want := checker(context.Background(), clients, stripped, cache)
			if want.State() != domain.RelatedResolved || want.Count() == 0 {
				t.Fatalf("control: with a bare struct the checker answered %v/%d; "+
					"this pair no longer proves anything and the case should be re-derived",
					want.State(), want.Count())
			}

			warm := row
			warm.RawStruct = nil
			got := checker(context.Background(), clients, warm, cache)
			if got.State() != want.State() || got.Count() != want.Count() {
				t.Errorf("warm row answered %v/%d, want %v/%d — the struct supplied nothing "+
					"the scan used, so the early nil guard is discarding a count the row backs",
					got.State(), got.Count(), want.State(), want.Count())
			}
		})
	}
}
