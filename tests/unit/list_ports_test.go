package unit_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// openListController builds a Controller via newTestController, pre-navigated
// to a ScreenResourceList for the given resource type ShortName.
func openListController(t *testing.T, shortName string) *app.Controller {
	t.Helper()
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})
	return c
}

// newListController is the helper name other test files call; construction
// routes through openListController.
func newListController(t *testing.T, shortName string) *app.Controller {
	return openListController(t, shortName)
}

// wave3FilterEC2Resources returns 3 EC2 instances distinguishing the filter
// branches under test: private IP and instance type live in Fields (matched
// by the generic Fields-value loop); the third instance carries NO matching
// Name/Fields signal at all — it can only be found via its Findings[].Phrase.
func wave3FilterEC2Resources() []resource.Resource {
	return []resource.Resource{
		{
			ID: "i-0aaa111111111111a", Name: "web-server", Type: "ec2",
			Fields: map[string]string{
				"instance_id": "i-0aaa111111111111a",
				"name":        "web-server",
				"state":       "running",
				"type":        "m5.large",
				"private_ip":  "10.0.48.11",
				"public_ip":   "203.0.113.20",
			},
		},
		{
			ID: "i-0bbb222222222222b", Name: "db-server", Type: "ec2",
			Fields: map[string]string{
				"instance_id": "i-0bbb222222222222b",
				"name":        "db-server",
				"state":       "stopped",
				"type":        "g4dn.xlarge",
				"private_ip":  "10.0.48.55",
				"public_ip":   "",
			},
		},
		{
			ID: "i-0ccc333333333333c", Name: "cache-node", Type: "ec2",
			Fields: map[string]string{
				"instance_id": "i-0ccc333333333333c",
				"name":        "cache-node",
				"state":       "running",
				"type":        "t3.micro",
				"private_ip":  "10.0.99.7",
				"public_ip":   "",
			},
			Findings: []domain.Finding{
				{Code: "ec2.degraded", Phrase: "instance degraded", Severity: domain.SevWarn},
			},
		},
	}
}

func TestListFilter_MatchesFieldsValue_PrivateIP(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3FilterEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "10.0.48"})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 2 {
		t.Fatalf("filter '10.0.48' (private_ip Fields value): Rows count: got %d want 2", len(lb.Rows))
	}
	got := map[string]bool{lb.Rows[0].ResourceID: true, lb.Rows[1].ResourceID: true}
	if !got["i-0aaa111111111111a"] || !got["i-0bbb222222222222b"] {
		t.Errorf("filter '10.0.48': wrong rows matched: %v", got)
	}
}

func TestListFilter_MatchesFieldsValue_PublicIP(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3FilterEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "203.0.113.20"})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("filter '203.0.113.20' (public_ip Fields value): Rows count: got %d want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "i-0aaa111111111111a" {
		t.Errorf("filter '203.0.113.20': wrong ResourceID: got %q", lb.Rows[0].ResourceID)
	}
}

func TestListFilter_MatchesFieldsValue_InstanceType(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3FilterEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "g4dn"})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("filter 'g4dn' (instance-type Fields value): Rows count: got %d want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "i-0bbb222222222222b" {
		t.Errorf("filter 'g4dn': wrong ResourceID: got %q", lb.Rows[0].ResourceID)
	}
}

// cache-node has no Name or Fields value containing "degraded"; only its
// finding phrase matches, case-insensitively.
func TestListFilter_MatchesFindingsPhrase_CaseInsensitive(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3FilterEC2Resources(), nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "DEGRADED"})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("filter 'DEGRADED' (Findings[].Phrase, case-insensitive): Rows count: got %d want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "i-0ccc333333333333c" {
		t.Errorf("filter 'DEGRADED': wrong ResourceID: got %q — should match cache-node via its finding phrase only", lb.Rows[0].ResourceID)
	}
}

// reapplyCheckerAgainst runs only from c.Handle(messages.ResourcesLoaded{...});
// the ApplyResourcesLoaded seam does not call it.

