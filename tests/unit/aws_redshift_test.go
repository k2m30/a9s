// aws_redshift_test.go — Unit tests for the Redshift fetcher.
//
// Tests assert on the contract surface of FetchRedshiftClustersPage:
//   - Resource.Status  (§4 phrase, optionally suffixed with "(+N)")
//   - Resource.Issues  (slice of §4 phrases in §4 precedence order)
//   - Resource.Fields["status"]          (mirrors Resource.Status)
//   - Resource.Fields["cluster_status"]  (raw ClusterStatus value — read by Color func)
//
// Wave 1 signals only (Wave 2 = None per spec).
// Anti-tests verify no CloudWatch-derived phrases surface (§3.3 out of scope).
// Severity-precedence tests verify Broken beats Warning (U8).
// Multi-finding tests verify rule-7 "(+N)" suffix and Issues ordering.
//
// Phase 6a fixture source: internal/demo/fixtures.NewRedshiftFixtures().
// Phase 7 coder output is not yet complete; all signal tests are expected to
// FAIL against the current redshift.go until phase 7 delivers the rewritten
// fetcher. See "Expected failures" section in the task handoff notes.
package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Helpers
// The mockRedshiftClient type is declared in mocks_services_test.go (package unit).
// ---------------------------------------------------------------------------

// redshiftFixtureByID returns the redshifttypes.Cluster from the canonical
// fixture set whose ClusterIdentifier matches id.
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

// fetchSingleCluster wraps one redshifttypes.Cluster in a mock and calls
// FetchRedshiftClustersPage, returning the single resource.
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

// assertStatus asserts Resource.Status and Resource.Fields["status"].
func assertStatus(t *testing.T, r resource.Resource, want string) {
	t.Helper()
	if r.Status != want {
		t.Errorf("Resource.Status = %q, want %q", r.Status, want)
	}
	if r.Fields["status"] != want {
		t.Errorf("Resource.Fields[\"status\"] = %q, want %q", r.Fields["status"], want)
	}
}

// assertIssues asserts Resource.Issues deep-equals want (nil vs empty-slice
// treated identically when want is nil).
func assertIssues(t *testing.T, r resource.Resource, want []string) {
	t.Helper()
	if want == nil {
		if len(r.Issues) != 0 {
			t.Errorf("Resource.Issues = %v, want nil/empty", r.Issues)
		}
		return
	}
	if len(r.Issues) != len(want) {
		t.Errorf("Resource.Issues = %v (len=%d), want %v (len=%d)", r.Issues, len(r.Issues), want, len(want))
		return
	}
	for i, w := range want {
		if r.Issues[i] != w {
			t.Errorf("Resource.Issues[%d] = %q, want %q", i, r.Issues[i], w)
		}
	}
}

// assertClusterStatusField asserts Resource.Fields["cluster_status"] == want.
func assertClusterStatusField(t *testing.T, r resource.Resource, want string) {
	t.Helper()
	got := r.Fields["cluster_status"]
	if got != want {
		t.Errorf("Resource.Fields[\"cluster_status\"] = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Silence test (healthy clusters → blank status, nil issues)
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_HealthyHasEmptyIssues verifies that healthy graph-root clusters
// (acme-warehouse, acme-reporting) produce empty Status and nil Issues.
func TestRedshift_Fetch_HealthyHasEmptyIssues(t *testing.T) {
	healthyIDs := []string{
		fixtures.AcmeWarehouseID,
		fixtures.AcmeReportingID,
	}
	for _, id := range healthyIDs {
		t.Run(id, func(t *testing.T) {
			r := fetchSingleCluster(t, redshiftFixtureByID(t, id))
			assertStatus(t, r, "")
			assertIssues(t, r, nil)
			assertClusterStatusField(t, r, "available")
		})
	}
}

// TestRedshift_Fetch_Healthy_Silent verifies the full silence contract on acme-warehouse:
// Status=="", Issues==nil, no glyph prefix, Fields["status"]==""
func TestRedshift_Fetch_Healthy_Silent(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.AcmeWarehouseID))
	assertStatus(t, r, "")
	assertIssues(t, r, nil)
	assertClusterStatusField(t, r, "available")
	// No jargon in status field
	for _, bad := range []string{"OK", "ACTIVE", "available", "healthy", "-", "Available"} {
		if r.Status == bad {
			t.Errorf("Resource.Status = %q, must be blank for healthy clusters (not %q)", r.Status, bad)
		}
	}
}

// ---------------------------------------------------------------------------
// Wave 1 — Transitional (Warning bucket)
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_Transitional_Resizing asserts that ClusterStatus=resizing
// yields Status=="resizing", Issues==["resizing"].
func TestRedshift_Fetch_Transitional_Resizing(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftResizingID))
	assertStatus(t, r, "resizing")
	assertIssues(t, r, []string{"resizing"})
	assertClusterStatusField(t, r, "resizing")
}

