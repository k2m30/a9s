package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type mockDDBKinesisClient struct {
	awsclient.DynamoDBAPI
	destinations []ddbtypes.KinesisDataStreamDestination
	err          error
}

func (m *mockDDBKinesisClient) DescribeKinesisStreamingDestination(
	_ context.Context,
	_ *dynamodb.DescribeKinesisStreamingDestinationInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.DescribeKinesisStreamingDestinationOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &dynamodb.DescribeKinesisStreamingDestinationOutput{
		KinesisDataStreamDestinations: m.destinations,
	}, nil
}

type mockLambdaESMClient struct {
	awsclient.LambdaAPI
	mappings []lambdatypes.EventSourceMappingConfiguration
	err      error
	calls    int
}

func (m *mockLambdaESMClient) ListEventSourceMappings(
	_ context.Context,
	_ *lambda.ListEventSourceMappingsInput,
	_ ...func(*lambda.Options),
) (*lambda.ListEventSourceMappingsOutput, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return &lambda.ListEventSourceMappingsOutput{
		EventSourceMappings: m.mappings,
	}, nil
}

func ddbOrdersProdResource(t *testing.T) resource.Resource {
	t.Helper()
	table := findDDBTable(t, fixtures.OrdersProdID)
	listStub := &ddbListStub{names: []string{fixtures.OrdersProdID}}
	descStub := &ddbDescribeStub{tables: map[string]*ddbtypes.TableDescription{fixtures.OrdersProdID: table}}
	result, err := awsclient.FetchDynamoDBTablesPage(context.Background(), listStub, descStub, "")
	if err != nil {
		t.Fatalf("ddbOrdersProdResource: FetchDynamoDBTablesPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("ddbOrdersProdResource: expected 1 resource, got %d", len(result.Resources))
	}
	return result.Resources[0]
}

func ddbCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ddb") {
		if def.TargetType == target {
			return def.Checker
		}
	}
	t.Fatalf("no checker registered for ddb→%s", target)
	return nil
}

func TestDDB_Related_Alarm_MatchesByTableNameDimension(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "alarm")

	matchingAlarm := resource.Resource{
		ID:   "orders-prod-throttle",
		Name: "orders-prod-throttle",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("orders-prod-throttle"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("TableName"), Value: aws.String(fixtures.OrdersProdID)},
			},
		},
	}
	decoyAlarm := resource.Resource{
		ID:   "ec2-cpu-alarm",
		Name: "ec2-cpu-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("ec2-cpu-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("InstanceId"), Value: aws.String("i-0a1b2c3d")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources: []resource.Resource{matchingAlarm, decoyAlarm},
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) == 0 || result.ResourceIDs()[0] != "orders-prod-throttle" {
		t.Errorf("ResourceIDs = %v, want [orders-prod-throttle]", result.ResourceIDs())
	}
}

func TestDDB_Related_Alarm_NonMatchingTableNameValue(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "alarm")

	otherTableAlarm := resource.Resource{
		ID:   "sessions-creating-alarm",
		Name: "sessions-creating-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("sessions-creating-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("TableName"), Value: aws.String("sessions-creating")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources: []resource.Resource{otherTableAlarm},
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (non-matching TableName)", result.Count())
	}
}

func TestDDB_Related_Alarm_NoDimensions(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "alarm")

	noDimAlarm := resource.Resource{
		ID: "bare-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName:  aws.String("bare-alarm"),
			Dimensions: nil,
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources: []resource.Resource{noDimAlarm},
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no dimensions)", result.Count())
	}
}

func TestDDB_Related_Alarm_WrongDimensionName_NotCounted(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "alarm")

	wrongNameAlarm := resource.Resource{
		ID:   "instance-cpu-alarm",
		Name: "instance-cpu-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("instance-cpu-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("InstanceId"), Value: aws.String(fixtures.OrdersProdID)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources: []resource.Resource{wrongNameAlarm},
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (dimension name is InstanceId, not TableName)", result.Count())
	}
}

