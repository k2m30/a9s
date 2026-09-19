package unit

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func dbiSnapEnricher(t *testing.T) awsclient.IssueEnricherFunc {
	t.Helper()
	e, ok := awsclient.Wave2EnricherFor("dbi-snap")
	if !ok {
		t.Fatal("awsclient.Wave2EnricherFor(\"dbi-snap\") not registered (catalog Wave2 field missing)")
	}
	if e.Fn == nil {
		t.Fatal("Wave2EnricherFor(\"dbi-snap\").Fn is nil")
	}
	return e.Fn
}

func snapResource(snap rdstypes.DBSnapshot) resource.Resource {
	id := ""
	if snap.DBSnapshotIdentifier != nil {
		id = *snap.DBSnapshotIdentifier
	}
	// The enricher operates on Resources produced by FetchDBISnapshotsPage;
	// it reads RawStruct + Findings.
	r := resource.Resource{
		ID:        id,
		Name:      id,
		Fields:    map[string]string{},
		RawStruct: snap,
	}
	return r
}

func snapResourceWithStatus(snap rdstypes.DBSnapshot, preStatus string) resource.Resource {
	r := snapResource(snap)
	if preStatus != "" {
		r.Fields["status"] = preStatus
		r.Findings = []domain.Finding{{
			Code:     domain.FindingCode("dbi-snap." + preStatus),
			Phrase:   preStatus,
			Severity: domain.SevBroken,
			Source:   "wave1",
		}}
	}
	return r
}

// dbiCacheFromFixtures builds the "dbi" cache entry from the same list
// DescribeDBInstances serves. Built from the canonical set alone it would
// report every snapshot whose parent lives in the RDSFixtures pool as an
// orphan, which is a verdict no running demo produces.
func dbiCacheFromFixtures(t *testing.T) resource.ResourceCache {
	t.Helper()
	instances := fixtures.NewRDSFixtures().DBInstances
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

func TestDBISnap_Enricher_Orphan_DbiMissingFromCache(t *testing.T) {
	enricher := dbiSnapEnricher(t)

	snap := rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.WarnDBISnapOrphanID),
		DBInstanceIdentifier: aws.String("deleted-legacy-db"),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
		SnapshotType:         aws.String("manual"),
	}
	cache := dbiCacheWith([]rdstypes.DBInstance{
		{DBInstanceIdentifier: aws.String("prod-dbi-1"), DBInstanceStatus: aws.String("available")},
	})
	resources := []resource.Resource{snapResource(snap)}

	result, err := enricher(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("enricher returned unexpected error: %v", err)
	}

	snapID := fixtures.WarnDBISnapOrphanID
	findings, hasFinding := result.Findings[snapID]
	if !hasFinding {
		t.Fatalf("Findings[%q] missing, want a finding with Summary matching the §4 phrase", snapID)
	}
	finding := findings[0]
	if finding.Phrase != "orphan: source DB deleted" {
		t.Errorf("Findings[%q].Phrase = %q, want %q", snapID, finding.Phrase, "orphan: source DB deleted")
	}
	// FieldUpdates must be empty — the merged display phrase is
	// computed at render time by phraseFromFindings(r.Findings).
	if updates, ok := result.FieldUpdates[snapID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", snapID, updates)
	}
}

func TestDBISnap_Enricher_AutomatedPastRetention_BasicCase(t *testing.T) {
	enricher := dbiSnapEnricher(t)

	// "prod-dbi-retention-parent" is the value of fixtures.WarnDbiPastRetentionParentID
	// (core/demo/fixtures/dbi.go); the literal keeps this test independent of
	// the fixtures package.
	const parentID = "prod-dbi-retention-parent"
	pastTime := time.Now().UTC().Add(-30 * 24 * time.Hour)
	snap := rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.WarnDBISnapPastRetentionID),
		DBInstanceIdentifier: aws.String(parentID),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
		SnapshotType:         aws.String("automated"),
		SnapshotCreateTime:   &pastTime,
	}
	cache := dbiCacheWith([]rdstypes.DBInstance{
		{
			DBInstanceIdentifier:  aws.String(parentID),
			DBInstanceStatus:      aws.String("available"),
			BackupRetentionPeriod: aws.Int32(7),
		},
	})
	resources := []resource.Resource{snapResource(snap)}

	result, err := enricher(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("enricher returned unexpected error: %v", err)
	}

	snapID := fixtures.WarnDBISnapPastRetentionID
	findings, hasFinding := result.Findings[snapID]
	if !hasFinding {
		t.Fatalf("Findings[%q] missing, want past-retention finding", snapID)
	}
	finding := findings[0]
	if !strings.Contains(finding.Phrase, "automated") || !strings.Contains(finding.Phrase, "past retention") {
		t.Errorf("Findings[%q].Phrase = %q, want a phrase matching \"automated, <N>d past retention\"", snapID, finding.Phrase)
	}
	if strings.Contains(finding.Phrase, "past retention") && !strings.Contains(finding.Phrase, "23d") {
		t.Errorf("past-retention Summary %q should say 23d (30-7=23), got different days", finding.Phrase)
	}
	// FieldUpdates must be empty — the merged display phrase is
	// computed at render time by phraseFromFindings(r.Findings).
	if updates, ok := result.FieldUpdates[snapID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", snapID, updates)
	}
}

