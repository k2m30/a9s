package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sfn"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("sfn", []string{"name", "type", "arn", "creation_date"})

	resource.RegisterPaginated("sfn", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchStepFunctionsPage(ctx, c.SFN, continuationToken)
	})

	resource.RegisterRelated("sfn", []resource.RelatedDef{
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkSFNAlarm, NeedsTargetCache: true},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkSFNLogs, NeedsTargetCache: true},
		{TargetType: "role", DisplayName: "IAM Role", Checker: nil, NeedsTargetCache: false},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: nil, NeedsTargetCache: true},
	})
}

// FetchStepFunctions calls the SFN ListStateMachines API and converts
// the response into a slice of generic Resource structs.
func FetchStepFunctions(ctx context.Context, api SFNListStateMachinesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchStepFunctionsPage(ctx, api, token)
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

// FetchStepFunctionsPage fetches a single page of Step Functions state machines.
func FetchStepFunctionsPage(ctx context.Context, api SFNListStateMachinesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &sfn.ListStateMachinesInput{}
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
			ID:     name,
			Name:   name,
			Status: "",
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
