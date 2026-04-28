// aws_redshift_test.go — Unit tests for the Redshift fetcher.
//
// Tests assert on the contract surface of FetchRedshiftClustersPage:
//   - Resource.Status  always "" (PR-03e: phrases moved to Findings and Fields["status"])
//   - Resource.Findings (slice of domain.Finding in §4 precedence order, Source="wave1")
//   - Resource.Fields["status"]          (§4 display phrase — drives list-view color)
//   - Resource.Fields["cluster_status"]  (raw ClusterStatus value — read by Color func)
//
// Wave 1 signals only (Wave 2 = None per spec).
// Anti-tests verify no CloudWatch-derived phrases surface (§3.3 out of scope).
// Severity-precedence tests verify Broken beats Warning (U8).
// Multi-finding tests verify rule-7 "(+N)" suffix and Findings ordering.
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
	"github.com/k2m30/a9s/v3/internal/domain"
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

// assertStatus asserts Resource.Status is always empty (PR-03e) and
// Resource.Fields["status"] carries the display phrase.
func assertStatus(t *testing.T, r resource.Resource, wantPhrase string) {
	t.Helper()
	if r.Status != "" {
		t.Errorf("Resource.Status = %q, want %q (PR-03e: fetcher must not write Status)", r.Status, "")
	}
	if r.Fields["status"] != wantPhrase {
		t.Errorf("Resource.Fields[\"status\"] = %q, want %q", r.Fields["status"], wantPhrase)
	}
}

// assertFindings asserts Resource.Findings matches the expected phrase list.
// Each element of wantPhrases must match the corresponding Finding.Phrase in
// Findings order. Nil/empty wantPhrases means healthy — no findings expected.
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

// assertFindingCode asserts that Findings[i].Code matches the expected code.
func assertFindingCode(t *testing.T, r resource.Resource, i int, want domain.FindingCode) {
	t.Helper()
	if i >= len(r.Findings) {
		t.Errorf("Findings[%d] out of range (len=%d)", i, len(r.Findings))
		return
	}
	if r.Findings[i].Code != want {
		t.Errorf("Findings[%d].Code = %q, want %q", i, r.Findings[i].Code, want)
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
			assertFindings(t, r, nil)
			assertClusterStatusField(t, r, "available")
		})
	}
}

// TestRedshift_Fetch_Healthy_Silent verifies the full silence contract on acme-warehouse:
// Status=="", Issues==nil, no glyph prefix, Fields["status"]==""
func TestRedshift_Fetch_Healthy_Silent(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.AcmeWarehouseID))
	assertStatus(t, r, "")
	assertFindings(t, r, nil)
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
	assertFindings(t, r, []string{"resizing"})
	assertClusterStatusField(t, r, "resizing")
}

// TestRedshift_Fetch_Transitional_Rebooting asserts ClusterStatus=rebooting.
func TestRedshift_Fetch_Transitional_Rebooting(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftRebootingID))
	assertStatus(t, r, "rebooting")
	assertFindings(t, r, []string{"rebooting"})
	assertClusterStatusField(t, r, "rebooting")
}

// ---------------------------------------------------------------------------
// Wave 1 — Broken (ClusterStatus)
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_Broken_IncompatibleNetwork asserts ClusterStatus=incompatible-network.
func TestRedshift_Fetch_Broken_IncompatibleNetwork(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftIncompatibleNetworkID))
	assertStatus(t, r, "broken: incompatible-network")
	assertFindings(t, r, []string{"broken: incompatible-network"})
	assertClusterStatusField(t, r, "incompatible-network")
}

// TestRedshift_Fetch_Broken_HardwareFailure asserts ClusterStatus=hardware-failure.
func TestRedshift_Fetch_Broken_HardwareFailure(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftHardwareFailureID))
	assertStatus(t, r, "broken: hardware-failure")
	assertFindings(t, r, []string{"broken: hardware-failure"})
	assertClusterStatusField(t, r, "hardware-failure")
}

// TestRedshift_Fetch_Broken_StorageFull asserts ClusterStatus=storage-full.
func TestRedshift_Fetch_Broken_StorageFull(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftStorageFullID))
	assertStatus(t, r, "broken: storage-full")
	assertFindings(t, r, []string{"broken: storage-full"})
	assertClusterStatusField(t, r, "storage-full")
}

// ---------------------------------------------------------------------------
// Wave 1 — ClusterAvailabilityStatus (Broken)
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_Availability_Unavailable asserts ClusterAvailabilityStatus=Unavailable.
func TestRedshift_Fetch_Availability_Unavailable(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftAvailUnavailableID))
	assertStatus(t, r, "unavailable")
	assertFindings(t, r, []string{"unavailable"})
	assertClusterStatusField(t, r, "available")
}

