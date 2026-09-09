package unit

// prowler_w1_ebs_snap_test.go — behavioural pins for ebs-snap.public
// (batch w1).
//
// A snapshot whose createVolumePermission names the "all" group can be
// restored into any AWS account, which hands over everything the source
// volume held. AWS answers the question for the whole account in one call:
// DescribeSnapshots(OwnerIds=self, RestorableByUserIds=all). The tests pin
// that shape, because a per-snapshot DescribeSnapshotAttribute walk would
// cost one call per row.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const pw1EBSSnapCodePublic = domain.FindingCode("ebs-snap.public")

// pw1EBSSnapFake answers DescribeSnapshots. Calls whose RestorableByUserIds
// names "all" get the public set; every other call gets the full set, which
// is what the list path asks for.
type pw1EBSSnapFake struct {
	awsclient.EC2API
	public     []ec2types.Snapshot
	all        []ec2types.Snapshot
	err        error
	pages      int
	restorable [][]string
}

func (f *pw1EBSSnapFake) DescribeSnapshots(_ context.Context, in *ec2.DescribeSnapshotsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.pages++
	f.restorable = append(f.restorable, in.RestorableByUserIds)
	for _, u := range in.RestorableByUserIds {
		if u == "all" {
			return &ec2.DescribeSnapshotsOutput{Snapshots: f.public}, nil
		}
	}
	return &ec2.DescribeSnapshotsOutput{Snapshots: f.all}, nil
}

// pw1Snapshot builds a completed snapshot of volumeID.
func pw1Snapshot(id, volumeID string) ec2types.Snapshot {
	return ec2types.Snapshot{
		SnapshotId:  aws.String(id),
		VolumeId:    aws.String(volumeID),
		OwnerId:     aws.String("123456789012"),
		State:       ec2types.SnapshotStateCompleted,
		Progress:    aws.String("100%"),
		VolumeSize:  aws.Int32(100),
		Encrypted:   aws.Bool(true),
		StartTime:   aws.Time(time.Now().Add(-72 * time.Hour)),
		Description: aws.String("acme nightly"),
	}
}

// pw1EBSCache builds a cache["ebs"] entry naming the parent volumes, so the
// orphan rule stays silent and the public rule is observed on its own.
func pw1EBSCache(volumeIDs ...string) resource.ResourceCache {
	rs := make([]resource.Resource, 0, len(volumeIDs))
	for _, id := range volumeIDs {
		rs = append(rs, resource.Resource{
			ID:        id,
			RawStruct: ec2types.Volume{VolumeId: aws.String(id), State: ec2types.VolumeStateInUse},
		})
	}
	return resource.ResourceCache{"ebs": resource.ResourceCacheEntry{Resources: rs}}
}

func pw1EBSSnapEnricher(t *testing.T) awsclient.IssueEnricherFunc {
	t.Helper()
	e, ok := awsclient.Wave2EnricherFor("ebs-snap")
	if !ok || e.Fn == nil {
		t.Fatal("no Wave 2 enricher registered for ebs-snap")
	}
	return e.Fn
}

func pw1EnrichEBSSnap(t *testing.T, fake *pw1EBSSnapFake, cache resource.ResourceCache, snaps ...ec2types.Snapshot) awsclient.IssueEnricherResult {
	t.Helper()
	rs := make([]resource.Resource, 0, len(snaps))
	for _, s := range snaps {
		rs = append(rs, resource.Resource{
			ID:        aws.ToString(s.SnapshotId),
			Name:      aws.ToString(s.SnapshotId),
			RawStruct: s,
			Fields:    map[string]string{"snapshot_id": aws.ToString(s.SnapshotId), "volume_id": aws.ToString(s.VolumeId)},
		})
	}
	res, err := pw1EBSSnapEnricher(t)(context.Background(),
		&awsclient.ServiceClients{EC2: fake}, rs, cache)
	if err != nil && fake.err == nil {
		t.Fatalf("ebs-snap enricher: %v", err)
	}
	return res
}

// TestEBSSnap_Public_RestorableByAnyone pins the Broken finding and its row.
func TestEBSSnap_Public_RestorableByAnyone(t *testing.T) {
	pub := pw1Snapshot("snap-0public00aaaaa1", "vol-0aaaa1111bbbb2222")
	fake := &pw1EBSSnapFake{public: []ec2types.Snapshot{pub}}

	res := pw1EnrichEBSSnap(t, fake, pw1EBSCache("vol-0aaaa1111bbbb2222"), pub)
	pw1RequireFinding(t, res.Findings["snap-0public00aaaaa1"], pw1EBSSnapCodePublic,
		"shared with all AWS accounts", domain.SevBroken, "wave2")
	// d4 row 20: the value is a word, not the SDK field's shape. Do not
	// restore "true" — TestNetworkingRowValues_AreWordsNotLiterals fails on it.
	pw1RequireRow(t, pw1Rows(res, "snap-0public00aaaaa1", pw1EBSSnapCodePublic), "Public", "yes")
}

