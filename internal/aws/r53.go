package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("r53", []string{"zone_id", "name", "record_count", "private_zone", "comment", "alias_targets"})

	resource.RegisterPaginated("r53", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchHostedZonesPage(ctx, c.Route53, continuationToken)
	})
}

// FetchHostedZones calls the Route53 ListHostedZones API and converts
// the response into a slice of generic Resource structs.
func FetchHostedZones(ctx context.Context, api Route53ListHostedZonesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchHostedZonesPage(ctx, api, token)
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

// FetchHostedZonesPage fetches a single page of Route53 hosted zones.
func FetchHostedZonesPage(ctx context.Context, api Route53ListHostedZonesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &route53.ListHostedZonesInput{
		MaxItems: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.ListHostedZones(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching Route53 hosted zones: %w", err)
	}

	recordSetsAPI, _ := api.(Route53ListResourceRecordSetsAPI)

	var resources []resource.Resource

	for _, zone := range output.HostedZones {
		zoneID := ""
		if zone.Id != nil {
			zoneID = *zone.Id
		}

		name := ""
		if zone.Name != nil {
			name = *zone.Name
		}

		recordCount := ""
		if zone.ResourceRecordSetCount != nil {
			recordCount = fmt.Sprintf("%d", *zone.ResourceRecordSetCount)
		}

		privateZone := "false"
		comment := ""
		if zone.Config != nil {
			if zone.Config.PrivateZone {
				privateZone = "true"
			}
			if zone.Config.Comment != nil {
				comment = *zone.Config.Comment
			}
		}

		aliasTargets := ""
		if recordSetsAPI != nil && zoneID != "" {
			aliasTargets = enumerateR53AliasTargets(ctx, recordSetsAPI, zoneID)
		}

		r := resource.Resource{
			ID:     zoneID,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"zone_id":       zoneID,
				"name":          name,
				"record_count":  recordCount,
				"private_zone":  privateZone,
				"comment":       comment,
				"alias_targets": aliasTargets,
			},
			RawStruct: zone,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := output.IsTruncated
	if isTruncated && output.NextMarker != nil {
		nextToken = *output.NextMarker
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

// enumerateR53AliasTargets lists the zone's record sets and returns a
// comma-separated AliasTarget.DNSName list. Used so sibling pivots
// (s3, cf, elb, …) can scan alias_targets and resolve non-zero.
// Cost: one ListResourceRecordSets call per zone.
func enumerateR53AliasTargets(ctx context.Context, api Route53ListResourceRecordSetsAPI, zoneID string) string {
	out, err := api.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
	})
	if err != nil || out == nil {
		return ""
	}
	var aliases []string
	for _, rr := range out.ResourceRecordSets {
		if rr.AliasTarget == nil || rr.AliasTarget.DNSName == nil {
			continue
		}
		aliases = append(aliases, *rr.AliasTarget.DNSName)
	}
	return strings.Join(aliases, ",")
}
