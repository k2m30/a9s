// SPDX-License-Identifier: GPL-3.0-or-later

// related_helpers.go — BT-coupled related-navigation helpers that stay in
// the tui package because they reference views.ResourceList and tea.Cmd.
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
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
	covered := false
	if opts.reapplyChecker != nil {
		// Reverse-scan seed (truncated "(0+)"/"(N+)"): a NON-nil set (empty for
		// "(0+)") scopes to the found IDs so it renders ZERO rows, never "all",
		// until the reapply-checker extends it as later pages load. The population
		// fetch always follows, so there is no cache seed here.
		ids := opts.relatedIDs
		if ids == nil {
			ids = []string{}
		}
		rl.SetRelatedIDFilter(ids)
		rl.SetReapplyChecker(opts.reapplyChecker, src)
	} else if len(opts.relatedIDs) > 0 {
		// Exact-ID list: seed the found rows from the any-lane cache (Partial lane
		// included) so a hit renders with no fetch. Shared with the web renderer
		// via SeedRelatedExactRows — the two cannot diverge.
		covered = m.ctrl.SeedRelatedExactRows(rt.ShortName, opts.relatedIDs)
	}
	if opts.autoOpenSingleDetail {
		rl.SetAutoOpenSingleDetail(true)
	}
	rl.SetEscPops(true)
	rl.SetSize(m.innerSize())
	_, initCmd := rl.Init()
	if covered {
		// Fully cache-covered: rows are seeded and Loading is cleared, so nothing
		// fetches and nothing spins — drop the spinner tick so a pure cache hit
		// dispatches no command.
		initCmd = nil
	}
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
