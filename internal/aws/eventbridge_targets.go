package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("eb_rule_targets", []string{
		"target_id", "target_arn", "role_arn", "resource_type_name", "input_summary",
	})

	resource.RegisterPaginatedChild("eb_rule_targets", func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEventBridgeRuleTargets(ctx, c.EventBridge, parentCtx, continuationToken)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "EB Rule Targets",
		ShortName: "eb_rule_targets",
		Columns:   resource.EbRuleTargetColumns(),
		CopyField: "target_arn",
	})
}

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
		resources = append(resources, convertEventBridgeTarget(target))
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

// convertEventBridgeTarget converts a single EventBridge Target into a generic Resource.
func convertEventBridgeTarget(target ebtypes.Target) resource.Resource {
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
		},
		RawStruct: target,
	}
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