// TestRedshift_Fetch_Transitional_Rebooting asserts ClusterStatus=rebooting.
func TestRedshift_Fetch_Transitional_Rebooting(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftRebootingID))
	assertStatus(t, r, "rebooting")
	assertIssues(t, r, []string{"rebooting"})
	assertClusterStatusField(t, r, "rebooting")
}

// ---------------------------------------------------------------------------
// Wave 1 — Broken (ClusterStatus)
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_Broken_IncompatibleNetwork asserts ClusterStatus=incompatible-network.
func TestRedshift_Fetch_Broken_IncompatibleNetwork(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftIncompatibleNetworkID))
	assertStatus(t, r, "broken: incompatible-network")
	assertIssues(t, r, []string{"broken: incompatible-network"})
	assertClusterStatusField(t, r, "incompatible-network")
}

// TestRedshift_Fetch_Broken_HardwareFailure asserts ClusterStatus=hardware-failure.
func TestRedshift_Fetch_Broken_HardwareFailure(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftHardwareFailureID))
	assertStatus(t, r, "broken: hardware-failure")
	assertIssues(t, r, []string{"broken: hardware-failure"})
	assertClusterStatusField(t, r, "hardware-failure")
}

// TestRedshift_Fetch_Broken_StorageFull asserts ClusterStatus=storage-full.
func TestRedshift_Fetch_Broken_StorageFull(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftStorageFullID))
	assertStatus(t, r, "broken: storage-full")
	assertIssues(t, r, []string{"broken: storage-full"})
	assertClusterStatusField(t, r, "storage-full")
}

// ---------------------------------------------------------------------------
// Wave 1 — ClusterAvailabilityStatus (Broken)
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_Availability_Unavailable asserts ClusterAvailabilityStatus=Unavailable.
func TestRedshift_Fetch_Availability_Unavailable(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftAvailUnavailableID))
	assertStatus(t, r, "unavailable")
	assertIssues(t, r, []string{"unavailable"})
	assertClusterStatusField(t, r, "available")
}

// TestRedshift_Fetch_Availability_Failed asserts ClusterAvailabilityStatus=Failed.
func TestRedshift_Fetch_Availability_Failed(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftAvailFailedID))
	assertStatus(t, r, "failed")
	assertIssues(t, r, []string{"failed"})
	assertClusterStatusField(t, r, "available")
}

// ---------------------------------------------------------------------------
// Wave 1 — ClusterAvailabilityStatus (Warning)
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_Availability_Maintenance asserts ClusterAvailabilityStatus=Maintenance.
func TestRedshift_Fetch_Availability_Maintenance(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftAvailMaintenanceID))
	assertStatus(t, r, "maintenance")
	assertIssues(t, r, []string{"maintenance"})
	assertClusterStatusField(t, r, "available")
}

// TestRedshift_Fetch_Availability_Modifying asserts ClusterAvailabilityStatus=Modifying.
func TestRedshift_Fetch_Availability_Modifying(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftAvailModifyingID))
	assertStatus(t, r, "modifying")
	assertIssues(t, r, []string{"modifying"})
	assertClusterStatusField(t, r, "available")
}

// ---------------------------------------------------------------------------
// Wave 1 — PendingModifiedValues / DeferredMaintenanceWindows
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_PendingChange asserts PendingModifiedValues non-empty.
func TestRedshift_Fetch_PendingChange(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftPendingChangeID))
	assertStatus(t, r, "pending change queued")
	assertIssues(t, r, []string{"pending change queued"})
	assertClusterStatusField(t, r, "available")
}

