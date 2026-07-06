// aws_false_zero_cache_miss_test.go pins the contract that a missing
// required cache entry is UNKNOWN (Count:-1, renders "?"), never a
// definitive 0. Five checkers currently return the zero-value
// resource.RelatedCheckResult{TargetType: ...} on a cache-miss branch,
// which is a false zero: a missing cache is not an authoritative answer.
//
// Established contract (this branch, e.g. the kinesis/msk lambda fix):
// missing required cache -> Count:-1. A PRESENT-but-empty cache entry
// (entry exists, zero resources) must still return a definitive 0 — the
// fix must not turn legitimate zeros into -1.
//
// These pins are RED against the current cache-miss branches:
//   - checkCbPipeline        (internal/aws/codebuild_related.go:36)
//   - checkECRPipeline       (internal/aws/ecr_related_extra.go:98)
//   - checkSecretsEB         (internal/aws/secrets_related_extra.go:88)
//   - checkKinesisDDB        (internal/aws/kinesis_related.go:179)
//   - checkECSSvcSFN         (internal/aws/ecs_svc_related_extra.go:397)
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- 1. cb -> pipeline (checkCbPipeline, internal/aws/codebuild_related.go:36) ---

// TestRelated_Cb_Pipeline_CacheMiss_ReturnsUnknown verifies that when the
// "pipeline" cache key is entirely absent, checkCbPipeline returns Count:-1
// (unknown), not the false zero the zero-value struct currently produces.
// No AWS client is needed to reach this branch: the miss check runs before
// the CodePipeline-client guard, so nil clients still isolate the cache-miss
// behavior specifically.
func TestRelated_Cb_Pipeline_CacheMiss_ReturnsUnknown(t *testing.T) {
	checker := cbCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), nil, cbSourceResource("my-build-project"), resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (missing pipeline cache is unknown, not a definitive zero)", result.Count)
	}
}

// TestRelated_Cb_Pipeline_PresentEmptyCache_ReturnsDefinitiveZero verifies
// that a present-but-empty "pipeline" cache entry (entry exists, zero
// resources, IsTruncated=false) still returns a definitive Count:0 — the
// cache-miss fix must not turn legitimate zeros into -1.
func TestRelated_Cb_Pipeline_PresentEmptyCache_ReturnsDefinitiveZero(t *testing.T) {
	cache := resource.ResourceCache{
		"pipeline": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{},
			IsTruncated: false,
		},
	}

	checker := cbCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), nil, cbSourceResource("my-build-project"), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (present-but-empty cache is a definitive zero)", result.Count)
	}
}

// --- 2. ecr -> pipeline (checkECRPipeline, internal/aws/ecr_related_extra.go:98) ---

// TestRelated_ECR_Pipeline_CacheMiss_ReturnsUnknown verifies that when the
// "pipeline" cache key is entirely absent, checkECRPipeline returns
// Count:-1 (unknown), not a false zero.
func TestRelated_ECR_Pipeline_CacheMiss_ReturnsUnknown(t *testing.T) {
	source := resource.Resource{
		ID:   "acme/api-service",
		Name: "acme/api-service",
		Fields: map[string]string{
			"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service",
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String("acme/api-service"),
		},
	}

	checker := ecrCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (missing pipeline cache is unknown, not a definitive zero)", result.Count)
	}
}

// TestRelated_ECR_Pipeline_PresentEmptyCache_ReturnsDefinitiveZero verifies
// that a present-but-empty "pipeline" cache entry still returns a
// definitive Count:0.
func TestRelated_ECR_Pipeline_PresentEmptyCache_ReturnsDefinitiveZero(t *testing.T) {
	source := resource.Resource{
		ID:   "acme/api-service",
		Name: "acme/api-service",
		Fields: map[string]string{
			"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service",
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String("acme/api-service"),
		},
	}
	cache := resource.ResourceCache{
		"pipeline": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{},
			IsTruncated: false,
		},
	}

	checker := ecrCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (present-but-empty cache is a definitive zero)", result.Count)
	}
}

// --- 3. secrets -> eb (checkSecretsEB, internal/aws/secrets_related_extra.go:88) ---

// TestRelated_Secrets_EB_CacheMiss_ReturnsUnknown verifies that when the
// "eb" cache key is entirely absent, checkSecretsEB returns Count:-1
// (unknown), not a false zero. The cache-miss check (line 88) runs before
// the ServiceClients-nil guard (line 93), so nil clients still isolate the
// cache-miss behavior specifically.
func TestRelated_Secrets_EB_CacheMiss_ReturnsUnknown(t *testing.T) {
	source := secretsSourceWithARN(
		"arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password",
		"prod/db/password",
	)

	checker := secretsCheckerByTarget(t, "eb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (missing eb cache is unknown, not a definitive zero)", result.Count)
	}
}

