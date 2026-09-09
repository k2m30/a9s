// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_codex_list_test.go pins the list-lifecycle findings: a sequence
// allocated for a lane that does not use it, an error forgotten by the page
// after it, a failure that skips the seam every success goes through, and a
// loading flag cleared by a request that no longer owns it.
package unit_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// --- row 44: a canonical sequence for a lane that is not canonical ---------

// TestStampListFetchSeq_EveryLaneTakesItsOwnScreensSequence is row 44's rule
// re-stated for the key runtime8 row 4 moved the guard to.
//
// INVERTED for runtime8 row 4. It used to require that ONLY the canonical lane
// draws a sequence, because one counter served the whole type and a drill
// drawing from it superseded the canonical refresh in flight. The counter is
// now keyed by the issuing screen, so a drill's sequence orders the drill's own
// requests and reaches no other screen — every lane draws one, and the lane is
// not consulted at all. What decides is whether a screen owns the task. Do not
// "restore" the lane test: with the per-screen key it would leave every drill
// unable to order its own two refreshes, which is the defect row 4 fixes.
func TestStampListFetchSeq_EveryLaneTakesItsOwnScreensSequence(t *testing.T) {
	b := newWipfixBench(t)

	const drill domain.Gen = 7
	for _, lane := range []messages.FetchProvenance{
		messages.FetchProvenanceFilteredList,
		messages.FetchProvenanceChild,
		messages.FetchProvenanceCanonicalList,
	} {
		req := runtime.TaskRequest{
			Key:      runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "s3"},
			Payload:  runtime.FetchResourcesPayload{Provenance: lane},
			ScreenID: drill,
		}
		b.core.StampListFetchSeq(&req)
		if req.ListSeq == 0 {
			t.Errorf("a %v fetch issued by a list screen drew no sequence — the screen cannot "+
				"order its own two refreshes against each other", lane)
		}
	}

	// A task no list screen owns has nothing to be ordered against.
	orphan := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "s3"}}
	b.core.StampListFetchSeq(&orphan)
	if orphan.ListSeq != 0 {
		t.Errorf("a fetch no list screen issued was stamped sequence %d — there is no screen whose "+
			"requests it could supersede or be superseded by", orphan.ListSeq)
	}
}

// TestRelatedFetchDoesNotSupersedeACanonicalRefresh is row 44's scenario:
// both results have to land. It holds by the key rather than by the lane now —
// the two fetches are two screens, so neither draws from the other's counter.
func TestRelatedFetchDoesNotSupersedeACanonicalRefresh(t *testing.T) {
	b := newWipfixBench(t)

	const parent, drill domain.Gen = 1, 2
	canonical := runtime.TaskRequest{
		Key:      runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "s3"},
		Payload:  runtime.FetchResourcesPayload{Provenance: messages.FetchProvenanceCanonicalList},
		ScreenID: parent,
	}
	b.core.StampListFetchSeq(&canonical)

	related := runtime.TaskRequest{
		Key:      runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "s3"},
		Payload:  runtime.FetchResourcesPayload{Provenance: messages.FetchProvenanceFilteredList},
		ScreenID: drill,
	}
	b.core.StampListFetchSeq(&related)

	if b.core.ListResultSuperseded(parent, canonical.ListSeq) {
		t.Error("a related fetch superseded the canonical refresh already in flight — " +
			"the canonical result is discarded and its screen keeps loading")
	}
}

// --- row 45: an error the next page forgets --------------------------------

