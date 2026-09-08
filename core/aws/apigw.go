// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"

	"github.com/k2m30/a9s/v3/core/resource"
)

// apigwMergedCursor encodes the two independent pagination lanes
// FetchAPIGatewaysPageMerged combines: V1Position resumes APIGateway V1's
// own Position-based REST API walk — present only while apigwV1PageCap
// interrupted a still-incomplete walk, since V1 is otherwise fully drained
// on the very first call; V2Token resumes APIGateway V2's GetApis
// NextToken, the ordinary case. An empty continuationToken legitimately
// means "first page of both" and decodes to the zero cursor. Round-tripped
// opaquely — nothing outside this fetcher interprets it.
type apigwMergedCursor struct {
	V1Position string `json:"v1,omitempty"`
	V2Token    string `json:"v2,omitempty"`
}

// decodeAPIGWMergedCursor decodes a compound continuation token previously
// produced by apigwMergedCursor.encode(), via strictDecodeCursor. An empty
// token legitimately means "first page" and returns the zero cursor. Any
// other input that strictDecodeCursor rejects — unknown fields, trailing
// bytes after the JSON value, or a decode that lands on the exact zero
// value (encode() never itself produces one, since a token is only ever
// encoded when there is real state — a non-empty V1Position or V2Token — to
// resume from) — has no legitimate origin other than this same encoder, so
// it is reported as an error rather than silently restarting both lanes
// from page 1 with no signal.
func decodeAPIGWMergedCursor(token string) (apigwMergedCursor, error) {
	if token == "" {
		return apigwMergedCursor{}, nil
	}
	cur, _, malformed := strictDecodeCursor[apigwMergedCursor](token)
	if malformed {
		return apigwMergedCursor{}, fmt.Errorf("decoding API gateway continuation token: invalid or foreign cursor %q", token)
	}
	return cur, nil
}

func (c apigwMergedCursor) encode() string {
	b, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(b)
}

