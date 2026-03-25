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

	resource.RegisterChildFetcher("dbi_events", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchRDSEvents(ctx, c.RDS, parentCtx["db_identifier"])
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "RDS Events",
		ShortName: "dbi_events",
		Columns:   resource.DbiEventColumns(),
		CopyField: "message",
	})
}

// FetchRDSEvents calls the RDS DescribeEvents API for a specific DB instance
// and converts the response into a slice of generic Resource structs.
// It paginates via Marker and caps collection at 200 events.
func FetchRDSEvents(ctx context.Context, api RDSDescribeEventsAPI, dbIdentifier string) ([]resource.Resource, error) {
	const maxEvents = 200

	var resources []resource.Resource
	var marker *string

	for {
		input := &rds.DescribeEventsInput{
			SourceIdentifier: aws.String(dbIdentifier),
			SourceType:       rdstypes.SourceTypeDbInstance,
			Duration:         aws.Int32(10080),
			Marker:           marker,
		}

		output, err := api.DescribeEvents(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("fetching RDS events for %s: %w", dbIdentifier, err)
		}

		for _, event := range output.Events {
			if len(resources) >= maxEvents {
				return resources, nil
			}
			resources = append(resources, convertRDSEvent(event))
		}

		if output.Marker == nil || *output.Marker == "" {
			break
		}
		if len(resources) >= maxEvents {
			break
		}
		marker = output.Marker
	}

	return resources, nil
}

// convertRDSEvent converts a single RDS Event into a generic Resource.
func convertRDSEvent(event rdstypes.Event) resource.Resource {
	timestamp := ""
	if event.Date != nil {
		timestamp = event.Date.UTC().Format("2006-01-02 15:04:05")
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
