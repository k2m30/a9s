package unit

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	spDBISnapOrphan        = domain.FindingCode("dbi-snap.orphan")
	spDBISnapPastRetention = domain.FindingCode("dbi-snap.past-retention")
	spDBCSnapOrphan        = domain.FindingCode("dbc-snap.orphan")
	spEBSSnapOrphan        = domain.FindingCode("ebs-snap.orphan")
)

func spHas(fs []domain.Finding, code domain.FindingCode) bool {
	return slices.ContainsFunc(fs, func(f domain.Finding) bool { return f.Code == code })
}

func spEnricher(t *testing.T, shortName string) awsclient.IssueEnricherFunc {
	t.Helper()
	e, ok := awsclient.Wave2EnricherFor(shortName)
	if !ok || e.Fn == nil {
		t.Fatalf("no Wave 2 enricher registered for %s", shortName)
	}
	return e.Fn
}

// A parent that is not expected in the local list proves nothing by being
// absent from it.
func TestSnapshotCrossRef_ParentNotLocalIsNeverOrphan(t *testing.T) {
	cfg := makeCrossRefCfg(true)
	cfg.ParentIsLocal = func(raw any) bool {
		s, ok := raw.(testSnap)
		return !ok || s.ID != "snap-copied"
	}
	fn := awsclient.EnrichSnapshotCrossRef(cfg)

	created := time.Now().Add(-3 * 24 * time.Hour)
	copied := testSnap{ID: "snap-copied", ParentID: "acme-orders", Type: "manual", CreatedAt: created}
	native := testSnap{ID: "snap-native", ParentID: "acme-orders", Type: "manual", CreatedAt: created}
	cache := parentCache(false, testParent{ID: "acme-billing", BackupRetentionPeriod: 7})

	res, err := fn(context.Background(), nil, []resource.Resource{snapRes(copied), snapRes(native)}, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs := res.Findings["snap-copied"]; len(fs) != 0 {
		t.Errorf("snapshot whose parent is not local carries findings %+v; want none", fs)
	}
	if ad := res.AttentionDetails["snap-copied"]; len(ad) != 0 {
		t.Errorf("snapshot whose parent is not local carries attention rows %+v; want none", ad)
	}
	if !spHas(res.Findings["snap-native"], spDBISnapOrphan) {
		t.Errorf("snapshot whose parent is local and missing lost its orphan finding: %+v", res.Findings["snap-native"])
	}
}

const (
	spAccount      = "123456789012"
	spOtherAccount = "111122223333"
)

func spDBISnap(id, instance string) rdstypes.DBSnapshot {
	return rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(id),
		DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:" + spAccount + ":snapshot:" + id),
		DBInstanceIdentifier: aws.String(instance),
		DbiResourceId:        aws.String("db-7QZ3KXAMPLEN4BLVW2HJPOEYRM"),
		Status:               aws.String("available"),
		Engine:               aws.String("postgres"),
		EngineVersion:        aws.String("16.2"),
		SnapshotType:         aws.String("manual"),
		SnapshotCreateTime:   aws.Time(time.Now().UTC().Add(-3 * 24 * time.Hour)),
		AllocatedStorage:     aws.Int32(100),
		StorageType:          aws.String("gp3"),
		Encrypted:            aws.Bool(true),
		AvailabilityZone:     aws.String("us-east-1a"),
		PercentProgress:      aws.Int32(100),
		SourceRegion:         aws.String("us-east-1"),
	}
}

func spDBIInstance(id, dbiResourceID string, retention int32) rdstypes.DBInstance {
	return rdstypes.DBInstance{
		DBInstanceIdentifier:  aws.String(id),
		DbiResourceId:         aws.String(dbiResourceID),
		DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:" + spAccount + ":db:" + id),
		DBInstanceStatus:      aws.String("available"),
		Engine:                aws.String("postgres"),
		BackupRetentionPeriod: aws.Int32(retention),
	}
}

