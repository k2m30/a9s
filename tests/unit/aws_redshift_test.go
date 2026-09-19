package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

func redshiftFixtureByID(t *testing.T, id string) redshifttypes.Cluster {
	t.Helper()
	for _, c := range fixtures.NewRedshiftFixtures().Clusters {
		if c.ClusterIdentifier != nil && *c.ClusterIdentifier == id {
			return c
		}
	}
	t.Fatalf("redshiftFixtureByID: no fixture with id %q", id)
	return redshifttypes.Cluster{}
}

func fetchSingleCluster(t *testing.T, cluster redshifttypes.Cluster) resource.Resource {
	t.Helper()
	mock := &mockRedshiftClient{
		output: &redshift.DescribeClustersOutput{
			Clusters: []redshifttypes.Cluster{cluster},
		},
	}
	result, err := awsclient.FetchRedshiftClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedshiftClustersPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("FetchRedshiftClustersPage: expected 1 resource, got %d", len(result.Resources))
	}
	return result.Resources[0]
}

func assertStatus(t *testing.T, r resource.Resource, wantPhrase string) {
	t.Helper()
	if r.Fields["status"] != wantPhrase {
		t.Errorf("Resource.Fields[\"status\"] = %q, want %q", r.Fields["status"], wantPhrase)
	}
}

func assertFindings(t *testing.T, r resource.Resource, wantPhrases []string) {
	t.Helper()
	if len(wantPhrases) == 0 {
		if len(r.Findings) != 0 {
			phrases := make([]string, len(r.Findings))
			for i, f := range r.Findings {
				phrases[i] = f.Phrase
			}
			t.Errorf("Resource.Findings = %v, want nil/empty (healthy)", phrases)
		}
		return
	}
	if len(r.Findings) != len(wantPhrases) {
		phrases := make([]string, len(r.Findings))
		for i, f := range r.Findings {
			phrases[i] = f.Phrase
		}
		t.Errorf("Resource.Findings len=%d (%v), want len=%d (%v)", len(r.Findings), phrases, len(wantPhrases), wantPhrases)
		return
	}
	for i, want := range wantPhrases {
		if r.Findings[i].Phrase != want {
			t.Errorf("Resource.Findings[%d].Phrase = %q, want %q", i, r.Findings[i].Phrase, want)
		}
		if r.Findings[i].Source != "wave1" {
			t.Errorf("Resource.Findings[%d].Source = %q, want %q", i, r.Findings[i].Source, "wave1")
		}
	}
}

func assertClusterStatusField(t *testing.T, r resource.Resource, want string) {
	t.Helper()
	got := r.Fields["cluster_status"]
	if got != want {
		t.Errorf("Resource.Fields[\"cluster_status\"] = %q, want %q", got, want)
	}
}

func TestRedshift_Fetch_HealthyHasEmptyIssues(t *testing.T) {
	healthyIDs := []string{
		fixtures.AcmeWarehouseID,
		fixtures.AcmeReportingID,
	}
	for _, id := range healthyIDs {
		t.Run(id, func(t *testing.T) {
			r := fetchSingleCluster(t, redshiftFixtureByID(t, id))
			assertStatus(t, r, "")
			assertFindings(t, r, nil)
			assertClusterStatusField(t, r, "available")
		})
	}
}

func TestRedshift_Fetch_Healthy_Silent(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.AcmeWarehouseID))
	assertStatus(t, r, "")
	assertFindings(t, r, nil)
	assertClusterStatusField(t, r, "available")
}

func TestRedshift_Fetch_Transitional_Resizing(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftResizingID))
	assertStatus(t, r, "resizing")
	assertFindings(t, r, []string{"resizing"})
	assertClusterStatusField(t, r, "resizing")
}

func TestRedshift_Fetch_Transitional_Rebooting(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftRebootingID))
	assertStatus(t, r, "rebooting")
	assertFindings(t, r, []string{"rebooting"})
	assertClusterStatusField(t, r, "rebooting")
}

func TestRedshift_Fetch_Broken_IncompatibleNetwork(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftIncompatibleNetworkID))
	assertStatus(t, r, "subnet group cannot host the cluster")
	assertFindings(t, r, []string{"subnet group cannot host the cluster"})
	assertClusterStatusField(t, r, "incompatible-network")
}

func TestRedshift_Fetch_Broken_HardwareFailure(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftHardwareFailureID))
	assertStatus(t, r, "node hardware failed")
	assertFindings(t, r, []string{"node hardware failed"})
	assertClusterStatusField(t, r, "hardware-failure")
}

