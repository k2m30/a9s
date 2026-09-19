package unit_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
	unit "github.com/k2m30/a9s/v3/tests/unit"
)

func spPivot(t *testing.T, from, to string, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	t.Helper()
	i := slices.IndexFunc(resource.GetRelated(from), func(d resource.RelatedDef) bool { return d.TargetType == to })
	if i < 0 || resource.GetRelated(from)[i].Checker == nil {
		t.Fatalf("no %s → %s related checker", from, to)
	}
	return resource.GetRelated(from)[i].Checker(context.Background(), spSessionClients(), res, cache)
}

func spWantIDs(t *testing.T, label string, got resource.RelatedCheckResult, want ...string) {
	t.Helper()
	if got.State() != domain.RelatedResolved || got.Count() != len(want) || !slices.Equal(got.ResourceIDs(), want) {
		t.Errorf("%s: state %v count %d ids %v; want resolved %v", label, got.State(), got.Count(), got.ResourceIDs(), want)
	}
}

// A copied snapshot's source lives in another region or account; a local
// row that happens to share the source's name is not its parent.
func TestSnapshotPivots_DBCSnapCopyNeverMatchesSameNamedLocalCluster(t *testing.T) {
	created := aws.Time(time.Now().UTC().Add(-3 * 24 * time.Hour))
	cases := []struct {
		shape   string
		cluster any
		copied  any
		native  any
	}{
		{
			shape: "docdb",
			cluster: docdbtypes.DBCluster{
				DBClusterIdentifier: aws.String("acme-ledger"),
				DBClusterArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster:acme-ledger"),
				Engine:              aws.String("docdb"),
				Status:              aws.String("available"),
			},
			copied: docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("acme-ledger-dr-copy"),
				DBClusterIdentifier:         aws.String("acme-ledger"),
				DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:acme-ledger-dr-copy"),
				SourceDBClusterSnapshotArn:  aws.String("arn:aws:rds:us-west-2:123456789012:cluster-snapshot:acme-ledger-2026-09-01"),
				Engine:                      aws.String("docdb"),
				SnapshotType:                aws.String("manual"),
				Status:                      aws.String("available"),
				SnapshotCreateTime:          created,
			},
			native: docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("acme-ledger-pre-upgrade"),
				DBClusterIdentifier:         aws.String("acme-ledger"),
				DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:acme-ledger-pre-upgrade"),
				Engine:                      aws.String("docdb"),
				SnapshotType:                aws.String("manual"),
				Status:                      aws.String("available"),
				SnapshotCreateTime:          created,
			},
		},
		{
			shape: "aurora",
			cluster: rdstypes.DBCluster{
				DBClusterIdentifier: aws.String("acme-ledger"),
				DBClusterArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster:acme-ledger"),
				Engine:              aws.String("aurora-postgresql"),
				Status:              aws.String("available"),
			},
			copied: rdstypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("acme-ledger-dr-copy"),
				DBClusterIdentifier:         aws.String("acme-ledger"),
				DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:acme-ledger-dr-copy"),
				SourceDBClusterSnapshotArn:  aws.String("arn:aws:rds:us-west-2:123456789012:cluster-snapshot:acme-ledger-2026-09-01"),
				Engine:                      aws.String("aurora-postgresql"),
				SnapshotType:                aws.String("manual"),
				Status:                      aws.String("available"),
				SnapshotCreateTime:          created,
			},
			native: rdstypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("acme-ledger-pre-upgrade"),
				DBClusterIdentifier:         aws.String("acme-ledger"),
				DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:acme-ledger-pre-upgrade"),
				Engine:                      aws.String("aurora-postgresql"),
				SnapshotType:                aws.String("manual"),
				Status:                      aws.String("available"),
				SnapshotCreateTime:          created,
			},
		},
	}
	for _, c := range cases {
		t.Run(c.shape, func(t *testing.T) {
			cluster := resource.Resource{ID: "acme-ledger", Name: "acme-ledger", Fields: map[string]string{"cluster_id": "acme-ledger"}, RawStruct: c.cluster}
			copied := resource.Resource{ID: "acme-ledger-dr-copy", Name: "acme-ledger-dr-copy", Fields: map[string]string{"cluster_id": "acme-ledger"}, RawStruct: c.copied}
			native := resource.Resource{ID: "acme-ledger-pre-upgrade", Name: "acme-ledger-pre-upgrade", Fields: map[string]string{"cluster_id": "acme-ledger"}, RawStruct: c.native}
			cache := resource.ResourceCache{
				"dbc":      resource.ResourceCacheEntry{Resources: []resource.Resource{cluster}},
				"dbc-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{copied, native}},
			}

			spWantIDs(t, "copy → dbc", spPivot(t, "dbc-snap", "dbc", copied, cache))
			spWantIDs(t, "native → dbc", spPivot(t, "dbc-snap", "dbc", native, cache), "acme-ledger")
			spWantIDs(t, "dbc → dbc-snap", spPivot(t, "dbc", "dbc-snap", cluster, cache), "acme-ledger-pre-upgrade")
		})
	}
}

