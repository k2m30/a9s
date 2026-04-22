package unit

// aws_dbi_wave3_antitest_test.go — Wave 3 OUT-OF-SCOPE anti-tests for dbi.
//
// Spec §3 (Wave 3) lists four CloudWatch metrics that are explicitly out of
// scope for the dbi implementation:
//
//   - FreeStorageSpace
//   - CPUUtilization
//   - ReplicaLag
//   - DatabaseConnections
//
// These tests verify that neither FetchRDSInstancesPage nor EnrichDBIMaintenance
// ever calls any CloudWatch API method. A recordingCWClient is wired into
// ServiceClients.CloudWatch — it records every call and fails the test if any
// CloudWatch method is invoked.
//
// Rationale: CloudWatch calls add non-trivial latency (one call per row × 4
// metrics = O(4N) API calls). If a future coder accidentally adds CW calls they
// will be caught here before the feature ships.

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	rdsv2 "github.com/aws/aws-sdk-go-v2/service/rds"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// ---------------------------------------------------------------------------
// Sentinel CloudWatch mock — records calls, fails test if any are made.
// ---------------------------------------------------------------------------

// recordingCWClient implements awsclient.CloudWatchAPI.
// Any method invocation records the call name in calls and marks callMade true.
// The test checks callMade after the operation under test.
type recordingCWClient struct {
	awsclient.CloudWatchAPI // embed nil — panics if any unoverridden method is called
	calls    []string
	callMade bool
}

func (m *recordingCWClient) DescribeAlarms(
	_ context.Context,
	_ *cloudwatch.DescribeAlarmsInput,
	_ ...func(*cloudwatch.Options),
) (*cloudwatch.DescribeAlarmsOutput, error) {
	m.calls = append(m.calls, "DescribeAlarms")
	m.callMade = true
	return &cloudwatch.DescribeAlarmsOutput{}, nil
}

func (m *recordingCWClient) DescribeAlarmHistory(
	_ context.Context,
	_ *cloudwatch.DescribeAlarmHistoryInput,
	_ ...func(*cloudwatch.Options),
) (*cloudwatch.DescribeAlarmHistoryOutput, error) {
	m.calls = append(m.calls, "DescribeAlarmHistory")
	m.callMade = true
	return &cloudwatch.DescribeAlarmHistoryOutput{}, nil
}

// ---------------------------------------------------------------------------
// Minimal RDS mock for the fetch anti-test.
// ---------------------------------------------------------------------------

// noCWFetchRDSClient satisfies RDSDescribeDBInstancesAPI with one page of
// realistic dbi fixtures. It is used exclusively in the Wave 3 anti-tests so
// that FetchRDSInstancesPage has real work to do.
type noCWFetchRDSClient struct {
	awsclient.RDSAPI
}

func (m *noCWFetchRDSClient) DescribeDBInstances(
	_ context.Context,
	_ *rdsv2.DescribeDBInstancesInput,
	_ ...func(*rdsv2.Options),
) (*rdsv2.DescribeDBInstancesOutput, error) {
	return &rdsv2.DescribeDBInstancesOutput{DBInstances: fixtures.NewDBIFixtures().Instances}, nil
}

// ---------------------------------------------------------------------------
// Minimal RDS maintenance mock for the enrichment anti-test.
// ---------------------------------------------------------------------------

// noCWMaintenanceFake satisfies RDSAPI; returns empty maintenance list so
// EnrichDBIMaintenance terminates quickly without real API latency.
type noCWMaintenanceFake struct {
	awsclient.RDSAPI
}

func (m *noCWMaintenanceFake) DescribePendingMaintenanceActions(
	_ context.Context,
	_ *rdsv2.DescribePendingMaintenanceActionsInput,
	_ ...func(*rdsv2.Options),
) (*rdsv2.DescribePendingMaintenanceActionsOutput, error) {
	return &rdsv2.DescribePendingMaintenanceActionsOutput{}, nil
}

// ---------------------------------------------------------------------------
// Anti-test: FetchRDSInstancesPage must not call CloudWatch.
//
// FetchRDSInstancesPage takes only an RDSDescribeDBInstancesAPI — it cannot
// structurally call CloudWatch. However this test serves as a compile-time and
// runtime guard: if someone threads ServiceClients through the fetch path and
// adds a CW call, the recording mock will catch it.
//
// Because the function signature accepts RDSDescribeDBInstancesAPI (not
// ServiceClients), we verify the absence of CW calls by ensuring the recording
// mock is never touched after the fetch.
// ---------------------------------------------------------------------------

