// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// BucketEnriched wraps s3types.Bucket with the per-bucket configuration the
// list call (ListBuckets) never carries: resource policy, CORS rules, and
// lifecycle rules.
type BucketEnriched struct {
	s3types.Bucket
	Policy         any                     `json:"Policy,omitempty" yaml:"Policy,omitempty"`
	CORSRules      []s3types.CORSRule      `json:"CORSRules,omitempty" yaml:"CORSRules,omitempty"`
	LifecycleRules []s3types.LifecycleRule `json:"LifecycleRules,omitempty" yaml:"LifecycleRules,omitempty"`
}

// s3DetailPayload is enrichS3's fetch result: the bucket policy, CORS
// rules, and lifecycle rules from three independent per-bucket calls.
type s3DetailPayload struct {
	Policy         any
	CORSRules      []s3types.CORSRule
	LifecycleRules []s3types.LifecycleRule
}

// enrichS3 fetches the bucket's resource policy, CORS configuration, and
// lifecycle configuration. Uncached: three cheap per-bucket calls, and any
// of the three can change mid-session as an operator (or another engineer)
// edits bucket configuration — the case this detail view exists to surface.
// A missing configuration (NoSuchBucketPolicy / NoSuchCORSConfiguration /
// NoSuchLifecycleConfiguration) is a legitimate "not configured" 0, not an
// enrichment failure — the corresponding field is left nil and enrichment
// continues. A cross-region rejection (isS3CrossRegionErr: PermanentRedirect
// / IllegalLocationConstraintException) is the same kind of benign-incomplete
// result every other S3 path treats it as — the field is left nil, not an
// error.
func enrichS3(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
	return enrichDetail(ctx, clients, res, detailEnrichSpec[s3types.Bucket, s3DetailPayload]{
		unwrap: unwrapEnriched(func(w BucketEnriched) s3types.Bucket { return w.Bucket }),
		id: func(bucket s3types.Bucket, _ resource.Resource) (string, error) {
			if bucket.Name == nil || *bucket.Name == "" {
				return "", fmt.Errorf("bucket has no name")
			}
			return *bucket.Name, nil
		},
		fetch: func(ctx context.Context, c *ServiceClients, id string, _ s3types.Bucket, _ resource.Resource) (s3DetailPayload, error) {
			var payload s3DetailPayload

			policyAPI, ok := c.S3.(S3GetBucketPolicyAPI)
			if !ok {
				return s3DetailPayload{}, fmt.Errorf("S3 client does not support GetBucketPolicy")
			}
			policyOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketPolicyOutput, error) {
				return policyAPI.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(id)})
			})
			switch {
			case err != nil && !s3BenignAbsenceErr(err, "NoSuchBucketPolicy") && !isS3CrossRegionErr(err):
				return s3DetailPayload{}, err
			case err == nil && policyOut.Policy != nil:
				payload.Policy = parseJSONOrRaw(*policyOut.Policy)
			}

			corsAPI, ok := c.S3.(S3GetBucketCorsAPI)
			if !ok {
				return s3DetailPayload{}, fmt.Errorf("S3 client does not support GetBucketCors")
			}
			corsOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketCorsOutput, error) {
				return corsAPI.GetBucketCors(ctx, &s3.GetBucketCorsInput{Bucket: aws.String(id)})
			})
			switch {
			case err != nil && !s3BenignAbsenceErr(err, "NoSuchCORSConfiguration") && !isS3CrossRegionErr(err):
				return s3DetailPayload{}, err
			case err == nil:
				payload.CORSRules = corsOut.CORSRules
			}

			lifecycleAPI, ok := c.S3.(S3GetBucketLifecycleAPI)
			if !ok {
				return s3DetailPayload{}, fmt.Errorf("S3 client does not support GetBucketLifecycleConfiguration")
			}
			lifecycleOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetBucketLifecycleConfigurationOutput, error) {
				return lifecycleAPI.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: aws.String(id)})
			})
			switch {
			case err != nil && !s3BenignAbsenceErr(err, "NoSuchLifecycleConfiguration") && !isS3CrossRegionErr(err):
				return s3DetailPayload{}, err
			case err == nil:
				payload.LifecycleRules = lifecycleOut.Rules
			}

			return payload, nil
		},
		wrap: func(bucket s3types.Bucket, payload s3DetailPayload) any {
			return BucketEnriched{Bucket: bucket, Policy: payload.Policy, CORSRules: payload.CORSRules, LifecycleRules: payload.LifecycleRules}
		},
	})
}