// SourceDBSnapshotIdentifier is set only on a cross-account or cross-region
// copy. A snapshot without DbiResourceId is matched to its instance by name.
func TestSnapshotPivots_DBISnapCopyNeverMatchesSameNamedLocalInstance(t *testing.T) {
	created := aws.Time(time.Now().UTC().Add(-3 * 24 * time.Hour))
	instance := resource.Resource{ID: "acme-orders", Name: "acme-orders", RawStruct: rdstypes.DBInstance{
		DBInstanceIdentifier: aws.String("acme-orders"),
		DbiResourceId:        aws.String("db-ORDERSXAMPLE0RIG1NAL00001"),
		DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:acme-orders"),
		DBInstanceStatus:     aws.String("available"),
		Engine:               aws.String("postgres"),
	}}
	snap := func(id string) rdstypes.DBSnapshot {
		return rdstypes.DBSnapshot{
			DBSnapshotIdentifier: aws.String(id),
			DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:" + id),
			DBInstanceIdentifier: aws.String("acme-orders"),
			Engine:               aws.String("postgres"),
			SnapshotType:         aws.String("manual"),
			Status:               aws.String("available"),
			SnapshotCreateTime:   created,
			SourceRegion:         aws.String("us-east-1"),
		}
	}
	copiedRaw := snap("acme-orders-dr-2026-09-01")
	copiedRaw.SourceRegion = aws.String("us-west-2")
	copiedRaw.SourceDBSnapshotIdentifier = aws.String("arn:aws:rds:us-west-2:123456789012:snapshot:acme-orders-2026-09-01")
	copied := resource.Resource{ID: "acme-orders-dr-2026-09-01", Name: "acme-orders-dr-2026-09-01", Fields: map[string]string{"db_instance_identifier": "acme-orders"}, RawStruct: copiedRaw}
	native := resource.Resource{ID: "acme-orders-pre-upgrade", Name: "acme-orders-pre-upgrade", Fields: map[string]string{"db_instance_identifier": "acme-orders"}, RawStruct: snap("acme-orders-pre-upgrade")}
	cache := resource.ResourceCache{
		"dbi":      resource.ResourceCacheEntry{Resources: []resource.Resource{instance}},
		"dbi-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{copied, native}},
	}

	spWantIDs(t, "copy → dbi", spPivot(t, "dbi-snap", "dbi", copied, cache))
	spWantIDs(t, "native → dbi", spPivot(t, "dbi-snap", "dbi", native, cache), "acme-orders")
	spWantIDs(t, "dbi → dbi-snap", spPivot(t, "dbi", "dbi-snap", instance, cache), "acme-orders-pre-upgrade")
}

