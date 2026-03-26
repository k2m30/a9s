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

	resource.RegisterChildFetcher("eb_rule_targets", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEventBridgeRuleTargets(ctx, c.EventBridge, parentCtx)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "EB Rule Targets",
		ShortName: "eb_rule_targets",
		Columns:   resource.EbRuleTargetColumns(),
		CopyField: "target_arn",
	})
}

// FetchEventBridgeRuleTargets calls the EventBridge ListTargetsByRule API
// and converts the response into a slice of generic Resource structs.
// This is a child fetcher: it reads rule_name and event_bus from parentCtx.
func FetchEventBridgeRuleTargets(
	ctx context.Context,
	api EventBridgeListTargetsByRuleAPI,
	parentCtx map[string]string,
) ([]resource.Resource, error) {
	ruleName := parentCtx["rule_name"]
	eventBus := parentCtx["event_bus"]

	input := &eventbridge.ListTargetsByRuleInput{
		Rule:         &ruleName,
		EventBusName: &eventBus,
	}

	output, err := api.ListTargetsByRule(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("listing targets for rule %s: %w", ruleName, err)
	}

	resources := make([]resource.Resource, 0, len(output.Targets))

	for _, target := range output.Targets {
		resources = append(resources, convertEventBridgeTarget(target))
	}

	return resources, nil
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
// configuration. Priority: InputTransformer → InputPath → Input (truncated) → em-dash.
func ComputeInputSummary(target ebtypes.Target) string {
	if target.InputTransformer != nil {
		return "InputTransformer"
	}

	if target.InputPath != nil && *target.InputPath != "" {
		return *target.InputPath
	}

	if target.Input != nil && *target.Input != "" {
		input := *target.Input
		if len(input) > 34 {
			return input[:34] + "..."
		}
		return input
	}

	return "\u2014"
}
