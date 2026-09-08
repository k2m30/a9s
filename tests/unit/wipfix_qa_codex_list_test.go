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

// TestStampListFetchSeq_OnlyTheCanonicalLaneTakesASequence pins row 44. The
// sequence orders the type's canonical list against itself. A related fetch
// declares its own lane and must not draw one, or it supersedes a canonical
// refresh in flight and is superseded by the next.
func TestStampListFetchSeq_OnlyTheCanonicalLaneTakesASequence(t *testing.T) {
	b := newWipfixBench(t)

	for _, lane := range []messages.FetchProvenance{
		messages.FetchProvenanceFilteredList,
		messages.FetchProvenanceChild,
	} {
		req := runtime.TaskRequest{
			Key:     runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "s3"},
			Payload: runtime.FetchResourcesPayload{Provenance: lane},
		}
		b.core.StampListFetchSeq(&req)
		if req.ListSeq != 0 {
			t.Errorf("a %v fetch was stamped list sequence %d — the sequence orders the "+
				"canonical list against itself, and drawing one here supersedes a canonical "+
				"refresh in flight", lane, req.ListSeq)
		}
	}

	// The two that must keep drawing one: the canonical lane, and a payload
	// that declares no lane at all (the legacy default).
	canonical := runtime.TaskRequest{
		Key:     runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "s3"},
		Payload: runtime.FetchResourcesPayload{Provenance: messages.FetchProvenanceCanonicalList},
	}
	b.core.StampListFetchSeq(&canonical)
	if canonical.ListSeq == 0 {
		t.Error("a canonical fetch was not stamped — it is the lane the sequence exists for")
	}
	unstamped := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "s3"}}
	b.core.StampListFetchSeq(&unstamped)
	if unstamped.ListSeq == 0 {
		t.Error("a fetch declaring no lane was not stamped — the legacy default is canonical")
	}
}

// TestRelatedFetchDoesNotSupersedeACanonicalRefresh is row 44's scenario:
// both results have to land.
func TestRelatedFetchDoesNotSupersedeACanonicalRefresh(t *testing.T) {
	b := newWipfixBench(t)

	canonical := runtime.TaskRequest{
		Key:     runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "s3"},
		Payload: runtime.FetchResourcesPayload{Provenance: messages.FetchProvenanceCanonicalList},
	}
	b.core.StampListFetchSeq(&canonical)

	related := runtime.TaskRequest{
		Key:     runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "s3"},
		Payload: runtime.FetchResourcesPayload{Provenance: messages.FetchProvenanceFilteredList},
	}
	b.core.StampListFetchSeq(&related)

	if b.core.ListResultSuperseded("s3", canonical.ListSeq) {
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

	_, _ = c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    wipfixSaveLaneRows(20),
		Pagination:   &domain.PaginationMeta{IsTruncated: true, NextToken: "page-2"},
		Err:          errors.New("operation error EC2: DescribeTags, api error AccessDenied"),
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	_, _ = c.Handle(messages.ResourcesLoaded{
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
	if title := c.Snapshot().FrameTitle; !strings.Contains(title, "+") {
		t.Errorf("the list titles %q with no %q — the population is still unconfirmed", title, "+")
	}
}

// TestCleanPageOneThenExhaustingPageTwo_IsSavedAsExact is the negative half:
// two clean pages that exhaust pagination really are the whole list.
func TestCleanPageOneThenExhaustingPageTwo_IsSavedAsExact(t *testing.T) {
	c := newTestController(t)
	cfg := os.Getenv("A9S_CONFIG_FOLDER")
	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	_, _ = c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    wipfixSaveLaneRows(20),
		Pagination:   &domain.PaginationMeta{IsTruncated: true, NextToken: "page-2"},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	_, _ = c.Handle(messages.ResourcesLoaded{
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
// failure for "rds" has to find the "dbi" screen, or that screen loads for
// ever with nothing to show for it.
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

	_, _ = b.c.Handle(messages.ResourcesLoaded{
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
