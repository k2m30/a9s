// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_identities_test.go pins the four identities that make a deferred
// piece of work still know what it was for: which visit to a pair a queued
// save belongs to, which observation a frozen snapshot froze, which screen a
// body was built for, and which window an anomaly overlay covers.
package unit_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// --- row 41: which visit to a pair -----------------------------------------

// TestQueuedSave_ForAnEarlierVisitToTheSamePair pins row 41. A save is
// prepared while pair A is current. The operator switches to B, B's rows
// populate the store, and switches back to A. The pair's name is the same on
// both visits, so a guard that reads only the name lets B's counts land in
// A's file.
func TestQueuedSave_ForAnEarlierVisitToTheSamePair(t *testing.T) {
	b := newWipfixBench(t)
	cfg := os.Getenv("A9S_CONFIG_FOLDER")
	s := b.core.Session()

	// Visit one to A: a real population, saved. The visit is entered the way
	// the app enters one, so the pair carries the generation of this visit and
	// not the zero of a session that has not resolved a pair yet.
	s.SetProfileRegion("example-readonly", "us-east-1")
	s.Rotate()
	firstVisit := s.CurrentPairValue()
	s.RowStore.Observe("ec2", wipfixSaveLaneRows(40),
		&domain.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	firstGen, _ := s.RowStore.SnapshotMeta("ec2")
	if err := b.core.SaveTypeRows(
		runtime.SaveTarget{Pair: firstVisit, ObsGen: firstGen, Type: "ec2", ExactPopulation: true},
		runtime.SaveContent{Resources: wipfixSaveLaneRows(40), Count: 40}); err != nil {
		t.Fatalf("A's own save: %v", err)
	}
	b.c.WaitForCacheWrites()

	// Away to B and back. Both visits answer to the same name.
	s.SetProfileRegion("example-other", "eu-west-1")
	s.Rotate()
	s.RowStore.Observe("ec2", wipfixSaveLaneRows(3),
		&domain.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	bGen, _ := s.RowStore.SnapshotMeta("ec2")
	s.SetProfileRegion(firstVisit.Profile, firstVisit.Region)
	s.Rotate()

	// The save prepared while B was current finally runs, carrying the rows it
	// froze there and the pair it was prepared for.
	if err := b.core.SaveTypeRows(
		runtime.SaveTarget{Pair: firstVisit, ObsGen: bGen, Type: "ec2", ExactPopulation: true},
		runtime.SaveContent{Resources: wipfixSaveLaneRows(3), Count: 3}); err != nil {
		t.Fatalf("the queued save returned an error rather than declining: %v", err)
	}
	b.c.WaitForCacheWrites()

	versions := wipfixWatchTypeFile(t, cfg, "ec2")
	last := versions[len(versions)-1]
	if strings.Contains(last, "count: 3") {
		t.Errorf("A's ec2 file records the 3 rows the operator saw under a different pair — "+
			"the queued save named its pair and not its visit, and the name came back\n%s",
			wipfixTypeFileHead(last))
	}
	if !strings.Contains(last, "count: 40") {
		t.Errorf("A's ec2 file no longer records its own 40 rows\n%s", wipfixTypeFileHead(last))
	}
}

// TestQueuedSave_ForTheCurrentVisitStillLands is the negative half: rejecting
// an earlier visit must not reject the visit that is happening.
func TestQueuedSave_ForTheCurrentVisitStillLands(t *testing.T) {
	b := newWipfixBench(t)
	cfg := os.Getenv("A9S_CONFIG_FOLDER")
	s := b.core.Session()

	s.RowStore.Observe("ec2", wipfixSaveLaneRows(7),
		&domain.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	if err := b.core.SaveAvailabilityFromRows(s.CurrentPairValue()); err != nil {
		t.Fatalf("SaveAvailabilityFromRows: %v", err)
	}
	b.c.WaitForCacheWrites()

	versions := wipfixWatchTypeFile(t, cfg, "ec2")
	if last := versions[len(versions)-1]; !strings.Contains(last, "count: 7") {
		t.Errorf("the current visit's save did not land\n%s", wipfixTypeFileHead(last))
	}
}

// --- row 42: which observation was frozen ----------------------------------

// TestFrozenSweepSave_OlderThanTheLatestObservation pins row 42. A sweep
// freezes an exact 100 rows; before its save runs, a foreground refresh finds
// the type really holds 90 and saves that. The frozen write is later only
// because it was queued later, and accepting it leaves an obsolete population
// on disk that every subsequent refresh writes and the next sweep overwrites
// again.
func TestFrozenSweepSave_OlderThanTheLatestObservation(t *testing.T) {
	b := newWipfixBench(t)
	cfg := os.Getenv("A9S_CONFIG_FOLDER")
	s := b.core.Session()
	pair := s.CurrentPairValue()

	// The sweep's snapshot, and the generation the store was at when it froze.
	s.RowStore.Observe("ec2", wipfixSaveLaneRows(100),
		&domain.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	frozenGen, _ := s.RowStore.SnapshotMeta("ec2")
	if frozenGen == 0 {
		t.Fatal("precondition: the row store has no generation for ec2")
	}
	frozen := map[string][]resource.Resource{"ec2": wipfixSaveLaneRows(100)}

	// The foreground refresh observes the smaller, current, exact population.
	s.RowStore.Observe("ec2", wipfixSaveLaneRows(90),
		&domain.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	newGen, _ := s.RowStore.SnapshotMeta("ec2")
	if err := b.core.SaveTypeRows(
		runtime.SaveTarget{Pair: pair, ObsGen: newGen, Type: "ec2", ExactPopulation: true},
		runtime.SaveContent{Resources: wipfixSaveLaneRows(90), Count: 90}); err != nil {
		t.Fatalf("foreground save: %v", err)
	}
	b.c.WaitForCacheWrites()

	// The sweep's save finally executes, carrying the generation it froze at.
	if _, err := b.core.ExecuteTaskAt(context.Background(), runtime.TaskRequest{
		Key: runtime.TaskKey{Kind: runtime.TaskKindSaveCache},
		Payload: &runtime.SaveCachePayload{
			Resources: frozen,
			Truncated: map[string]bool{"ec2": false},
			Gens:      map[string]domain.Gen{"ec2": frozenGen},
		},
	}, b.core.CaptureDispatch()); err != nil {
		t.Fatalf("frozen sweep save: %v", err)
	}
	b.c.WaitForCacheWrites()

	versions := wipfixWatchTypeFile(t, cfg, "ec2")
	last := versions[len(versions)-1]
	if strings.Contains(last, "count: 100") {
		t.Errorf("the ec2 file records 100 rows after a newer observation found 90 — the "+
			"snapshot is older than the save it overwrote, and its write is later only "+
			"because it was queued later\n%s", wipfixTypeFileHead(last))
	}
	if !strings.Contains(last, "count: 90") {
		t.Errorf("the ec2 file does not record the 90 the newest observation found\n%s",
			wipfixTypeFileHead(last))
	}
}

// TestFrozenSweepSave_AtTheLatestObservationStillLands is the negative half: a
// snapshot nothing has superseded is the one that should be written.
func TestFrozenSweepSave_AtTheLatestObservationStillLands(t *testing.T) {
	b := newWipfixBench(t)
	cfg := os.Getenv("A9S_CONFIG_FOLDER")
	s := b.core.Session()

	s.RowStore.Observe("ec2", wipfixSaveLaneRows(100),
		&domain.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	gen, _ := s.RowStore.SnapshotMeta("ec2")

	if _, err := b.core.ExecuteTaskAt(context.Background(), runtime.TaskRequest{
		Key: runtime.TaskKey{Kind: runtime.TaskKindSaveCache},
		Payload: &runtime.SaveCachePayload{
			Resources: map[string][]resource.Resource{"ec2": wipfixSaveLaneRows(100)},
			Truncated: map[string]bool{"ec2": false},
			Gens:      map[string]domain.Gen{"ec2": gen},
		},
	}, b.core.CaptureDispatch()); err != nil {
		t.Fatalf("sweep save: %v", err)
	}
	b.c.WaitForCacheWrites()

	versions := wipfixWatchTypeFile(t, cfg, "ec2")
	if last := versions[len(versions)-1]; !strings.Contains(last, "count: 100") {
		t.Errorf("an unsuperseded sweep snapshot did not land\n%s", wipfixTypeFileHead(last))
	}
}

// --- row 48: which screen the body was built for ---------------------------

// TestPoppedScreensBody_DoesNotRenderOnItsSuccessor pins row 48. The list body
// for a freshly absorbed result is built off the controller lock and swapped in
// when the lock comes back. If the screen it was built for is popped inside
// that window, the screen underneath is of the same type and agrees on every
// field the memo key records — rows generation, filter, sort, enrichment — so
// neither the type nor the key can tell the build's owner from its successor.
// The body must go to the screen it was built for or nowhere.
func TestPoppedScreensBody_DoesNotRenderOnItsSuccessor(t *testing.T) {
	td := wipfixObjTypeDef()

	// Two drills over one type, each over its own set of ids: everything the
	// memo key records agrees, and only the set each screen filters by differs.
	all := append(wipfixDrillRows("b", 3), wipfixDrillRows("a", 8000)...)
	bIDs := wipfixDrillIDs("b", 3)
	aIDs := wipfixDrillIDs("a", 8000)

	// The window is a real interleaving, not a seam: several attempts, with the
	// pop aimed at different points inside a build long enough to be hit.
	for attempt := 0; attempt < 24; attempt++ {
		b := newWipfixBench(t)
		b.c.RegisterFallbackTypeDef(td)

		b.c.PushChildListScreen(td.ShortName)
		b.c.SeedRelatedExactRows(td.ShortName, bIDs)
		if _, err := b.c.Handle(messages.ResourcesLoaded{
			ResourceType: td.ShortName,
			Resources:    all,
			Pagination:   &domain.PaginationMeta{IsTruncated: false},
			Provenance:   messages.FetchProvenanceChild,
		}); err != nil {
			t.Fatalf("the underneath drill's result: %v", err)
		}

		b.c.PushChildListScreen(td.ShortName)
		b.c.SeedRelatedExactRows(td.ShortName, aIDs)
		popped := make(chan struct{})
		go func() {
			defer close(popped)
			time.Sleep(time.Duration(attempt) * 40 * time.Microsecond)
			_, _ = b.c.Apply(app.Action{Kind: app.ActionBack})
		}()
		if _, err := b.c.Handle(messages.ResourcesLoaded{
			ResourceType: td.ShortName,
			Resources:    all,
			Pagination:   &domain.PaginationMeta{IsTruncated: false},
			Provenance:   messages.FetchProvenanceChild,
		}); err != nil {
			t.Fatalf("the top drill's result: %v", err)
		}
		<-popped

		rows := b.list(t).Rows
		for _, r := range rows {
			if strings.HasPrefix(r.ResourceID, "a-") {
				t.Fatalf("attempt %d: the drill underneath renders %q, a row of the screen popped "+
					"out from under the build — a body built for one screen installed into "+
					"another of the same type", attempt, r.ResourceID)
			}
		}
		if len(rows) != len(bIDs) {
			t.Fatalf("attempt %d: the drill underneath renders %d row(s), want its own %d",
				attempt, len(rows), len(bIDs))
		}
	}
}

// wipfixDrillIDs names the same rows wipfixDrillRows builds.
func wipfixDrillIDs(prefix string, n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("%s-%03d", prefix, i)
	}
	return ids
}

// wipfixDrillRows builds n rows carrying a per-screen prefix, so a row rendered
// on the wrong screen names the screen it came from.
func wipfixDrillRows(prefix string, n int) []resource.Resource {
	rows := make([]resource.Resource, n)
	for i := range rows {
		id := fmt.Sprintf("%s-%03d", prefix, i)
		rows[i] = resource.Resource{ID: id, Name: id, Fields: map[string]string{"key": id}}
	}
	return rows
}

// TestAnomalyOverlay_WideWindowPrefersTheOverlayThatCoversIt pins row 49. The
// authoritative set is complete for twelve months and says nothing about the
// years before them. Asked about a multi-year window it is not an answer, and
// preferring it hides both the wider partial marks and the warning that they
// are a lower bound.
func TestAnomalyOverlay_WideWindowPrefersTheOverlayThatCoversIt(t *testing.T) {
	store := costs.NewMemoryStore("example-readonly")
	q := wipfixOverlayQuery()
	now := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	recent := wipfixMonths(2025, 12)
	wide := append(wipfixMonths(2023, 12), append(wipfixMonths(2024, 12), recent...)...)

	// The complete answer for the last twelve months.
	store.ApplyFetchResult(costs.FetchResult{
		Query:    costs.Query{Granularity: q.Granularity, GroupBy: q.GroupBy, Range: wipfixSpan(recent)},
		Coverage: recent,
		Anomalies: costs.AnomalyResult{
			Requested: true,
			Marks:     []costs.AnomalyMark{wipfixMark(recent[0], "recent-1")},
		},
	}, now)

	// The capped answer for the whole multi-year window.
	store.ApplyFetchResult(costs.FetchResult{
		Query:    costs.Query{Granularity: q.Granularity, GroupBy: q.GroupBy, Range: wipfixSpan(wide)},
		Coverage: wide,
		Anomalies: costs.AnomalyResult{
			Requested: true, Truncated: true,
			Marks: []costs.AnomalyMark{wipfixMark(wide[0], "old-1"), wipfixMark(recent[0], "recent-1")},
		},
	}, now)

	marks, partial := store.AnomalyOverlay(wide, now)
	if !partial {
		t.Error("the multi-year overlay is reported complete — the only answer covering that " +
			"window came back capped, and the operator has to be told so")
	}
	if len(marks) < 2 {
		t.Errorf("the multi-year overlay carries %d mark(s), want the capped walk's 2 — a "+
			"twelve-month answer is not an answer about four years", len(marks))
	}

	// The narrow window still gets the complete answer, with no warning.
	narrowMarks, narrowPartial := store.AnomalyOverlay(recent, now)
	if narrowPartial {
		t.Error("the twelve-month overlay is reported partial — it is covered authoritatively")
	}
	if len(narrowMarks) != 1 {
		t.Errorf("the twelve-month overlay carries %d mark(s), want 1", len(narrowMarks))
	}
}

// TestAnomalyOverlay_NarrowAuthoritativeDoesNotDiscardAWiderCappedWalk is the
// other order: the capped walk covering four years arrives first and a
// twelve-month complete answer lands after it. The wide window still has to
// find the capped marks.
func TestAnomalyOverlay_NarrowAuthoritativeDoesNotDiscardAWiderCappedWalk(t *testing.T) {
	store := costs.NewMemoryStore("example-readonly")
	q := wipfixOverlayQuery()
	now := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	recent := wipfixMonths(2025, 12)
	wide := append(wipfixMonths(2023, 12), append(wipfixMonths(2024, 12), recent...)...)

	store.ApplyFetchResult(costs.FetchResult{
		Query:    costs.Query{Granularity: q.Granularity, GroupBy: q.GroupBy, Range: wipfixSpan(wide)},
		Coverage: wide,
		Anomalies: costs.AnomalyResult{
			Requested: true, Truncated: true,
			Marks: []costs.AnomalyMark{wipfixMark(wide[0], "old-1"), wipfixMark(recent[0], "recent-1")},
		},
	}, now)
	store.ApplyFetchResult(costs.FetchResult{
		Query:    costs.Query{Granularity: q.Granularity, GroupBy: q.GroupBy, Range: wipfixSpan(recent)},
		Coverage: recent,
		Anomalies: costs.AnomalyResult{
			Requested: true,
			Marks:     []costs.AnomalyMark{wipfixMark(recent[0], "recent-1")},
		},
	}, now)

	marks, partial := store.AnomalyOverlay(wide, now)
	if !partial || len(marks) < 2 {
		t.Errorf("the multi-year overlay carries %d mark(s), partial=%v — a twelve-month "+
			"complete answer landing afterwards must not discard the only walk that covered "+
			"four years", len(marks), partial)
	}
}

// wipfixOverlayQuery is the shape both overlay answers are fetched under: one
// cache key, two windows, so the store's choice between them is about coverage
// and nothing else.
func wipfixOverlayQuery() costs.Query {
	return costs.Query{
		Granularity: string(costs.GranularityMonth),
		GroupBy:     []costs.Dimension{costs.DimensionService},
	}
}

// wipfixMonths is n consecutive calendar months from January of startYear.
func wipfixMonths(startYear, n int) []costs.Period {
	out := make([]costs.Period, n)
	for i := range out {
		start := time.Date(startYear, time.January, 1, 0, 0, 0, 0, time.UTC).AddDate(0, i, 0)
		out[i] = costs.Period{
			Start: start.Format("2006-01-02"),
			End:   start.AddDate(0, 1, 0).Format("2006-01-02"),
		}
	}
	return out
}

// wipfixSpan is the single Period a walk over window was requested under —
// what the store records as the reach of the answer it got back.
func wipfixSpan(window []costs.Period) costs.Period {
	return costs.Period{Start: window[0].Start, End: window[len(window)-1].End}
}

func wipfixMark(p costs.Period, id string) costs.AnomalyMark {
	return costs.AnomalyMark{
		ID:        id,
		Score:     0.9,
		Impact:    costs.Amount{Value: 120, Unit: "USD"},
		Period:    p,
		RootCause: "usage",
		Dimension: map[costs.Dimension]string{costs.DimensionService: "AmazonEC2"},
	}
}
