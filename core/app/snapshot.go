// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// Snapshot builds the full ViewState from the controller's screen state:
// Header, FrameTitle, Footer, and the per-screen Body (menu/list/detail/text/
// selector/help/identity). Both the TUI and web render from this snapshot.
//
// Snapshot never panics on an empty stack — it returns a ViewState with
// BodyKindUnknown.
//
// Takes a WRITE lock, not a read lock: buildListBody (list_body.go) may
// populate a list screen's ListState.bodyMemo cache on a miss, which is a
// mutation. Two goroutines calling Snapshot concurrently on the same
// Controller (e.g. two web requests against the same session) must not race
// that write — see Controller's Concurrency doc comment.
func (c *Controller) Snapshot() ViewState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshot()
}

// snapshot is the lock-free implementation of Snapshot.
// Callers must hold c.mu for WRITE — buildListBody (reached via
// vs.Body.List below) may populate ListState.bodyMemo on a cache miss.
func (c *Controller) snapshot() ViewState {
	vs := ViewState{
		Header: Header{
			Profile:          c.core.Profile(),
			Region:           c.core.Region(),
			Mode:             c.uiMode,
			Flash:            c.flash,
			ErrorHintVisible: c.showErrorHint && len(c.errorHistory) > 0,
		},
	}
	if len(c.stack) == 0 {
		vs.Body.Kind = BodyKindUnknown
		return vs
	}
	top := c.stack[len(c.stack)-1]
	vs.FrameTitle = string(top.ID)
	vs.Body.Kind = bodyKindForScreen(top)
	if top.State.Menu != nil {
		vs.Body.Menu = buildMenuBody(top.State.Menu)
		vs.Body.Menu.Refreshing = c.menuRefreshing()
		vs.FrameTitle = menuFrameTitle(top.State.Menu)
		vs.Footer = MenuFooterHintsFor(c.uiMode)
	}
	if top.State.List != nil {
		vs.Body.List = c.buildListBody(top.Ctx, top.State.List)
		vs.FrameTitle = c.buildListFrameTitle(top.Ctx, top.State.List)
		vs.Footer = c.buildListFooterHints(top.Ctx, top.State.List)
	}
	if top.State.Selector != nil {
		vs.Body.Selector = buildSelectorBody(top.State.Selector)
		vs.FrameTitle = selectorFrameTitle(top.State.Selector)
	}
	if top.State.Text != nil {
		vs.Body.Text = buildTextBody(top.State.Text)
		vs.Footer = c.buildTextFooterHints(top.ID, top.Ctx)
	}
	if top.State.Detail != nil {
		vs.Body.Detail = c.buildDetailBody(top.State.Detail)
		vs.FrameTitle = c.detailFrameTitleLocked()
		vs.Footer = c.buildDetailFooterHints(top.State.Detail)
	}
	if top.ID == runtime.ScreenHelp {
		vs.Body.Help = c.buildHelpBody()
	}
	if top.ID == runtime.ScreenIdentity {
		vs.Body.Identity = c.buildIdentityBody()
	}
	if top.State.Costs != nil {
		vs.Body.Costs = buildCostsBody(top.State.Costs)
		vs.FrameTitle = costsFrameTitle(vs.Body.Costs)
		vs.Footer = CostsFooterHintsFor(c.uiMode)
	}
	vs.CopyText, vs.CopyLabel = c.copyContent()
	vs.ConsoleURL, _ = c.consoleURL()
	vs.IsDemo = c.core.IsDemo()
	return vs
}

// consoleURL resolves the AWS console link for the current screen's target
// resource via ConsoleTarget (list selected row, detail resource, or a
// focused single-target related row) and runs it through consolelink.Resolve
// plus the same Valid guard the TUI's exec path uses before ever spawning a
// browser. Callers must hold c.mu (consoleTarget is lock-free).
func (c *Controller) consoleURL() (string, bool) {
	td, res, ok := c.consoleTarget()
	if !ok {
		return "", false
	}
	accountID := ""
	if c.identityResult != nil {
		accountID = c.identityResult.AccountID
	}
	u, ok := consolelink.Resolve(*td, res, c.core.Region(), accountID)
	if !ok || !consolelink.Valid(u) {
		return "", false
	}
	return u, true
}

// ConsoleTarget resolves the (typeDef, resource) pair for the console-link
// actions (the TUI's o/O keys and the web ConsoleURL snapshot field): the
// active list's selected row (including child lists), the active detail
// screen's resource, or — when the detail screen's related panel has focus
// on a row that resolves to exactly one target resource — that related row's
// resource instead. An aggregate related row (0 or several targets) has no
// single resource to link to and resolves to false.
func (c *Controller) ConsoleTarget() (*resource.ResourceTypeDef, resource.Resource, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.consoleTarget()
}

