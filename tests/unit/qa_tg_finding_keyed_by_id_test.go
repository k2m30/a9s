package unit

// qa_tg_finding_keyed_by_id_test.go — Regression: EnrichTargetGroupHealth keys findings by r.ID
// (the bare TG name set by the tg fetcher) and calls DescribeTargetHealth with
// r.Fields["target_group_arn"] (the full ARN, which is what AWS requires).
//
// This regression exists because an earlier version passed r.ID directly to
// DescribeTargetHealth — which produced "target group not found" against both real AWS
// and the demo fake, since r.ID is the bare name and AWS requires the ARN.

import (
	"context"
	"testing"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
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

	// Finding must be keyed by r.ID (the bare name), so downstream consumers
	// (S2/S3/S4 row resolution) can join against the resource list.
	if _, ok := result.Findings[tgName]; !ok {
		t.Errorf("finding must be keyed by r.ID=%q (bare TG name)", tgName)
	}
	if _, ok := result.Findings[tgARN]; ok {
		t.Errorf("finding must NOT be keyed by ARN=%q — keys must match r.ID", tgARN)
	}
}
