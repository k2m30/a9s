package unit_test

import (
	"context"
	"slices"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// t570List reads every page of a type's list the way the app does, through
// the type's registered fetcher on the given clients.
func t570List(t *testing.T, clients *awsclient.ServiceClients, typ string) []resource.Resource {
	t.Helper()
	fetch := resource.GetPaginatedFetcher(typ)
	if fetch == nil {
		t.Fatalf("no paginated fetcher for %s", typ)
	}
	var rows []resource.Resource
	token := ""
	for range 20 {
		res, err := fetch(context.Background(), clients, token)
		if err != nil && len(res.Resources) == 0 {
			t.Fatalf("%s list: %v", typ, err)
		}
		rows = append(rows, res.Resources...)
		if res.Pagination == nil || !res.Pagination.IsTruncated || res.Pagination.NextToken == "" {
			break
		}
		token = res.Pagination.NextToken
	}
	return rows
}

// t570Row is the row of typ with the given id, as the list fetcher built it.
func t570Row(t *testing.T, clients *awsclient.ServiceClients, typ, id string) resource.Resource {
	t.Helper()
	rows := t570List(t, clients, typ)
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	t.Fatalf("%s row %q not in the list; ids %v", typ, id, ids)
	return resource.Resource{}
}

// t570Pivot runs the registered source -> target checker the way a detail
// open does before any list is cached: the checker reads its target list
// through the clients.
func t570Pivot(t *testing.T, clients *awsclient.ServiceClients, src resource.Resource, source, target string) resource.RelatedCheckResult {
	t.Helper()
	return checkerByTarget(t, source, target)(context.Background(), clients, src, nil)
}

func t570IDs(r resource.RelatedCheckResult) []string {
	ids := slices.Clone(r.ResourceIDs())
	slices.Sort(ids)
	return ids
}

// t570Exact asserts a resolved, complete answer holding exactly want.
func t570Exact(t *testing.T, r resource.RelatedCheckResult, want ...string) {
	t.Helper()
	slices.Sort(want)
	if r.State() != domain.RelatedResolved {
		t.Fatalf("state = %v (err %v), want resolved %v", r.State(), r.Err(), want)
	}
	if got := t570IDs(r); !slices.Equal(got, want) {
		t.Errorf("ResourceIDs = %v, want %v", got, want)
	}
	if r.Truncated() {
		t.Errorf("Truncated = true, want an exact answer %v", want)
	}
}

// t570Contains asserts a resolved answer that lists every one of want and
// none of absent.
func t570Contains(t *testing.T, r resource.RelatedCheckResult, want, absent []string) {
	t.Helper()
	if r.State() != domain.RelatedResolved {
		t.Fatalf("state = %v (err %v), want resolved listing %v", r.State(), r.Err(), want)
	}
	got := r.ResourceIDs()
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("%s missing from %v", w, got)
		}
	}
	for _, a := range absent {
		if slices.Contains(got, a) {
			t.Errorf("%s listed in %v, want it absent", a, got)
		}
	}
}

// t570NotProvenZero asserts the pivot does not claim a confident zero: it is
// unknown, errored, or a lower bound.
func t570NotProvenZero(t *testing.T, r resource.RelatedCheckResult) {
	t.Helper()
	if r.State() == domain.RelatedResolved && r.Count() == 0 && !r.Truncated() {
		t.Errorf("pivot reads a proven 0 for a place it never read")
	}
}

func t570Demo() *awsclient.ServiceClients { return demo.NewServiceClients() }
