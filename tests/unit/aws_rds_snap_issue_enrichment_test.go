package unit

// aws_rds_snap_issue_enrichment_test.go — Cross-ref enricher tests for rds-snap.
//
// Spec: docs/resources/rds-snap.md §3.1 (orphan + past-retention signals) +
//       impl-plan §1.1 (enricher test cases) + §3.3 (enricher contract).
//
// The enricher is registered in IssueEnricherRegistry["rds-snap"]. Tests drive
// it by retrieving the registered function, NOT by importing the production file
// directly. This ensures we are testing the wired function, not an unregistered one.
//
// Enricher contract (§4.2 + §3.3):
//   - Zero API calls — pure cross-ref against the dbi ResourceCache.
//   - IssueAppends[id] = []string of Wave-1 phrases to append to Resource.Issues.
//   - FieldUpdates[id]["status"] = merged §4 phrase (BumpFindingSuffix if needed).
//   - Findings = empty map (Wave 2 = None; orphan/past-retention are Wave-1 phrases
//     routed via IssueAppends, NOT via EnrichmentFinding).
//   - nil error always.
//   - nil clients are safe (no API calls made).

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// rdsSnapEnricher retrieves the registered rds-snap IssueEnricherFunc.
func rdsSnapEnricher(t *testing.T) awsclient.IssueEnricherFunc {
	t.Helper()
	e, ok := awsclient.IssueEnricherRegistry["rds-snap"]
	if !ok {
		t.Fatal("IssueEnricherRegistry[\"rds-snap\"] not registered")
	}
	if e.Fn == nil {
		t.Fatal("IssueEnricherRegistry[\"rds-snap\"].Fn is nil")
	}
	return e.Fn
}

// snapResource builds a resource.Resource from a DBSnapshot for enricher input.
// The fetcher normally produces this; we replicate the minimal fields needed.
func snapResource(snap rdstypes.DBSnapshot) resource.Resource {
	id := ""
	if snap.DBSnapshotIdentifier != nil {
		id = *snap.DBSnapshotIdentifier
	}
	status := ""
	// Apply fetcher-level status computation: for the enricher tests we
	// only need the status the fetcher would have set. We keep it simple here
	// since the enricher operates on Resources produced by FetchRDSSnapshotsPage.
	r := resource.Resource{
		ID:        id,
		Name:      id,
		Status:    status,
		Fields:    map[string]string{},
		RawStruct: snap,
	}
	return r
}

// snapResourceWithStatus builds a resource.Resource with a pre-set Status
// (simulating what the fetcher emits before enrichment).
func snapResourceWithStatus(snap rdstypes.DBSnapshot, preStatus string) resource.Resource {
	r := snapResource(snap)
	r.Status = preStatus
	if preStatus != "" {
		r.Issues = []string{preStatus}
	}
	return r
}

// dbiCacheFromFixtures builds a ResourceCache with the "dbi" key populated
// from the canonical DBI fixtures. Used for tests that need a real dbi list.
func dbiCacheFromFixtures(t *testing.T) resource.ResourceCache {
	t.Helper()
	fix := fixtures.NewDBIFixtures()
	res := make([]resource.Resource, 0, len(fix.Instances))
	for _, db := range fix.Instances {
		id := ""
		if db.DBInstanceIdentifier != nil {
			id = *db.DBInstanceIdentifier
		}
		res = append(res, resource.Resource{
			ID:        id,
			Name:      id,
			RawStruct: db,
		})
	}
	return resource.ResourceCache{"dbi": resource.ResourceCacheEntry{Resources: res}}
}

