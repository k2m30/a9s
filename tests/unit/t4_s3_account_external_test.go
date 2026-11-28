package unit_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3control"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/session"
)

// t4NoAccountBlock answers S3 Control GetPublicAccessBlock for an account
// that never configured an account-level block.
type t4NoAccountBlock struct{}

func (t4NoAccountBlock) GetPublicAccessBlock(context.Context, *s3control.GetPublicAccessBlockInput, ...func(*s3control.Options)) (*s3control.GetPublicAccessBlockOutput, error) {
	return nil, &smithy.GenericAPIError{
		Code:    "NoSuchPublicAccessBlockConfiguration",
		Message: "The public access block configuration was not found",
	}
}

func t4WithAccount(c *awsclient.ServiceClients) *awsclient.ServiceClients {
	c.S3Control = t4NoAccountBlock{}
	id := session.NewIdentityStore()
	id.Set("123456789012", nil)
	c.SetIdentityStore(id)
	return c
}
