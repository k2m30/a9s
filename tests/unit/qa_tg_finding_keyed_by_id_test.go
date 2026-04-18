package unit

// qa_tg_finding_keyed_by_id_test.go — Regression: EnrichTargetGroupHealth keys findings by r.ID.
//
// The existing TestEnrichTargetGroupHealth_FindingKeyedByARN uses a resource where
// r.Name is empty so it cannot distinguish between keying by ID vs by Name.
// This test uses r.ID != r.Name to pin that the key is r.ID (the TG ARN).

import (
	"context"
	"testing"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestEnrichTargetGroupHealth_FindingKeyedByID_NotByName verifies findings are keyed
// by r.ID (ARN) when r.ID != r.Name. Regresses if the enricher switches to findings[r.Name].
func TestEnrichTargetGroupHealth_FindingKeyedByID_NotByName(t *testing.T) {
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
	// r.ID is the ARN; r.Name is the human-readable TG name.
	resources := []resource.Resource{{ID: tgARN, Name: tgName}}

	result, err := awsclient.EnrichTargetGroupHealth(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Finding must be keyed by r.ID (the ARN).
	if _, ok := result.Findings[tgARN]; !ok {
		t.Errorf("finding must be keyed by r.ID=%q — was the key changed from ARN to name?", tgARN)
	}
	if _, ok := result.Findings[tgName]; ok {
		t.Errorf("finding must NOT be keyed by r.Name=%q — enricher must use r.ID as the key", tgName)
	}
}
