package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("sns", []string{"topic_arn", "display_name"})

	resource.RegisterPaginated("sns", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSNSTopicsPage(ctx, c.SNS, continuationToken)
	})
}

// FetchSNSTopics calls the SNS ListTopics API and returns all pages of topics.
// Used by existing tests and the legacy fetcher.
func FetchSNSTopics(ctx context.Context, api SNSListTopicsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchSNSTopicsPage(ctx, api, token)
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

// FetchSNSTopicsPage calls the SNS ListTopics API and returns a single page
// of topics. Pass an empty continuationToken for the first page.
func FetchSNSTopicsPage(ctx context.Context, api SNSListTopicsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &sns.ListTopicsInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.ListTopics(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching SNS topics: %w", err)
	}

	var resources []resource.Resource
	for _, topic := range output.Topics {
		topicArn := ""
		if topic.TopicArn != nil {
			topicArn = *topic.TopicArn
		}

		// Extract display name from ARN (last segment after :)
		displayName := topicArn
		if parts := strings.Split(topicArn, ":"); len(parts) > 0 {
			displayName = parts[len(parts)-1]
		}

		r := resource.Resource{
			ID:     topicArn,
			Name:   displayName,
			Status: "",
			Fields: map[string]string{
				"topic_arn":    topicArn,
				"display_name": displayName,
			},
			RawStruct: topic,
		}

		resources = append(resources, r)
	}

	// Build pagination metadata
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
