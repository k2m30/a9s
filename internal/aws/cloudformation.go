package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("cfn", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCloudFormationStacks(ctx, c.CloudFormation)
	})
}

// FetchCloudFormationStacks calls the CloudFormation DescribeStacks API and converts the
// response into a slice of generic Resource structs.
func FetchCloudFormationStacks(ctx context.Context, api CFNDescribeStacksAPI) ([]resource.Resource, error) {
	output, err := api.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, stack := range output.Stacks {
		stackName := ""
		if stack.StackName != nil {
			stackName = *stack.StackName
		}

		status := string(stack.StackStatus)

		creationTime := ""
		if stack.CreationTime != nil {
			creationTime = stack.CreationTime.Format("2006-01-02T15:04:05Z07:00")
		}

		lastUpdated := ""
		if stack.LastUpdatedTime != nil {
			lastUpdated = stack.LastUpdatedTime.Format("2006-01-02T15:04:05Z07:00")
		}

		description := ""
		if stack.Description != nil {
			description = *stack.Description
		}

		detail := map[string]string{
			"Stack Name":    stackName,
			"Status":        status,
			"Creation Time": creationTime,
			"Last Updated":  lastUpdated,
			"Description":   description,
		}

		if stack.StackId != nil {
			detail["Stack ID"] = *stack.StackId
		}

		if stack.StackStatusReason != nil {
			detail["Status Reason"] = *stack.StackStatusReason
		}

		if stack.RoleARN != nil {
			detail["Role ARN"] = *stack.RoleARN
		}

		if stack.ParentId != nil {
			detail["Parent Stack"] = *stack.ParentId
		}

		if stack.DriftInformation != nil {
			detail["Drift Status"] = string(stack.DriftInformation.StackDriftStatus)
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(stack, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     stackName,
			Name:   stackName,
			Status: status,
			Fields: map[string]string{
				"stack_name":    stackName,
				"status":        status,
				"creation_time": creationTime,
				"last_updated":  lastUpdated,
				"description":   description,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  stack,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
