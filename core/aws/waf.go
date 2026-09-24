// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// wafScopeCloudFront is the scope of a web ACL that protects CloudFront
// distributions, as the fetcher writes it into Fields["scope"].
const wafScopeCloudFront = string(wafv2types.ScopeCloudfront)

// wafRegionOf returns the region a web ACL of scope, and the logging
// destination AWS requires to sit beside it, live in. AWS serves every
// CLOUDFRONT-scope operation from the us-east-1 endpoint alone, whatever
// region the session is browsing; a REGIONAL ACL is in the session's own.
func wafRegionOf(scope string) string {
	if scope == wafScopeCloudFront {
		return "us-east-1"
	}
	return ""
}

// wafIn returns the WAFv2 client that reads a web ACL of scope.
func (c *ServiceClients) wafIn(scope string) WAFv2API {
	return c.InRegion(wafRegionOf(scope)).WAFv2
}

// wafDistributionIDs returns the CloudFront distributions the web ACL at
// webACLArn is associated with. AWS answers this through CloudFront alone:
// wafv2:ListResourcesForWebACL covers the regional resource types and its
// reference directs CloudFront callers to ListDistributionsByWebACLId.
// complete is false when the answer stopped at PerParentPageCap pages.
func wafDistributionIDs(ctx context.Context, c *ServiceClients, webACLArn string) (ids []string, complete bool, err error) {
	api, ok := c.CloudFront.(CloudFrontListDistributionsByWebACLIdAPI)
	if !ok || c.CloudFront == nil {
		return nil, false, errClientMissing
	}
	return PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]string, *string, error) {
		out, err := api.ListDistributionsByWebACLId(ctx, &cloudfront.ListDistributionsByWebACLIdInput{WebACLId: &webACLArn, Marker: marker})
		if err != nil {
			return nil, nil, err
		}
		if out.DistributionList == nil {
			return nil, nil, UnusableAnswerErr{Call: "ListDistributionsByWebACLId", Field: "distribution list"}
		}
		var ids []string
		for _, d := range out.DistributionList.Items {
			if d.Id != nil && *d.Id != "" {
				ids = append(ids, *d.Id)
			}
		}
		return ids, out.DistributionList.NextMarker, nil
	})
}

// wafMergedCursor encodes the two independent pagination lanes
// FetchWAFWebACLsPageWithCloudFront combines: the REGIONAL scope's own
// NextMarker-based walk (the caller's ordinary load-more path) and the
// CLOUDFRONT scope's full account-wide walk, which is folded in only until
// it drains — CFDone latches true the moment CLOUDFRONT is fully listed, so
// it is never re-listed on a later REGIONAL page. RegionalDone/CFDone are
// tracked explicitly (rather than inferred from an empty next-token) because
// an empty token means "not yet started" on the very first call and "fully
// drained" on every later one — the two are indistinguishable without a
// separate flag. An empty continuationToken means "first page of both" and
// decodes to the zero cursor. Round-tripped opaquely — nothing outside this
// fetcher interprets it.
type wafMergedCursor struct {
	RegionalNext string `json:"rn,omitempty"`
	RegionalDone bool   `json:"rd,omitempty"`
	CFNext       string `json:"cn,omitempty"`
	CFDone       bool   `json:"cd,omitempty"`
}

// decodeWAFMergedCursor decodes a continuation token previously produced by
// this fetcher, via strictDecodeCursor. An empty token legitimately means
// "first page" and returns the zero cursor. A token that fails the
// underlying json.Decode outright (strictDecodeCursor's decodeErr) is a bare
// REGIONAL NextMarker — the external format for the common
// case where CLOUDFRONT needed no resuming (folded in once on page 1 and
// never touched again), round-tripped by the caller with no wrapping — and
// is treated as "resume REGIONAL from this marker, CLOUDFRONT already
// done". A token that DOES decode as JSON but fails strictDecodeCursor's
// other checks — unknown fields, trailing bytes after the JSON value, or a
// decode that lands on the exact zero value (encode()
// never itself produces one, since a token is only ever encoded when there
// is real state — a non-empty next-token or a Done flag — to resume from)
// — has no legitimate origin other than this same encoder, so it is
// reported as an error rather than silently restarting both lanes from
// page 1 with no signal.
func decodeWAFMergedCursor(token string) (wafMergedCursor, error) {
	if token == "" {
		return wafMergedCursor{}, nil
	}
	cur, decodeErr, malformed := strictDecodeCursor[wafMergedCursor](token)
	if decodeErr != nil {
		return wafMergedCursor{RegionalNext: token, CFDone: true}, nil
	}
	if malformed {
		return wafMergedCursor{}, fmt.Errorf("decoding WAF web ACL continuation token: invalid or foreign cursor %q", token)
	}
	return cur, nil
}