// dbiCacheWith builds a ResourceCache with the "dbi" key populated from the
// provided slice of DBInstance structs.
func dbiCacheWith(instances []rdstypes.DBInstance) resource.ResourceCache {
	res := make([]resource.Resource, 0, len(instances))
	for _, db := range instances {
		id := ""
		if db.DBInstanceIdentifier != nil {
			id = *db.DBInstanceIdentifier
		}
		res = append(res, resource.Resource{
			ID:        id,
			Name:      id,
			RawStruct: db,
		})
	}
	return resource.ResourceCache{"dbi": resource.ResourceCacheEntry{Resources: res}}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestRDSSnap_Enricher_Orphan_DbiMissingFromCache verifies that when the dbi
// cache is loaded but does NOT contain the snapshot's parent instance,
// IssueAppends carries "orphan: source DB deleted" and FieldUpdates sets
// the status phrase.
func TestRDSSnap_Enricher_Orphan_DbiMissingFromCache(t *testing.T) {
	enricher := rdsSnapEnricher(t)

	// Snapshot whose parent "deleted-legacy-db" is absent from the dbi cache.
	snap := rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.WarnRDSSnapOrphanID),
		DBInstanceIdentifier: aws.String("deleted-legacy-db"),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
		SnapshotType:         aws.String("manual"),
	}
	// The dbi cache exists but contains only prod-dbi-1 (not "deleted-legacy-db").
	cache := dbiCacheWith([]rdstypes.DBInstance{
		{DBInstanceIdentifier: aws.String("prod-dbi-1"), DBInstanceStatus: aws.String("available")},
	})
	resources := []resource.Resource{snapResource(snap)}

	result, err := enricher(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("enricher returned unexpected error: %v", err)
	}

	snapID := fixtures.WarnRDSSnapOrphanID
	appends, hasAppends := result.IssueAppends[snapID]
	if !hasAppends || len(appends) == 0 {
		t.Fatalf("IssueAppends[%q] = %v, want [\"orphan: source DB deleted\"]", snapID, appends)
	}
	found := false
	for _, phrase := range appends {
		if phrase == "orphan: source DB deleted" {
			found = true
		}
	}
	if !found {
		t.Errorf("IssueAppends[%q] = %v, want to contain %q", snapID, appends, "orphan: source DB deleted")
	}
	if result.FieldUpdates == nil || result.FieldUpdates[snapID] == nil {
		t.Fatalf("FieldUpdates[%q] is nil, want status phrase set", snapID)
	}
	statusPhrase := result.FieldUpdates[snapID]["status"]
	if !strings.Contains(statusPhrase, "orphan") {
		t.Errorf("FieldUpdates[%q][status] = %q, want to contain \"orphan\"", snapID, statusPhrase)
	}
}

// TestRDSSnap_Enricher_AutomatedPastRetention_BasicCase verifies that when the
// parent dbi has BackupRetentionPeriod=7 and the snapshot is automated and
// 30 days old, IssueAppends carries "automated, 23d past retention".
func TestRDSSnap_Enricher_AutomatedPastRetention_BasicCase(t *testing.T) {
	enricher := rdsSnapEnricher(t)

	// "prod-dbi-retention-parent" is the value of fixtures.WarnDbiPastRetentionParentID
	// (defined in internal/demo/fixtures/dbi.go by the coder). Using the literal here
	// so this test does not create a circular compile dependency on an in-flight constant.
	const parentID = "prod-dbi-retention-parent"
	// Snapshot: automated, 30 days old, parent has 7-day retention.
	pastTime := time.Now().UTC().Add(-30 * 24 * time.Hour)
	snap := rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.WarnRDSSnapPastRetentionID),
		DBInstanceIdentifier: aws.String(parentID),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
		SnapshotType:         aws.String("automated"),
		SnapshotCreateTime:   &pastTime,
	}
	// Parent dbi with BackupRetentionPeriod=7.
	cache := dbiCacheWith([]rdstypes.DBInstance{
		{
			DBInstanceIdentifier: aws.String(parentID),
			DBInstanceStatus:     aws.String("available"),
			BackupRetentionPeriod: aws.Int32(7),
		},
	})
	resources := []resource.Resource{snapResource(snap)}

	result, err := enricher(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("enricher returned unexpected error: %v", err)
	}

	snapID := fixtures.WarnRDSSnapPastRetentionID
	appends := result.IssueAppends[snapID]
	if len(appends) == 0 {
		t.Fatalf("IssueAppends[%q] = empty, want past-retention phrase", snapID)
	}
	found := false
	for _, phrase := range appends {
		if strings.Contains(phrase, "automated") && strings.Contains(phrase, "past retention") {
			found = true
		}
	}
	if !found {
		t.Errorf("IssueAppends[%q] = %v, want a phrase matching \"automated, <N>d past retention\"", snapID, appends)
	}
	// Verify the phrase contains "23d" (30 - 7 = 23 days past retention).
	for _, phrase := range appends {
		if strings.Contains(phrase, "past retention") && !strings.Contains(phrase, "23d") {
			t.Errorf("past-retention phrase %q should say 23d (30-7=23), got different days", phrase)
		}
	}
	// FieldUpdates must be set.
	if result.FieldUpdates == nil || result.FieldUpdates[snapID] == nil {
		t.Fatalf("FieldUpdates[%q] is nil, want status phrase set", snapID)
	}
}

