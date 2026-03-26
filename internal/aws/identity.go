package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
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

	// Best-effort account alias lookup
	if iamClient != nil {
		aliasOut, aliasErr := iamClient.ListAccountAliases(ctx, &iam.ListAccountAliasesInput{})
		if aliasErr == nil && aliasOut != nil && len(aliasOut.AccountAliases) > 0 {
			id.AccountAlias = aliasOut.AccountAliases[0]
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
	arnStr := id.Arn
	if arnStr == "" {
		return
	}

	// ARN format: arn:partition:service:region:account:resource
	parts := strings.SplitN(arnStr, ":", 6)
	if len(parts) < 6 {
		return
	}

	resourcePart := parts[5] // e.g., "assumed-role/ROLE/SESSION" or "user/USERNAME"

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