// TestRedshift_Fetch_MaintenanceDeferred_Active asserts active DeferredMaintenanceWindow triggers Warning.
func TestRedshift_Fetch_MaintenanceDeferred_Active(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftMaintenanceDeferredID))
	assertStatus(t, r, "maintenance deferred")
	assertIssues(t, r, []string{"maintenance deferred"})
	assertClusterStatusField(t, r, "available")
}

// TestRedshift_Fetch_MaintenanceDeferred_Expired asserts expired DeferredMaintenanceWindow is silent.
// The fixture has DeferMaintenanceEndTime in the past — must NOT trigger a finding.
func TestRedshift_Fetch_MaintenanceDeferred_Expired(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftMaintenanceDeferredExpiredID))
	assertStatus(t, r, "")
	assertIssues(t, r, nil)
	assertClusterStatusField(t, r, "available")
}

// ---------------------------------------------------------------------------
// Wave 1 — PubliclyAccessible / Unencrypted
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_PubliclyAccessible asserts PubliclyAccessible=true → warning phrase.
func TestRedshift_Fetch_PubliclyAccessible(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftPubliclyAccessibleID))
	assertStatus(t, r, "publicly accessible")
	assertIssues(t, r, []string{"publicly accessible"})
	assertClusterStatusField(t, r, "available")
}

// TestRedshift_Fetch_Unencrypted asserts Encrypted=false → warning phrase.
func TestRedshift_Fetch_Unencrypted(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftUnencryptedID))
	assertStatus(t, r, "unencrypted at rest")
	assertIssues(t, r, []string{"unencrypted at rest"})
	assertClusterStatusField(t, r, "available")
}

// ---------------------------------------------------------------------------
// Multi-finding rule-7 cases
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_MultiW1_PendingPlusPublicPlusUnencrypted verifies the
// "(+2)" suffix and Issues ordering for 3 coexisting §3.1 warnings.
// §4 precedence: pending-change (row 9) > publicly-accessible (row 11) > unencrypted (row 12).
func TestRedshift_Fetch_MultiW1_PendingPlusPublicPlusUnencrypted(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.WarnRedshiftMultiID))
	assertStatus(t, r, "pending change queued (+2)")
	assertIssues(t, r, []string{
		"pending change queued",
		"publicly accessible",
		"unencrypted at rest",
	})
}

// TestRedshift_Fetch_MultiW1_Two_Warnings_Suffix_Plus_1 verifies the "(+1)" suffix
// for 2 coexisting warnings.
func TestRedshift_Fetch_MultiW1_Two_Warnings_Suffix_Plus_1(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.WarnRedshiftTwoID))
	assertStatus(t, r, "publicly accessible (+1)")
	assertIssues(t, r, []string{
		"publicly accessible",
		"unencrypted at rest",
	})
}

// ---------------------------------------------------------------------------
// Severity precedence (U8)
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_Broken_ClusterStatus_Beats_Availability_Modifying verifies
// that Broken (ClusterStatus=storage-full) suppresses the Warning from
// ClusterAvailabilityStatus=Modifying — only the Broken phrase surfaces.
func TestRedshift_Fetch_Broken_ClusterStatus_Beats_Availability_Modifying(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftBrokenWithWarningHiddenID))
	assertStatus(t, r, "broken: storage-full")
	assertIssues(t, r, []string{"broken: storage-full"})
	assertClusterStatusField(t, r, "storage-full")
	// The Issues slice must NOT contain any Warning phrases.
	for _, issue := range r.Issues {
		if strings.Contains(issue, "publicly") || strings.Contains(issue, "unencrypted") || strings.Contains(issue, "modifying") {
			t.Errorf("Issues contains warning phrase %q when Broken should suppress all Warnings", issue)
		}
	}
}

// TestRedshift_Fetch_Broken_Availability_Beats_Warning_PubliclyAccessible verifies
// that Broken from ClusterAvailabilityStatus=Unavailable suppresses the Warning
// from PubliclyAccessible=true.
func TestRedshift_Fetch_Broken_Availability_Beats_Warning_PubliclyAccessible(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftAvailUnavailableWithWarningHiddenID))
	assertStatus(t, r, "unavailable")
	assertIssues(t, r, []string{"unavailable"})
	// No Warning phrases in Issues.
	for _, issue := range r.Issues {
		if strings.Contains(issue, "publicly") || strings.Contains(issue, "unencrypted") {
			t.Errorf("Issues contains warning phrase %q when Broken should suppress all Warnings", issue)
		}
	}
}

