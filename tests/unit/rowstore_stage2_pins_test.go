// rowstore_stage2_pins_test.go — RowStore is the source of truth for a
// type's rows: docs/design/cache-requirements.md D12 and D16, the
// dispatch-time payload freeze (the requirements doc calls this defect
// class out under the snapshotProbeResourcesForSave doc comment), the
// observed-empty guard on the disk-store fallback, and the
// sweep-vs-list-lane save depth.
package unit_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// newStage2PinTestController mirrors newSeededTestController/
// newDifferentialTestController (same package, same isolated-disk-store
// precondition via A9S_CONFIG_FOLDER redirected to t.TempDir()) — duplicated
// as a small variant so this file has no cross-file coupling to another test
// file's helper lifetime.
func newStage2PinTestController(t *testing.T) (*session.Session, *runtime.Core, *app.Controller) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	return s, core, c
}

// stage2PinReadTypeFile re-reads the on-disk TypeFile for shortName under
// (profile, region), failing the test if it is missing. A local variant of
// qa_cache_lifecycle_test.go's readTypeFile (package unit, not unit_test —
// this file's package cannot see it directly) so the D16 lockstep pin can use
// the identical byte-compare pattern the dispatch calls out
// (qa_cache_lifecycle_test.go:219).
func stage2PinReadTypeFile(t *testing.T, profile, region, shortName string) cache.TypeFile {
	t.Helper()
	store := cache.LoadDirForTest(profile, region)
	tf, ok := store.Type(shortName)
	if !ok {
		t.Fatalf("cache.LoadDirForTest(%q, %q).Type(%q) missing — expected a persisted TypeFile", profile, region, shortName)
	}
	return tf
}

// -----------------------------------------------------------------------
// Pin 1 — D12 strengthened: post-sweep-completion list open still seeds
// title+rows+enriched field, no disk-store fallback needed.
// -----------------------------------------------------------------------

// stage2PinWave2Type is a catalog-absent Wave-2 short name registered so the
// enrichment queue drains on the pinned type's own EnrichmentChecked
// delivery — a single-type queue makes that delivery the terminal one,
// exercising the exact "all done" free (handlers_availability.go's
// ProbeResources/ProbeTruncated nil-out) that D12 was originally caught by.
const stage2PinType = "ec2"