// CopySnapshot gives the new snapshot the arbitrary volume ID vol-ffffffff,
// which names no volume.
func TestSnapshotPivots_EBSSnapCopyResolvesNoVolume(t *testing.T) {
	volume := resource.Resource{ID: "vol-0aaaa1111bbbb2222", Name: "vol-0aaaa1111bbbb2222", Fields: map[string]string{"volume_id": "vol-0aaaa1111bbbb2222"}, RawStruct: ec2types.Volume{
		VolumeId: aws.String("vol-0aaaa1111bbbb2222"),
		State:    ec2types.VolumeStateInUse,
		Size:     aws.Int32(100),
	}}
	ebsSnap := func(id, vol string) resource.Resource {
		return resource.Resource{ID: id, Name: id, Fields: map[string]string{"volume_id": vol}, RawStruct: ec2types.Snapshot{
			SnapshotId: aws.String(id),
			VolumeId:   aws.String(vol),
			State:      ec2types.SnapshotStateCompleted,
			OwnerId:    aws.String("123456789012"),
		}}
	}
	copied := ebsSnap("snap-0c0p1ed00aa11bb22", "vol-ffffffff")
	native := ebsSnap("snap-0a1b2c3d4e5f60718", "vol-0aaaa1111bbbb2222")
	cache := resource.ResourceCache{
		"ebs":      resource.ResourceCacheEntry{Resources: []resource.Resource{volume}},
		"ebs-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{copied, native}},
	}

	spWantIDs(t, "copy → ebs", spPivot(t, "ebs-snap", "ebs", copied, cache))
	spWantIDs(t, "native → ebs", spPivot(t, "ebs-snap", "ebs", native, cache), "vol-0aaaa1111bbbb2222")
	spWantIDs(t, "ebs → ebs-snap", spPivot(t, "ebs", "ebs-snap", volume, cache), "snap-0a1b2c3d4e5f60718")
}

const (
	spRegion     = "us-east-1"
	spAccountID  = "123456789012"
	spLedgerARN  = "arn:aws:rds:us-east-1:123456789012:cluster:acme-ledger"
	spLedgerPlan = "plan-ledger-daily"
)

func spSessionClients() *awsclient.ServiceClients {
	c := &awsclient.ServiceClients{Region: spRegion}
	store := session.NewIdentityStore()
	store.Set(spAccountID, nil)
	c.SetIdentityStore(store)
	return c
}

func spFindings(t *testing.T, typ string, rows []resource.Resource, cache resource.ResourceCache) map[string][]domain.Finding {
	t.Helper()
	return spEnrich(t, typ, rows, cache).Findings
}

func spEnrich(t *testing.T, typ string, rows []resource.Resource, cache resource.ResourceCache) awsclient.IssueEnricherResult {
	t.Helper()
	e, ok := awsclient.Wave2EnricherFor(typ)
	if !ok || e.Fn == nil {
		t.Fatalf("no Wave 2 enricher registered for %s", typ)
	}
	res, err := e.Fn(context.Background(), spSessionClients(), rows, cache)
	if err != nil {
		t.Fatalf("%s enricher: %v", typ, err)
	}
	return res
}

// A cluster generation is one life of the name acme-ledger: deleting the
// cluster and creating a new one under the same name starts a new generation
// with a new DbClusterResourceId and a new ClusterCreateTime.
type spClusterShape struct {
	name    string
	cluster func(gen string) resource.Resource
	snap    func(id, gen, sourceARN string) resource.Resource
}

var spGenCreated = map[string]time.Time{
	"old": time.Date(2024, 1, 10, 9, 0, 0, 0, time.UTC),
	"new": time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC),
}

var spGenResourceID = map[string]string{
	"old": "cluster-LEDGERXAMPLE0LDGEN000001",
	"new": "cluster-LEDGERXAMPLENEWGEN000002",
}

