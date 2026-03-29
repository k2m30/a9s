package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ses", []string{"identity_name", "identity_type", "verification_status", "sending_enabled"})

	resource.RegisterPaginated("ses", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSESIdentitiesPage(ctx, c.SESv2, continuationToken)
	})
}

// FetchSESIdentities calls the SES v2 ListEmailIdentities API and converts the
// response into a slice of generic Resource structs.
func FetchSESIdentities(ctx context.Context, api SESv2ListEmailIdentitiesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchSESIdentitiesPage(ctx, api, token)
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

// FetchSESIdentitiesPage fetches a single page of SES email identities.
func FetchSESIdentitiesPage(ctx context.Context, api SESv2ListEmailIdentitiesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &sesv2.ListEmailIdentitiesInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.ListEmailIdentities(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching SES identities: %w", err)
	}

	var resources []resource.Resource

	for _, identity := range output.EmailIdentities {
		identityName := ""
		if identity.IdentityName != nil {
			identityName = *identity.IdentityName
		}

		identityType := string(identity.IdentityType)
		sendingEnabled := fmt.Sprintf("%t", identity.SendingEnabled)
		verificationStatus := string(identity.VerificationStatus)

		r := resource.Resource{
			ID:     identityName,
			Name:   identityName,
			Status: verificationStatus,
			Fields: map[string]string{
				"identity_name":       identityName,
				"identity_type":       identityType,
				"sending_enabled":     sendingEnabled,
				"verification_status": verificationStatus,
			},
			RawStruct: identity,
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
