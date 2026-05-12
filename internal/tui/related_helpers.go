// related_helpers.go — BT-coupled related-navigation helpers that stay in
// the tui package because they reference views.ResourceList and tea.Cmd.
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// relatedListOpts configures a related-navigation resource list.
type relatedListOpts struct {
	pendingFilter        string
	relatedIDs           []string
	autoOpenSingleDetail bool
	reapplyChecker       resource.RelatedChecker
}

// newRelatedList creates a ResourceList configured for related-resource
// navigation, pushes it onto the view stack, and returns the init command.
func (m *Model) newRelatedList(rt resource.ResourceTypeDef, src resource.Resource, opts relatedListOpts) tea.Cmd {
	rl := views.NewResourceList(rt, m.viewConfig, m.keys)
	rl.SetTitleSuffix(runtime.RelatedTitleSuffix(src))
	if opts.pendingFilter != "" {
		rl.SetPendingFilter(opts.pendingFilter)
	}
	if len(opts.relatedIDs) > 0 {
		rl.SetRelatedIDFilter(opts.relatedIDs)
	}
	if opts.reapplyChecker != nil {
		rl.SetReapplyChecker(opts.reapplyChecker, src)
	}
	if opts.autoOpenSingleDetail {
		rl.SetAutoOpenSingleDetail(true)
	}
	rl.SetEscPops(true)
	rl.SetSize(m.innerSize())
	rl, initCmd := rl.Init()
	m.pushView(&rl)
	return initCmd
}

// buildResourceCacheSnapshot delegates to runtime.Core for the canonical
// multi-cache merge (ResourceCache + LazyResourceCache + ProbeResources).
func (m *Model) buildResourceCacheSnapshot() resource.ResourceCache {
	return runtime.New(m.core.Session(), resource.AllResourceTypes()).BuildResourceCacheSnapshot()
}
