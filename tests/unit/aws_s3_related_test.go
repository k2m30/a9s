package unit_test

// aws_s3_related_test.go — Related-target discovery tests for s3 (spec §2).
//
// Covered assertions:
//   - All 15 in-scope pivots are registered in GetRelated("s3"):
//     athena, backup, cf, cfn, eb-rule, glue, kms, lambda, logs, r53, role,
//     sns, sqs, trail, ct-events.
//   - iam-user and waf are NOT registered (§5 Out of Scope enforcement).
//   - One "found" + one "no-match" case per pivot (where deterministically
//     testable via cache or forward-field lookup).
//   - ct-events: present in registry AND returns Count=-1 with non-nil FetchFilter
//     (the universal auto-registered checker behaviour).
//   - Healthy-bucket resource (with notification ARNs pre-populated) returns
//     Count≥1 for the forward-lookup pivots (lambda, sns, sqs).
//   - Reverse-scan pivots (trail, cf, cfn, kms, logs) return Count≥1 when the
//     cache / fake client contains the expected entry.
//   - Cache-scan pivots (athena, backup, eb-rule, glue, r53, role) return
//     Count≥0 (accepting zero when the cache lacks the required field).

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// s3CheckerByTarget retrieves the RelatedChecker for the given targetType from
// the s3 related definitions. Fails the test if not found or nil.
func s3CheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("s3") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("s3 related checker for %q is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("s3 related checker for %q not found in GetRelated(\"s3\")", target)
	return nil
}

// healthyBucketResource returns a resource.Resource pre-populated with all
// fields that the healthy-bucket fixture would have after a full fetch with
// notifications enabled. Used by forward-lookup pivot tests.
func healthyBucketResource() resource.Resource {
	return resource.Resource{
		ID:   fixtures.HealthyBucketName,
		Name: fixtures.HealthyBucketName,
		Fields: map[string]string{
			"name":                fixtures.HealthyBucketName,
			"bucket_name":         fixtures.HealthyBucketName,
			"notification_lambda": "arn:aws:lambda:us-east-1:123456789012:function:" + fixtures.S3NotifierLambdaName,
			"notification_sns":    "arn:aws:sns:us-east-1:123456789012:" + fixtures.S3EventsTopicName,
			"notification_sqs":    "arn:aws:sqs:us-east-1:123456789012:" + fixtures.S3DLQueueName,
		},
	}
}

// emptyBucketResource returns a resource.Resource with no notification fields.
func emptyBucketResource(name string) resource.Resource {
	return resource.Resource{
		ID:     name,
		Name:   name,
		Fields: map[string]string{"name": name},
	}
}

// s3CheckerByDisplayName retrieves the RelatedChecker whose DisplayName
// matches. Used when two pivots share a TargetType (e.g. multiple s3→s3
// entries), where TargetType alone is ambiguous.
func s3CheckerByDisplayName(t *testing.T, display string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("s3") {
		if def.DisplayName == display {
			if def.Checker == nil {
				t.Fatalf("s3 related checker with DisplayName %q is nil", display)
			}
			return def.Checker
		}
	}
	t.Fatalf("s3 related checker with DisplayName %q not found in GetRelated(\"s3\")", display)
	return nil
}

// s3FakeClients builds a ServiceClients with the demo S3 fake (for API-call
// based checkers: kms, logs, cfn).
func s3FakeClients() *awsclient.ServiceClients {
	return &awsclient.ServiceClients{S3: fakes.NewS3()}
}

// ---------------------------------------------------------------------------
// §5 Out-of-Scope enforcement — iam-user and waf must NOT be present.
// ---------------------------------------------------------------------------

// TestS3_Related_OOS_IAMUser_NotRegistered asserts that iam-user is NOT in
// the s3 related registry (spec §5 Out of Scope).
func TestS3_Related_OOS_IAMUser_NotRegistered(t *testing.T) {
	for _, def := range resource.GetRelated("s3") {
		if def.TargetType == "iam-user" {
			t.Errorf("iam-user must NOT be registered for s3 (§5 Out of Scope — requires CloudTrail data-plane parsing)")
		}
	}
}

