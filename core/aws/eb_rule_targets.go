// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchEventBridgeRuleTargets calls the EventBridge ListTargetsByRule API
// and converts the response into a FetchResult. This is a single-call API,
// but uses FetchResult for consistency with the paginated child fetcher interface.
func FetchEventBridgeRuleTargets(
	ctx context.Context,
	api EventBridgeListTargetsByRuleAPI,
	parentCtx map[string]string,
	continuationToken string,
) (resource.FetchResult, error) {
	ruleName := parentCtx["rule_name"]
	eventBus := parentCtx["event_bus"]

	if ruleName == "" {
		return resource.FetchResult{}, nil
	}

	input := &eventbridge.ListTargetsByRuleInput{
		Rule:         &ruleName,
		EventBusName: &eventBus,
	}

	output, err := api.ListTargetsByRule(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing targets for rule %s: %w", ruleName, err)
	}

	resources := make([]resource.Resource, 0, len(output.Targets))

	for _, target := range output.Targets {
		resources = append(resources, convertEventBridgeTarget(target, ruleName, eventBus))
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

// convertEventBridgeTarget converts a single EventBridge Target into a
// generic Resource. ruleName and eventBus are threaded through to
// Fields["rule_name"]/Fields["event_bus"] so the console-link builder can
// deep-link to the parent rule's page.
func convertEventBridgeTarget(target ebtypes.Target, ruleName, eventBus string) resource.Resource {
	targetID := ""
	if target.Id != nil {
		targetID = *target.Id
	}

	targetArn := ""
	if target.Arn != nil {
		targetArn = *target.Arn
	}

	roleArn := ""
	if target.RoleArn != nil {
		roleArn = *target.RoleArn
	}

	return resource.Resource{
		ID:   targetID,
		Name: targetID,
		Fields: map[string]string{
			"target_id":          targetID,
			"target_arn":         targetArn,
			"role_arn":           roleArn,
			"resource_type_name": ArnToResourceName(targetArn),
			"input_summary":      ComputeInputSummary(target),
			"rule_name":          ruleName,
			"event_bus":          eventBus,
		},
		Findings:  ebRuleTargetFindings(target),
		RawStruct: target,
	}
}

// ebRuleTargetFindings flags a target with no DeadLetterConfig. Without a
// DLQ, an event this target fails to process after exhausting its retry
// policy is silently dropped — there is no queue to inspect and no way to
// redrive it. ListTargetsByRule already returns DeadLetterConfig inline, so
// this is a Wave-1 (no extra API call) structural check.
func ebRuleTargetFindings(target ebtypes.Target) []domain.Finding {
	if target.DeadLetterConfig == nil || target.DeadLetterConfig.Arn == nil || *target.DeadLetterConfig.Arn == "" {
		return []domain.Finding{wave1Finding(CodeEBRuleTargetNoDLQ, domain.SevWarn)}
	}
	return nil
}

// arnServiceMap maps AWS service names from ARNs to friendly display names.
var arnServiceMap = map[string]string{
	"lambda":    "Lambda",
	"sqs":       "SQS",
	"states":    "SFN",
	"ecs":       "ECS",
	"sns":       "SNS",
	"events":    "EventBridge",
	"kinesis":   "Kinesis",
	"codebuild": "CodeBuild",
}

// ArnToResourceName parses an ARN and returns a "Service: name" string.
// For empty ARNs, returns "". For unparseable strings, returns the input as-is.
func ArnToResourceName(arn string) string {
	if arn == "" {
		return ""
	}

	// ARN format: arn:partition:service:region:account:resource
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) < 6 || parts[0] != "arn" {
		return arn
	}

	service := parts[2]
	resourcePart := parts[5]

	// Extract the resource name: prefer splitting on ":" first, then "/" only
	// if no ":" was found. This preserves path-like names (e.g. /aws/lambda/my-func).
	name := resourcePart
	if idx := strings.LastIndex(name, ":"); idx >= 0 {
		name = name[idx+1:]
	} else if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	friendlyName, ok := arnServiceMap[service]
	if ok {
		return friendlyName + ": " + name
	}

	return service + ": " + name
}

// ComputeInputSummary returns a human-readable summary of the target's input
// configuration. Priority: InputTransformer → Input (truncated) → InputPath → em-dash.
func ComputeInputSummary(target ebtypes.Target) string {
	if target.InputTransformer != nil {
		return "transformer"
	}

	if target.Input != nil && *target.Input != "" {
		input := *target.Input
		if len(input) > 34 {
			return input[:34] + "..."
		}
		return input
	}

	if target.InputPath != nil && *target.InputPath != "" {
		return *target.InputPath
	}

	return "\u2014"
}
