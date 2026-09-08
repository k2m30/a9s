// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_lifecycle_blind_test.go reproduces the reviewer's list-lifecycle
// scenarios from the keys the operator presses, with every message produced by
// the real executor so the stamping is production's rather than the test's:
//
//   - a drill's own load-more appends to that drill and declares the drill's
//     lane rather than claiming to be the type's canonical list;
//   - Ctrl+R pressed while a load-more is outstanding leaves load-more usable;
//   - the "m" key the terminal builds itself carries the same lane.
package unit_test

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// wipfixTwoPageFetcher registers a fetcher for shortName that hands back one
// row per page across two pages — the smallest list that can be loaded more.
func wipfixTwoPageFetcher(t *testing.T, shortName string) {
	t.Helper()
	prev := resource.GetPaginatedFetcher(shortName)
	t.Cleanup(func() { resource.SetPaginatedForTest(shortName, prev) })
	resource.SetPaginatedForTest(shortName, func(_ context.Context, _ any, token string) (resource.FetchResult, error) {
		if token == "" {
			return resource.FetchResult{
				Resources:  []resource.Resource{{ID: "row-page-1", Name: "row-page-1", Fields: map[string]string{}}},
				Pagination: &domain.PaginationMeta{IsTruncated: true, NextToken: "page-2"},
			}, nil
		}
		return resource.FetchResult{
			Resources:  []resource.Resource{{ID: "row-page-2", Name: "row-page-2", Fields: map[string]string{}}},
			Pagination: &domain.PaginationMeta{IsTruncated: false},
		}, nil
	})
}

// wipfixBench is a controller plus the core behind it, so a test can run the
// tasks the controller emits through the real executor and hand the messages
// it produces back — the dispatch loop the hosts run, with the stamping the
// hosts get.
type wipfixBench struct {
	c    *app.Controller
	core *runtime.Core
}

func newWipfixBench(t *testing.T) *wipfixBench {
	t.Helper()
	c, core := newTestControllerAndCore(t)
	// The load-more executor refuses without a transport; the demo one keeps
	// the whole dispatch loop hermetic.
	core.Session().Clients = demo.NewServiceClients()
	return &wipfixBench{c: c, core: core}
}

// execute runs one task through the executor and returns the message, without
// delivering it — so a test can hold a result back and deliver it late.
func (b *wipfixBench) execute(t *testing.T, task runtime.TaskRequest) messages.Event {
	t.Helper()
	b.core.StampListFetchSeq(&task)
	ev, err := b.core.ExecuteTask(context.Background(), task)
	if err != nil {
		t.Fatalf("executing %v: %v", task.Key, err)
	}
	return ev
}

// pump runs tasks to quiescence: execute, deliver, execute whatever that
// produced, bounded so a task that re-emits itself fails loudly.
func (b *wipfixBench) pump(t *testing.T, tasks []runtime.TaskRequest) {
	t.Helper()
	for round := 0; len(tasks) > 0; round++ {
		if round > 8 {
			t.Fatal("the dispatch loop did not settle within 8 rounds")
		}
		var next []runtime.TaskRequest
		for _, task := range tasks {
			ev := b.execute(t, task)
			if ev == nil {
				continue
			}
			_, more := b.c.Handle(ev)
			next = append(next, more...)
		}
		tasks = next
	}
}

func (b *wipfixBench) list(t *testing.T) *app.ListBody {
	t.Helper()
	body := b.c.Snapshot().Body.List
	if body == nil {
		t.Fatal("no list body on the snapshot")
	}
	return body
}