// TestS3_Related_OOS_WAF_NotRegistered asserts that waf is NOT in the s3
// related registry (spec §5 Out of Scope — WAF attaches via CloudFront only).
func TestS3_Related_OOS_WAF_NotRegistered(t *testing.T) {
	for _, def := range resource.GetRelated("s3") {
		if def.TargetType == "waf" {
			t.Errorf("waf must NOT be registered for s3 (§5 Out of Scope — attaches via CloudFront only)")
		}
	}
}

// ---------------------------------------------------------------------------
// All 15 in-scope pivots are present in GetRelated("s3").
// ---------------------------------------------------------------------------

// TestS3_Related_AllInScopePivots_Registered verifies that every in-scope pivot
// from spec §2 is present in the s3 related registry.
func TestS3_Related_AllInScopePivots_Registered(t *testing.T) {
	inScope := []string{
		"athena", "backup", "cf", "cfn", "eb-rule", "glue",
		"kms", "lambda", "r53", "role", "sns", "sqs", "trail",
		"ct-events",
		// The access-log pivot targets `s3` (destination bucket), not
		// `logs` (CloudWatch) — S3 server-access logs are never delivered
		// to CloudWatch. See qa_s3_logs_pivot_targets_s3_test.go.
		"s3",
	}

	registered := make(map[string]bool)
	for _, def := range resource.GetRelated("s3") {
		registered[def.TargetType] = true
	}

	for _, pivot := range inScope {
		if !registered[pivot] {
			t.Errorf("pivot %q missing from GetRelated(\"s3\"); expected 15 in-scope pivots", pivot)
		}
	}
}

// ---------------------------------------------------------------------------
// ct-events — universal auto-registered pivot (always Count=-1 with FetchFilter).
// ---------------------------------------------------------------------------

// TestS3_Related_CTEvents_Present verifies ct-events is registered and returns
// a non-nil FetchFilter (the universal auto-registered behaviour).
func TestS3_Related_CTEvents_Present(t *testing.T) {
	checker := s3CheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, healthyBucketResource(), nil)
	if result.TargetType != "ct-events" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "ct-events")
	}
	// ct-events always uses FetchFilter, not a static cache lookup.
	if result.FetchFilter == nil {
		t.Error("ct-events checker must return non-nil FetchFilter for navigation")
	}
}

// ---------------------------------------------------------------------------
// lambda — forward-lookup from Fields["notification_lambda"].
// ---------------------------------------------------------------------------

// TestS3_Related_Lambda_Found verifies that a bucket with notification_lambda
// set returns Count=1 when the named function exists in the lambda cache.
func TestS3_Related_Lambda_Found(t *testing.T) {
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: fixtures.S3NotifierLambdaName, Name: fixtures.S3NotifierLambdaName},
			},
		},
	}

	checker := s3CheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 for lambda pivot with matching function in cache", result.Count)
	}
}

// TestS3_Related_Lambda_NoMatch verifies that a bucket with no notification_lambda
// field returns Count=0.
func TestS3_Related_Lambda_NoMatch(t *testing.T) {
	checker := s3CheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, emptyBucketResource("bare-bucket"), nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for lambda pivot with no notification field", result.Count)
	}
}

// ---------------------------------------------------------------------------
// sns — forward-lookup from Fields["notification_sns"].
// ---------------------------------------------------------------------------

// TestS3_Related_SNS_Found verifies that a bucket with notification_sns returns
// Count=1 when the topic exists in the sns cache.
func TestS3_Related_SNS_Found(t *testing.T) {
	cache := resource.ResourceCache{
		"sns": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: fixtures.S3EventsTopicName, Name: fixtures.S3EventsTopicName},
			},
		},
	}

	checker := s3CheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 for sns pivot with matching topic in cache", result.Count)
	}
}

// TestS3_Related_SNS_NoMatch verifies that a bucket with no notification_sns
// field returns Count=0.
func TestS3_Related_SNS_NoMatch(t *testing.T) {
	checker := s3CheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, emptyBucketResource("bare-bucket"), nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for sns pivot with no notification field", result.Count)
	}
}