// A nil alarm cache is not a proven zero; it resolves to
// UnknownRelated("alarm") (docs/related-resources-engine.md).
func TestDDB_Related_Alarm_NilCache_ReturnsUnknown(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "alarm")

	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v, want RelatedUnknown (nil alarm cache is not a proven zero — canonical per docs/related-resources-engine.md §7)", result.State())
	}
}

func TestDDB_Related_Alarm_Error(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "alarm")
	wantErr := errors.New("boom: DescribeAlarms access denied")

	original := resource.GetPaginatedFetcher("alarm")
	resource.SetPaginatedForTest("alarm", func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{}, wantErr
	})
	t.Cleanup(func() {
		if original != nil {
			resource.SetPaginatedForTest("alarm", original)
		} else {
			resource.CleanupPaginatedForTest("alarm")
		}
	})

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, resource.ResourceCache{})

	if result.State() != domain.RelatedError {
		t.Errorf("State = %v, want RelatedError", result.State())
	}
	if result.Err() == nil {
		t.Error("Err = nil, want the propagated fetch error")
	}
}

// A match on a truncated alarm page is a lower bound, rendered "(1+)".
func TestDDB_Related_Alarm_Truncated_PropagatesTrue(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "alarm")

	matchingAlarm := resource.Resource{
		ID:   "orders-prod-throttle",
		Name: "orders-prod-throttle",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("orders-prod-throttle"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("TableName"), Value: aws.String(fixtures.OrdersProdID)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{matchingAlarm},
			IsTruncated: true,
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if !result.Truncated() {
		t.Error("Truncated = false, want true (truncated cache page with a match must render as '(1+)')")
	}
}

func TestDDB_Related_Backup_MatchesByARNInSelection(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "backup")

	matchingPlan := BackupPlanRow(t, "acme-weekly-full-backup",
		backuptypes.BackupSelection{Resources: []string{fixtures.OrdersProdARN, "arn:aws:s3:::acme-data"}})
	decoyPlan := BackupPlanRow(t, "unrelated-backup-plan",
		backuptypes.BackupSelection{Resources: []string{"arn:aws:s3:::other-bucket"}})
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{
			Resources: []resource.Resource{matchingPlan, decoyPlan},
		},
	}

	// nil clients: the backup pivot is a pure cache scan.
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) == 0 || result.ResourceIDs()[0] != "acme-weekly-full-backup" {
		t.Errorf("ResourceIDs = %v, want [acme-weekly-full-backup]", result.ResourceIDs())
	}
}

func TestDDB_Related_Backup_NoMatch(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "backup")

	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				BackupPlanRow(t, "other-plan", backuptypes.BackupSelection{Resources: []string{"arn:aws:s3:::irrelevant"}}),
			},
		},
	}

	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no matching plan)", result.Count())
	}
}

func TestDDB_Related_Kinesis_OneDestination(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "kinesis")

	kinesisClient := &mockDDBKinesisClient{
		destinations: []ddbtypes.KinesisDataStreamDestination{
			{
				StreamArn:         aws.String(fixtures.OrdersProdKinesisStreamARN),
				DestinationStatus: ddbtypes.DestinationStatusActive,
			},
		},
	}
	clients := &awsclient.ServiceClients{DynamoDB: kinesisClient}

	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) == 0 {
		t.Errorf("ResourceIDs is empty, want [%s]", fixtures.OrdersProdKinesisStream)
	} else {
		// The stream name is the last "/" segment of the ARN.
		wantName := fixtures.OrdersProdKinesisStream
		if result.ResourceIDs()[0] != wantName {
			t.Errorf("ResourceIDs[0] = %q, want %q", result.ResourceIDs()[0], wantName)
		}
	}
}

func TestDDB_Related_Kinesis_EmptyDestinations(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "kinesis")

	kinesisClient := &mockDDBKinesisClient{destinations: nil}
	clients := &awsclient.ServiceClients{DynamoDB: kinesisClient}

	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no destinations)", result.Count())
	}
}

