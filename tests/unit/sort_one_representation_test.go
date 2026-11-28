// SPDX-License-Identifier: GPL-3.0-or-later

// A list sorts the same warm or live.
//
// A list opened over the disk cache renders rows with no SDK struct: every
// cell is the text the save lane left behind. The same list a second later,
// once the fetch lands, renders rows that still carry the struct. The
// comparator reads one representation on both frames — the cell text — so
// the same column orders the same rows one way and the list does not
// re-order under the operator when the fetch arrives.
package unit_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/resource"
)

// sortFrameIDs is the rendered row order of the list body, top to bottom.
func sortFrameIDs(t *testing.T, ctrl *app.Controller) []string {
	t.Helper()
	lb := ctrl.Snapshot().Body.List
	if lb == nil {
		t.Fatal("no list body rendered")
	}
	out := make([]string, 0, len(lb.Rows))
	for _, r := range lb.Rows {
		out = append(out, r.ResourceID)
	}
	return out
}

type sortInstancesAPI struct {
	instances []ec2types.Instance
}

func (f sortInstancesAPI) DescribeInstances(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{Reservations: []ec2types.Reservation{{Instances: f.instances}}}, nil
}

func (f sortInstancesAPI) DescribeInstanceStatus(_ context.Context, _ *ec2.DescribeInstanceStatusInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceStatusOutput, error) {
	return &ec2.DescribeInstanceStatusOutput{}, nil
}

// sortSubMinuteInstances are two EC2 rows launched in the same minute, as the
// fetcher builds them and in the order DescribeInstances returns them — which
// is not their chronological order. The fetcher keeps a launch time to the
// minute, so the text frame cannot tell them apart; a comparator reading the
// struct can, and that is the disagreement the operator sees as a list
// re-ordering itself.
func sortSubMinuteInstances(t *testing.T) []resource.Resource {
	t.Helper()
	minute := time.Date(2026, 1, 2, 10, 30, 0, 0, time.UTC)
	mk := func(id string, offset time.Duration) ec2types.Instance {
		return ec2types.Instance{
			InstanceId:   aws.String(id),
			InstanceType: ec2types.InstanceTypeT3Micro,
			State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
			LaunchTime:   aws.Time(minute.Add(offset)),
		}
	}
	got, err := awsclient.FetchEC2InstancesPage(context.Background(), sortInstancesAPI{instances: []ec2types.Instance{
		mk("i-0000000000000late", 40*time.Second),
		mk("i-000000000000early", 10*time.Second),
	}}, "")
	if err != nil {
		t.Fatalf("ec2 fetch: %v", err)
	}
	return got.Resources
}

// TestSort_TheSameColumnOrdersTheSameWarmOrLive sorts one column on the live
// frame and on the frame the disk cache replays, and demands the same order.
func TestSort_TheSameColumnOrdersTheSameWarmOrLive(t *testing.T) {
	profile, region := "sort-one-representation", "us-east-1"
	ctrl := newTestControllerForProfile(t, profile, region)
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	live := sortSubMinuteInstances(t)
	ctrl.ApplyResourcesLoaded("ec2", live, nil, false)
	ctrl.Apply(app.Action{Kind: app.ActionSort, Arg: "Launch Time"})
	liveOrder := sortFrameIDs(t, ctrl)

	ctrl.WaitForCacheWrites()
	store := cache.LoadDirForTest(profile, region)
	tf, found := store.Type("ec2")
	if !found || len(tf.Rows) != len(live) {
		t.Fatalf("the list-open save persisted %d rows, want %d", len(tf.Rows), len(live))
	}
	replay := make([]resource.Resource, len(tf.Rows))
	for i, r := range tf.Rows {
		// A warm row that reached the sort without the key the Launch Time
		// column reads would render blank and tie with every other such row.
		if got := r.Fields["launch_time"]; got != "2026-01-02 10:30" {
			t.Errorf("row %s reached the warm frame with launch_time %q, want %q — the sort would read a different cell from the one the live frame renders; persisted fields: %v",
				r.ID, got, "2026-01-02 10:30", r.Fields)
		}
		replay[i] = resource.Resource{ID: r.ID, Name: r.Name, Type: "ec2", Fields: r.Fields, Findings: r.Findings}
	}

	ctrl.ApplyResourcesLoaded("ec2", replay, nil, false)
	replayOrder := sortFrameIDs(t, ctrl)

	if strings.Join(liveOrder, ",") != strings.Join(replayOrder, ",") {
		t.Errorf("the same column ordered the two frames differently:\n  live   = %v\n  cached = %v", liveOrder, replayOrder)
	}
}

// TestSort_TheComparatorReadsNoSDKStruct: a second representation in the
// comparator is a second answer to one question, and the warm and live frames
// are where it shows.
func TestSort_TheComparatorReadsNoSDKStruct(t *testing.T) {
	src, err := os.ReadFile("../../core/app/list_filter.go")
	if err != nil {
		t.Fatalf("read list_filter.go: %v", err)
	}
	for i, line := range strings.Split(string(src), "\n") {
		code, _, _ := strings.Cut(line, "//")
		if strings.Contains(code, "RawStruct") {
			t.Errorf("core/app/list_filter.go:%d reads RawStruct: %s", i+1, strings.TrimSpace(line))
		}
	}
}