func spClusterShapes() []spClusterShape {
	snapTime := aws.Time(time.Now().UTC().Add(-3 * 24 * time.Hour))
	fields := map[string]string{"cluster_id": "acme-ledger"}
	return []spClusterShape{
		{
			name: "docdb",
			cluster: func(gen string) resource.Resource {
				return resource.Resource{ID: "acme-ledger", Name: "acme-ledger", Fields: fields, RawStruct: docdbtypes.DBCluster{
					DBClusterIdentifier:   aws.String("acme-ledger"),
					DBClusterArn:          aws.String(spLedgerARN),
					DbClusterResourceId:   aws.String(spGenResourceID[gen]),
					ClusterCreateTime:     aws.Time(spGenCreated[gen]),
					Engine:                aws.String("docdb"),
					Status:                aws.String("available"),
					BackupRetentionPeriod: aws.Int32(7),
				}}
			},
			snap: func(id, gen, sourceARN string) resource.Resource {
				raw := docdbtypes.DBClusterSnapshot{
					DBClusterSnapshotIdentifier: aws.String(id),
					DBClusterIdentifier:         aws.String("acme-ledger"),
					DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:" + id),
					ClusterCreateTime:           aws.Time(spGenCreated[gen]),
					Engine:                      aws.String("docdb"),
					SnapshotType:                aws.String("manual"),
					Status:                      aws.String("available"),
					SnapshotCreateTime:          snapTime,
				}
				if sourceARN != "" {
					raw.SourceDBClusterSnapshotArn = aws.String(sourceARN)
				}
				return resource.Resource{ID: id, Name: id, Fields: fields, RawStruct: raw}
			},
		},
		{
			name: "aurora",
			cluster: func(gen string) resource.Resource {
				return resource.Resource{ID: "acme-ledger", Name: "acme-ledger", Fields: fields, RawStruct: rdstypes.DBCluster{
					DBClusterIdentifier:   aws.String("acme-ledger"),
					DBClusterArn:          aws.String(spLedgerARN),
					DbClusterResourceId:   aws.String(spGenResourceID[gen]),
					ClusterCreateTime:     aws.Time(spGenCreated[gen]),
					Engine:                aws.String("aurora-postgresql"),
					Status:                aws.String("available"),
					BackupRetentionPeriod: aws.Int32(7),
				}}
			},
			snap: func(id, gen, sourceARN string) resource.Resource {
				raw := rdstypes.DBClusterSnapshot{
					DBClusterSnapshotIdentifier: aws.String(id),
					DBClusterIdentifier:         aws.String("acme-ledger"),
					DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:" + id),
					DbClusterResourceId:         aws.String(spGenResourceID[gen]),
					ClusterCreateTime:           aws.Time(spGenCreated[gen]),
					Engine:                      aws.String("aurora-postgresql"),
					SnapshotType:                aws.String("manual"),
					Status:                      aws.String("available"),
					SnapshotCreateTime:          snapTime,
				}
				if sourceARN != "" {
					raw.SourceDBClusterSnapshotArn = aws.String(sourceARN)
				}
				return resource.Resource{ID: id, Name: id, Fields: fields, RawStruct: raw}
			},
		},
	}
}

// A snapshot belongs to the cluster generation it was taken from, not to
// whichever cluster carries that name today.
func TestSnapshotParent_RecreatedClusterDoesNotAdoptOldSnapshots(t *testing.T) {
	for _, sh := range spClusterShapes() {
		t.Run(sh.name, func(t *testing.T) {
			cluster := sh.cluster("new")
			old := sh.snap("acme-ledger-before-rebuild", "old", "")
			current := sh.snap("acme-ledger-pre-upgrade", "new", "")
			cache := resource.ResourceCache{
				"dbc":      resource.ResourceCacheEntry{Resources: []resource.Resource{cluster}},
				"dbc-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{old, current}},
				"backup": resource.ResourceCacheEntry{Resources: []resource.Resource{
					unit.BackupPlanRow(t, spLedgerPlan, backuptypes.BackupSelection{Resources: []string{spLedgerARN}}),
				}},
			}

			fs := spFindings(t, "dbc-snap", []resource.Resource{old, current}, cache)
			if !spHas(fs[old.ID], "dbc-snap.orphan") {
				t.Errorf("snapshot of the deleted generation is not orphan: %+v", fs[old.ID])
			}
			if spHas(fs[current.ID], "dbc-snap.orphan") {
				t.Errorf("snapshot of the live generation is orphan: %+v", fs[current.ID])
			}

			spWantIDs(t, "old → dbc", spPivot(t, "dbc-snap", "dbc", old, cache))
			spWantIDs(t, "old → backup", spPivot(t, "dbc-snap", "backup", old, cache))
			spWantIDs(t, "current → dbc", spPivot(t, "dbc-snap", "dbc", current, cache), "acme-ledger")
			spWantIDs(t, "current → backup", spPivot(t, "dbc-snap", "backup", current, cache), spLedgerPlan)
			spWantIDs(t, "dbc → dbc-snap", spPivot(t, "dbc", "dbc-snap", cluster, cache), current.ID)
		})
	}
}