func TestRedshift_Fetch_Broken_StorageFull(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftStorageFullID))
	assertStatus(t, r, "out of storage")
	assertFindings(t, r, []string{"out of storage"})
	assertClusterStatusField(t, r, "storage-full")
}

func TestRedshift_Fetch_Availability_Unavailable(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftAvailUnavailableID))
	assertStatus(t, r, "unavailable")
	assertFindings(t, r, []string{"unavailable"})
	assertClusterStatusField(t, r, "available")
}

func TestRedshift_Fetch_Availability_Failed(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftAvailFailedID))
	assertStatus(t, r, "failed")
	assertFindings(t, r, []string{"failed"})
	assertClusterStatusField(t, r, "available")
}

func TestRedshift_Fetch_Availability_Maintenance(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftAvailMaintenanceID))
	assertStatus(t, r, "maintenance")
	assertFindings(t, r, []string{"maintenance"})
	assertClusterStatusField(t, r, "available")
}

func TestRedshift_Fetch_Availability_Modifying(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftAvailModifyingID))
	assertStatus(t, r, "modifying — availability affected")
	assertFindings(t, r, []string{"modifying — availability affected"})
	assertClusterStatusField(t, r, "available")
}

func TestRedshift_Fetch_PendingChange(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftPendingChangeID))
	assertStatus(t, r, "pending change queued")
	assertFindings(t, r, []string{"pending change queued"})
	assertClusterStatusField(t, r, "available")
}

func TestRedshift_Fetch_MaintenanceDeferred_Active(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftMaintenanceDeferredID))
	assertStatus(t, r, "maintenance deferred")
	assertFindings(t, r, []string{"maintenance deferred"})
	assertClusterStatusField(t, r, "available")
}

func TestRedshift_Fetch_MaintenanceDeferred_Expired(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftDeferralLapsedID))
	assertStatus(t, r, "")
	assertFindings(t, r, nil)
	assertClusterStatusField(t, r, "available")
}

func TestRedshift_Fetch_PubliclyAccessible(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftPubliclyAccessibleID))
	assertStatus(t, r, "public endpoint")
	assertFindings(t, r, []string{"public endpoint"})
	assertClusterStatusField(t, r, "available")
}

func TestRedshift_Fetch_Unencrypted(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftUnencryptedID))
	assertStatus(t, r, "unencrypted at rest")
	assertFindings(t, r, []string{"unencrypted at rest"})
	assertClusterStatusField(t, r, "available")
}

func TestRedshift_Fetch_MultiW1_PendingPlusPublicPlusUnencrypted(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.WarnRedshiftMultiID))
	assertStatus(t, r, "pending change queued (+2)")
	assertFindings(t, r, []string{
		"pending change queued",
		"public endpoint",
		"unencrypted at rest",
	})
}

func TestRedshift_Fetch_MultiW1_Two_Warnings_Suffix_Plus_1(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.WarnRedshiftTwoID))
	assertStatus(t, r, "public endpoint (+1)")
	assertFindings(t, r, []string{
		"public endpoint",
		"unencrypted at rest",
	})
}

func TestRedshift_Fetch_Broken_ClusterStatus_Beats_Availability_Modifying(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftBrokenWithWarningHiddenID))
	assertStatus(t, r, "out of storage")
	assertFindings(t, r, []string{"out of storage"})
	assertClusterStatusField(t, r, "storage-full")
	for _, f := range r.Findings {
		if strings.Contains(f.Phrase, "publicly") || strings.Contains(f.Phrase, "unencrypted") || strings.Contains(f.Phrase, "modifying") {
			t.Errorf("Findings contains warning phrase %q when Broken should suppress all Warnings", f.Phrase)
		}
	}
}

func TestRedshift_Fetch_Broken_Availability_Beats_Warning_PubliclyAccessible(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftAvailUnavailableWithWarningHiddenID))
	assertStatus(t, r, "unavailable")
	assertFindings(t, r, []string{"unavailable"})
	for _, f := range r.Findings {
		if strings.Contains(f.Phrase, "publicly") || strings.Contains(f.Phrase, "unencrypted") {
			t.Errorf("Findings contains warning phrase %q when Broken should suppress all Warnings", f.Phrase)
		}
	}
}

