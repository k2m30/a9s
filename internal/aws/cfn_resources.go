package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("cfn_resources", []string{
		"logical_resource_id", "physical_resource_id", "resource_type",
		"resource_status", "drift_status", "last_updated",
	})

	resource.RegisterPaginatedChild("cfn_resources", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCfnResources(ctx, c.CloudFormation, parentCtx["stack_name"], continuationToken)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Stack Resources",
		ShortName: "cfn_resources",
		Columns:   resource.CfnResourceColumns(),
	})
}

// FetchCfnResources calls the CloudFormation ListStackResources API and converts
// the response into a FetchResult with pagination support. Pagination is followed
// via NextToken. When a continuation token is provided, fetching resumes from that page.
func FetchCfnResources(
	ctx context.Context,
	api CFNListStackResourcesAPI,
	stackName string,
	continuationToken string,
) (resource.FetchResult, error) {
	var resources []resource.Resource
	var nextToken *string
	if continuationToken != "" {
		nextToken = &continuationToken
	}

	for {
		input := &cloudformation.ListStackResourcesInput{
			StackName: &stackName,
			NextToken: nextToken,
		}

		output, err := api.ListStackResources(ctx, input)
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("listing CloudFormation stack resources for %s: %w", stackName, err)
		}

		for _, summary := range output.StackResourceSummaries {
			resources = append(resources, convertCfnResource(summary))
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
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

// convertCfnResource converts a single CloudFormation StackResourceSummary into a generic Resource.
func convertCfnResource(summary cfntypes.StackResourceSummary) resource.Resource {
	logicalResourceID := ""
	if summary.LogicalResourceId != nil {
		logicalResourceID = *summary.LogicalResourceId
	}

	physicalResourceID := ""
	if summary.PhysicalResourceId != nil {
		physicalResourceID = *summary.PhysicalResourceId
	}

	resourceType := ""
	if summary.ResourceType != nil {
		resourceType = *summary.ResourceType
	}

	resourceStatus := string(summary.ResourceStatus)

	driftStatus := ""
	if summary.DriftInformation != nil {
		driftStatus = string(summary.DriftInformation.StackResourceDriftStatus)
	}

	lastUpdated := ""
	if summary.LastUpdatedTimestamp != nil {
		lastUpdated = summary.LastUpdatedTimestamp.UTC().Format("2006-01-02 15:04:05")
	}

	return resource.Resource{
		ID:     logicalResourceID,
		Name:   logicalResourceID,
		Status: resourceStatus,
		Fields: map[string]string{
			"logical_resource_id":  logicalResourceID,
			"physical_resource_id": physicalResourceID,
			"resource_type":        resourceType,
			"resource_status":      resourceStatus,
			"drift_status":         driftStatus,
			"last_updated":         lastUpdated,
		},
		RawStruct: summary,
	}
}
