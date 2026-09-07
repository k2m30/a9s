// SPDX-License-Identifier: GPL-3.0-or-later

package views

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// handleSortByCol checks all 10 positional sort bindings against msg.
// Key "1" = absolute column 0, "2" = column 1, …, "0" = column 9.
// Returns true if a sort binding was matched (key consumed).
func (m *ResourceListModel) handleSortByCol(msg tea.KeyMsg) bool {
	for colIdx, kb := range m.keys.SortByCol {
		if key.Matches(msg, kb) {
			// Use ResolveColumnsForType so that viewConfig and fallback typeDefs
			// are applied — this makes sort-by-key consistent with buildListBody.
			cols := m.ctrl.ResolveColumnsForType(m.typeDef.ShortName)
			if colIdx >= len(cols) {
				return true // key pressed but no such column — absorb it
			}
			m.ctrl.Apply(app.Action{Kind: app.ActionSort, Arg: cols[colIdx].SortColKey()})
			m.styledRowCache = nil
			return true
		}
	}
	return false
}

// SetPendingFilter stores filter text to be applied when resources are loaded.
// In the controller-driven path this applies the filter immediately (there is no
// pending state — the controller owns the filter).
func (m *ResourceListModel) SetPendingFilter(text string) {
	if text != "" {
		m.ctrl.Apply(app.Action{Kind: app.ActionSetFilter, Arg: text})
		m.styledRowCache = nil
	}
}

// SetFetchFilter sets server-side filter parameters for initial fetch and load-more.
func (m *ResourceListModel) SetFetchFilter(filter map[string]string) {
	m.ctrl.PatchListFetchFilter(filter)
}

// SetRelatedIDFilter constrains the list to an exact set of resource IDs.
func (m *ResourceListModel) SetRelatedIDFilter(ids []string) {
	m.ctrl.PatchListRelatedIDSet(ids)
	m.styledRowCache = nil
}

// SetReapplyChecker registers the originating RelatedDef.Checker for re-application
// on subsequent resource loads.
func (m *ResourceListModel) SetReapplyChecker(checker resource.RelatedChecker, src resource.Resource) {
	m.ctrl.PatchListReapplyChecker(checker, src)
	m.styledRowCache = nil
}

// SetAutoOpenSingleDetail configures one-shot auto-navigation to detail when
// a ResourcesLoaded update leaves exactly one filtered row.
func (m *ResourceListModel) SetAutoOpenSingleDetail(v bool) {
	m.ctrl.SetListAutoOpenSingle(v)
}

// SetSize updates dimensions.
func (m *ResourceListModel) SetSize(w, h int) {
	if m.width != w {
		m.styledRowCache = nil
	}
	m.width = w
	m.height = h
}

// enterChildFor returns the Children entry registered under Key="enter" for
// this resource type, or nil if none is registered or its DrillCondition
// vetoes the given row.
func (m ResourceListModel) enterChildFor(r resource.Resource) *resource.ChildViewDef {
	for i := range m.typeDef.Children {
		c := &m.typeDef.Children[i]
		if c.Key != "enter" {
			continue
		}
		if c.DrillCondition != nil && !c.DrillCondition(r) {
			return nil
		}
		return c
	}
	return nil
}

// handleChildKey iterates through the typeDef's Children looking for a match
// on keyName. If found, checks DrillCondition, builds context, and returns
// an EnterChildViewMsg command. Returns the model and nil cmd if no child matched.
func (m ResourceListModel) handleChildKey(keyName string, r *resource.Resource) (ResourceListModel, tea.Cmd) {
	for _, child := range m.typeDef.Children {
		if child.Key != keyName {
			continue
		}
		if child.DrillCondition != nil && !child.DrillCondition(*r) {
			if child.DrillBlockMessage != "" {
				msg := child.DrillBlockMessage
				return m, func() tea.Msg {
					return messages.Flash{Text: msg, IsError: true}
				}
			}
			continue
		}
		ctx := m.buildChildContext(child, r)
		displayName := ctx[child.DisplayNameKey]
		childType := child.ChildType

		return m, func() tea.Msg {
			return messages.EnterChildView{
				ChildType:     childType,
				ParentContext: ctx,
				DisplayName:   displayName,
			}
		}
	}
	return m, nil
}

// buildChildContext resolves ContextKeys for a ChildViewDef given the selected
// resource. Delegates to resource.ResolveChildContext — the single resolver
// shared with the auto-open-single-detail path in
// internal/tui/runtime_adapter_resources.go, so "ID"/"Name"/"@parent." source
// expressions resolve identically on every navigation path into a child view.
func (m ResourceListModel) buildChildContext(child resource.ChildViewDef, r *resource.Resource) map[string]string {
	return resource.ResolveChildContext(child, r, m.ctrl.GetListParentContext())
}

// SetTitleSuffix sets a suffix appended to the frame title after count rendering.
func (m *ResourceListModel) SetTitleSuffix(s string) {
	m.ctrl.PatchListTitleSuffix(s)
}

// SetDisplayName overrides the base title name used in FrameTitle.
func (m *ResourceListModel) SetDisplayName(name string) {
	m.ctrl.PatchListDisplayName(name)
}

// SetEscPops configures Esc behavior for this list.
func (m *ResourceListModel) SetEscPops(v bool) {
	m.ctrl.PatchListEscPops(v)
}
