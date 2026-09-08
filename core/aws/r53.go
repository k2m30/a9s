// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// r53CodeUnusedZone is the canonical FindingCode for a hosted zone with two
// or fewer record sets (only the default NS+SOA remain) — likely unused.
const r53CodeUnusedZone domain.FindingCode = "r53.zone.unused"

// r53ZoneFindings is the one predicate for a hosted zone. It reads the record
// count the fetcher formats into Fields, so a row rebuilt from Fields alone
// reaches the same verdict. A zone whose count did not come back carries no
// count to judge and yields nothing.
func r53ZoneFindings(recordCount string) []domain.Finding {
	n, err := strconv.Atoi(recordCount)
	if err != nil || n > 2 {
		return nil
	}
	// Two or fewer records means only the default NS+SOA remain.
	return []domain.Finding{wave1Finding(r53CodeUnusedZone)}
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
		s3WebsiteAliasNames := ""
		if recordSetsAPI != nil && zoneID != "" {
			aliasTargets, s3WebsiteAliasNames = enumerateR53AliasTargets(ctx, recordSetsAPI, zoneID)
		}

		r := resource.Resource{
			ID:   zoneID,
			Name: name,
			Fields: map[string]string{
				"zone_id":               zoneID,
				"name":                  name,
				"record_count":          recordCount,
				"private_zone":          privateZone,
				"comment":               comment,
				"alias_targets":         aliasTargets,
				"s3website_alias_names": s3WebsiteAliasNames,
			},
			RawStruct: zone,
		}

		r.Findings = r53ZoneFindings(recordCount)

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

// enumerateR53AliasTargets lists the zone's record sets and returns two
// comma-separated lists:
//  1. aliasTargets — every AliasTarget.DNSName (for pivots that key off
//     the DNSName shape, e.g. elb/cf).
//  2. s3WebsiteAliasNames — record FQDNs (trailing dot stripped) whose
//     AliasTarget.DNSName matches the S3-website regional endpoint
//     (s3-website-<region>.amazonaws.com or s3-website.<region>.
//     amazonaws.com). Used by checkS3R53 per spec §2 — Route 53 alias
//     to S3 requires bucket-name == FQDN, so the join key is the record
//     name, NOT a substring of DNSName (which never contains the bucket
//     name in real AWS).
//
// Cost: one ListResourceRecordSets call per zone.
func enumerateR53AliasTargets(ctx context.Context, api Route53ListResourceRecordSetsAPI, zoneID string) (string, string) {
	out, err := api.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
	})
	if err != nil || out == nil {
		return "", ""
	}
	var aliases, s3Website []string
	for _, rr := range out.ResourceRecordSets {
		if rr.AliasTarget == nil || rr.AliasTarget.DNSName == nil {
			continue
		}
		dns := *rr.AliasTarget.DNSName
		aliases = append(aliases, dns)
		if isS3WebsiteEndpoint(dns) && rr.Name != nil && *rr.Name != "" {
			s3Website = append(s3Website, strings.TrimSuffix(*rr.Name, "."))
		}
	}
	return strings.Join(aliases, ","), strings.Join(s3Website, ",")
}

// isS3WebsiteEndpoint reports whether a Route 53 AliasTarget DNSName is
// an S3 static-website regional endpoint. AWS returns one of:
//   - s3-website-<region>.amazonaws.com.   (legacy hyphen form)
//   - s3-website.<region>.amazonaws.com.   (newer dot form)
//
// The bucket name is NEVER part of this DNSName — the join to a specific
// bucket is by record name (FQDN) which per AWS must equal the bucket name.
func isS3WebsiteEndpoint(dns string) bool {
	return strings.Contains(dns, "s3-website-") || strings.Contains(dns, "s3-website.")
}
