package unit_test

// helpers_external_test.go consolidates shared helpers for the unit_test
// (external test) package.  Previously these lived inside qa_detail_test.go,
// which duplicated helpers from helpers_test.go (package unit).
//
// Shared by: qa_detail_test.go, qa_list_rawstruct_test.go,
//            qa_s3_object_detail_test.go

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/resource"
	unit "github.com/k2m30/a9s/v3/tests/unit"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// ---------------------------------------------------------------------------
// ANSI stripping -- canonical implementation for the unit_test package.
// The package-unit (non _test) equivalent lives in helpers_test.go as stripANSI.
// ---------------------------------------------------------------------------

func stripAnsi(s string) string {
	return tuitest.StripANSI(s)
}

// ---------------------------------------------------------------------------
// NO_COLOR management
// ---------------------------------------------------------------------------

// ensureNoColor sets NO_COLOR=1 and reinitializes styles so all Render calls
// pass through without ANSI escape codes, making string assertions reliable.
func ensureNoColor(t *testing.T) {
	tuitest.NoColor(t)
}

// ---------------------------------------------------------------------------
// Key press helpers
// ---------------------------------------------------------------------------

func detailKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

// ---------------------------------------------------------------------------
// Resource builders
// ---------------------------------------------------------------------------

// buildResource constructs a resource.Resource with a RawStruct set.
func buildResource(id, name string, rawStruct any) resource.Resource {
	return resource.Resource{
		ID:        id,
		Name:      name,
		RawStruct: rawStruct,
	}
}

// buildResourceWithFields constructs a resource.Resource with only Fields (no RawStruct),
// for testing the fallback rendering path.
func buildResourceWithFields(id, name string, fields map[string]string) resource.Resource {
	return resource.Resource{
		ID:     id,
		Name:   name,
		Fields: fields,
	}
}

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

// configForType returns a ViewsConfig containing only the ViewDef for the given
// resource type. This avoids non-deterministic map iteration over the full
// config matching a wrong ViewDef whose paths extract values from the struct.
func configForType(typeName string) *config.ViewsConfig {
	full := config.DefaultConfig()
	vd, ok := full.Views[typeName]
	if !ok {
		return full
	}
	return &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			typeName: vd,
		},
	}
}

// ---------------------------------------------------------------------------
// Paginated fetcher draining -- canonical implementation for the unit_test
// package. The package-unit (non _test) equivalent lives in helpers_test.go.
// ---------------------------------------------------------------------------

// collectAllPages drains a paginated fetcher (a *Page function bound to its
// ctx/api args via closure) until IsTruncated is false, mirroring the
// deleted all-pages FetchX wrappers' exact semantics: append each page's
// resources, stop on the first error (discarding any partial results), and
// stop once Pagination is nil or IsTruncated is false.
func collectAllPages(fetch func(token string) (resource.FetchResult, error)) ([]resource.Resource, error) {
	return unit.CollectAllPages(fetch)
}
