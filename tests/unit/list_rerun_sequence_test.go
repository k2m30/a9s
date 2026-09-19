// The enrichment-rerun reseed orders two results for one list by the
// per-type request sequence, not by enrichment generation. Pressing Ctrl+R
// and then `m` before the refresh lands leaves the refresh's result carrying
// the current enrichment generation and a superseded request sequence.
package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestRerunReseed_SupersededRequestNeverReseeds walks the real interleaving.
// Ctrl+R is pressed first, so it holds the current enrichment generation; the
// load-more dispatched after it takes the newer request sequence and lands
// first. When the refresh's own result finally arrives, its enrichment
// generation still matches — and it is nonetheless the older request.
func TestRerunReseed_SupersededRequestNeverReseeds(t *testing.T) {
	ctrl, core := newDetailParityHeadlessController(t)
	_ = ctrl

	typeGen := core.RefreshListEnrichment(listGenType)
	if typeGen == 0 {
		t.Fatalf("RefreshListEnrichment(%q) returned 0 — this type has no issue enricher, so the rerun branch is unreachable", listGenType)
	}
	refreshSeq := core.NextListFetchSeq(listGenScreen)

	loadMoreSeq := core.NextListFetchSeq(listGenScreen)
	deep := listGenRows(6, "i-deep")
	core.ObserveRows(listGenType, deep, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	if loadMoreSeq <= refreshSeq {
		t.Fatalf("load-more sequence %d did not outrank the refresh's %d", loadMoreSeq, refreshSeq)
	}

	core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: listGenType,
		Resources:    listGenRows(2, "i-stale"),
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "n"},
		TypeGen:      typeGen,
		ListSeq:      refreshSeq,
		ScreenID:     listGenScreen,
	})

	entry, ok := core.ResourceCache(listGenType)
	if !ok {
		t.Fatalf("the row store holds nothing for %s", listGenType)
	}
	if len(entry.Resources) != len(deep) {
		t.Fatalf("the row store holds %d rows, want the load-more's %d — a superseded refresh reseeded over the deeper page", len(entry.Resources), len(deep))
	}
}

// When the rerun is the newest request it reseeds, so the guard cannot be
// satisfied by refusing every rerun.
func TestRerunReseed_LatestRequestStillReseeds(t *testing.T) {
	_, core := newDetailParityHeadlessController(t)

	core.ObserveRows(listGenType, listGenRows(6, "i-deep"), &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)

	typeGen := core.RefreshListEnrichment(listGenType)
	refreshSeq := core.NextListFetchSeq(listGenScreen)

	_, tasks := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: listGenType,
		Resources:    listGenRows(2, "i-fresh"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		TypeGen:      typeGen,
		ListSeq:      refreshSeq,
		ScreenID:     listGenScreen,
	})

	entry, ok := core.ResourceCache(listGenType)
	if !ok {
		t.Fatalf("the row store holds nothing for %s", listGenType)
	}
	if len(entry.Resources) != 2 {
		t.Fatalf("the row store holds %d rows, want the refresh's 2 — the newest request must still reseed", len(entry.Resources))
	}
	found := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.TaskKindProbeEnrich {
			found = true
		}
	}
	if !found {
		t.Fatalf("a current rerun returned %d task(s) and no TaskKindProbeEnrich", len(tasks))
	}
}