// consoleTarget is the lock-free core of ConsoleTarget. Callers must already
// hold c.mu (read or write).
func (c *Controller) consoleTarget() (*resource.ResourceTypeDef, resource.Resource, bool) {
	if len(c.stack) == 0 {
		return nil, resource.Resource{}, false
	}
	top := c.stack[len(c.stack)-1]
	switch bodyKindForScreen(top) {
	case BodyKindList:
		r, ok := c.listSelected()
		if !ok {
			return nil, resource.Resource{}, false
		}
		td := consoleTypeDefFor(top.Ctx.ResourceType)
		if td == nil {
			return nil, resource.Resource{}, false
		}
		return td, r, true
	case BodyKindDetail:
		ds := c.topDetailState()
		if ds == nil {
			return nil, resource.Resource{}, false
		}
		if ds.RelatedFocus {
			if row := ds.focusedRelatedRow(); row != nil {
				return c.consoleTargetFromRelatedRow(*row)
			}
		}
		td := consoleTypeDefFor(ds.ResourceType)
		if td == nil {
			return nil, resource.Resource{}, false
		}
		return td, ds.Resource, true
	default:
		return nil, resource.Resource{}, false
	}
}

// consoleTargetFromRelatedRow resolves a console-link target from a focused
// related-panel row carrying exactly one target ID. Resolution order: (1)
// the full row already loaded into the session's RowStore for
// (TargetType, id), so a cached row's complete Fields feed the type's
// ConsoleURL builder; (2) td.StubCreator, which synthesizes a resource
// carrying the fields (e.g. an ARN) the builder needs; (3) a bare ID-only
// resource — safe because every ConsoleURL builder returns "" rather than a
// wrong URL on missing input.
func (c *Controller) consoleTargetFromRelatedRow(row DetailRelatedRow) (*resource.ResourceTypeDef, resource.Resource, bool) {
	if len(row.ResourceIDs) != 1 {
		return nil, resource.Resource{}, false
	}
	td := consoleTypeDefFor(row.TargetType)
	if td == nil {
		return nil, resource.Resource{}, false
	}
	id := row.ResourceIDs[0]
	if cached, ok := c.core.AnyLaneResourceByID(row.TargetType, id); ok {
		return td, cached, true
	}
	if td.StubCreator != nil {
		stub := td.StubCreator(id)
		return td, stub, true
	}
	return td, resource.Resource{ID: id, Type: row.TargetType}, true
}

// consoleTypeDefFor resolves shortName against the top-level catalog first,
// then the child-type registry — the same fallback copyContentList uses,
// since a child list's Ctx.ResourceType (and a related row's TargetType) is
// the child type's own ShortName.
func consoleTypeDefFor(shortName string) *resource.ResourceTypeDef {
	if td := resource.FindResourceType(shortName); td != nil {
		return td
	}
	return resource.GetChildType(shortName)
}

// ScreenIDs returns the ScreenID of every entry on the controller's screen
// stack, bottom-to-top. It exists so that renderer adapters (today: the TUI's
// stackInSync debug assertion in internal/tui/app_stack_invariant.go) can
// compare their own view stack against the controller's without reaching
// into unexported Controller state — the controller stays the single source
// of truth for stack depth and per-level screen identity (see docs/architecture.md
// "the controller stack is authoritative").
func (c *Controller) ScreenIDs() []runtime.ScreenID {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ids := make([]runtime.ScreenID, len(c.stack))
	for i, s := range c.stack {
		ids[i] = s.ID
	}
	return ids
}

// bodyKindForScreen maps a Screen to the BodyKind a renderer uses to
// select the correct template/view.
func bodyKindForScreen(s Screen) BodyKind {
	switch s.ID {
	case runtime.ScreenMenu:
		return BodyKindMenu
	case runtime.ScreenProfileSelector, runtime.ScreenRegion, runtime.ScreenTheme:
		return BodyKindSelector
	case runtime.ScreenReveal, runtime.ScreenDetail:
		return BodyKindDetail
	case runtime.ScreenChildList, runtime.ScreenResourceList:
		return BodyKindList
	case runtime.ScreenYAML, runtime.ScreenJSON, runtime.ScreenErrorLog:
		return BodyKindText
	case runtime.ScreenHelp:
		return BodyKindHelp
	case runtime.ScreenIdentity:
		return BodyKindIdentity
	case runtime.ScreenCosts:
		return BodyKindCosts
	default:
		// Capability screens and future IDs not yet enumerated here.
		return BodyKindUnknown
	}
}