func TestDBISnap_Enricher_SkipOrphan_WhenDbiCacheMissing(t *testing.T) {
	enricher := dbiSnapEnricher(t)

	snap := rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String("snap-x"),
		DBInstanceIdentifier: aws.String("some-db"),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
	}
	emptyCache := resource.ResourceCache{}
	resources := []resource.Resource{snapResource(snap)}

	result, err := enricher(context.Background(), nil, resources, emptyCache)
	if err != nil {
		t.Fatalf("enricher returned unexpected error: %v", err)
	}

	if _, has := result.Findings["snap-x"]; has {
		t.Errorf("Findings[snap-x] present, want absent (orphan rule skipped when dbi cache absent)")
	}
	if fu := result.FieldUpdates["snap-x"]; fu != nil && fu["status"] != "" {
		t.Errorf("FieldUpdates[snap-x][status] = %q, want empty (no findings when dbi cache absent)", fu["status"])
	}
}

func TestDBISnap_Enricher_SkipPastRetention_WhenParentNotInCache(t *testing.T) {
	enricher := dbiSnapEnricher(t)

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

	findings, hasFinding := result.Findings["snap-automated-missing-parent"]
	if !hasFinding {
		t.Fatalf("Findings[snap-automated-missing-parent] missing, want orphan finding")
	}
	finding := findings[0]
	if finding.Phrase != "orphan: source DB deleted" {
		t.Errorf("Findings[snap-automated-missing-parent].Phrase = %q, want \"orphan: source DB deleted\"", finding.Phrase)
	}
	if strings.Contains(finding.Phrase, "past retention") {
		t.Errorf("past-retention phrase in finding even though parent is not in dbi cache — should be skipped; Summary=%q", finding.Phrase)
	}
	if fu := result.FieldUpdates["snap-automated-missing-parent"]; fu != nil {
		statusPhrase := fu["status"]
		if strings.Contains(statusPhrase, "past retention") {
			t.Errorf("FieldUpdates status = %q, must not contain past-retention phrase when parent absent", statusPhrase)
		}
	}
}

func TestDBISnap_Enricher_MultiW1_UnencryptedPlusOrphan_Suffix(t *testing.T) {
	enricher := dbiSnapEnricher(t)

	snap := rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.MultiW1DBISnapID),
		DBInstanceIdentifier: aws.String("deleted-legacy-db"),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(false),
		SnapshotType:         aws.String("manual"),
	}
	cache := dbiCacheWith([]rdstypes.DBInstance{
		{DBInstanceIdentifier: aws.String("prod-dbi-1"), DBInstanceStatus: aws.String("available")},
	})
	res := snapResourceWithStatus(snap, "unencrypted")
	resources := []resource.Resource{res}

	result, err := enricher(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("enricher returned unexpected error: %v", err)
	}

	snapID := fixtures.MultiW1DBISnapID
	findings, hasFinding := result.Findings[snapID]
	if !hasFinding {
		t.Fatalf("Findings[%q] missing, want orphan finding", snapID)
	}
	finding := findings[0]
	if finding.Phrase != "orphan: source DB deleted" {
		t.Errorf("Findings[%q].Phrase = %q, want \"orphan: source DB deleted\"", snapID, finding.Phrase)
	}
	// FieldUpdates must be empty — the merged "unencrypted (+1)"
	// stack is computed at render time by phraseFromFindings(r.Findings),
	// which aggregates the Wave-1 "unencrypted" finding and this enricher's
	// Wave-2 orphan finding.
	if updates, ok := result.FieldUpdates[snapID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", snapID, updates)
	}
}

func TestDBISnap_Enricher_NoOp_WhenNoCrossRefSignalsApply(t *testing.T) {
	enricher := dbiSnapEnricher(t)

	snap := rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.ProdDBISnapID),
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

	snapID := fixtures.ProdDBISnapID
	if _, has := result.Findings[snapID]; has {
		t.Errorf("Findings[%q] present, want absent (no cross-ref signals)", snapID)
	}
	// The merged display phrase is computed at render time, so FieldUpdates stays
	// empty.
	if updates, ok := result.FieldUpdates[snapID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", snapID, updates)
	}
	if result.Findings == nil {
		t.Error("Findings is nil, want non-nil empty map on success")
	}
	if result.TruncatedIDs == nil {
		t.Error("TruncatedIDs is nil, want non-nil empty map on success")
	}
}

