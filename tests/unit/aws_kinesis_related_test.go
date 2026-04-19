package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func kinesisCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("kinesis") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("kinesis related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("kinesis related checker for %s not found", target)
	return nil
}

// --- Navigable Fields ---

func TestNavigableFields_Kinesis_None(t *testing.T) {
	nav := resource.IsFieldNavigable("kinesis", "StreamName")
	if nav != nil {
		t.Errorf("expected no navigable fields for kinesis, but StreamName resolved to %v", nav)
	}
}

// --- CloudWatch Alarms checker (Pattern C — cache, StreamName dimension) ---

func TestRelated_Kinesis_Alarms_Found(t *testing.T) {
	const streamName = "clickstream-ingest"

	alarmRes := resource.Resource{
		ID: "kinesis-iterator-age",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("kinesis-iterator-age"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("StreamName"), Value: aws.String(streamName)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   streamName,
		Name: streamName,
		Fields: map[string]string{
			"stream_name": streamName,
		},
	}

	checker := kinesisCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "kinesis-iterator-age" {
		t.Errorf("ResourceIDs = %v, want [kinesis-iterator-age]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Kinesis_Alarms_NotFound(t *testing.T) {
	const streamName = "clickstream-ingest"

	alarmRes := resource.Resource{
		ID: "other-stream-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-stream-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("StreamName"), Value: aws.String("different-stream")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   streamName,
		Name: streamName,
		Fields: map[string]string{
			"stream_name": streamName,
		},
	}

	checker := kinesisCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Kinesis_Alarms_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "clickstream-ingest",
		Name: "clickstream-ingest",
		Fields: map[string]string{
			"stream_name": "clickstream-ingest",
		},
	}

	checker := kinesisCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- checkKinesisLambda (scan lambda cache for event_source_arn match) ---

func TestRelated_Kinesis_Lambda_Found(t *testing.T) {
	const streamARN = "arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest"
	lambdaRes := resource.Resource{
		ID:   "process-clickstream",
		Name: "process-clickstream",
		Fields: map[string]string{
			"event_source_arn": streamARN,
		},
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}
	source := resource.Resource{
		ID:   "clickstream-ingest",
		Name: "clickstream-ingest",
		Fields: map[string]string{
			"stream_name": "clickstream-ingest",
			"stream_arn":  streamARN,
		},
	}

	checker := kinesisCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "process-clickstream" {
		t.Errorf("ResourceIDs = %v, want [process-clickstream]", result.ResourceIDs)
	}
}

func TestRelated_Kinesis_Lambda_NotFound(t *testing.T) {
	lambdaRes := resource.Resource{
		ID:   "unrelated-fn",
		Name: "unrelated-fn",
		Fields: map[string]string{
			"event_source_arn": "arn:aws:sqs:us-east-1:123456789012:other-queue",
		},
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}
	source := resource.Resource{
		ID:   "clickstream-ingest",
		Name: "clickstream-ingest",
		Fields: map[string]string{
			"stream_name": "clickstream-ingest",
			"stream_arn":  "arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest",
		},
	}

	checker := kinesisCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no lambda event-source match)", result.Count)
	}
}

func TestRelated_Kinesis_Lambda_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "clickstream-ingest",
		Name: "clickstream-ingest",
		Fields: map[string]string{
			"stream_name": "clickstream-ingest",
			"stream_arn":  "arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest",
		},
	}
	checker := kinesisCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache miss, no clients)", result.Count)
	}
}

// --- kinesis→cfn: undeterminable without ListTagsForStream, returns Count: -1 ---

func TestRelated_Kinesis_CFN_Unknown(t *testing.T) {
	source := resource.Resource{
		ID:   "clickstream-ingest",
		Name: "clickstream-ingest",
	}
	checker := kinesisCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (tags need ListTagsForStream enrichment)", result.Count)
	}
	if result.TargetType != "cfn" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cfn")
	}
}

// ---------------------------------------------------------------------------
// kinesis→ddb (Pattern C+reverse: cache["ddb"] scan, DescribeKinesisStreamingDestination)
// ---------------------------------------------------------------------------

// kinesisSourceResource builds a Kinesis stream resource used as the parent.
func kinesisSourceResource(streamName, streamARN string) resource.Resource {
	return resource.Resource{
		ID:   streamName,
		Name: streamName,
		Fields: map[string]string{
			"stream_arn": streamARN,
		},
	}
}

// ddbTableResource builds a DynamoDB table cache entry.
func ddbTableResource(tableName string) resource.Resource {
	return resource.Resource{
		ID:   tableName,
		Name: tableName,
	}
}