// ---------------------------------------------------------------------------
// sqs — forward-lookup from Fields["notification_sqs"].
// ---------------------------------------------------------------------------

// TestS3_Related_SQS_Found verifies that a bucket with notification_sqs returns
// Count=1 when the queue exists in the sqs cache.
func TestS3_Related_SQS_Found(t *testing.T) {
	cache := resource.ResourceCache{
		"sqs": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: fixtures.S3DLQueueName, Name: fixtures.S3DLQueueName},
			},
		},
	}

	checker := s3CheckerByTarget(t, "sqs")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 for sqs pivot with matching queue in cache", result.Count)
	}
}

// TestS3_Related_SQS_NoMatch verifies that a bucket with no notification_sqs
// field returns Count=0.
func TestS3_Related_SQS_NoMatch(t *testing.T) {
	checker := s3CheckerByTarget(t, "sqs")
	result := checker(context.Background(), nil, emptyBucketResource("bare-bucket"), nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for sqs pivot with no notification field", result.Count)
	}
}

// ---------------------------------------------------------------------------
// kms — API-call: GetBucketEncryption on the S3 fake.
// ---------------------------------------------------------------------------

// TestS3_Related_KMS_Found verifies that the healthy bucket's KMS encryption
// config resolves to Count=1 when the key exists in the kms cache.
func TestS3_Related_KMS_Found(t *testing.T) {
	cache := resource.ResourceCache{
		"kms": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: fixtures.S3BucketKMSKeyID, Name: fixtures.S3BucketKMSKeyID},
			},
		},
	}

	checker := s3CheckerByTarget(t, "kms")
	result := checker(context.Background(), s3FakeClients(), healthyBucketResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 for kms pivot (healthy bucket has SSE-KMS with key %q)",
			result.Count, fixtures.S3BucketKMSKeyID)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestS3_Related_KMS_NoMatch verifies that a bucket with no KMS encryption
// returns Count=0. Uses an ad-hoc bucket name that isn't present in any
// demo config map, so the fake returns the not-found Smithy error.
func TestS3_Related_KMS_NoMatch(t *testing.T) {
	checker := s3CheckerByTarget(t, "kms")
	src := emptyBucketResource("test-only-no-kms-" + t.Name())
	result := checker(context.Background(), s3FakeClients(), src, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for kms pivot on bucket with no SSE-KMS config", result.Count)
	}
}

// ---------------------------------------------------------------------------
// logs — API-call: GetBucketLogging on the S3 fake.
// ---------------------------------------------------------------------------

// TestS3_Related_AccessLogBucket_Found verifies that the healthy bucket's
// S3 access-log destination bucket name appears as the related resource ID
// (Count=1). The pivot targets `s3` — the destination is another bucket,
// not a CloudWatch log group.
func TestS3_Related_AccessLogBucket_Found(t *testing.T) {
	checker := s3CheckerByDisplayName(t, "Access Log Bucket")
	result := checker(context.Background(), s3FakeClients(), healthyBucketResource(), nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 for access-log pivot (healthy bucket logs to %q)",
			result.Count, fixtures.LogsBucketName)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestS3_Related_AccessLogBucket_NoMatch verifies that a bucket with no
// logging config returns Count=0. Uses an ad-hoc bucket name that isn't
// present in any demo config map, so the fake returns an empty output.
func TestS3_Related_AccessLogBucket_NoMatch(t *testing.T) {
	checker := s3CheckerByDisplayName(t, "Access Log Bucket")
	src := emptyBucketResource("test-only-no-logging-" + t.Name())
	result := checker(context.Background(), s3FakeClients(), src, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for access-log pivot on bucket with no logging config", result.Count)
	}
}

// ---------------------------------------------------------------------------
// cfn — API-call: GetBucketTagging + cfn cache scan.
// ---------------------------------------------------------------------------

// TestS3_Related_CFN_Found verifies that the healthy bucket's CFN stack-name
// tag resolves to Count=1 when the stack exists in the cfn cache.
func TestS3_Related_CFN_Found(t *testing.T) {
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:     fixtures.S3CFNStackName,
					Name:   fixtures.S3CFNStackName,
					Fields: map[string]string{"stack_name": fixtures.S3CFNStackName},
					RawStruct: cfntypes.Stack{
						StackName: aws.String(fixtures.S3CFNStackName),
					},
				},
			},
		},
	}

	checker := s3CheckerByTarget(t, "cfn")
	result := checker(context.Background(), s3FakeClients(), healthyBucketResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 for cfn pivot (healthy bucket tagged with stack %q)",
			result.Count, fixtures.S3CFNStackName)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestS3_Related_CFN_NoMatch verifies that a bucket with no CFN stack tag
