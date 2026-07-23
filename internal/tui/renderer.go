// SPDX-License-Identifier: GPL-3.0-or-later

// renderer.go — per-screen renderer state and free render functions.
//
// rendererState holds ONLY renderer-local values: viewport, search widget,
// right-column widget, scroll/cursor ints, reveal payload, help context, and
// terminal dimensions. It contains ZERO stored view model pointers.
//
// The free render functions (renderMenu, renderList, renderDetail, etc.) each
// create a short-lived transient view model populated from the controller body
// + rendererState, call the existing Render* method, and discard the model.
// This keeps the existing Render* methods (and all tests that call them) working
// unchanged while removing the stored model from the stack.
package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// rsKind identifies what kind of screen a rendererState represents.
// Used by View() to dispatch to the correct free render function directly
// from rs.kind (rather than ctrl.Snapshot().Body.Kind) since one rsKind may
// cover more than one ScreenID (e.g. rsKindText covers YAML/JSON/error-log;
// rsKindList covers top-level and child lists) — see screenIDMatchesRSKind
// in app_stack_invariant.go for the full kind-to-ScreenID mapping.
type rsKind int

const (
	rsKindMenu     rsKind = iota
	rsKindList            // resource list or child list
	rsKindDetail          // resource detail
	rsKindReveal          // secret reveal overlay (BodyKindDetail with nil Detail)
	rsKindText            // YAML / JSON / error-log
	rsKindSelector        // profile / region / theme selector
	rsKindHelp            // help overlay
	rsKindIdentity        // identity overlay
	rsKindCosts           // Cost Explorer grid
)

// rendererState is per-stack-entry renderer state. One rendererState is pushed
// onto m.stack for every screen the TUI pushes (menu, list, detail, text,
// selector, help, identity, reveal). It carries only the values that the free
// render functions need to produce output — no stored view model fields.
type rendererState struct {
	// kind identifies which free render function to call for this rs.
	kind rsKind
	// Common terminal dimensions (set by propagateSize).
	width  int
	height int

	// Viewport for detail, reveal, and text views.
	viewport viewport.Model
	ready    bool // true after the first SetSize call that initialised viewport

	// Detail / text search widget (non-nil only on detail and text screens).
	search views.SearchModel

	// Related-resource right column (detail screens only).
	rightCol            views.RightColumnModel
	rightColVisible     bool
	rightColAutoShown   bool
	rightColUserToggled bool

	// Main-menu scroll offset (owned by the renderer, not the controller).
	scrollOffset int

	// Reveal overlay: secret name/value/wrap state pushed via ScreenReveal.
	revealName  string
	revealValue string
	revealWrap  bool

	// Help overlay context: which screen opened help + active resource short name.
	helpContext   views.HelpContext
	helpShortName string

	// Identity overlay: tracks loading/error state when identity screen is active.
	identityLoading bool
	identityErr     string
	identityData    views.IdentityData

	// Resource type for this screen (needed to build transient ResourceListModel
	// so it can resolve typeDef for column rendering). Mirrors rs.ctrl screen Ctx.
	resourceType string

	// textResource holds the resource whose YAML/JSON content is being shown.
	// Set when navigating to YAML/JSON so frameTitle can show "<name> yaml".
	textResource *resource.Resource

	// onSelect is the callback stored for selector screens so the selector's key
	// handler can emit the right message without a stored SelectorModel.
	onSelect func(string) tea.Msg

	// ctrlBacked is true when a corresponding controller screen was pushed at the
	// same time as this rendererState. Used by popRS() to decide whether to call
	// ActionBack on the controller when popping this entry. As of goal-4 wave 4a
	// every rsKind pushes a matching controller screen (help/identity/error-log
	// included) — see docs/architecture.md "the controller stack is authoritative".
	ctrlBacked bool

	// costsViewportCols is the value View() last passed to
	// Controller.SetCostsViewportCols for a rsKindCosts screen — zero means
	// never set. Compared before every call so an unchanged terminal width
	// (the overwhelming majority of renders) skips the controller's write
	// lock entirely instead of taking it just to write back the same value.
	costsViewportCols int
}

// newMenuRS returns a fresh rendererState for the main-menu screen.
func newMenuRS() *rendererState {
	return &rendererState{kind: rsKindMenu, helpContext: views.HelpFromMainMenu}
}

// newListRS returns a fresh rendererState for a resource-list screen.
// ctrlBacked=true: the caller must push a ctrl screen before or after calling this.
// The help context is HelpFromSecretsList for resource types with a reveal fetcher,
// and HelpFromResourceList for all others.
func newListRS(resourceType string) *rendererState {
	ctx := views.HelpFromResourceList
	if resource.HasRevealFetcher(resourceType) {
		ctx = views.HelpFromSecretsList
	}
	return &rendererState{kind: rsKindList, resourceType: resourceType, ctrlBacked: true, helpContext: ctx}
}