// TestRelated_Kinesis_DDB_Match verifies that a DynamoDB table whose
// DescribeKinesisStreamingDestination returns this stream's ARN is returned
// with Count=1.
func TestRelated_Kinesis_DDB_Match(t *testing.T) {
	const streamName = "clickstream-ingest"
	const streamARN = "arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest"
	const tableName = "events-table"

	fakeDDB := newFakeDynamoDBWithKinesisDestination(tableName, streamARN)
	clients := &awsclient.ServiceClients{DynamoDB: fakeDDB}

	cache := resource.ResourceCache{
		"ddb": resource.ResourceCacheEntry{
			Resources: []resource.Resource{ddbTableResource(tableName)},
		},
	}

	checker := kinesisCheckerByTarget(t, "ddb")
	result := checker(context.Background(), clients, kinesisSourceResource(streamName, streamARN), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != tableName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, tableName)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Kinesis_DDB_Match_Truncated verifies that IsTruncated propagates
// to Approximate=true while Count still reflects found matches.
func TestRelated_Kinesis_DDB_Match_Truncated(t *testing.T) {
	const streamName = "clickstream-ingest"
	const streamARN = "arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest"
	const tableName = "events-table"

	fakeDDB := newFakeDynamoDBWithKinesisDestination(tableName, streamARN)
	clients := &awsclient.ServiceClients{DynamoDB: fakeDDB}

	cache := resource.ResourceCache{
		"ddb": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{ddbTableResource(tableName)},
			IsTruncated: true,
		},
	}

	checker := kinesisCheckerByTarget(t, "ddb")
	result := checker(context.Background(), clients, kinesisSourceResource(streamName, streamARN), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if !result.Approximate {
		t.Error("Approximate = false, want true (cache is truncated)")
	}
}

// TestRelated_Kinesis_DDB_Empty verifies that a DynamoDB table not streaming to
// this Kinesis stream returns Count=0.
func TestRelated_Kinesis_DDB_Empty(t *testing.T) {
	const streamName = "clickstream-ingest"
	const streamARN = "arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest"
	const otherStreamARN = "arn:aws:kinesis:us-east-1:123456789012:stream/other-stream"
	const tableName = "events-table"

	fakeDDB := newFakeDynamoDBWithKinesisDestination(tableName, otherStreamARN)
	clients := &awsclient.ServiceClients{DynamoDB: fakeDDB}

	cache := resource.ResourceCache{
		"ddb": resource.ResourceCacheEntry{
			Resources: []resource.Resource{ddbTableResource(tableName)},
		},
	}

	checker := kinesisCheckerByTarget(t, "ddb")
	result := checker(context.Background(), clients, kinesisSourceResource(streamName, streamARN), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (table streams to a different Kinesis stream)", result.Count)
	}
}

// TestRelated_Kinesis_DDB_MissingCache verifies that a missing "ddb" cache key
// returns the zero-value (Count=0), not Count=-1.
func TestRelated_Kinesis_DDB_MissingCache(t *testing.T) {
	const streamARN = "arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest"

	fakeDDB := &fakeDynamoDBBatch4{}
	clients := &awsclient.ServiceClients{DynamoDB: fakeDDB}

	checker := kinesisCheckerByTarget(t, "ddb")
	result := checker(context.Background(), clients, kinesisSourceResource("clickstream-ingest", streamARN), resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (cache key missing returns zero-value)", result.Count)
	}
}

// TestRelated_Kinesis_DDB_NoStreamARN verifies that a stream resource with no
// stream_arn field returns Count=0 (not an error).
func TestRelated_Kinesis_DDB_NoStreamARN(t *testing.T) {
	source := resource.Resource{
		ID:     "clickstream-ingest",
		Name:   "clickstream-ingest",
		Fields: map[string]string{},
	}

	checker := kinesisCheckerByTarget(t, "ddb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no stream ARN field)", result.Count)
	}
}

// TestRelated_Kinesis_DDB_NoClient verifies that nil/missing DynamoDB client
// returns Count=-1.
func TestRelated_Kinesis_DDB_NoClient(t *testing.T) {
	const streamARN = "arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest"

	cache := resource.ResourceCache{
		"ddb": resource.ResourceCacheEntry{
			Resources: []resource.Resource{ddbTableResource("events-table")},
		},
	}

	checker := kinesisCheckerByTarget(t, "ddb")
	result := checker(context.Background(), nil, kinesisSourceResource("clickstream-ingest", streamARN), cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no DynamoDB client)", result.Count)
	}
}

// TestRelated_Kinesis_DDB_FetchFilter verifies that the checker does NOT populate
// FetchFilter — reverse-scan checkers must not set FetchFilter (Fix 3).
func TestRelated_Kinesis_DDB_FetchFilter(t *testing.T) {
	const streamARN = "arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest"

	fakeDDB := &fakeDynamoDBBatch4{}
	clients := &awsclient.ServiceClients{DynamoDB: fakeDDB}

	cache := resource.ResourceCache{
		"ddb": resource.ResourceCacheEntry{Resources: []resource.Resource{}},
	}

	checker := kinesisCheckerByTarget(t, "ddb")
	result := checker(context.Background(), clients, kinesisSourceResource("clickstream-ingest", streamARN), cache)

	if len(result.FetchFilter) != 0 {
		t.Errorf("FetchFilter = %v, want empty (reverse-scan checkers must not set FetchFilter)", result.FetchFilter)
	}
}

