package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("cfn_events", []string{
		"timestamp", "logical_resource_id", "resource_type",
		"resource_status", "resource_status_reason",
	})

	resource.RegisterPaginatedChild("cfn_events", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCfnEvents(ctx, c.CloudFormation, parentCtx["stack_name"], continuationToken)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Stack Events",
		ShortName: "cfn_events",
		Columns:   resource.CfnEventColumns(),
	})
}

// FetchCfnEvents calls the CloudFormation DescribeStackEvents API and converts
// the response into a FetchResult with pagination support. Each call returns up
// to maxEvents (200) items. When the cap is reached and more pages exist,
// FetchResult.Pagination.IsTruncated is set to true with a NextToken for continuation.
func FetchCfnEvents(
	ctx context.Context,
	api CFNDescribeStackEventsAPI,
	stackName string,
	continuationToken string,
) (resource.FetchResult, error) {
	const maxEvents = 200

	var resources []resource.Resource
	var nextToken *string
	if continuationToken != "" {
		nextToken = &continuationToken
	}

	for {
		input := &cloudformation.DescribeStackEventsInput{
			StackName: &stackName,
			NextToken: nextToken,
		}

		output, err := api.DescribeStackEvents(ctx, input)
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("describing CloudFormation stack events for %s: %w", stackName, err)
		}

		for _, event := range output.StackEvents {
			resources = append(resources, convertCfnEvent(event))

			if len(resources) >= maxEvents {
				apiNextToken := ""
				if output.NextToken != nil {
					apiNextToken = *output.NextToken
				}
				return resource.FetchResult{
					Resources: resources,
					Pagination: &resource.PaginationMeta{
						IsTruncated: apiNextToken != "",
						NextToken:   apiNextToken,
						PageSize:    len(resources),
					},
				}, nil
			}
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

// convertCfnEvent converts a single CloudFormation StackEvent into a generic Resource.
func convertCfnEvent(event cfntypes.StackEvent) resource.Resource {
	id := ""
	if event.EventId != nil {
		id = *event.EventId
	}

	timestamp := ""
	name := ""
	if event.Timestamp != nil {
		timestamp = event.Timestamp.UTC().Format("2006-01-02 15:04")
		name = timestamp
	}

	logicalResourceID := ""
	if event.LogicalResourceId != nil {
		logicalResourceID = *event.LogicalResourceId
	}

	resourceType := ""
	if event.ResourceType != nil {
		resourceType = *event.ResourceType
	}

	resourceStatus := string(event.ResourceStatus)

	resourceStatusReason := ""
	if event.ResourceStatusReason != nil {
		resourceStatusReason = strings.ReplaceAll(*event.ResourceStatusReason, "\n", " ")
		resourceStatusReason = strings.ReplaceAll(resourceStatusReason, "\r", " ")
	}

	return resource.Resource{
		ID:     id,
		Name:   name,
		Status: resourceStatus,
		Fields: map[string]string{
			"timestamp":              timestamp,
			"logical_resource_id":    logicalResourceID,
			"resource_type":          resourceType,
			"resource_status":        resourceStatus,
			"resource_status_reason": resourceStatusReason,
		},
		RawStruct: event,
	}
}