func spEnrichDBISnap(t *testing.T, cache resource.ResourceCache, snaps ...rdstypes.DBSnapshot) awsclient.IssueEnricherResult {
	t.Helper()
	rs := make([]resource.Resource, 0, len(snaps))
	for _, s := range snaps {
		rs = append(rs, snapResource(s))
	}
	res, err := spEnricher(t, "dbi-snap")(context.Background(), nil, rs, cache)
	if err != nil {
		t.Fatalf("dbi-snap enricher: %v", err)
	}
	return res
}

// SourceRegion is populated on every RDS snapshot ("created in or copied
// from"), so a native snapshot carries its own region there.
// SourceDBSnapshotIdentifier has a value only for a cross-account or
// cross-region copy.
func TestDBISnap_CopiedSnapshotIsNeverOrphan(t *testing.T) {
	cache := dbiCacheWith([]rdstypes.DBInstance{spDBIInstance("acme-billing", "db-BILL1NGXAMPLE000000000001", 7)})

	crossRegion := spDBISnap("acme-orders-dr-2026-09-01", "acme-orders")
	crossRegion.SourceRegion = aws.String("us-west-2")
	crossRegion.SourceDBSnapshotIdentifier = aws.String("arn:aws:rds:us-west-2:" + spAccount + ":snapshot:acme-orders-2026-09-01")

	crossAccount := spDBISnap("acme-reports-from-partner", "partner-reports")
	crossAccount.SourceDBSnapshotIdentifier = aws.String("arn:aws:rds:us-east-1:" + spOtherAccount + ":snapshot:partner-reports-2026-08-30")

	native := spDBISnap("acme-orders-final", "acme-orders")

	res := spEnrichDBISnap(t, cache, crossRegion, crossAccount, native)

	for _, id := range []string{"acme-orders-dr-2026-09-01", "acme-reports-from-partner"} {
		if spHas(res.Findings[id], spDBISnapOrphan) {
			t.Errorf("%s is a copy whose parent lives in another region or account, yet it is flagged orphan: %+v", id, res.Findings[id])
		}
	}
	if !spHas(res.Findings["acme-orders-final"], spDBISnapOrphan) {
		t.Errorf("native snapshot of a deleted local instance (SourceRegion = its own region) lost its orphan finding: %+v", res.Findings["acme-orders-final"])
	}
}

func spDocDBClusterSnap(id, cluster string) docdbtypes.DBClusterSnapshot {
	return docdbtypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String(id),
		DBClusterIdentifier:         aws.String(cluster),
		DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:" + spAccount + ":cluster-snapshot:" + id),
		Status:                      aws.String("available"),
		Engine:                      aws.String("docdb"),
		EngineVersion:               aws.String("5.0.0"),
		SnapshotType:                aws.String("manual"),
		SnapshotCreateTime:          aws.Time(time.Now().UTC().Add(-3 * 24 * time.Hour)),
		PercentProgress:             aws.Int32(100),
		StorageEncrypted:            aws.Bool(true),
	}
}

func spAuroraClusterSnap(id, cluster string) rdstypes.DBClusterSnapshot {
	return rdstypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String(id),
		DBClusterIdentifier:         aws.String(cluster),
		DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:" + spAccount + ":cluster-snapshot:" + id),
		Status:                      aws.String("available"),
		Engine:                      aws.String("aurora-postgresql"),
		EngineVersion:               aws.String("16.4"),
		SnapshotType:                aws.String("manual"),
		SnapshotCreateTime:          aws.Time(time.Now().UTC().Add(-3 * 24 * time.Hour)),
		PercentProgress:             aws.Int32(100),
		StorageEncrypted:            aws.Bool(true),
	}
}

