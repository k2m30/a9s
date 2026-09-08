// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

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
// REGIONAL NextMarker — the pre-existing external format for the common
// case where CLOUDFRONT needed no resuming (folded in once on page 1 and
// never touched again), round-tripped by the caller with no wrapping — and
// is treated as "resume REGIONAL from this marker, CLOUDFRONT already
// done". A token that DOES decode as JSON but fails strictDecodeCursor's
// other checks — unknown fields, trailing bytes after the JSON value (a
// syntactically valid prefix followed by garbage no longer slips through
// unnoticed), or a decode that lands on the exact zero value (encode()
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

	var resources []resource.Resource

	regionalDone := cur.RegionalDone
	regionalNext := cur.RegionalNext
	if !regionalDone {
		regResult, regErr := fetchWAFWebACLsScopePage(ctx, api, wafv2types.ScopeRegional, regionalNext)
		if regErr != nil {
			return resource.FetchResult{}, regErr
		}
		resources = append(resources, regResult.Resources...)
		regionalNext = ""
		if regResult.Pagination != nil && regResult.Pagination.IsTruncated {
			regionalNext = regResult.Pagination.NextToken
		} else {
			regionalDone = true
		}
	}

	cfDone := cfAPI == nil || cur.CFDone
	cfNext := cur.CFNext
	if !cfDone {
		for pages := 0; ; pages++ {
			cfResult, cfErr := fetchWAFWebACLsScopePage(ctx, cfAPI, wafv2types.ScopeCloudfront, cfNext)
			if cfErr != nil {
				return resource.FetchResult{}, cfErr
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

	isTruncated := !regionalDone || !cfDone
	nextToken := ""
	switch {
	case !isTruncated:
		// nextToken stays "" — both lanes are done.
	case cfDone:
		// CLOUDFRONT needs no more resuming: round-trip the bare REGIONAL
		// marker directly, matching decodeWAFMergedCursor's bare-marker
		// fallback and the pre-existing external format for this common
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
	}, nil
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
	// (REGIONAL scope) and the CLOUDFRONT full-walk loop above, so fixing it
	// here fixes both call sites at once.
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