// TestPartialPageOneThenExhaustingPageTwo_IsNotSavedAsExact pins row 45. Page
// one came back short with an error; page two finished the pagination but
// knows nothing about what page one failed to enumerate. The population is
// still unconfirmed, and a file that calls it exact is a confident wrong
// answer that survives the session.
func TestPartialPageOneThenExhaustingPageTwo_IsNotSavedAsExact(t *testing.T) {
	c := newTestController(t)
	cfg := os.Getenv("A9S_CONFIG_FOLDER")
	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	_, _ = handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    wipfixSaveLaneRows(20),
		Pagination:   &domain.PaginationMeta{IsTruncated: true, NextToken: "page-2"},
		Err:          errors.New("operation error EC2: DescribeTags, api error AccessDenied"),
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	_, _ = handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    wipfixSaveLaneRows(10),
		Pagination:   &domain.PaginationMeta{IsTruncated: false},
		Append:       true,
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	c.WaitForCacheWrites()

	body := wipfixWatchTypeFile(t, cfg, "ec2")
	if wipfixExactFlag(t, body[len(body)-1]) {
		t.Errorf("the ec2 type file says exact: true — page one failed to enumerate part of "+
			"the list and page two only says there are no more pages, which is a different "+
			"fact\n%s", wipfixTypeFileHead(body[len(body)-1]))
	}
	// Row 27's contract owns the title: the "+" means another page exists, and
	// pagination is exhausted here. The incompleteness is the file's business
	// and the partial-success flash's, not the count's.
	if title := c.Snapshot().FrameTitle; strings.Contains(title, "+") {
		t.Errorf("the list titles %q — pagination is exhausted, so there is no page to offer", title)
	}
	if _, tasks := c.Apply(app.Action{Kind: app.ActionLoadMore}); len(tasks) != 0 {
		t.Errorf("load-more produced %d task(s) with pagination exhausted, want 0", len(tasks))
	}
}

// TestCleanPageOneThenExhaustingPageTwo_IsSavedAsExact is the negative half:
// two clean pages that exhaust pagination really are the whole list.
func TestCleanPageOneThenExhaustingPageTwo_IsSavedAsExact(t *testing.T) {
	c := newTestController(t)
	cfg := os.Getenv("A9S_CONFIG_FOLDER")
	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	_, _ = handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    wipfixSaveLaneRows(20),
		Pagination:   &domain.PaginationMeta{IsTruncated: true, NextToken: "page-2"},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	_, _ = handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    wipfixSaveLaneRows(10),
		Pagination:   &domain.PaginationMeta{IsTruncated: false},
		Append:       true,
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	c.WaitForCacheWrites()

	body := wipfixWatchTypeFile(t, cfg, "ec2")
	if !wipfixExactFlag(t, body[len(body)-1]) {
		t.Errorf("two clean pages that exhausted pagination did not save as exact\n%s",
			wipfixTypeFileHead(body[len(body)-1]))
	}
}

// --- rows 46 and 47: a failure and a stale success -------------------------

// wipfixRefreshSeq issues a refresh through the controller and returns the
// sequence the runtime stamped on it.
func wipfixRefreshSeq(t *testing.T, b *wipfixBench) domain.Gen {
	t.Helper()
	_, tasks := b.c.Apply(app.Action{Kind: app.ActionRefresh})
	if len(tasks) != 1 {
		t.Fatalf("a refresh produced %d task(s), want 1", len(tasks))
	}
	b.core.StampListFetchSeq(&tasks[0])
	return tasks[0].ListSeq
}

// TestAliasFailure_ReachesTheCanonicalScreen pins row 46's second half: a
// failure for a list opened under an alias has to reach that list, or it
// loads for ever with nothing to show for it. The alias half of the question
// is answered by the identity now — the failure names the screen that asked,
// and "rds" versus "dbi" never enters into it — so what is left to pin is
// that a failure retires the loading flag and leaves a marker.
func TestAliasFailure_ReachesTheCanonicalScreen(t *testing.T) {
	c := newTestController(t)
	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "rds"})
	if body := c.Snapshot().Body.List; body == nil || !body.Loading {
		t.Fatalf("precondition: the rds list is not loading after being opened (%v)", body)
	}

	_, _ = c.Handle(messages.APIError{
		ResourceType: "rds",
		Err:          errors.New("operation error RDS: DescribeDBInstances, api error AccessDenied"),
		Provenance:   messages.FetchProvenanceCanonicalList,
		ScreenID:     c.GetListInstance(),
	})

	body := c.Snapshot().Body.List
	if body == nil {
		t.Fatal("no list body after the failure")
	}
	if body.Loading {
		t.Error("the screen opened as \"rds\" is still loading after the fetch for it failed — " +
			"the failure could not find the canonical dbi screen it belongs to")
	}
	if body.LastFetchError == "" {
		t.Error("the screen carries no error marker after its fetch failed")
	}
}