// SourceDBClusterSnapshotArn is set only when the cluster snapshot was copied
// from another cluster snapshot. The dbc-snap list carries both the DocumentDB
// and the Aurora SDK shape.
func TestDBCSnap_CopiedSnapshotIsNeverOrphan(t *testing.T) {
	cache := resource.ResourceCache{"dbc": resource.ResourceCacheEntry{Resources: []resource.Resource{
		{ID: "acme-billing-cluster", RawStruct: docdbtypes.DBCluster{
			DBClusterIdentifier:   aws.String("acme-billing-cluster"),
			BackupRetentionPeriod: aws.Int32(7),
		}},
	}}}

	docCopy := spDocDBClusterSnap("acme-ledger-dr-copy", "acme-ledger")
	docCopy.SourceDBClusterSnapshotArn = aws.String("arn:aws:rds:us-west-2:" + spAccount + ":cluster-snapshot:acme-ledger-2026-09-01")
	docNative := spDocDBClusterSnap("acme-ledger-final", "acme-ledger")

	auroraCopy := spAuroraClusterSnap("acme-events-from-partner", "partner-events")
	auroraCopy.SourceDBClusterSnapshotArn = aws.String("arn:aws:rds:us-east-1:" + spOtherAccount + ":cluster-snapshot:partner-events-2026-08-30")
	auroraNative := spAuroraClusterSnap("acme-events-final", "acme-events")

	rs := []resource.Resource{
		{ID: "acme-ledger-dr-copy", RawStruct: docCopy},
		{ID: "acme-ledger-final", RawStruct: docNative},
		{ID: "acme-events-from-partner", RawStruct: auroraCopy},
		{ID: "acme-events-final", RawStruct: auroraNative},
	}
	res, err := spEnricher(t, "dbc-snap")(context.Background(), nil, rs, cache)
	if err != nil {
		t.Fatalf("dbc-snap enricher: %v", err)
	}

	for _, id := range []string{"acme-ledger-dr-copy", "acme-events-from-partner"} {
		if spHas(res.Findings[id], spDBCSnapOrphan) {
			t.Errorf("%s carries SourceDBClusterSnapshotArn (a copy), yet it is flagged orphan: %+v", id, res.Findings[id])
		}
	}
	for _, id := range []string{"acme-ledger-final", "acme-events-final"} {
		if !spHas(res.Findings[id], spDBCSnapOrphan) {
			t.Errorf("%s is a native snapshot of a deleted local cluster and lost its orphan finding: %+v", id, res.Findings[id])
		}
	}
}

// AWS gives a snapshot made by CopySnapshot the arbitrary volume ID
// vol-ffffffff, which "you should not use for any purpose" — no volume list
// will ever contain it.
func TestEBSSnap_CopiedSnapshotIsNeverOrphan(t *testing.T) {
	copied := pw1Snapshot("snap-0c0p1ed00aa11bb22", "vol-ffffffff")
	copied.Description = aws.String("[Copied snap-0a1b2c3d4e5f60718 from us-west-2]")
	native := pw1Snapshot("snap-0de1e7ed0aa11bb22", "vol-0deleted111bbbb2")

	res := pw1EnrichEBSSnap(t, &pw1EBSSnapFake{}, pw1EBSCache("vol-0aaaa1111bbbb2222"), copied, native)

	if spHas(res.Findings["snap-0c0p1ed00aa11bb22"], spEBSSnapOrphan) {
		t.Errorf("snapshot copy (volume vol-ffffffff) is flagged orphan: %+v", res.Findings["snap-0c0p1ed00aa11bb22"])
	}
	if !spHas(res.Findings["snap-0de1e7ed0aa11bb22"], spEBSSnapOrphan) {
		t.Errorf("snapshot of a deleted local volume lost its orphan finding: %+v", res.Findings["snap-0de1e7ed0aa11bb22"])
	}
}

const (
	spOrdersOriginalResID = "db-ORDERSXAMPLE0RIG1NAL00001"
	spOrdersNewResID      = "db-ORDERSXAMPLENEWINSTANCE02"
)

