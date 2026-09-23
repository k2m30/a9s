package unit_test

// A Lambda function's state and last_update_status come only from wave 2
// (GetFunction). A wave-1 rows write — the sweep-completion save to disk or
// the in-memory probe write — replaces the row with ListFunctions' answer,
// which has neither, so both are carried forward like every other field an
// enricher writes.

import (
	"context"
	"maps"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// t561CarryWant is what wave 2 wrote and what must survive a wave-1 write.
var t561CarryWant = map[string]map[string]string{ //nolint:gochecknoglobals // test-only table
	"acme-orders-api":    {"state": "Active", "last_update_status": "Successful"},
	"acme-image-resizer": {"state": "Failed", "last_update_status": "Successful"},
	"acme-billing-sync":  {"state": "Active", "last_update_status": "Failed"},
}

// t561EnrichedAndBare returns the rows after the wave-2 pass (fetch, enricher,
// fold) and the bare rows a later ListFunctions page produces for the same
// functions.
func t561EnrichedAndBare(t *testing.T) (enriched, bare []resource.Resource) {
	t.Helper()
	fake := &t561LambdaFake{fns: t561LifecycleFns()}
	enriched, _ = t561LambdaRows(t, fake)
	for id, want := range t561CarryWant {
		r := t561Row(t, enriched, id)
		for k, v := range want {
			if r.Fields[k] != v {
				t.Fatalf("%s: wave 2 wrote %s=%q, want %q; the carry cannot be exercised", id, k, r.Fields[k], v)
			}
		}
	}
	page, err := awsclient.FetchLambdaFunctionsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchLambdaFunctionsPage: %v", err)
	}
	for i := range enriched {
		enriched[i].Type = "lambda"
	}
	for i := range page.Resources {
		page.Resources[i].Type = "lambda"
	}
	return enriched, page.Resources
}

func t561AssertCarried(t *testing.T, where string, fieldsOf func(id string) map[string]string) {
	t.Helper()
	for id, want := range t561CarryWant {
		got := fieldsOf(id)
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s: %s %s = %q after a wave-1 write, want %q", where, id, k, got[k], v)
			}
		}
	}
}

func TestT561_LambdaLifecycleSurvivesInMemoryProbeWrite(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	enriched, bare := t561EnrichedAndBare(t)
	c := newSaveCacheRegressionCore(t, false)

	c.Session().RowStore.Observe("lambda", enriched, &resource.PaginationMeta{IsTruncated: false}, session.OriginProbe, false)
	c.Session().AvailChecked = 0
	c.Session().AvailTotal = 1
	c.Session().AvailQueue = nil
	_, _ = c.HandleEvent(messages.AvailabilityChecked{
		ResourceType: "lambda",
		HasResources: true,
		Count:        len(bare),
		Resources:    bare,
		Gen:          c.AvailabilityGen(),
	})

	rows := c.Session().RowStore.Snapshot("lambda").Rows
	t561AssertCarried(t, "RowStore", func(id string) map[string]string {
		for _, r := range rows {
			if r.ID == id {
				return r.Fields
			}
		}
		return nil
	})
}

func TestT561_LambdaLifecycleSurvivesSweepSave(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	enriched, bare := t561EnrichedAndBare(t)

	toRows := func(rs []resource.Resource) []cache.Row {
		out := make([]cache.Row, 0, len(rs))
		for _, r := range rs {
			out = append(out, cache.Row{ID: r.ID, Name: r.Name, Fields: maps.Clone(r.Fields), Findings: r.Findings})
		}
		return out
	}
	store := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	store.Put("lambda", cache.TypeFile{HasResources: true, Count: len(enriched), Exact: true, Rows: toRows(enriched)})
	if err := store.SaveType("lambda"); err != nil {
		t.Fatalf("seed SaveType(lambda): %v", err)
	}

	c := newSaveCacheRegressionCore(t, false)
	if err := c.SaveResourceListCache(c.Pair(), "lambda", toRows(bare), len(bare), true, 0, false, false); err != nil {
		t.Fatalf("SaveResourceListCache: %v", err)
	}

	tf, ok := cache.LoadDirForTest(saveRegProfile, saveRegRegion).Type("lambda")
	if !ok {
		t.Fatal("lambda type file missing after the wave-1 save")
	}
	t561AssertCarried(t, "type file", func(id string) map[string]string {
		for _, r := range tf.Rows {
			if r.ID == id {
				return r.Fields
			}
		}
		return nil
	})
}