// TestStage2Pin_D12_PostSweepListOpen_SeedsTitleRowsAndEnrichedField drives
// the full flow D12 describes: probe → enrichment completes → HandleNavigate
// (via Controller.Apply, the same seam app_cache_first_seeding_test.go uses)
// opens the list. Asserts the seed comes through with BOTH the row set
// (title-driving count) AND the enriched field — sourced from RowStore, not
// from a disk-store fallback (this test's isolated A9S_CONFIG_FOLDER has no
// on-disk file for this type at all, so a passing assertion cannot be
// explained by the disk fallback branch in HandleNavigate). RowStore has no
// free at enrichment-completion, so a warm list open AFTER the enrichment
// sweep completed never renders a bare Loading… shell.
func TestStage2Pin_D12_PostSweepListOpen_SeedsTitleRowsAndEnrichedField(t *testing.T) {
	s, _, c := newStage2PinTestController(t)

	seed := []resource.Resource{
		{ID: "i-0d12seed0001", Name: "d12-seed-1", Type: stage2PinType, Fields: map[string]string{"state": "running"}},
	}
	_, _ = c.Handle(messages.AvailabilityChecked{
		ResourceType: stage2PinType,
		HasResources: true,
		Count:        1,
		Gen:          s.AvailabilityGen,
		Resources:    seed,
	})
	if s.EnrichTotal != 1 {
		t.Fatalf("precondition: want EnrichTotal=1 (single pinned type) so the next EnrichmentChecked is the terminal, sweep-completing one, got %d", s.EnrichTotal)
	}

	_, _ = c.Handle(messages.EnrichmentChecked{
		ResourceType: stage2PinType,
		Gen:          s.EnrichmentGen,
		TypeGen:      s.EnrichmentTypeGen[stage2PinType],
		FieldUpdates: map[string]map[string]string{
			// instance_status is a real ec2 list column (.a9s/views/ec2.yaml's
			// "Health" column, key: instance_status) — cost_estimate is not a
			// registered column for ec2 and would never surface in Cells
			// regardless of RowStore/legacy-map sourcing, making it unfit to
			// prove the "enriched field renders" half of this pin.
			"i-0d12seed0001": {"instance_status": "42.00"},
		},
	})
	if s.EnrichChecked < s.EnrichTotal {
		t.Fatalf("precondition: want the sweep-completion ('all done') branch to have fired, got EnrichChecked=%d EnrichTotal=%d", s.EnrichChecked, s.EnrichTotal)
	}

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: stage2PinType})
	snap := c.Snapshot()

	if snap.Body.Kind != app.BodyKindList {
		t.Fatalf("Body.Kind = %q, want %q", snap.Body.Kind, app.BodyKindList)
	}
	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening the list post-sweep-completion")
	}
	if lb.Loading {
		t.Error("Loading = true, want false — a post-sweep list open must still seed rows (D12), not show a bare Loading shell")
	}
	if len(lb.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1 — D12: the seed must come through even though the sweep-completion free nils legacy ProbeResources", len(lb.Rows))
	}
	row := lb.Rows[0]
	if row.ResourceID != "i-0d12seed0001" {
		t.Fatalf("Rows[0].ResourceID = %q, want i-0d12seed0001", row.ResourceID)
	}
	found := false
	for _, cell := range row.Cells {
		if cell == "42.00" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Rows[0].Cells = %v, want the enriched instance_status=42.00 field to be present in the seeded row (D12 requires title+ROWS+enriched field, not a bare count)", row.Cells)
	}
}

// -----------------------------------------------------------------------
// Pin 2 — the dispatch-time payload freeze restated structurally: the enrichment-completion save
// payload equals the store snapshot AT DISPATCH TIME even when a later
// Amend lands before the executor runs.
// -----------------------------------------------------------------------

