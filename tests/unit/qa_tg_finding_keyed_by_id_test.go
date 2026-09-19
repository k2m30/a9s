package unit

// EnrichTargetGroupHealth keys findings by
// r.ID (the bare TG name the tg fetcher sets) and calls DescribeTargetHealth
// with r.Fields["target_group_arn"]: AWS requires the ARN and answers the bare
// name with "target group not found".

import (
	"context"
	"testing"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestEnrichTargetGroupHealth_UsesARNFromFields verifies the enricher calls
// DescribeTargetHealth with the ARN from Fields["target_group_arn"], not r.ID,
// and keys findings back by r.ID (the bare name set by the tg fetcher).
func TestEnrichTargetGroupHealth_UsesARNFromFields(t *testing.T) {
	const tgARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/prod-api-tg/deadbeef1234"
	const tgName = "prod-api-tg"

	fake := &tgHealthFake{
		outputs: map[string]*elbv2.DescribeTargetHealthOutput{
			tgARN: {
				TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
					{TargetHealth: &elbtypes.TargetHealth{
						State:  elbtypes.TargetHealthStateEnumUnhealthy,
						Reason: elbtypes.TargetHealthReasonEnumRegistrationInProgress,
					}},
					{TargetHealth: &elbtypes.TargetHealth{State: elbtypes.TargetHealthStateEnumHealthy}},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ELBv2: fake}
	// Mirrors what tg.go fetcher emits: ID/Name = bare TG name, ARN in Fields.
	resources := []resource.Resource{{
		ID:     tgName,
		Name:   tgName,
		Fields: map[string]string{"target_group_arn": tgARN},
	}}

	result, err := awsclient.EnrichTargetGroupHealth(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Findings are keyed by r.ID (the bare name) so row resolution can join
	// against the resource list.
	if _, ok := result.Findings[tgName]; !ok {
		t.Errorf("finding must be keyed by r.ID=%q (bare TG name)", tgName)
	}
	if _, ok := result.Findings[tgARN]; ok {
		t.Errorf("finding must NOT be keyed by ARN=%q — keys must match r.ID", tgARN)
	}
}
