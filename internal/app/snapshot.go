package app

import "github.com/k2m30/a9s/v3/internal/runtime"

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
		vs.FrameTitle = menuFrameTitle(top.State.Menu)
		vs.Footer = MenuFooterHints()
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
		vs.Body.Help = buildHelpBody()
	}
	if top.ID == runtime.ScreenIdentity {
		vs.Body.Identity = c.buildIdentityBody()
	}
	return vs
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
	default:
		// Capability screens and future IDs not yet enumerated here.
		return BodyKindUnknown
	}
}

// buildHelpBody constructs the HelpBody that the web renderer uses to populate
// the ? help overlay. It mirrors the helpGroup structure from
// internal/tui/views/help.go, sourcing the same static keybinding strings.
// The context is "main-menu" (the default) since the controller does not track
// which view opened help; a richer context can be wired in PR-C when per-screen
// state is lifted.
func buildHelpBody() *HelpBody {
	nav := HelpSection{
		Title: "NAVIGATION",
		Hints: []KeyHint{
			{Key: "j/k", Help: "up/down"},
			{Key: "g", Help: "top"},
			{Key: "G", Help: "bottom"},
			{Key: "pgup", Help: "page up"},
			{Key: "pgdn", Help: "page down"},
		},
	}
	actions := HelpSection{
		Title: "ACTIONS",
		Hints: []KeyHint{
			{Key: "enter", Help: "select"},
			{Key: "/", Help: "filter"},
			{Key: ":", Help: "command"},
			{Key: "q", Help: "quit"},
			{Key: "ctrl+c", Help: "force quit"},
		},
	}
	other := HelpSection{
		Title: "OTHER",
		Hints: []KeyHint{
			{Key: "i", Help: "identity"},
			{Key: "!", Help: "error log"},
			{Key: "?", Help: "help"},
			{Key: "esc", Help: "back"},
		},
	}
	commands := HelpSection{
		Title: "COMMANDS",
		Hints: []KeyHint{
			{Key: ":q", Help: "exit"},
			{Key: ":ctx", Help: "switch profile"},
			{Key: ":profile", Help: "switch profile"},
			{Key: ":region", Help: "switch region"},
			{Key: ":theme", Help: "switch theme"},
			{Key: ":help", Help: "show help"},
			{Key: ":root", Help: "main menu"},
			{Key: ":main", Help: "main menu"},
			{Key: ":<res>", Help: "e.g. :ec2 :s3 :lambda"},
		},
	}
	return &HelpBody{
		Context:  "main-menu",
		Sections: []HelpSection{nav, actions, other, commands},
	}
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