// The Findings channel drives the detail-view Attention section for orphan
// and past-retention rows.
func TestDBISnap_Enricher_FindingMirrorsIssueAppend(t *testing.T) {
	enricher := dbiSnapEnricher(t)

	snap := rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String("snap-orphan-check"),
		DBInstanceIdentifier: aws.String("deleted-db"),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
		SnapshotType:         aws.String("manual"),
	}
	cache := dbiCacheWith([]rdstypes.DBInstance{
		{DBInstanceIdentifier: aws.String("prod-dbi-1"), DBInstanceStatus: aws.String("available")},
	})
	resources := []resource.Resource{snapResource(snap)}

	result, err := enricher(context.Background(), nil, resources, cache)
	if err != nil {
		t.Fatalf("enricher returned unexpected error: %v", err)
	}

	findings, ok := result.Findings["snap-orphan-check"]
	if !ok {
		t.Fatalf("Findings missing entry for snap-orphan-check; want a Finding with orphan Summary")
	}
	finding := findings[0]
	if finding.Phrase != "orphan: source DB deleted" {
		t.Errorf("Finding.Phrase = %q, want %q", finding.Phrase, "orphan: source DB deleted")
	}
	if finding.Severity != domain.SevBroken {
		t.Errorf("Finding.Severity = %v, want SevBroken", finding.Severity)
	}
}

func TestDBISnap_Enricher_PartialFailure_NoAPICalls(t *testing.T) {
	enricher := dbiSnapEnricher(t)

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

func TestDBISnap_Enricher_FullFixtures_OrphanAndRetentionFound(t *testing.T) {
	enricher := dbiSnapEnricher(t)

	fix := fixtures.NewDBISnapFixtures()
	dbiCache := dbiCacheFromFixtures(t)

	resources := make([]resource.Resource, 0, len(fix.Instances))
	for _, snap := range fix.Instances {
		id := ""
		if snap.DBSnapshotIdentifier != nil {
			id = *snap.DBSnapshotIdentifier
		}
		resources = append(resources, resource.Resource{
			ID:        id,
			Name:      id,
			Fields:    map[string]string{},
			RawStruct: snap,
		})
	}

	result, err := enricher(context.Background(), nil, resources, dbiCache)
	if err != nil {
		t.Fatalf("enricher returned unexpected error: %v", err)
	}

	orphanID := fixtures.WarnDBISnapOrphanID
	orphanFindings, hasOrphan := result.Findings[orphanID]
	if !hasOrphan {
		t.Errorf("WarnDBISnapOrphanID: Findings[%q] missing, want orphan finding", orphanID)
	} else if orphanFinding := orphanFindings[0]; orphanFinding.Phrase != "orphan: source DB deleted" {
		t.Errorf("WarnDBISnapOrphanID: Findings[%q].Phrase = %q, want \"orphan: source DB deleted\"", orphanID, orphanFinding.Phrase)
	}

	multiID := fixtures.MultiW1DBISnapID
	multiFindings, hasMulti := result.Findings[multiID]
	if !hasMulti {
		t.Errorf("MultiW1DBISnapID: Findings[%q] missing, want orphan finding", multiID)
	} else if multiFinding := multiFindings[0]; multiFinding.Phrase != "orphan: source DB deleted" {
		t.Errorf("MultiW1DBISnapID: Findings[%q].Phrase = %q, want \"orphan: source DB deleted\"", multiID, multiFinding.Phrase)
	}

	retentionID := fixtures.WarnDBISnapPastRetentionID
	retFindings, hasRetention := result.Findings[retentionID]
	if !hasRetention {
		t.Errorf("WarnDBISnapPastRetentionID: Findings[%q] missing, want past-retention finding", retentionID)
	} else if retFinding := retFindings[0]; !strings.Contains(retFinding.Phrase, "automated") || !strings.Contains(retFinding.Phrase, "past retention") {
		t.Errorf("WarnDBISnapPastRetentionID: Findings[%q].Phrase = %q, want \"automated, Nd past retention\"", retentionID, retFinding.Phrase)
	}

	for _, id := range []string{fixtures.ProdDBISnapID} {
		if fs, has := result.Findings[id]; has {
			f := fs[0]
			if strings.Contains(f.Phrase, "orphan") || strings.Contains(f.Phrase, "past retention") {
				t.Errorf("healthy snap %q: unexpected finding %q", id, f.Phrase)
			}
		}
	}
}
