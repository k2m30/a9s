package unit

// Per-row CloudWatch metrics such as FreeStorageSpace or CPUUtilization would
// cost one API call per row per metric.

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	rdsv2 "github.com/aws/aws-sdk-go-v2/service/rds"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

type recordingCWClient struct {
	awsclient.CloudWatchAPI // embed nil — panics if any unoverridden method is called
	calls                   []string
	callMade                bool
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

	if cwMock.callMade {
		t.Errorf("CloudWatch was called during FetchRDSInstancesPage: %v", cwMock.calls)
	}

	outOfScopeMetrics := []string{
		"FreeStorageSpace",
		"CPUUtilization",
		"ReplicaLag",
		"DatabaseConnections",
	}
	for _, metric := range outOfScopeMetrics {
		for _, r := range result.Resources {
			if _, ok := r.Fields[metric]; ok {
				t.Errorf("resource %s unexpectedly contains out-of-scope Wave 3 metric field %q", r.ID, metric)
			}
		}
	}
}

func TestDBI_Wave3_EnrichDBIMaintenance_NoCloudWatchCalls(t *testing.T) {
	cwMock := &recordingCWClient{}
	clients := &awsclient.ServiceClients{
		RDS:        &noCWMaintenanceFake{},
		CloudWatch: cwMock,
	}

	resources := buildDbiResources(t)
	_, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance returned unexpected error: %v", err)
	}

	if cwMock.callMade {
		t.Errorf("EnrichDBIMaintenance called CloudWatch (Wave 3 violation): %v", cwMock.calls)
	}
}

func TestDBI_Wave3_EnrichDBIMaintenance_NilCloudWatchNoPanic(t *testing.T) {
	clients := &awsclient.ServiceClients{
		RDS:        &noCWMaintenanceFake{},
		CloudWatch: nil,
	}

	resources := buildDbiResources(t)
	_, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance returned unexpected error with nil CW client: %v", err)
	}
}

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
	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
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