// TestEBSSnap_Public_PrivateIsHealthy pins the negative case: a snapshot the
// account-wide query does not return is not shared.
func TestEBSSnap_Public_PrivateIsHealthy(t *testing.T) {
	priv := pw1Snapshot("snap-0private0aaaaa1", "vol-0aaaa1111bbbb2222")
	fake := &pw1EBSSnapFake{}
	res := pw1EnrichEBSSnap(t, fake, pw1EBSCache("vol-0aaaa1111bbbb2222"), priv)
	pw1RequireNoFinding(t, res.Findings["snap-0private0aaaaa1"], pw1EBSSnapCodePublic)
}

// TestEBSSnap_Public_QueriesTheAccountOnceForTheAllGroup pins the call shape:
// one account-wide query scoped to self-owned, all-restorable snapshots,
// regardless of how many rows are on screen.
func TestEBSSnap_Public_QueriesTheAccountOnceForTheAllGroup(t *testing.T) {
	a := pw1Snapshot("snap-0aaaa1111bbbb1", "vol-0aaaa1111bbbb2222")
	b := pw1Snapshot("snap-0aaaa1111bbbb2", "vol-0aaaa1111bbbb2222")
	c := pw1Snapshot("snap-0aaaa1111bbbb3", "vol-0aaaa1111bbbb2222")
	fake := &pw1EBSSnapFake{public: []ec2types.Snapshot{b}}

	res := pw1EnrichEBSSnap(t, fake, pw1EBSCache("vol-0aaaa1111bbbb2222"), a, b, c)
	if fake.pages != 1 {
		t.Errorf("DescribeSnapshots called %d times for 3 snapshots; the public check is one account-wide query", fake.pages)
	}
	if len(fake.restorable) != 1 || len(fake.restorable[0]) != 1 || fake.restorable[0][0] != "all" {
		t.Errorf("RestorableByUserIds = %v, want [[all]]", fake.restorable)
	}
	pw1RequireFinding(t, res.Findings["snap-0aaaa1111bbbb2"], pw1EBSSnapCodePublic,
		"shared with all AWS accounts", domain.SevBroken, "wave2")
	for _, id := range []string{"snap-0aaaa1111bbbb1", "snap-0aaaa1111bbbb3"} {
		pw1RequireNoFinding(t, res.Findings[id], pw1EBSSnapCodePublic)
	}
}

// TestEBSSnap_Public_ReturnedButNotOnScreenIsNotARow pins that a public
// snapshot outside the loaded page does not create a phantom row: findings
// are keyed by a resource the list actually holds.
func TestEBSSnap_Public_ReturnedButNotOnScreenIsNotARow(t *testing.T) {
	onScreen := pw1Snapshot("snap-0onscreen0aaaa1", "vol-0aaaa1111bbbb2222")
	elsewhere := pw1Snapshot("snap-0offscreen0aaa1", "vol-0aaaa1111bbbb2222")
	fake := &pw1EBSSnapFake{public: []ec2types.Snapshot{elsewhere}}

	res := pw1EnrichEBSSnap(t, fake, pw1EBSCache("vol-0aaaa1111bbbb2222"), onScreen)
	if fs, ok := res.Findings["snap-0offscreen0aaa1"]; ok && len(fs) > 0 {
		t.Errorf("finding emitted for a snapshot that is not in the input: %+v", fs)
	}
	pw1RequireNoFinding(t, res.Findings["snap-0onscreen0aaaa1"], pw1EBSSnapCodePublic)
}

// TestEBSSnap_Public_FiresWithoutTheParentVolumeCache pins independence from
// the orphan rule: whether the ebs list has been loaded says nothing about
// who can restore the snapshot, so the public finding must not be gated on it.
func TestEBSSnap_Public_FiresWithoutTheParentVolumeCache(t *testing.T) {
	pub := pw1Snapshot("snap-0nocache00aaaa1", "vol-0aaaa1111bbbb2222")
	fake := &pw1EBSSnapFake{public: []ec2types.Snapshot{pub}}

	res := pw1EnrichEBSSnap(t, fake, resource.ResourceCache{}, pub)
	pw1RequireFinding(t, res.Findings["snap-0nocache00aaaa1"], pw1EBSSnapCodePublic,
		"shared with all AWS accounts", domain.SevBroken, "wave2")
}

