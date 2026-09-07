// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

// phrase_tg_rows_test.go — a target group names every failing target, and says
// nothing it cannot say.
//
// The ratio belongs to the phrase, so a row repeating it added no line the
// reader could act on. What the phrase cannot carry is which targets are down
// and why each one is: those are one row per failing target. The reason in
// particular was first-item-wins one level below the phrase — whichever
// unhealthy target came first supplied the only reason any of them got.
//
// The floor is the case where a target answers with neither an identity nor a
// reason. There is nothing to print, so no row is invented; the finding stays
// readable because its code declares an operator sentence.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const phraseTGARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-api-tg/abc123"

// phraseTGTarget builds one health description for a registered target.
func phraseTGTarget(id string, port int32, state elbtypes.TargetHealthStateEnum, reason elbtypes.TargetHealthReasonEnum) elbtypes.TargetHealthDescription {
	return elbtypes.TargetHealthDescription{
		Target:       &elbtypes.TargetDescription{Id: aws.String(id), Port: aws.Int32(port)},
		TargetHealth: &elbtypes.TargetHealth{State: state, Reason: reason},
	}
}

// phraseTGEnrich runs the real enricher over one target group.
func phraseTGEnrich(t *testing.T, descs []elbtypes.TargetHealthDescription) awsclient.IssueEnricherResult {
	t.Helper()
	clients := &awsclient.ServiceClients{ELBv2: &tgHealthFake{
		outputs: map[string]*elbv2.DescribeTargetHealthOutput{
			phraseTGARN: {TargetHealthDescriptions: descs},
		},
	}}
	result, err := awsclient.EnrichTargetGroupHealth(context.Background(), clients,
		[]resource.Resource{{ID: "acme-api-tg", Fields: map[string]string{"target_group_arn": phraseTGARN}}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result
}

func TestTGTwoUnhealthyTargetsAreBothNamedWithTheirOwnReason(t *testing.T) {
	const code domain.FindingCode = "tg.unhealthy-targets"
	result := phraseTGEnrich(t, []elbtypes.TargetHealthDescription{
		phraseTGTarget("i-0aaa111111111111a", 8080,
			elbtypes.TargetHealthStateEnumUnhealthy, elbtypes.TargetHealthReasonEnumFailedHealthChecks),
		phraseTGTarget("i-0bbb222222222222b", 8080,
			elbtypes.TargetHealthStateEnumUnhealthy, elbtypes.TargetHealthReasonEnumTimeout),
		phraseTGTarget("i-0ccc333333333333c", 8080,
			elbtypes.TargetHealthStateEnumHealthy, ""),
	})

	fs := result.Findings["acme-api-tg"]
	if len(fs) != 1 || fs[0].Code != code {
		t.Fatalf("findings = %v, want exactly one %s", fs, code)
	}
	// The ratio is the whole group's measurement, so it stays in the phrase the
	// code declares; naming a target there would pick one of the two.
	if want := "unhealthy targets: 2/3"; fs[0].Phrase != want {
		t.Errorf("Phrase = %q, want %q", fs[0].Phrase, want)
	}
	if fs[0].Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn — one healthy target keeps this short of an outage", fs[0].Severity)
	}

	rows := result.AttentionDetails["acme-api-tg"][code].Rows
	if len(rows) != 2 {
		t.Fatalf("rows = %v, want one per unhealthy target (2)", rows)
	}
	for i, want := range []string{
		"i-0aaa111111111111a:8080 — failed health checks",
		"i-0bbb222222222222b:8080 — timeout",
	} {
		if rows[i].Label != "Unhealthy target" {
			t.Errorf("rows[%d].Label = %q, want %q", i, rows[i].Label, "Unhealthy target")
		}
		if rows[i].Value != want {
			t.Errorf("rows[%d].Value = %q, want %q — each target reports its own reason; one "+
				"target's reason must not stand for the others", i, rows[i].Value, want)
		}
	}
}

func TestTGHealthyTargetGroupSaysNothing(t *testing.T) {
	result := phraseTGEnrich(t, []elbtypes.TargetHealthDescription{
		phraseTGTarget("i-0aaa111111111111a", 8080, elbtypes.TargetHealthStateEnumHealthy, ""),
		phraseTGTarget("i-0bbb222222222222b", 8080, elbtypes.TargetHealthStateEnumHealthy, ""),
	})
	if len(result.Findings) != 0 {
		t.Errorf("findings = %v, want none — every target is passing its health check", result.Findings)
	}
}

func TestTGUnnamedUnhealthyTargetYieldsNoRowAndNoBareFinding(t *testing.T) {
	const code domain.FindingCode = "tg.unhealthy-targets"
	result := phraseTGEnrich(t, []elbtypes.TargetHealthDescription{
		// AWS answered with a state and nothing else: no identity to print and
		// no reason to give.
		{TargetHealth: &elbtypes.TargetHealth{State: elbtypes.TargetHealthStateEnumUnhealthy}},
		phraseTGTarget("i-0bbb222222222222b", 8080, elbtypes.TargetHealthStateEnumHealthy, ""),
	})

	fs := result.Findings["acme-api-tg"]
	if len(fs) != 1 || fs[0].Code != code {
		t.Fatalf("findings = %v, want exactly one %s — a target that cannot be named is still down",
			fs, code)
	}
	if want := "unhealthy targets: 1/2"; fs[0].Phrase != want {
		t.Errorf("Phrase = %q, want %q — the count does not depend on being able to name the target",
			fs[0].Phrase, want)
	}
	if rows := result.AttentionDetails["acme-api-tg"][code].Rows; len(rows) != 0 {
		t.Errorf("rows = %v, want none — there is nothing to print, and a row with an empty value "+
			"is worse than no row", rows)
	}
	if fs[0].Detail == "" {
		t.Error("Detail is empty — with no rows the detail view would render the phrase and " +
			"nothing under it")
	}
}
