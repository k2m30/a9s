package unit_test

// helpers_external_test.go consolidates shared helpers for the unit_test
// (external test) package.  Previously these lived inside qa_detail_test.go,
// which duplicated helpers from helpers_test.go (package unit).
//
// Shared by: qa_detail_test.go, qa_list_rawstruct_test.go,
//            qa_s3_object_detail_test.go

import (
	"os"
	"regexp"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/config"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/styles"
	"github.com/k2m30/a9s/internal/tui/views"
)

// ---------------------------------------------------------------------------
// ANSI stripping -- canonical implementation for the unit_test package.
// The package-unit (non _test) equivalent lives in helpers_test.go as stripANSI.
// ---------------------------------------------------------------------------

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// ---------------------------------------------------------------------------
// NO_COLOR management
// ---------------------------------------------------------------------------

// ensureNoColor sets NO_COLOR=1 and reinitializes styles so all Render calls
// pass through without ANSI escape codes, making string assertions reliable.
func ensureNoColor(t *testing.T) {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})
}

// ---------------------------------------------------------------------------
// Key press helpers
// ---------------------------------------------------------------------------

func detailKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

func detailSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

// ---------------------------------------------------------------------------
// Detail model builders
// ---------------------------------------------------------------------------

// newDetailModel creates a DetailModel with config-driven rendering via RawStruct,
// calls SetSize, and returns the model ready for View().
func newDetailModel(res resource.Resource, resourceType string, cfg *config.ViewsConfig) views.DetailModel {
	k := keys.Default()
	m := views.NewDetail(res, resourceType, cfg, k)
	m.SetSize(200, 100)
	return m
}

// newDetailModelSmall creates a DetailModel with a small viewport to test scrolling.
func newDetailModelSmall(res resource.Resource, resourceType string, cfg *config.ViewsConfig) views.DetailModel {
	k := keys.Default()
	m := views.NewDetail(res, resourceType, cfg, k)
	m.SetSize(80, 5)
	return m
}

// detailApplyMsg sends a message through the DetailModel's Update.
func detailApplyMsg(m views.DetailModel, msg tea.Msg) (views.DetailModel, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated, cmd
}

// ---------------------------------------------------------------------------
// Resource builders
// ---------------------------------------------------------------------------

// buildResource constructs a resource.Resource with a RawStruct set.
func buildResource(id, name string, rawStruct interface{}) resource.Resource {
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
// resource type. This avoids the non-deterministic map iteration in renderFromConfig
// matching a wrong ViewDef whose paths happen to extract values from the struct.
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