// ---------------------------------------------------------------------------
// Anti-tests: §3.3 Wave 3 OUT OF SCOPE
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_IgnoresCloudWatchPercentageDiskSpaceUsed verifies that
// no PercentageDiskSpaceUsed-derived phrase appears in Status or Issues for
// any fixture. The fetcher must make zero CloudWatch calls.
func TestRedshift_Fetch_IgnoresCloudWatchPercentageDiskSpaceUsed(t *testing.T) {
	for _, cluster := range fixtures.NewRedshiftFixtures().Clusters {
		r := fetchSingleCluster(t, cluster)
		id := ""
		if cluster.ClusterIdentifier != nil {
			id = *cluster.ClusterIdentifier
		}
		forbidden := []string{"PercentageDiskSpaceUsed", "disk_space", "disk space", "percentage disk"}
		for _, f := range forbidden {
			if strings.Contains(strings.ToLower(r.Status), strings.ToLower(f)) {
				t.Errorf("cluster %s: Status contains CloudWatch phrase %q (out of scope)", id, f)
			}
			for _, issue := range r.Issues {
				if strings.Contains(strings.ToLower(issue), strings.ToLower(f)) {
					t.Errorf("cluster %s: Issues contains CloudWatch phrase %q (out of scope)", id, f)
				}
			}
		}
	}
}

// TestRedshift_Fetch_IgnoresCloudWatchHealthStatus verifies that no HealthStatus-derived
// phrase appears in Status or Issues for any fixture.
func TestRedshift_Fetch_IgnoresCloudWatchHealthStatus(t *testing.T) {
	for _, cluster := range fixtures.NewRedshiftFixtures().Clusters {
		r := fetchSingleCluster(t, cluster)
		id := ""
		if cluster.ClusterIdentifier != nil {
			id = *cluster.ClusterIdentifier
		}
		// "health" as a substring would catch HealthStatus, HealthState, etc.
		// Note: "unhealthy" is also out of scope; none of these come from CW.
		for _, phrase := range []string{"HealthStatus", "health_status"} {
			if strings.Contains(r.Status, phrase) {
				t.Errorf("cluster %s: Status contains CloudWatch phrase %q (out of scope)", id, phrase)
			}
			for _, issue := range r.Issues {
				if strings.Contains(issue, phrase) {
					t.Errorf("cluster %s: Issues contains CloudWatch phrase %q (out of scope)", id, phrase)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// API error handling
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_APIError verifies that a mock API error is propagated.
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

// ---------------------------------------------------------------------------
// Pagination / empty response
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_EmptyResponse verifies a zero-cluster response.
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

// TestRedshift_Fetch_Pagination verifies that a non-empty Marker signals truncation.
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

// ---------------------------------------------------------------------------
// Fields contract — required keys must be present
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_RequiredFieldsPresent verifies all required field keys are
// populated on a healthy cluster.
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

// TestRedshift_Fetch_PubliclyAccessibleFieldTrue verifies Fields["publicly_accessible"]=="true".
func TestRedshift_Fetch_PubliclyAccessibleFieldTrue(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftPubliclyAccessibleID))
	if r.Fields["publicly_accessible"] != "true" {
		t.Errorf("Fields[\"publicly_accessible\"] = %q, want %q", r.Fields["publicly_accessible"], "true")
	}
}

// TestRedshift_Fetch_EncryptedFieldFalse verifies Fields["encrypted"]=="false" for unencrypted cluster.
func TestRedshift_Fetch_EncryptedFieldFalse(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftUnencryptedID))
	if r.Fields["encrypted"] != "false" {
		t.Errorf("Fields[\"encrypted\"] = %q, want %q", r.Fields["encrypted"], "false")
	}
}

// ---------------------------------------------------------------------------
// RawStruct contract
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_RawStructIsCluster verifies that Resource.RawStruct is
// a redshifttypes.Cluster (required by related checkers that call assertStruct).
func TestRedshift_Fetch_RawStructIsCluster(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.AcmeWarehouseID))
	if _, ok := r.RawStruct.(redshifttypes.Cluster); !ok {
		t.Errorf("RawStruct type = %T, want redshifttypes.Cluster", r.RawStruct)
	}
}
