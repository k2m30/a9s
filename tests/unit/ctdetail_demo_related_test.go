package unit

// ctdetail_demo_related_test.go — per-case right-column typed-group count assertions.
//
// For each of the 9 demo ct-events fixtures (Case A–I by EventID), this test
// invokes the RegisterRelatedDemo("ct-events") checker and asserts the exact count
// for each of the 13 typed related groups.
//
// The test is intentionally written before per-event logic exists in the demo
// checker, so it starts red. That is the TDD deliverable.
//
// Expected count matrix source: docs/design/ct-event-detail.md §7b.10 + §4b.* wireframes.

import (
	"fmt"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

type ctRelatedCaseExpectation struct {
	eventID string
	label   string           // human-readable case label for test names
	counts  map[string]int   // TargetType → expected count
}

func TestCtEventsDemoRelatedCounts(t *testing.T) {
	cases := []ctRelatedCaseExpectation{
		{
			eventID: "e-a1b2c3d4",
			label:   "A-Karpenter-DescribeInstances",
			counts: map[string]int{
				"role":      1,
				"iam-user":  0,
				"ec2":       0,
				"s3":        0,
				"s3_objects": 0,
				"lambda":    0,
				"rds":       0,
				"kms":       0,
				"secrets":   0,
				"vpce":      0,
				"sg":        0,
				"ddb":       0,
				"cfn":       0,
			},
		},
		{
			eventID: "e-b2c3d4e5",
			label:   "B-SSO-TerminateInstances",
			counts: map[string]int{
				"role":      1,
				"iam-user":  0,
				"ec2":       2,
				"s3":        0,
				"s3_objects": 0,
				"lambda":    0,
				"rds":       0,
				"kms":       0,
				"secrets":   0,
				"vpce":      0,
				"sg":        0,
				"ddb":       0,
				"cfn":       0,
			},
		},
		{
			eventID: "e-c3d4e5f6",
			label:   "C-IAMUser-PutObject-AccessDenied",
			counts: map[string]int{
				"role":      0,
				"iam-user":  1,
				"ec2":       0,
				"s3":        1,
				"s3_objects": 1,
				"lambda":    0,
				"rds":       0,
				"kms":       0,
				"secrets":   0,
				"vpce":      0,
				"sg":        0,
				"ddb":       0,
				"cfn":       0,
			},
		},
		{
			eventID: "e-d4e5f6a7",
			label:   "D-KMS-RotateKey-AwsServiceEvent",
			counts: map[string]int{
				"role":      0,
				"iam-user":  0,
				"ec2":       0,
				"s3":        0,
				"s3_objects": 0,
				"lambda":    0,
				"rds":       0,
				"kms":       1,
				"secrets":   0,
				"vpce":      0,
				"sg":        0,
				"ddb":       0,
				"cfn":       0,
			},
		},
		{
			eventID: "e-e5f6a7b8",
			label:   "E-Root-PutBucketPolicy",
			counts: map[string]int{
				"role":      0,
				"iam-user":  0,
				"ec2":       0,
				"s3":        1,
				"s3_objects": 0,
				"lambda":    0,
				"rds":       0,
				"kms":       0,
				"secrets":   0,
				"vpce":      0,
				"sg":        0,
				"ddb":       0,
				"cfn":       0,
			},
		},
		{
			eventID: "e-f6a7b8c9",
			label:   "F-IRSA-GetObject",
			counts: map[string]int{
				"role":      1,
				"iam-user":  0,
				"ec2":       0,
				"s3":        1,
				"s3_objects": 1,
				"lambda":    0,
				"rds":       0,
				"kms":       0,
				"secrets":   0,
				"vpce":      1,
				"sg":        0,
				"ddb":       0,
				"cfn":       0,
			},
		},
		{
			eventID: "e-a7b8c9d0",
			label:   "G-CrossAccount-PutObject",
			counts: map[string]int{
				"role":      1,
				"iam-user":  0,
				"ec2":       0,
				"s3":        1,
				"s3_objects": 1,
				"lambda":    0,
				"rds":       0,
				"kms":       0,
				"secrets":   0,
				"vpce":      0,
				"sg":        0,
				"ddb":       0,
				"cfn":       0,
			},
		},
		{
			eventID: "e-b8c9d0e1",
			label:   "H-Insight-RunInstances",
			counts: map[string]int{
				"role":      0,
				"iam-user":  0,
				"ec2":       0,
				"s3":        0,
				"s3_objects": 0,
				"lambda":    0,
				"rds":       0,
				"kms":       0,
				"secrets":   0,
				"vpce":      0,
				"sg":        0,
				"ddb":       0,
				"cfn":       0,
			},
		},
		{
			eventID: "e-c9d0e1f2",
			label:   "I-NetworkActivity-VPCE-deny",
			counts: map[string]int{
				"role":      1,
				"iam-user":  0,
				"ec2":       0,
				"s3":        1,
				"s3_objects": 1,
				"lambda":    0,
				"rds":       0,
				"kms":       0,
				"secrets":   0,
				"vpce":      1,
				"sg":        0,
				"ddb":       0,
				"cfn":       0,
			},
		},
	}

	// Retrieve the demo checker registered for ct-events.
	demoFn := resource.GetRelatedDemo("ct-events")
	if demoFn == nil {
		t.Fatal("no demo checker registered for ct-events — RegisterRelatedDemo(\"ct-events\", ...) was not called")
	}

	// Load all ct-events demo fixtures once and build an ID→Resource index.
	fixtures, ok := demo.GetResources("ct-events")
	if !ok || len(fixtures) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}
	byID := make(map[string]resource.Resource, len(fixtures))
	for _, f := range fixtures {
		byID[f.ID] = f
	}

	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("Case%s", tc.label), func(t *testing.T) {
			res, found := byID[tc.eventID]
			if !found {
				t.Fatalf("fixture %q not found in demo.GetResources(\"ct-events\")", tc.eventID)
			}

			// Invoke the demo checker.
			results := demoFn(res)

			// Build a TargetType → count map from actual results.
			got := make(map[string]int, len(results))
			for _, r := range results {
				got[r.TargetType] = r.Count
			}

			// Assert each expected group individually so every failing
			// (case, group) pair is reported, not just the first.
			for targetType, wantCount := range tc.counts {
				targetType := targetType
				wantCount := wantCount
				t.Run(targetType, func(t *testing.T) {
					gotCount, present := got[targetType]
					if !present {
						t.Errorf("event %s (%s): target type %q absent from demo checker results; want count %d",
							tc.eventID, tc.label, targetType, wantCount)
						return
					}
					if gotCount != wantCount {
						t.Errorf("event %s (%s): target type %q: got count %d, want %d",
							tc.eventID, tc.label, targetType, gotCount, wantCount)
					}
				})
			}
		})
	}
}