// TestStage2Pin_SavePayloadFrozenAtDispatch_SurvivesLaterAmend drives an
// enrichment-completion sweep to termination (producing a TaskKindSaveCache
// dispatch carrying a SaveCachePayload), then applies a LATER Amend (a
// second, distinct enrichment landing for the same type with different
// field values, simulating a rerun's FieldUpdates fold arriving after the
// save was already dispatched) and asserts the ALREADY-DISPATCHED payload's
// rows still carry the ORIGINAL field value, not the later Amend's — the
// "snapshot BEFORE any subsequent same-call OR later-call mutation"
// contract. RowStore.SnapshotAll documents the by-construction immunity
// (cloneRows: fresh slice + fresh Fields map per row); a save that
// snapshotted lazily (a reference into the live store instead of
// SnapshotAll at dispatch time) fails this.
func TestStage2Pin_SavePayloadFrozenAtDispatch_SurvivesLaterAmend(t *testing.T) {
	s, core, c := newStage2PinTestController(t)

	seed := []resource.Resource{
		{ID: "i-0frozenseed0001", Name: "frozen-seed-1", Type: stage2PinType, Fields: map[string]string{"state": "running"}},
	}
	_, _ = c.Handle(messages.AvailabilityChecked{
		ResourceType: stage2PinType,
		HasResources: true,
		Count:        1,
		Gen:          s.AvailabilityGen,
		Resources:    seed,
	})
	if s.EnrichTotal != 1 {
		t.Fatalf("precondition: want EnrichTotal=1, got %d", s.EnrichTotal)
	}

	_, tasks := c.Handle(messages.EnrichmentChecked{
		ResourceType: stage2PinType,
		Gen:          s.EnrichmentGen,
		TypeGen:      s.EnrichmentTypeGen[stage2PinType],
		FieldUpdates: map[string]map[string]string{
			"i-0frozenseed0001": {"cost_estimate": "10.00"},
		},
	})

	var payload *runtime.SaveCachePayload
	for _, tr := range tasks {
		if tr.Key.Kind == runtime.TaskKindSaveCache {
			p, ok := tr.Payload.(*runtime.SaveCachePayload)
			if !ok {
				t.Fatalf("TaskKindSaveCache payload type = %T, want *runtime.SaveCachePayload", tr.Payload)
			}
			payload = p
		}
	}
	if payload == nil {
		t.Fatal("no TaskKindSaveCache task dispatched by the sweep-completion EnrichmentChecked — precondition for the payload-freeze pin failed")
	}
	dispatchedRows := payload.Resources[stage2PinType]
	if len(dispatchedRows) != 1 || dispatchedRows[0].Fields["cost_estimate"] != "10.00" {
		t.Fatalf("dispatched payload rows = %+v, want single row with cost_estimate=10.00", dispatchedRows)
	}

	// Simulate a LATER Amend landing after dispatch but before the executor
	// runs (e.g. a rerun's FieldUpdates fold for the same type).
	core.AmendRows(stage2PinType, func(rows []resource.Resource) []resource.Resource {
		out := make([]resource.Resource, len(rows))
		for i, r := range rows {
			fields := make(map[string]string, len(r.Fields))
			for k, v := range r.Fields {
				fields[k] = v
			}
			fields["cost_estimate"] = "999.99"
			r.Fields = fields
			out[i] = r
		}
		return out
	})

	if dispatchedRows[0].Fields["cost_estimate"] != "10.00" {
		t.Errorf("dispatched payload rows[0].Fields[cost_estimate] = %q after a later Amend, want unchanged 10.00 — the dispatch-time payload freeze: a dispatched save payload must be frozen at dispatch time, immune to any later store mutation", dispatchedRows[0].Fields["cost_estimate"])
	}

	storeSnap := s.RowStore.Snapshot(stage2PinType)
	if len(storeSnap.Rows) != 1 || storeSnap.Rows[0].Fields["cost_estimate"] != "999.99" {
		t.Errorf("live RowStore snapshot after Amend = %+v, want cost_estimate=999.99 — the live store itself SHOULD reflect the later Amend (only the already-dispatched payload must stay frozen)", storeSnap.Rows)
	}
}

// -----------------------------------------------------------------------
// Pin 3 — D16 lockstep: open list → load-more to depth 2 →
// sweep-completion save persists the SAME accumulated depth the list lane
// would save. No SyncProbeResourcesForType caller left in production.
// -----------------------------------------------------------------------