// TestRDSSnap_Enricher_SkipOrphan_WhenDbiCacheMissing verifies that when
// the ResourceCache does NOT contain the "dbi" key at all, the orphan rule
// is skipped entirely (no false-positive orphan flags).
func TestRDSSnap_Enricher_SkipOrphan_WhenDbiCacheMissing(t *testing.T) {
	enricher := rdsSnapEnricher(t)

	snap := rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String("snap-x"),
		DBInstanceIdentifier: aws.String("some-db"),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
	}
	// Empty cache — "dbi" key absent.
	emptyCache := resource.ResourceCache{}
	resources := []resource.Resource{snapResource(snap)}

	result, err := enricher(context.Background(), nil, resources, emptyCache)
	if err != nil {
		t.Fatalf("enricher returned unexpected error: %v", err)
	}

	if appends := result.IssueAppends["snap-x"]; len(appends) > 0 {
		t.Errorf("IssueAppends[snap-x] = %v, want empty (orphan rule skipped when dbi cache absent)", appends)
	}
	if fu := result.FieldUpdates["snap-x"]; fu != nil && fu["status"] != "" {
		t.Errorf("FieldUpdates[snap-x][status] = %q, want empty (no findings when dbi cache absent)", fu["status"])
	}
}

// TestRDSSnap_Enricher_SkipPastRetention_WhenParentNotInCache verifies that
// when the dbi cache is loaded but the parent is NOT present, the orphan rule
// fires but the past-retention rule does NOT (spec §3.1: "skip when parent
// not in loaded sibling list"). The orphan rule is the only finding.
func TestRDSSnap_Enricher_SkipPastRetention_WhenParentNotInCache(t *testing.T) {
	enricher := rdsSnapEnricher(t)

	// Automated snapshot — past-retention rule would apply IF parent were present.
	pastTime := time.Now().UTC().Add(-30 * 24 * time.Hour)
	snap := rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String("snap-automated-missing-parent"),
		DBInstanceIdentifier: aws.String("missing-from-dbi"),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
		SnapshotType:         aws.String("automated"),
		SnapshotCreateTime:   &pastTime,
	}
	// dbi cache exists but parent is absent — only "other-db" is present.
	cache := dbiCacheWith([]rdstypes.DBInstance{
		{
			DBInstanceIdentifier:  aws.String("other-db"),
			DBInstanceStatus:      aws.String("available"),
			BackupRetentionPeriod: aws.Int32(7),
		},
	})
	resources := []resource.Resource{snapResource(snap)}

	result, err := enricher(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("enricher returned unexpected error: %v", err)
	}

	appends := result.IssueAppends["snap-automated-missing-parent"]
	// Orphan rule should fire (parent not found in loaded dbi cache).
	orphanFound := false
	pastRetentionFound := false
	for _, phrase := range appends {
		if phrase == "orphan: source DB deleted" {
			orphanFound = true
		}
		if strings.Contains(phrase, "past retention") {
			pastRetentionFound = true
		}
	}
	if !orphanFound {
		t.Errorf("expected orphan finding in IssueAppends, got %v", appends)
	}
	if pastRetentionFound {
		t.Errorf("past-retention rule fired even though parent is not in dbi cache — should be skipped; got %v", appends)
	}
	// Status must say "orphan: source DB deleted" (orphan wins; no double-emit).
	if fu := result.FieldUpdates["snap-automated-missing-parent"]; fu != nil {
		statusPhrase := fu["status"]
		if strings.Contains(statusPhrase, "past retention") {
			t.Errorf("FieldUpdates status = %q, must not contain past-retention phrase when parent absent", statusPhrase)
		}
	}
}

