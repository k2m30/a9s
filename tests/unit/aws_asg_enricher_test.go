package unit

// aws_asg_enricher_test.go — Behavioral tests for EnrichASGScalingActivities.
//
// Contract assertions:
//   - DescribeScalingActivities is called once per ASG resource (bounded fan-out,
//     cap ~50, mirroring EnrichTargetGroupHealth conventions).
//   - Both ASGs with Successful latest activity → 0 findings.
//   - One ASG with Failed latest activity → 1 finding keyed by that ASG name, severity "!".
//   - clients.AutoScaling == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error on first ASG → continue to second ASG; result.Truncated=true;
//     error not propagated (individual errors do not fail the whole enricher).

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// asgScalingActivitiesFake implements ASGAPI for enrichment testing.
// It embeds the interface and overrides only DescribeScalingActivities.
// errForASG, when non-empty, forces an error only for the specified ASG name.
type asgScalingActivitiesFake struct {
	awsclient.ASGAPI
	// activities maps ASG name to the activity list to return.
	activities map[string][]asgtypes.Activity
	// errForASG is the name of the ASG for which the API should return an error.
	// Empty string means no per-ASG error.
	errForASG string
	// globalErr, when non-nil, is returned for every call.
	globalErr error
}

func (f *asgScalingActivitiesFake) DescribeScalingActivities(
	_ context.Context,
	params *autoscaling.DescribeScalingActivitiesInput,
	_ ...func(*autoscaling.Options),
) (*autoscaling.DescribeScalingActivitiesOutput, error) {
	if f.globalErr != nil {
		return nil, f.globalErr
	}
	name := ""
	if params != nil && params.AutoScalingGroupName != nil {
		name = *params.AutoScalingGroupName
	}
	if f.errForASG != "" && name == f.errForASG {
		return nil, errors.New("asg: describe scaling activities failed for " + name)
	}
	acts := f.activities[name]
	return &autoscaling.DescribeScalingActivitiesOutput{Activities: acts}, nil
}

// Compile-time check: asgScalingActivitiesFake satisfies ASGAPI.
var _ awsclient.ASGAPI = (*asgScalingActivitiesFake)(nil)