// TestStage2Pin_SweepSaveMatchesListLaneDepth_NoSyncCallerLeft pins
// the byte-equivalence contract: after a list-open + one load-more append
// (accumulated depth 2), an independent sweep-completion save (a SEPARATE,
// smaller probe result for the same type landing via EnrichmentChecked's
// "all done" branch — the exact D16 scenario, where the sweep lane's own
// first-page-only probe result would otherwise stomp the list lane's deeper
// accumulated rows) must persist the list lane's FULL accumulated depth, not
// the sweep's shallower snapshot — using the readTypeFile byte-compare
// pattern from qa_cache_lifecycle_test.go. Also asserts (via source grep)
// that no production caller of SyncProbeResourcesForType exists: RowStore's
// own lockstep through ObserveRows/AmendRows carries the depth-2
// accumulation without a sync function.
func TestStage2Pin_SweepSaveMatchesListLaneDepth_NoSyncCallerLeft(t *testing.T) {
	t.Run("no_production_caller_of_SyncProbeResourcesForType_remains", func(t *testing.T) {
		out, err := caseInsensitiveGrepSyncProbeResourcesForTypeCallers(t)
		if err != nil {
			t.Fatalf("grep for SyncProbeResourcesForType callers failed: %v", err)
		}
		if out != "" {
			t.Errorf("production caller(s) of SyncProbeResourcesForType still present after Stage 2 (want the function and every caller deleted per the plan):\n%s", out)
		}
	})

	s, _, c := newStage2PinTestController(t)

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	page1 := []resource.Resource{
		{ID: "bucket-stage2-1", Name: "bucket-stage2-1", Type: "s3"},
		{ID: "bucket-stage2-2", Name: "bucket-stage2-2", Type: "s3"},
	}
	_, _ = handlePage(c, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    page1,
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-1"},
		Append:       false,
		Gen:          0, Provenance: messages.FetchProvenanceCanonicalList,
	})
	page2 := []resource.Resource{
		{ID: "bucket-stage2-3", Name: "bucket-stage2-3", Type: "s3"},
	}
	_, _ = handlePage(c, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    page2,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Append:       true,
		Gen:          0, Provenance: messages.FetchProvenanceCanonicalList,
	})

	lb := c.Snapshot().Body.List
	if lb == nil || len(lb.Rows) != 3 {
		t.Fatalf("precondition: list-lane accumulated rows after load-more = %+v, want 3 (depth 2: page1=2 + page2=1)", lb)
	}

	// The list lane's own save already fired as a side effect of the load-more
	// ResourcesLoaded Handle call above: Controller.handleResourcesLoadedEvent
	// -> syncExactTotalToMenu -> maybeSaveResourceListCache persists ls.Rows
	// unconditionally on every ResourcesLoaded delivery (core/app/
	// handle.go), not just on list-close — so no separate save call is needed
	// here.
	// The save the delivery above queued runs on the cache writer's goroutine,
	// so the file it produces is read after waiting for it.
	c.WaitForCacheWrites()
	listLaneTF := stage2PinReadTypeFile(t, s.Profile, s.Region, "s3")
	if len(listLaneTF.Rows) != 3 {
		t.Fatalf("list-lane save persisted %d rows, want 3 (precondition for the D16 comparison)", len(listLaneTF.Rows))
	}

	// Independent sweep-completion save for the SAME type: a Wave-1 probe
	// result carrying only ONE row (the D16 scenario's shallower, independent
	// AWS list call) followed by its own enrichment-completion "all done"
	// save — must NOT stomp the list lane's deeper 3-row file with its own
	// shallower snapshot.
	sweepSeed := []resource.Resource{
		{ID: "bucket-stage2-1", Name: "bucket-stage2-1", Type: "s3"},
	}
	_, _ = c.Handle(messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: true,
		Count:        1,
		Gen:          s.AvailabilityGen,
		Resources:    sweepSeed,
	})
	if s.EnrichTotal < 1 {
		t.Fatalf("precondition: expected s3 to enter the enrichment queue after AvailabilityChecked, EnrichTotal=%d", s.EnrichTotal)
	}
	_, _ = c.Handle(messages.EnrichmentChecked{
		ResourceType: "s3",
		Gen:          s.EnrichmentGen,
		TypeGen:      s.EnrichmentTypeGen["s3"],
	})

	sweepLaneTF := stage2PinReadTypeFile(t, s.Profile, s.Region, "s3")
	if len(sweepLaneTF.Rows) != len(listLaneTF.Rows) {
		t.Fatalf("sweep-completion save persisted %d rows, want the SAME accumulated depth the list lane saved (%d) — D16: an independent, shallower sweep observation must not stomp the deeper list-lane rows", len(sweepLaneTF.Rows), len(listLaneTF.Rows))
	}
	listIDs := make(map[string]bool, len(listLaneTF.Rows))
	for _, r := range listLaneTF.Rows {
		listIDs[r.ID] = true
	}
	for _, r := range sweepLaneTF.Rows {
		if !listIDs[r.ID] {
			t.Errorf("sweep-lane row ID %q not present in list-lane's row set %v — the two lanes disagree on which rows are persisted", r.ID, listIDs)
		}
	}
}

