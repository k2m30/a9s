package unit

// enrichment_tg_findings_test.go — Behavioral tests for EnrichTargetGroupHealth.
//
// Contract assertions (docs/attention-signals.md `tg` Wave 2 row):
//   - Returns EnricherResult.Findings keyed by r.ID (target group name set by tg fetcher).
//   - DescribeTargetHealth is called with r.Fields["target_group_arn"] (full ARN required by AWS).
//   - Only literal TargetHealth.State == "unhealthy" counts toward the unhealthy
//     numerator — "initial", "draining", "unused", "unavailable", and
//     "unhealthy.draining" targets are NOT unhealthy and must not trigger a
//     finding by themselves.
//   - Severity is graduated: any target State=="unhealthy" (numerator > 0) →
//     "~" (SevWarn); EVERY target reporting State=="unhealthy" (numerator ==
//     denominator, i.e. no healthy/initial/draining/unused/unavailable target
//     present) → "!" (SevBroken). A single non-"unhealthy" target in the mix
//     blocks the "!" escalation even if every other target is unhealthy.
//   - Summary format: "unhealthy targets: X/Y".
//   - IssueCount = len(Findings) (one entry per TG with any unhealthy targets).
//   - Truncated = true when len(resources) > EnrichmentCap.
//   - TG with zero literally-unhealthy targets must NOT appear in Findings.
//   - Empty resources slice → non-nil empty Findings map.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
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
	f := result.Findings[tgName][0]
	// Summary must be "unhealthy targets: 2/3"
	if !strings.HasPrefix(f.Phrase, "unhealthy targets:") {
		t.Errorf("summary %q must start with %q", f.Phrase, "unhealthy targets:")
	}
	if !strings.Contains(f.Phrase, "2/3") {
		t.Errorf("summary %q must contain %q (2 of 3 unhealthy)", f.Phrase, "2/3")
	}
	// 2 of 3 unhealthy is NOT "every target unhealthy" — must be Warning, not
	// Broken (docs/attention-signals.md `tg` Wave 2: "any target unhealthy ->
	// Warning; all unhealthy -> Broken"). Current code always stamps "!".
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v (2/3 unhealthy is partial, not all)", f.Severity, domain.SevWarn)
	}
}

// TestEnrichTargetGroupHealth_NonUnhealthyStatesExcluded pins that targets in
// "initial", "draining", or "unused" state are NOT counted as unhealthy — only
// literal State=="unhealthy" is. A TG with zero literally-unhealthy targets
// must not appear in Findings at all, even though every target here is
// non-"healthy".
//
// Current code treats State != Healthy as unhealthy, so this TG would
// (wrongly) get a finding today.
func TestEnrichTargetGroupHealth_NonUnhealthyStatesExcluded(t *testing.T) {
	tgName := "mixed-nonunhealthy-tg"
	tgARN := "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/mixed-nonunhealthy-tg/333"
	fake := &tgHealthFake{
		outputs: map[string]*elbv2.DescribeTargetHealthOutput{
			tgARN: {
				TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
					tgHealthDesc(elbtypes.TargetHealthStateEnumHealthy),
					tgHealthDesc(elbtypes.TargetHealthStateEnumInitial),
					tgHealthDesc(elbtypes.TargetHealthStateEnumDraining),
					tgHealthDesc(elbtypes.TargetHealthStateEnumUnused),
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
		t.Errorf("TG with [healthy, initial, draining, unused] targets (zero literally-unhealthy) must NOT appear in Findings; got %v", result.Findings[tgName])
	}
}

// TestEnrichTargetGroupHealth_AllUnhealthyIsBroken pins the Broken tier: every
// target reporting State=="unhealthy" must produce a finding with severity "!"
// (SevBroken).
func TestEnrichTargetGroupHealth_AllUnhealthyIsBroken(t *testing.T) {
	tgName := "all-unhealthy-tg"
	tgARN := "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/all-unhealthy-tg/555"
	fake := &tgHealthFake{
		outputs: map[string]*elbv2.DescribeTargetHealthOutput{
			tgARN: {
				TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
					tgHealthDesc(elbtypes.TargetHealthStateEnumUnhealthy),
					tgHealthDesc(elbtypes.TargetHealthStateEnumUnhealthy),
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
	fs, ok := result.Findings[tgName]
	if !ok {
		t.Fatalf("expected a finding for %q (all targets unhealthy)", tgName)
	}
	if fs[0].Severity != domain.SevBroken {
		t.Errorf("severity = %v, want %v (every target unhealthy)", fs[0].Severity, domain.SevBroken)
	}
}

// TestEnrichTargetGroupHealth_MixedUnhealthyAndNonReportingIsWarning pins the
// "honest reading" of the all-unhealthy escalation: a target group with one
// unhealthy target and one "initial" target (zero healthy targets) must stay
// at Warning, NOT escalate to Broken, because not every target reporting is
// unhealthy — the "initial" target breaks the "all unhealthy" condition even
// though it isn't "healthy" either.
func TestEnrichTargetGroupHealth_MixedUnhealthyAndNonReportingIsWarning(t *testing.T) {
	tgName := "mixed-unhealthy-initial-tg"
	tgARN := "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/mixed-unhealthy-initial-tg/666"
	fake := &tgHealthFake{
		outputs: map[string]*elbv2.DescribeTargetHealthOutput{
			tgARN: {
				TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
					tgHealthDesc(elbtypes.TargetHealthStateEnumUnhealthy),
					tgHealthDesc(elbtypes.TargetHealthStateEnumInitial),
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
	fs, ok := result.Findings[tgName]
	if !ok {
		t.Fatalf("expected a finding for %q (1 unhealthy target present)", tgName)
	}
	if fs[0].Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v (unhealthy+initial: not every target is unhealthy, must not escalate to Broken)", fs[0].Severity, domain.SevWarn)
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