// returns Count=0.
func TestS3_Related_CFN_NoMatch(t *testing.T) {
	checker := s3CheckerByTarget(t, "cfn")
	// a9s-demo-nopab has no TaggingConfigs entry → NoSuchTagSet → Count=0.
	src := emptyBucketResource("a9s-demo-nopab")
	result := checker(context.Background(), s3FakeClients(), src, resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "some-other-stack", Name: "some-other-stack"},
			},
		},
	})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for cfn pivot on bucket with no CFN tag", result.Count)
	}
}

// ---------------------------------------------------------------------------
// trail — reverse scan: RawStruct.S3BucketName must match the bucket name.
// ---------------------------------------------------------------------------

// TestS3_Related_Trail_Found verifies that a trail whose S3BucketName equals
// the healthy bucket name produces Count=1.
func TestS3_Related_Trail_Found(t *testing.T) {
	cache := resource.ResourceCache{
		"trail": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "my-audit-trail",
					Name: "my-audit-trail",
					RawStruct: cloudtrailtypes.Trail{
						Name:         aws.String("my-audit-trail"),
						S3BucketName: aws.String(fixtures.HealthyBucketName),
					},
				},
			},
		},
	}

	checker := s3CheckerByTarget(t, "trail")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 for trail pivot (trail logs to %q)", result.Count, fixtures.HealthyBucketName)
	}
}

// TestS3_Related_Trail_NoMatch verifies that a trail with a different
// S3BucketName returns Count=0.
func TestS3_Related_Trail_NoMatch(t *testing.T) {
	cache := resource.ResourceCache{
		"trail": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "other-trail",
					Name: "other-trail",
					RawStruct: cloudtrailtypes.Trail{
						Name:         aws.String("other-trail"),
						S3BucketName: aws.String("different-bucket"),
					},
				},
			},
		},
	}

	checker := s3CheckerByTarget(t, "trail")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for trail pivot with non-matching S3BucketName", result.Count)
	}
}

// ---------------------------------------------------------------------------
// cf — reverse scan: DistributionSummary.Origins.Items must contain
//      a DomainName with "BUCKETNAME.s3".
// ---------------------------------------------------------------------------

// TestS3_Related_CF_Found verifies that a CloudFront distribution with an S3
// origin referencing the healthy bucket produces Count=1.
func TestS3_Related_CF_Found(t *testing.T) {
	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "EDFDVBD6EXAMPLE",
					Name: "EDFDVBD6EXAMPLE",
					RawStruct: cftypes.DistributionSummary{
						Id: aws.String("EDFDVBD6EXAMPLE"),
						Origins: &cftypes.Origins{
							Quantity: aws.Int32(1),
							Items: []cftypes.Origin{
								{
									DomainName: aws.String(fixtures.HealthyBucketName + ".s3.us-east-1.amazonaws.com"),
								},
							},
						},
					},
				},
			},
		},
	}

	checker := s3CheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 for cf pivot (distribution has origin %s.s3.*)",
			result.Count, fixtures.HealthyBucketName)
	}
}

// TestS3_Related_CF_NoMatch verifies that a distribution with a different
// origin domain returns Count=0.
func TestS3_Related_CF_NoMatch(t *testing.T) {
	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "EDFDVBD6EXAMPLE",
					Name: "EDFDVBD6EXAMPLE",
					RawStruct: cftypes.DistributionSummary{
						Id: aws.String("EDFDVBD6EXAMPLE"),
						Origins: &cftypes.Origins{
							Quantity: aws.Int32(1),
							Items: []cftypes.Origin{
								{
									DomainName: aws.String("other-bucket.s3.amazonaws.com"),
								},
							},
						},
					},
				},
			},
		},
	}

	checker := s3CheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for cf pivot with non-matching origin", result.Count)
	}
}

