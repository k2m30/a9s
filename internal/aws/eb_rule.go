package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("eb-rule", []string{"name", "state", "event_bus", "schedule", "description"})

	resource.RegisterPaginated("eb-rule", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEventBridgeRulesPage(ctx, c.EventBridge, continuationToken)
	})
}

// FetchEventBridgeRules calls the EventBridge ListRules API and converts
// the response into a slice of generic Resource structs.
func FetchEventBridgeRules(ctx context.Context, api EventBridgeListRulesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchEventBridgeRulesPage(ctx, api, token)
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

// FetchEventBridgeRulesPage fetches a single page of EventBridge rules.
func FetchEventBridgeRulesPage(ctx context.Context, api EventBridgeListRulesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &eventbridge.ListRulesInput{
		Limit: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.ListRules(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching EventBridge rules: %w", err)
	}

	var resources []resource.Resource

	for _, rule := range output.Rules {
		name := ""
		if rule.Name != nil {
			name = *rule.Name
		}

		state := string(rule.State)

		description := ""
		if rule.Description != nil {
			description = *rule.Description
		}

		eventBus := ""
		if rule.EventBusName != nil {
			eventBus = *rule.EventBusName
		}

		schedule := ""
		if rule.ScheduleExpression != nil {
			schedule = *rule.ScheduleExpression
		}

		r := resource.Resource{
			ID:     name,
			Name:   name,
			Status: state,
			Fields: map[string]string{
				"name":        name,
				"state":       state,
				"description": description,
				"event_bus":   eventBus,
				"schedule":    schedule,
			},
			RawStruct: rule,
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