// TestSupersededSuccess_LeavesTheNewerRefreshLoading pins row 47. Two
// refreshes are out. The older returns first and is correctly discarded, then
// clears the refreshing flag the newer one still owns, so the screen reports
// that loading is over while a fetch is still in flight.
func TestSupersededSuccess_LeavesTheNewerRefreshLoading(t *testing.T) {
	wipfixTwoPageFetcher(t, "s3")
	b := newWipfixBench(t)
	_, open := b.c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	b.pump(t, open)

	first := wipfixRefreshSeq(t, b)
	second := wipfixRefreshSeq(t, b)
	if first == second {
		t.Fatalf("precondition: both refreshes drew sequence %d", first)
	}

	_, _ = handlePage(b.c, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    wipfixSaveLaneRows(1),
		Pagination:   &domain.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
		ListSeq:      first,
	})

	if !b.list(t).Refreshing {
		t.Errorf("the screen stopped showing the refresh marker when sequence %d returned, "+
			"while sequence %d is still in flight — the flag belongs to the request that "+
			"raised it", first, second)
	}
}

// --- row 43: two lists of one type -----------------------------------------

// TestSameTypeDrills_PageTwoLandsOnTheScreenThatAskedForIt pins row 43. Two
// drills of one type are stacked and the deeper one's continuation is still
// out. Routing by lane alone cannot tell them apart, so the page lands on
// whichever screen is on top.
func TestSameTypeDrills_PageTwoLandsOnTheScreenThatAskedForIt(t *testing.T) {
	td := wipfixObjTypeDef()
	wipfixTwoPageFetcher(t, td.ShortName)
	b := newWipfixBench(t)
	b.c.RegisterFallbackTypeDef(td)

	// Drill A, loaded and truncated, with its continuation in flight.
	b.c.PushChildListScreen(td.ShortName)
	_, tasks := b.c.Apply(app.Action{Kind: app.ActionRefresh})
	b.pump(t, tasks)
	if !b.list(t).Truncated {
		t.Fatal("precondition: drill A is not truncated after its first page")
	}
	_, loadMore := b.c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(loadMore) != 1 {
		t.Fatalf("precondition: drill A's load-more produced %d task(s), want 1", len(loadMore))
	}
	pending := b.execute(t, loadMore[0])

	// Drill B of the same type opens on top while A's page is still out.
	b.c.PushChildListScreen(td.ShortName)
	b.c.ApplyResourcesLoaded(td.ShortName,
		[]resource.Resource{{ID: "b-only", Name: "b-only", Fields: map[string]string{}}}, nil, false)
	if got := len(b.list(t).Rows); got != 1 {
		t.Fatalf("precondition: drill B holds %d rows, want its own 1", got)
	}

	// A's page two finally lands.
	_, more := b.c.Handle(pending)
	b.pump(t, more)

	onB := b.list(t).Rows
	for _, r := range onB {
		if strings.HasPrefix(r.ResourceID, "row-page-") {
			t.Errorf("drill B shows %q, a row from the continuation drill A asked for — "+
				"routing by lane cannot tell two lists of one type apart", r.ResourceID)
		}
	}
	if len(onB) != 1 {
		t.Errorf("drill B holds %d rows, want the 1 it fetched itself", len(onB))
	}

	_, _ = b.c.Apply(app.Action{Kind: app.ActionBack})
	onA := b.list(t).Rows
	if len(onA) != 2 {
		t.Errorf("drill A holds %d rows after its own page two, want 2 — the page it asked "+
			"for never came back to it", len(onA))
	}
	if b.list(t).LoadingMore {
		t.Error("drill A is still marked loading-more after its own continuation landed")
	}
}
