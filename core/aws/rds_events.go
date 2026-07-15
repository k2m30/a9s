// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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
		Findings:  dbiEventFindings(event.EventCategories),
		RawStruct: event,
	}
}

// dbiEventFindings maps RDS's documented EventCategories vocabulary
// (https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_Events.html)
// to a Wave-1 Finding. "failure" and "low storage" are broken — both signal
// the instance is (or is about to be) unable to serve traffic. "failover" and
// "recovery" are warn — both signal a disruption just occurred even though
// the instance is back up. Routine categories (backup, availability,
// configuration change, creation, deletion, maintenance, notification, read
// replica, restoration, backtrack) carry no operator-facing risk and emit no
// finding. Priority order matches severity: broken categories are checked
// before warn ones, and an event may legitimately carry more than one
// category.
func dbiEventFindings(categories []string) []domain.Finding {
	has := func(name string) bool {
		return slices.Contains(categories, name)
	}

	switch {
	case has("failure"):
		return []domain.Finding{{Code: CodeDBIEventFailure, Phrase: "failure", Severity: domain.SevBroken, Source: "wave1"}}
	case has("low storage"):
		return []domain.Finding{{Code: CodeDBIEventLowStorage, Phrase: "low storage", Severity: domain.SevBroken, Source: "wave1"}}
	case has("failover"):
		return []domain.Finding{{Code: CodeDBIEventFailover, Phrase: "failover", Severity: domain.SevWarn, Source: "wave1"}}
	case has("recovery"):
		return []domain.Finding{{Code: CodeDBIEventRecovery, Phrase: "recovery", Severity: domain.SevWarn, Source: "wave1"}}
	}
	return nil
}
