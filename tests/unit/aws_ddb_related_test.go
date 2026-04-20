package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ddbtypesForTest constructs a ddbtypes.TableDescription for tests without
// pulling in the aws.String helper at every call site.
type ddbtypesForTest struct {
	TableName       string
	LatestStreamArn string
}

func (d ddbtypesForTest) Build() ddbtypes.TableDescription {
	out := ddbtypes.TableDescription{}
	if d.TableName != "" {
		out.TableName = aws.String(d.TableName)
	}
	if d.LatestStreamArn != "" {
		out.LatestStreamArn = aws.String(d.LatestStreamArn)
	}
	return out
}

func TestRelated_DDB_Registered(t *testing.T) {
	defs := resource.GetRelated("ddb")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ddb")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"kms":    {"KMS Key", true},
		"lambda": {"Lambda Functions", true},
		"alarm":  {"CloudWatch Alarms", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("ddb %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("ddb %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("ddb %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// ddbCheckerByTarget returns the RelatedChecker for the given target type registered
// under "ddb". It fails the test immediately if the checker is nil or not found.
func ddbCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ddb") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ddb related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ddb related checker for %s not found", target)
	return nil
}

// --- checkDdbAlarm tests (Pattern D — dimension-based) ---

func TestRelated_DDB_Alarm_MatchByDimension(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "ddb-cpu-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("ddb-cpu-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("TableName"),
					Value: aws.String("my-table"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{
		ID:     "my-table",
		Fields: map[string]string{},
	}

	checker := ddbCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "ddb-cpu-alarm" {
		t.Errorf("ResourceIDs = %v, want [ddb-cpu-alarm]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_DDB_Alarm_NoMatch(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "ddb-other-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("ddb-other-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("TableName"),
					Value: aws.String("other-table"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{
		ID:     "my-table",
		Fields: map[string]string{},
	}

	checker := ddbCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_DDB_Alarm_EmptyID(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "ddb-cpu-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("ddb-cpu-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("TableName"),
					Value: aws.String("my-table"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{
		ID:     "",
		Fields: map[string]string{},
	}

	checker := ddbCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

func TestRelated_DDB_Alarm_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "my-table",
		Fields: map[string]string{},
	}

	checker := ddbCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — empty cache, no clients)", result.Count)
	}
}

// --- ddb→lambda: requires live API (lambda:ListEventSourceMappings on stream ARN) ---

// TestRelated_DDB_Lambda_NoStreamReturnsZero verifies that when streams are
// disabled on the table (LatestStreamArn is nil/empty), the checker reports
// Count=0 without calling any API — no Lambda trigger is possible.
func TestRelated_DDB_Lambda_NoStreamReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-orders-table",
		Name: "acme-orders-table",
		RawStruct: ddbtypesForTest{
			TableName: "acme-orders-table",
			// No LatestStreamArn — streams disabled.
		}.Build(),
	}
	checker := ddbCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (streams disabled)", result.Count)
	}
	if result.TargetType != "lambda" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "lambda")
	}
}

// TestRelated_DDB_Lambda_StreamsEnabledUnknownWithoutClients verifies that when
// streams are enabled but no live Lambda client is available, the checker
// reports Count=-1 (undeterminable) rather than a silent zero.
func TestRelated_DDB_Lambda_StreamsEnabledUnknownWithoutClients(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-orders-table",
		Name: "acme-orders-table",
		RawStruct: ddbtypesForTest{
			TableName:       "acme-orders-table",
			LatestStreamArn: "arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders-table/stream/2026-01-01T00:00:00.000",
		}.Build(),
	}
	checker := ddbCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (requires live lambda:ListEventSourceMappings)", result.Count)
	}
}