func (c wafMergedCursor) encode() string {
	b, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(b)
}

// FetchWAFWebACLsPageWithCloudFront fetches a single page of REGIONAL-scope
// WAF web ACLs and, on every call until fully drained, also pages through
// CLOUDFRONT-scope ACLs and appends them. CLOUDFRONT-scope ACLs are
// account-wide/global (not region-paginated the way REGIONAL ACLs are), so
// they are walked to exhaustion (bounded per call by wafCloudFrontPageCap)
// and folded into the merged result rather than paginated alongside
// REGIONAL — once CLOUDFRONT reports done, it is never re-listed on a
// later REGIONAL page. If CLOUDFRONT's own walk is not yet exhausted when
// the cap fires, its resume marker is carried in the returned cursor so the
// NEXT call continues it instead of abandoning the remainder; the merged
// result's IsTruncated/NextToken only ever go to "complete" once both
// REGIONAL and CLOUDFRONT report done.
// cfAPI may be nil (e.g. not yet wired) — in that case only REGIONAL ACLs
// are returned and the CLOUDFRONT lane is treated as already done.
func FetchWAFWebACLsPageWithCloudFront(ctx context.Context, api WAFv2ListWebACLsAPI, cfAPI WAFv2ListWebACLsAPI, continuationToken string) (resource.FetchResult, error) {
	cur, err := decodeWAFMergedCursor(continuationToken)
	if err != nil {
		return resource.FetchResult{}, err
	}

	// A scope that fails keeps the rows both scopes read, and the cursor
	// resumes it where it failed: the page is a lower bound carrying the
	// error.
	var resources []resource.Resource
	var errs []error

	regionalDone := cur.RegionalDone
	regionalNext := cur.RegionalNext
	if !regionalDone {
		regResult, regErr := fetchWAFWebACLsScopePage(ctx, api, wafv2types.ScopeRegional, regionalNext)
		switch {
		case regErr != nil:
			errs = append(errs, regErr)
		case regResult.Pagination != nil && regResult.Pagination.IsTruncated:
			regionalNext = regResult.Pagination.NextToken
		default:
			regionalNext, regionalDone = "", true
		}
		resources = append(resources, regResult.Resources...)
	}

	cfDone := cfAPI == nil || cur.CFDone
	cfNext := cur.CFNext
	if !cfDone {
		for pages := 0; ; pages++ {
			cfResult, cfErr := fetchWAFWebACLsScopePage(ctx, cfAPI, wafv2types.ScopeCloudfront, cfNext)
			if cfErr != nil {
				errs = append(errs, cfErr)
				break
			}
			resources = append(resources, cfResult.Resources...)
			if cfResult.Pagination == nil || !cfResult.Pagination.IsTruncated {
				cfDone = true
				cfNext = ""
				break
			}
			cfNext = cfResult.Pagination.NextToken
			if pages+1 >= wafCloudFrontPageCap {
				break
			}
		}
	}
	err = errors.Join(errs...)
	if err != nil && len(resources) == 0 {
		return resource.FetchResult{}, err
	}

	isTruncated := !regionalDone || !cfDone
	nextToken := ""
	switch {
	case !isTruncated:
		// nextToken stays "" — both lanes are done.
	case cfDone && regionalNext != "":
		// CLOUDFRONT needs no more resuming: round-trip the bare REGIONAL
		// marker directly, matching decodeWAFMergedCursor's bare-marker
		// fallback, the external format for this common
		// case.
		nextToken = regionalNext
	default:
		nextToken = wafMergedCursor{
			RegionalNext: regionalNext,
			RegionalDone: regionalDone,
			CFNext:       cfNext,
			CFDone:       cfDone,
		}.encode()
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
	}, err
}

