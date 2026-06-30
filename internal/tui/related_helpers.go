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
//
// A ScreenChildList for rt is pushed onto m.ctrl before constructing the
// ResourceListModel so that m.ctrl.topListState() points to this list's own
// ListState (not the parent EC2/etc. list state). The popView guard in
// app_stack.go pops m.ctrl when this ResourceListModel is later removed.
func (m *Model) newRelatedList(rt resource.ResourceTypeDef, src resource.Resource, opts relatedListOpts) tea.Cmd {
	m.ctrl.PushChildListScreen(rt.ShortName)
	rl := views.NewResourceList(rt, m.viewConfig, m.keys, m.ctrl)
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
	_, initCmd := rl.Init()
	rs := newListRS(rt.ShortName)
	w, h := m.innerSize()
	rs.width, rs.height = w, h
	m.pushRS(rs)
	return initCmd
}

// buildResourceCacheSnapshot delegates to runtime.Core for the canonical
// multi-cache merge (ResourceCache + LazyResourceCache + ProbeResources).
func (m *Model) buildResourceCacheSnapshot() resource.ResourceCache {
	return m.core.BuildResourceCacheSnapshot()
}
