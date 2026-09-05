package unit

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// d1_transitional_status_test.go pins the rule that a state a9s does not
// enumerate is reported rather than swallowed. Snapshots say so with a
// keyword-passthrough finding; a DB instance being deleted says so through the
// transitional set it already had. Both directions matter: the arm must fire on
// the states nobody named, and must not fire on the ones that already have a
// finding of their own.

// d1FindOne returns the single finding with code, failing when the row carries
// a different number of them.
func d1FindOne(t *testing.T, findings []domain.Finding, code domain.FindingCode) domain.Finding {
	t.Helper()
	var hits []domain.Finding
	for _, f := range findings {
		if f.Code == code {
			hits = append(hits, f)
		}
	}
	if len(hits) != 1 {
		t.Fatalf("findings carry %d entries with code %q, want exactly 1; findings = %+v", len(hits), code, findings)
	}
	return hits[0]
}

func d1AssertFinding(t *testing.T, f domain.Finding, phrase string, sev domain.Severity) {
	t.Helper()
	if f.Phrase != phrase {
		t.Errorf("Phrase = %q, want %q", f.Phrase, phrase)
	}
	if f.Severity != sev {
		t.Errorf("Severity = %v, want %v", f.Severity, sev)
	}
	if f.Source != "wave1" {
		t.Errorf("Source = %q, want %q", f.Source, "wave1")
	}
}

// ---------------------------------------------------------------------------
// dbi-snap
// ---------------------------------------------------------------------------

func d1FetchDbiSnap(t *testing.T, snap rdstypes.DBSnapshot) []domain.Finding {
	t.Helper()
	rows := fetchSnap(t, snap)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	return rows[0].findings
}

func TestD1_DbiSnapUnnamedStatusIsReported(t *testing.T) {
	snap := d1DbiSnapBaseline()
	snap.Status = aws.String("copying")

	f := d1FindOne(t, d1FetchDbiSnap(t, snap), awsclient.CodeDBISnapTransitional)
	d1AssertFinding(t, f, "copying", domain.SevWarn)
}

// The creating arm owns its own phrase and percentage. A status that already
// has a finding must not also collect the passthrough one.
func TestD1_DbiSnapCreatingDoesNotAlsoFireTransitional(t *testing.T) {
	snap := d1DbiSnapBaseline()
	snap.Status = aws.String("creating")
	snap.PercentProgress = aws.Int32(42)

	findings := d1FetchDbiSnap(t, snap)
	f := d1FindOne(t, findings, awsclient.CodeDBISnapCreating)
	d1AssertFinding(t, f, "creating: 42%", domain.SevWarn)
	for _, got := range findings {
		if got.Code == awsclient.CodeDBISnapTransitional {
			t.Errorf("creating snapshot also carries %q; findings = %+v", awsclient.CodeDBISnapTransitional, findings)
		}
	}
}

// A broken end-state returns before the transitional arm, so the row reports
// the failure alone rather than the failure plus "not restorable yet".
func TestD1_DbiSnapBrokenStatusesDoNotFireTransitional(t *testing.T) {
	for _, status := range []string{"failed", "incompatible-restore", "incompatible-parameters"} {
		t.Run(status, func(t *testing.T) {
			snap := d1DbiSnapBaseline()
			snap.Status = aws.String(status)
			snap.Encrypted = aws.Bool(false)

			findings := d1FetchDbiSnap(t, snap)
			if len(findings) != 1 {
				t.Fatalf("findings = %+v, want exactly the broken one", findings)
			}
			if findings[0].Severity != domain.SevBroken {
				t.Errorf("Severity = %v, want SevBroken", findings[0].Severity)
			}
		})
	}
}

func TestD1_DbiSnapReadyStatusesStaySilent(t *testing.T) {
	for _, status := range []string{"available", ""} {
		t.Run("status_"+status, func(t *testing.T) {
			snap := d1DbiSnapBaseline()
			snap.Status = aws.String(status)
			if findings := d1FetchDbiSnap(t, snap); len(findings) != 0 {
				t.Errorf("findings = %+v, want none", findings)
			}
		})
	}
}