// newDetailRS returns a fresh rendererState for a detail screen.
// ctrlBacked=true: the caller pushes ScreenDetail onto the controller.
func newDetailRS(resourceType string) *rendererState {
	return &rendererState{kind: rsKindDetail, resourceType: resourceType, ctrlBacked: true, helpContext: views.HelpFromDetail}
}

// newRevealRS returns a fresh rendererState for the reveal overlay.
// ctrlBacked=true: reveal pushes ScreenReveal onto the controller.
func newRevealRS(secretName, value string) *rendererState {
	return &rendererState{kind: rsKindReveal, revealName: secretName, revealValue: value, ctrlBacked: true, helpContext: views.HelpFromReveal}
}

// newTextRS returns a fresh rendererState for a YAML/JSON/text screen.
// ctrlBacked=true: YAML/JSON screens push ScreenYAML/ScreenJSON onto the controller.
func newTextRS() *rendererState {
	return &rendererState{kind: rsKindText, ctrlBacked: true, helpContext: views.HelpFromYAML}
}

// newErrorLogRS returns a fresh rendererState for the error-log text screen.
// ctrlBacked=true: the caller pushes runtime.ScreenErrorLog onto the
// controller and seeds its TextState via m.ctrl.EnsureTextState (mirroring
// YAML/JSON) before calling this, so the error-log viewer renders from
// snap.Body.Text like every other ctrl-backed text screen — no adapter-local
// errorLogText copy (goal-4 wave 4a: StackInSync sees error-log as ctrl-backed).
func newErrorLogRS() *rendererState {
	return &rendererState{kind: rsKindText, ctrlBacked: true}
}

// newSelectorRS returns a fresh rendererState for a selector screen.
// ctrlBacked=true: selector screens push a selector screen onto the controller.
func newSelectorRS(onSelect func(string) tea.Msg) *rendererState {
	return &rendererState{kind: rsKindSelector, onSelect: onSelect, ctrlBacked: true, helpContext: views.HelpFromSelector}
}

// newHelpRS returns a fresh rendererState for the help overlay.
// ctrlBacked=true: the caller pushes runtime.ScreenHelp onto the controller
// before calling this (goal-4 wave 4a), so StackInSync sees the help overlay
// as ctrl-backed. The rs still carries helpContext/helpShortName — the TUI's
// help rendering stays view-model-driven (NewHelpWithResource) rather than
// consuming Body.Help, since the TUI already resolves the richer per-context
// groups (secrets/paginated variants, CloudTrail legend) that the controller's
// Snapshot-derived context cannot distinguish from ScreenID alone.
func newHelpRS(ctx views.HelpContext, shortName string) *rendererState {
	return &rendererState{kind: rsKindHelp, helpContext: ctx, helpShortName: shortName, ctrlBacked: true}
}

// newCostsRS returns a fresh rendererState for the Cost Explorer screen.
// ctrlBacked=true: the caller pushes runtime.ScreenCosts onto the
// controller and seeds it via m.ctrl.EnsureCostsState before calling this.
func newCostsRS() *rendererState {
	return &rendererState{kind: rsKindCosts, ctrlBacked: true, helpContext: views.HelpFromCosts}
}

// newIdentityRS returns a fresh rendererState for the identity overlay.
// ctrlBacked=true: the caller pushes runtime.ScreenIdentity onto the
// controller before calling this (goal-4 wave 4a), so StackInSync sees the
// identity overlay as ctrl-backed. The rs still carries identity
// loading/error/data state directly (SetIdentityIntent updates it via
// applyIntents) rather than reading Body.Identity, since the identity fetch
// lifecycle is driven by the same TaskKindFetchIdentity flow that already
// targets this rs.
func newIdentityRS() *rendererState {
	return &rendererState{kind: rsKindIdentity, identityLoading: true, ctrlBacked: true}
}

// ── Free render functions ────────────────────────────────────────────────────
//
// Each function creates a TRANSIENT (zero-lifetime, never stored) view model
// from the controller body + rendererState dimensions, calls the existing
// Render* method, and returns the rendered string.

// renderMenu renders the main-menu screen from the controller MenuBody.
func renderMenu(body *app.MenuBody, rs *rendererState) string {
	if body == nil {
		return "No resource types"
	}
	m := views.NewMainMenu(keys.Default())
	m.SetSize(rs.width, rs.height)
	m.SetScrollOffset(rs.scrollOffset)
	return m.RenderBody(*body)
}