func TestRedshift_Fetch_IgnoresCloudWatchPercentageDiskSpaceUsed(t *testing.T) {
	for _, cluster := range fixtures.NewRedshiftFixtures().Clusters {
		r := fetchSingleCluster(t, cluster)
		id := ""
		if cluster.ClusterIdentifier != nil {
			id = *cluster.ClusterIdentifier
		}
		forbidden := []string{"PercentageDiskSpaceUsed", "disk_space", "disk space", "percentage disk"}
		for _, forbiddenPhrase := range forbidden {
			if strings.Contains(strings.ToLower(r.Fields["status"]), strings.ToLower(forbiddenPhrase)) {
				t.Errorf("cluster %s: Fields[status] contains CloudWatch phrase %q (out of scope)", id, forbiddenPhrase)
			}
			for _, finding := range r.Findings {
				if strings.Contains(strings.ToLower(finding.Phrase), strings.ToLower(forbiddenPhrase)) {
					t.Errorf("cluster %s: Findings contains CloudWatch phrase %q (out of scope)", id, forbiddenPhrase)
				}
			}
		}
	}
}

func TestRedshift_Fetch_IgnoresCloudWatchHealthStatus(t *testing.T) {
	for _, cluster := range fixtures.NewRedshiftFixtures().Clusters {
		r := fetchSingleCluster(t, cluster)
		id := ""
		if cluster.ClusterIdentifier != nil {
			id = *cluster.ClusterIdentifier
		}
		for _, phrase := range []string{"HealthStatus", "health_status"} {
			if strings.Contains(r.Fields["status"], phrase) {
				t.Errorf("cluster %s: Fields[status] contains CloudWatch phrase %q (out of scope)", id, phrase)
			}
			for _, finding := range r.Findings {
				if strings.Contains(finding.Phrase, phrase) {
					t.Errorf("cluster %s: Findings contains CloudWatch phrase %q (out of scope)", id, phrase)
				}
			}
		}
	}
}

func TestRedshift_Fetch_APIError(t *testing.T) {
	mock := &mockRedshiftClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}
	_, err := awsclient.FetchRedshiftClustersPage(context.Background(), mock, "")
	if err == nil {
		t.Error("expected an error from FetchRedshiftClustersPage, got nil")
	}
}

func TestRedshift_Fetch_EmptyResponse(t *testing.T) {
	mock := &mockRedshiftClient{
		output: &redshift.DescribeClustersOutput{Clusters: []redshifttypes.Cluster{}},
	}
	result, err := awsclient.FetchRedshiftClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

func TestRedshift_Fetch_Pagination(t *testing.T) {
	marker := "next-page-token"
	mock := &mockRedshiftClient{
		output: &redshift.DescribeClustersOutput{
			Clusters: []redshifttypes.Cluster{
				{ClusterIdentifier: aws.String("cluster-pg"), ClusterStatus: aws.String("available")},
			},
			Marker: &marker,
		},
	}
	result, err := awsclient.FetchRedshiftClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true when Marker is set")
	}
	if result.Pagination.NextToken != marker {
		t.Errorf("NextToken = %q, want %q", result.Pagination.NextToken, marker)
	}
}

func TestRedshift_Fetch_RequiredFieldsPresent(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.AcmeWarehouseID))
	required := []string{
		"cluster_id", "status", "cluster_status", "cluster_availability_status",
		"publicly_accessible", "encrypted", "node_type", "num_nodes",
		"db_name", "endpoint",
	}
	for _, key := range required {
		if _, ok := r.Fields[key]; !ok {
			t.Errorf("Resource.Fields missing required key %q", key)
		}
	}
	if r.Fields["cluster_id"] != fixtures.AcmeWarehouseID {
		t.Errorf("Fields[\"cluster_id\"] = %q, want %q", r.Fields["cluster_id"], fixtures.AcmeWarehouseID)
	}
	if r.Fields["publicly_accessible"] != "false" {
		t.Errorf("Fields[\"publicly_accessible\"] = %q, want %q", r.Fields["publicly_accessible"], "false")
	}
	if r.Fields["encrypted"] != "true" {
		t.Errorf("Fields[\"encrypted\"] = %q, want %q", r.Fields["encrypted"], "true")
	}
}

func TestRedshift_Fetch_PubliclyAccessibleFieldTrue(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftPubliclyAccessibleID))
	if r.Fields["publicly_accessible"] != "true" {
		t.Errorf("Fields[\"publicly_accessible\"] = %q, want %q", r.Fields["publicly_accessible"], "true")
	}
}

func TestRedshift_Fetch_EncryptedFieldFalse(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftUnencryptedID))
	if r.Fields["encrypted"] != "false" {
		t.Errorf("Fields[\"encrypted\"] = %q, want %q", r.Fields["encrypted"], "false")
	}
}

// Related checkers assert RawStruct as redshifttypes.Cluster.
func TestRedshift_Fetch_RawStructIsCluster(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.AcmeWarehouseID))
	if _, ok := r.RawStruct.(redshifttypes.Cluster); !ok {
		t.Errorf("RawStruct type = %T, want redshifttypes.Cluster", r.RawStruct)
	}
}
