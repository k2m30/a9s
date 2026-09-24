// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// cfn_related.go contains CloudFormation related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkCfnRole extracts the RoleARN from the CloudFormation Stack RawStruct.
// It extracts the role name from the last path segment of the ARN (after the last "/")
// and searches the role cache by name or ID.
func checkCfnRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stack, ok := assertStruct[cfntypes.Stack](res.RawStruct)
	if !ok {
		return NotRead("role")
	}
	if stack.RoleARN == nil || *stack.RoleARN == "" {
		return foundNone("role", "stack.RoleARN")
	}
	// In-body: the stack's service RoleARN normalizes to the role name (== the
	// role's Resource.ID). Resolve by identity — no role-list fetch.
	return relatedRefs("role", []string{*stack.RoleARN}, refContext(clients, cache, "role"))
}

// checkCFNCFN finds related CloudFormation stacks — parent and child (nested) stacks.
// Forward lookup for ParentId (this is a nested stack) and reverse scan
// for stacks whose ParentId matches this stack's StackId (children of this stack).
func checkCFNCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stack, ok := assertStruct[cfntypes.Stack](res.RawStruct)
	if !ok {
		return NotRead("cfn")
	}

	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
	if err != nil {
		return ReadFailed("cfn", err)
	}
	if cfnList == nil {
		return NotRead("cfn")
	}

	thisStackID := ""
	if stack.StackId != nil {
		thisStackID = *stack.StackId
	}

	seen := make(map[string]struct{})

	// Forward: if this stack has a ParentId, it is a nested stack — add the parent.
	if stack.ParentId != nil && *stack.ParentId != "" {
		parentID := *stack.ParentId
		for _, cfnRes := range cfnList {
			rawCFN, cfnOk := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
			if !cfnOk {
				continue
			}
			if rawCFN.StackId != nil && *rawCFN.StackId == parentID {
				seen[cfnRes.ID] = struct{}{}
			}
		}
	}

	// Reverse: scan for stacks whose ParentId matches this stack's StackId (child stacks).
	if thisStackID != "" {
		for _, cfnRes := range cfnList {
			rawCFN, cfnOk := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
			if !cfnOk {
				continue
			}
			if rawCFN.ParentId != nil && *rawCFN.ParentId == thisStackID {
				seen[cfnRes.ID] = struct{}{}
			}
		}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}

	return relatedResultTrunc("cfn", ids, truncated)
}

// checkCfnSNS extracts notification ARNs from the CloudFormation Stack's
// NotificationARNs field and returns SNS topic identifiers.
func checkCfnSNS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	stack, ok := assertStruct[cfntypes.Stack](res.RawStruct)
	if !ok {
		return NotRead("sns")
	}
	var ids []string
	for _, arn := range stack.NotificationARNs {
		if arn != "" {
			ids = append(ids, arn)
		}
	}
	if len(ids) == 0 {
		return foundNone("sns", "ids")
	}
	return relatedResultTrunc("sns", ids, false)
}

// cfnStackResourcesByType walks cloudformation:ListStackResources(stack) and
// returns the PhysicalResourceIds whose ResourceType matches the given value
// (e.g. "AWS::S3::Bucket").
//
// truncated reports that the walk stopped at the page cap: resources of the
// wanted type may sit on pages nobody read, so the count the caller renders
// is a lower bound and never an exact figure.
func cfnStackResourcesByType(ctx context.Context, clients any, stackName, resourceType string) (ids []string, truncated, ok bool) {
	if stackName == "" {
		return nil, false, true
	}
	c, isClients := clients.(*ServiceClients)
	if !isClients || c == nil || c.CloudFormation == nil {
		return nil, false, false
	}
	summaries, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]cfntypes.StackResourceSummary, *string, error) {
		out, err := c.CloudFormation.ListStackResources(ctx, &cloudformation.ListStackResourcesInput{
			StackName: aws.String(stackName),
			NextToken: token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.StackResourceSummaries, out.NextToken, nil
	})
	for _, r := range summaries {
		if r.ResourceType == nil || *r.ResourceType != resourceType {
			continue
		}
		if r.PhysicalResourceId == nil || *r.PhysicalResourceId == "" {
			continue
		}
		ids = append(ids, *r.PhysicalResourceId)
	}
	// Both checkCfn* callers turn false into UnknownRelated, so the pivot
	// renders "?" rather than claiming the stack holds no such resources; a
	// failed page beside resources found leaves them a lower bound.
	// no finding: false is the answer this helper exists to give.
	if err != nil && len(ids) == 0 {
		return nil, false, false
	}
	return ids, !complete || err != nil, true
}

// checkCfnS3 calls ListStackResources and returns S3 buckets created by the
// stack (ResourceType=AWS::S3::Bucket).
func checkCfnS3(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ids, truncated, ok := cfnStackResourcesByType(ctx, clients, res.ID, "AWS::S3::Bucket")
	if !ok {
		return NotRead("s3")
	}
	return relatedResultTrunc("s3", ids, truncated)
}

// checkCfnEBRule calls ListStackResources and returns EventBridge rules
// created by the stack (ResourceType=AWS::Events::Rule). The PhysicalResourceId
// of an Events::Rule is the rule name.
func checkCfnEBRule(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	refs, truncated, ok := cfnStackResourcesByType(ctx, clients, res.ID, "AWS::Events::Rule")
	if !ok {
		return NotRead("eb-rule")
	}
	ids, dropped := resolveRefs("eb-rule", refs, refContext(clients, cache, "eb-rule"))
	return relatedResultTrunc("eb-rule", ids, truncated || dropped)
}