// TestRelated_Secrets_EB_PresentEmptyCache_ReturnsDefinitiveZero verifies
// that a present-but-empty "eb" cache entry still returns a definitive
// Count:0. A non-nil ServiceClients is required to pass the client guard
// that runs immediately after the cache-presence check.
func TestRelated_Secrets_EB_PresentEmptyCache_ReturnsDefinitiveZero(t *testing.T) {
	source := secretsSourceWithARN(
		"arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password",
		"prod/db/password",
	)
	clients := &awsclient.ServiceClients{}
	cache := resource.ResourceCache{
		"eb": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{},
			IsTruncated: false,
		},
	}

	checker := secretsCheckerByTarget(t, "eb")
	result := checker(context.Background(), clients, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (present-but-empty cache is a definitive zero)", result.Count)
	}
}

// --- 4. kinesis -> ddb (checkKinesisDDB, internal/aws/kinesis_related.go:179) ---

// TestRelated_Kinesis_DDB_CacheMiss_ReturnsUnknown verifies that when the
// "ddb" cache key is entirely absent, checkKinesisDDB returns Count:-1
// (unknown), not a false zero. Unlike the other four checkers in this file,
// checkKinesisDDB checks the DynamoDB client BEFORE the cache-presence
// check, so a non-nil ServiceClients with a DynamoDB client satisfying
// DynamoDBDescribeKinesisStreamingDestinationAPI is required to reach the
// cache-miss branch at all — nil clients would short-circuit to -1 for an
// unrelated reason (no client), not the cache-miss bug this test isolates.
func TestRelated_Kinesis_DDB_CacheMiss_ReturnsUnknown(t *testing.T) {
	const streamARN = "arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest"
	fakeDDB := &fakeDynamoDBBatch4{}
	clients := &awsclient.ServiceClients{DynamoDB: fakeDDB}

	checker := kinesisCheckerByTarget(t, "ddb")
	result := checker(context.Background(), clients, kinesisSourceResource("clickstream-ingest", streamARN), resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (missing ddb cache is unknown, not a definitive zero)", result.Count)
	}
}

// TestRelated_Kinesis_DDB_PresentEmptyCache_ReturnsDefinitiveZero verifies
// that a present-but-empty "ddb" cache entry still returns a definitive
// Count:0.
func TestRelated_Kinesis_DDB_PresentEmptyCache_ReturnsDefinitiveZero(t *testing.T) {
	const streamARN = "arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest"
	fakeDDB := &fakeDynamoDBBatch4{}
	clients := &awsclient.ServiceClients{DynamoDB: fakeDDB}
	cache := resource.ResourceCache{
		"ddb": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{},
			IsTruncated: false,
		},
	}

	checker := kinesisCheckerByTarget(t, "ddb")
	result := checker(context.Background(), clients, kinesisSourceResource("clickstream-ingest", streamARN), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (present-but-empty cache is a definitive zero)", result.Count)
	}
}

// --- 5. ecs-svc -> sfn (checkECSSvcSFN, internal/aws/ecs_svc_related_extra.go:397) ---

// TestRelated_ECSSvc_SFN_CacheMiss_ReturnsUnknown verifies that when the
// "sfn" cache key is entirely absent, checkECSSvcSFN returns Count:-1
// (unknown), not a false zero. No client-nil guard sits in front of the
// cache-presence check, so nil clients isolate the cache-miss behavior
// specifically.
func TestRelated_ECSSvc_SFN_CacheMiss_ReturnsUnknown(t *testing.T) {
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"

	checker := ecsSvcCheckerByTarget(t, "sfn")
	result := checker(context.Background(), nil, ecsSvcSourceResource("api-service", "prod-cluster", taskDefARN), resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (missing sfn cache is unknown, not a definitive zero)", result.Count)
	}
}

// TestRelated_ECSSvc_SFN_PresentEmptyCache_ReturnsDefinitiveZero verifies
// that a present-but-empty "sfn" cache entry still returns a definitive
// Count:0.
func TestRelated_ECSSvc_SFN_PresentEmptyCache_ReturnsDefinitiveZero(t *testing.T) {
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"
	cache := resource.ResourceCache{
		"sfn": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{},
			IsTruncated: false,
		},
	}

	checker := ecsSvcCheckerByTarget(t, "sfn")
	result := checker(context.Background(), nil, ecsSvcSourceResource("api-service", "prod-cluster", taskDefARN), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (present-but-empty cache is a definitive zero)", result.Count)
	}
}
