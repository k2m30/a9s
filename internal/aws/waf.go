package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchWAFWebACLs calls the WAFv2 ListWebACLs API with Scope=REGIONAL and converts
// the response into a slice of generic Resource structs.
func FetchWAFWebACLs(ctx context.Context, api WAFv2ListWebACLsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchWAFWebACLsPage(ctx, api, token)
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

// FetchWAFWebACLsPage fetches a single page of WAF web ACLs.
func FetchWAFWebACLsPage(ctx context.Context, api WAFv2ListWebACLsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &wafv2.ListWebACLsInput{
		Scope: wafv2types.ScopeRegional,
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
			ID:     id,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"name":        name,
				"id":          id,
				"arn":         arn,
				"description": description,
				"lock_token":  lockToken,
				"scope":       string(wafv2types.ScopeRegional),
			},
			RawStruct: acl,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextMarker != nil {
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
