package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

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
		{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkSESEbRule, NeedsTargetCache: false},
		{TargetType: "kinesis", DisplayName: "Kinesis Streams", Checker: checkSESKinesis, NeedsTargetCache: false},
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

		topPhrase, issues := computeSESStatusAndIssues(identity)

		r := resource.Resource{
			ID:     identityName,
			Name:   identityName,
			Status: topPhrase,
			Issues: issues,
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

// computeSESStatusAndIssues maps Wave-1 signals from an IdentityInfo to the
// top S4 phrase (with `(+N)` suffix when multiple issues coexist) and the full
// ordered issue slice (spec §4 precedence table, impl-plan §3.2).
//
// Precedence order:
//  1. VerificationStatus==FAILED         → "verification failed" (Broken)
//  2. VerificationStatus==TEMPORARY_FAILURE → "verify: temp failure" (Broken)
//  3. VerificationStatus==NOT_STARTED    → "verification not started" (Broken)
//  4. VerificationStatus==PENDING        → "pending verification" (Warning)
//  5. SendingEnabled==false (any row)    → append "sending disabled" (Warning)
//  6. Healthy (SUCCESS + enabled)        → "", nil
func computeSESStatusAndIssues(identity sesv2types.IdentityInfo) (string, []string) {
	var issues []string

	switch identity.VerificationStatus {
	case sesv2types.VerificationStatusFailed:
		issues = append(issues, "verification failed")
	case sesv2types.VerificationStatusTemporaryFailure:
		issues = append(issues, "verify: temp failure")
	case sesv2types.VerificationStatusNotStarted:
		issues = append(issues, "verification not started")
	case sesv2types.VerificationStatusPending:
		issues = append(issues, "pending verification")
	}

	if !identity.SendingEnabled {
		issues = append(issues, "sending disabled")
	}

	switch len(issues) {
	case 0:
		return "", nil
	case 1:
		return issues[0], issues
	default:
		return fmt.Sprintf("%s (+%d)", issues[0], len(issues)-1), issues
	}
}