// ---------------------------------------------------------------------------
// checkKinesisCFN — ListTagsForStream + cfn cache lookup
// ---------------------------------------------------------------------------

// TestRelated_Kinesis_CFN_FoundViaTags verifies that when the stream has the
// aws:cloudformation:stack-name tag and the matching stack is in the cfn cache,
// the checker returns Count=1.
func TestRelated_Kinesis_CFN_FoundViaTags(t *testing.T) {
	const streamName = "clickstream-ingest"

	fakeKinesis := &fakeKinesisWithTagsAndDesc{cfnStackName: "data-platform-stack"}
	clients := &awsclient.ServiceClients{Kinesis: fakeKinesis}

	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "data-platform-stack", Name: "data-platform-stack", Fields: map[string]string{"stack_name": "data-platform-stack"}},
			{ID: "unrelated-stack", Name: "unrelated-stack", Fields: map[string]string{"stack_name": "unrelated-stack"}},
		}},
	}
	source := resource.Resource{ID: streamName, Name: streamName}

	checker := kinesisCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "data-platform-stack" {
		t.Errorf("ResourceIDs = %v, want [data-platform-stack]", result.ResourceIDs)
	}
}

// TestRelated_Kinesis_CFN_NoMatchingStack verifies that when the stream has the
// cfn tag but the named stack is not in the cache, Count=0 is returned.
func TestRelated_Kinesis_CFN_NoMatchingStack(t *testing.T) {
	const streamName = "clickstream-ingest"

	fakeKinesis := &fakeKinesisWithTagsAndDesc{cfnStackName: "missing-stack"}
	clients := &awsclient.ServiceClients{Kinesis: fakeKinesis}

	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "different-stack", Name: "different-stack", Fields: map[string]string{"stack_name": "different-stack"}},
		}},
	}
	source := resource.Resource{ID: streamName, Name: streamName}

	checker := kinesisCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (stack not in cache)", result.Count)
	}
}

// TestRelated_Kinesis_CFN_NoTag verifies that when ListTagsForStream returns no
// cfn tag, Count=0 is returned immediately without scanning the cfn cache.
func TestRelated_Kinesis_CFN_NoTag(t *testing.T) {
	const streamName = "clickstream-ingest"

	fakeKinesis := &fakeKinesisWithTagsAndDesc{cfnStackName: ""} // no cfn tag
	clients := &awsclient.ServiceClients{Kinesis: fakeKinesis}

	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "some-stack", Name: "some-stack"},
		}},
	}
	source := resource.Resource{ID: streamName, Name: streamName}

	checker := kinesisCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no cfn tag on stream)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkKinesisKMS — DescribeStreamSummary → KeyId extraction
// ---------------------------------------------------------------------------

// TestRelated_Kinesis_KMS_Present verifies that a stream with KMS encryption
// has its KeyId extracted (stripping the alias prefix) and returned as Count=1.
func TestRelated_Kinesis_KMS_Present(t *testing.T) {
	const streamName = "clickstream-ingest"

	fakeKinesis := &fakeKinesisWithTagsAndDesc{kmsKeyID: "alias/aws/kinesis/mrk-abc1234"}
	clients := &awsclient.ServiceClients{Kinesis: fakeKinesis}

	source := resource.Resource{ID: streamName, Name: streamName}

	checker := kinesisCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (KMS key present)", result.Count)
	}
	// KeyId after stripping last "/" prefix segment: "mrk-abc1234"
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "mrk-abc1234" {
		t.Errorf("ResourceIDs = %v, want [mrk-abc1234]", result.ResourceIDs)
	}
}

// TestRelated_Kinesis_KMS_Absent verifies that a stream with no KMS key returns Count=0.
func TestRelated_Kinesis_KMS_Absent(t *testing.T) {
	const streamName = "clickstream-ingest"

	fakeKinesis := &fakeKinesisWithTagsAndDesc{kmsKeyID: ""} // no encryption
	clients := &awsclient.ServiceClients{Kinesis: fakeKinesis}

	source := resource.Resource{ID: streamName, Name: streamName}

	checker := kinesisCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no KMS key)", result.Count)
	}
}

// TestRelated_Kinesis_KMS_BareKeyID verifies that a plain key ID (no "/" prefix)
// is returned unchanged.
func TestRelated_Kinesis_KMS_BareKeyID(t *testing.T) {
	const streamName = "clickstream-ingest"
	const bareKeyID = "mrk-00112233445566778899aabbccddeeff"

	fakeKinesis := &fakeKinesisWithTagsAndDesc{kmsKeyID: bareKeyID}
	clients := &awsclient.ServiceClients{Kinesis: fakeKinesis}

	source := resource.Resource{ID: streamName, Name: streamName}

	checker := kinesisCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (bare key ID)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != bareKeyID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, bareKeyID)
	}
}
