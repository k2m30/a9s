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
		topPhrase := domain.StatusPhrase(findings)

		r := resource.Resource{
			ID:       identityName,
			Name:     identityName,
			Findings: findings,
			Fields: map[string]string{
				"identity_name":       identityName,
				"identity_type":       identityType,
				"sending_enabled":     sendingEnabled,
				"verification_status": verificationStatus,
				"status":              topPhrase,
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

// The words the Type column renders for the two SES identity kinds. The
// SDK's own DOMAIN / EMAIL_ADDRESS constants never reach a rendered surface,
// and Fields["identity_type"] carries the word, so every consumer of that
// field compares against these.
const (
	sesIdentityTypeDomain       = "domain"
	sesIdentityTypeEmailAddress = "email address"
)

// sesIdentityTypeWord maps the SDK identity type onto the rendered word.
func sesIdentityTypeWord(t sesv2types.IdentityType) string {
	switch t {
	case sesv2types.IdentityTypeEmailAddress:
		return sesIdentityTypeEmailAddress
	case sesv2types.IdentityTypeDomain:
		return sesIdentityTypeDomain
	default:
		return domain.HumanizeStatusPhrase(string(t))
	}
}

func sesIdentityFindings(identity sesv2types.IdentityInfo) []domain.Finding {
	var findings []domain.Finding
	switch identity.VerificationStatus {
	case sesv2types.VerificationStatusFailed:
		findings = append(findings, wave1Finding(CodeSESVerificationFailed))
	case sesv2types.VerificationStatusTemporaryFailure:
		findings = append(findings, wave1Finding(CodeSESVerificationTempFail))
	case sesv2types.VerificationStatusNotStarted:
		findings = append(findings, wave1Finding(CodeSESVerificationNotStarted))
	case sesv2types.VerificationStatusPending:
		findings = append(findings, wave1Finding(CodeSESVerificationPending))
	}
	if !identity.SendingEnabled {
		findings = append(findings, wave1Finding(CodeSESSendingDisabled))
	}
	return findings
}
