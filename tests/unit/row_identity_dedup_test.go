// row_identity_dedup_test.go — a duplicate row ID is reported, never dropped
// silently.
//
// Rows are keyed by ID everywhere downstream (findings, attention details,
// related sets, the cursor), so two rows with one ID cannot both be shown: one
// is collapsed. What must not happen is that the operator loses a resource
// without being told. DedupByID returns what it collapsed, and the list
// accumulator puts it on screen.
package unit_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

func TestDedupByID_KeepsFirstOccurrenceAndNamesWhatItCollapsed(t *testing.T) {
	row := func(id, name string) resource.Resource {
		return resource.Resource{ID: id, Name: name}
	}
	cases := []struct {
		name      string
		in        []resource.Resource
		wantNames []string
		wantDups  []string
	}{
		{
			name:      "no duplicates",
			in:        []resource.Resource{row("a", "a-1"), row("b", "b-1"), row("c", "c-1")},
			wantNames: []string{"a-1", "b-1", "c-1"},
		},
		{
			name:      "later duplicate collapses onto the first",
			in:        []resource.Resource{row("a", "a-1"), row("b", "b-1"), row("a", "a-2"), row("c", "c-1")},
			wantNames: []string{"a-1", "b-1", "c-1"},
			wantDups:  []string{"a"},
		},
		{
			name:      "two different IDs duplicated, reported in the order they collapse",
			in:        []resource.Resource{row("y", "y-1"), row("x", "x-1"), row("y", "y-2"), row("x", "x-2")},
			wantNames: []string{"y-1", "x-1"},
			wantDups:  []string{"y", "x"},
		},
		{
			name: "empty input",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, dups := resource.DedupByID(tc.in)
			var gotNames []string
			for _, r := range out {
				gotNames = append(gotNames, r.Name)
			}
			if !reflect.DeepEqual(gotNames, tc.wantNames) {
				t.Errorf("out = %v, want %v", gotNames, tc.wantNames)
			}
			if len(dups)+len(tc.wantDups) > 0 && !reflect.DeepEqual(dups, tc.wantDups) {
				t.Errorf("dups = %v, want %v", dups, tc.wantDups)
			}
		})
	}
}

// ridDupWarning returns the text the screen shows the operator for this
// snapshot's list: the status-bar flash and the list's own marker.
func ridDupWarning(vs app.ViewState) string {
	s := vs.Header.Flash.Text
	if vs.Body.List != nil {
		s += "\n" + vs.Body.List.LastFetchError
	}
	return strings.TrimSpace(s)
}

func ridListRowCount(t *testing.T, vs app.ViewState, id string) int {
	t.Helper()
	if vs.Body.List == nil {
		t.Fatal("no list body on screen")
	}
	n := 0
	for _, r := range vs.Body.List.Rows {
		if r.ResourceID == id {
			n++
		}
	}
	return n
}

const ridBlueAPIArn = "arn:aws:ecs:us-east-1:123456789012:service/blue/api"

func ridSvcRow(id, name, cluster string) resource.Resource {
	return resource.Resource{
		ID:   id,
		Name: name,
		Type: "ecs-svc",
		Fields: map[string]string{
			"service_name":  name,
			"cluster":       cluster,
			"status":        "ACTIVE",
			"desired_count": "2",
			"running_count": "2",
			"launch_type":   "FARGATE",
			"arn":           id,
		},
	}
}

// TestListAccumulator_DuplicateInOnePageIsReported drives the canonical
// top-level list (the lane that goes through the session RowStore): a page
// carrying two rows with one ID shows the ID once and tells the operator which
// ID collided. A clean page says nothing.
func TestListAccumulator_DuplicateInOnePageIsReported(t *testing.T) {
	c := openListController(t, "ecs-svc")
	worker := ridSvcRow("arn:aws:ecs:us-east-1:123456789012:service/green/worker", "worker", "green")

	vs, _ := handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ecs-svc",
		Provenance:   messages.FetchProvenanceCanonicalList,
		Pagination:   &resource.PaginationMeta{IsTruncated: false, PageSize: 2, TotalHint: 2},
		Resources:    []resource.Resource{ridSvcRow(ridBlueAPIArn, "api", "blue"), worker},
	})
	if got := ridDupWarning(vs); got != "" {
		t.Errorf("a page with unique IDs shows %q, want nothing", got)
	}

	vs, _ = handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ecs-svc",
		Provenance:   messages.FetchProvenanceCanonicalList,
		Pagination:   &resource.PaginationMeta{IsTruncated: false, PageSize: 3, TotalHint: 3},
		Resources: []resource.Resource{
			ridSvcRow(ridBlueAPIArn, "api", "blue"),
			worker,
			ridSvcRow(ridBlueAPIArn, "api", "green"),
		},
	})
	if n := ridListRowCount(t, vs, ridBlueAPIArn); n != 1 {
		t.Errorf("the duplicated ID shows as %d rows, want 1", n)
	}
	if n := ridListRowCount(t, vs, worker.ID); n != 1 {
		t.Errorf("the unique row shows as %d rows, want 1", n)
	}
	if got := ridDupWarning(vs); !strings.Contains(got, ridBlueAPIArn) {
		t.Errorf("a page with a duplicate ID shows %q, want a warning naming %q", got, ridBlueAPIArn)
	}
}

// TestListAccumulator_DuplicateInChildPageIsReported drives the other lane: a
// child list keeps its rows on the screen only, never in the RowStore, so it
// has its own accumulator and must report the same way.
func TestListAccumulator_DuplicateInChildPageIsReported(t *testing.T) {
	c := openListController(t, "ecs-svc")
	handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ecs-svc",
		Provenance:   messages.FetchProvenanceCanonicalList,
		Pagination:   &resource.PaginationMeta{IsTruncated: false, PageSize: 1, TotalHint: 1},
		Resources:    []resource.Resource{ridSvcRow(ridBlueAPIArn, "api", "blue")},
	})
	c.Apply(app.Action{Kind: app.ActionChildView, Arg: "e"})
	vs := c.Snapshot()
	if vs.Body.List == nil {
		t.Fatal("pressing e on an ECS service did not open its events list")
	}

	const dupEventID = "9d1f3c52-6b7a-4e8f-a1b2-c3d4e5f60718"
	event := func(id, message string) resource.Resource {
		return resource.Resource{
			ID:   id,
			Name: "2026-09-18 10:15",
			Type: "ecs_svc_events",
			Fields: map[string]string{
				"timestamp":   "2026-09-18 10:15",
				"message":     message,
				"service_arn": ridBlueAPIArn,
			},
		}
	}
	vs, _ = handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ecs_svc_events",
		Provenance:   messages.FetchProvenanceChild,
		Pagination:   &resource.PaginationMeta{IsTruncated: false, PageSize: 3, TotalHint: 3},
		Resources: []resource.Resource{
			event(dupEventID, "(service api) has reached a steady state."),
			event("0a7e2b41-5c6d-4f8e-9a0b-1c2d3e4f5a6b", "(service api) registered 1 targets in (target-group api-blue)"),
			event(dupEventID, "(service api) has started 1 tasks: (task 9f1c2b7e4d3a4f0e)."),
		},
	})
	if n := ridListRowCount(t, vs, dupEventID); n != 1 {
		t.Errorf("the duplicated event ID shows as %d rows, want 1", n)
	}
	if got := ridDupWarning(vs); !strings.Contains(got, dupEventID) {
		t.Errorf("a child page with a duplicate ID shows %q, want a warning naming %q", got, dupEventID)
	}
}
