package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("asg_activities", []string{
		"start_time", "status_code", "description", "cause",
	})

	resource.RegisterPaginatedChild("asg_activities", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchAsgActivities(ctx, c.AutoScaling, parentCtx, continuationToken)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Scaling Activities",
		ShortName: "asg_activities",
		Columns:   resource.AsgActivityColumns(),
	})
}

// FetchAsgActivities calls the AutoScaling DescribeScalingActivities API and
// converts the response into a FetchResult with pagination support. Each call
// returns up to 200 activities. When the cap is reached and more pages exist,
// FetchResult.Pagination.IsTruncated is set to true with a NextToken for
// continuation.
func FetchAsgActivities(
	ctx context.Context,
	api ASGDescribeScalingActivitiesAPI,
	parentCtx map[string]string,
	continuationToken string,
) (resource.FetchResult, error) {
	const maxActivities = 200

	asgName := parentCtx["asg_name"]

	var resources []resource.Resource
	var nextToken *string
	if continuationToken != "" {
		nextToken = &continuationToken
	}

	for {
		input := &autoscaling.DescribeScalingActivitiesInput{
			AutoScalingGroupName: &asgName,
			NextToken:            nextToken,
		}

		output, err := api.DescribeScalingActivities(ctx, input)
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("describing scaling activities for %s: %w", asgName, err)
		}

		for _, activity := range output.Activities {
			resources = append(resources, convertAsgActivity(activity))

			if len(resources) >= maxActivities {
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

// convertAsgActivity converts a single AutoScaling Activity into a generic Resource.
func convertAsgActivity(activity asgtypes.Activity) resource.Resource {
	id := ""
	if activity.ActivityId != nil {
		id = *activity.ActivityId
	}

	startTime := ""
	name := ""
	if activity.StartTime != nil {
		startTime = activity.StartTime.UTC().Format("2006-01-02 15:04")
		name = startTime
	}

	statusCode := string(activity.StatusCode)

	description := ""
	if activity.Description != nil {
		description = strings.ReplaceAll(*activity.Description, "\n", " ")
		description = strings.ReplaceAll(description, "\r", " ")
	}

	cause := ""
	if activity.Cause != nil {
		cause = strings.ReplaceAll(*activity.Cause, "\n", " ")
		cause = strings.ReplaceAll(cause, "\r", " ")
	}

	return resource.Resource{
		ID:     id,
		Name:   name,
		Status: statusCode,
		Fields: map[string]string{
			"start_time":  startTime,
			"status_code": statusCode,
			"description": description,
			"cause":       cause,
		},
		RawStruct: activity,
	}
}
