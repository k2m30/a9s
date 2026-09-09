// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_pagination_facts_test.go pins that a list screen keeps two
// separate facts about its population: whether the rows in hand are the
// confirmed whole population, and whether the fetcher offered another page to
// ask for. A partial-success fetch (some rows landed, a sibling enumeration
// failed) makes only the first fact false. Folding both into HasPagination
// makes a completely loaded list title as "N+" and offer "m: load more" with
// no cursor to follow, which is what the user sees as a list that never
// finishes loading.
package unit

import (
	"errors"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// wipfixS3Rows returns n synthetic s3 rows in the shape the s3 fetcher writes.
func wipfixS3Rows(n int) []resource.Resource {
	out := make([]resource.Resource, 0, n)
	for i := range n {
		id := "a9s-example-bucket-" + string(rune('a'+i))
		out = append(out, resource.Resource{
			ID:     id,
			Name:   id,
			Fields: map[string]string{"region": "us-east-1", "created": "2026-01-0" + string(rune('1'+i))},
		})
	}
	return out
}

// TestPartialSuccessFetch_FullyLoadedList_TitlesWithoutPlus pins row 27: a
// fetch that returned every row (pagination present, IsTruncated false) but
// also reported a partial failure has confirmed the whole population is on
// screen — the failure was a sibling enumeration, not a missing page. The
// title must show the exact count, and "m" must produce no continuation:
// there is no cursor to continue from.
func TestPartialSuccessFetch_FullyLoadedList_TitlesWithoutPlus(t *testing.T) {
	c := newTestController(t)
	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	_, _ = handlePage(c, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    wipfixS3Rows(3),
		Pagination:   &domain.PaginationMeta{IsTruncated: false, NextToken: ""},
		Err:          errors.New("operation error S3: GetBucketTagging, api error AccessDenied"),
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	title := c.Snapshot().FrameTitle
	if strings.Contains(title, "+") {
		t.Errorf("frame title after a fully loaded partial-success fetch = %q, want no %q: "+
			"the fetch confirmed all 3 rows are the whole list, only a sibling enumeration failed",
			title, "+")
	}
	if want := "s3(3)"; !strings.HasPrefix(title, want) {
		t.Errorf("frame title = %q, want it to start with %q", title, want)
	}

	_, tasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(tasks) != 0 {
		t.Errorf("load-more on a fully loaded list produced %d task(s), want 0 — "+
			"there is no continuation cursor to follow", len(tasks))
	}

	// The failure itself is still reported: it is the marker, not the count,
	// that carries it.
	if got := c.Snapshot().Body.List; got == nil || got.LastFetchError == "" {
		t.Errorf("the partial failure left no error marker on the list — the fix must "+
			"keep reporting the failure, only stop calling the population unconfirmed (list=%v)", got)
	}
}

// TestPartialSuccessFetch_TruncatedList_StillTitlesWithPlus is the negative
// half: when the same partial-success fetch DID leave a page behind, the "+"
// and the load-more offer must both survive. A fix that simply stops forcing
// HasPagination would break this.
func TestPartialSuccessFetch_TruncatedList_StillTitlesWithPlus(t *testing.T) {
	c := newTestController(t)
	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	_, _ = handlePage(c, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    wipfixS3Rows(3),
		Pagination:   &domain.PaginationMeta{IsTruncated: true, NextToken: "page-2"},
		Err:          errors.New("operation error S3: GetBucketTagging, api error AccessDenied"),
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	if title := c.Snapshot().FrameTitle; !strings.HasPrefix(title, "s3(3+)") {
		t.Errorf("frame title on a truncated partial-success fetch = %q, want it to start with %q", title, "s3(3+)")
	}
	_, tasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(tasks) != 1 {
		t.Fatalf("load-more on a truncated list produced %d task(s), want 1", len(tasks))
	}
	p, ok := tasks[0].Payload.(runtime.FetchMorePayload)
	if !ok {
		t.Fatalf("load-more payload type = %T, want runtime.FetchMorePayload", tasks[0].Payload)
	}
	if p.ContinuationToken != "page-2" {
		t.Errorf("continuation token = %q, want %q", p.ContinuationToken, "page-2")
	}
}

// TestPartialSuccessFetch_NoPagination_TitlesWithoutPlus covers the fetcher
// that reports no pagination at all (Pagination nil) alongside a partial
// failure — the same "population not confirmed, no page to fetch" case
// reached by a different route.
func TestPartialSuccessFetch_NoPagination_TitlesWithoutPlus(t *testing.T) {
	c := newTestController(t)
	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	_, _ = handlePage(c, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    wipfixS3Rows(2),
		Pagination:   nil,
		Err:          errors.New("operation error S3: GetBucketPolicyStatus, api error AccessDenied"),
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	if title := c.Snapshot().FrameTitle; strings.Contains(title, "+") {
		t.Errorf("frame title with no pagination at all = %q, want no %q", title, "+")
	}
	if _, tasks := c.Apply(app.Action{Kind: app.ActionLoadMore}); len(tasks) != 0 {
		t.Errorf("load-more produced %d task(s) with no pagination at all, want 0", len(tasks))
	}
}