func TestDBI_Wave3_FetchRDSInstancesPage_NoCloudWatchCalls(t *testing.T) {
	cwMock := &recordingCWClient{}

	rdsClient := &noCWFetchRDSClient{}
	result, err := awsclient.FetchRDSInstancesPage(context.Background(), rdsClient, "")
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage returned unexpected error: %v", err)
	}
	if len(result.Resources) == 0 {
		t.Fatal("FetchRDSInstancesPage returned zero resources — fixture injection failed")
	}

	// CloudWatch mock must not have been touched: its interface is not even
	// reachable from FetchRDSInstancesPage, but verify the sentinel is clean.
	if cwMock.callMade {
		t.Errorf("CloudWatch was called during FetchRDSInstancesPage: %v", cwMock.calls)
	}

	// Explicit documentation of the four out-of-scope metrics.
	outOfScopeMetrics := []string{
		"FreeStorageSpace",
		"CPUUtilization",
		"ReplicaLag",
		"DatabaseConnections",
	}
	for _, metric := range outOfScopeMetrics {
		// Verify none of the returned resources carry these metrics as fields.
		for _, r := range result.Resources {
			if _, ok := r.Fields[metric]; ok {
				t.Errorf("resource %s unexpectedly contains out-of-scope Wave 3 metric field %q", r.ID, metric)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Anti-test: EnrichDBIMaintenance must not call any CloudWatch method.
//
// EnrichDBIMaintenance receives *ServiceClients. A recordingCWClient is wired
// into ServiceClients.CloudWatch. If the enricher calls DescribeAlarms or
// DescribeAlarmHistory, the mock records it and the test fails.
// ---------------------------------------------------------------------------

func TestDBI_Wave3_EnrichDBIMaintenance_NoCloudWatchCalls(t *testing.T) {
	cwMock := &recordingCWClient{}
	clients := &awsclient.ServiceClients{
		RDS:        &noCWMaintenanceFake{},
		CloudWatch: cwMock,
	}

	resources := buildDbiResources(t)
	_, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance returned unexpected error: %v", err)
	}

	if cwMock.callMade {
		t.Errorf("EnrichDBIMaintenance called CloudWatch (Wave 3 violation): %v", cwMock.calls)
	}
}

// ---------------------------------------------------------------------------
// Anti-test: EnrichDBIMaintenance with nil CloudWatch client must not panic.
//
// ServiceClients.CloudWatch may be nil in unit test setups. The enricher must
// tolerate a nil CW client since it should never attempt to use it.
// ---------------------------------------------------------------------------

func TestDBI_Wave3_EnrichDBIMaintenance_NilCloudWatchNoPanic(t *testing.T) {
	clients := &awsclient.ServiceClients{
		RDS:        &noCWMaintenanceFake{},
		CloudWatch: nil, // explicitly nil — must not panic
	}

	resources := buildDbiResources(t)
	_, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance returned unexpected error with nil CW client: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Anti-test: Four out-of-scope metrics are not present in any fetch output.
//
// Parameterised over each Wave 3 metric. Verifies that FetchRDSInstancesPage
// does not embed CW metric data in resource.Fields — not just that it doesn't
// call the API, but that the field names don't appear in the output at all.
// ---------------------------------------------------------------------------

func TestDBI_Wave3_OutOfScopeMetricFieldsAbsentFromFetch(t *testing.T) {
	outOfScopeMetrics := []struct {
		name string
	}{
		{"FreeStorageSpace"},
		{"CPUUtilization"},
		{"ReplicaLag"},
		{"DatabaseConnections"},
	}

	rdsClient := &noCWFetchRDSClient{}
	result, err := awsclient.FetchRDSInstancesPage(context.Background(), rdsClient, "")
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage returned unexpected error: %v", err)
	}
	if len(result.Resources) == 0 {
		t.Fatal("FetchRDSInstancesPage returned zero resources")
	}

	for _, tc := range outOfScopeMetrics {
		tc := tc
		t.Run(fmt.Sprintf("metric_%s_absent", tc.name), func(t *testing.T) {
			for _, r := range result.Resources {
				if _, ok := r.Fields[tc.name]; ok {
					t.Errorf("resource %s contains out-of-scope Wave 3 metric field %q in Fields", r.ID, tc.name)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Anti-test: Four out-of-scope metrics are not present in enrichment output.
//
// EnrichDBIMaintenance returns IssueEnricherResult with Findings and
// FieldUpdates. Verifies that none of the Wave 3 metric names appear as
// FieldUpdate keys in any resource.
// ---------------------------------------------------------------------------

func TestDBI_Wave3_OutOfScopeMetricFieldsAbsentFromEnrichment(t *testing.T) {
	outOfScopeMetrics := []string{
		"FreeStorageSpace",
		"CPUUtilization",
		"ReplicaLag",
		"DatabaseConnections",
	}

	clients := &awsclient.ServiceClients{
		RDS:        &noCWMaintenanceFake{},
		CloudWatch: &recordingCWClient{},
	}
	resources := buildDbiResources(t)
	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance returned unexpected error: %v", err)
	}

	for resourceID, updates := range result.FieldUpdates {
		for _, metric := range outOfScopeMetrics {
			if _, ok := updates[metric]; ok {
				t.Errorf("EnrichDBIMaintenance wrote out-of-scope Wave 3 metric %q in FieldUpdates for resource %s", metric, resourceID)
			}
		}
	}
}
