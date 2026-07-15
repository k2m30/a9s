package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sfn"

	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchStepFunctionsPage fetches a single page of Step Functions state machines.
func FetchStepFunctionsPage(ctx context.Context, api SFNListStateMachinesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &sfn.ListStateMachinesInput{
		MaxResults: int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.ListStateMachines(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching Step Functions: %w", err)
	}

	var resources []resource.Resource

	for _, sm := range output.StateMachines {
		name := ""
		if sm.Name != nil {
			name = *sm.Name
		}

		arn := ""
		if sm.StateMachineArn != nil {
			arn = *sm.StateMachineArn
		}

		smType := string(sm.Type)

		creationDate := ""
		if sm.CreationDate != nil {
			creationDate = sm.CreationDate.Format("2006-01-02 15:04")
		}

		r := resource.Resource{
			ID:   name,
			Name: name,
			Fields: map[string]string{
				"name":          name,
				"arn":           arn,
				"type":          smType,
				"creation_date": creationDate,
			},
			RawStruct: sm,
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
