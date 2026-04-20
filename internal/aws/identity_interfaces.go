package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// STSGetCallerIdentityAPI defines the interface for the STS GetCallerIdentity operation.
type STSGetCallerIdentityAPI interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}
