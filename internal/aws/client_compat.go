package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// NewAWSSession is a backward-compatible wrapper around NewAWSSessionContext that
// uses context.Background(). Existing callers (tests, integration code) can use
// this signature unchanged. Production fetch paths should prefer NewAWSSessionContext
// and pass a derived context so that app-level cancellation propagates.
func NewAWSSession(profile, region string) (aws.Config, error) {
	return NewAWSSessionContext(context.Background(), profile, region)
}
