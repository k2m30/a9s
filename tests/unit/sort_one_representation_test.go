// SPDX-License-Identifier: GPL-3.0-or-later

// sort_one_representation_test.go — a list sorts the same warm or live.
//
// A list opened over the disk cache renders rows with no SDK struct: every
// cell is the text the save lane left behind. The same list a second later,
// once the fetch lands, renders rows that still carry the struct. The
// comparator used to read the struct on the live frame and the text on the
// cached one, so the same column could order the same rows two ways and the
// list re-ordered under the operator when the fetch arrived.
//
// One representation now: the cell text, on both frames.
package unit_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/config"
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

// sortSubMinuteInstances are two EC2 rows launched in the same minute, in the
// order the fetcher returns them — which is not their chronological order.
// The cache keeps a launch time to the minute, so the text frame cannot tell
// them apart; a comparator reading the struct can, and that is the
// disagreement the operator sees as a list re-ordering itself.
func sortSubMinuteInstances() []resource.Resource {
	minute := time.Date(2026, 1, 2, 10, 30, 0, 0, time.UTC)
	mk := func(id string, offset time.Duration) resource.Resource {
		return resource.Resource{
			ID:   id,
			Name: id,
			Type: "ec2",
			RawStruct: ec2types.Instance{
				InstanceId: aws.String(id),
				LaunchTime: aws.Time(minute.Add(offset)),
			},
		}
	}
	return []resource.Resource{
		mk("i-0000000000000late", 40*time.Second),
		mk("i-000000000000early", 10*time.Second),
	}
}

// TestSort_TheSameColumnOrdersTheSameWarmOrLive sorts one column on the live
// frame and on the frame the disk cache replays, and demands the same order.
func TestSort_TheSameColumnOrdersTheSameWarmOrLive(t *testing.T) {
	profile, region := "sort-one-representation", "us-east-1"
	ctrl := newTestControllerForProfile(t, profile, region)
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	live := sortSubMinuteInstances()
	ctrl.ApplyResourcesLoaded("ec2", live, nil, false)
	ctrl.Apply(app.Action{Kind: app.ActionSort, Arg: "Launch Time"})
	liveOrder := sortFrameIDs(t, ctrl)

	store := cache.LoadDirForTest(profile, region)
	tf, found := store.Type("ec2")
	if !found || len(tf.Rows) != len(live) {
		t.Fatalf("the list-open save persisted %d rows, want %d", len(tf.Rows), len(live))
	}
	replay := make([]resource.Resource, len(tf.Rows))
	for i, r := range tf.Rows {
		// A warm row that reached the sort without the key its path-only column
		// is read back under would render blank and tie with every other such
		// row — the one way the two frames could still disagree once the
		// comparator reads the cell alone. The save writes
		// config.TitleFieldKey and the render cascade reads
		// config.TitleFieldKeys, of which that is the second spelling, so the
		// key cannot be missing for a column whose live cell had a value; this
		// asserts it on the row rather than on the two helpers. The
		// registry-wide form is TestCols_CacheReplayRendersTheSameCellsAsTheLiveFetch
		// (replay_cache_round_trip_test.go), which requires every cell of every
		// registered type to survive the round trip.
		if got := r.Fields[config.TitleFieldKey("Launch Time")]; got == "" {
			t.Errorf("row %s reached the warm frame with no value under %q — the sort would read a blank cell for a column the live frame renders; persisted fields: %v",
				r.ID, config.TitleFieldKey("Launch Time"), r.Fields)
		}
		replay[i] = resource.Resource{ID: r.ID, Name: r.Name, Type: "ec2", Fields: r.Fields, Findings: r.Findings}
	}

	ctrl.ApplyResourcesLoaded("ec2", replay, nil, false)
	replayOrder := sortFrameIDs(t, ctrl)

	if strings.Join(liveOrder, ",") != strings.Join(replayOrder, ",") {
		t.Errorf("the same column ordered the two frames differently:\n  live   = %v\n  cached = %v", liveOrder, replayOrder)
	}
}

// TestSort_TheComparatorReadsNoSDKStruct is the gate that keeps it that way.
// A second representation reintroduced in the comparator is a second answer to
// one question, and the two frames are where it shows.
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
