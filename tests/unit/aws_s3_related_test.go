package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

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

// The backup pivot builds a bucket ARN whose partition comes from the
// session's region; a session that recorded none declines rather than
// guessing commercial.
func s3BackupSession() *awsclient.ServiceClients {
	return &awsclient.ServiceClients{Region: "us-east-1"}
}

func healthyBucketResource() resource.Resource {
	return resource.Resource{
		ID:   fixtures.HealthyBucketName,
		Name: fixtures.HealthyBucketName,
		Fields: map[string]string{
			"name":                fixtures.HealthyBucketName,
			"notification_lambda": "arn:aws:lambda:us-east-1:123456789012:function:" + fixtures.S3NotifierLambdaName,
			"notification_sns":    "arn:aws:sns:us-east-1:123456789012:" + fixtures.S3EventsTopicName,
			"notification_sqs":    "arn:aws:sqs:us-east-1:123456789012:" + fixtures.S3DLQueueName,
		},
	}
}

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

func s3FakeClients() *awsclient.ServiceClients {
	return &awsclient.ServiceClients{S3: fakes.NewS3()}
}

func TestS3_Related_OOS_IAMUser_NotRegistered(t *testing.T) {
	for _, def := range resource.GetRelated("s3") {
		if def.TargetType == "iam-user" {
			t.Errorf("iam-user must NOT be registered for s3 (§5 Out of Scope — requires CloudTrail data-plane parsing)")
		}
	}
}

// WAF attaches to a bucket only through CloudFront.
func TestS3_Related_OOS_WAF_NotRegistered(t *testing.T) {
	for _, def := range resource.GetRelated("s3") {
		if def.TargetType == "waf" {
			t.Errorf("waf must NOT be registered for s3 (§5 Out of Scope — attaches via CloudFront only)")
		}
	}
}

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

func TestS3_Related_CTEvents_Present(t *testing.T) {
	checker := s3CheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, healthyBucketResource(), nil)
	if result.TargetType() != "ct-events" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "ct-events")
	}
	if result.FetchFilter() == nil {
		t.Error("ct-events checker must return non-nil FetchFilter for navigation")
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 for lambda pivot with matching function in cache", result.Count())
	}
}

func TestS3_Related_Lambda_NoMatch(t *testing.T) {
	checker := s3CheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, emptyBucketResource("bare-bucket"), nil)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for lambda pivot with no notification field", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 for sns pivot with matching topic in cache", result.Count())
	}
}

func TestS3_Related_SNS_NoMatch(t *testing.T) {
	checker := s3CheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, emptyBucketResource("bare-bucket"), nil)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for sns pivot with no notification field", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 for sqs pivot with matching queue in cache", result.Count())
	}
}

func TestS3_Related_SQS_NoMatch(t *testing.T) {
	checker := s3CheckerByTarget(t, "sqs")
	result := checker(context.Background(), nil, emptyBucketResource("bare-bucket"), nil)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for sqs pivot with no notification field", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 for kms pivot (healthy bucket has SSE-KMS with key %q)",
			result.Count(), fixtures.S3BucketKMSKeyID)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestS3_Related_KMS_NoMatch(t *testing.T) {
	checker := s3CheckerByTarget(t, "kms")
	src := emptyBucketResource("test-only-no-kms-" + t.Name())
	result := checker(context.Background(), s3FakeClients(), src, nil)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for kms pivot on bucket with no SSE-KMS config", result.Count())
	}
}

