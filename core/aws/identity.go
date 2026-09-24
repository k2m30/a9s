// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// CallerIdentity holds parsed information about the current AWS caller.
type CallerIdentity struct {
	AccountID     string
	AccountAlias  string
	Arn           string
	UserID        string
	IdentityName  string // role name or user name — for header display
	RoleName      string
	UserName      string
	SessionName   string
	IsAssumedRole bool
}

// FetchCallerIdentity calls STS GetCallerIdentity and IAM ListAccountAliases
// to build a CallerIdentity. The IAM alias lookup is best-effort (non-fatal).
func FetchCallerIdentity(ctx context.Context, stsClient STSGetCallerIdentityAPI, iamClient IAMListAccountAliasesAPI) (*CallerIdentity, error) {
	out, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, err
	}

	id := &CallerIdentity{
		AccountID: aws.ToString(out.Account),
		Arn:       aws.ToString(out.Arn),
		UserID:    aws.ToString(out.UserId),
	}

	parseARN(id)

	if iamClient != nil {
		// ListAccountAliases pages by IsTruncated and Marker
		// (https://docs.aws.amazon.com/IAM/latest/APIReference/API_ListAccountAliases.html).
		aliases, _, aliasErr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]string, *string, error) {
			out, err := iamClient.ListAccountAliases(ctx, &iam.ListAccountAliasesInput{Marker: marker})
			if err != nil {
				return nil, nil, err
			}
			return out.AccountAliases, iamNextMarker(out.IsTruncated, out.Marker), nil
		})
		if aliasErr == nil && len(aliases) > 0 {
			id.AccountAlias = aliases[0]
		}
	}

	return id, nil
}

// parseARN extracts role/user/session from the ARN.
//
// Patterns:
//   - arn:aws:sts::ACCOUNT:assumed-role/ROLE/SESSION
//   - arn:aws:iam::ACCOUNT:user/USERNAME
//   - arn:aws:iam::ACCOUNT:user/PATH/USERNAME
//   - arn:aws:sts::ACCOUNT:federated-user/NAME
func parseARN(id *CallerIdentity) {
	a, err := arn.Parse(id.Arn)
	if err != nil {
		return
	}
	resourcePart := a.Resource

	switch {
	case strings.HasPrefix(resourcePart, "assumed-role/"):
		segments := strings.SplitN(resourcePart, "/", 3)
		if len(segments) >= 2 {
			id.RoleName = segments[1]
			id.IdentityName = segments[1]
			id.IsAssumedRole = true
		}
		if len(segments) >= 3 {
			id.SessionName = segments[2]
		}
	case strings.HasPrefix(resourcePart, "user/"):
		// May have a path: user/path/to/USERNAME
		segments := strings.Split(resourcePart, "/")
		userName := segments[len(segments)-1]
		id.UserName = userName
		id.IdentityName = userName
	case strings.HasPrefix(resourcePart, "federated-user/"):
		segments := strings.SplitN(resourcePart, "/", 2)
		if len(segments) >= 2 {
			id.UserName = segments[1]
			id.IdentityName = segments[1]
		}
	}
}