// TestRelated_DDB_Lambda_InvalidRawStruct verifies the checker reports
// Count=-1 when the RawStruct is not a TableDescription (cannot read streams).
func TestRelated_DDB_Lambda_InvalidRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "acme-orders-table",
		RawStruct: "not-a-table",
	}
	checker := ddbCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (bad raw struct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkDdbBackup — Pattern C: ListRecoveryPointsByResource on table ARN
// ---------------------------------------------------------------------------

// TestRelated_Ddb_Backup_Match verifies that a table with a known ARN field,
// and a Backup fake returning 2 recovery points, yields Count=2.
func TestRelated_Ddb_Backup_Match(t *testing.T) {
	const tableARN = "arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders-table"
	rp1 := "arn:aws:backup:us-east-1:123456789012:recovery-point:ddb-00000001"
	rp2 := "arn:aws:backup:us-east-1:123456789012:recovery-point:ddb-00000002"

	src := resource.Resource{
		ID:   "acme-orders-table",
		Name: "acme-orders-table",
		Fields: map[string]string{
			"arn": tableARN,
		},
		RawStruct: ddbtypes.TableDescription{
			TableName: aws.String("acme-orders-table"),
		},
	}
	clients := &awsclient.ServiceClients{
		Backup: newFakeBackupWithRecoveryPoints([]backuptypes.RecoveryPointByResource{
			{RecoveryPointArn: &rp1},
			{RecoveryPointArn: &rp2},
		}),
	}
	checker := ddbCheckerByTarget(t, "backup")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Errorf("ResourceIDs = %v, want 2 entries", result.ResourceIDs)
	}
}

// TestRelated_Ddb_Backup_Empty verifies that a table with no ARN field
// returns Count=0.
func TestRelated_Ddb_Backup_Empty(t *testing.T) {
	src := resource.Resource{
		ID:     "acme-orders-table",
		Name:   "acme-orders-table",
		Fields: map[string]string{},
		RawStruct: ddbtypes.TableDescription{
			TableName: aws.String("acme-orders-table"),
		},
	}
	checker := ddbCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty table ARN)", result.Count)
	}
}

// TestRelated_Ddb_Backup_WrongRawStruct verifies that a table with a valid ARN
// field but nil clients returns Count=-1 (no Backup client).
func TestRelated_Ddb_Backup_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:   "acme-orders-table",
		Name: "acme-orders-table",
		Fields: map[string]string{
			"arn": "arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders-table",
		},
		RawStruct: "not-a-table",
	}
	checker := ddbCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkDdbKinesis — Pattern C: DescribeKinesisStreamingDestination on table name
// ---------------------------------------------------------------------------

// TestRelated_Ddb_Kinesis_Match verifies that a table with 2 Kinesis stream
// destinations yields Count=2 with stream names in ResourceIDs.
func TestRelated_Ddb_Kinesis_Match(t *testing.T) {
	streamARN1 := "arn:aws:kinesis:us-east-1:123456789012:stream/orders-stream"
	streamARN2 := "arn:aws:kinesis:us-east-1:123456789012:stream/events-stream"

	src := resource.Resource{
		ID:   "acme-orders-table",
		Name: "acme-orders-table",
		Fields: map[string]string{
			"arn": "arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders-table",
		},
		RawStruct: ddbtypes.TableDescription{
			TableName: aws.String("acme-orders-table"),
		},
	}
	clients := &awsclient.ServiceClients{
		DynamoDB: newFakeDDBWithKinesisDestinations([]ddbtypes.KinesisDataStreamDestination{
			{StreamArn: &streamARN1},
			{StreamArn: &streamARN2},
		}),
	}
	checker := ddbCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs {
		seen[id] = true
	}
	if !seen["orders-stream"] {
		t.Errorf("ResourceIDs missing orders-stream; got %v", result.ResourceIDs)
	}
	if !seen["events-stream"] {
		t.Errorf("ResourceIDs missing events-stream; got %v", result.ResourceIDs)
	}
}

// TestRelated_Ddb_Kinesis_Empty verifies that a table with an empty ID returns
// Count=0 (no table name to look up).
func TestRelated_Ddb_Kinesis_Empty(t *testing.T) {
	src := resource.Resource{
		ID:   "",
		Name: "",
		RawStruct: ddbtypes.TableDescription{
			TableName: aws.String(""),
		},
	}
	checker := ddbCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty table ID)", result.Count)
	}
}

// TestRelated_Ddb_Kinesis_WrongRawStruct verifies that a table with a valid ID
// but nil clients returns Count=-1 (no DynamoDB client).
func TestRelated_Ddb_Kinesis_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "acme-orders-table",
		RawStruct: "not-a-table",
	}
	checker := ddbCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkDdbLogs — Pattern C: substring match on log group ID
