package unit

import (
	"regexp"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// ctrlZ constructs the ctrl+z key press message understood by bubbles/v2 key.Matches.
// Equivalent to key.NewBinding(key.WithKeys("ctrl+z")).
func ctrlZ() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl}
}

// ansiRe is kept for direct call sites in search tests that use it inline.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI removes ANSI escape sequences from a string for plain-text comparison.
func stripANSI(s string) string {
	return tuitest.StripANSI(s)
}

// lipglossWidth measures the visible width of a string (ANSI-aware).
func lipglossWidth(s string) int {
	return tuitest.Width(s)
}

// collectAllPages drains a paginated fetcher (a *Page function bound to its
// ctx/api args via closure) until IsTruncated is false, mirroring the
// deleted all-pages FetchX wrappers' exact semantics: append each page's
// resources, stop on the first error (discarding any partial results), and
// stop once Pagination is nil or IsTruncated is false.
func collectAllPages(fetch func(token string) (resource.FetchResult, error)) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := fetch(token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// newDetailControllerUnit builds a Controller (via the blessed newTestController
// helper — see qa_controller_construction_discipline_test.go) with a ScreenDetail
// on the stack for res, ready to call Snapshot().Body.Detail. Package-unit
// equivalent of wave3_detail_ports_test.go's newDetailController (package
// unit_test symbols aren't visible from here).
func newDetailControllerUnit(t *testing.T, res resource.Resource, resourceType string) *app.Controller {
	t.Helper()
	c := newTestController(t)
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenDetail},
	})
	c.EnsureDetailState(res, resourceType)
	return c
}