// TestRDSSnap_Enricher_MultiW1_UnencryptedPlusOrphan_Suffix verifies (U7a) that
// when the fetcher already set Status="unencrypted" and the enricher finds the
// orphan signal, BumpFindingSuffix is applied: final status = "unencrypted (+1)".
func TestRDSSnap_Enricher_MultiW1_UnencryptedPlusOrphan_Suffix(t *testing.T) {
	enricher := rdsSnapEnricher(t)

	snap := rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.MultiW1RDSSnapID),
		DBInstanceIdentifier: aws.String("deleted-legacy-db"),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(false),
		SnapshotType:         aws.String("manual"),
	}
	// dbi cache loaded but "deleted-legacy-db" is absent.
	cache := dbiCacheWith([]rdstypes.DBInstance{
		{DBInstanceIdentifier: aws.String("prod-dbi-1"), DBInstanceStatus: aws.String("available")},
	})
	// The resource arrives at the enricher with fetcher-set Status="unencrypted".
	res := snapResourceWithStatus(snap, "unencrypted")
	resources := []resource.Resource{res}

	result, err := enricher(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("enricher returned unexpected error: %v", err)
	}

	snapID := fixtures.MultiW1RDSSnapID
	appends := result.IssueAppends[snapID]
	orphanFound := false
	for _, phrase := range appends {
		if phrase == "orphan: source DB deleted" {
			orphanFound = true
		}
	}
	if !orphanFound {
		t.Errorf("IssueAppends[%q] = %v, want \"orphan: source DB deleted\"", snapID, appends)
	}
	// FieldUpdates["status"] must be "unencrypted (+1)" — BumpFindingSuffix applied.
	fu := result.FieldUpdates[snapID]
	if fu == nil {
		t.Fatalf("FieldUpdates[%q] is nil", snapID)
	}
	if fu["status"] != "unencrypted (+1)" {
		t.Errorf("FieldUpdates[%q][status] = %q, want %q", snapID, fu["status"], "unencrypted (+1)")
	}
}

// TestRDSSnap_Enricher_NoOp_WhenNoCrossRefSignalsApply verifies that a Healthy
// snapshot whose parent IS in the dbi cache and is a manual type produces no
// findings, and the result maps are non-nil but empty.
func TestRDSSnap_Enricher_NoOp_WhenNoCrossRefSignalsApply(t *testing.T) {
	enricher := rdsSnapEnricher(t)

	snap := rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.ProdRDSSnapID),
		DBInstanceIdentifier: aws.String(fixtures.ProdDbiID),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
		SnapshotType:         aws.String("manual"),
	}
	cache := dbiCacheWith([]rdstypes.DBInstance{
		{
			DBInstanceIdentifier:  aws.String(fixtures.ProdDbiID),
			DBInstanceStatus:      aws.String("available"),
			BackupRetentionPeriod: aws.Int32(7),
		},
	})
	resources := []resource.Resource{snapResource(snap)}

	result, err := enricher(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("enricher returned unexpected error: %v", err)
	}

	snapID := fixtures.ProdRDSSnapID
	if appends := result.IssueAppends[snapID]; len(appends) > 0 {
		t.Errorf("IssueAppends[%q] = %v, want empty (no cross-ref signals)", snapID, appends)
	}
	if fu := result.FieldUpdates[snapID]; fu != nil && fu["status"] != "" {
		t.Errorf("FieldUpdates[%q][status] = %q, want empty (no findings)", snapID, fu["status"])
	}
	// Maps must be non-nil (contract: "MUST NOT be nil on success").
	if result.IssueAppends == nil {
		t.Error("IssueAppends is nil, want non-nil empty map on success")
	}
	if result.FieldUpdates == nil {
		t.Error("FieldUpdates is nil, want non-nil empty map on success")
	}
	if result.Findings == nil {
		t.Error("Findings is nil, want non-nil empty map on success")
	}
	if result.TruncatedIDs == nil {
		t.Error("TruncatedIDs is nil, want non-nil empty map on success")
	}
}