// AWS sets SourceDBClusterSnapshotArn on every copied cluster snapshot. A copy
// made in the session's own Region and account keeps its parent in the local
// cluster list.
func TestSnapshotParent_SameRegionClusterCopyResolvesItsCluster(t *testing.T) {
	const sourceARN = "arn:aws:rds:us-east-1:123456789012:cluster-snapshot:rds:acme-ledger-2026-09-15-04-00"
	for _, sh := range spClusterShapes() {
		t.Run(sh.name, func(t *testing.T) {
			cluster := sh.cluster("new")
			copied := sh.snap("acme-ledger-keep-2026-09", "new", sourceARN)
			cache := resource.ResourceCache{
				"dbc":      resource.ResourceCacheEntry{Resources: []resource.Resource{cluster}},
				"dbc-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{copied}},
			}

			spWantIDs(t, "copy → dbc", spPivot(t, "dbc-snap", "dbc", copied, cache), "acme-ledger")
			spWantIDs(t, "dbc → dbc-snap", spPivot(t, "dbc", "dbc-snap", cluster, cache), copied.ID)
			if fs := spFindings(t, "dbc-snap", []resource.Resource{copied}, cache); spHas(fs[copied.ID], "dbc-snap.orphan") {
				t.Errorf("same-Region copy of a live cluster is orphan: %+v", fs[copied.ID])
			}

			gone := resource.ResourceCache{"dbc": resource.ResourceCacheEntry{Resources: []resource.Resource{
				{ID: "acme-billing", Name: "acme-billing", RawStruct: docdbtypes.DBCluster{DBClusterIdentifier: aws.String("acme-billing")}},
			}}}
			if fs := spFindings(t, "dbc-snap", []resource.Resource{copied}, gone); !spHas(fs[copied.ID], "dbc-snap.orphan") {
				t.Errorf("same-Region copy whose cluster is gone is not orphan: %+v", fs[copied.ID])
			}
		})
	}
}

func spDBIRow(id, resID string) resource.Resource {
	return resource.Resource{ID: id, Name: id, Fields: map[string]string{"db_instance_identifier": id}, RawStruct: rdstypes.DBInstance{
		DBInstanceIdentifier: aws.String(id),
		DbiResourceId:        aws.String(resID),
		DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:" + id),
		DBInstanceStatus:     aws.String("available"),
		Engine:               aws.String("postgres"),
	}}
}

func spDBISnapRow(id, instance, resID, source string) resource.Resource {
	raw := rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(id),
		DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:" + id),
		DBInstanceIdentifier: aws.String(instance),
		DbiResourceId:        aws.String(resID),
		Engine:               aws.String("postgres"),
		SnapshotType:         aws.String("manual"),
		Status:               aws.String("available"),
		SnapshotCreateTime:   aws.Time(time.Now().UTC().Add(-3 * 24 * time.Hour)),
		SourceRegion:         aws.String("us-east-1"),
	}
	if source != "" {
		raw.SourceRegion = aws.String("us-west-2")
		raw.SourceDBSnapshotIdentifier = aws.String(source)
	}
	return resource.Resource{ID: id, Name: id, Fields: map[string]string{"db_instance_identifier": instance}, RawStruct: raw}
}

