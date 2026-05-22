package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchCloudFormationStacks calls the CloudFormation DescribeStacks API and converts the
// response into a slice of generic Resource structs.
func FetchCloudFormationStacks(ctx context.Context, api CFNDescribeStacksAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchCloudFormationStacksPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchCloudFormationStacksPage fetches a single page of CloudFormation stacks.
func FetchCloudFormationStacksPage(ctx context.Context, api CFNDescribeStacksAPI, continuationToken string) (resource.FetchResult, error) {
	input := &cloudformation.DescribeStacksInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeStacks(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching CloudFormation stacks: %w", err)
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
			creationTime = stack.CreationTime.Format("2006-01-02 15:04")
		}

		lastUpdated := ""
		if stack.LastUpdatedTime != nil {
			lastUpdated = stack.LastUpdatedTime.Format("2006-01-02 15:04")
		}

		description := ""
		if stack.Description != nil {
			description = *stack.Description
		}

		r := resource.Resource{
			ID:       stackName,
			Name:     stackName,
			Findings: cfnStackFindings(status),
			Fields: map[string]string{
				"stack_name":    stackName,
				"status":        status,
				"creation_time": creationTime,
				"last_updated":  lastUpdated,
				"description":   description,
			},
			RawStruct: stack,
		}

		resources = append(resources, r)
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

func cfnStackFindings(status string) []domain.Finding {
	switch status {
	case "ROLLBACK_COMPLETE", "ROLLBACK_FAILED",
		"UPDATE_ROLLBACK_COMPLETE", "UPDATE_ROLLBACK_FAILED",
		"IMPORT_ROLLBACK_COMPLETE", "IMPORT_ROLLBACK_FAILED":
		return []domain.Finding{{Code: CodeCFNStackRollback, Phrase: strings.ToLower(status), Severity: domain.SevBroken, Source: "wave1"}}
	}
	if strings.HasSuffix(status, "_FAILED") {
		return []domain.Finding{{Code: CodeCFNStackFailed, Phrase: strings.ToLower(status), Severity: domain.SevBroken, Source: "wave1"}}
	}
	if strings.HasSuffix(status, "_IN_PROGRESS") {
		return []domain.Finding{{Code: CodeCFNStackInProgress, Phrase: strings.ToLower(status), Severity: domain.SevWarn, Source: "wave1"}}
	}
	return nil
}
