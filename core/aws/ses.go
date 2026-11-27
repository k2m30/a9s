// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchSESIdentitiesPage fetches a single page of SES email identities.
func FetchSESIdentitiesPage(ctx context.Context, api SESv2ListEmailIdentitiesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &sesv2.ListEmailIdentitiesInput{
		PageSize: aws.Int32(DefaultPageSize),
	}
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

		identityType := sesIdentityTypeWord(identity.IdentityType)
		sendingEnabled := strconv.FormatBool(identity.SendingEnabled)
		verificationStatus := string(identity.VerificationStatus)

		findings := sesIdentityFindings(identity)

		r := withWave1(resource.Resource{
			ID:   identityName,
			Name: identityName,
			Fields: map[string]string{
				"identity_name":       identityName,
				"identity_type":       identityType,
				"sending_enabled":     sendingEnabled,
				"verification_status": verificationStatus,
				"status":              verificationStatus,
			},
			RawStruct: identity,
		}, findings...)

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

// The words the Type column renders for the three SES identity types. The
// SDK's own constants never reach a rendered surface,
// and Fields["identity_type"] carries the word, so every consumer of that
// field compares against these.
const (
	sesIdentityTypeDomain       = "domain"
	sesIdentityTypeEmailAddress = "email address"
	sesIdentityTypeManaged      = "managed domain"
)

var sesIdentityTypeWords = map[sesv2types.IdentityType]string{ //nolint:gochecknoglobals // static AWS value set
	sesv2types.IdentityTypeEmailAddress:  sesIdentityTypeEmailAddress,
	sesv2types.IdentityTypeDomain:        sesIdentityTypeDomain,
	sesv2types.IdentityTypeManagedDomain: sesIdentityTypeManaged,
}

// sesIdentityTypeWord maps the SDK identity type onto the rendered word.
func sesIdentityTypeWord(t sesv2types.IdentityType) string {
	if word, ok := sesIdentityTypeWords[t]; ok {
		return word
	}
	return domain.HumanizeStatusPhrase(string(t))
}

// sesKind is the identity's IdentityType. A row rebuilt from Fields alone
// holds the rendered word or the type itself, each naming exactly one type.
func sesKind(r domain.Resource) string {
	if identity, ok := assertStruct[sesv2types.IdentityInfo](r.RawStruct); ok {
		return string(identity.IdentityType)
	}
	for _, t := range sesv2types.IdentityType("").Values() {
		if stored := r.Fields["identity_type"]; stored == string(t) || stored == sesIdentityTypeWord(t) {
			return string(t)
		}
	}
	return ""
}

func sesIdentityFindings(identity sesv2types.IdentityInfo) []domain.Finding {
	var posture []domain.Finding
	if !identity.SendingEnabled {
		posture = append(posture, wave1Finding(CodeSESSendingDisabled))
	}
	return withState("ses", string(identity.VerificationStatus), posture)
}
