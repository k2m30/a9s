// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchTargetHealth calls the ELBv2 DescribeTargetHealth API for a given
// target group ARN and converts the response into a FetchResult.
// No pagination — a single API call returns all targets.
func FetchTargetHealth(ctx context.Context, api ELBv2DescribeTargetHealthAPI, targetGroupArn string, continuationToken string) (resource.FetchResult, error) {
	output, err := api.DescribeTargetHealth(ctx, &elbv2.DescribeTargetHealthInput{
		TargetGroupArn: &targetGroupArn,
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching target health: %w", err)
	}

	var resources []resource.Resource

	for _, thd := range output.TargetHealthDescriptions {
		resources = append(resources, convertTargetHealth(thd, targetGroupArn))
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			TotalHint:   len(resources),
			PageSize:    len(resources),
		},
	}, nil
}

// convertTargetHealth converts a single ELBv2 TargetHealthDescription into a
// generic Resource. targetGroupArn is threaded through to
// Fields["target_group_arn"] so the console-link builder can deep-link to
// the parent target group's page.
func convertTargetHealth(thd elbv2types.TargetHealthDescription, targetGroupArn string) resource.Resource {
	targetID := ""
	port := ""
	az := ""

	if thd.Target != nil {
		if thd.Target.Id != nil {
			targetID = *thd.Target.Id
		}
		if thd.Target.Port != nil {
			port = fmt.Sprintf("%d", *thd.Target.Port)
		}
		if thd.Target.AvailabilityZone != nil {
			az = *thd.Target.AvailabilityZone
		}
	}

	health := ""
	reason := ""
	description := ""

	if thd.TargetHealth != nil {
		health = string(thd.TargetHealth.State)
		reason = string(thd.TargetHealth.Reason)
		if thd.TargetHealth.Description != nil {
			description = *thd.TargetHealth.Description
		}
	}
	reasonHuman := humanizeTargetHealthReason(reason)

	return resource.Resource{
		ID:   targetID,
		Name: targetID,
		Fields: map[string]string{
			"target_id":        targetID,
			"port":             port,
			"az":               az,
			"status":           health,
			"health":           health,
			"reason":           reason,
			"reason_human":     reasonHuman,
			"description":      description,
			"target_group_arn": targetGroupArn,
		},
		Findings:  targetHealthFindings(health, reasonHuman),
		RawStruct: thd,
	}
}

// humanizeTargetHealthReason converts a raw elbv2types.TargetHealthReasonEnum
// (e.g. "Target.FailedHealthChecks") into an operator-readable phrase (e.g.
// "failed health checks"). The enum's leading "Target."/"Elb." namespace
// prefix carries no operator-facing meaning, so it is stripped before the
// remaining CamelCase segment is run through domain.HumanizeStatusPhrase.
func humanizeTargetHealthReason(rawReason string) string {
	if rawReason == "" {
		return ""
	}
	_, rest, found := strings.Cut(rawReason, ".")
	if !found {
		return domain.HumanizeStatusPhrase(rawReason)
	}
	return domain.HumanizeStatusPhrase(rest)
}

// targetHealthFindings maps a TargetHealthStateEnum value to the Wave-1
// Finding(s) that communicate its cause. healthy/unused targets carry no
// finding — the row renders with the default healthy color.
func targetHealthFindings(health, humanizedReason string) []domain.Finding {
	phrase := humanizedReason
	switch health {
	case string(elbv2types.TargetHealthStateEnumUnhealthy), string(elbv2types.TargetHealthStateEnumUnhealthyDraining):
		if phrase == "" {
			phrase = "unhealthy"
		}
		return []domain.Finding{{Code: CodeTGHealthUnhealthy, Phrase: phrase, Severity: domain.SevBroken, Source: "wave1"}}
	case string(elbv2types.TargetHealthStateEnumUnavailable):
		if phrase == "" {
			phrase = "target unavailable"
		}
		return []domain.Finding{{Code: CodeTGHealthUnavailable, Phrase: phrase, Severity: domain.SevWarn, Source: "wave1"}}
	case string(elbv2types.TargetHealthStateEnumDraining):
		if phrase == "" {
			phrase = "draining"
		}
		return []domain.Finding{{Code: CodeTGHealthDraining, Phrase: phrase, Severity: domain.SevWarn, Source: "wave1"}}
	case string(elbv2types.TargetHealthStateEnumInitial):
		if phrase == "" {
			phrase = "initial health check pending"
		}
		return []domain.Finding{{Code: CodeTGHealthInitial, Phrase: phrase, Severity: domain.SevDim, Source: "wave1"}}
	default:
		return nil
	}
}
