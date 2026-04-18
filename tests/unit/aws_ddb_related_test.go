package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	_ "github.com/k2m30/a9s/v3/internal/aws"
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
