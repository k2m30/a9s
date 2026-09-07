package unit_test

// finding_row_cap_render_test.go — the closing "… +K more" row is a row, so
// the detail view renders it with no special case. This lives in the external
// test package because the real detail-render drive (detailAttentionValuesFor)
// does; the sink-level assertions on the same cap are in finding_row_cap_test.go.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	a9sruntime "github.com/k2m30/a9s/v3/core/runtime"
)

func TestFindingRowCap_OverflowRowRendersInTheDetailAttentionSection(t *testing.T) {
	const arn = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/render-tg/1d4b7f0a3c6e2958"
	hidden := 5
	targets := awsclient.FindingRowCap + hidden

	descs := make([]elbtypes.TargetHealthDescription, 0, targets)
	for i := range targets {
		descs = append(descs, elbtypes.TargetHealthDescription{
			Target: &elbtypes.TargetDescription{
				Id:   aws.String(fmt.Sprintf("i-0c3d2e1f0a9b%04d", i)),
				Port: aws.Int32(8080),
			},
			TargetHealth: &elbtypes.TargetHealth{
				State:  elbtypes.TargetHealthStateEnumUnhealthy,
				Reason: elbtypes.TargetHealthReasonEnumFailedHealthChecks,
			},
		})
	}
	fake := &fakeELBv2CR{describeTargetHealthOutput: &elbv2.DescribeTargetHealthOutput{
		TargetHealthDescriptions: descs,
	}}

	rows := []resource.Resource{{
		ID:     "render-tg",
		Name:   "render-tg",
		Fields: map[string]string{"target_group_arn": arn},
	}}

	result, err := awsclient.EnrichTargetGroupHealth(context.Background(),
		&awsclient.ServiceClients{ELBv2: fake}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichTargetGroupHealth: unexpected error: %v", err)
	}

	var td resource.ResourceTypeDef
	for _, d := range resource.AllResourceTypes() {
		if d.ShortName == "tg" {
			td = d
			break
		}
	}
	if td.ShortName == "" {
		t.Fatalf("no registered resource type \"tg\"")
	}

	row := rows[0]
	a9sruntime.ApplyWave2ToRow(&row, td, result.Findings, result.AttentionDetails)

	want := fmt.Sprintf("… +%d more", hidden)
	values := detailAttentionValuesFor(t, row, "tg")
	for _, v := range values {
		if strings.Contains(v, want) {
			return
		}
	}
	t.Errorf("detail Attention section never rendered the overflow row %q; values=%q", want, values)
}
