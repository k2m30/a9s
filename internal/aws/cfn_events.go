package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("cfn_events", []string{
		"timestamp", "logical_resource_id", "resource_type",
		"resource_status", "resource_status_reason",
	})

	resource.RegisterChildFetcher("cfn_events", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCfnEvents(ctx, c.CloudFormation, parentCtx["stack_name"])
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Stack Events",
		ShortName: "cfn_events",
		Columns:   resource.CfnEventColumns(),
	})
}

// FetchCfnEvents calls the CloudFormation DescribeStackEvents API and converts
// the response into a slice of generic Resource structs. Pagination is followed
// via NextToken, capped at 200 events.
func FetchCfnEvents(
	ctx context.Context,
	api CFNDescribeStackEventsAPI,
	stackName string,
) ([]resource.Resource, error) {
	const maxEvents = 200

	var resources []resource.Resource
	var nextToken *string

	for {
		input := &cloudformation.DescribeStackEventsInput{
			StackName: &stackName,
			NextToken: nextToken,
		}

		output, err := api.DescribeStackEvents(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("describing CloudFormation stack events for %s: %w", stackName, err)
		}

		for _, event := range output.StackEvents {
			id := ""
			if event.EventId != nil {
				id = *event.EventId
			}

			timestamp := ""
			name := ""
			if event.Timestamp != nil {
				timestamp = event.Timestamp.UTC().Format("2006-01-02 15:04:05")
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

			r := resource.Resource{
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

			resources = append(resources, r)

			if len(resources) >= maxEvents {
				return resources, nil
			}
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}
