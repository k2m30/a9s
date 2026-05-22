package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchAsgActivities calls the AutoScaling DescribeScalingActivities API and
// converts the response into a FetchResult with pagination support. A single
// API call is made per invocation; IsTruncated and NextToken are forwarded as
// pagination metadata for the caller to request the next page.
func FetchAsgActivities(
	ctx context.Context,
	api ASGDescribeScalingActivitiesAPI,
	parentCtx map[string]string,
	continuationToken string,
) (resource.FetchResult, error) {
	asgName := parentCtx["asg_name"]

	input := &autoscaling.DescribeScalingActivitiesInput{
		AutoScalingGroupName: &asgName,
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeScalingActivities(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("describing scaling activities for %s: %w", asgName, err)
	}

	var resources []resource.Resource
	for _, activity := range output.Activities {
		resources = append(resources, convertAsgActivity(activity))
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
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