// ---------------------------------------------------------------------------

// TestRelated_Ddb_Logs_Match verifies that log groups whose ID contains the
// table name as a substring are returned.
func TestRelated_Ddb_Logs_Match(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/aws/dynamodb/my-orders-table/data-plane",
		Fields: map[string]string{},
	}
	otherLog := resource.Resource{
		ID:     "/aws/lambda/unrelated-function",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes, otherLog}},
	}
	src := resource.Resource{ID: "my-orders-table", Fields: map[string]string{}}

	checker := ddbCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/aws/dynamodb/my-orders-table/data-plane" {
		t.Errorf("ResourceIDs = %v, want [/aws/dynamodb/my-orders-table/data-plane]", result.ResourceIDs)
	}
}

// TestRelated_Ddb_Logs_NoMatch verifies that when no log group contains the
// table name, Count=0 is returned.
func TestRelated_Ddb_Logs_NoMatch(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/aws/lambda/unrelated-function",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}
	src := resource.Resource{ID: "my-orders-table", Fields: map[string]string{}}

	checker := ddbCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// TestRelated_Ddb_Logs_EmptyID verifies that an empty table ID returns Count=0.
func TestRelated_Ddb_Logs_EmptyID(t *testing.T) {
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "/aws/dynamodb/something"},
		}},
	}
	src := resource.Resource{ID: "", Fields: map[string]string{}}

	checker := ddbCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty table ID)", result.Count)
	}
}

// TestRelated_Ddb_Logs_NilCache verifies that an empty cache returns Count=-1.
func TestRelated_Ddb_Logs_NilCache(t *testing.T) {
	src := resource.Resource{ID: "my-orders-table", Fields: map[string]string{}}

	checker := ddbCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, no clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkDdbVPCE — Pattern C: service_name field contains ".dynamodb"
// ---------------------------------------------------------------------------

// TestRelated_Ddb_VPCE_Match verifies that VPC endpoints with a DynamoDB
// service_name are returned regardless of table name (account-level resource).
func TestRelated_Ddb_VPCE_Match(t *testing.T) {
	vpceRes := resource.Resource{
		ID: "vpce-0a1b2c3d4e5f67890",
		Fields: map[string]string{
			"service_name": "com.amazonaws.us-east-1.dynamodb",
		},
	}
	otherVPCE := resource.Resource{
		ID: "vpce-aaabbbbccc0000111",
		Fields: map[string]string{
			"service_name": "com.amazonaws.us-east-1.s3",
		},
	}
	cache := resource.ResourceCache{
		"vpce": resource.ResourceCacheEntry{Resources: []resource.Resource{vpceRes, otherVPCE}},
	}
	src := resource.Resource{ID: "my-orders-table", Fields: map[string]string{}}

	checker := ddbCheckerByTarget(t, "vpce")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vpce-0a1b2c3d4e5f67890" {
		t.Errorf("ResourceIDs = %v, want [vpce-0a1b2c3d4e5f67890]", result.ResourceIDs)
	}
}

// TestRelated_Ddb_VPCE_NoMatch verifies that endpoints without ".dynamodb" in
// service_name return Count=0.
func TestRelated_Ddb_VPCE_NoMatch(t *testing.T) {
	vpceRes := resource.Resource{
		ID: "vpce-aaabbbbccc0000111",
		Fields: map[string]string{
			"service_name": "com.amazonaws.us-east-1.s3",
		},
	}
	cache := resource.ResourceCache{
		"vpce": resource.ResourceCacheEntry{Resources: []resource.Resource{vpceRes}},
	}
	src := resource.Resource{ID: "my-orders-table", Fields: map[string]string{}}

	checker := ddbCheckerByTarget(t, "vpce")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no DynamoDB endpoint)", result.Count)
	}
}

