package unit

import (
	"context"
	"fmt"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// noopChecker is a RelatedChecker that returns zero results. Use it in
// RelatedDef structs when the test only needs the def to be non-nil
// (e.g. to trigger right-column rendering) but doesn't need real data.
var noopChecker resource.RelatedChecker = func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.KnownRelated("", nil, false)
}

// replaceEC2Related registers defs for "ec2" and restores the originals on
// cleanup so tests that temporarily override related defs don't leave the
// registry poisoned for subsequent tests running in shuffled order.
func replaceEC2Related(t *testing.T, defs []resource.RelatedDef) {
	t.Helper()
	resource.SetRelatedForTest("ec2", defs)
	t.Cleanup(func() { resource.CleanupRelatedForTest("ec2") })
}

// newDemoColdCacheApp constructs a tui.Model exactly as cmd/a9s/main.go will
// after feature 014-demo-transport-mock is fully wired (T036d). It uses
// demo.NewServiceClients() to supply fake clients and passes them via
// tui.WithClients so no live AWS calls are made.
//
// The model is cold-cache: resourceCache starts empty, no preloading, no nil
// clients. Callers drive it by sending messages via model.Update().
func newDemoColdCacheApp(t *testing.T) *tui.Model {
	t.Helper()
	clients := demo.NewServiceClients()
	m := newBlessedModel(t,
		demo.DemoProfile,
		demo.DemoRegion,
		tui.WithClients(clients),
		tui.WithNoCache(true),
	)
	return &m
}

// DemoDrainMaxPages bounds every fixture drain as a safety valve against a
// fake that never clears IsTruncated. Real demo fixture sets fit in a handful
// of pages, so reaching this bound is a runaway, not a large account.
const DemoDrainMaxPages = 50

// DrainPages runs fetch to exhaustion the way the production fetch loop does
// and returns every resource across every page.
//
// Rows and a composite error arriving together are the designed partial-success
// outcome — lt's and mwaa's details-denied witnesses are fetched exactly that
// way — so only a row-less error aborts. A harness that stops at page one
// instead loses every witness that sorts past the first page without ever
// failing, which is the failure this helper exists to make impossible.
func DrainPages(t *testing.T, label string, fetch func(token string) (resource.FetchResult, error)) []resource.Resource {
	t.Helper()
	var all []resource.Resource
	token := ""
	for page := range DemoDrainMaxPages {
		result, err := fetch(token)
		if err != nil && len(result.Resources) == 0 {
			t.Fatalf("%s: fetch page %d returned error: %v", label, page, err)
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			return all
		}
		token = result.Pagination.NextToken
	}
	t.Fatalf("%s: fetch did not terminate within %d pages — runaway pagination in demo fixtures", label, DemoDrainMaxPages)
	return all
}

// DrainFixtures drains a registered type's demo rows through its own Wave-1
// Fetcher. Reports false when the type has no Fetcher, which is the caller's
// signal to skip rather than fail.
func DrainFixtures(t *testing.T, td resource.ResourceTypeDef, clients *awsclient.ServiceClients) ([]resource.Resource, bool) {
	t.Helper()
	if td.Fetcher == nil {
		return nil, false
	}
	return DrainPages(t, td.ShortName, func(token string) (resource.FetchResult, error) {
		return td.Fetcher(context.Background(), clients, token)
	}), true
}

// CollectAllPages drains fetch and hands back the error instead of failing, for
// tests whose subject is the error itself. The page bound still applies.
//
// It discards every row on any error, unlike DrainPages/FetchRelatedTarget's
// "only a row-less error aborts" rule — 100+ call sites rely on that strict
// stop, so it is not changed here. A demo cache builder needs DrainPages, not
// this.
func CollectAllPages(fetch func(token string) (resource.FetchResult, error)) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for range DemoDrainMaxPages {
		result, err := fetch(token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			return all, nil
		}
		token = result.Pagination.NextToken
	}
	return all, fmt.Errorf("fetch did not terminate within %d pages", DemoDrainMaxPages)
}