// TestRDSSnap_Enricher_FindingMirrorsIssueAppend verifies that the enricher
// emits an EnrichmentFinding alongside IssueAppends for every cross-ref
// signal. Both paths carry the SAME §4 phrase: IssueAppends drives merging
// into Resource.Issues for cached rows, Findings drives the detail view's
// Attention section for resources fetched fresh (the test harness uses a
// fresh fetch path in OpenDetailResource — without Findings, the detail
// Attention section would be invisible for orphan/past-retention rows).
//
// Spec §3.2 says "Wave 2 = None" because no extra AWS API calls are made;
// emitting through the Findings channel is an internal routing decision,
// not a Wave-2 API claim.
func TestRDSSnap_Enricher_FindingMirrorsIssueAppend(t *testing.T) {
	enricher := rdsSnapEnricher(t)

	snap := rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String("snap-orphan-check"),
		DBInstanceIdentifier: aws.String("deleted-db"),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
		SnapshotType:         aws.String("manual"),
	}
	// dbi cache loaded but parent absent → orphan fires.
	cache := dbiCacheWith([]rdstypes.DBInstance{
		{DBInstanceIdentifier: aws.String("prod-dbi-1"), DBInstanceStatus: aws.String("available")},
	})
	resources := []resource.Resource{snapResource(snap)}

	result, err := enricher(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("enricher returned unexpected error: %v", err)
	}

	finding, ok := result.Findings["snap-orphan-check"]
	if !ok {
		t.Fatalf("Findings missing entry for snap-orphan-check; want a Finding mirroring the IssueAppends phrase")
	}
	if finding.Summary != "orphan: source DB deleted" {
		t.Errorf("Finding.Summary = %q, want %q", finding.Summary, "orphan: source DB deleted")
	}
	if finding.Severity != "!" {
		t.Errorf("Finding.Severity = %q, want %q", finding.Severity, "!")
	}
	appends := result.IssueAppends["snap-orphan-check"]
	if len(appends) == 0 || appends[0] != finding.Summary {
		t.Errorf("IssueAppends[0] = %v, want first entry to match Finding.Summary %q", appends, finding.Summary)
	}
}