// wave3VPCSGChecker mirrors the shape of a real reverse-scan checker (e.g.
// checkVPCSecurityGroup): matches "sg" resources whose Fields["vpc_id"]
// equals the source VPC's ID.
func wave3VPCSGChecker(_ context.Context, _ any, src resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	entry, ok := cache["sg"]
	if !ok {
		return resource.KnownRelated("sg", nil, false)
	}
	var matched []string
	for _, r := range entry.Resources {
		if r.Fields["vpc_id"] == src.ID {
			matched = append(matched, r.ID)
		}
	}
	if len(matched) == 0 {
		return resource.KnownRelated("sg", nil, false)
	}
	return resource.KnownRelated("sg", matched, false)
}

func wave3SG(id, vpcID string) resource.Resource {
	return resource.Resource{
		ID: id, Name: id, Type: "sg",
		Fields: map[string]string{"group_name": id, "group_id": id, "vpc_id": vpcID},
	}
}

// A (0+) truncated pivot with an empty, non-nil RelatedIDSet hides unrelated
// rows at once and grows across load-more pages.
func TestRelatedCheckerCarry_ZeroInitialGrowsOnLoadMore(t *testing.T) {
	c := openListController(t, "sg")
	c.PatchListReapplyChecker(wave3VPCSGChecker, resource.Resource{ID: "vpc-target"})

	handlePage(c, messages.ResourcesLoaded{ResourceType: "sg", Resources: []resource.Resource{
		wave3SG("sg-1", "vpc-other-1"),
		wave3SG("sg-2", "vpc-other-2"),
	}, Provenance: messages.FetchProvenanceCanonicalList})
	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 0 {
		t.Fatalf("after page1 (no matches): want 0 visible rows, got %d", len(lb.Rows))
	}

	handlePage(c, messages.ResourcesLoaded{ResourceType: "sg", Append: true, Resources: []resource.Resource{
		wave3SG("sg-3", "vpc-target"),
		wave3SG("sg-4", "vpc-other-3"),
	}, Provenance: messages.FetchProvenanceCanonicalList})
	lb = *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("after page2 (1 match): want 1 visible row, got %d", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "sg-3" {
		t.Errorf("after page2: wrong matched row: got %q want sg-3", lb.Rows[0].ResourceID)
	}

	handlePage(c, messages.ResourcesLoaded{ResourceType: "sg", Append: true, Resources: []resource.Resource{
		wave3SG("sg-5", "vpc-target"),
		wave3SG("sg-6", "vpc-target"),
	}, Provenance: messages.FetchProvenanceCanonicalList})
	lb = *c.Snapshot().Body.List
	if len(lb.Rows) != 3 {
		t.Fatalf("after page3 (2 more matches): want 3 visible rows (grown, not reset), got %d", len(lb.Rows))
	}
}

// A non-truncated pivot extends on load-more the same way.
func TestRelatedCheckerCarry_NonTruncatedStillExtends(t *testing.T) {
	c := openListController(t, "sg")
	c.PatchListRelatedIDSet([]string{"sg-a", "sg-b"})
	c.PatchListReapplyChecker(wave3VPCSGChecker, resource.Resource{ID: "vpc-target"})

	if got := len(c.GetListRelatedIDSet()); got != 2 {
		t.Fatalf("precondition: RelatedIDSet size = %d, want 2", got)
	}

	handlePage(c, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "sg", Append: true, Resources: []resource.Resource{
		wave3SG("sg-c", "vpc-target"),
	}})

	if got := len(c.GetListRelatedIDSet()); got != 3 {
		t.Errorf("after load-more match: RelatedIDSet size = %d, want 3 (sg-a, sg-b, sg-c)", got)
	}
}

func TestRelatedCheckerCarry_PreservesSortAfterMerge(t *testing.T) {
	c := openListController(t, "sg")

	alwaysMatch := func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
		entry, ok := cache["sg"]
		if !ok {
			return resource.KnownRelated("sg", nil, false)
		}
		ids := make([]string, len(entry.Resources))
		for i, r := range entry.Resources {
			ids[i] = r.ID
		}
		return resource.KnownRelated("sg", ids, false)
	}
	c.PatchListReapplyChecker(alwaysMatch, resource.Resource{ID: "vpc-target"})
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "group_name"})

	handlePage(c, messages.ResourcesLoaded{ResourceType: "sg", Resources: []resource.Resource{
		wave3SG("sg-zeta", "vpc-target"),
		wave3SG("sg-mu", "vpc-target"),
		wave3SG("sg-alpha", "vpc-target"),
	}, Provenance: messages.FetchProvenanceCanonicalList})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 3 {
		t.Fatalf("want 3 visible rows, got %d", len(lb.Rows))
	}
	got := []string{lb.Rows[0].Cells[0], lb.Rows[1].Cells[0], lb.Rows[2].Cells[0]}
	want := []string{"sg-alpha", "sg-mu", "sg-zeta"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sort not re-applied after checker merge; got order %v, want %v", got, want)
		}
	}
}

