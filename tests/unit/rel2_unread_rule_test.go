package unit_test

// rel2_unread_rule_test.go — the answer-side rule at its four boundaries.
//
// unreadZero turns a resolved zero from a row with no RawStruct into Unknown.
// unreadZeroScanned is the same except a zero over a population of none keeps
// its zero, because no row existed for the source to match and the answer is
// proven whatever the source could have said. The cases below are the ones
// where getting the boundary wrong is invisible in the aggregate gates: a
// proven zero silently becoming "?", or a lower bound silently losing its "+".

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// rel2WarmEBSRow is a volume as a disk-cache replay produces it: identity and
// Fields, no RawStruct.
func rel2WarmEBSRow() resource.Resource {
	return resource.Resource{
		ID:     "vol-0a1b2c3d4e5f60001",
		Name:   "vol-0a1b2c3d4e5f60001",
		Type:   "ebs",
		Fields: map[string]string{"state": "in-use"},
	}
}

// TestRel2UnreadZeroKeepsAProvenZeroOverAnEmptyPopulation pins the exception
// unreadZeroScanned exists for. The backup list was read and holds nothing, so
// no plan could have covered this volume whatever its tags said. Turning that
// into "?" would claim we might have missed a plan that demonstrably is not
// there.
func TestRel2UnreadZeroKeepsAProvenZeroOverAnEmptyPopulation(t *testing.T) {
	checker := rel2CheckerFor(t, "ebs", "backup")
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{Resources: []resource.Resource{}},
	}
	result := checker(context.Background(), nil, rel2WarmEBSRow(), cache)

	if result.State() != domain.RelatedResolved || result.Count() != 0 {
		t.Errorf("State = %v, Count = %d; want Resolved 0: the backup list was read and was empty, "+
			"so no plan could cover this volume regardless of what the unread row would have said",
			result.State(), result.Count())
	}
}

// TestRel2UnreadZeroOverRealRowsIsUnknown is the same checker one row away from
// the case above. Plans exist, the match runs off tags that live only in the
// RawStruct, and the row carries none — so the zero is a claim about a row
// nothing read, and the panel owes "?".
func TestRel2UnreadZeroOverRealRowsIsUnknown(t *testing.T) {
	checker := rel2CheckerFor(t, "ebs", "backup")
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{Resources: []resource.Resource{{
			ID:     "acme-daily-plan",
			Fields: map[string]string{"selection_tags": "Backup=daily"},
		}}},
	}
	result := checker(context.Background(), nil, rel2WarmEBSRow(), cache)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v, Count = %d; want Unknown: a plan exists and the tags that would "+
			"match it were never read", result.State(), result.Count())
	}
}

// TestRel2LowerBoundSurvivesTheUnreadRule pins the interaction between the two
// rules the batch has now landed. A truncated list that matched nothing is a
// lower bound — "(0+)" — and it must stay one even when the source row was
// never read, because the "+" is a fact about the list, not about the row.
// Collapsing it to "?" would lose the one thing the scan did establish.
func TestRel2LowerBoundSurvivesTheUnreadRule(t *testing.T) {
	checker := rel2CheckerFor(t, "ebs", "backup")
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{
				ID:     "acme-daily-plan",
				Fields: map[string]string{"selection_tags": "Backup=daily"},
			}},
			IsTruncated: true,
		},
	}
	result := checker(context.Background(), nil, rel2WarmEBSRow(), cache)

	if result.State() != domain.RelatedResolved || result.Count() != 0 || !result.Truncated() {
		t.Errorf("State = %v, Count = %d, Truncated = %v; want Resolved 0 truncated: "+
			"the list was cut short, which is a fact about the list and survives the source row "+
			"being unread", result.State(), result.Count(), result.Truncated())
	}
}

// TestRel2VolumeIDCallersAnswerBothWays pins ec2VolumeIDs' two callers, the
// helper that swallowed its assertion and returned an empty map. Dev declined
// to give it an ok result, so the two callers are the only place the
// distinction can be checked: an unread instance owes "?", and an instance
// that was read and genuinely has no block devices owes a zero. If the helper
// ever loses that distinction again both halves cannot hold at once.
func TestRel2VolumeIDCallersAnswerBothWays(t *testing.T) {
	cache := resource.ResourceCache{
		"ebs":      resource.ResourceCacheEntry{Resources: []resource.Resource{{ID: "vol-0a1b2c3d4e5f60001"}}},
		"ebs-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{{ID: "snap-0a1b2c3d4e5f60001"}}},
	}
	warm := resource.Resource{
		ID:     "i-0a1b2c3d4e5f60001",
		Name:   "acme-api-1",
		Type:   "ec2",
		Fields: map[string]string{"state": "running"},
	}
	read := warm
	// An instance that was read and has no volumes attached: the zero is a
	// fact about the instance, not about what we failed to look at.
	read.RawStruct = ec2types.Instance{InstanceId: aws.String(warm.ID)}

	for _, target := range []string{"ebs", "ebs-snap"} {
		t.Run(target, func(t *testing.T) {
			checker := rel2CheckerFor(t, "ec2", target)

			if got := checker(context.Background(), nil, warm, cache); got.State() != domain.RelatedUnknown {
				t.Errorf("unread instance: State = %v, Count = %d; want Unknown",
					got.State(), got.Count())
			}
			got := checker(context.Background(), nil, read, cache)
			if got.State() != domain.RelatedResolved || got.Count() != 0 {
				t.Errorf("read instance with no block devices: State = %v, Count = %d; want Resolved 0",
					got.State(), got.Count())
			}
		})
	}
}
