package unit

// aws_lazy_add_fetchers_test.go — unit tests for the FetchByIDs functions
// that back the related-panel lazy-add path. Each test uses a minimal mock
// to verify:
//   - the function makes a single batched (where possible) API call,
//   - the mock receives the exact IDs (no filter injected),
//   - the returned Resources carry the same Field shape as the paginated
//     fetcher (so reverse-scan checkers reading Fields["..."] on a
//     lazily-added resource find what they expect).

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// AMI — FetchAMIsByIDs
// ---------------------------------------------------------------------------

type fetchAMIsByIDsMock struct {
	gotInput *ec2.DescribeImagesInput
	out      *ec2.DescribeImagesOutput
}

func (m *fetchAMIsByIDsMock) DescribeImages(_ context.Context, in *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	m.gotInput = in
	return m.out, nil
}

func TestFetchAMIsByIDs_PassesIDsUnfiltered(t *testing.T) {
	mock := &fetchAMIsByIDsMock{
		out: &ec2.DescribeImagesOutput{
			Images: []ec2types.Image{
				{
					ImageId:      aws.String("ami-public-0001"),
					Name:         aws.String("ubuntu-22.04"),
					State:        ec2types.ImageStateAvailable,
					Architecture: ec2types.ArchitectureValuesX8664,
					Public:       aws.Bool(true),
				},
			},
		},
	}

	got, err := awsclient.FetchAMIsByIDs(context.Background(), mock, []string{"ami-public-0001", "", "ami-public-0002"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.gotInput == nil {
		t.Fatal("DescribeImages was not called")
	}
	// FetchByIDs MUST NOT inject Owners=self (that would re-apply the filter
	// the paginated fetcher applies — the whole point of lazy-add is to
	// reach public / cross-account AMIs).
	if len(mock.gotInput.Owners) != 0 {
		t.Errorf("DescribeImages input Owners = %v, want none (lazy-add must not filter by owner)", mock.gotInput.Owners)
	}
	// Empty IDs get dropped but non-empty IDs pass through verbatim.
	sort.Strings(mock.gotInput.ImageIds)
	want := []string{"ami-public-0001", "ami-public-0002"}
	if !reflect.DeepEqual(mock.gotInput.ImageIds, want) {
		t.Errorf("DescribeImages input ImageIds = %v, want %v", mock.gotInput.ImageIds, want)
	}

	if len(got) != 1 || got[0].ID != "ami-public-0001" {
		t.Errorf("got = %+v, want one resource with ID=ami-public-0001", got)
	}
}

func TestFetchAMIsByIDs_EmptyInput_NoAPICall(t *testing.T) {
	mock := &fetchAMIsByIDsMock{}
	got, err := awsclient.FetchAMIsByIDs(context.Background(), mock, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
	if mock.gotInput != nil {
		t.Error("DescribeImages was called with empty input slice; expected a fast-path skip")
	}
}

// ---------------------------------------------------------------------------
// EBS Snapshot — FetchEBSSnapshotsByIDs
// ---------------------------------------------------------------------------

type fetchEBSSnapshotsByIDsMock struct {
	gotInput *ec2.DescribeSnapshotsInput
	out      *ec2.DescribeSnapshotsOutput
}

func (m *fetchEBSSnapshotsByIDsMock) DescribeSnapshots(_ context.Context, in *ec2.DescribeSnapshotsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	m.gotInput = in
	return m.out, nil
}

func TestFetchEBSSnapshotsByIDs_PassesIDsUnfiltered(t *testing.T) {
	mock := &fetchEBSSnapshotsByIDsMock{
		out: &ec2.DescribeSnapshotsOutput{
			Snapshots: []ec2types.Snapshot{
				{
					SnapshotId:  aws.String("snap-shared-001"),
					VolumeId:    aws.String("vol-0001"),
					State:       ec2types.SnapshotStateCompleted,
					Description: aws.String("shared from another account"),
				},
			},
		},
	}

	got, err := awsclient.FetchEBSSnapshotsByIDs(context.Background(), mock, []string{"snap-shared-001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.gotInput == nil {
		t.Fatal("DescribeSnapshots was not called")
	}
	// FetchByIDs must NOT apply OwnerIds=self.
	if len(mock.gotInput.OwnerIds) != 0 {
		t.Errorf("DescribeSnapshots input OwnerIds = %v, want none", mock.gotInput.OwnerIds)
	}
	if len(got) != 1 || got[0].ID != "snap-shared-001" {
		t.Errorf("got = %+v, want one resource with ID=snap-shared-001", got)
	}
	// Field shape must match the paginated fetcher so reverse-scan checkers
	// that read Fields["volume_id"] find the value on a lazily-added snapshot.
	if got[0].Fields["volume_id"] != "vol-0001" {
		t.Errorf("Fields[volume_id] = %q, want %q", got[0].Fields["volume_id"], "vol-0001")
	}
}

func TestFetchEBSSnapshotsByIDs_EmptyInput_NoAPICall(t *testing.T) {
	mock := &fetchEBSSnapshotsByIDsMock{}
	got, err := awsclient.FetchEBSSnapshotsByIDs(context.Background(), mock, []string{""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
	if mock.gotInput != nil {
		t.Error("DescribeSnapshots was called with empty filtered slice; expected a fast-path skip")
	}
}
