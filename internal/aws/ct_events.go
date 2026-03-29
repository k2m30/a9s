package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ct-events", []string{"event_name", "time", "user", "source", "resource_type", "resource_name", "read_only"})

	// Legacy fetcher for availability probe (one page only).
	resource.Register("ct-events", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		result, err := FetchCloudTrailEventsPage(ctx, c.CloudTrail, "")
		if err != nil {
			return nil, err
		}
		return result.Resources, nil
	})

	// Paginated fetcher for resource list browsing (M key load-more).
	resource.RegisterPaginated("ct-events", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCloudTrailEventsPage(ctx, c.CloudTrail, continuationToken)
	})
}

// FetchCloudTrailEvents is a compatibility wrapper that fetches the first page
// of CloudTrail events and returns the resources. Used by existing tests.
func FetchCloudTrailEvents(ctx context.Context, api CloudTrailLookupEventsAPI) ([]resource.Resource, error) {
	result, err := FetchCloudTrailEventsPage(ctx, api, "")
	if err != nil {
		return nil, err
	}
	return result.Resources, nil
}

// FetchCloudTrailEventsPage calls the CloudTrail LookupEvents API and returns
// a single page of events. Pass an empty continuationToken for the first page.
func FetchCloudTrailEventsPage(ctx context.Context, api CloudTrailLookupEventsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &cloudtrail.LookupEventsInput{
		MaxResults: aws.Int32(50),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.LookupEvents(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching CloudTrail events: %w", err)
	}

	var resources []resource.Resource
	for _, event := range output.Events {
		eventID := ""
		if event.EventId != nil {
			eventID = *event.EventId
		}

		eventName := ""
		if event.EventName != nil {
			eventName = *event.EventName
		}

		eventTime := ""
		if event.EventTime != nil {
			eventTime = event.EventTime.Format("2006-01-02 15:04:05")
		}

		user := ""
		if event.Username != nil {
			user = *event.Username
		}

		source := ""
		if event.EventSource != nil {
			source = *event.EventSource
		}

		// Guard Resources slice: extract first resource if present
		resourceType := ""
		resourceName := ""
		if len(event.Resources) > 0 {
			if event.Resources[0].ResourceType != nil {
				resourceType = *event.Resources[0].ResourceType
			}
			if event.Resources[0].ResourceName != nil {
				resourceName = *event.Resources[0].ResourceName
			}
		}

		// ReadOnly is *string ("true" or "false")
		readOnly := ""
		if event.ReadOnly != nil {
			readOnly = *event.ReadOnly
		}

		r := resource.Resource{
			ID:     eventID,
			Name:   eventName,
			Status: readOnly,
			Fields: map[string]string{
				"event_name":    eventName,
				"time":          eventTime,
				"user":          user,
				"source":        source,
				"resource_type": resourceType,
				"resource_name": resourceName,
				"read_only":     readOnly,
			},
			RawStruct: event,
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

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, nil
}
