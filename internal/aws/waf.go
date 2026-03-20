package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("waf", []string{"name", "id", "description"})
	resource.Register("waf", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchWAFWebACLs(ctx, c.WAFv2)
	})
}

// FetchWAFWebACLs calls the WAFv2 ListWebACLs API with Scope=REGIONAL and converts
// the response into a slice of generic Resource structs.
func FetchWAFWebACLs(ctx context.Context, api WAFv2ListWebACLsAPI) ([]resource.Resource, error) {
	output, err := api.ListWebACLs(ctx, &wafv2.ListWebACLsInput{
		Scope: wafv2types.ScopeRegional,
	})
	if err != nil {
		return nil, fmt.Errorf("fetching WAF web ACLs: %w", err)
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
			},
			RawStruct: acl,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