// The passthrough is a lifecycle finding, not a replacement for the posture
// pack: a snapshot that is both mid-copy and unencrypted reports both.
func TestD1_DbiSnapUnnamedStatusStacksWithUnencrypted(t *testing.T) {
	snap := d1DbiSnapBaseline()
	snap.Status = aws.String("copying")
	snap.Encrypted = aws.Bool(false)

	findings := d1FetchDbiSnap(t, snap)
	d1AssertFinding(t, d1FindOne(t, findings, awsclient.CodeDBISnapTransitional), "copying", domain.SevWarn)
	d1AssertFinding(t, d1FindOne(t, findings, awsclient.CodeDBISnapUnencrypted), "unencrypted", domain.SevWarn)
}

// ---------------------------------------------------------------------------
// dbc-snap — two predicates behind one type, so both are driven here
// ---------------------------------------------------------------------------

func d1FetchDbcSnapRDS(t *testing.T, snap rdstypes.DBClusterSnapshot) []domain.Finding {
	t.Helper()
	mock := &dbcSnapRDSSinglePageMock{
		output: &rds.DescribeDBClusterSnapshotsOutput{DBClusterSnapshots: []rdstypes.DBClusterSnapshot{snap}},
	}
	page, err := awsclient.FetchRDSDBClusterSnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSDBClusterSnapshotsPage: %v", err)
	}
	if len(page.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(page.Resources))
	}
	return page.Resources[0].Findings
}

// d1DbcSnapDocDBBaseline is the DocumentDB-side counterpart of
// d1DbcSnapBaseline: healthy, encrypted, and recent enough to stay silent.
func d1DbcSnapDocDBBaseline() docdbtypes.DBClusterSnapshot {
	return docdbtypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String("acme-docdb-prod-snap"),
		DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:acme-docdb-prod-snap"),
		DBClusterIdentifier:         aws.String("acme-docdb-prod"),
		Engine:                      aws.String("docdb"),
		Status:                      aws.String("available"),
		StorageEncrypted:            aws.Bool(true),
		SnapshotType:                aws.String("automated"),
		SnapshotCreateTime:          aws.Time(time.Now().AddDate(0, 0, -3)),
	}
}

func d1FetchDbcSnapDocDB(t *testing.T, snap docdbtypes.DBClusterSnapshot) []domain.Finding {
	t.Helper()
	mock := &dbcSnapDocDBSinglePageMock{
		output: &docdb.DescribeDBClusterSnapshotsOutput{DBClusterSnapshots: []docdbtypes.DBClusterSnapshot{snap}},
	}
	page, err := awsclient.FetchDocDBClusterSnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchDocDBClusterSnapshotsPage: %v", err)
	}
	if len(page.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(page.Resources))
	}
	return page.Resources[0].Findings
}

// One short name, two SDKs, one contract: a DocumentDB cluster snapshot and an
// Aurora one must answer the same way, or the type reads differently depending
// on which API returned the row.
func TestD1_DbcSnapUnnamedStatusIsReportedOnBothPredicates(t *testing.T) {
	t.Run("rds", func(t *testing.T) {
		snap := d1DbcSnapBaseline()
		snap.Status = aws.String("copying")
		f := d1FindOne(t, d1FetchDbcSnapRDS(t, snap), awsclient.CodeDBCSnapTransitional)
		d1AssertFinding(t, f, "copying", domain.SevWarn)
	})
	t.Run("docdb", func(t *testing.T) {
		snap := d1DbcSnapDocDBBaseline()
		snap.Status = aws.String("copying")
		f := d1FindOne(t, d1FetchDbcSnapDocDB(t, snap), awsclient.CodeDBCSnapTransitional)
		d1AssertFinding(t, f, "copying", domain.SevWarn)
	})
}

func TestD1_DbcSnapCreatingDoesNotAlsoFireTransitional(t *testing.T) {
	t.Run("rds", func(t *testing.T) {
		snap := d1DbcSnapBaseline()
		snap.Status = aws.String("creating")
		findings := d1FetchDbcSnapRDS(t, snap)
		d1AssertFinding(t, d1FindOne(t, findings, awsclient.CodeDBCSnapCreating), "creating", domain.SevWarn)
		for _, got := range findings {
			if got.Code == awsclient.CodeDBCSnapTransitional {
				t.Errorf("creating snapshot also carries the passthrough; findings = %+v", findings)
			}
		}
	})
	t.Run("docdb", func(t *testing.T) {
		snap := d1DbcSnapDocDBBaseline()
		snap.Status = aws.String("creating")
		findings := d1FetchDbcSnapDocDB(t, snap)
		d1AssertFinding(t, d1FindOne(t, findings, awsclient.CodeDBCSnapCreating), "creating", domain.SevWarn)
		for _, got := range findings {
			if got.Code == awsclient.CodeDBCSnapTransitional {
				t.Errorf("creating snapshot also carries the passthrough; findings = %+v", findings)
			}
		}
	})
}