// TestRDSSnap_Enricher_PartialFailure_NoAPICalls verifies that the enricher
// makes zero API calls and never returns a non-nil error, even when clients is nil.
func TestRDSSnap_Enricher_PartialFailure_NoAPICalls(t *testing.T) {
	enricher := rdsSnapEnricher(t)

	// Various cache states — all should succeed with nil error.
	caches := []struct {
		name  string
		cache resource.ResourceCache
	}{
		{"nil_cache", nil},
		{"empty_cache", resource.ResourceCache{}},
		{"dbi_loaded", dbiCacheFromFixtures(t)},
	}

	snap := rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String("snap-nil-clients"),
		DBInstanceIdentifier: aws.String("any-db"),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
	}
	resources := []resource.Resource{snapResource(snap)}

	for _, tc := range caches {
		t.Run(tc.name, func(t *testing.T) {
			_, err := enricher(context.Background(), nil, resources, tc.cache)
			if err != nil {
				t.Errorf("enricher(nil clients, %s): unexpected error %v", tc.name, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Additional: verify full fixture round-trip through enricher
// ---------------------------------------------------------------------------

// TestRDSSnap_Enricher_FullFixtures_OrphanAndRetentionFound verifies that
// running the full fixture set + dbi cache produces orphan findings for
// fixtures with "deleted-legacy-db" as parent and past-retention findings
// for WarnRDSSnapPastRetentionID.
func TestRDSSnap_Enricher_FullFixtures_OrphanAndRetentionFound(t *testing.T) {
	enricher := rdsSnapEnricher(t)

	fix := fixtures.NewRDSSnapFixtures()
	dbiCache := dbiCacheFromFixtures(t)

	// Convert all snap fixtures to Resources (status pre-set from fetcher values).
	resources := make([]resource.Resource, 0, len(fix.Instances))
	for _, snap := range fix.Instances {
		id := ""
		if snap.DBSnapshotIdentifier != nil {
			id = *snap.DBSnapshotIdentifier
		}
		// Build resource without fetcher (we only care about cross-ref logic).
		resources = append(resources, resource.Resource{
			ID:        id,
			Name:      id,
			Status:    "",
			Fields:    map[string]string{},
			RawStruct: snap,
		})
	}

	result, err := enricher(context.Background(), nil, resources, dbiCache)
	if err != nil {
		t.Fatalf("enricher returned unexpected error: %v", err)
	}

	// WarnRDSSnapOrphanID ("orphan-deleted-db-snap") has parent "deleted-legacy-db"
	// which is NOT in the dbi fixtures → orphan finding expected.
	orphanID := fixtures.WarnRDSSnapOrphanID
	foundOrphan := false
	for _, phrase := range result.IssueAppends[orphanID] {
		if phrase == "orphan: source DB deleted" {
			foundOrphan = true
		}
	}
	if !foundOrphan {
		t.Errorf("WarnRDSSnapOrphanID: IssueAppends = %v, want \"orphan: source DB deleted\"",
			result.IssueAppends[orphanID])
	}

	// MultiW1RDSSnapID also has "deleted-legacy-db" as parent → orphan.
	multiID := fixtures.MultiW1RDSSnapID
	foundMultiOrphan := false
	for _, phrase := range result.IssueAppends[multiID] {
		if phrase == "orphan: source DB deleted" {
			foundMultiOrphan = true
		}
	}
	if !foundMultiOrphan {
		t.Errorf("MultiW1RDSSnapID: IssueAppends = %v, want \"orphan: source DB deleted\"",
			result.IssueAppends[multiID])
	}

	// WarnRDSSnapPastRetentionID has parent WarnDbiPastRetentionParentID (in dbi cache)
	// with BackupRetentionPeriod=7 and SnapshotCreateTime=now-30d → past-retention expected.
	retentionID := fixtures.WarnRDSSnapPastRetentionID
	foundRetention := false
	for _, phrase := range result.IssueAppends[retentionID] {
		if strings.Contains(phrase, "automated") && strings.Contains(phrase, "past retention") {
			foundRetention = true
		}
	}
	if !foundRetention {
		t.Errorf("WarnRDSSnapPastRetentionID: IssueAppends = %v, want \"automated, Nd past retention\"",
			result.IssueAppends[retentionID])
	}

	// Healthy fixtures (ProdRDSSnapID, ProdRDSSnapAuroraID) with parent in dbi cache
	// must produce no orphan or retention findings.
	for _, id := range []string{fixtures.ProdRDSSnapID, fixtures.ProdRDSSnapAuroraID} {
		for _, phrase := range result.IssueAppends[id] {
			if strings.Contains(phrase, "orphan") || strings.Contains(phrase, "past retention") {
				t.Errorf("healthy snap %q: unexpected finding %q", id, phrase)
			}
		}
	}
}

