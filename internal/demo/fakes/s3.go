// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// S3Fake implements the S3 bucket and object interfaces against fixture data.
type S3Fake struct {
	fix *fixtures.S3Fixtures
}

// NewS3 constructs an S3Fake backed by fixture data from the fixtures package.
func NewS3() *S3Fake {
	return &S3Fake{fix: fixtures.NewS3Fixtures()}
}

func (f *S3Fake) ListBuckets(_ context.Context, input *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	out := &s3.ListBucketsOutput{Buckets: f.fix.Buckets}
	if input.ContinuationToken != nil && *input.ContinuationToken != "" {
		// Demo fixture has no pagination — signal end of list.
		out.Buckets = nil
	}
	return out, nil
}

func (f *S3Fake) ListObjectsV2(_ context.Context, input *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	if input.Bucket == nil {
		return nil, fmt.Errorf("ListObjectsV2: bucket name is required")
	}
	bucket := *input.Bucket
	prefix := ""
	if input.Prefix != nil {
		prefix = *input.Prefix
	}

	objs := f.fix.Objects[bucket][prefix]
	prefixes := f.fix.CommonPrefixes[bucket][prefix]

	// If bucket doesn't exist, return NoSuchBucket.
	if _, ok := f.fix.Objects[bucket]; !ok {
		if _, ok2 := f.fix.CommonPrefixes[bucket]; !ok2 {
			// Check if bucket is in bucket list
			found := false
			for _, b := range f.fix.Buckets {
				if b.Name != nil && *b.Name == bucket {
					found = true
					break
				}
			}
			if !found {
				return nil, &s3types.NoSuchBucket{}
			}
			// Bucket exists but has no objects defined — return empty
			return &s3.ListObjectsV2Output{Name: input.Bucket}, nil
		}
	}

	return &s3.ListObjectsV2Output{
		Name:           input.Bucket,
		Prefix:         input.Prefix,
		Contents:       objs,
		CommonPrefixes: prefixes,
		KeyCount:       int32ptr(int32(len(objs) + len(prefixes))),
	}, nil
}

func (f *S3Fake) GetBucketNotificationConfiguration(_ context.Context, input *s3.GetBucketNotificationConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketNotificationConfigurationOutput, error) {
	if input.Bucket == nil {
		return nil, fmt.Errorf("GetBucketNotificationConfiguration: bucket name is required")
	}
	if cfg, ok := f.fix.NotificationConfigs[*input.Bucket]; ok {
		return cfg, nil
	}
	return &s3.GetBucketNotificationConfigurationOutput{}, nil
}

func (f *S3Fake) GetBucketLocation(_ context.Context, input *s3.GetBucketLocationInput, _ ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error) {
	if input.Bucket == nil {
		return nil, fmt.Errorf("GetBucketLocation: bucket name is required")
	}
	// All demo buckets are in us-east-1; S3 uses empty string for us-east-1.
	return &s3.GetBucketLocationOutput{}, nil
}

func int32ptr(v int32) *int32 { return &v }