// caseInsensitiveGrepSyncProbeResourcesForTypeCallers scans core/runtime
// and core/app production Go source (*.go, excluding *_test.go) for any
// CODE reference to the identifier "SyncProbeResourcesForType", declaration
// or call. Returns a "path:line: text" report of every match found (empty
// when none), or an error if the source tree could not be walked.
//
// Pure comment lines (first non-whitespace characters "//") are skipped: a
// comment naming the identifier is explanatory prose, not a caller or
// declaration, and asserting against it would make this pin permanently
// unsatisfiable rather than tracking the actual "is the function
// called/declared" invariant.
func caseInsensitiveGrepSyncProbeResourcesForTypeCallers(t *testing.T) (string, error) {
	t.Helper()
	const needle = "SyncProbeResourcesForType"
	roots := []string{
		"../../core/runtime",
		"../../core/app",
		"../../internal/tui",
	}
	var hits []string
	for _, root := range roots {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			for i, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "//") {
					continue
				}
				if strings.Contains(line, needle) {
					hits = append(hits, fmt.Sprintf("%s:%d: %s", path, i+1, strings.TrimSpace(line)))
				}
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	return strings.Join(hits, "\n"), nil
}

// -----------------------------------------------------------------------
// Pin 4 — Observed-empty: a live probe returning zero rows must
// seed an EMPTY list on navigation. Origin=Probe empty beats Origin=Disk
// rows; no stale disk rows resurrect.
// -----------------------------------------------------------------------

// TestStage2Pin_ObservedEmptyProbe_BeatsStaleDiskRows pins the
// "observed-empty is fresher than any disk row" rule the handlers_navigate.go
// doc comment states explicitly: a live Wave-1 probe confirming a type is
// genuinely empty this session must seed a BARE list on navigation — even
// when a populated, stale on-disk per-type cache file exists for the same
// pair. The disk fallback must never resurrect rows a live probe has already
// superseded. RowStore.Observe's Origin field (OriginProbe vs OriginDisk)
// and the Disk-never-overwrites-Fetch/Probe rule carry this precedence; a
// store keyed only by "has any TypeRows entry ever been written" would
// conflate "never observed" with "observed empty" and fall through to a
// stale disk seed.
func TestStage2Pin_ObservedEmptyProbe_BeatsStaleDiskRows(t *testing.T) {
	s, core, c := newStage2PinTestController(t)

	store := core.EnsureCacheStore()
	if store == nil {
		t.Fatal("core.EnsureCacheStore() = nil — test fixture requires a live disk store to seed stale rows into")
	}
	store.Put(stage2PinType, cache.TypeFile{
		HasResources: true,
		Count:        1,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "i-0staledisk0001", Name: "stale-disk-row", Fields: map[string]string{"state": "running"}},
		},
	})
	if err := store.SaveType(stage2PinType); err != nil {
		t.Fatalf("seed fixture SaveType(%s): %v", stage2PinType, err)
	}

	// Live Wave-1 probe observes the type as genuinely empty this session.
	_, _ = c.Handle(messages.AvailabilityChecked{
		ResourceType: stage2PinType,
		HasResources: false,
		Count:        0,
		Gen:          s.AvailabilityGen,
		Resources:    []resource.Resource{},
	})

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: stage2PinType})
	snap := c.Snapshot()

	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening the list for an observed-empty type")
	}
	if len(lb.Rows) != 0 {
		t.Errorf("Rows = %+v, want empty — the observed-empty guard: a live observed-empty probe result must beat a stale populated disk cache, not resurrect its rows", lb.Rows)
	}
	for _, r := range lb.Rows {
		if r.ResourceID == "i-0staledisk0001" {
			t.Error("stale disk row i-0staledisk0001 resurrected into the seeded list despite a live observed-empty probe result")
		}
	}
}

// -----------------------------------------------------------------------
// Pin 5 — Issue-count parity: unifiedIssueCount-driven menu badge equals the
// same aggregation computed from the store rows.
// -----------------------------------------------------------------------