// Enter on the snapshot's DBInstanceIdentifier opens the instance the
// snapshot was taken from, which the DB Instances related row counts; when
// that instance is not in the list, the field opens nothing.
func TestSnapshotParent_DBISnapInstanceFieldOpensTheParent(t *testing.T) {
	cases := []struct {
		name      string
		instances []resource.Resource
		snap      resource.Resource
		want      string
	}{
		{
			name:      "native snapshot of a live instance",
			instances: []resource.Resource{spDBIRow("acme-orders", "db-ORDERSXAMPLE0RIG1NAL00001")},
			snap:      spDBISnapRow("acme-orders-pre-upgrade", "acme-orders", "db-ORDERSXAMPLE0RIG1NAL00001", ""),
			want:      "acme-orders",
		},
		{
			name:      "instance renamed after the snapshot",
			instances: []resource.Resource{spDBIRow("acme-orders-v2", "db-ORDERSXAMPLE0RIG1NAL00001")},
			snap:      spDBISnapRow("acme-orders-pre-rename", "acme-orders", "db-ORDERSXAMPLE0RIG1NAL00001", ""),
			want:      "acme-orders-v2",
		},
		{
			name:      "cross-Region copy next to a same-named local instance",
			instances: []resource.Resource{spDBIRow("acme-orders", "db-ORDERSXAMPLE0RIG1NAL00001")},
			snap:      spDBISnapRow("acme-orders-dr-2026-09-01", "acme-orders", "db-ORDERSWESTXAMPLE000000003", "arn:aws:rds:us-west-2:123456789012:snapshot:acme-orders-2026-09-01"),
		},
		{
			name:      "instance recreated under the old name",
			instances: []resource.Resource{spDBIRow("acme-orders", "db-ORDERSXAMPLENEWINSTANCE02")},
			snap:      spDBISnapRow("acme-orders-before-rebuild", "acme-orders", "db-ORDERSXAMPLE0RIG1NAL00001", ""),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := refBench{byType: map[string][]resource.Resource{"dbi": tc.instances, "dbi-snap": {tc.snap}}}
			f := fieldAt(t, openDetail(refDetailController(t, b), "dbi-snap", tc.snap), "DBInstanceIdentifier")
			switch {
			case tc.want == "" && f.IsNavigable:
				t.Errorf("DBInstanceIdentifier is navigable to %s %q; want not navigable", f.TargetType, navTarget(f))
			case tc.want != "" && (!f.IsNavigable || f.TargetType != "dbi" || navTarget(f) != tc.want):
				t.Errorf("DBInstanceIdentifier: navigable=%v target=%q opens %q; want dbi %q", f.IsNavigable, f.TargetType, navTarget(f), tc.want)
			}
		})
	}
}

// CreateImage writes "Created by CreateImage(<instance>) for <ami>" into the
// snapshot description; the instance is often terminated since.
func TestSnapshotParent_EBSSnapCreateImageInstanceMustBeLoaded(t *testing.T) {
	snap := resource.Resource{ID: "snap-0a1b2c3d4e5f60718", Name: "snap-0a1b2c3d4e5f60718",
		Fields: map[string]string{
			"volume_id":   "vol-0aaaa1111bbbb2222",
			"description": "Created by CreateImage(i-0abc1234def567890) for ami-0fedcba9876543210",
		},
		RawStruct: ec2types.Snapshot{
			SnapshotId:  aws.String("snap-0a1b2c3d4e5f60718"),
			VolumeId:    aws.String("vol-0aaaa1111bbbb2222"),
			Description: aws.String("Created by CreateImage(i-0abc1234def567890) for ami-0fedcba9876543210"),
			State:       ec2types.SnapshotStateCompleted,
			OwnerId:     aws.String("123456789012"),
		},
	}
	instance := func(id string) resource.Resource {
		return resource.Resource{ID: id, Name: id, RawStruct: ec2types.Instance{
			InstanceId: aws.String(id),
			State:      &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
		}}
	}
	ec2 := func(truncated bool, ids ...string) resource.ResourceCache {
		rows := make([]resource.Resource, 0, len(ids))
		for _, id := range ids {
			rows = append(rows, instance(id))
		}
		return resource.ResourceCache{"ec2": resource.ResourceCacheEntry{Resources: rows, IsTruncated: truncated}}
	}

	spWantIDs(t, "instance loaded", spPivot(t, "ebs-snap", "ec2", snap, ec2(false, "i-0abc1234def567890", "i-0fff0000aaaa11112")), "i-0abc1234def567890")
	spWantIDs(t, "instance not in a complete list", spPivot(t, "ebs-snap", "ec2", snap, ec2(false, "i-0fff0000aaaa11112")))
	if got := spPivot(t, "ebs-snap", "ec2", snap, resource.ResourceCache{}); got.State() != domain.RelatedUnknown {
		t.Errorf("ec2 list absent: state %v count %d ids %v; want unknown", got.State(), got.Count(), got.ResourceIDs())
	}
	got := spPivot(t, "ebs-snap", "ec2", snap, ec2(true, "i-0fff0000aaaa11112"))
	spWantIDs(t, "ec2 list truncated", got)
	if !got.Truncated() {
		t.Errorf("ec2 list truncated: Truncated() = false; want the lower bound 0+")
	}
}

