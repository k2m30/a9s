// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"
	"fmt"
	"slices"

	smithy "github.com/aws/smithy-go"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// S3Fake implements the S3 bucket and object interfaces against fixture data.
type S3Fake struct {
	fix *fixtures.S3Fixtures
}

// NewS3 constructs an S3Fake backed by fixture data from the fixtures package.
func NewS3() *S3Fake {
	return &S3Fake{fix: fixtures.NewS3Fixtures()}
}

// NewS3FromFixturesForTest constructs an S3Fake backed by a caller-supplied
// fixture set, bypassing the default NewS3Fixtures builder. Test-only: no
// production caller — demo mode always goes through NewS3. Intended for
// tests that need to override specific maps (BucketPolicies,
// EncryptionConfigs, …) without mutating shared demo state.
func NewS3FromFixturesForTest(f *fixtures.S3Fixtures) *S3Fake {
	return &S3Fake{fix: f}
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
		KeyCount:       new(int32(len(objs) + len(prefixes))),
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

// HeadBucket answers for this account's buckets and for the ones the fixtures
// place in another account. Only a name in neither is NotFound, which is the
// one answer that proves a bucket does not exist.
func (f *S3Fake) HeadBucket(_ context.Context, input *s3.HeadBucketInput, _ ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	if input.Bucket == nil {
		return nil, fmt.Errorf("HeadBucket: bucket name is required")
	}
	if f.fix.CrossAccountBuckets[*input.Bucket] {
		return &s3.HeadBucketOutput{}, nil
	}
	for _, b := range f.fix.Buckets {
		if b.Name != nil && *b.Name == *input.Bucket {
			return &s3.HeadBucketOutput{}, nil
		}
	}
	return nil, &s3types.NotFound{}
}

func (f *S3Fake) GetBucketLocation(_ context.Context, input *s3.GetBucketLocationInput, _ ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error) {
	if input.Bucket == nil {
		return nil, fmt.Errorf("GetBucketLocation: bucket name is required")
	}
	// All demo buckets are in us-east-1; S3 uses empty string for us-east-1.
	return &s3.GetBucketLocationOutput{}, nil
}

// GetPublicAccessBlock dispatches per-bucket PAB configuration.
// Semantics driven by PublicAccessBlockConfigs map in fixtures:
//   - Key present, non-nil value → return that output (may have nil inner config).
//   - Key present, nil value     → return NoSuchPublicAccessBlockConfiguration Smithy error.
//   - Key absent                 → return all-flags-true output (healthy default).
//
// The healthy default for absent keys ensures non-spec buckets (CloudTrail-event
// fixtures, name-pool fillers) don't noise up the S1 issues badge. Only the 4
// spec-driven finding fixtures register explicit configs in the map.
func (f *S3Fake) GetPublicAccessBlock(_ context.Context, input *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
	if input.Bucket == nil {
		return nil, fmt.Errorf("GetPublicAccessBlock: bucket name is required")
	}
	bucket := *input.Bucket
	cfg, ok := f.fix.PublicAccessBlockConfigs[bucket]
	if !ok {
		t := true
		return &s3.GetPublicAccessBlockOutput{
			PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
				BlockPublicAcls:       &t,
				IgnorePublicAcls:      &t,
				BlockPublicPolicy:     &t,
				RestrictPublicBuckets: &t,
			},
		}, nil
	}
	if cfg == nil {
		// Explicit nil → simulate bucket with no PAB config set.
		return nil, &smithy.GenericAPIError{
			Code:    "NoSuchPublicAccessBlockConfiguration",
			Message: "The public access block configuration was not found",
		}
	}
	return cfg, nil
}

// GetBucketEncryption returns the server-side encryption configuration for a bucket.
// If the bucket has no entry in EncryptionConfigs, returns ServerSideEncryptionConfigurationNotFoundError.
func (f *S3Fake) GetBucketEncryption(_ context.Context, input *s3.GetBucketEncryptionInput, _ ...func(*s3.Options)) (*s3.GetBucketEncryptionOutput, error) {
	if input.Bucket == nil {
		return nil, fmt.Errorf("GetBucketEncryption: bucket name is required")
	}
	bucket := *input.Bucket
	cfg, ok := f.fix.EncryptionConfigs[bucket]
	if !ok {
		return nil, &smithy.GenericAPIError{
			Code:    "ServerSideEncryptionConfigurationNotFoundError",
			Message: "The server side encryption configuration was not found",
		}
	}
	return cfg, nil
}

// GetBucketTagging returns the tag set for a bucket.
// If the bucket has no entry in TaggingConfigs, returns NoSuchTagSet.
func (f *S3Fake) GetBucketTagging(_ context.Context, input *s3.GetBucketTaggingInput, _ ...func(*s3.Options)) (*s3.GetBucketTaggingOutput, error) {
	if input.Bucket == nil {
		return nil, fmt.Errorf("GetBucketTagging: bucket name is required")
	}
	bucket := *input.Bucket
	cfg, ok := f.fix.TaggingConfigs[bucket]
	if !ok {
		return nil, &smithy.GenericAPIError{
			Code:    "NoSuchTagSet",
			Message: "The TagSet does not exist",
		}
	}
	return cfg, nil
}

// GetBucketLogging returns the logging configuration for a bucket.
// If the bucket has no entry in LoggingConfigs, returns an empty output (no logging configured).
func (f *S3Fake) GetBucketLogging(_ context.Context, input *s3.GetBucketLoggingInput, _ ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	if input.Bucket == nil {
		return nil, fmt.Errorf("GetBucketLogging: bucket name is required")
	}
	bucket := *input.Bucket
	cfg, ok := f.fix.LoggingConfigs[bucket]
	if !ok {
		return &s3.GetBucketLoggingOutput{}, nil
	}
	return cfg, nil
}

