// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

// aws3_partial_failure_vocabulary_test.go — a resource that refused more
// than one check says so, and a walk stopped by a failed page names the page
// as a page rather than posing as a resource id.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// s3TwoDenialsFake refuses two different per-bucket calls with two different
// actions, which is what a scoped read-only role actually looks like.
type s3TwoDenialsFake struct{ awsclient.S3API }

func (f *s3TwoDenialsFake) GetPublicAccessBlock(_ context.Context, _ *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
	return nil, &smithy.GenericAPIError{Code: "AccessDenied", Message: "User is not authorized to perform: s3:GetBucketPublicAccessBlock"}
}

func (f *s3TwoDenialsFake) GetBucketPolicyStatus(_ context.Context, _ *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	return nil, &smithy.GenericAPIError{Code: "AccessDenied", Message: "User is not authorized to perform: s3:GetBucketPolicyStatus"}
}

func (f *s3TwoDenialsFake) GetBucketVersioning(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	return &s3.GetBucketVersioningOutput{}, nil
}

func (f *s3TwoDenialsFake) GetBucketLogging(_ context.Context, _ *s3.GetBucketLoggingInput, _ ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	return &s3.GetBucketLoggingOutput{}, nil
}

func (f *s3TwoDenialsFake) GetBucketLifecycleConfiguration(_ context.Context, _ *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	return &s3.GetBucketLifecycleConfigurationOutput{}, nil
}

func (f *s3TwoDenialsFake) GetObjectLockConfiguration(_ context.Context, _ *s3.GetObjectLockConfigurationInput, _ ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	return &s3.GetObjectLockConfigurationOutput{}, nil
}

// TestS3BucketDeniedTwoChecksNamesBoth pins that a bucket refused two
// different actions reports both. Keeping only the first told the operator to
// grant one permission, after which the bucket would fail again on the second.
func TestS3BucketDeniedTwoChecksNamesBoth(t *testing.T) {
	clients := &awsclient.ServiceClients{S3: &s3TwoDenialsFake{}}
	rows := []resource.Resource{{ID: "acme-logs", Name: "acme-logs", Fields: map[string]string{"name": "acme-logs"}}}

	_, err := awsclient.EnrichS3Posture(context.Background(), clients, rows, nil)
	if err == nil {
		t.Fatal("expected a composite error for a bucket that refused two checks")
	}
	for _, action := range []string{"s3:GetBucketPublicAccessBlock", "s3:GetBucketPolicyStatus"} {
		if !strings.Contains(err.Error(), action) {
			t.Errorf("composite error %q does not name %s — one denial is speaking for the other", err.Error(), action)
		}
	}
}