// TestRedshift_Fetch_Availability_Failed asserts ClusterAvailabilityStatus=Failed.
func TestRedshift_Fetch_Availability_Failed(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftAvailFailedID))
	assertStatus(t, r, "failed")
	assertFindings(t, r, []string{"failed"})
	assertClusterStatusField(t, r, "available")
}

// ---------------------------------------------------------------------------
// Wave 1 — ClusterAvailabilityStatus (Warning)
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_Availability_Maintenance asserts ClusterAvailabilityStatus=Maintenance.
func TestRedshift_Fetch_Availability_Maintenance(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftAvailMaintenanceID))
	assertStatus(t, r, "maintenance")
	assertFindings(t, r, []string{"maintenance"})
	assertClusterStatusField(t, r, "available")
}

// TestRedshift_Fetch_Availability_Modifying asserts ClusterAvailabilityStatus=Modifying.
func TestRedshift_Fetch_Availability_Modifying(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftAvailModifyingID))
	assertStatus(t, r, "modifying")
	assertFindings(t, r, []string{"modifying"})
	assertClusterStatusField(t, r, "available")
}

// ---------------------------------------------------------------------------
// Wave 1 — PendingModifiedValues / DeferredMaintenanceWindows
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_PendingChange asserts PendingModifiedValues non-empty.
func TestRedshift_Fetch_PendingChange(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftPendingChangeID))
	assertStatus(t, r, "pending change queued")
	assertFindings(t, r, []string{"pending change queued"})
	assertClusterStatusField(t, r, "available")
}

// TestRedshift_Fetch_MaintenanceDeferred_Active asserts active DeferredMaintenanceWindow triggers Warning.
func TestRedshift_Fetch_MaintenanceDeferred_Active(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftMaintenanceDeferredID))
	assertStatus(t, r, "maintenance deferred")
	assertFindings(t, r, []string{"maintenance deferred"})
	assertClusterStatusField(t, r, "available")
}

// TestRedshift_Fetch_MaintenanceDeferred_Expired asserts expired DeferredMaintenanceWindow is silent.
// The fixture has DeferMaintenanceEndTime in the past — must NOT trigger a finding.
func TestRedshift_Fetch_MaintenanceDeferred_Expired(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftMaintenanceDeferredExpiredID))
	assertStatus(t, r, "")
	assertFindings(t, r, nil)
	assertClusterStatusField(t, r, "available")
}

// ---------------------------------------------------------------------------
// Wave 1 — PubliclyAccessible / Unencrypted
// ---------------------------------------------------------------------------

// TestRedshift_Fetch_PubliclyAccessible asserts PubliclyAccessible=true → warning phrase.
func TestRedshift_Fetch_PubliclyAccessible(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftPubliclyAccessibleID))
	assertStatus(t, r, "publicly accessible")
	assertFindings(t, r, []string{"publicly accessible"})
	assertClusterStatusField(t, r, "available")
}

// TestRedshift_Fetch_Unencrypted asserts Encrypted=false → warning phrase.
func TestRedshift_Fetch_Unencrypted(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftUnencryptedID))
	assertStatus(t, r, "unencrypted at rest")
	assertFindings(t, r, []string{"unencrypted at rest"})
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
	assertFindings(t, r, []string{
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
	assertFindings(t, r, []string{
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
	assertFindings(t, r, []string{"broken: storage-full"})
	assertClusterStatusField(t, r, "storage-full")
	// The Findings slice must NOT contain any Warning phrases.
	for _, f := range r.Findings {
		if strings.Contains(f.Phrase, "publicly") || strings.Contains(f.Phrase, "unencrypted") || strings.Contains(f.Phrase, "modifying") {
			t.Errorf("Findings contains warning phrase %q when Broken should suppress all Warnings", f.Phrase)
		}
	}
}

// TestRedshift_Fetch_Broken_Availability_Beats_Warning_PubliclyAccessible verifies
// that Broken from ClusterAvailabilityStatus=Unavailable suppresses the Warning
// from PubliclyAccessible=true.
func TestRedshift_Fetch_Broken_Availability_Beats_Warning_PubliclyAccessible(t *testing.T) {
	r := fetchSingleCluster(t, redshiftFixtureByID(t, fixtures.RedshiftAvailUnavailableWithWarningHiddenID))
	assertStatus(t, r, "unavailable")
	assertFindings(t, r, []string{"unavailable"})
	// No Warning phrases in Findings.
	for _, f := range r.Findings {
		if strings.Contains(f.Phrase, "publicly") || strings.Contains(f.Phrase, "unencrypted") {
			t.Errorf("Findings contains warning phrase %q when Broken should suppress all Warnings", f.Phrase)
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