func TestD1_DbcSnapReadyStatusesStaySilentOnBothPredicates(t *testing.T) {
	for _, status := range []string{"available", ""} {
		t.Run("rds_status_"+status, func(t *testing.T) {
			snap := d1DbcSnapBaseline()
			snap.Status = aws.String(status)
			if findings := d1FetchDbcSnapRDS(t, snap); len(findings) != 0 {
				t.Errorf("findings = %+v, want none", findings)
			}
		})
		t.Run("docdb_status_"+status, func(t *testing.T) {
			snap := d1DbcSnapDocDBBaseline()
			snap.Status = aws.String(status)
			if findings := d1FetchDbcSnapDocDB(t, snap); len(findings) != 0 {
				t.Errorf("findings = %+v, want none", findings)
			}
		})
	}
}

func TestD1_DbcSnapBrokenStatusesDoNotFireTransitional(t *testing.T) {
	for _, status := range []string{"failed", "incompatible-restore"} {
		t.Run("rds_"+status, func(t *testing.T) {
			snap := d1DbcSnapBaseline()
			snap.Status = aws.String(status)
			snap.StorageEncrypted = aws.Bool(false)
			findings := d1FetchDbcSnapRDS(t, snap)
			if len(findings) != 1 || findings[0].Severity != domain.SevBroken {
				t.Errorf("findings = %+v, want exactly the broken one", findings)
			}
		})
		t.Run("docdb_"+status, func(t *testing.T) {
			snap := d1DbcSnapDocDBBaseline()
			snap.Status = aws.String(status)
			snap.StorageEncrypted = aws.Bool(false)
			findings := d1FetchDbcSnapDocDB(t, snap)
			if len(findings) != 1 || findings[0].Severity != domain.SevBroken {
				t.Errorf("findings = %+v, want exactly the broken one", findings)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// dbi
// ---------------------------------------------------------------------------

func TestD1_DbiDeletingCarriesTheTransitionalWarning(t *testing.T) {
	inst := findDBI(t, fixtures.ProdDbiID)
	inst.DBInstanceStatus = aws.String("deleting")

	r := fetchSingleResource(t, inst)
	d1AssertFinding(t, d1FindOne(t, r.Findings, awsclient.CodeDBITransitional), "deleting", domain.SevWarn)
}

// An instance on its way out reports that it is going and nothing else. Its
// posture is not worth an operator's attention and would outrank the lifecycle
// row in the status column.
func TestD1_DbiDeletingSuppressesPosture(t *testing.T) {
	inst := findDBI(t, fixtures.ProdDbiID)
	inst.DBInstanceStatus = aws.String("deleting")
	inst.PubliclyAccessible = aws.Bool(true)
	inst.StorageEncrypted = aws.Bool(false)
	inst.MultiAZ = aws.Bool(false)
	inst.MasterUsername = aws.String("admin")

	r := fetchSingleResource(t, inst)
	if len(r.Findings) != 1 {
		t.Fatalf("findings = %+v, want only the deleting one", r.Findings)
	}
	if r.Findings[0].Code != awsclient.CodeDBITransitional {
		t.Errorf("Code = %q, want %q", r.Findings[0].Code, awsclient.CodeDBITransitional)
	}
}

// A status a9s does not recognise stays silent on the lifecycle for dbi, which
// is the opposite of the snapshot rule above. The two differ because a DB
// instance reports a settled state a9s can read posture from, while a snapshot
// in an unnamed state cannot be restored from at all.
func TestD1_DbiUnknownStatusEmitsNoLifecycleFinding(t *testing.T) {
	inst := findDBI(t, fixtures.ProdDbiID)
	inst.DBInstanceStatus = aws.String("some-future-aws-status")

	r := fetchSingleResource(t, inst)
	for _, f := range r.Findings {
		if f.Code == awsclient.CodeDBITransitional {
			t.Errorf("unknown status produced a transitional finding; findings = %+v", r.Findings)
		}
	}
}

// ---------------------------------------------------------------------------
// The demo witnesses
// ---------------------------------------------------------------------------

// A finding nobody can see in the demo is a finding the acceptance bench cannot
// check, so each new code needs a fixture that reaches it and reaches it alone.
func TestD1_DbiSnapCopyingWitnessCarriesTheTransitionalFindingAlone(t *testing.T) {
	fix := fixtures.NewDBISnapFixtures()
	mock := &fakeRDSDescribeDBSnapshots{Output: snapOutput(fix.Instances...)}
	page, err := awsclient.FetchDBISnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchDBISnapshotsPage: %v", err)
	}

	var seen int
	for _, r := range page.Resources {
		for _, f := range r.Findings {
			if f.Code != awsclient.CodeDBISnapTransitional {
				continue
			}
			seen++
			if r.ID != fixtures.WarnDBISnapCopyingID {
				t.Errorf("row %q carries %q; only the witness should", r.ID, f.Code)
			}
			d1AssertFinding(t, f, "copying", domain.SevWarn)
			if len(r.Findings) != 1 {
				t.Errorf("witness %q carries %+v, want the transitional finding alone", r.ID, r.Findings)
			}
		}
	}
	if seen != 1 {
		t.Errorf("%d demo rows carry %q, want exactly 1", seen, awsclient.CodeDBISnapTransitional)
	}
}

func TestD1_DbcSnapCopyingWitnessCarriesTheTransitionalFindingAlone(t *testing.T) {
	fix := fixtures.NewDBCFixtures()
	mock := &dbcSnapDocDBSinglePageMock{
		output: &docdb.DescribeDBClusterSnapshotsOutput{DBClusterSnapshots: fix.DBClusterSnapshots},
	}
	page, err := awsclient.FetchDocDBClusterSnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchDocDBClusterSnapshotsPage: %v", err)
	}

	var seen int
	for _, r := range page.Resources {
		for _, f := range r.Findings {
			if f.Code != awsclient.CodeDBCSnapTransitional {
				continue
			}
			seen++
			if r.ID != fixtures.WarnDBCSnapCopyingID {
				t.Errorf("row %q carries %q; only the witness should", r.ID, f.Code)
			}
			d1AssertFinding(t, f, "copying", domain.SevWarn)
			if len(r.Findings) != 1 {
				t.Errorf("witness %q carries %+v, want the transitional finding alone", r.ID, r.Findings)
			}
		}
	}
	if seen != 1 {
		t.Errorf("%d demo rows carry %q, want exactly 1", seen, awsclient.CodeDBCSnapTransitional)
	}
}

// The witness has to survive wave 2 to be worth anything: a row that picks up a
// Broken cross-ref finding renders that phrase in the status column instead, so
// the state the fixture exists to demonstrate is never shown.
func TestD1_DbiSnapCopyingWitnessSurvivesWave2(t *testing.T) {
	enricher := dbiSnapEnricher(t)
	fix := fixtures.NewDBISnapFixtures()

	resources := make([]resource.Resource, 0, len(fix.Instances))
	for _, snap := range fix.Instances {
		resources = append(resources, resource.Resource{
			ID:        aws.ToString(snap.DBSnapshotIdentifier),
			Fields:    map[string]string{},
			RawStruct: snap,
		})
	}

	result, err := enricher(context.Background(), nil, resources, dbiCacheFromFixtures(t))
	if err != nil {
		t.Fatalf("enricher: %v", err)
	}
	if got, has := result.Findings[fixtures.WarnDBISnapCopyingID]; has {
		t.Errorf("the copying witness picked up wave-2 findings %+v; its status column will show one of those, not \"copying\"", got)
	}
}

func TestD1_DbcSnapCopyingWitnessSurvivesWave2(t *testing.T) {
	fix := fixtures.NewDBCFixtures()

	clusters := make([]resource.Resource, 0, len(fix.DBClusters))
	for _, c := range fix.DBClusters {
		clusters = append(clusters, resource.Resource{ID: aws.ToString(c.DBClusterIdentifier), RawStruct: c})
	}
	cache := resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{Resources: clusters},
	}

	snaps := make([]resource.Resource, 0, len(fix.DBClusterSnapshots))
	for _, s := range fix.DBClusterSnapshots {
		snaps = append(snaps, resource.Resource{
			ID:        aws.ToString(s.DBClusterSnapshotIdentifier),
			Fields:    map[string]string{},
			RawStruct: s,
		})
	}

	result, err := dbcSnapEnricher(t)(context.Background(), nil, snaps, cache)
	if err != nil {
		t.Fatalf("enricher: %v", err)
	}
	if got, has := result.Findings[fixtures.WarnDBCSnapCopyingID]; has {
		t.Errorf("the copying witness picked up wave-2 findings %+v; its status column will show one of those, not \"copying\"", got)
	}
}