func TestRelatedCheckerCarry_NoChecker_Inert(t *testing.T) {
	c := openListController(t, "ec2")
	c.PatchListRelatedIDSet([]string{"i-1", "i-2"})

	handlePage(c, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: []resource.Resource{
		{ID: "i-3", Name: "i-3", Type: "ec2", Fields: map[string]string{"instance_id": "i-3"}},
	}})

	if got := len(c.GetListRelatedIDSet()); got != 2 {
		t.Errorf("without a carried checker, RelatedIDSet must remain untouched by Handle; size = %d, want 2", got)
	}
}

// RenderList places the enrichment glyph at the cell body.IdentityCol and
// body.ScrollX identify, for every catalog type.

func wave3IdentityColResources(td resource.ResourceTypeDef, n int) []resource.Resource {
	statuses := []string{"running", "stopped", "pending", "available", "active", "terminated"}
	lk := td.LifecycleKey
	if lk == "" {
		lk = "state"
	}
	out := make([]resource.Resource, n)
	for i := range n {
		id := fmt.Sprintf("%s-%03d", td.ShortName, i+1)
		fields := make(map[string]string, len(td.Columns)+4)
		for _, col := range td.Columns {
			switch {
			case col.Key == "name" || strings.Contains(strings.ToLower(col.Key), "name"):
				fields[col.Key] = fmt.Sprintf("demo-%s-%d", td.ShortName, i+1)
			case col.Key == lk || col.Key == "state" || col.Key == "status":
				fields[col.Key] = statuses[i%len(statuses)]
			default:
				fields[col.Key] = fmt.Sprintf("v-%s-%d", col.Key, i+1)
			}
		}
		fields["name"] = fmt.Sprintf("demo-%s-%d", td.ShortName, i+1)
		fields[lk] = statuses[i%len(statuses)]
		out[i] = resource.Resource{ID: id, Name: fmt.Sprintf("demo-%s-%d", td.ShortName, i+1), Fields: fields}
	}
	return out
}

// A resource whose Wave-1 Color resolves Healthy and whose r.Findings is
// empty is still shown under the attention filter when it carries a Wave-2
// enrichment finding keyed by ID.

func TestAttentionFilter_IncludesResourcesWithWave2OnlyFindings(t *testing.T) {
	c := openListController(t, "s3")

	// s3's Color always resolves Healthy regardless of Fields, so all three
	// resources below fail the Wave-1 IsIssue() gate — only the Wave-2
	// enrichment-map lookup can surface res-0 under the attention filter.
	resources := []resource.Resource{
		{ID: "res-0", Name: "bucket-alpha", Fields: map[string]string{"name": "bucket-alpha"}},
		{ID: "res-1", Name: "bucket-beta", Fields: map[string]string{"name": "bucket-beta"}},
		{ID: "res-2", Name: "bucket-gamma", Fields: map[string]string{"name": "bucket-gamma"}},
	}
	c.ApplyResourcesLoaded("s3", resources, nil, false)

	findings := map[string][]domain.Finding{
		"res-0": {{Code: "s3.public.access.enabled", Phrase: "public access enabled", Severity: domain.SevWarn, Source: "wave2:s3"}},
	}
	c.ApplyEnrichmentState("s3", 1, false, findings, nil)
	c.Apply(app.Action{Kind: app.ActionToggleAttention})

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("attention filter with Wave-2-only finding: Rows count: got %d want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "res-0" {
		t.Errorf("attention filter must show res-0 (Wave-2 finding only, Wave-1 Color Healthy), got %q", lb.Rows[0].ResourceID)
	}
}