// TestRelated_Ddb_VPCE_NilCache verifies that an empty cache returns Count=-1.
func TestRelated_Ddb_VPCE_NilCache(t *testing.T) {
	src := resource.Resource{ID: "my-orders-table", Fields: map[string]string{}}

	checker := ddbCheckerByTarget(t, "vpce")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkDdbKMS — Pattern F (SSEDescription.KMSMasterKeyArn)
// ---------------------------------------------------------------------------

// TestRelated_Ddb_KMS_InvalidRawStruct verifies Count=-1 when the RawStruct
// is not a TableDescription.
func TestRelated_Ddb_KMS_InvalidRawStruct(t *testing.T) {
	src := resource.Resource{ID: "acme-orders", RawStruct: "not-a-table"}
	checker := ddbCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (bad raw struct)", result.Count)
	}
}

// TestRelated_Ddb_KMS_NoSSEDescription verifies Count=0 when SSEDescription is nil.
func TestRelated_Ddb_KMS_NoSSEDescription(t *testing.T) {
	src := resource.Resource{
		ID:        "acme-orders",
		RawStruct: ddbtypes.TableDescription{TableName: aws.String("acme-orders")},
	}
	checker := ddbCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no SSEDescription)", result.Count)
	}
}

// TestRelated_Ddb_KMS_ARNMissingSlash verifies Count=0 when KMSMasterKeyArn
// has no "/" separator (cannot extract key ID).
func TestRelated_Ddb_KMS_ARNMissingSlash(t *testing.T) {
	src := resource.Resource{
		ID: "acme-orders",
		RawStruct: ddbtypes.TableDescription{
			SSEDescription: &ddbtypes.SSEDescription{
				KMSMasterKeyArn: aws.String("arn:aws:kms:us-east-1:123456789012:key"),
			},
		},
	}
	checker := ddbCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no slash in ARN)", result.Count)
	}
}

// TestRelated_Ddb_KMS_ValidARN verifies that a well-formed ARN yields
// Count=1 and the key ID extracted after the last "/".
func TestRelated_Ddb_KMS_ValidARN(t *testing.T) {
	const keyID = "a1b2c3d4-5678-90ab-cdef-111111111111"
	src := resource.Resource{
		ID: "acme-orders",
		RawStruct: ddbtypes.TableDescription{
			SSEDescription: &ddbtypes.SSEDescription{
				KMSMasterKeyArn: aws.String("arn:aws:kms:us-east-1:123456789012:key/" + keyID),
			},
		},
	}
	checker := ddbCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != keyID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, keyID)
	}
}

// ---------------------------------------------------------------------------
// checkDdbLambda — Pattern A: live ListEventSourceMappings
// ---------------------------------------------------------------------------

// TestRelated_Ddb_Lambda_StreamEnabledWithMappings verifies that when streams
// are enabled and the Lambda fake returns two function ARNs, Count=2.
func TestRelated_Ddb_Lambda_StreamEnabledWithMappings(t *testing.T) {
	fn1 := "arn:aws:lambda:us-east-1:123456789012:function:orders-processor"
	fn2 := "arn:aws:lambda:us-east-1:123456789012:function:orders-dlq"
	src := resource.Resource{
		ID: "acme-orders",
		RawStruct: ddbtypes.TableDescription{
			TableName:       aws.String("acme-orders"),
			LatestStreamArn: aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders/stream/2026-01-01T00:00:00.000"),
		},
	}
	clients := &awsclient.ServiceClients{
		Lambda: newFakeLambdaWithESMFunctions([]string{fn1, fn2}),
	}
	checker := ddbCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})
	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Errorf("ResourceIDs = %v, want 2 entries", result.ResourceIDs)
	}
}

// TestRelated_Ddb_Lambda_StreamEnabledNoMappings verifies Count=0 when the
// Lambda API returns no event source mappings.
func TestRelated_Ddb_Lambda_StreamEnabledNoMappings(t *testing.T) {
	src := resource.Resource{
		ID: "acme-orders",
		RawStruct: ddbtypes.TableDescription{
			TableName:       aws.String("acme-orders"),
			LatestStreamArn: aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders/stream/2026-01-01T00:00:00.000"),
		},
	}
	clients := &awsclient.ServiceClients{
		Lambda: newFakeLambdaWithESMFunctions([]string{}),
	}
	checker := ddbCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no mappings)", result.Count)
	}
}
