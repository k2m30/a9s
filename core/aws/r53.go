// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

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
	// Every zone's alias enumeration is its own call, so one denial is one
	// zone's coverage gap rather than the page's failure.
	var failures []Failure

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
		recordsTruncated := ""
		if recordSetsAPI != nil && zoneID != "" {
			var aliasErr error
			var unread bool
			aliasTargets, s3WebsiteAliasNames, unread, aliasErr = enumerateR53AliasTargets(ctx, recordSetsAPI, zoneID)
			if aliasErr != nil {
				failures = append(failures, FailedCall(zoneID, aliasErr))
			}
			if unread {
				recordsTruncated = "true"
			}
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
				"records_truncated":     recordsTruncated,
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
	}, AggregateFailures("ListResourceRecordSets", failures, len(output.HostedZones))
}

// enumerateR53AliasTargets reads the zone's record sets and returns two
// comma-separated lists:
//  1. aliasTargets — every AliasTarget.DNSName (for pivots that key off
//     the DNSName shape, e.g. elb/cf).
//  2. s3WebsiteAliasNames — record FQDNs (trailing dot stripped) whose
//     AliasTarget.DNSName matches the S3-website regional endpoint
//     (s3-website-<region>.amazonaws.com or s3-website.<region>.
//     amazonaws.com). Used by checkS3R53 — Route 53 alias
//     to S3 requires bucket-name == FQDN, so the join key is the record
//     name, NOT a substring of DNSName (which never contains the bucket
//     name in real AWS).
//
// It also reports unread: record sets the scan did not read, past the page
// cap or behind a failed page, so both lists are partial and the pivots
// joining on them owe the operator a lower bound rather than a zero.
func enumerateR53AliasTargets(ctx context.Context, api Route53ListResourceRecordSetsAPI, zoneID string) (aliasTargets, s3WebsiteNames string, unread bool, _ error) {
	sets, complete, err := r53ZoneRecords(ctx, api, zoneID)
	var aliases []string
	for _, rr := range sets {
		if rr.AliasTarget == nil || rr.AliasTarget.DNSName == nil {
			continue
		}
		aliases = append(aliases, *rr.AliasTarget.DNSName)
	}
	return strings.Join(aliases, ","),
		strings.Join(r53S3WebsiteBucketNames(sets), ","),
		!complete,
		err
}

// r53ZoneRecords is the one reader of a zone's record sets. The list pages
// by IsTruncated; the next request starts at NextRecordName, NextRecordType
// and NextRecordIdentifier
// (https://docs.aws.amazon.com/Route53/latest/APIReference/API_ListResourceRecordSets.html).
// A truncated page with no next record is read as a scan that stopped, not a
// whole zone. The cursor joins the three Start* values on a newline, which no
// record name holds.
func r53ZoneRecords(ctx context.Context, api Route53ListResourceRecordSetsAPI, zoneID string) ([]r53types.ResourceRecordSet, bool, error) {
	stuck := false
	sets, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, cursor *string) ([]r53types.ResourceRecordSet, *string, error) {
		in := &route53.ListResourceRecordSetsInput{HostedZoneId: aws.String(zoneID)}
		if cursor != nil {
			name, rest, _ := strings.Cut(*cursor, "\n")
			typ, id, _ := strings.Cut(rest, "\n")
			in.StartRecordName, in.StartRecordType = aws.String(name), r53types.RRType(typ)
			if id != "" {
				in.StartRecordIdentifier = aws.String(id)
			}
		}
		out, err := api.ListResourceRecordSets(ctx, in)
		if err != nil {
			return nil, nil, err
		}
		if out == nil {
			return nil, nil, UnusableAnswerErr{Call: "ListResourceRecordSets", Field: "record sets"}
		}
		if !out.IsTruncated {
			return out.ResourceRecordSets, nil, nil
		}
		if out.NextRecordName == nil {
			stuck = true
			return out.ResourceRecordSets, nil, nil
		}
		next := aws.ToString(out.NextRecordName) + "\n" + string(out.NextRecordType) + "\n" + aws.ToString(out.NextRecordIdentifier)
		return out.ResourceRecordSets, &next, nil
	})
	return sets, complete && !stuck && err == nil, err
}

// r53S3WebsiteBucketNames returns the bucket each S3-website alias record in
// the set addresses. AWS requires such a record's name to equal the bucket
// name, and the alias target is the bare regional endpoint, so the record name
// is the only place the bucket can be read from.
//
// It is the single owner of that derivation: the zone fetcher stores the names
// in Fields["s3website_alias_names"] and checkR53S3 joins them against the
// bucket list, and the two joining on different rules reports a zone's buckets
// in one direction only.
func r53S3WebsiteBucketNames(sets []r53types.ResourceRecordSet) []string {
	var names []string
	for _, rr := range sets {
		if rr.AliasTarget == nil || rr.AliasTarget.DNSName == nil || rr.Name == nil {
			continue
		}
		if !isS3WebsiteEndpoint(*rr.AliasTarget.DNSName) {
			continue
		}
		if name := strings.TrimSuffix(strings.ToLower(*rr.Name), "."); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// isS3WebsiteEndpoint reports whether a Route 53 AliasTarget DNSName is
// an S3 static-website regional endpoint. AWS returns one of:
//   - s3-website-<region>.amazonaws.com.   (hyphen form)
//   - s3-website.<region>.amazonaws.com.   (dot form)
//
// The bucket name is NEVER part of this DNSName — the join to a specific
// bucket is by record name (FQDN) which per AWS must equal the bucket name.
// A CloudFront origin puts the bucket in front of the same endpoint, which is
// why the token is looked for anywhere in the host rather than at the start;
// the host still has to be under amazonaws.com, or an ordinary origin that
// happens to carry the token in its own name reads as an S3 website.
func isS3WebsiteEndpoint(dns string) bool {
	host := strings.TrimSuffix(strings.ToLower(dns), ".")
	// .com.cn is the China partition's suffix; its website endpoints are the
	// same shape and serve HTTP only just the same.
	if !strings.HasSuffix(host, ".amazonaws.com") && !strings.HasSuffix(host, ".amazonaws.com.cn") {
		return false
	}
	return strings.Contains(host, "s3-website-") || strings.Contains(host, "s3-website.")
}