// Enabling the attention filter before Wave 2 lands still surfaces the
// enriched row: every render derives Body.List from the current filter and
// enrichment state.
func TestAttentionFilter_ReappliesOnLateEnrichmentArrival(t *testing.T) {
	c := openListController(t, "s3")

	resources := []resource.Resource{
		{ID: "b-0", Name: "bucket-alpha", Fields: map[string]string{"name": "bucket-alpha"}},
		{ID: "b-1", Name: "bucket-beta", Fields: map[string]string{"name": "bucket-beta"}},
		{ID: "b-2", Name: "bucket-gamma", Fields: map[string]string{"name": "bucket-gamma"}},
	}
	c.ApplyResourcesLoaded("s3", resources, nil, false)

	c.Apply(app.Action{Kind: app.ActionToggleAttention})
	if got := len(c.Snapshot().Body.List.Rows); got != 0 {
		t.Fatalf("precondition: attention filter with no findings yet should hide all healthy rows, got %d visible", got)
	}

	findings := map[string][]domain.Finding{
		"b-0": {{Code: "s3.public.access.enabled", Phrase: "public access enabled", Severity: domain.SevBroken, Source: "wave2:s3"}},
	}
	c.ApplyEnrichmentState("s3", 1, false, findings, nil)

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 1 {
		t.Fatalf("an already-active attention filter must immediately reflect a Wave-2 finding that lands afterward: got %d rows, want 1", len(lb.Rows))
	}
	if lb.Rows[0].ResourceID != "b-0" {
		t.Errorf("attention filter must show b-0 (newly Wave-2-flagged), got %q", lb.Rows[0].ResourceID)
	}
}

// A ResourcesLoaded event applies only to a screen whose canonical
// ResourceType matches the message's: a late fetch for another type never
// populates the active list, and an alias on either side still matches.

func TestResourcesLoaded_DropsMismatchedType(t *testing.T) {
	cases := []struct {
		name          string
		listShortName string // active screen's resource type
		staleType     string // ResourceType on the mismatched message (alias or canonical)
	}{
		{"S3 list rejects EC2 rows", "s3", "ec2"},
		{"EC2 list rejects S3 rows", "ec2", "s3"},
		// "rds" is a registered alias for canonical ShortName "dbi".
		{"S3 list rejects RDS alias rows", "s3", "rds"},
		{"S3 list rejects RDS canonical rows", "s3", "dbi"},
		{"Lambda list rejects EC2 rows", "lambda", "ec2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := openListController(t, tc.listShortName)

			// Deliberately not handlePage: this page names a type the open screen is not,
			// so it belongs to no screen and is delivered exactly as it is — stamping it
			// for the screen on top is the guess the identity exists to remove.
			c.Handle(messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
				ResourceType: tc.staleType,
				Resources: []resource.Resource{
					{ID: "x-0001", Name: "x-0001", Fields: map[string]string{"id": "x-0001"}},
				},
			})

			lb := c.Snapshot().Body.List
			if lb != nil && len(lb.Rows) != 0 {
				t.Errorf("mismatched ResourcesLoaded (%s -> %s) must not populate rows: got %d rows, want 0", tc.staleType, tc.listShortName, len(lb.Rows))
			}
		})
	}
}

func TestResourcesLoaded_AppliesMatchingType(t *testing.T) {
	cases := []struct {
		name          string
		listShortName string
		msgType       string // may be an alias
	}{
		{"S3 exact match", "s3", "s3"},
		{"EC2 exact match", "ec2", "ec2"},
		// The fetcher stamps "rds" (alias) on the wire while the list holds
		// canonical ShortName "dbi" — the alias-aware match must not drop it.
		{"RDS alias match (rds to dbi)", "dbi", "rds"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := openListController(t, tc.listShortName)

			handlePage(c, messages.ResourcesLoaded{
				ResourceType: tc.msgType,
				Resources: []resource.Resource{
					{ID: "res-a", Name: "res-a", Fields: map[string]string{"id": "res-a"}},
					{ID: "res-b", Name: "res-b", Fields: map[string]string{"id": "res-b"}},
				}, Provenance: messages.FetchProvenanceCanonicalList,
			})

			lb := c.Snapshot().Body.List
			if lb == nil || len(lb.Rows) != 2 {
				got := 0
				if lb != nil {
					got = len(lb.Rows)
				}
				t.Errorf("matching-type ResourcesLoaded (%s -> %s) did not populate rows: got %d, want 2", tc.msgType, tc.listShortName, got)
			}
		})
	}
}

// A narrow terminal shrinks a wide column, never drops it. log_events's
// catalog columns (Timestamp:22, Message:120) exceed 80 columns.