func TestS3_Related_AccessLogBucket_Found(t *testing.T) {
	checker := s3CheckerByDisplayName(t, "Access Log Bucket")
	result := checker(context.Background(), s3FakeClients(), healthyBucketResource(), nil)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 for access-log pivot (healthy bucket logs to %q)",
			result.Count(), fixtures.LogsBucketName)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestS3_Related_AccessLogBucket_NoMatch(t *testing.T) {
	checker := s3CheckerByDisplayName(t, "Access Log Bucket")
	src := emptyBucketResource("test-only-no-logging-" + t.Name())
	result := checker(context.Background(), s3FakeClients(), src, nil)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for access-log pivot on bucket with no logging config", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 for cfn pivot (healthy bucket tagged with stack %q)",
			result.Count(), fixtures.S3CFNStackName)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestS3_Related_CFN_NoMatch(t *testing.T) {
	checker := s3CheckerByTarget(t, "cfn")
	src := emptyBucketResource("a9s-demo-nopab")
	result := checker(context.Background(), s3FakeClients(), src, resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "some-other-stack", Name: "some-other-stack"},
			},
		},
	})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for cfn pivot on bucket with no CFN tag", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 for trail pivot (trail logs to %q)", result.Count(), fixtures.HealthyBucketName)
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for trail pivot with non-matching S3BucketName", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 for cf pivot (distribution has origin %s.s3.*)",
			result.Count(), fixtures.HealthyBucketName)
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for cf pivot with non-matching origin", result.Count())
	}
}

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
	if result.Count() < 0 {
		t.Errorf("Count = %d, want ≥0 for athena pivot with no match", result.Count())
	}
}

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
	if result.Count() < 1 {
		t.Errorf("Count = %d, want ≥1 for athena pivot when workgroup references %q",
			result.Count(), fixtures.HealthyBucketName)
	}
}

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
	result := checker(context.Background(), s3BackupSession(), healthyBucketResource(), cache)
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for backup pivot with non-matching ARN", result.Count())
	}
}

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
	result := checker(context.Background(), s3BackupSession(), healthyBucketResource(), cache)
	if result.Count() < 1 {
		t.Errorf("Count = %d, want ≥1 for backup pivot when entry references %q", result.Count(), bucketARN)
	}
}

func TestS3_Related_EBRule_NoMatch(t *testing.T) {
	cache := resource.ResourceCache{
		"eb-rule": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "other-rule",
					Name: "other-rule",
					Fields: map[string]string{
						"event_pattern": `{"source":["aws.ec2"],"detail-type":["EC2 Instance State-change Notification"]}`,
					},
				},
				{
					ID:   "s3-other-bucket-rule",
					Name: "s3-other-bucket-rule",
					Fields: map[string]string{
						"event_pattern": `{"source":["aws.s3"],"detail":{"bucket":{"name":["some-other-bucket"]}}}`,
					},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), s3BackupSession(), healthyBucketResource(), cache)
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for eb-rule pivot with non-matching EventPattern", result.Count())
	}
}

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
	result := checker(context.Background(), s3BackupSession(), healthyBucketResource(), cache)
	if result.Count() < 1 {
		t.Errorf("Count = %d, want ≥1 for eb-rule pivot when EventPattern sources from aws.s3 and names bucket %q",
			result.Count(), fixtures.HealthyBucketName)
	}
}

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
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for glue pivot with ScriptLocation pointing to a different bucket", result.Count())
	}
}

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
	if result.Count() < 1 {
		t.Errorf("Count = %d, want ≥1 for glue pivot when job ScriptLocation is in %q", result.Count(), fixtures.HealthyBucketName)
	}
}

func TestS3_Related_R53_NoMatch(t *testing.T) {
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "Z1D633PJN98FT9",
					Name: "Z1D633PJN98FT9",
					Fields: map[string]string{
						"s3website_alias_names": "other-bucket",
						"alias_targets":         "s3-website-us-east-1.amazonaws.com.",
					},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for r53 pivot when no alias record name equals this bucket", result.Count())
	}
}

// The bucket name equals the record FQDN, the only join key:
// AliasTarget.DNSName is the regional endpoint and never carries the bucket
// name. The r53 fetcher emits S3-website alias FQDNs into
// s3website_alias_names.
func TestS3_Related_R53_Found(t *testing.T) {
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "Z1D633PJN98FT9",
					Name: "Z1D633PJN98FT9",
					Fields: map[string]string{
						"s3website_alias_names": fixtures.HealthyBucketName,
						"alias_targets":         "s3-website-us-east-1.amazonaws.com.",
					},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, healthyBucketResource(), cache)
	if result.Count() < 1 {
		t.Errorf("Count = %d, want ≥1 for r53 pivot when an alias record's NAME equals the bucket name %q",
			result.Count(), fixtures.HealthyBucketName)
	}
}

// The role pivot reads the bucket policy: without one it is 0 whatever the
// role's own policies say.
func TestS3_Related_Role_NoBucketPolicy_Count0(t *testing.T) {
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "any-role", Name: "any-role"},
			},
		},
	}
	checker := s3CheckerByTarget(t, "role")
	src := emptyBucketResource("test-only-no-policy-" + t.Name())
	result := checker(context.Background(), s3FakeClients(), src, cache)
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for role pivot when bucket has no policy", result.Count())
	}
}