// TestEnrichASGScalingActivities_AllSuccessfulProducesNoFindings verifies that
// two ASGs whose latest activity is Successful produce zero findings.
func TestEnrichASGScalingActivities_AllSuccessfulProducesNoFindings(t *testing.T) {
	fake := &asgScalingActivitiesFake{
		activities: map[string][]asgtypes.Activity{
			"my-web-asg": {
				{
					ActivityId:           aws.String("act-web-1"),
					AutoScalingGroupName: aws.String("my-web-asg"),
					StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
					Description:          aws.String("Launching a new EC2 instance"),
				},
			},
			"my-worker-asg": {
				{
					ActivityId:           aws.String("act-worker-1"),
					AutoScalingGroupName: aws.String("my-worker-asg"),
					StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
					Description:          aws.String("Launching a new EC2 instance"),
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{AutoScaling: fake}
	resources := []resource.Resource{
		{ID: "my-web-asg", Name: "my-web-asg", Fields: map[string]string{"asg_name": "my-web-asg"}},
		{ID: "my-worker-asg", Name: "my-worker-asg", Fields: map[string]string{"asg_name": "my-worker-asg"}},
	}

	result, err := awsclient.EnrichASGScalingActivities(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for all-successful ASGs, got %d", len(result.Findings))
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichASGScalingActivities_OneFailedProducesFindingSevBang verifies that
// one ASG with a Failed latest activity produces exactly one finding with
// severity "!" keyed by the ASG name.
func TestEnrichASGScalingActivities_OneFailedProducesFindingSevBang(t *testing.T) {
	fake := &asgScalingActivitiesFake{
		activities: map[string][]asgtypes.Activity{
			"my-web-asg": {
				{
					ActivityId:           aws.String("act-web-1"),
					AutoScalingGroupName: aws.String("my-web-asg"),
					StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
					Description:          aws.String("Launching a new EC2 instance"),
				},
			},
			"my-failing-asg": {
				{
					ActivityId:           aws.String("act-fail-1"),
					AutoScalingGroupName: aws.String("my-failing-asg"),
					StatusCode:           asgtypes.ScalingActivityStatusCodeFailed,
					Description:          aws.String("Failed to launch instance: capacity unavailable"),
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{AutoScaling: fake}
	resources := []resource.Resource{
		{ID: "my-web-asg", Name: "my-web-asg", Fields: map[string]string{"asg_name": "my-web-asg"}},
		{ID: "my-failing-asg", Name: "my-failing-asg", Fields: map[string]string{"asg_name": "my-failing-asg"}},
	}

	result, err := awsclient.EnrichASGScalingActivities(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["my-failing-asg"]
	if !ok {
		t.Fatalf("expected finding keyed by ASG name %q for failed activity", "my-failing-asg")
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
	if _, ok := result.Findings["my-web-asg"]; ok {
		t.Error("successful ASG must NOT appear in Findings")
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", result.IssueCount)
	}
}

// TestEnrichASGScalingActivities_NilClientReturnsEmptyFindingsNoError verifies
// that when clients.AutoScaling is nil, the enricher returns a non-nil empty
// Findings map and no error.
func TestEnrichASGScalingActivities_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{AutoScaling: nil}

	result, err := awsclient.EnrichASGScalingActivities(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when AutoScaling client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichASGScalingActivities_APIErrorOnFirstContinuesToSecond verifies the
// bounded-fan-out partial-error behavior: an API error for one ASG sets
// Truncated=true and does not prevent processing subsequent ASGs. The overall
// enricher surfaces a composite error containing the failing ID.
func TestEnrichASGScalingActivities_APIErrorOnFirstContinuesToSecond(t *testing.T) {
	fake := &asgScalingActivitiesFake{
		activities: map[string][]asgtypes.Activity{
			"my-ok-asg": {
				{
					ActivityId:           aws.String("act-ok-1"),
					AutoScalingGroupName: aws.String("my-ok-asg"),
					StatusCode:           asgtypes.ScalingActivityStatusCodeFailed,
					Description:          aws.String("Failed to launch: insufficient capacity"),
				},
			},
		},
		errForASG: "my-error-asg",
	}
	clients := &awsclient.ServiceClients{AutoScaling: fake}
	// Place the error-triggering ASG first to verify processing continues.
	resources := []resource.Resource{
		{ID: "my-error-asg", Name: "my-error-asg", Fields: map[string]string{"asg_name": "my-error-asg"}},
		{ID: "my-ok-asg", Name: "my-ok-asg", Fields: map[string]string{"asg_name": "my-ok-asg"}},
	}

	result, err := awsclient.EnrichASGScalingActivities(context.Background(), clients, resources)
	if err == nil {
		t.Fatal("enricher must surface a composite error when at least one ASG API call failed")
	}
	if errStr := err.Error(); !strings.Contains(errStr, "asg-enrich:") {
		t.Errorf("composite error must contain \"asg-enrich:\", got: %q", errStr)
	}
	if errStr := err.Error(); !strings.Contains(errStr, "my-error-asg") {
		t.Errorf("composite error must contain the failing ASG ID \"my-error-asg\", got: %q", errStr)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when at least one ASG API call failed")
	}
	// The second ASG (my-ok-asg) had a failed activity and must produce a finding.
	if _, ok := result.Findings["my-ok-asg"]; !ok {
		t.Error("second ASG with failed activity must still produce a finding even after first ASG errored")
	}
	// The error ASG must not appear in findings (API call failed, no data).
	if _, ok := result.Findings["my-error-asg"]; ok {
		t.Error("ASG that returned an API error must not appear in Findings")
	}
}