// helpContextName is the ViewState.Body.Help.Context string for each
// domain.HelpContext, so the web renderer can label/filter sections the same
// way the TUI's per-context HelpModel constructors imply via their name.
func helpContextName(ctx domain.HelpContext) string {
	switch ctx {
	case domain.HelpFromResourceList, domain.HelpFromResourceListPaginated:
		return "resource-list"
	case domain.HelpFromSecretsList, domain.HelpFromSecretsListPaginated:
		return "secrets-list"
	case domain.HelpFromDetail:
		return "detail"
	case domain.HelpFromYAML:
		return "yaml"
	case domain.HelpFromJSON:
		return "json"
	case domain.HelpFromSelector:
		return "selector"
	case domain.HelpFromReveal:
		return "reveal"
	case domain.HelpFromCosts:
		return "costs"
	default:
		return "main-menu"
	}
}

// helpContextForScreen derives the domain.HelpContext from the ScreenID of
// the screen directly under ScreenHelp on the controller's stack — the
// screen that was active when the user opened help. Screens with no help
// context of their own (unknown/absent) fall back to the main-menu context,
// matching the TUI's own HelpModel default.
func helpContextForScreen(id runtime.ScreenID) domain.HelpContext {
	switch id {
	case runtime.ScreenResourceList, runtime.ScreenChildList:
		return domain.HelpFromResourceList
	case runtime.ScreenDetail:
		return domain.HelpFromDetail
	case runtime.ScreenYAML:
		return domain.HelpFromYAML
	case runtime.ScreenJSON:
		return domain.HelpFromJSON
	case runtime.ScreenProfileSelector, runtime.ScreenRegion, runtime.ScreenTheme:
		return domain.HelpFromSelector
	case runtime.ScreenReveal:
		return domain.HelpFromReveal
	case runtime.ScreenCosts:
		return domain.HelpFromCosts
	default:
		return domain.HelpFromMainMenu
	}
}

// buildHelpBody constructs the HelpBody that the web renderer uses to
// populate the ? help overlay, sourced from the same domain.HelpGroupsFor
// table the TUI's internal/tui/views/help.go renders directly (single source
// of truth for help-overlay key/description content — see
// core/domain/helpkeys.go). The context is derived from the screen
// directly beneath ScreenHelp on the controller stack, i.e. the screen that
// was active when help was opened.
//
// toggleAttentionKey is hardcoded to "ctrl+z" — the built-in default for
// keys.Map.ToggleAttentionOnly (internal/tui/keys/keys.go). The web renderer
// has no per-session remapped keymap today, so this matches what every web
// session actually sees; if per-session keymaps are added later this must
// read the live binding the same way the TUI does.
func (c *Controller) buildHelpBody() *HelpBody {
	ctx := domain.HelpFromMainMenu
	if len(c.stack) >= 2 {
		ctx = helpContextForScreen(c.stack[len(c.stack)-2].ID)
	}
	sections := domain.HelpGroupsFor(ctx, "ctrl+z")
	body := &HelpBody{
		Context:  helpContextName(ctx),
		Sections: make([]HelpSection, len(sections)),
	}
	for i, s := range sections {
		hints := make([]KeyHint, len(s.Hints))
		for j, h := range s.Hints {
			hints[j] = KeyHint{Key: h.Key, Help: h.Help}
		}
		body.Sections[i] = HelpSection{Title: s.Title, Hints: hints}
	}
	return body
}

// buildIdentityBody constructs the IdentityBody from the controller's
// in-memory identity state. It returns Loading=true while the fetch is in
// flight, ErrorMsg on failure, or the fully-populated fields on success.
// Callers must hold c.mu (at least read).
func (c *Controller) buildIdentityBody() *IdentityBody {
	body := &IdentityBody{
		Profile: c.core.Profile(),
		Region:  c.core.Region(),
	}
	if c.identityLoading {
		body.Loading = true
		return body
	}
	if c.identityErrMsg != "" {
		body.ErrorMsg = c.identityErrMsg
		return body
	}
	if c.identityResult != nil {
		id := c.identityResult
		body.AccountID = id.AccountID
		body.AccountAlias = id.AccountAlias
		body.ARN = id.Arn
		body.IsAssumedRole = id.IsAssumedRole
		body.RoleName = id.RoleName
		body.SessionName = id.SessionName
		body.UserName = id.UserName
	}
	return body
}
