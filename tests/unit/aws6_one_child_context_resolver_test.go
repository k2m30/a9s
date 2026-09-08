package unit_test

// aws6_one_child_context_resolver_test.go — one resolver for a child view's
// parent context.
//
// resource.ResolveChildContext is the single place a ChildViewDef's ContextKeys
// become the map handed to a ChildFetcher. It understands four sources: "ID",
// "Name", "@parent.<key>" for a grandparent's context, and any other string as
// a Fields key.
//
// The controller's own drill path keeps a copy of that switch, and the copy is
// missing the "@parent." case. A one-level drill never notices, because nothing
// above it has a context to inherit. A second-level drill does: the S3 object
// browser's folder-into-folder step reads its bucket from "@parent.bucket", and
// the copy resolves that to Fields["@parent.bucket"], which no row carries.
//
// The pin drives the controller's real drill and compares against the resolver,
// so the expectation cannot drift from what the resolver actually does.

import (
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// TestGrandchildDrillReadsTheParentSource drives s3 → objects → folder through
// the controller and asserts the fetch it dispatches carries the bucket the
// grandparent context holds.
//
// Without it the child fetcher is handed an empty bucket, so the folder opens
// onto nothing and the operator sees an empty directory that is not empty.
func TestGrandchildDrillReadsTheParentSource(t *testing.T) {
	c, bucket, folders := openDemoS3FolderList(t)

	_, tasks := c.Apply(app.Action{Kind: app.ActionChildView, Arg: "enter"})

	payload, ok := childFetchPayload(tasks)
	if !ok {
		t.Fatalf("drilling into folder %q dispatched no child fetch (%d task(s)); the grandchild view never "+
			"opens, so the operator gets nothing at all", folders[0].ID, len(tasks))
	}
	if payload.ChildType != "s3_objects" {
		t.Fatalf("drill dispatched a fetch for %q, want \"s3_objects\"", payload.ChildType)
	}

	// The expectation comes from the one resolver, not from a literal: a copy
	// of the map here would be a third answer to the same question.
	child := s3ObjectsSelfChild(t)
	dr := domain.Resource(folders[0])
	want := resource.ResolveChildContext(child, &dr, map[string]string{"bucket": bucket})

	if got := payload.ParentContext["bucket"]; got != want["bucket"] {
		t.Errorf("grandchild fetch context bucket = %q, want %q — the \"@parent.bucket\" source reads the "+
			"grandparent's context, and a path that resolves it as a Fields key finds nothing",
			got, want["bucket"])
	}
	if got := payload.ParentContext["prefix"]; got != want["prefix"] {
		t.Errorf("grandchild fetch context prefix = %q, want %q — the \"ID\" source is the folder's own key",
			got, want["prefix"])
	}
	if len(payload.ParentContext) != len(want) {
		t.Errorf("grandchild fetch context = %v, want %v — one resolver, one map", payload.ParentContext, want)
	}
}

// TestGrandchildDrillContextMatchesTheResolverExactly is the same comparison
// over every key at once, so a source the resolver grows later is covered here
// without this test changing.
func TestGrandchildDrillContextMatchesTheResolverExactly(t *testing.T) {
	c, bucket, folders := openDemoS3FolderList(t)

	_, tasks := c.Apply(app.Action{Kind: app.ActionChildView, Arg: "enter"})
	payload, ok := childFetchPayload(tasks)
	if !ok {
		t.Fatalf("drilling into folder %q dispatched no child fetch", folders[0].ID)
	}

	child := s3ObjectsSelfChild(t)
	dr := domain.Resource(folders[0])
	want := resource.ResolveChildContext(child, &dr, map[string]string{"bucket": bucket})

	for k, v := range want {
		if payload.ParentContext[k] != v {
			t.Errorf("context[%q] = %q, want %q", k, payload.ParentContext[k], v)
		}
	}
	for k := range payload.ParentContext {
		if _, declared := want[k]; !declared {
			t.Errorf("context carries %q = %q, which the resolver does not produce",
				k, payload.ParentContext[k])
		}
	}
}

// TestFileRowDoesNotDrill is the negative case: the same key on a row the
// DrillCondition rejects opens nothing. A path that dispatched a fetch for
// every row would pass the two tests above while breaking the object browser.
func TestFileRowDoesNotDrill(t *testing.T) {
	clients := demo.NewServiceClients()
	listings := demoS3ObjectListings(t, clients)

	bucket, mixed := firstBucketWith(t, listings, func(rows []resource.Resource) bool {
		var folder, file bool
		for _, r := range rows {
			if r.Fields["kind"] == "folder" {
				folder = true
			} else {
				file = true
			}
		}
		return folder && file
	})

	var files, folders []resource.Resource
	for _, r := range mixed {
		if r.Fields["kind"] == "folder" {
			folders = append(folders, r)
		} else {
			files = append(files, r)
		}
	}

	c := openDemoBucketObjectList(t, bucket, files)
	_, tasks := c.Apply(app.Action{Kind: app.ActionChildView, Arg: "enter"})
	if _, ok := childFetchPayload(tasks); ok {
		t.Errorf("drilling into the object row %q dispatched a child fetch; only a prefix opens",
			files[0].ID)
	}

	// Without this the test above passes on a path that opens NOTHING, which
	// is what it does today: the same listing's prefix row must drill, so
	// "no fetch" means the condition rejected the row and not that the drill
	// is broken for everything.
	c2 := openDemoBucketObjectList(t, bucket, folders)
	_, folderTasks := c2.Apply(app.Action{Kind: app.ActionChildView, Arg: "enter"})
	if _, ok := childFetchPayload(folderTasks); !ok {
		t.Errorf("the prefix row %q in the same listing dispatched no child fetch either, so the object "+
			"row's silence says nothing about the drill condition", folders[0].ID)
	}
}

// firstBucketWith returns the lexicographically first demo bucket whose listing
// satisfies want, so a failure names the same bucket on every run.
func firstBucketWith(
	t *testing.T,
	listings map[string][]resource.Resource,
	want func([]resource.Resource) bool,
) (string, []resource.Resource) {
	t.Helper()
	names := make([]string, 0, len(listings))
	for b := range listings {
		names = append(names, b)
	}
	sort.Strings(names)
	for _, b := range names {
		if want(listings[b]) {
			return b, listings[b]
		}
	}
	t.Fatal("no demo bucket listing matches what this test needs")
	return "", nil
}

// s3ObjectsSelfChild returns the s3_objects type's own s3_objects child view —
// the folder-into-folder step, and the only registered child view whose context
// needs a grandparent.
func s3ObjectsSelfChild(t *testing.T) resource.ChildViewDef {
	t.Helper()
	td := resource.GetChildType("s3_objects")
	if td == nil {
		t.Fatal("no s3_objects child type registered")
	}
	for _, ch := range td.Children {
		if ch.ChildType != "s3_objects" {
			continue
		}
		for _, source := range ch.ContextKeys {
			if strings.HasPrefix(source, "@parent.") && len(source) > len("@parent.") {
				return ch
			}
		}
	}
	t.Fatal("the s3_objects folder drill no longer declares an \"@parent.\" context source, " +
		"so nothing on the bench exercises the grandparent case this path drops")
	return resource.ChildViewDef{}
}

// childFetchPayload returns the child-resources fetch among tasks.
func childFetchPayload(tasks []runtime.TaskRequest) (runtime.FetchChildResourcesPayload, bool) {
	for _, tk := range tasks {
		if p, ok := tk.Payload.(runtime.FetchChildResourcesPayload); ok {
			return p, true
		}
	}
	return runtime.FetchChildResourcesPayload{}, false
}

// openDemoS3FolderList drives the real controller from the s3 list into one
// bucket's object listing and leaves a PREFIX row selected, which is the state
// the folder-into-folder drill starts from. It returns the bucket the
// grandparent context holds and the prefix rows on screen.
func openDemoS3FolderList(t *testing.T) (*app.Controller, string, []resource.Resource) {
	t.Helper()
	clients := demo.NewServiceClients()
	listings := demoS3ObjectListings(t, clients)

	bucket, rows := firstBucketWith(t, listings, func(rows []resource.Resource) bool {
		for _, r := range rows {
			if r.Fields["kind"] == "folder" {
				return true
			}
		}
		return false
	})
	var folders []resource.Resource
	for _, r := range rows {
		if r.Fields["kind"] == "folder" {
			folders = append(folders, r)
		}
	}
	return openDemoBucketObjectList(t, bucket, folders), bucket, folders
}

// openDemoBucketObjectList opens the s3 list on one bucket, drills into it the
// way the operator does, and loads rows into the resulting object listing. The
// first row is the selected one.
func openDemoBucketObjectList(t *testing.T, bucket string, rows []resource.Resource) *app.Controller {
	t.Helper()
	clients := demo.NewServiceClients()
	s3 := resource.FindResourceType("s3")
	if s3 == nil {
		t.Fatal("no s3 resource type registered")
	}
	buckets, ok := drainVisibilityFixtures(t, *s3, clients)
	if !ok {
		t.Fatal("s3 has no fetcher")
	}

	var only []resource.Resource
	for _, b := range buckets {
		if b.ID == bucket {
			only = append(only, b)
		}
	}
	if len(only) != 1 {
		t.Fatalf("expected exactly one demo bucket named %q, found %d", bucket, len(only))
	}

	c := newVisibilityListController(t, "s3")
	c.ApplyResourcesLoaded("s3", only, nil, false)

	// The first drill: bucket → its object listing. This one needs no
	// grandparent, which is why the missing source never showed up here.
	if _, tasks := c.Apply(app.Action{Kind: app.ActionChildView, Arg: "enter"}); len(tasks) == 0 {
		t.Fatalf("drilling into bucket %q dispatched no child fetch", bucket)
	}
	c.ApplyResourcesLoaded("s3_objects", rows, nil, false)

	body := c.Snapshot().Body.List
	if body == nil || len(body.Rows) == 0 {
		t.Fatalf("the object listing for %q rendered no rows", bucket)
	}
	if body.Rows[0].ResourceID != rows[0].ID {
		t.Fatalf("selected row is %q, want %q", body.Rows[0].ResourceID, rows[0].ID)
	}
	return c
}
