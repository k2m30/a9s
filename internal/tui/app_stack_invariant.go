// app_stack_invariant.go — the goal-4 mechanical check that the renderer
// stack (m.stack, []*rendererState) is a strict 1:1 mirror of the headless
// controller's screen stack (m.ctrl.ScreenIDs()).
//
// StackInSync is exported (not merely an internal debug assertion) so that
// tests in tests/unit/ can pin the invariant directly against the public
// Model surface without a reflection-based back door into unexported stack
// fields — the harness already treats m.ctrl.Snapshot()/View() as the
// observable seam for renderer-vs-state parity, and this method is the same
// kind of read-only observation, just aimed at stack shape rather than body
// content.
package tui

import "github.com/k2m30/a9s/v3/internal/runtime"

// rsIsCtrlBacked reports whether the rendererState at the given stack index
// has a corresponding controller screen. As of goal-4 wave 4a there is no
// exception list: help, identity, and error-log all push their controller
// screen (ScreenHelp / ScreenIdentity / ScreenErrorLog) at the same time as
// their rendererState (see newHelpRS / newIdentityRS / newErrorLogRS in
// renderer.go and the push sites in app_input.go).
//
// The permanent root menu (index 0, rsKindMenu) is NOT ctrlBacked-flagged
// (newMenuRS leaves ctrlBacked at its zero value), but it is still counted in
// the mirror check by construction: both stacks are seeded with exactly one
// menu/ScreenMenu entry at index 0 in tui.New/app.New and it is never popped
// (popRSWithCtrlPop refuses to pop the last entry), so treating index 0 as an
// implicit ctrl-backed entry keeps the invariant total without special-casing
// the constructor.
func rsIsCtrlBacked(rs *rendererState, index int) bool {
	if index == 0 {
		return true
	}
	return rs.ctrlBacked
}

// screenIDMatchesRSKind reports whether the controller's ScreenID at a given
// depth is one of the IDs the corresponding rendererState kind is allowed to
// pair with. Some rsKind values map to more than one ScreenID (e.g. list
// screens may be a top-level ScreenResourceList or a ScreenChildList; text
// screens may be ScreenYAML, ScreenJSON, or ScreenErrorLog; selector screens
// may be any of the three selector flavors) — the renderer collapses these
// distinctions into one rsKind because the free render functions (renderList,
// renderText, renderSelector) do not need to know which.
func screenIDMatchesRSKind(kind rsKind, id runtime.ScreenID) bool {
	switch kind {
	case rsKindMenu:
		return id == runtime.ScreenMenu
	case rsKindList:
		return id == runtime.ScreenResourceList || id == runtime.ScreenChildList
	case rsKindDetail:
		return id == runtime.ScreenDetail
	case rsKindReveal:
		return id == runtime.ScreenReveal
	case rsKindText:
		return id == runtime.ScreenYAML || id == runtime.ScreenJSON || id == runtime.ScreenErrorLog
	case rsKindSelector:
		return id == runtime.ScreenProfileSelector || id == runtime.ScreenRegion || id == runtime.ScreenTheme
	case rsKindHelp:
		return id == runtime.ScreenHelp
	case rsKindIdentity:
		return id == runtime.ScreenIdentity
	default:
		return false
	}
}

// StackInSync reports whether m.stack is a strict 1:1 mirror of the
// controller's screen stack, per the goal-4 invariant: every ctrl-backed
// rendererState (see rsIsCtrlBacked) must correspond, at the same depth, to
// a controller Screen whose ID is compatible with that rendererState's kind
// (see screenIDMatchesRSKind). As of wave 4a every rsKind is ctrl-backed
// except the permanent root menu (see rsIsCtrlBacked) — there is no overlay
// exception list left.
//
// Exported so tests can pin the invariant directly (StackInSync() must stay
// true after every Update() in the TUI's own test harness) and so any future
// renderer adapter reusing app.Controller can perform the same check without
// reflection. This is a read-only diagnostic — it never mutates either
// stack — so exporting it carries no risk of a second write path.
func (m Model) StackInSync() bool {
	ctrlIDs := m.ctrl.ScreenIDs()
	// Build the filtered list of ctrl-backed renderer-stack depths first so a
	// length mismatch (missing/extra ctrl push, or an ctrl-only pop that
	// forgot its rendererState half) is caught before per-entry comparison.
	var ctrlBackedIdx []int
	for i, rs := range m.stack {
		if rsIsCtrlBacked(rs, i) {
			ctrlBackedIdx = append(ctrlBackedIdx, i)
		}
	}
	if len(ctrlBackedIdx) != len(ctrlIDs) {
		return false
	}
	for depth, i := range ctrlBackedIdx {
		if !screenIDMatchesRSKind(m.stack[i].kind, ctrlIDs[depth]) {
			return false
		}
	}
	return true
}
