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
	resource.RegisterPaginatedChild("r53_records", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
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
// It paginates using IsTruncated/NextRecordName/NextRecordType/NextRecordIdentifier.
func FetchR53Records(ctx context.Context, api Route53ListResourceRecordSetsAPI, hostedZoneId string, continuationToken string) (resource.FetchResult, error) {
	var resources []resource.Resource
	var nextName *string
	var nextType r53types.RRType
	var nextIdentifier *string

	if continuationToken != "" {
		var ct r53ContinuationToken
		if err := json.Unmarshal([]byte(continuationToken), &ct); err == nil {
			if ct.NextRecordName != "" {
				nextName = &ct.NextRecordName
			}
			if ct.NextRecordType != "" {
				nextType = r53types.RRType(ct.NextRecordType)
			}
			if ct.NextRecordIdentifier != "" {
				nextIdentifier = &ct.NextRecordIdentifier
			}
		}
	}

	for {
		input := &route53.ListResourceRecordSetsInput{
			HostedZoneId: &hostedZoneId,
		}
		if nextName != nil {
			input.StartRecordName = nextName
		}
		if nextType != "" {
			input.StartRecordType = nextType
		}
		if nextIdentifier != nil {
			input.StartRecordIdentifier = nextIdentifier
		}

		output, err := api.ListResourceRecordSets(ctx, input)
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("fetching R53 records: %w", err)
		}

		for _, record := range output.ResourceRecordSets {
			resources = append(resources, convertR53Record(record))
		}

		if !output.IsTruncated {
			break
		}
		nextName = output.NextRecordName
		nextType = output.NextRecordType
		nextIdentifier = output.NextRecordIdentifier
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			TotalHint:   len(resources),
			PageSize:    len(resources),
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