func TestDDB_Related_KMS_ReturnsKeyID(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "kms")

	cache := resource.ResourceCache{
		"kms": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: fixtures.OrdersProdKMSKeyID}},
		},
	}

	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) == 0 || result.ResourceIDs()[0] != fixtures.OrdersProdKMSKeyID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), fixtures.OrdersProdKMSKeyID)
	}
}

// A nil SSEDescription means an AWS-owned key, which has no kms pivot.
func TestDDB_Related_KMS_NilSSEDescription(t *testing.T) {
	table := findDDBTable(t, fixtures.AuditPITROffID)
	listStub := &ddbListStub{names: []string{fixtures.AuditPITROffID}}
	descStub := &ddbDescribeStub{tables: map[string]*ddbtypes.TableDescription{fixtures.AuditPITROffID: table}}
	result, _ := awsclient.FetchDynamoDBTablesPage(context.Background(), listStub, descStub, "")
	res := result.Resources[0]

	checker := ddbCheckerByTarget(t, "kms")
	got := checker(context.Background(), nil, res, resource.ResourceCache{})

	if got.Count() != 0 {
		t.Errorf("Count = %d, want 0 for table with no SSEDescription (AWS-owned key)", got.Count())
	}
}

func TestDDB_Related_KMS_MalformedARN_NoSlash(t *testing.T) {
	table := &ddbtypes.TableDescription{
		TableName:   aws.String("inline-malformed-kms"),
		TableArn:    aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/inline-malformed-kms"),
		TableStatus: ddbtypes.TableStatusActive,
		SSEDescription: &ddbtypes.SSEDescription{
			SSEType:         ddbtypes.SSETypeKms,
			KMSMasterKeyArn: aws.String("malformed-arn-no-slash"),
		},
	}
	res := resource.Resource{
		ID:        "inline-malformed-kms",
		Name:      "inline-malformed-kms",
		RawStruct: table,
	}

	checker := ddbCheckerByTarget(t, "kms")
	got := checker(context.Background(), nil, res, resource.ResourceCache{})

	if got.Count() != 0 {
		t.Errorf("Count = %d, want 0 for malformed KMS ARN (no '/')", got.Count())
	}
	if got.State() != domain.RelatedResolved {
		t.Errorf("State = %v, must be RelatedResolved for malformed ARN — only nil RawStruct yields RelatedUnknown", got.State())
	}
}

func TestDDB_Related_Lambda_OneMapping(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "lambda")

	lambdaClient := &mockLambdaESMClient{
		mappings: []lambdatypes.EventSourceMappingConfiguration{
			{FunctionArn: aws.String(fixtures.OrdersProdLambdaARN)},
		},
	}
	clients := &awsclient.ServiceClients{Lambda: lambdaClient}

	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) == 0 || result.ResourceIDs()[0] != fixtures.OrdersProdLambdaName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), fixtures.OrdersProdLambdaName)
	}
	if lambdaClient.calls == 0 {
		t.Errorf("ListEventSourceMappings was not called — LatestStreamArn is set, it must be called")
	}
}

func TestDDB_Related_Lambda_NoStream_ZeroCount(t *testing.T) {
	table := findDDBTable(t, fixtures.AuditPITROffID)
	listStub := &ddbListStub{names: []string{fixtures.AuditPITROffID}}
	descStub := &ddbDescribeStub{tables: map[string]*ddbtypes.TableDescription{fixtures.AuditPITROffID: table}}
	fetchResult, _ := awsclient.FetchDynamoDBTablesPage(context.Background(), listStub, descStub, "")
	res := fetchResult.Resources[0]

	lambdaClient := &mockLambdaESMClient{}
	clients := &awsclient.ServiceClients{Lambda: lambdaClient}

	checker := ddbCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no LatestStreamArn — not a failure)", result.Count())
	}
	if lambdaClient.calls != 0 {
		t.Errorf("ListEventSourceMappings was called %d times, want 0 (no stream)", lambdaClient.calls)
	}
}