func TestRenderList_NarrowScreen_ShrinksWideColumnInsteadOfDropping(t *testing.T) {
	td := resource.GetChildType("log_events")
	if td == nil {
		t.Fatal("log_events child type not registered")
	}

	c := wave3ChildListController(t, "log_events")
	c.ApplyResourcesLoaded("log_events", []resource.Resource{
		{
			ID: "evt-1", Name: "test event",
			Fields: map[string]string{
				"timestamp": "2025-07-25 16:05",
				"message":   "Downloading snowflake_connector_python-3.2.1",
			},
		},
	}, nil, false)
	body := *c.Snapshot().Body.List

	m := views.NewResourceList(*td, nil, keys.Default())
	m.SetSize(80, 20) // narrow — 80 cols can't fit 22+120

	// Colors are on (the package TestMain baseline), so ANSI is stripped before
	// substring checks.
	out := stripAnsi(m.RenderList(body))
	if !strings.Contains(out, "Timestamp") {
		t.Errorf("Timestamp header should be visible on narrow screen:\n%s", out)
	}
	if !strings.Contains(out, "Message") {
		t.Errorf("Message header should be visible on narrow screen (shrunk to fit):\n%s", out)
	}
	if !strings.Contains(out, "Downloading") || !strings.Contains(out, "snowflake") {
		t.Errorf("Message content should be visible (truncated) on narrow screen:\n%s", out)
	}
}

// NewChildResourceList calls PatchListDisplayName and PatchListParentContext.
// The display name is the child breadcrumb; the parent context feeds the
// related-panel ContextKeys lookup (docs/related-resources.md).

// Drives views.NewChildResourceList rather than the setters, so the test
// fails if the constructor stops calling either one.
func TestChildList_DisplayNameAndParentContext_SetByConstructorPath(t *testing.T) {
	c := wave3ChildListController(t, "s3_objects")
	td := resource.GetChildType("s3_objects")
	if td == nil {
		t.Fatal("s3_objects child resource type not registered")
	}

	views.NewChildResourceList(*td, map[string]string{"bucket": "test-bucket"}, "test-bucket", nil, keys.Default(), c)

	if got := c.GetListDisplayName(); got != "test-bucket" {
		t.Errorf("GetListDisplayName(): got %q, want %q", got, "test-bucket")
	}
	if got := c.GetListParentContext()["bucket"]; got != "test-bucket" {
		t.Errorf("GetListParentContext()[bucket]: got %q, want %q", got, "test-bucket")
	}
	if title := c.ListFrameTitle(); title != "test-bucket" {
		t.Errorf("ListFrameTitle() with a DisplayName set and no rows loaded: got %q, want %q", title, "test-bucket")
	}
}

// TestChildList_DisplayName_CombinesWithRowCount verifies the
// DisplayName+count FrameTitle format ("b1(2)") once rows are loaded — the
// display-name substitutes only the leading name, the count suffix behaves
// identically to a top-level list.
func TestChildList_DisplayName_CombinesWithRowCount(t *testing.T) {
	c := wave3ChildListController(t, "s3_objects")
	c.PatchListDisplayName("b1")
	c.ApplyResourcesLoaded("s3_objects", []resource.Resource{
		{ID: "file1.txt", Name: "file1.txt", Fields: map[string]string{"status": "file", "key": "file1.txt"}},
		{ID: "file2.txt", Name: "file2.txt", Fields: map[string]string{"status": "file", "key": "file2.txt"}},
	}, nil, false)

	if title := c.ListFrameTitle(); title != "b1(2)" {
		t.Errorf("ListFrameTitle() with DisplayName + 2 rows: got %q, want %q", title, "b1(2)")
	}
}

func TestChildList_ParentContext_EmptyForTopLevelList(t *testing.T) {
	c := openListController(t, "ec2")
	if got := c.GetListParentContext(); len(got) != 0 {
		t.Errorf("GetListParentContext() on a top-level (non-child) list: got %v, want empty", got)
	}
}

// ct-events sorts by event_time (RFC3339), not the display time string, so
// the default sort holds across a month boundary.