// TestCtrlRDuringLoadMore_LeavesTheMKeyUsable pins row 3: Ctrl+R while a
// continuation is outstanding supersedes it, and the continuation's result is
// discarded when it finally lands. Whatever discards it must retire the flag
// it raised, because no completion ever will — and a stuck loading-more makes
// "m" dead for the rest of the session.
func TestCtrlRDuringLoadMore_LeavesTheMKeyUsable(t *testing.T) {
	wipfixTwoPageFetcher(t, "s3")
	b := newWipfixBench(t)

	_, tasks := b.c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	b.pump(t, tasks)
	if !b.list(t).Truncated {
		t.Fatal("precondition: the s3 list is not truncated after its first page")
	}

	// "m" — the continuation goes out and is still in flight.
	_, loadMore := b.c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(loadMore) != 1 {
		t.Fatalf("precondition: load-more produced %d task(s), want 1", len(loadMore))
	}
	pending := b.execute(t, loadMore[0])

	// Ctrl+R lands first and its own result is applied.
	_, refresh := b.c.Apply(app.Action{Kind: app.ActionRefresh})
	b.pump(t, refresh)

	// The continuation finally arrives, answering a request the refresh replaced.
	_, more := b.c.Handle(pending)
	b.pump(t, more)

	if b.list(t).LoadingMore {
		t.Error("the list is still marked loading-more after a refresh superseded its " +
			"continuation — nothing else will ever clear it")
	}
	if _, tasks := b.c.Apply(app.Action{Kind: app.ActionLoadMore}); len(tasks) != 1 {
		t.Errorf("the m key produced %d task(s) after a refresh superseded the previous "+
			"continuation — load-more is dead for the rest of the session", len(tasks))
	}
}

// TestOrdinaryLoadMore_ClearsItsOwnFlag is the negative half: an undisturbed
// continuation still retires the flag when it completes.
func TestOrdinaryLoadMore_ClearsItsOwnFlag(t *testing.T) {
	wipfixTwoPageFetcher(t, "s3")
	b := newWipfixBench(t)

	_, tasks := b.c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	b.pump(t, tasks)
	_, loadMore := b.c.Apply(app.Action{Kind: app.ActionLoadMore})
	b.pump(t, loadMore)

	body := b.list(t)
	if body.LoadingMore {
		t.Error("the list is still marked loading-more after its own continuation completed")
	}
	if len(body.Rows) != 2 {
		t.Errorf("the list holds %d rows after load-more, want 2", len(body.Rows))
	}
}

// TestDrillLoadMore_AppendsToTheDrillThatAskedForIt pins row 2: a drill's
// continuation belongs to the drill. Deriving its lane from the (empty)
// context maps instead of from the screen makes it claim to be the type's
// canonical list, and the delivery gate then refuses it on the very screen
// that asked for it.
func TestDrillLoadMore_AppendsToTheDrillThatAskedForIt(t *testing.T) {
	td := wipfixObjTypeDef()
	wipfixTwoPageFetcher(t, td.ShortName)
	b := newWipfixBench(t)
	b.c.PushChildListScreen(td.ShortName)
	b.c.RegisterFallbackTypeDef(td)

	_, tasks := b.c.Apply(app.Action{Kind: app.ActionRefresh})
	b.pump(t, tasks)
	if !b.list(t).Truncated {
		t.Fatal("precondition: the drill is not truncated after its first page")
	}

	_, loadMore := b.c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(loadMore) != 1 {
		t.Fatalf("load-more on the drill produced %d task(s), want 1", len(loadMore))
	}
	p, ok := loadMore[0].Payload.(runtime.FetchMorePayload)
	if !ok {
		t.Fatalf("payload type = %T, want runtime.FetchMorePayload", loadMore[0].Payload)
	}
	if p.Provenance == messages.FetchProvenanceCanonicalList {
		t.Errorf("the drill's own continuation declares the canonical-list lane — " +
			"the delivery gate refuses a canonical result on a drill screen, so this page " +
			"lands nowhere")
	}

	stamped := loadMore[0]
	b.core.StampListFetchSeq(&stamped)
	if stamped.ListSeq != 0 {
		t.Errorf("the drill's continuation was stamped list sequence %d — only a canonical-list "+
			"fetch takes one, or the drill supersedes the verification of the list beneath it",
			stamped.ListSeq)
	}

	b.pump(t, loadMore)

	body := b.list(t)
	if len(body.Rows) != 2 {
		t.Errorf("the drill holds %d rows after load-more, want 2 — the second page never landed "+
			"on the list that asked for it", len(body.Rows))
	}
	if body.LoadingMore {
		t.Error("the drill is still marked loading-more after its own continuation landed")
	}
}

func wipfixObjTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		Name:      "S3 Objects",
		ShortName: "s3_objects",
		Columns:   resource.S3ObjectColumns(),
	}
}