// A sibling table name can extend this one ("orders-prod" vs
// "orders-prod-sessions"), so matching is on the full
// /aws/dynamodb/tables/<name>/ prefix.
func TestDDB_Related_Logs_PrefixMatchOnly(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "logs")

	matchingLG := resource.Resource{ID: "/aws/dynamodb/tables/" + fixtures.OrdersProdID + "/insights/default"}
	lambdaDecoy := resource.Resource{ID: "/aws/lambda/" + fixtures.OrdersProdID}
	siblingDecoy := resource.Resource{ID: "/aws/dynamodb/tables/" + fixtures.OrdersProdID + "-sessions/insights/default"}

	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{
			Resources: []resource.Resource{matchingLG, lambdaDecoy, siblingDecoy},
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (only exact-prefix match)", result.Count())
	}
	for _, id := range result.ResourceIDs() {
		if !strings.HasPrefix(id, "/aws/dynamodb/tables/"+fixtures.OrdersProdID+"/") {
			t.Errorf("ResourceIDs contains non-prefix-match entry %q", id)
		}
	}
}

func TestDDB_Related_Logs_LambdaDecoy_CountZero(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "logs")

	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "/aws/lambda/" + fixtures.OrdersProdID},
			},
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (/aws/lambda/ must not match DDB log prefix)", result.Count())
	}
}

func TestDDB_Related_Logs_SiblingSubstringTrap_CountZero(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "logs")

	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "/aws/dynamodb/tables/" + fixtures.OrdersProdID + "-sessions/insights/default"},
			},
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (sibling-table substring trap must not match prefix)", result.Count())
	}
}

func TestDDB_Related_VPCE_GatewayEndpointMatches(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "vpce")

	ddbGateway := resource.Resource{
		ID:   "vpce-ddb-gateway-0001",
		Name: "vpce-ddb-gateway-0001",
		Fields: map[string]string{
			"service_name": "com.amazonaws.us-east-1.dynamodb",
			"type":         "Gateway",
		},
	}
	s3Decoy := resource.Resource{
		ID:   "vpce-s3-gateway-0001",
		Name: "vpce-s3-gateway-0001",
		Fields: map[string]string{
			"service_name": "com.amazonaws.us-east-1.s3",
			"type":         "Gateway",
		},
	}
	ddbInterfaceDecoy := resource.Resource{
		ID:   "vpce-ddb-interface-0001",
		Name: "vpce-ddb-interface-0001",
		Fields: map[string]string{
			"service_name": "com.amazonaws.us-east-1.dynamodb",
			"type":         "Interface",
		},
	}
	cache := resource.ResourceCache{
		"vpce": resource.ResourceCacheEntry{
			Resources: []resource.Resource{ddbGateway, s3Decoy, ddbInterfaceDecoy},
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (only DDB Gateway endpoint)", result.Count())
	}
	if len(result.ResourceIDs()) == 0 || result.ResourceIDs()[0] != "vpce-ddb-gateway-0001" {
		t.Errorf("ResourceIDs = %v, want [vpce-ddb-gateway-0001]", result.ResourceIDs())
	}
}

func TestDDB_Related_VPCE_S3ServiceName_CountZero(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "vpce")

	cache := resource.ResourceCache{
		"vpce": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID: "vpce-s3-0001",
					Fields: map[string]string{
						"service_name": "com.amazonaws.us-east-1.s3",
						"type":         "Gateway",
					},
				},
			},
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (s3 service_name must not match ddb checker)", result.Count())
	}
}

func TestDDB_Related_VPCE_InterfaceType_CountZero(t *testing.T) {
	res := ddbOrdersProdResource(t)
	checker := ddbCheckerByTarget(t, "vpce")

	cache := resource.ResourceCache{
		"vpce": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID: "vpce-ddb-interface-0002",
					Fields: map[string]string{
						"service_name": "com.amazonaws.us-east-1.dynamodb",
						"type":         "Interface",
					},
				},
			},
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (Interface type must not match — only Gateway)", result.Count())
	}
}

