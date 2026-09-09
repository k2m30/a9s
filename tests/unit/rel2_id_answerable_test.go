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

// TestRel2AnswerableCheckersProveTheirZero is the other half of the pair
// above. Answering without a nil guard is only right if the zero it would
// hide is itself honest, so each of the six is handed a warm row and a
// target list that was read and holds nothing. The filter came from the
// row's own ID or Fields, the list was read, nothing matched — that is a
// zero the row backs, and turning it into "?" would be the same information
// loss one return lower down.
func TestRel2AnswerableCheckersProveTheirZero(t *testing.T) {
	clients := demo.NewServiceClients()
	pairs := []struct{ source, target, rowID string }{
		{"ec2", "tg", "i-0a1b2c3d4e5f60001"},
		{"ec2", "asg", "i-0a1b2c3d4e5f60001"},
		{"ec2", "alarm", "i-0a1b2c3d4e5f60001"},
		{"ec2", "eip", "i-0a1b2c3d4e5f60001"},
		{"secrets", "eb", "prod/database/primary"},
		{"secrets", "ecs-task", "prod/database/primary"},
	}

	for _, tc := range pairs {
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
			row.RawStruct = nil

			// Present and read, holding nothing: the population is empty, not absent.
			cache := resource.ResourceCache{
				tc.target: resource.ResourceCacheEntry{Resources: []resource.Resource{}},
			}
			result := rel2CheckerFor(t, tc.source, tc.target)(context.Background(), clients, row, cache)

			if result.State() != domain.RelatedResolved || result.Count() != 0 {
				t.Errorf("State = %v, Count = %d; want Resolved 0: the list was read and was empty, "+
					"and the filter came from the row itself", result.State(), result.Count())
			}
		})
	}
}

// TestRel2StructDependentCheckersStayUnknown pins the three guards that stay.
// Each filter lives only in the struct — an instance's CloudFormation tag, its
// EKS tags, a service's task definition — so a warm row cannot produce it, and
// the count these checkers reach with the struct is exactly what they must not
// claim to have ruled out without it.
func TestRel2StructDependentCheckersStayUnknown(t *testing.T) {
	clients := demo.NewServiceClients()
	cache := rel2DemoCacheFor(t, "ec2", "cfn", "ng", "ecs-svc", "sfn")

	pairs := []struct{ source, target string }{
		{"ec2", "cfn"},
		{"ec2", "ng"},
		{"ecs-svc", "sfn"},
	}

	for _, tc := range pairs {
		t.Run(tc.source+"→"+tc.target, func(t *testing.T) {
			checker := rel2CheckerFor(t, tc.source, tc.target)
			var matched resource.Resource
			for _, r := range rel2DemoList(t, tc.source) {
				if checker(context.Background(), clients, r, cache).Count() > 0 {
					matched = r
					break
				}
			}
			if matched.ID == "" {
				t.Fatalf("no demo %s row resolves a %s count; the pair proves nothing",
					tc.source, tc.target)
			}

			warm := matched
			warm.RawStruct = nil
			result := checker(context.Background(), clients, warm, cache)
			if result.State() != domain.RelatedUnknown {
				t.Errorf("row %s: State = %v, Count = %d; want Unknown — the filter lives only in "+
					"the struct, so a warm row has ruled nothing out", matched.ID, result.State(), result.Count())
			}
		})
	}
}
