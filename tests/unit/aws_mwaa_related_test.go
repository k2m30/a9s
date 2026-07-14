package unit_test

// aws_mwaa_related_test.go — related-resource checker tests for mwaa
// (docs/resources/mwaa.md §2, docs/resources/mwaa-impl-plan.md §1
// "related_targets"). Checkers live in internal/aws/mwaa_related.go.
//
// kms/logs/role/s3/sg/subnet are field-driven (read a field on the
// Environment, no API call) — these are exercised against the REAL
// FetchMWAAEnvironmentsPage output for the graph-root fixture, avoiding any
// guess at internal Fields/RawStruct shape (mirrors
// TestSubnet_Related_EKS_ResolvesViaRealFetcherOutput). alarm is a
// cache-scan checker matching the CloudWatch AWS/MWAA EnvironmentName
// dimension (same shape as checkDbiAlarm/checkGlueAlarms). ct-events is the
// universal ctEventsCheckerFor("mwaa") pivot.

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// mwaaGraphRootResource fetches the real prod-airflow-etl Resource via
// FetchMWAAEnvironmentsPage backed by the shared demo fixtures/fake.
func mwaaGraphRootResource(t *testing.T) resource.Resource {
	t.Helper()
	clients := &awsclient.ServiceClients{MWAA: fakes.NewMWAA()}
	result, err := awsclient.FetchMWAAEnvironmentsPage(context.Background(), clients, "")
	if err != nil && len(result.Resources) == 0 {
		// The demo set includes the details-denied witness, so the fetch
		// legitimately returns rows + a composite error (E5 partial success).
		t.Fatalf("FetchMWAAEnvironmentsPage returned error: %v", err)
	}
	for _, r := range result.Resources {
		if r.ID == fixtures.ProdAirflowEtlID {
			return r
		}
	}
	t.Fatalf("resource %q not found in fetch result", fixtures.ProdAirflowEtlID)
	return resource.Resource{}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func TestRelated_MWAA_Registered(t *testing.T) {
	defs := resource.GetRelated("mwaa")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for mwaa")
	}

	expected := map[string]string{
		"kms":       "KMS Key",
		"logs":      "Log Groups",
		"role":      "IAM Roles",
		"s3":        "S3 Buckets",
		"sg":        "Security Groups",
		"subnet":    "Subnets",
		"alarm":     "CW Alarms",
		"ct-events": "CloudTrail Events",
	}
	seen := map[string]bool{}
	for _, def := range defs {
		wantDisplay, ok := expected[def.TargetType]
		if !ok {
			continue
		}
		seen[def.TargetType] = true
		if def.Checker == nil {
			t.Errorf("mwaa %q: Checker should not be nil", def.TargetType)
		}
		if def.DisplayName != wantDisplay {
			t.Errorf("mwaa %q: DisplayName = %q, want %q", def.TargetType, def.DisplayName, wantDisplay)
		}
		if def.TargetType == "alarm" && !def.NeedsTargetCache {
			// Every "CW Alarms" registration in this codebase sets
			// NeedsTargetCache so the related panel prefetches the alarm
			// list before the row can resolve a count.
			t.Error("mwaa alarm related def: NeedsTargetCache = false, want true")
		}
	}
	for target := range expected {
		if !seen[target] {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

func TestRelated_MWAA_ExcludedTargetsNotRegistered(t *testing.T) {
	defs := resource.GetRelated("mwaa")
	for _, excluded := range []string{"vpc", "sqs", "vpce"} {
		for _, def := range defs {
			if def.TargetType == excluded {
				t.Errorf("mwaa: target %q must not be registered (spec §2 explicitly excludes it)", excluded)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Field-driven checkers — graph root (prod-airflow-etl) counts per
// mwaa-impl-plan.md §1 "related_targets": kms 1, logs 5, role 1, s3 1, sg 2,
// subnet 2.
// ---------------------------------------------------------------------------

func TestRelated_MWAA_GraphRootCounts(t *testing.T) {
	res := mwaaGraphRootResource(t)
	for _, tc := range []struct {
		target string
		want   int
		field  string
	}{
		{"kms", 1, "KmsKey"},
		{"logs", 5, "five per-component LoggingConfiguration log groups"},
		{"role", 1, "ExecutionRoleArn"},
		{"s3", 1, "SourceBucketArn"},
		{"sg", 2, "NetworkConfiguration.SecurityGroupIds"},
		{"subnet", 2, "NetworkConfiguration.SubnetIds"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			checker := checkerByTarget(t, "mwaa", tc.target)
			result := checker(context.Background(), nil, res, resource.ResourceCache{})
			if result.Count != tc.want {
				t.Errorf("Count = %d, want %d (%s)", result.Count, tc.want, tc.field)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// alarm — cache-scan checker matching the AWS/MWAA EnvironmentName dimension.
// ---------------------------------------------------------------------------

func TestRelated_MWAA_Alarm_MatchesByEnvironmentNameDimension(t *testing.T) {
	res := mwaaGraphRootResource(t)
	checker := checkerByTarget(t, "mwaa", "alarm")

	mkAlarm := func(id, namespace, dimName, dimValue string) resource.Resource {
		return resource.Resource{
			ID:   id,
			Name: id,
			RawStruct: cwtypes.MetricAlarm{
				AlarmName: aws.String(id),
				Namespace: aws.String(namespace),
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String(dimName), Value: aws.String(dimValue)},
				},
			},
		}
	}
	matching1 := mkAlarm("mwaa-scheduler-heartbeat", "AWS/MWAA", "EnvironmentName", fixtures.ProdAirflowEtlID)
	matching2 := mkAlarm("mwaa-queued-tasks", "AWS/MWAA", "EnvironmentName", fixtures.ProdAirflowEtlID)
	otherEnv := mkAlarm("mwaa-other-env-heartbeat", "AWS/MWAA", "EnvironmentName", fixtures.ProdAirflowReportingID)
	otherNamespace := mkAlarm("ec2-cpu-alarm", "AWS/EC2", "InstanceId", "i-0a1b2c3d")

	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources: []resource.Resource{matching1, matching2, otherEnv, otherNamespace},
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Fatalf("ResourceIDs = %v, want 2 entries", result.ResourceIDs)
	}
}

func TestRelated_MWAA_Alarm_ColdCache_NoErr(t *testing.T) {
	res := mwaaGraphRootResource(t)
	checker := checkerByTarget(t, "mwaa", "alarm")

	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Err != nil {
		t.Errorf("Err = %v, want nil — a cold cache renders a blank navigable row, not an error state", result.Err)
	}
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (cold cache)", result.Count)
	}
}

// mwaaFailingCloudWatchFake implements awsclient.CloudWatchAPI, failing every
// call — used to exercise the alarm checker's live-fetch-on-cache-miss error
// path.
type mwaaFailingCloudWatchFake struct{}

func (f *mwaaFailingCloudWatchFake) DescribeAlarms(
	_ context.Context, _ *cloudwatch.DescribeAlarmsInput, _ ...func(*cloudwatch.Options),
) (*cloudwatch.DescribeAlarmsOutput, error) {
	return nil, fmt.Errorf("cloudwatch: DescribeAlarms: AccessDeniedException")
}

func (f *mwaaFailingCloudWatchFake) DescribeAlarmHistory(
	_ context.Context, _ *cloudwatch.DescribeAlarmHistoryInput, _ ...func(*cloudwatch.Options),
) (*cloudwatch.DescribeAlarmHistoryOutput, error) {
	return nil, fmt.Errorf("cloudwatch: DescribeAlarmHistory: AccessDeniedException")
}

var _ awsclient.CloudWatchAPI = (*mwaaFailingCloudWatchFake)(nil)

func TestRelated_MWAA_Alarm_LiveFetchErrorSurfacesErr(t *testing.T) {
	res := mwaaGraphRootResource(t)
	checker := checkerByTarget(t, "mwaa", "alarm")

	clients := &awsclient.ServiceClients{CloudWatch: &mwaaFailingCloudWatchFake{}}
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Err == nil {
		t.Fatal("mwaa alarm checker must surface Err when the alarm list must be live-fetched (cache miss) " +
			"and DescribeAlarms fails")
	}
}

// ---------------------------------------------------------------------------
// ct-events — universal ctEventsCheckerFor("mwaa") pivot: deferred,
// server-side FetchFilter, drillable.
// ---------------------------------------------------------------------------

func TestRelated_MWAA_CtEvents_Drillable(t *testing.T) {
	res := mwaaGraphRootResource(t)
	checker := checkerByTarget(t, "mwaa", "ct-events")

	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State != domain.RelatedDeferred {
		t.Errorf("State = %v, want RelatedDeferred (ct-events is a universal server-side pivot, "+
			"drillable by resource name)", result.State)
	}
	if len(result.FetchFilter) == 0 {
		t.Error("FetchFilter is empty, want a CloudTrail LookupEvents filter keyed on the environment name")
	}
}