// TestEBSSnap_PublicAndOrphanAreTwoFindings pins independence: a public
// snapshot whose source volume is gone carries both findings, each with its
// own code and its own rows.
func TestEBSSnap_PublicAndOrphanAreTwoFindings(t *testing.T) {
	pub := pw1Snapshot("snap-0puborphan0aa1", "vol-0deleted111bbbb2")
	fake := &pw1EBSSnapFake{public: []ec2types.Snapshot{pub}}

	res := pw1EnrichEBSSnap(t, fake, pw1EBSCache("vol-0other1111bbbb2"), pub)
	pw1RequireFinding(t, res.Findings["snap-0puborphan0aa1"], pw1EBSSnapCodePublic,
		"shared with all AWS accounts", domain.SevBroken, "wave2")
	if _, ok := pw1FindFinding(res.Findings["snap-0puborphan0aa1"], domain.FindingCode("ebs-snap.orphan")); !ok {
		t.Errorf("orphan finding lost when the public finding was added: %+v", res.Findings["snap-0puborphan0aa1"])
	}
	if len(pw1Rows(res, "snap-0puborphan0aa1", pw1EBSSnapCodePublic)) == 0 {
		t.Errorf("public rows crowded out by the orphan finding's rows")
	}
}

// TestEBSSnap_Public_APIErrorMarksEveryRowUninspected pins the failure
// contract: when the account-wide query fails nothing is known about any
// snapshot, so every input row goes to TruncatedIDs and none is reported
// clean.
func TestEBSSnap_Public_APIErrorMarksEveryRowUninspected(t *testing.T) {
	a := pw1Snapshot("snap-0errora00aaaa1", "vol-0aaaa1111bbbb2222")
	b := pw1Snapshot("snap-0errorb00aaaa1", "vol-0aaaa1111bbbb2222")
	fake := &pw1EBSSnapFake{err: errors.New("AccessDeniedException: ec2:DescribeSnapshots")}

	res := pw1EnrichEBSSnap(t, fake, pw1EBSCache("vol-0aaaa1111bbbb2222"), a, b)
	for _, id := range []string{"snap-0errora00aaaa1", "snap-0errorb00aaaa1"} {
		if _, marked := res.TruncatedIDs[id]; !marked {
			t.Errorf("TruncatedIDs missing %s after the public-snapshot query failed", id)
		}
		pw1RequireNoFinding(t, res.Findings[id], pw1EBSSnapCodePublic)
	}
	if !res.Truncated {
		t.Errorf("Truncated = false after a failed query for a Broken-severity signal")
	}
}

// TestEBSSnap_Public_NilClientIsANoOp pins that the enricher is silent before
// the EC2 client exists, with every reference map non-nil.
func TestEBSSnap_Public_NilClientIsANoOp(t *testing.T) {
	snap := pw1Snapshot("snap-0noclient0aaaa1", "vol-0aaaa1111bbbb2222")
	res, err := pw1EBSSnapEnricher(t)(context.Background(), &awsclient.ServiceClients{},
		[]resource.Resource{{ID: "snap-0noclient0aaaa1", RawStruct: snap}}, pw1EBSCache("vol-0aaaa1111bbbb2222"))
	if err != nil {
		t.Fatalf("nil EC2 client must not error: %v", err)
	}
	if res.Findings == nil || res.TruncatedIDs == nil {
		t.Errorf("reference maps must be non-nil on success: %+v", res)
	}
	pw1RequireNoFinding(t, res.Findings["snap-0noclient0aaaa1"], pw1EBSSnapCodePublic)
}

// TestEBSSnap_DemoBench_OnlyWitnessIsPublic pins the demo fixture contract.
func TestEBSSnap_DemoBench_OnlyWitnessIsPublic(t *testing.T) {
	out, err := awsclient.FetchEBSSnapshotsPage(context.Background(), fakes.NewEC2(), "")
	if err != nil {
		t.Fatalf("FetchEBSSnapshotsPage(demo): %v", err)
	}
	volumes, verr := awsclient.FetchEBSVolumesPage(context.Background(), fakes.NewEC2(), "")
	if verr != nil {
		t.Fatalf("FetchEBSVolumesPage(demo): %v", verr)
	}
	cache := resource.ResourceCache{"ebs": resource.ResourceCacheEntry{Resources: volumes.Resources}}

	res, eerr := pw1EBSSnapEnricher(t)(context.Background(),
		&awsclient.ServiceClients{EC2: fakes.NewEC2()}, out.Resources, cache)
	if eerr != nil {
		t.Fatalf("ebs-snap enricher(demo): %v", eerr)
	}
	var carriers []string
	for id, fs := range res.Findings {
		if _, ok := pw1FindFinding(fs, pw1EBSSnapCodePublic); ok {
			carriers = append(carriers, id)
		}
	}
	pw1RequireOnlyWitness(t, pw1EBSSnapCodePublic, fixtures.EBSSnapPublic, carriers)
}