// FetchAPIGatewaysPageMerged fetches a single page of API Gateways from both
// APIGateway V2 (HTTP/WEBSOCKET) and APIGateway V1 (REST), merging results.
// V1 REST APIs are walked via fetchAPIGWV1RestApisBatch, up to
// apigwV1PageCap pages per call, starting on the very first call
// (continuationToken == "") or resuming whenever the previous call's cursor
// carries a V1Position — so a V1 walk interrupted by the cap is continued
// on the NEXT call rather than abandoned; V2 pagination advances one page
// per call the whole time via its own NextToken. The two lanes are
// independent: either can still be truncated while the other is done, and
// the merged result is truncated (with a cursor carrying whichever lane(s)
// still have more) until both report exhaustion.
// V1 REST API resources have Fields["protocol"] == "REST".
// If c.APIGatewayV1 is nil, only V2 resources are returned.
func FetchAPIGatewaysPageMerged(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
	cur, err := decodeAPIGWMergedCursor(continuationToken)
	if err != nil {
		return resource.FetchResult{}, err
	}

	var resources []resource.Resource
	v1Position := cur.V1Position
	if c.APIGatewayV1 != nil && (continuationToken == "" || v1Position != "") {
		v1Resources, resumePosition, v1Err := fetchAPIGWV1RestApisBatch(ctx, c.APIGatewayV1, v1Position)
		if v1Err != nil {
			return resource.FetchResult{}, v1Err
		}
		resources = append(resources, v1Resources...)
		v1Position = resumePosition
	}

	v2Result, err := FetchAPIGatewaysPage(ctx, c.APIGatewayV2, cur.V2Token)
	if err != nil {
		return resource.FetchResult{}, err
	}
	resources = append(resources, v2Result.Resources...)

	v2Token := ""
	v2Truncated := false
	if v2Result.Pagination != nil {
		v2Token = v2Result.Pagination.NextToken
		v2Truncated = v2Result.Pagination.IsTruncated
	}

	isTruncated := v1Position != "" || v2Truncated
	nextToken := ""
	if isTruncated {
		nextToken = apigwMergedCursor{V1Position: v1Position, V2Token: v2Token}.encode()
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

// apigwV1PageCap bounds fetchAPIGWV1RestApisBatch's per-call walk above.
// Regional (600) + private (120) + edge (600) API Gateway quotas are all
// adjustable (increase-on-request), so this is not a hard ceiling — but at
// DefaultPageSize (50) a fully-quota'd DEFAULT account never exceeds 27
// pages (1320 REST APIs / 50), so 40 sits safely above that and can only
// ever fire on an account whose quota was explicitly raised — and even
// then, FetchAPIGatewaysPageMerged's cursor resumes the walk on the next
// call rather than losing the remainder.
const apigwV1PageCap = 40

// fetchAPIGWV1RestApisBatch fetches API Gateway V1 REST APIs starting at
// position (empty for the very first page), stopping after apigwV1PageCap
// pages even if AWS reports more. resumePosition is non-empty only when the
// cap fired before AWS's own pagination reached its terminal (nil-or-empty)
// Position — the caller threads it back into the next call's cursor so the
// walk continues from exactly where this batch stopped.
func fetchAPIGWV1RestApisBatch(ctx context.Context, api APIGatewayV1GetRestApisAPI, position string) (resources []resource.Resource, resumePosition string, err error) {
	pos := position
	for pages := 0; ; pages++ {
		input := &apigateway.GetRestApisInput{
			Limit: aws.Int32(DefaultPageSize),
		}
		if pos != "" {
			input.Position = aws.String(pos)
		}
		out, err := api.GetRestApis(ctx, input)
		if err != nil {
			return nil, "", fmt.Errorf("fetching REST API gateways: %w", err)
		}
		for _, item := range out.Items {
			apiID := aws.ToString(item.Id)
			name := aws.ToString(item.Name)
			description := aws.ToString(item.Description)
			// The endpoint type decides whether an unauthorized API is
			// exposed at all. The enum is one word (PRIVATE, REGIONAL,
			// EDGE) and the console prints it lowercased; absent
			// configuration stays empty, which reads as unknown rather
			// than as public.
			endpoint := ""
			if ec := item.EndpointConfiguration; ec != nil && len(ec.Types) > 0 {
				endpoint = strings.ToLower(string(ec.Types[0]))
			}
			resources = append(resources, resource.Resource{
				ID:   apiID,
				Name: name,
				Fields: map[string]string{
					"api_id":      apiID,
					"name":        name,
					"protocol":    "REST",
					"endpoint":    endpoint,
					"description": description,
				},
				RawStruct: item,
			})
		}
		// A non-nil but empty Position is exhaustion, not "one more page" —
		// GetRestApis has no generated paginator of its own to inherit this
		// contract from.
		if out.Position == nil || *out.Position == "" {
			return resources, "", nil
		}
		if pages+1 >= apigwV1PageCap {
			return resources, *out.Position, nil
		}
		pos = *out.Position
	}
}

// FetchAPIGatewaysPage fetches a single page of API Gateways.
func FetchAPIGatewaysPage(ctx context.Context, api APIGatewayV2GetApisAPI, continuationToken string) (resource.FetchResult, error) {
	input := &apigatewayv2.GetApisInput{
		MaxResults: aws.String(fmt.Sprintf("%d", DefaultPageSize)),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.GetApis(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching API gateways: %w", err)
	}

	var resources []resource.Resource

	for _, item := range output.Items {
		apiID := ""
		if item.ApiId != nil {
			apiID = *item.ApiId
		}

		name := ""
		if item.Name != nil {
			name = *item.Name
		}

		protocol := string(item.ProtocolType)

		endpoint := ""
		if item.ApiEndpoint != nil {
			endpoint = *item.ApiEndpoint
		}

		description := ""
		if item.Description != nil {
			description = *item.Description
		}

		r := resource.Resource{
			ID:   apiID,
			Name: name,
			Fields: map[string]string{
				"api_id":      apiID,
				"name":        name,
				"protocol":    protocol,
				"endpoint":    endpoint,
				"description": description,
			},
			RawStruct: item,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
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