// GetBucketPolicy returns the bucket's resource-policy JSON document.
// Buckets with no entry in BucketPolicies get NoSuchBucketPolicy —
// the canonical AWS response the checker honors as a legitimate 0.
func (f *S3Fake) GetBucketPolicy(_ context.Context, input *s3.GetBucketPolicyInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyOutput, error) {
	if input.Bucket == nil {
		return nil, fmt.Errorf("GetBucketPolicy: bucket name is required")
	}
	policy, ok := f.fix.BucketPolicies[*input.Bucket]
	if !ok || policy == "" {
		return nil, &smithy.GenericAPIError{
			Code:    "NoSuchBucketPolicy",
			Message: "The bucket policy does not exist",
		}
	}
	return &s3.GetBucketPolicyOutput{Policy: &policy}, nil
}

// GetBucketCors returns the bucket's CORS rules. Buckets with no entry in
// CORSConfigs get NoSuchCORSConfiguration — the canonical AWS response the
// s3 detail enricher honors as a legitimate 0.
func (f *S3Fake) GetBucketCors(_ context.Context, input *s3.GetBucketCorsInput, _ ...func(*s3.Options)) (*s3.GetBucketCorsOutput, error) {
	if input.Bucket == nil {
		return nil, fmt.Errorf("GetBucketCors: bucket name is required")
	}
	rules, ok := f.fix.CORSConfigs[*input.Bucket]
	if !ok {
		return nil, &smithy.GenericAPIError{
			Code:    "NoSuchCORSConfiguration",
			Message: "The CORS configuration does not exist",
		}
	}
	return &s3.GetBucketCorsOutput{CORSRules: rules}, nil
}

// GetBucketLifecycleConfiguration returns the bucket's lifecycle rules.
// Buckets with no entry in LifecycleConfigs get NoSuchLifecycleConfiguration
// — the canonical AWS response the s3 detail enricher honors as a
// legitimate 0.
func (f *S3Fake) GetBucketLifecycleConfiguration(_ context.Context, input *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	if input.Bucket == nil {
		return nil, fmt.Errorf("GetBucketLifecycleConfiguration: bucket name is required")
	}
	rules, ok := f.fix.LifecycleConfigs[*input.Bucket]
	if !ok {
		return nil, &smithy.GenericAPIError{
			Code:    "NoSuchLifecycleConfiguration",
			Message: "The lifecycle configuration does not exist",
		}
	}
	return &s3.GetBucketLifecycleConfigurationOutput{Rules: rules}, nil
}

// GetBucketPolicyStatus returns AWS's own public/not-public verdict on the
// bucket policy. Buckets with no entry in PolicyStatuses are not public —
// the healthy default, so only the witness bucket lights up the finding.
func (f *S3Fake) GetBucketPolicyStatus(_ context.Context, input *s3.GetBucketPolicyStatusInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	if input.Bucket == nil {
		return nil, fmt.Errorf("GetBucketPolicyStatus: bucket name is required")
	}
	if out, ok := f.fix.PolicyStatuses[*input.Bucket]; ok && out != nil {
		return out, nil
	}
	return &s3.GetBucketPolicyStatusOutput{
		PolicyStatus: &s3types.PolicyStatus{IsPublic: new(false)},
	}, nil
}

// GetBucketVersioning returns the bucket's versioning state. Buckets with no
// entry in VersioningConfigs are versioned with MFA delete enabled — the
// healthy default.
func (f *S3Fake) GetBucketVersioning(_ context.Context, input *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	if input.Bucket == nil {
		return nil, fmt.Errorf("GetBucketVersioning: bucket name is required")
	}
	if !f.hasBucket(*input.Bucket) {
		return nil, &s3types.NoSuchBucket{Message: notFoundMessage("Bucket", *input.Bucket)}
	}
	if out, ok := f.fix.VersioningConfigs[*input.Bucket]; ok && out != nil {
		return out, nil
	}
	return &s3.GetBucketVersioningOutput{
		Status:    s3types.BucketVersioningStatusEnabled,
		MFADelete: s3types.MFADeleteStatusEnabled,
	}, nil
}

// GetObjectLockConfiguration returns the bucket's object lock configuration.
// A nil entry in ObjectLockConfigs yields ObjectLockConfigurationNotFoundError
// (the canonical AWS response for a bucket without object lock); buckets with
// no entry have object lock enabled — the healthy default.
func (f *S3Fake) GetObjectLockConfiguration(_ context.Context, input *s3.GetObjectLockConfigurationInput, _ ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	if input.Bucket == nil {
		return nil, fmt.Errorf("GetObjectLockConfiguration: bucket name is required")
	}
	cfg, ok := f.fix.ObjectLockConfigs[*input.Bucket]
	if !ok {
		return &s3.GetObjectLockConfigurationOutput{
			ObjectLockConfiguration: &s3types.ObjectLockConfiguration{
				ObjectLockEnabled: s3types.ObjectLockEnabledEnabled,
			},
		}, nil
	}
	if cfg == nil {
		return nil, &smithy.GenericAPIError{
			Code:    "ObjectLockConfigurationNotFoundError",
			Message: "Object Lock configuration does not exist for this bucket",
		}
	}
	return cfg, nil
}

// hasBucket reports whether the fixtures register this bucket.
func (f *S3Fake) hasBucket(name string) bool {
	return slices.ContainsFunc(f.fix.Buckets, func(b s3types.Bucket) bool {
		return aws.ToString(b.Name) == name
	})
}