// TestTUILoadMoreKey_CarriesTheDrillsLane pins row 28 through the key the
// terminal actually handles: the "m" press builds its own messages.LoadMore
// rather than going through the controller's action, and it must stamp the
// lane the controller's one owner reports for that screen. Left unstamped the
// continuation is classified from the drill's empty context maps and claims
// the canonical list.
func TestTUILoadMoreKey_CarriesTheDrillsLane(t *testing.T) {
	td := wipfixObjTypeDef()
	c, _ := newTestControllerAndCore(t)
	c.PushChildListScreen(td.ShortName)
	c.RegisterFallbackTypeDef(td)
	c.ApplyResourcesLoaded(td.ShortName,
		[]resource.Resource{{ID: "row-page-1", Name: "row-page-1"}},
		&domain.PaginationMeta{IsTruncated: true, NextToken: "page-2"}, false)

	want := c.GetListLane()
	if want == messages.FetchProvenanceCanonicalList {
		t.Fatal("precondition: the controller reports the canonical lane for a drill screen")
	}

	m := views.NewChildResourceList(td, map[string]string{"bucket": "example-bucket"},
		"example-bucket", nil, keys.Default(), c)
	m.SetSize(120, 20)
	m, _ = m.Init()

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'm'})
	if cmd == nil {
		t.Fatal("the m key on a truncated drill produced no command")
	}
	msg, ok := cmd().(messages.LoadMore)
	if !ok {
		t.Fatalf("the m key produced %T, want messages.LoadMore", cmd())
	}
	if msg.Provenance != want {
		t.Errorf("the terminal's load-more carries lane %v, want %v — the screen's lane has one "+
			"owner and every continuation reads it", msg.Provenance, want)
	}
}

// TestExactRelatedDrill_StartingWithFetchMore_RendersItsRows pins row 4: an
// exact-ID drill whose targets are not in the cached page, while that page is
// itself truncated, opens on a continuation rather than a fresh fetch. The
// completion must clear the flag its own request raised — clearing
// loading-more, which nothing had raised, leaves the loading screen sitting
// over the rows the drill just fetched.
func TestExactRelatedDrill_StartingWithFetchMore_RendersItsRows(t *testing.T) {
	wipfixTwoPageFetcher(t, "ec2")
	// A source detail whose related check names two instances, neither of them
	// on the cached first page. Two, not one: a single target resolves to a
	// by-ID detail open instead of a list drill.
	resource.SetRelatedForTest("s3", []domain.RelatedDef{{
		TargetType: "ec2", DisplayName: "EC2 Instances",
		Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
			return resource.KnownRelated("ec2", []string{"row-page-2", "row-page-3"}, false)
		},
	}})
	t.Cleanup(func() { resource.CleanupRelatedForTest("s3") })

	b := newWipfixBench(t)

	// The ec2 cache holds page one only, and knows there is more.
	_, tasks := b.c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	b.pump(t, tasks)
	if !b.list(t).Truncated {
		t.Fatal("precondition: the ec2 cache is not truncated after page one")
	}
	_, _ = b.c.Apply(app.Action{Kind: app.ActionBack})

	_, tasks = b.c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	b.pump(t, tasks)
	_, tasks = b.c.Apply(app.Action{Kind: app.ActionSelect})
	b.pump(t, tasks)

	_, drill := b.c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})
	if len(drill) != 1 {
		t.Fatalf("the drill produced %d task(s), want 1", len(drill))
	}
	if drill[0].Key.Kind != runtime.KindFetchMore {
		t.Fatalf("precondition: the drill dispatched %v, want a continuation — this scenario is "+
			"about a drill that opens on one", drill[0].Key.Kind)
	}
	if !b.list(t).Loading {
		t.Fatal("precondition: the drill is not loading before its fetch lands")
	}

	b.pump(t, drill)

	body := b.list(t)
	if body.Loading {
		t.Error("the drill is still loading after its own fetch landed — the completion cleared " +
			"a flag nothing had raised and left the loading screen over the rows it just fetched")
	}
	if len(body.Rows) == 0 {
		t.Error("the drill rendered no rows after its continuation landed")
	}
}
