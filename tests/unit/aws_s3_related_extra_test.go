// aws_s3_related_extra_test.go covers the S3 related checkers that were
// skipped in the prior wave: checkS3SNS, checkS3SQS, checkS3KMS,
// checkS3Logs, checkS3Athena, checkS3Glue, checkS3Backup, checkS3EBRule,
// checkS3IAMUser, checkS3R53, checkS3Role, checkS3WAF.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// checkS3SNS — ARN parse: arn:aws:sns:region:account:TopicName → TopicName
// ---------------------------------------------------------------------------

func TestRelated_S3_SNS_Found(t *testing.T) {
	source := resource.Resource{
		ID:   "events-bucket",
		Name: "events-bucket",
		Fields: map[string]string{
			"notification_sns": "arn:aws:sns:us-east-1:123456789012:order-events",
		},
	}
	checker := s3CheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "order-events" {
		t.Errorf("ResourceIDs = %v, want [order-events]", result.ResourceIDs)
	}
	if result.TargetType != "sns" {
		t.Errorf("TargetType = %q, want \"sns\"", result.TargetType)
	}
}

func TestRelated_S3_SNS_Empty(t *testing.T) {
	source := resource.Resource{
		ID:     "events-bucket",
		Fields: map[string]string{"notification_sns": ""},
	}
	checker := s3CheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_S3_SNS_ShortARN(t *testing.T) {
	// ARN with fewer than 6 segments → Count: 0 (too short to extract name).
	source := resource.Resource{
		ID:     "events-bucket",
		Fields: map[string]string{"notification_sns": "arn:aws:sns:us-east-1"},
	}
	checker := s3CheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (ARN too short)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkS3SQS — ARN parse: arn:aws:sqs:region:account:QueueName → QueueName
// ---------------------------------------------------------------------------

func TestRelated_S3_SQS_Found(t *testing.T) {
	source := resource.Resource{
		ID:   "ingest-bucket",
		Name: "ingest-bucket",
		Fields: map[string]string{
			"notification_sqs": "arn:aws:sqs:us-east-1:123456789012:ingest-queue",
		},
	}
	checker := s3CheckerByTarget(t, "sqs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "ingest-queue" {
		t.Errorf("ResourceIDs = %v, want [ingest-queue]", result.ResourceIDs)
	}
}

func TestRelated_S3_SQS_Empty(t *testing.T) {
	source := resource.Resource{
		ID:     "ingest-bucket",
		Fields: map[string]string{"notification_sqs": ""},
	}
	checker := s3CheckerByTarget(t, "sqs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkS3KMS — GetBucketEncryption, extracts KMSMasterKeyID last-segment
// ---------------------------------------------------------------------------

func TestRelated_S3_KMS_FoundFullARN(t *testing.T) {
	const keyID = "arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-1234-5678-abcd-111111111111"
	source := resource.Resource{
		ID:   "encrypted-bucket",
		Name: "encrypted-bucket",
	}
	clients := &awsclient.ServiceClients{
		S3: newFakeS3CRWithKMS(keyID),
	}
	checker := s3CheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "a1b2c3d4-1234-5678-abcd-111111111111" {
		t.Errorf("ResourceIDs = %v, want [a1b2c3d4-1234-5678-abcd-111111111111]", result.ResourceIDs)
	}
}

func TestRelated_S3_KMS_FoundBareID(t *testing.T) {
	// When KMSMasterKeyID is a bare UUID (not an ARN), it is returned as-is.
	const keyID = "a1b2c3d4-bare-id"
	source := resource.Resource{
		ID:   "encrypted-bucket",
		Name: "encrypted-bucket",
	}
	clients := &awsclient.ServiceClients{
		S3: newFakeS3CRWithKMS(keyID),
	}
	checker := s3CheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != keyID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, keyID)
	}
}

func TestRelated_S3_KMS_NilClients(t *testing.T) {
	source := resource.Resource{ID: "encrypted-bucket", Fields: map[string]string{}}
	checker := s3CheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkS3Logs — GetBucketLogging, returns target bucket name
// ---------------------------------------------------------------------------

func TestRelated_S3_Logs_Found(t *testing.T) {
	const targetBucket = "access-log-archive"
	source := resource.Resource{
		ID:   "webapp-bucket",
		Name: "webapp-bucket",
	}
	clients := &awsclient.ServiceClients{
		S3: newFakeS3CRWithLogging(targetBucket),
	}
	checker := s3CheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != targetBucket {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, targetBucket)
	}
}

func TestRelated_S3_Logs_NotEnabled(t *testing.T) {
	// Empty GetBucketLoggingOutput (no LoggingEnabled) → Count: 0.
	source := resource.Resource{
		ID:   "webapp-bucket",
		Name: "webapp-bucket",
	}
	// newFakeS3CR with nil getBucketLoggingOutput returns an empty output.
	clients := &awsclient.ServiceClients{
		S3: &fakeS3CR{},
	}
	checker := s3CheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (logging not enabled)", result.Count)
	}
}

func TestRelated_S3_Logs_NilClients(t *testing.T) {
	source := resource.Resource{ID: "webapp-bucket"}
	checker := s3CheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkS3Athena — cache scan: bucketFromS3URI(Fields["result_output_location"])
// ---------------------------------------------------------------------------

func TestRelated_S3_Athena_Found(t *testing.T) {
	const bucketName = "athena-query-results"
	wgRes := resource.Resource{
		ID:   "primary",
		Name: "primary",
		Fields: map[string]string{
			"result_output_location": "s3://athena-query-results/prefix/",
		},
	}
	cache := resource.ResourceCache{
		"athena": resource.ResourceCacheEntry{Resources: []resource.Resource{wgRes}},
	}
	source := resource.Resource{ID: bucketName, Name: bucketName}

	checker := s3CheckerByTarget(t, "athena")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "primary" {
		t.Errorf("ResourceIDs = %v, want [primary]", result.ResourceIDs)
	}
}

func TestRelated_S3_Athena_NoMatch(t *testing.T) {
	wgRes := resource.Resource{
		ID:   "secondary",
		Name: "secondary",
		Fields: map[string]string{
			"result_output_location": "s3://other-bucket/prefix/",
		},
	}
	cache := resource.ResourceCache{
		"athena": resource.ResourceCacheEntry{Resources: []resource.Resource{wgRes}},
	}
	source := resource.Resource{ID: "athena-query-results", Name: "athena-query-results"}

	checker := s3CheckerByTarget(t, "athena")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkS3Glue — RawStruct scan: gluetypes.Job.Command.ScriptLocation bucket
// ---------------------------------------------------------------------------

func TestRelated_S3_Glue_Found(t *testing.T) {
	jobRes := resource.Resource{
		ID:   "my-etl-job",
		Name: "my-etl-job",
		RawStruct: gluetypes.Job{
			Name: aws.String("my-etl-job"),
			Command: &gluetypes.JobCommand{
				ScriptLocation: aws.String("s3://glue-scripts-bucket/jobs/etl.py"),
			},
		},
	}
	cache := resource.ResourceCache{
		"glue": resource.ResourceCacheEntry{Resources: []resource.Resource{jobRes}},
	}
	source := resource.Resource{ID: "glue-scripts-bucket", Name: "glue-scripts-bucket"}

	checker := s3CheckerByTarget(t, "glue")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-etl-job" {
		t.Errorf("ResourceIDs = %v, want [my-etl-job]", result.ResourceIDs)
	}
}

func TestRelated_S3_Glue_NoMatch(t *testing.T) {
	jobRes := resource.Resource{
		ID:   "other-job",
		Name: "other-job",
		RawStruct: gluetypes.Job{
			Command: &gluetypes.JobCommand{
				ScriptLocation: aws.String("s3://different-bucket/jobs/other.py"),
			},
		},
	}
	cache := resource.ResourceCache{
		"glue": resource.ResourceCacheEntry{Resources: []resource.Resource{jobRes}},
	}
	source := resource.Resource{ID: "glue-scripts-bucket"}

	checker := s3CheckerByTarget(t, "glue")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_S3_Glue_WrongRawStruct(t *testing.T) {
	// Non-gluetypes.Job RawStruct is skipped.
	jobRes := resource.Resource{
		ID:        "my-etl-job",
		RawStruct: "not-a-glue-job",
	}
	cache := resource.ResourceCache{
		"glue": resource.ResourceCacheEntry{Resources: []resource.Resource{jobRes}},
	}
	source := resource.Resource{ID: "glue-scripts-bucket"}

	checker := s3CheckerByTarget(t, "glue")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct type skipped)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkS3Backup — Fields["resource_arn"] or Fields["resources"] contains ARN
// ---------------------------------------------------------------------------

func TestRelated_S3_Backup_FoundByResourceARN(t *testing.T) {
	const bucket = "prod-data-bucket"
	const bucketARN = "arn:aws:s3:::prod-data-bucket"
	bkRes := resource.Resource{
		ID: "backup-plan-1",
		Fields: map[string]string{
			"resource_arn": bucketARN,
		},
	}
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{Resources: []resource.Resource{bkRes}},
	}
	source := resource.Resource{ID: bucket}

	checker := s3CheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "backup-plan-1" {
		t.Errorf("ResourceIDs = %v, want [backup-plan-1]", result.ResourceIDs)
	}
}

func TestRelated_S3_Backup_NoMatch(t *testing.T) {
	bkRes := resource.Resource{
		ID: "backup-plan-2",
		Fields: map[string]string{
			"resource_arn": "arn:aws:s3:::different-bucket",
		},
	}
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{Resources: []resource.Resource{bkRes}},
	}
	source := resource.Resource{ID: "prod-data-bucket"}

	checker := s3CheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkS3EBRule — Fields["target_arns"] contains bucket ARN
// ---------------------------------------------------------------------------

func TestRelated_S3_EBRule_Found(t *testing.T) {
	const bucket = "audit-bucket"
	const bucketARN = "arn:aws:s3:::audit-bucket"
	ruleRes := resource.Resource{
		ID: "daily-archive-rule",
		Fields: map[string]string{
			"target_arns": "arn:aws:logs:::log-group " + bucketARN + " arn:aws:sqs:::queue",
		},
	}
	cache := resource.ResourceCache{
		"eb-rule": resource.ResourceCacheEntry{Resources: []resource.Resource{ruleRes}},
	}
	source := resource.Resource{ID: bucket}

	checker := s3CheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "daily-archive-rule" {
		t.Errorf("ResourceIDs = %v, want [daily-archive-rule]", result.ResourceIDs)
	}
}

func TestRelated_S3_EBRule_NoMatch(t *testing.T) {
	ruleRes := resource.Resource{
		ID: "other-rule",
		Fields: map[string]string{
			"target_arns": "arn:aws:s3:::other-bucket",
		},
	}
	cache := resource.ResourceCache{
		"eb-rule": resource.ResourceCacheEntry{Resources: []resource.Resource{ruleRes}},
	}
	source := resource.Resource{ID: "audit-bucket"}

	checker := s3CheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkS3IAMUser — Fields["policy_resources"] contains bucket ARN
// ---------------------------------------------------------------------------

func TestRelated_S3_IAMUser_Found(t *testing.T) {
	const bucket = "shared-reports"
	const bucketARN = "arn:aws:s3:::shared-reports"
	userRes := resource.Resource{
		ID: "analyst-user",
		Fields: map[string]string{
			"policy_resources": "arn:aws:s3:::* " + bucketARN + "/*",
		},
	}
	cache := resource.ResourceCache{
		"iam-user": resource.ResourceCacheEntry{Resources: []resource.Resource{userRes}},
	}
	source := resource.Resource{ID: bucket}

	checker := s3CheckerByTarget(t, "iam-user")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_S3_IAMUser_NoMatch(t *testing.T) {
	userRes := resource.Resource{
		ID: "unrelated-user",
		Fields: map[string]string{
			"policy_resources": "arn:aws:s3:::different-bucket/*",
		},
	}
	cache := resource.ResourceCache{
		"iam-user": resource.ResourceCacheEntry{Resources: []resource.Resource{userRes}},
	}
	source := resource.Resource{ID: "shared-reports"}

	checker := s3CheckerByTarget(t, "iam-user")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkS3R53 — Fields["alias_targets"] contains bucket+".s3" probe
// ---------------------------------------------------------------------------

func TestRelated_S3_R53_Found(t *testing.T) {
	const bucket = "static-site"
	zoneRes := resource.Resource{
		ID: "Z1D633PJN98FT9",
		Fields: map[string]string{
			"alias_targets": "static-site.s3-website.us-east-1.amazonaws.com",
		},
	}
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{zoneRes}},
	}
	source := resource.Resource{ID: bucket}

	checker := s3CheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "Z1D633PJN98FT9" {
		t.Errorf("ResourceIDs = %v, want [Z1D633PJN98FT9]", result.ResourceIDs)
	}
}

func TestRelated_S3_R53_NoMatch(t *testing.T) {
	zoneRes := resource.Resource{
		ID: "ZXXXXX",
		Fields: map[string]string{
			"alias_targets": "different-bucket.s3-website.us-east-1.amazonaws.com",
		},
	}
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{zoneRes}},
	}
	source := resource.Resource{ID: "static-site"}

	checker := s3CheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkS3Role — Fields["policy_resources"] contains bucket ARN
// ---------------------------------------------------------------------------

func TestRelated_S3_Role_Found(t *testing.T) {
	const bucket = "app-data"
	const bucketARN = "arn:aws:s3:::app-data"
	roleRes := resource.Resource{
		ID: "app-role",
		Fields: map[string]string{
			"policy_resources": bucketARN + " " + bucketARN + "/*",
		},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}
	source := resource.Resource{ID: bucket}

	checker := s3CheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_S3_Role_NoMatch(t *testing.T) {
	roleRes := resource.Resource{
		ID: "other-role",
		Fields: map[string]string{
			"policy_resources": "arn:aws:s3:::other-bucket/*",
		},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}
	source := resource.Resource{ID: "app-data"}

	checker := s3CheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkS3WAF — Fields["log_destination_arns"] contains bucket ARN
// ---------------------------------------------------------------------------

func TestRelated_S3_WAF_Found(t *testing.T) {
	const bucket = "waf-logs"
	const bucketARN = "arn:aws:s3:::waf-logs"
	wafRes := resource.Resource{
		ID: "prod-web-acl",
		Fields: map[string]string{
			"log_destination_arns": bucketARN,
		},
	}
	cache := resource.ResourceCache{
		"waf": resource.ResourceCacheEntry{Resources: []resource.Resource{wafRes}},
	}
	source := resource.Resource{ID: bucket}

	checker := s3CheckerByTarget(t, "waf")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "prod-web-acl" {
		t.Errorf("ResourceIDs = %v, want [prod-web-acl]", result.ResourceIDs)
	}
}

func TestRelated_S3_WAF_NoMatch(t *testing.T) {
	wafRes := resource.Resource{
		ID: "other-acl",
		Fields: map[string]string{
			"log_destination_arns": "arn:aws:s3:::different-bucket",
		},
	}
	cache := resource.ResourceCache{
		"waf": resource.ResourceCacheEntry{Resources: []resource.Resource{wafRes}},
	}
	source := resource.Resource{ID: "waf-logs"}

	checker := s3CheckerByTarget(t, "waf")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}