// renderList renders a resource-list screen from the controller ListBody.
func renderList(body *app.ListBody, rs *rendererState) string {
	if body == nil {
		return ""
	}
	// Build transient model with only the fields RenderList reads:
	// typeDef (for column/sort cross-reference), width, height, spinner.
	var td resource.ResourceTypeDef
	if rt := resource.FindResourceType(rs.resourceType); rt != nil {
		td = *rt
	} else if child := resource.GetChildType(rs.resourceType); child != nil {
		td = *child
	}
	m := views.NewTransientResourceList(td, rs.width, rs.height)
	return m.RenderList(*body)
}

// renderDetail renders a detail (or reveal) screen from the controller DetailBody.
// For the reveal case body is nil — the caller checks and routes to renderReveal.
func renderDetail(body *app.DetailBody, rs *rendererState) string {
	if body == nil {
		// Reveal overlay reuses BodyKindDetail with nil Detail.
		return renderReveal(rs)
	}
	vp := rs.viewport
	if vp.Height() == 0 && rs.height > 0 {
		vp = viewport.New(viewport.WithWidth(rs.width), viewport.WithHeight(rs.height))
	}
	m := views.NewTransientDetail(rs.width, rs.height, vp)
	result := m.RenderDetail(*body)
	// Propagate mutated viewport back so scroll state persists across frames.
	rs.viewport = m.Viewport()
	rs.ready = true
	// Keep the search widget in sync so MatchInfo() reflects actual matches.
	// SetContent recomputes the match list; SyncCursor then aligns currentIdx
	// with the controller's SearchCursor (advanced by ActionSearchNext/Prev).
	if rs.search.IsActive() {
		rs.search.SetContent(ansi.Strip(result))
		rs.search.SyncCursor(body.SearchCursor)
	}
	return result
}

// renderReveal renders the secret-reveal overlay.
func renderReveal(rs *rendererState) string {
	rv := views.NewReveal(rs.revealName, rs.revealValue, keys.Default())
	rv.SetSize(rs.width, rs.height)
	rv.SetWrap(rs.revealWrap)
	if rs.ready {
		rv.SetViewport(rs.viewport)
	}
	result := rv.View()
	rs.viewport = rv.GetViewport()
	rs.ready = true
	return result
}

// renderText renders a YAML/JSON/text screen from the controller TextBody.
func renderText(body *app.TextBody, rs *rendererState) string {
	if body == nil {
		return ""
	}
	vp := rs.viewport
	if vp.Height() == 0 && rs.height > 0 {
		vp = viewport.New(viewport.WithWidth(rs.width), viewport.WithHeight(rs.height))
	}
	m := views.NewTransientYAML(rs.width, rs.height, vp)
	result := m.RenderText(*body)
	rs.viewport = m.Viewport()
	rs.ready = true
	// Sync the search widget from body so MatchInfo() reflects the current match
	// count and cursor position. SyncCursor aligns currentIdx with the
	// controller's SearchCursor (advanced by ActionSearchNext/Prev).
	if rs.search.IsActive() {
		plain := ansi.Strip(strings.Join(body.Lines, "\n"))
		rs.search.SetContent(plain)
		rs.search.SyncCursor(body.SearchCursor)
	}
	return result
}

// renderSelector renders a selector screen from the controller SelectorBody.
func renderSelector(body *app.SelectorBody, rs *rendererState) string {
	if body == nil {
		return ""
	}
	m := views.NewTransientSelector(rs.width, rs.height)
	return m.RenderSelector(*body)
}

// renderHelp renders the help overlay using the HelpContext stored in rs.
func renderHelp(rs *rendererState) string {
	m := views.NewHelpWithResource(keys.Default(), rs.helpContext, rs.helpShortName)
	m.SetSize(rs.width, rs.height)
	return m.View()
}

// renderCosts renders the Cost Explorer screen from the controller CostsBody.
func renderCosts(body *app.CostsBody, rs *rendererState) string {
	if body == nil {
		return ""
	}
	return views.RenderCosts(*body, rs.width, rs.height)
}

// renderIdentity renders the identity overlay from adapter-owned rs fields.
// The TUI identity overlay is NOT ctrl-backed: all state lives in rs, updated
// directly by the SetIdentityIntent handler in app_dispatch.go.
func renderIdentity(rs *rendererState, profile, region string) string {
	m := views.NewIdentity(profile, region, keys.Default())
	m.SetSize(rs.width, rs.height)
	switch {
	case rs.identityLoading:
		// stay in loading state
	case rs.identityErr != "":
		m.SetError(rs.identityErr)
	default:
		m.SetIdentity(rs.identityData)
	}
	return m.View()
}
