package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("dbi_events", []string{
		"timestamp", "event_categories", "message",
		"source_identifier", "source_type", "source_arn",
	})

	resource.RegisterPaginatedChild("dbi_events", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchRDSEvents(ctx, c.RDS, parentCtx["db_identifier"], continuationToken)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "RDS Events",
		ShortName: "dbi_events",
		Columns:   resource.DbiEventColumns(),
		CopyField: "message",
	})
}

// FetchRDSEvents calls the RDS DescribeEvents API for a specific DB instance
// and converts the response into a FetchResult with pagination support. A single
// API call is made per invocation; IsTruncated and NextToken (Marker) are
// forwarded as pagination metadata for the caller to request the next page.
func FetchRDSEvents(ctx context.Context, api RDSDescribeEventsAPI, dbIdentifier string, continuationToken string) (resource.FetchResult, error) {
	input := &rds.DescribeEventsInput{
		SourceIdentifier: aws.String(dbIdentifier),
		SourceType:       rdstypes.SourceTypeDbInstance,
		Duration:         aws.Int32(10080),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.DescribeEvents(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching RDS events for %s: %w", dbIdentifier, err)
	}

	var resources []resource.Resource
	for _, event := range output.Events {
		resources = append(resources, convertRDSEvent(event))
	}

	nextToken := ""
	isTruncated := false
	if output.Marker != nil && *output.Marker != "" {
		nextToken = *output.Marker
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

// convertRDSEvent converts a single RDS Event into a generic Resource.
func convertRDSEvent(event rdstypes.Event) resource.Resource {
	timestamp := ""
	if event.Date != nil {
		timestamp = event.Date.UTC().Format("2006-01-02 15:04")
	}

	categories := strings.Join(event.EventCategories, ", ")

	message := ""
	if event.Message != nil {
		msg := *event.Message
		msg = strings.ReplaceAll(msg, "\n", " ")
		msg = strings.ReplaceAll(msg, "\r", " ")
		message = msg
	}

	sourceIdentifier := ""
	if event.SourceIdentifier != nil {
		sourceIdentifier = *event.SourceIdentifier
	}

	sourceType := string(event.SourceType)

	sourceArn := ""
	if event.SourceArn != nil {
		sourceArn = *event.SourceArn
	}

	id := timestamp + "/" + sourceIdentifier

	return resource.Resource{
		ID:   id,
		Name: timestamp,
		Fields: map[string]string{
			"timestamp":         timestamp,
			"event_categories":  categories,
			"message":           message,
			"source_identifier": sourceIdentifier,
			"source_type":       sourceType,
			"source_arn":        sourceArn,
		},
		RawStruct: event,
	}
}
