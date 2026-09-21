// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// A snapshot made by CopySnapshot carries the placeholder volume id
// vol-ffffffff, which names no volume in any account. The Source Volume row
// has to be a proven zero rather than a dead reference, and the snapshot is
// not an orphan for having one.
func TestDemoEBSSnapCopyHasNoSourceVolume(t *testing.T) {
	const copyID = "snap-0copy00000000a1c"

	var copySnap ec2types.Snapshot
	for _, s := range fixtures.NewEC2Fixtures().Snapshots {
		if aws.ToString(s.SnapshotId) == copyID {
			copySnap = s
		}
	}
	if copySnap.SnapshotId == nil {
		t.Fatalf("%s is not in the demo snapshot fixtures", copyID)
	}
	if got := aws.ToString(copySnap.VolumeId); got != "vol-ffffffff" {
		t.Fatalf("%s VolumeId=%q, want vol-ffffffff — CopySnapshot's placeholder is what the "+
			"row under test turns on", copyID, got)
	}

	row := demoRowByID(t, "ebs-snap", copyID)
	def := demoPivotDef(t, "ebs-snap", "ebs")
	r := def.Checker(context.Background(), demo.NewServiceClients(), row, resource.ResourceCache{})
	if r.EffectiveState() != domain.RelatedResolved || r.Count() != 0 {
		t.Errorf("%s -> ebs resolved %v count %d, want a resolved zero — vol-ffffffff is no volume",
			copyID, r.EffectiveState(), r.Count())
	}
}

// CreateImage writes the instance id into the snapshot description and nothing
// rewrites it when that instance is terminated, so the EC2 Instances row on an
// old image's snapshot has to answer none rather than a stale id.
func TestDemoEBSSnapCreateImageInstanceIsGone(t *testing.T) {
	const goneID = "snap-0gone00000000b2d"
	const goneInstance = "i-0f9e8d7c6b5a40012"

	clients := demo.NewServiceClients()
	ec2td := resource.FindResourceType("ec2")
	instances, ok := DrainFixtures(t, *ec2td, clients)
	if !ok {
		t.Fatal("ec2 declares no Fetcher")
	}
	for _, i := range instances {
		if i.ID == goneInstance {
			t.Fatalf("%s is in the demo instance list; the snapshot's description would resolve "+
				"and the terminated case would not be shown", goneInstance)
		}
	}

	row := demoRowByID(t, "ebs-snap", goneID)
	cache := resource.ResourceCache{"ec2": resource.ResourceCacheEntry{Resources: instances}}

	def := demoPivotDef(t, "ebs-snap", "ec2")
	r := def.Checker(context.Background(), clients, row, cache)
	if r.EffectiveState() != domain.RelatedResolved || r.Count() != 0 {
		t.Errorf("%s -> ec2 resolved %v count %d ids %v, want a resolved zero — %s is terminated",
			goneID, r.EffectiveState(), r.Count(), r.ResourceIDs(), goneInstance)
	}
}

// A snapshot that carries DbiResourceId is matched on it alone, because AWS
// keeps that id with the instance through a rename and never gives it to
// another one.
func TestDemoDBISnapWithResourceIDResolvesItsInstance(t *testing.T) {
	var snap rdstypes.DBSnapshot
	for _, s := range fixtures.NewDBISnapFixtures().Instances {
		if aws.ToString(s.DBSnapshotIdentifier) == fixtures.ResourceIDDBISnapID {
			snap = s
		}
	}
	if snap.DBSnapshotIdentifier == nil {
		t.Fatalf("%s is not in the demo dbi-snap fixtures", fixtures.ResourceIDDBISnapID)
	}
	if aws.ToString(snap.DbiResourceId) == "" {
		t.Fatalf("%s carries no DbiResourceId — it is the field under test", fixtures.ResourceIDDBISnapID)
	}

	clients := demo.NewServiceClients()
	dbiTD := resource.FindResourceType("dbi")
	dbis, ok := DrainFixtures(t, *dbiTD, clients)
	if !ok {
		t.Fatal("dbi declares no Fetcher")
	}
	var parent resource.Resource
	for _, d := range dbis {
		raw, isDBI := d.RawStruct.(rdstypes.DBInstance)
		if isDBI && aws.ToString(raw.DbiResourceId) == aws.ToString(snap.DbiResourceId) {
			parent = d
		}
	}
	if parent.ID == "" {
		t.Fatalf("no demo DB instance carries DbiResourceId %q — the snapshot names an instance "+
			"the account does not have", aws.ToString(snap.DbiResourceId))
	}

	row := demoRowByID(t, "dbi-snap", fixtures.ResourceIDDBISnapID)
	cache := resource.ResourceCache{"dbi": resource.ResourceCacheEntry{Resources: dbis}}
	def := demoPivotDef(t, "dbi-snap", "dbi")
	r := def.Checker(context.Background(), clients, row, cache)
	if !slices.Contains(r.ResourceIDs(), parent.ID) {
		t.Errorf("%s -> dbi resolved %v ids %v, want %s — the DbiResourceId names it",
			fixtures.ResourceIDDBISnapID, r.EffectiveState(), r.ResourceIDs(), parent.ID)
	}
}

func demoRowByID(t *testing.T, shortName, id string) resource.Resource {
	t.Helper()
	clients := demo.NewServiceClients()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("no %s resource type", shortName)
	}
	rows, ok := DrainFixtures(t, *td, clients)
	if !ok {
		t.Fatalf("%s declares no Fetcher", shortName)
	}
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("%s is not in the demo %s list", id, shortName)
	return resource.Resource{}
}

func demoPivotDef(t *testing.T, shortName, target string) resource.RelatedDef {
	t.Helper()
	defs := resource.GetRelated(shortName)
	for i := range defs {
		if defs[i].TargetType == target && defs[i].Checker != nil {
			return defs[i]
		}
	}
	t.Fatalf("%s declares no %s pivot with a checker", shortName, target)
	return resource.RelatedDef{}
}