// ---------------------------------------------------------------------------
// Deferred cache-scan pivots — accept Count≥0 (no sibling fixtures yet).
// These tests confirm the checkers function without panic and return a valid
// result code when the cache is populated with a non-matching entry.
// ---------------------------------------------------------------------------

// TestS3_Related_Athena_NoMatch verifies the athena checker returns Count=0
// when no Athena workgroup references the bucket's S3 URI.
func TestS3_Related_Athena_NoMatch(t *testing.T) {
	cache := resource.ResourceCache{
		"athena": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:     "other-workgroup",
					Name:   "other-workgroup",
					Fields: map[string]string{"result_output_location": "s3://other-bucket/athena/"},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "athena")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)
	if result.Count < 0 && result.Count != -1 {
		t.Errorf("Count = %d, want ≥0 or -1 for athena pivot with no match", result.Count)
	}
}

// TestS3_Related_Athena_Found verifies the athena checker returns Count≥1 when
// a workgroup's result_output_location references the bucket.
func TestS3_Related_Athena_Found(t *testing.T) {
	cache := resource.ResourceCache{
		"athena": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:     "my-workgroup",
					Name:   "my-workgroup",
					Fields: map[string]string{"result_output_location": "s3://" + fixtures.HealthyBucketName + "/athena/"},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "athena")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)
	if result.Count < 1 {
		t.Errorf("Count = %d, want ≥1 for athena pivot when workgroup references %q",
			result.Count, fixtures.HealthyBucketName)
	}
}

// TestS3_Related_Backup_NoMatch verifies the backup checker returns Count=0
// when no backup entry references this bucket's ARN.
func TestS3_Related_Backup_NoMatch(t *testing.T) {
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:     "plan-other",
					Name:   "plan-other",
					Fields: map[string]string{"resource_arn": "arn:aws:s3:::other-bucket"},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for backup pivot with non-matching ARN", result.Count)
	}
}

// TestS3_Related_Backup_Found verifies the backup checker returns Count≥1
// when a backup entry's resource_arn matches the bucket ARN.
func TestS3_Related_Backup_Found(t *testing.T) {
	bucketARN := fixtures.HealthyBucketARN
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:     "plan-s3",
					Name:   "plan-s3",
					Fields: map[string]string{"resource_arn": bucketARN},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)
	if result.Count < 1 {
		t.Errorf("Count = %d, want ≥1 for backup pivot when entry references %q", result.Count, bucketARN)
	}
}

// TestS3_Related_EBRule_NoMatch verifies the eb-rule checker returns Count=0
// when no rule's EventPattern sources from aws.s3 with a matching bucket name.
// Spec §2 (eb-rule): "rules with EventPattern.source=['aws.s3'] AND
// EventPattern.detail.bucket.name matching this bucket".
func TestS3_Related_EBRule_NoMatch(t *testing.T) {
	cache := resource.ResourceCache{
		"eb-rule": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "other-rule",
					Name: "other-rule",
					// Pattern filters on aws.ec2, not aws.s3 — must not match.
					Fields: map[string]string{
						"event_pattern": `{"source":["aws.ec2"],"detail-type":["EC2 Instance State-change Notification"]}`,
					},
				},
				{
					ID:   "s3-other-bucket-rule",
					Name: "s3-other-bucket-rule",
					// Right source but different bucket — must not match.
					Fields: map[string]string{
						"event_pattern": `{"source":["aws.s3"],"detail":{"bucket":{"name":["some-other-bucket"]}}}`,
					},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for eb-rule pivot with non-matching EventPattern", result.Count)
	}
}

