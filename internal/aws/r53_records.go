package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("r53_records", []string{"name", "type", "ttl", "values"})

	// Register R53 records as a child type with its own fetcher.
	resource.RegisterPaginatedChild("r53_records", func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchR53Records(ctx, c.Route53, parentCtx["zone_id"], continuationToken)
	})
	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "R53 Records",
		ShortName: "r53_records",
		Columns:   resource.R53RecordColumns(),
	})
}

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
		resources = append(resources, convertR53Record(record))
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

// convertR53Record converts a single Route53 ResourceRecordSet into a generic Resource.
func convertR53Record(record r53types.ResourceRecordSet) resource.Resource {
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
		ID:     id,
		Name:   name,
		Status: recType,
		Fields: map[string]string{
			"name":   name,
			"type":   recType,
			"ttl":    ttl,
			"values": values,
		},
		RawStruct: record,
	}
}
