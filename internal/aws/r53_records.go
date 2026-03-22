package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("r53_records", []string{"name", "type", "ttl", "values"})
}

// FetchR53Records calls the Route53 ListResourceRecordSets API for a given
// hosted zone and converts the response into a slice of generic Resource structs.
// It paginates using IsTruncated/NextRecordName/NextRecordType/NextRecordIdentifier.
func FetchR53Records(ctx context.Context, api Route53ListResourceRecordSetsAPI, hostedZoneId string) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextName *string
	var nextType r53types.RRType
	var nextIdentifier *string

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
			return nil, fmt.Errorf("fetching R53 records: %w", err)
		}

		for _, record := range output.ResourceRecordSets {
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

			r := resource.Resource{
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

			resources = append(resources, r)
		}

		if !output.IsTruncated {
			break
		}
		nextName = output.NextRecordName
		nextType = output.NextRecordType
		nextIdentifier = output.NextRecordIdentifier
	}

	return resources, nil
}