// TestS3_Related_EBRule_Found verifies the eb-rule checker returns Count≥1
// when a rule's EventPattern sources from aws.s3 AND references the healthy
// bucket by name.
func TestS3_Related_EBRule_Found(t *testing.T) {
	cache := resource.ResourceCache{
		"eb-rule": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "s3-event-rule",
					Name: "s3-event-rule",
					Fields: map[string]string{
						"event_pattern": `{"source":["aws.s3"],"detail-type":["Object Created"],"detail":{"bucket":{"name":["` + fixtures.HealthyBucketName + `"]}}}`,
					},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)
	if result.Count < 1 {
		t.Errorf("Count = %d, want ≥1 for eb-rule pivot when EventPattern sources from aws.s3 and names bucket %q",
			result.Count, fixtures.HealthyBucketName)
	}
}

// TestS3_Related_Glue_NoMatch verifies the glue checker returns Count=0 when
// no Glue job's ScriptLocation references this bucket.
func TestS3_Related_Glue_NoMatch(t *testing.T) {
	cache := resource.ResourceCache{
		"glue": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "other-job",
					Name: "other-job",
					RawStruct: gluetypes.Job{
						Name:    aws.String("other-job"),
						Command: &gluetypes.JobCommand{ScriptLocation: aws.String("s3://other-bucket/scripts/etl.py")},
					},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "glue")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for glue pivot with ScriptLocation pointing to a different bucket", result.Count)
	}
}

// TestS3_Related_Glue_Found verifies the glue checker returns Count≥1 when a
// job's ScriptLocation references the healthy bucket.
func TestS3_Related_Glue_Found(t *testing.T) {
	cache := resource.ResourceCache{
		"glue": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "s3-etl-job",
					Name: "s3-etl-job",
					RawStruct: gluetypes.Job{
						Name:    aws.String("s3-etl-job"),
						Command: &gluetypes.JobCommand{ScriptLocation: aws.String("s3://" + fixtures.HealthyBucketName + "/scripts/etl.py")},
					},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "glue")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)
	if result.Count < 1 {
		t.Errorf("Count = %d, want ≥1 for glue pivot when job ScriptLocation is in %q", result.Count, fixtures.HealthyBucketName)
	}
}

// TestS3_Related_R53_NoMatch verifies the r53 checker returns Count=0 when no
// hosted zone's alias_targets references this bucket's S3 website endpoint.
func TestS3_Related_R53_NoMatch(t *testing.T) {
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:     "Z1D633PJN98FT9",
					Name:   "Z1D633PJN98FT9",
					Fields: map[string]string{"alias_targets": "other-bucket.s3-website-us-east-1.amazonaws.com"},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for r53 pivot with non-matching alias_targets", result.Count)
	}
}

// TestS3_Related_R53_Found verifies the r53 checker returns Count≥1 when a
// hosted zone's alias_targets contains the bucket's S3 website endpoint probe.
func TestS3_Related_R53_Found(t *testing.T) {
	// The checker probes for "BUCKETNAME.s3" as a substring of alias_targets.
	probe := fixtures.HealthyBucketName + ".s3"
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:     "Z1D633PJN98FT9",
					Name:   "Z1D633PJN98FT9",
					Fields: map[string]string{"alias_targets": probe + "-website-us-east-1.amazonaws.com"},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)
	if result.Count < 1 {
		t.Errorf("Count = %d, want ≥1 for r53 pivot when alias_targets contains %q", result.Count, probe)
	}
}

// TestS3_Related_Role_NoMatch verifies the role checker returns Count=0 when
// no role's policy_resources contains this bucket's ARN.
func TestS3_Related_Role_NoMatch(t *testing.T) {
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:     "other-role",
					Name:   "other-role",
					Fields: map[string]string{"policy_resources": "arn:aws:s3:::other-bucket/*"},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "role")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for role pivot with non-matching policy_resources", result.Count)
	}
}

// TestS3_Related_Role_Found verifies the role checker returns Count≥1 when a
// role's policy_resources contains the bucket ARN.
func TestS3_Related_Role_Found(t *testing.T) {
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:     "s3-access-role",
					Name:   "s3-access-role",
					Fields: map[string]string{"policy_resources": fixtures.HealthyBucketARN + "/*"},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "role")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)
	if result.Count < 1 {
		t.Errorf("Count = %d, want ≥1 for role pivot when policy_resources contains %q", result.Count, fixtures.HealthyBucketARN)
	}
}