// ModifyDBInstance --new-db-instance-identifier renames the instance; its
// DbiResourceId does not change, and existing snapshots keep the old name.
func TestDBISnap_SnapshotOfRenamedInstanceIsNotOrphan(t *testing.T) {
	cache := dbiCacheWith([]rdstypes.DBInstance{spDBIInstance("acme-orders-v2", spOrdersOriginalResID, 7)})

	ofRenamed := spDBISnap("acme-orders-pre-rename", "acme-orders")
	ofRenamed.DbiResourceId = aws.String(spOrdersOriginalResID)
	ofDeleted := spDBISnap("acme-carts-final", "acme-carts")
	ofDeleted.DbiResourceId = aws.String("db-CARTSXAMPLEDELETED0000003")

	res := spEnrichDBISnap(t, cache, ofRenamed, ofDeleted)

	if spHas(res.Findings["acme-orders-pre-rename"], spDBISnapOrphan) {
		t.Errorf("snapshot of an instance renamed to acme-orders-v2 (same DbiResourceId) is flagged orphan: %+v", res.Findings["acme-orders-pre-rename"])
	}
	if !spHas(res.Findings["acme-carts-final"], spDBISnapOrphan) {
		t.Errorf("snapshot whose DbiResourceId and name both match nothing lost its orphan finding: %+v", res.Findings["acme-carts-final"])
	}
}

// A snapshot's parent is the instance with its DbiResourceId, even when a
// newer instance has taken the old DBInstanceIdentifier.
func TestDBISnap_DbiResourceIdWinsOverAReusedName(t *testing.T) {
	cache := dbiCacheWith([]rdstypes.DBInstance{
		spDBIInstance("acme-orders-v2", spOrdersOriginalResID, 7),
		spDBIInstance("acme-orders", spOrdersNewResID, 35),
	})

	snap := spDBISnap("rds:acme-orders-2026-08-30-03-10", "acme-orders")
	snap.DbiResourceId = aws.String(spOrdersOriginalResID)
	snap.SnapshotType = aws.String("automated")
	snap.SnapshotCreateTime = aws.Time(time.Now().UTC().Add(-20 * 24 * time.Hour))

	res := spEnrichDBISnap(t, cache, snap)

	fs := res.Findings["rds:acme-orders-2026-08-30-03-10"]
	if !spHas(fs, spDBISnapPastRetention) {
		t.Errorf("automated snapshot 20 days old, parent (by DbiResourceId) retains 7 days: want %s, got %+v", spDBISnapPastRetention, fs)
	}
	if spHas(fs, spDBISnapOrphan) {
		t.Errorf("snapshot whose parent exists under a new name is flagged orphan: %+v", fs)
	}
}

// A snapshot without DbiResourceId still resolves by DBInstanceIdentifier.
func TestDBISnap_NoDbiResourceIdFallsBackToName(t *testing.T) {
	cache := dbiCacheWith([]rdstypes.DBInstance{spDBIInstance("acme-orders", spOrdersNewResID, 7)})
	snap := spDBISnap("acme-orders-manual", "acme-orders")
	snap.DbiResourceId = nil

	res := spEnrichDBISnap(t, cache, snap)

	if spHas(res.Findings["acme-orders-manual"], spDBISnapOrphan) {
		t.Errorf("snapshot without DbiResourceId whose instance name is in the list is flagged orphan: %+v", res.Findings["acme-orders-manual"])
	}
}

// DbiResourceId is unique to one instance for its whole life, so a snapshot
// whose DbiResourceId matches no instance has lost its parent even when a
// newer instance now carries the old DBInstanceIdentifier.
func TestDBISnap_UnmatchedDbiResourceIdIsOrphanDespiteReusedName(t *testing.T) {
	cache := dbiCacheWith([]rdstypes.DBInstance{spDBIInstance("acme-orders", spOrdersNewResID, 7)})
	snap := spDBISnap("acme-orders-before-rebuild", "acme-orders")
	snap.DbiResourceId = aws.String(spOrdersOriginalResID)

	res := spEnrichDBISnap(t, cache, snap)

	if !spHas(res.Findings["acme-orders-before-rebuild"], spDBISnapOrphan) {
		t.Errorf("snapshot of deleted instance %s is not orphan because a newer instance reused its name: %+v", spOrdersOriginalResID, res.Findings["acme-orders-before-rebuild"])
	}
}