func TestS3_Related_Role_BucketPolicyPrincipalResolves(t *testing.T) {
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "a9s-demo-s3-access-role", Name: "a9s-demo-s3-access-role"},
			},
		},
	}
	checker := s3CheckerByTarget(t, "role")
	result := checker(context.Background(), s3FakeClients(), healthyBucketResource(), cache)
	if result.Count() < 1 {
		t.Errorf("Count = %d, want ≥1 for role pivot when bucket policy names the role as a principal", result.Count())
	}
}

func s3ContainsID(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func TestCheckS3Backup_WildcardMatchingAndExclusion(t *testing.T) {
	plans := []resource.Resource{
		{ID: "plan-prefix", Fields: map[string]string{
			"resources":     "arn:aws:s3:::prod-*",
			"not_resources": "",
		}},
		{ID: "plan-catchall-except-quarantine", Fields: map[string]string{
			"resources":     "arn:aws:s3:::*",
			"not_resources": "arn:aws:s3:::quarantine-*",
		}},
		{ID: "plan-specific", Fields: map[string]string{
			"resources":     "arn:aws:s3:::specific-bucket",
			"not_resources": "",
		}},
	}
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{
			Resources:   plans,
			IsTruncated: false,
		},
	}
	checker := s3CheckerByTarget(t, "backup")

	t.Run("prod-logs covered by plan-prefix and plan-catchall-except-quarantine", func(t *testing.T) {
		// S3 checker derives bucket ARN as "arn:aws:s3:::"+bucket.ID
		res := resource.Resource{
			ID:     "prod-logs",
			Name:   "prod-logs",
			Fields: map[string]string{"name": "prod-logs"},
		}
		result := checker(context.Background(), s3BackupSession(), res, cache)
		if result.Count() != 2 {
			t.Errorf("Count = %d, want 2 (plan-prefix + plan-catchall-except-quarantine)", result.Count())
		}
		if !s3ContainsID(result.ResourceIDs(), "plan-prefix") {
			t.Errorf("ResourceIDs %v missing plan-prefix", result.ResourceIDs())
		}
		if !s3ContainsID(result.ResourceIDs(), "plan-catchall-except-quarantine") {
			t.Errorf("ResourceIDs %v missing plan-catchall-except-quarantine", result.ResourceIDs())
		}
	})

	t.Run("staging-data covered only by plan-catchall-except-quarantine", func(t *testing.T) {
		res := resource.Resource{
			ID:     "staging-data",
			Name:   "staging-data",
			Fields: map[string]string{"name": "staging-data"},
		}
		result := checker(context.Background(), s3BackupSession(), res, cache)
		if result.Count() != 1 {
			t.Errorf("Count = %d, want 1 (only plan-catchall-except-quarantine)", result.Count())
		}
		if !s3ContainsID(result.ResourceIDs(), "plan-catchall-except-quarantine") {
			t.Errorf("ResourceIDs %v missing plan-catchall-except-quarantine", result.ResourceIDs())
		}
	})

	t.Run("quarantine-pii excluded from all plans", func(t *testing.T) {
		res := resource.Resource{
			ID:     "quarantine-pii",
			Name:   "quarantine-pii",
			Fields: map[string]string{"name": "quarantine-pii"},
		}
		result := checker(context.Background(), s3BackupSession(), res, cache)
		if result.Count() != 0 {
			t.Errorf("Count = %d, want 0 (plan-catchall excludes quarantine-*, others miss the prefix)", result.Count())
		}
	})

	t.Run("specific-bucket covered by plan-catchall-except-quarantine and plan-specific", func(t *testing.T) {
		res := resource.Resource{
			ID:     "specific-bucket",
			Name:   "specific-bucket",
			Fields: map[string]string{"name": "specific-bucket"},
		}
		result := checker(context.Background(), s3BackupSession(), res, cache)
		if result.Count() != 2 {
			t.Errorf("Count = %d, want 2 (plan-catchall-except-quarantine + plan-specific)", result.Count())
		}
		if !s3ContainsID(result.ResourceIDs(), "plan-catchall-except-quarantine") {
			t.Errorf("ResourceIDs %v missing plan-catchall-except-quarantine", result.ResourceIDs())
		}
		if !s3ContainsID(result.ResourceIDs(), "plan-specific") {
			t.Errorf("ResourceIDs %v missing plan-specific", result.ResourceIDs())
		}
	})
}
