// aws_review_pins_test.go — efs registration and fixture pins.
package unit

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------
// efs SetFieldKeysForTest must include every key the fetcher writes: the
// fetcher populates Fields["throughput_mode"], and a key missing from the
// registered list is invisible to tooling that enumerates the registered
// keys (viewsgen, YAML merging).
// ---------------------------------------------------------------------------
func TestEFS_RegisterFieldKeys_IncludesThroughputMode(t *testing.T) {
	keys := resource.GetFieldKeys("efs")
	for _, k := range keys {
		if k == "throughput_mode" {
			return
		}
	}
	t.Fatalf("SetFieldKeysForTest(\"efs\") missing %q — fetcher writes Fields[%q] but it is not registered; keys=%v", "throughput_mode", "throughput_mode", keys)
}

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------
// EFS mount-target ENI Groups[].GroupName must match the GroupName on the
// SecurityGroup fixtures with the same GroupId; a name mismatch for the same
// GroupId is a self-inconsistent graph.
// ---------------------------------------------------------------------------
func TestEFS_FixtureENIGroupNamesMatchSecurityGroups(t *testing.T) {
	fix := fixtures.NewEC2Fixtures()

	// Build GroupId → GroupName map from SecurityGroup fixtures.
	sgNames := make(map[string]string)
	for _, sg := range fix.SecurityGroups {
		if sg.GroupId != nil && sg.GroupName != nil {
			sgNames[*sg.GroupId] = *sg.GroupName
		}
	}

	// For each ENI that references an EFS prod SG, its GroupName must match.
	checkIDs := map[string]bool{
		fixtures.ProdEFSSecurityGroupAID: true,
		fixtures.ProdEFSSecurityGroupBID: true,
	}

	for _, eni := range fix.NetworkInterfaces {
		for _, g := range eni.Groups {
			if g.GroupId == nil || g.GroupName == nil {
				continue
			}
			if !checkIDs[*g.GroupId] {
				continue
			}
			want, ok := sgNames[*g.GroupId]
			if !ok {
				continue
			}
			if *g.GroupName != want {
				t.Errorf("ENI %s references SG %s with GroupName=%q, but SecurityGroup fixture has GroupName=%q — fixtures are self-inconsistent",
					aws.ToString(eni.NetworkInterfaceId), *g.GroupId, *g.GroupName, want)
			}
		}
	}
}
