package unit

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

type t4SnapBlockFake struct {
	*pw1EBSSnapFake
	state ec2types.SnapshotBlockPublicAccessState
	err   error

	mu    sync.Mutex
	reads int
}

func (f *t4SnapBlockFake) GetSnapshotBlockPublicAccessState(_ context.Context, _ *ec2.GetSnapshotBlockPublicAccessStateInput, _ ...func(*ec2.Options)) (*ec2.GetSnapshotBlockPublicAccessStateOutput, error) {
	f.mu.Lock()
	f.reads++
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return &ec2.GetSnapshotBlockPublicAccessStateOutput{State: f.state, ManagedBy: ec2types.ManagedByAccount}, nil
}

func (f *t4SnapBlockFake) readCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads
}

func t4SnapRows(snaps ...ec2types.Snapshot) []resource.Resource {
	rs := make([]resource.Resource, 0, len(snaps))
	for _, s := range snaps {
		id := *s.SnapshotId
		rs = append(rs, resource.Resource{
			ID: id, Name: id, RawStruct: s,
			Fields: map[string]string{"snapshot_id": id, "volume_id": *s.VolumeId},
		})
	}
	return rs
}

func t4EnrichSnaps(t *testing.T, clients *awsclient.ServiceClients, snaps ...ec2types.Snapshot) awsclient.IssueEnricherResult {
	t.Helper()
	res, _ := pw1EBSSnapEnricher(t)(context.Background(), clients, t4SnapRows(snaps...), pw1EBSCache("vol-0aaaa1111bbbb2222")) //nolint:errcheck // a refused read is an input in some cases; the row state is asserted
	return res
}

// Under block-all-sharing AWS treats every publicly shared snapshot as
// private while its attributes still name the "all" group
// (AWS::EC2::SnapshotBlockPublicAccess); block-new-sharing refuses new public
// shares and leaves the existing ones public.
func TestT4_EBSSnap_RegionBlockDecidesWhetherAPublicShareIsPublic(t *testing.T) {
	const id = "snap-0t4shared0aaaa1"
	for _, tc := range []struct {
		state  ec2types.SnapshotBlockPublicAccessState
		public bool
	}{
		{ec2types.SnapshotBlockPublicAccessStateBlockAllSharing, false},
		{ec2types.SnapshotBlockPublicAccessStateBlockNewSharing, true},
		{ec2types.SnapshotBlockPublicAccessStateUnblocked, true},
	} {
		t.Run(string(tc.state), func(t *testing.T) {
			pub := pw1Snapshot(id, "vol-0aaaa1111bbbb2222")
			fake := &t4SnapBlockFake{pw1EBSSnapFake: &pw1EBSSnapFake{public: []ec2types.Snapshot{pub}}, state: tc.state}
			res := t4EnrichSnaps(t, &awsclient.ServiceClients{EC2: fake}, pub)
			if got := hasCode(res.Findings[id], pw1EBSSnapCodePublic); got != tc.public {
				t.Errorf("state %s: %s present = %v, want %v", tc.state, pw1EBSSnapCodePublic, got, tc.public)
			}
			if cause, marked := res.TruncatedIDs[id]; marked {
				t.Errorf("state %s: snapshot marked not inspected (%q); every read answered", tc.state, cause)
			}
		})
	}
}

func TestT4_EBSSnap_RefusedRegionBlockMarksThePublicSnapshotOnly(t *testing.T) {
	pub := pw1Snapshot("snap-0t4shared0bbbb1", "vol-0aaaa1111bbbb2222")
	priv := pw1Snapshot("snap-0t4private0bbb1", "vol-0aaaa1111bbbb2222")
	fake := &t4SnapBlockFake{
		pw1EBSSnapFake: &pw1EBSSnapFake{public: []ec2types.Snapshot{pub}},
		err:            t4Refusal("EC2", "GetSnapshotBlockPublicAccessState"),
	}
	res := t4EnrichSnaps(t, &awsclient.ServiceClients{EC2: fake}, pub, priv)

	cause, marked := res.TruncatedIDs["snap-0t4shared0bbbb1"]
	if !marked {
		t.Fatalf("TruncatedIDs = %v, want the shared snapshot marked: its verdict depends on the unread Region block", res.TruncatedIDs)
	}
	if !strings.Contains(cause, "GetSnapshotBlockPublicAccessState") {
		t.Errorf("cause = %q, want it to name GetSnapshotBlockPublicAccessState", cause)
	}
	if hasCode(res.Findings["snap-0t4shared0bbbb1"], pw1EBSSnapCodePublic) {
		t.Errorf("findings = %v, want no %s from an unread Region block", res.Findings["snap-0t4shared0bbbb1"], pw1EBSSnapCodePublic)
	}
	if cause, marked := res.TruncatedIDs["snap-0t4private0bbb1"]; marked {
		t.Errorf("private snapshot marked not inspected (%q); it never needed the Region block", cause)
	}
}

func TestT4_EBSSnap_RegionBlockIsNotReadWhenNothingIsShared(t *testing.T) {
	priv := pw1Snapshot("snap-0t4private0cccc1", "vol-0aaaa1111bbbb2222")
	elsewhere := pw1Snapshot("snap-0t4offpage00cc1", "vol-0aaaa1111bbbb2222")
	for name, fake := range map[string]*t4SnapBlockFake{
		"no public snapshot":           {pw1EBSSnapFake: &pw1EBSSnapFake{}},
		"public snapshot off the page": {pw1EBSSnapFake: &pw1EBSSnapFake{public: []ec2types.Snapshot{elsewhere}}},
	} {
		t4EnrichSnaps(t, &awsclient.ServiceClients{EC2: fake}, priv)
		if got := fake.readCount(); got != 0 {
			t.Errorf("%s: GetSnapshotBlockPublicAccessState called %d times, want 0", name, got)
		}
	}
}

func TestT4_EBSSnap_RegionBlockIsReadOncePerRegionUntilRotation(t *testing.T) {
	pub := pw1Snapshot("snap-0t4shared0dddd1", "vol-0aaaa1111bbbb2222")
	fake := &t4SnapBlockFake{
		pw1EBSSnapFake: &pw1EBSSnapFake{public: []ec2types.Snapshot{pub}},
		state:          ec2types.SnapshotBlockPublicAccessStateUnblocked,
	}
	clients := &awsclient.ServiceClients{EC2: fake, Region: "eu-central-1"}
	sess := session.New()
	sess.WireStores(clients)

	t4EnrichSnaps(t, clients, pub)
	t4EnrichSnaps(t, clients, pub)
	if got := fake.readCount(); got != 1 {
		t.Errorf("GetSnapshotBlockPublicAccessState called %d times over two passes, want 1", got)
	}

	sess.Rotate()
	sess.WireStores(clients)
	res := t4EnrichSnaps(t, clients, pub)
	if got := fake.readCount(); got != 2 {
		t.Errorf("GetSnapshotBlockPublicAccessState called %d times after a rotation, want 2: rotation drops the value", got)
	}
	if !hasCode(res.Findings["snap-0t4shared0dddd1"], pw1EBSSnapCodePublic) {
		t.Errorf("findings = %v, want %s after the re-read", res.Findings["snap-0t4shared0dddd1"], pw1EBSSnapCodePublic)
	}
}