// handCountDistinctIssueRows returns the count of rows carrying at least one
// finding — a faithful hand-count only when every finding in the row set is
// domain.SevBroken. unifiedIssueCount's wave2 contribution
// (handlers_availability.go:701-708) counts a finding toward the badge ONLY
// at SevBroken; a SevWarn-only row set would make this helper overcount
// relative to the production aggregation (see TestUnifiedIssueCount_
// IgnoresTildeSeverityFindings). Callers must seed SevBroken findings for
// this helper's count to remain a real cross-check rather than diverging
// from the enforced contract.
func handCountDistinctIssueRows(rows []resource.Resource) int {
	n := 0
	for _, r := range rows {
		if len(r.Findings) > 0 {
			n++
		}
	}
	return n
}

// TestStage2Pin_IssueCountParity_MenuBadgeMatchesStoreRowAggregation drives a
// probe+enrichment cycle that leaves two rows findings-bearing and one
// healthy, then asserts the menu's issue-count badge
// (Controller.GetMenuIssueCounts) equals a hand-count performed directly over
// RowStore's own retained rows for the type — the two aggregations (the
// production unifiedIssueCount path feeding the menu badge, and a
// store-rows-first-principles count) must agree. unifiedIssueCount reads
// RowStore.Snapshot(canon).Rows; a read of a DIFFERENT rows view (a
// partial-included snapshot, or a stale pre-Amend snapshot) would desync the
// menu badge from the store's own canonical row set.
func TestStage2Pin_IssueCountParity_MenuBadgeMatchesStoreRowAggregation(t *testing.T) {
	s, _, c := newStage2PinTestController(t)

	// Fields carry state "stopped"/"running" for realism only — colorEC2
	// derives entirely from r.Findings (colorFromAnyFinding), never
	// r.Fields["state"], so wave1 alone contributes zero issue-colored rows
	// here; the whole badge count below comes from the wave2 SevBroken
	// findings seeded further down.
	seed := []resource.Resource{
		{ID: "i-0issuerow0001", Name: "issue-row-1", Type: stage2PinType, Fields: map[string]string{"state": "stopped"}},
		{ID: "i-0issuerow0002", Name: "issue-row-2", Type: stage2PinType, Fields: map[string]string{"state": "stopped"}},
		{ID: "i-0issuerow0003", Name: "healthy-row-1", Type: stage2PinType, Fields: map[string]string{"state": "running"}},
	}
	_, _ = c.Handle(messages.AvailabilityChecked{
		ResourceType: stage2PinType,
		HasResources: true,
		Count:        3,
		Gen:          s.AvailabilityGen,
		Resources:    seed,
	})
	if s.EnrichTotal != 1 {
		t.Fatalf("precondition: want EnrichTotal=1, got %d", s.EnrichTotal)
	}

	findings := map[string][]domain.Finding{
		"i-0issuerow0001": {{Code: "ec2-stopped-has-eip", Phrase: "stopped, has EIP", Severity: domain.SevBroken, Source: "wave2:ec2"}},
		"i-0issuerow0002": {{Code: "ec2-stopped-has-eip", Phrase: "stopped, has EIP", Severity: domain.SevBroken, Source: "wave2:ec2"}},
	}
	_, _ = c.Handle(messages.EnrichmentChecked{
		ResourceType: stage2PinType,
		Gen:          s.EnrichmentGen,
		TypeGen:      s.EnrichmentTypeGen[stage2PinType],
		Findings:     findings,
	})

	menuCounts := c.GetMenuIssueCounts()
	if menuCounts == nil {
		t.Fatal("GetMenuIssueCounts() = nil after an enrichment cycle produced findings")
	}
	badge, ok := menuCounts[stage2PinType]
	if !ok {
		t.Fatalf("GetMenuIssueCounts()[%q] missing, want a badge entry after enrichment", stage2PinType)
	}

	storeSnap := s.RowStore.Snapshot(stage2PinType)
	handCount := handCountDistinctIssueRows(storeSnap.Rows)
	if handCount != 2 {
		t.Fatalf("precondition: hand-count over store rows = %d, want 2 (two findings-bearing rows seeded)", handCount)
	}

	if badge != handCount {
		t.Errorf("menu issue badge = %d, hand-count over RowStore rows = %d — the production unifiedIssueCount aggregation feeding the menu badge must agree with the store's own row set", badge, handCount)
	}
}