func spSourceRow(t *testing.T, res awsclient.IssueEnricherResult, id string, code domain.FindingCode, label string) string {
	t.Helper()
	if !spHas(res.Findings[id], code) {
		t.Fatalf("%s carries no %s: %+v", id, code, res.Findings[id])
	}
	for _, row := range res.AttentionDetails[id][code].Rows {
		if row.Label == label {
			return row.Value
		}
	}
	t.Fatalf("%s %s has no %q row: %+v", id, code, label, res.AttentionDetails[id][code])
	return ""
}

// An orphan whose parent name now belongs to a newer generation cannot be
// described as absent from the list: the operator sees a row with that name.
func TestSnapshotParent_OrphanSourceRowNamesTheNewerGeneration(t *testing.T) {
	for _, sh := range spClusterShapes() {
		t.Run(sh.name, func(t *testing.T) {
			old := sh.snap("acme-ledger-before-rebuild", "old", "")
			gone := sh.snap("acme-archive-final", "old", "")
			switch raw := gone.RawStruct.(type) {
			case docdbtypes.DBClusterSnapshot:
				raw.DBClusterIdentifier = aws.String("acme-archive")
				gone.RawStruct = raw
			case rdstypes.DBClusterSnapshot:
				raw.DBClusterIdentifier = aws.String("acme-archive")
				gone.RawStruct = raw
			}
			gone.Fields = map[string]string{"cluster_id": "acme-archive"}
			cache := resource.ResourceCache{"dbc": resource.ResourceCacheEntry{Resources: []resource.Resource{sh.cluster("new")}}}

			res := spEnrich(t, "dbc-snap", []resource.Resource{old, gone}, cache)

			if got := spSourceRow(t, res, old.ID, "dbc-snap.orphan", "Source Cluster"); got != "acme-ledger (a newer cluster has this name)" {
				t.Errorf("Source Cluster = %q; want %q", got, "acme-ledger (a newer cluster has this name)")
			}
			if got := spSourceRow(t, res, gone.ID, "dbc-snap.orphan", "Source Cluster"); got != "acme-archive (not in loaded list)" {
				t.Errorf("Source Cluster = %q; want %q", got, "acme-archive (not in loaded list)")
			}
		})
	}

	t.Run("dbi", func(t *testing.T) {
		old := spDBISnapRow("acme-orders-before-rebuild", "acme-orders", "db-ORDERSXAMPLE0RIG1NAL00001", "")
		gone := spDBISnapRow("acme-carts-final", "acme-carts", "db-CARTSXAMPLEDELETED0000003", "")
		cache := resource.ResourceCache{"dbi": resource.ResourceCacheEntry{Resources: []resource.Resource{
			spDBIRow("acme-orders", "db-ORDERSXAMPLENEWINSTANCE02"),
		}}}

		res := spEnrich(t, "dbi-snap", []resource.Resource{old, gone}, cache)

		if got := spSourceRow(t, res, old.ID, "dbi-snap.orphan", "Source DB"); got != "acme-orders (a newer instance has this name)" {
			t.Errorf("Source DB = %q; want %q", got, "acme-orders (a newer instance has this name)")
		}
		if got := spSourceRow(t, res, gone.ID, "dbi-snap.orphan", "Source DB"); got != "acme-carts (not in loaded list)" {
			t.Errorf("Source DB = %q; want %q", got, "acme-carts (not in loaded list)")
		}
	})
}