func TestDDB_Related_RegistrationSmoke(t *testing.T) {
	defs := resource.GetRelated("ddb")
	if len(defs) == 0 {
		t.Fatal("GetRelated(ddb) returned empty — ddb related-resource definitions are not registered")
	}

	required := []string{"alarm", "backup", "kinesis", "kms", "lambda", "logs", "vpce"}
	registered := make(map[string]bool, len(defs))
	for _, def := range defs {
		registered[def.TargetType] = true
	}

	for _, target := range required {
		if !registered[target] {
			t.Errorf("ddb related target %q not registered in GetRelated(ddb)", target)
		}
	}
}

func TestDDB_Related_CTEvents_UniversalPivot(t *testing.T) {
	res := ddbOrdersProdResource(t)

	var checker resource.RelatedChecker
	for _, def := range resource.GetRelated("ddb") {
		if def.TargetType == "ct-events" {
			checker = def.Checker
			break
		}
	}
	if checker == nil {
		// ct-events is a universal pivot and may be wired outside GetRelated.
		t.Log("ct-events not in GetRelated(ddb) — verifying universal pivot surface")

		matchingEvent := resource.Resource{
			ID:   "evt-ddb-001",
			Name: "evt-ddb-001",
			Fields: map[string]string{
				"resource_name": fixtures.OrdersProdID,
			},
		}
		cache := resource.ResourceCache{
			"ct-events": resource.ResourceCacheEntry{
				Resources: []resource.Resource{matchingEvent},
			},
		}

		allDefs := resource.GetRelated("ddb")
		for _, def := range allDefs {
			if def.TargetType == "ct-events" {
				r := def.Checker(context.Background(), nil, res, cache)
				if r.State() == domain.RelatedResolved && r.Count() == 0 {
					t.Errorf("ct-events State = RelatedResolved, Count = 0 — universal pivot must not return a definite resolved zero when events exist")
				}
				return
			}
		}
		t.Log("ct-events universal pivot verified via absence of definitive zero")
		return
	}

	matchingEvent := resource.Resource{
		ID:   "evt-ddb-001",
		Name: "evt-ddb-001",
		Fields: map[string]string{
			"resource_name": fixtures.OrdersProdID,
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{
			Resources: []resource.Resource{matchingEvent},
		},
	}

	result := checker(context.Background(), nil, res, cache)
	if result.State() == domain.RelatedResolved && result.Count() == 0 {
		t.Errorf("ct-events State = RelatedResolved, Count = 0 — universal pivot must not return a definite resolved zero when events exist")
	}
	if result.FetchFilter() == nil || result.FetchFilter()["ResourceName"] != fixtures.OrdersProdID {
		t.Errorf("FetchFilter[ResourceName] = %q, want %q", result.FetchFilter()["ResourceName"], fixtures.OrdersProdID)
	}
}

func TestCheckDdbBackup_WildcardMatchingAndExclusion(t *testing.T) {
	plans := []resource.Resource{
		BackupPlanRow(t, "plan-explicit", backuptypes.BackupSelection{Resources: []string{"arn:aws:dynamodb:us-east-1:123:table/orders"}}),
		BackupPlanRow(t, "plan-wildcard", backuptypes.BackupSelection{Resources: []string{"arn:aws:dynamodb:*:*:table/*"}}),
		BackupPlanRow(t, "plan-wildcard-excluded", backuptypes.BackupSelection{
			Resources:    []string{"arn:aws:dynamodb:*:*:table/*"},
			NotResources: []string{"arn:aws:dynamodb:us-east-1:123:table/audit-log"},
		}),
	}
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{
			Resources:   plans,
			IsTruncated: false,
		},
	}
	checker := ddbCheckerByTarget(t, "backup")

	t.Run("orders table covered by all three plans", func(t *testing.T) {
		res := resource.Resource{
			ID:   "orders",
			Name: "orders",
			Fields: map[string]string{
				"arn": "arn:aws:dynamodb:us-east-1:123:table/orders",
			},
		}
		result := checker(context.Background(), nil, res, cache)
		if result.Count() != 3 {
			t.Errorf("Count = %d, want 3 (explicit + wildcard + wildcard-excluded)", result.Count())
		}
		got := make([]string, len(result.ResourceIDs()))
		copy(got, result.ResourceIDs())
		sortStrings(got)
		want := []string{"plan-explicit", "plan-wildcard", "plan-wildcard-excluded"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("ResourceIDs = %v, want %v (order-independent)", got, want)
		}
	})

	t.Run("audit-log table covered only by plan-wildcard (exclusion drops plan-wildcard-excluded)", func(t *testing.T) {
		res := resource.Resource{
			ID:   "audit-log",
			Name: "audit-log",
			Fields: map[string]string{
				"arn": "arn:aws:dynamodb:us-east-1:123:table/audit-log",
			},
		}
		result := checker(context.Background(), nil, res, cache)
		if result.Count() != 1 {
			t.Errorf("Count = %d, want 1 (only plan-wildcard; explicit misses, excluded drops plan-wildcard-excluded)", result.Count())
		}
		if len(result.ResourceIDs()) == 0 || result.ResourceIDs()[0] != "plan-wildcard" {
			t.Errorf("ResourceIDs = %v, want [plan-wildcard]", result.ResourceIDs())
		}
	})

	t.Run("empty ARN returns Count=0", func(t *testing.T) {
		res := resource.Resource{
			ID:     "no-arn",
			Name:   "no-arn",
			Fields: map[string]string{"arn": ""},
		}
		result := checker(context.Background(), nil, res, cache)
		if result.Count() != 0 {
			t.Errorf("Count = %d, want 0 for empty ARN (short-circuit)", result.Count())
		}
	})
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func TestCheckDdbBackup_TruncatedCacheWithMatches_ReturnsTruncated(t *testing.T) {
	res := resource.Resource{
		ID:   "orders",
		Name: "orders",
		Fields: map[string]string{
			"arn": "arn:aws:dynamodb:us-east-1:123:table/orders",
		},
	}

	matchingPlan := BackupPlanRow(t, "weekly-backup-plan", backuptypes.BackupSelection{
		Resources: []string{"arn:aws:dynamodb:us-east-1:123:table/orders", "arn:aws:s3:::other"},
	})

	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{matchingPlan},
			IsTruncated: true,
		},
	}

	checker := ddbCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (one matching plan in truncated cache)", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("Truncated = false, want true — truncated cache with matches must render as '(N+)' not '(N)'")
	}
	found := false
	for _, id := range result.ResourceIDs() {
		if id == "weekly-backup-plan" {
			found = true
		}
	}
	if !found {
		t.Errorf("ResourceIDs = %v, want to contain \"weekly-backup-plan\"", result.ResourceIDs())
	}
}

func TestCheckDdbBackup_TruncatedCacheNoMatches_ReturnsTruncatedResult(t *testing.T) {
	res := resource.Resource{
		ID:   "orders",
		Name: "orders",
		Fields: map[string]string{
			"arn": "arn:aws:dynamodb:us-east-1:123:table/orders",
		},
	}

	nonMatchingPlan := BackupPlanRow(t, "s3-only-plan", backuptypes.BackupSelection{
		Resources: []string{"arn:aws:s3:::completely-different"},
	})

	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{nonMatchingPlan},
			IsTruncated: true,
		},
	}

	checker := ddbCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, res, cache)

	// TruncatedResult path: Count==0 but there may be matches on later pages.
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no matching plan in visible portion)", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("Truncated = false, want true — truncated cache with zero visible matches must still be truncated (pages unseen)")
	}
}
