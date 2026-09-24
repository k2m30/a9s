package unit_test

// The list view writes its rows back to the session store when its filter
// changes, and the store keeps the completeness the rows were first stored
// under: a page that lost rows beside an error stays a lower bound no cursor
// resumes.

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

func TestT567ListRestore_KeepsTheStoredLowerBound(t *testing.T) {
	m := newPreviewDemoModel(t, 120, 30)
	m, _ = previewApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ng"})
	dropped := awsclient.AggregateFailures("ng", []awsclient.Failure{awsclient.FailedCall("acme-staging", t567Denied("eks:ListNodegroups"))}, 2)
	m, _ = previewApplyMsg(m, messages.ResourcesLoaded{
		Provenance:   messages.FetchProvenanceCanonicalList,
		ResourceType: "ng",
		Resources:    []resource.Resource{{ID: "acme-prod/general-pool", Name: "general-pool"}},
		Pagination:   &resource.PaginationMeta{PageSize: 1, TotalHint: 1},
		Err:          dropped,
	})
	stored := func(when string) {
		t.Helper()
		entry, ok := m.Core().AnyOriginResourceCache("ng")
		if !ok || entry.Pagination == nil || !entry.Pagination.IsTruncated || !entry.Pagination.LowerBoundOnly {
			var p *resource.PaginationMeta
			if ok {
				p = entry.Pagination
			}
			t.Fatalf("%s: the stored ng page = %+v, want a lower bound no cursor resumes", when, p)
		}
	}
	stored("after the load")
	m, _ = previewApplyMsg(m, tea.KeyPressMsg{Code: '/', Text: "/"})
	m, _ = previewApplyMsg(m, tea.KeyPressMsg{Code: 'g', Text: "g"})
	stored("after the filter wrote the list back")
}
