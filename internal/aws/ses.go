package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ses", []string{"identity_name", "identity_type", "verification_status", "sending_enabled", "status"})

	resource.RegisterPaginated("ses", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSESIdentitiesPage(ctx, c.SESv2, continuationToken)
	})

	resource.RegisterRelated("ses", []resource.RelatedDef{
		{TargetType: "r53", DisplayName: "Route 53 (DNS)", Checker: checkSESR53, NeedsTargetCache: true},
		{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkSESEbRule, NeedsTargetCache: true},
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkSESLambda, NeedsTargetCache: false},
		{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkSESS3, NeedsTargetCache: false},
		{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkSESSns, NeedsTargetCache: false},
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

		identityType := string(identity.IdentityType)
		sendingEnabled := strconv.FormatBool(identity.SendingEnabled)
		verificationStatus := string(identity.VerificationStatus)

		findings := sesIdentityFindings(identity)
		topPhrase := sesTopPhrase(findings)

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

func sesIdentityFindings(identity sesv2types.IdentityInfo) []domain.Finding {
	var findings []domain.Finding
	switch identity.VerificationStatus {
	case sesv2types.VerificationStatusFailed:
		findings = append(findings, domain.Finding{Code: CodeSESVerificationFailed, Phrase: "verification failed", Severity: domain.SevBroken, Source: "wave1"})
	case sesv2types.VerificationStatusTemporaryFailure:
		findings = append(findings, domain.Finding{Code: CodeSESVerificationTempFail, Phrase: "verify: temp failure", Severity: domain.SevBroken, Source: "wave1"})
	case sesv2types.VerificationStatusNotStarted:
		findings = append(findings, domain.Finding{Code: CodeSESVerificationNotStarted, Phrase: "verification not started", Severity: domain.SevBroken, Source: "wave1"})
	case sesv2types.VerificationStatusPending:
		findings = append(findings, domain.Finding{Code: CodeSESVerificationPending, Phrase: "pending verification", Severity: domain.SevWarn, Source: "wave1"})
	}
	if !identity.SendingEnabled {
		findings = append(findings, domain.Finding{Code: CodeSESSendingDisabled, Phrase: "sending disabled", Severity: domain.SevWarn, Source: "wave1"})
	}
	return findings
}

func sesTopPhrase(findings []domain.Finding) string {
	if len(findings) == 0 {
		return ""
	}
	if len(findings) == 1 {
		return string(findings[0].Phrase)
	}
	return fmt.Sprintf("%s (+%d)", findings[0].Phrase, len(findings)-1)
}