// wafCloudFrontPageCap bounds the CLOUDFRONT-scope walk above to this many
// pages per call: it runs on every call until CLOUDFRONT reports done,
// account-wide and un-paged from the caller's perspective, so a belt against
// runaway enumeration belongs here even though fetchWAFWebACLsScopePage's own
// empty-marker guard already stops the common case (a non-nil, empty
// NextMarker) from spinning forever. If a single account's CLOUDFRONT ACL set
// needs more than wafCloudFrontPageCap pages, the walk resumes across
// multiple calls rather than losing the remainder.
const wafCloudFrontPageCap = 20

// fetchWAFWebACLsScopePage fetches a single page of WAF web ACLs for the
// given scope (REGIONAL or CLOUDFRONT).
func fetchWAFWebACLsScopePage(ctx context.Context, api WAFv2ListWebACLsAPI, scope wafv2types.Scope, continuationToken string) (resource.FetchResult, error) {
	input := &wafv2.ListWebACLsInput{
		Scope: scope,
		Limit: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextMarker = &continuationToken
	}

	output, err := api.ListWebACLs(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching WAF web ACLs: %w", err)
	}

	var resources []resource.Resource

	for _, acl := range output.WebACLs {
		name := ""
		if acl.Name != nil {
			name = *acl.Name
		}

		id := ""
		if acl.Id != nil {
			id = *acl.Id
		}

		arn := ""
		if acl.ARN != nil {
			arn = *acl.ARN
		}

		description := ""
		if acl.Description != nil {
			description = *acl.Description
		}

		lockToken := ""
		if acl.LockToken != nil {
			lockToken = *acl.LockToken
		}

		r := resource.Resource{
			ID:   id,
			Name: name,
			Fields: map[string]string{
				"name":        name,
				"id":          id,
				"arn":         arn,
				"description": description,
				"lock_token":  lockToken,
				"scope":       string(scope),
			},
			RawStruct: acl,
		}

		resources = append(resources, r)
	}

	// A non-nil but empty NextMarker is exhaustion, not "one more page" — the
	// same nil-or-empty contract every AWS-generated paginator in this SDK
	// enforces (wafv2 ListWebACLs has no generated paginator of its own to
	// inherit it from). Read by both the caller's own load-more pagination
	// (REGIONAL scope) and the CLOUDFRONT full-walk loop above.
	nextToken := ""
	isTruncated := false
	if output.NextMarker != nil && *output.NextMarker != "" {
		nextToken = *output.NextMarker
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

// wafWithMetricName is the web ACL row carrying Fields["metric_name"], the
// ACL's VisibilityConfig.MetricName, read with wafv2:GetWebACL when the list
// response left it out: ListWebACLs returns summaries without it.
func wafWithMetricName(ctx context.Context, clients any, row resource.Resource) (resource.Resource, error) {
	if row.Fields["metric_name"] != "" {
		return row, nil
	}
	name, id := row.Fields["name"], row.Fields["id"]
	if name == "" || id == "" {
		return row, nil
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return row, errClientMissing
	}
	api, ok := c.wafIn(row.Fields["scope"]).(WAFv2GetWebACLAPI)
	if !ok {
		return row, errClientMissing
	}
	scope := wafv2types.ScopeRegional
	if row.Fields["scope"] == wafScopeCloudFront {
		scope = wafv2types.ScopeCloudfront
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*wafv2.GetWebACLOutput, error) {
		return api.GetWebACL(ctx, &wafv2.GetWebACLInput{Name: &name, Id: &id, Scope: scope})
	})
	if err != nil {
		return row, err
	}
	if out.WebACL == nil || out.WebACL.VisibilityConfig == nil {
		return row, nil
	}
	fields := maps.Clone(row.Fields)
	fields["metric_name"] = aws.ToString(out.WebACL.VisibilityConfig.MetricName)
	row.Fields = fields
	return row, nil
}
