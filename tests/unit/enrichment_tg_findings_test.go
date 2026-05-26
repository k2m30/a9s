package unit

// enrichment_tg_findings_test.go — Behavioral tests for EnrichTargetGroupHealth.
//
// Contract assertions (enricher-contract.md):
//   - Returns EnricherResult.Findings keyed by r.ID (target group name set by tg fetcher).
//   - DescribeTargetHealth is called with r.Fields["target_group_arn"] (full ARN required by AWS).
//   - Severity "!" for all findings.
//   - Summary format: "unhealthy targets: X/Y".
//   - IssueCount = len(Findings) (one entry per TG with any unhealthy targets).
//   - Truncated = true when len(resources) > EnrichmentCap.
//   - TG with all-healthy targets must NOT appear in Findings.
//   - Empty resources slice → non-nil empty Findings map.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// tgHealthFake implements the ELBv2API subset needed by EnrichTargetGroupHealth.
type tgHealthFake struct {
	awsclient.ELBv2API
	// outputs maps TG ARN → DescribeTargetHealthOutput
	outputs map[string]*elbv2.DescribeTargetHealthOutput
	err     error
}

func (f *tgHealthFake) DescribeTargetHealth(
	_ context.Context,
	params *elbv2.DescribeTargetHealthInput,
	_ ...func(*elbv2.Options),
) (*elbv2.DescribeTargetHealthOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	arn := aws.ToString(params.TargetGroupArn)
	if out, ok := f.outputs[arn]; ok {
		return out, nil
	}
	return &elbv2.DescribeTargetHealthOutput{}, nil
}

func tgHealthDesc(state elbtypes.TargetHealthStateEnum) elbtypes.TargetHealthDescription {
	return elbtypes.TargetHealthDescription{
		TargetHealth: &elbtypes.TargetHealth{State: state},
	}
}

// TestEnrichTargetGroupHealth_FindingKeyedByID verifies findings are keyed by r.ID (TG name)
// while DescribeTargetHealth is called with the ARN from Fields["target_group_arn"].
func TestEnrichTargetGroupHealth_FindingKeyedByID(t *testing.T) {
	tgName := "my-tg"
	tgARN := "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-tg/abc123"
	fake := &tgHealthFake{
		outputs: map[string]*elbv2.DescribeTargetHealthOutput{
			tgARN: {
				TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
					tgHealthDesc(elbtypes.TargetHealthStateEnumUnhealthy),
					tgHealthDesc(elbtypes.TargetHealthStateEnumHealthy),
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ELBv2: fake}
	resources := []resource.Resource{{
		ID:     tgName,
		Fields: map[string]string{"target_group_arn": tgARN},
	}}

	result, err := awsclient.EnrichTargetGroupHealth(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings[tgName]; !ok {
		gotKeys := make([]string, 0, len(result.Findings))
		for k := range result.Findings {
			gotKeys = append(gotKeys, k)
		}
		t.Errorf("expected finding keyed by TG name %q (got keys: %v)", tgName, gotKeys)
	}
}

// TestEnrichTargetGroupHealth_SummaryUnhealthyXofY verifies the summary format.
func TestEnrichTargetGroupHealth_SummaryUnhealthyXofY(t *testing.T) {
	tgName := "sum-tg"
	tgARN := "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/sum-tg/111"
	fake := &tgHealthFake{
		outputs: map[string]*elbv2.DescribeTargetHealthOutput{
			tgARN: {
				TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
					tgHealthDesc(elbtypes.TargetHealthStateEnumUnhealthy),
					tgHealthDesc(elbtypes.TargetHealthStateEnumUnhealthy),
					tgHealthDesc(elbtypes.TargetHealthStateEnumHealthy),
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ELBv2: fake}
	resources := []resource.Resource{{
		ID:     tgName,
		Fields: map[string]string{"target_group_arn": tgARN},
	}}

	result, err := awsclient.EnrichTargetGroupHealth(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := result.Findings[tgName]
	// Summary must be "unhealthy targets: 2/3"
	if !strings.HasPrefix(f.Phrase, "unhealthy targets:") {
		t.Errorf("summary %q must start with %q", f.Phrase, "unhealthy targets:")
	}
	if !strings.Contains(f.Phrase, "2/3") {
		t.Errorf("summary %q must contain %q (2 of 3 unhealthy)", f.Phrase, "2/3")
	}
}

// TestEnrichTargetGroupHealth_AllHealthyExcluded verifies TGs with all-healthy targets
// do not appear in Findings.
func TestEnrichTargetGroupHealth_AllHealthyExcluded(t *testing.T) {
	tgName := "ok-tg"
	tgARN := "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/ok-tg/222"
	fake := &tgHealthFake{
		outputs: map[string]*elbv2.DescribeTargetHealthOutput{
			tgARN: {
				TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
					tgHealthDesc(elbtypes.TargetHealthStateEnumHealthy),
					tgHealthDesc(elbtypes.TargetHealthStateEnumHealthy),
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{ELBv2: fake}
	resources := []resource.Resource{{
		ID:     tgName,
		Fields: map[string]string{"target_group_arn": tgARN},
	}}

	result, err := awsclient.EnrichTargetGroupHealth(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings[tgName]; ok {
		t.Error("all-healthy TG must NOT appear in Findings")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 for all-healthy TG", result.IssueCount)
	}
}

// TestEnrichTargetGroupHealth_TruncatedWhenResourcesExceedCap verifies Truncated=true.
func TestEnrichTargetGroupHealth_TruncatedWhenResourcesExceedCap(t *testing.T) {
	count := awsclient.EnrichmentCap + 1
	resources := make([]resource.Resource, count)
	outputs := make(map[string]*elbv2.DescribeTargetHealthOutput, count)
	for i := range count {
		name := fmt.Sprintf("tg-%03d", i)
		arn := fmt.Sprintf("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/tg-%03d/%03d", i, i)
		resources[i] = resource.Resource{
			ID:     name,
			Fields: map[string]string{"target_group_arn": arn},
		}
		outputs[arn] = &elbv2.DescribeTargetHealthOutput{
			TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
				tgHealthDesc(elbtypes.TargetHealthStateEnumHealthy),
			},
		}
	}
	fake := &tgHealthFake{outputs: outputs}
	clients := &awsclient.ServiceClients{ELBv2: fake}

	result, err := awsclient.EnrichTargetGroupHealth(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Errorf("Truncated must be true when len(resources)=%d > EnrichmentCap=%d",
			count, awsclient.EnrichmentCap)
	}
}

// TestEnrichTargetGroupHealth_EmptyResourcesReturnsEmptyFindings verifies nil/empty
// resources returns non-nil empty Findings.
func TestEnrichTargetGroupHealth_EmptyResourcesReturnsEmptyFindings(t *testing.T) {
	fake := &tgHealthFake{outputs: map[string]*elbv2.DescribeTargetHealthOutput{}}
	clients := &awsclient.ServiceClients{ELBv2: fake}

	result, err := awsclient.EnrichTargetGroupHealth(context.Background(), clients, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil on empty resources")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}
