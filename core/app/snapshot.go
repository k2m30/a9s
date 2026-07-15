package app

import (
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// Snapshot builds the full ViewState from the controller's screen state:
// Header, FrameTitle, Footer, and the per-screen Body (menu/list/detail/text/
// selector/help/identity). Both the TUI and web render from this snapshot.
//
// Snapshot never panics on an empty stack — it returns a ViewState with
// BodyKindUnknown.
func (c *Controller) Snapshot() ViewState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snapshot()
}

// snapshot is the lock-free implementation of Snapshot.
// Callers must hold c.mu (at least read).
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
		vs.Body.Detail = buildDetailBody(top.State.Detail, c.viewConfig)
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
	return vs
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
// internal/domain/helpkeys.go). The context is derived from the screen
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
