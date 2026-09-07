// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// r53ContinuationToken encodes the three Route53 pagination cursors into a single
// JSON string for use as a continuation token.
type r53ContinuationToken struct {
	NextRecordName       string `json:"n,omitempty"`
	NextRecordType       string `json:"t,omitempty"`
	NextRecordIdentifier string `json:"i,omitempty"`
}

// FetchR53Records calls the Route53 ListResourceRecordSets API for a given
// hosted zone and converts the response into a FetchResult with pagination support.
// A single API call is made per invocation. The compound continuation token encodes
// all three Route53 pagination cursors (NextRecordName, NextRecordType,
// NextRecordIdentifier) as a JSON string.
func FetchR53Records(ctx context.Context, api Route53ListResourceRecordSetsAPI, hostedZoneId string, continuationToken string) (resource.FetchResult, error) {
	if hostedZoneId == "" {
		return resource.FetchResult{}, nil
	}

	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId: &hostedZoneId,
	}

	if continuationToken != "" {
		var ct r53ContinuationToken
		if err := json.Unmarshal([]byte(continuationToken), &ct); err == nil {
			if ct.NextRecordName != "" {
				input.StartRecordName = &ct.NextRecordName
			}
			if ct.NextRecordType != "" {
				input.StartRecordType = r53types.RRType(ct.NextRecordType)
			}
			if ct.NextRecordIdentifier != "" {
				input.StartRecordIdentifier = &ct.NextRecordIdentifier
			}
		}
	}

	output, err := api.ListResourceRecordSets(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching R53 records: %w", err)
	}

	var resources []resource.Resource
	for _, record := range output.ResourceRecordSets {
		resources = append(resources, convertR53Record(record, hostedZoneId))
	}

	nextToken := ""
	isTruncated := output.IsTruncated
	if isTruncated {
		ct := r53ContinuationToken{}
		if output.NextRecordName != nil {
			ct.NextRecordName = *output.NextRecordName
		}
		if output.NextRecordType != "" {
			ct.NextRecordType = string(output.NextRecordType)
		}
		if output.NextRecordIdentifier != nil {
			ct.NextRecordIdentifier = *output.NextRecordIdentifier
		}
		if b, err := json.Marshal(ct); err == nil {
			nextToken = string(b)
		}
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

// convertR53Record converts a single Route53 ResourceRecordSet into a
// generic Resource. hostedZoneId is threaded through to Fields["zone_id"]
// (in its raw "/hostedzone/XXXX" form, matching the parent r53 type's own
// r.ID) so the console-link builder can deep-link to the parent zone's page.
func convertR53Record(record r53types.ResourceRecordSet, hostedZoneId string) resource.Resource {
	name := ""
	if record.Name != nil {
		name = *record.Name
	}

	recType := string(record.Type)

	ttl := ""
	if record.TTL != nil {
		ttl = fmt.Sprintf("%d", *record.TTL)
	}

	// Compute values: join ResourceRecords or use AliasTarget
	var values string
	if record.AliasTarget != nil && record.AliasTarget.DNSName != nil {
		values = "ALIAS: " + *record.AliasTarget.DNSName
	} else {
		var vals []string
		for _, rr := range record.ResourceRecords {
			if rr.Value != nil {
				vals = append(vals, *rr.Value)
			}
		}
		values = strings.Join(vals, ", ")
	}

	// ID = Name|Type, appending |SetIdentifier if non-empty
	id := name + "|" + recType
	if record.SetIdentifier != nil && *record.SetIdentifier != "" {
		id += "|" + *record.SetIdentifier
	}

	return resource.Resource{
		ID:   id,
		Name: name,
		Fields: map[string]string{
			"name":    name,
			"type":    recType,
			"ttl":     ttl,
			"values":  values,
			"zone_id": hostedZoneId,
		},
		RawStruct: record,
	}
}

// errRecordPagesCapped is the walk stopping at its own page cap rather than
// on anything AWS said. It is a cause a9s owns, so it reads as one.
var errRecordPagesCapped = errors.New("record listing stopped at the page cap")

// listAllR53Records walks a zone's record sets to PerParentPageCap and reports
// whether it reached the end. Route 53 paginates records by a three-part
// cursor rather than a token, so the walk lives here beside the cursor
// encoding rather than in the enricher that consumes it.
//
// A non-nil error means the zone was not seen whole — a page failed, or the
// walk hit the cap (errRecordPagesCapped). A dangling record found in the
// pages that did arrive is still real, so the caller reports those and marks
// the zone truncated: what a short walk cannot say is that the rest of the
// zone is clean.
func listAllR53Records(ctx context.Context, api Route53ListResourceRecordSetsAPI, zoneID string) (records []r53types.ResourceRecordSet, err error) {
	input := &route53.ListResourceRecordSetsInput{HostedZoneId: &zoneID}
	for range PerParentPageCap {
		out, pageErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*route53.ListResourceRecordSetsOutput, error) {
			return api.ListResourceRecordSets(ctx, input)
		})
		if pageErr != nil {
			return records, pageErr
		}
		records = append(records, out.ResourceRecordSets...)
		if !out.IsTruncated {
			return records, nil
		}
		input.StartRecordName = out.NextRecordName
		input.StartRecordType = out.NextRecordType
		input.StartRecordIdentifier = out.NextRecordIdentifier
	}
	// The page cap stopped the walk: the records read are real, but the zone
	// was not seen whole.
	return records, errRecordPagesCapped
}