func TestCTEventsSort_RFC3339_AcrossMonthBoundary(t *testing.T) {
	c := openListController(t, "ct-events")
	resources := []resource.Resource{
		{
			ID: "event-a", Name: "GetObject",
			Fields: map[string]string{"time": "Apr 02 10:00:00", "event_time": "2026-04-02T10:00:00Z", "status": "ct-info"},
		},
		{
			ID: "event-b", Name: "DescribeInstances",
			Fields: map[string]string{"time": "Mar 28 10:00:00", "event_time": "2026-03-28T10:00:00Z", "status": "ct-info"},
		},
		{
			ID: "event-c", Name: "PutObject",
			Fields: map[string]string{"time": "Apr 07 17:00:59", "event_time": "2026-04-07T17:00:59Z", "status": "ct-info"},
		},
	}
	c.ApplyResourcesLoaded("ct-events", resources, nil, false)

	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(lb.Rows))
	}
	// Lexicographic display-string order would be B (Mar 28), C (Apr 07), A
	// (Apr 02) — 'M' > 'A' in ASCII, so "Mar 28" wrongly sorts newest. The
	// correct RFC3339 event_time DESC order is C, A, B (newest first).
	got := []string{lb.Rows[0].ResourceID, lb.Rows[1].ResourceID, lb.Rows[2].ResourceID}
	want := []string{"event-c", "event-a", "event-b"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ct-events default sort must use event_time (RFC3339), not the display time string, across a month boundary; got order %v, want %v", got, want)
		}
	}
}

// A Wave-2 finding present only in the enrichment-store map, not on
// r.Findings, still overrides the status cell. loadListController comes from
// phase03_view_reads_test.go.

func TestListStatusColumn_EnrichmentMapOnlyFinding_OverridesRawState(t *testing.T) {
	td := resource.ResourceTypeDef{
		ShortName: "wave3-s4-status-test",
		Name:      "S4 Status Test",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 28},
			{Key: "state", Title: "State", Width: 30},
		},
	}
	c := loadListController(t, td, []resource.Resource{
		{ID: "i-flagged-1", Name: "billing-db-01", Fields: map[string]string{"name": "billing-db-01", "state": "available"}},
	})

	findings := map[string][]domain.Finding{
		"i-flagged-1": {{Code: "dbi.no-backups", Phrase: "no automated backups", Severity: domain.SevWarn, Source: "wave2:dbi"}},
	}
	c.ApplyEnrichmentState(td.ShortName, 0, false, findings, nil)

	body := *c.Snapshot().Body.List
	if len(body.Rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(body.Rows))
	}
	joined := strings.Join(body.Rows[0].Cells, "|")
	if !strings.Contains(joined, "no automated backups") {
		t.Errorf("row must show the enrichment-map finding's Phrase in the status cell (r.Findings not yet mutated); got: %q", joined)
	}
	if strings.Contains(joined, "available") {
		t.Errorf("row must NOT still show the raw AWS state once an enrichment-map finding exists for it; got: %q", joined)
	}
}

// The 't' CloudTrail hint needs a CloudTrailKey and no ParentContext. The
// synthetic type sets CloudTrailKey, so the ParentContext half of the guard
// is what suppresses the hint on a child list.

func hasTKeyHint(hints []app.KeyHint) bool {
	for _, h := range hints {
		if h.Key == "t" {
			return true
		}
	}
	return false
}

func TestListFooterHints_CloudTrailTKey_GatedByParentContext(t *testing.T) {
	td := resource.ResourceTypeDef{
		ShortName:     "wave3-ct-hint-test",
		Name:          "CT Hint Test",
		CloudTrailKey: "ResourceName:ID",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 28},
		},
	}

	t.Run("top-level list: CloudTrailKey set, no parent context → hint present", func(t *testing.T) {
		c := newTestController(t)
		c.RegisterFallbackTypeDef(td)
		c.PushChildListScreen(td.ShortName)

		footer := c.Snapshot().Footer
		if !hasTKeyHint(footer) {
			t.Errorf("footer = %+v, want a %q hint (CloudTrailKey set, ParentContext nil)", footer, "t")
		}
	})

	t.Run("child list: CloudTrailKey set but ParentContext non-nil → hint suppressed", func(t *testing.T) {
		c := newTestController(t)
		c.RegisterFallbackTypeDef(td)
		c.PushChildListScreen(td.ShortName)
		c.PatchListParentContext(map[string]string{"bucket": "my-bucket"})

		footer := c.Snapshot().Footer
		if hasTKeyHint(footer) {
			t.Errorf("footer = %+v, must NOT contain a %q hint once ParentContext is set (child list)", footer, "t")
		}
	})
}
