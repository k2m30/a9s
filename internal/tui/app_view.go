// app_view.go — PR-05a-h4-c (AS-963) tui.Model render-path helpers.
//
// Split out of app.go so the View() composition + header-right /
// account-badge / identity-role / identity-to-view-data helpers live in
// their own file and app.go stays inside the 300–400 LOC budget that the
// spec acceptance check enforces (`wc -l internal/tui/app.go`).
//
// All five functions here are pure renderer-side composition: they read
// rendererState + controller snapshot plus a small slice of session metadata
// (profile, region, identity) and produce strings or tea.View values for the
// renderer to display. No runtime / handler logic lives here.
package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// View implements tea.Model. Composes header + frame around active view content.
func (m Model) View() tea.View {
	alt := func(s string) tea.View {
		v := tea.NewView(s)
		v.AltScreen = true
		return v
	}
	if m.width == 0 {
		return alt("")
	}
	if m.width < layout.MinTerminalWidth {
		return alt("Terminal too narrow (min 60 columns)")
	}
	if m.height < 7 {
		return alt("Terminal too short (min 7 lines)")
	}

	rs := m.activeRS()
	snap := m.ctrl.Snapshot()

	headerProfile := m.core.Profile()
	headerRegion := m.core.Region()
	// Selector screens show "..." in place of the real profile/region.
	if rs.kind == rsKindSelector && snap.Body.Selector != nil {
		switch snap.Body.Selector.Title {
		case "aws-regions":
			headerRegion = "..."
		case "aws-profiles":
			headerProfile = "..."
		}
	}
	// Render content first so that renderDetail/renderText can sync the search
	// widget's content (SetContent) before headerRight reads MatchInfo().
	var content string
	switch rs.kind {
	case rsKindMenu:
		content = renderMenu(snap.Body.Menu, rs)
	case rsKindList:
		content = renderList(snap.Body.List, rs, m.ctrl)
	case rsKindDetail:
		content = renderDetail(snap.Body.Detail, rs)
	case rsKindReveal:
		content = renderReveal(rs)
	case rsKindText:
		if rs.ctrlBacked {
			content = renderText(snap.Body.Text, rs)
		} else {
			// Non-ctrl-backed text = error-log overlay. Wrap raw text in a TextBody.
			content = renderText(&app.TextBody{Lines: strings.Split(rs.errorLogText, "\n")}, rs)
		}
	case rsKindSelector:
		content = renderSelector(snap.Body.Selector, rs)
	case rsKindHelp:
		content = renderHelp(rs)
	case rsKindIdentity:
		content = renderIdentity(rs, m.core.Profile(), m.core.Region())
	}

	rightContent := m.headerRight()
	badge := m.accountBadge()
	role := m.identityRoleName()
	// §7.4: when stack depth exceeds 4, show "[N]" in place of the version string.
	displayVersion := Version
	if len(m.stack) > 4 {
		displayVersion = fmt.Sprintf("[%d]", len(m.stack))
	}
	cacheKey := headerProfile + ":" + headerRegion + ":" + displayVersion + ":" + rightContent + ":" + badge + ":" + role + ":" + fmt.Sprintf("%d", m.width)
	header := m.headerCache
	if cacheKey != m.headerCacheKey {
		header = layout.RenderHeader(headerProfile, headerRegion, displayVersion, m.width, rightContent, badge, role)
		m.headerCache = header
		m.headerCacheKey = cacheKey
	}

	var lines []string
	if content != "" {
		lines = strings.Split(content, "\n")
	}
	frameHeight := max(m.height-1, 3)

	// Frame title: derive from rs.kind and controller state.
	frameTitle := m.frameTitle(rs, snap)

	// Footer hints: controller owns hints for ctrl-backed screens.
	var hints []layout.KeyHint
	switch rs.kind {
	case rsKindMenu, rsKindList, rsKindDetail:
		if ctrlHints := snap.Footer; len(ctrlHints) > 0 {
			hints = make([]layout.KeyHint, len(ctrlHints))
			for i, kh := range ctrlHints {
				hints[i] = layout.KeyHint{Key: kh.Key, Desc: kh.Help}
			}
		}
	case rsKindText:
		// Use the controller footer only when the controller is actually on a
		// text screen — so that the error-log overlay (not ctrl-backed) doesn't
		// inherit the previous screen's hints from the controller.
		if rs.ctrlBacked && snap.Body.Kind == app.BodyKindText {
			if ctrlHints := snap.Footer; len(ctrlHints) > 0 {
				hints = make([]layout.KeyHint, len(ctrlHints))
				for i, kh := range ctrlHints {
					hints[i] = layout.KeyHint{Key: kh.Key, Desc: kh.Help}
				}
			}
		}
	}

	frame := layout.RenderFrameWithHints(lines, frameTitle, hints, m.width, frameHeight)

	v := tea.NewView(header + "\n" + frame)
	v.AltScreen = true
	return v
}

// frameTitle returns the frame title string for the current active screen.
func (m Model) frameTitle(rs *rendererState, snap app.ViewState) string {
	switch rs.kind {
	case rsKindMenu:
		return m.ctrl.MenuFrameTitle()
	case rsKindList:
		if t := m.ctrl.ListFrameTitle(); t != "" {
			return t
		}
		return rs.resourceType
	case rsKindDetail:
		src := m.ctrl.GetDetailResource()
		if src.ID != "" {
			if src.Name != "" {
				return fmt.Sprintf("detail -- %s (%s)", src.ID, src.Name)
			}
			return "detail -- " + src.ID
		}
		return "detail"
	case rsKindReveal:
		if rs.revealName != "" {
			return "reveal -- " + rs.revealName
		}
		return "reveal"
	case rsKindText:
		if !rs.ctrlBacked {
			return "errors"
		}
		screenID := m.ctrl.TextFrameTitle()
		if screenID == "" {
			return "text"
		}
		// Build resource-qualified title: "<name> yaml" or "<id> yaml".
		if rs.textResource != nil && rs.textResource.ID != "" {
			label := rs.textResource.Name
			if label == "" {
				label = rs.textResource.ID
			}
			return label + " " + screenID
		}
		return screenID
	case rsKindSelector:
		if t := m.ctrl.SelectorFrameTitle(); t != "" {
			return t
		}
		if snap.Body.Selector != nil {
			return snap.Body.Selector.Title
		}
		return "selector"
	case rsKindHelp:
		return "help"
	case rsKindIdentity:
		return "identity"
	}
	return ""
}

// headerRight returns the pre-rendered right-side string for the header.
func (m Model) headerRight() string {
	switch m.inputMode {
	case modeFilter:
		return styles.FilterActive.Render("/" + m.cmdInput.Value())
	case modeCommand:
		return styles.FilterActive.Render(":" + m.cmdInput.Value())
	}
	// Show search state from active rendererState.
	rs := m.activeRS()
	if rs.search.IsInputMode() {
		return styles.FilterActive.Render("/" + rs.search.Query())
	}
	if rs.search.IsActive() {
		info := rs.search.MatchInfo()
		if info != "" {
			return styles.FilterActive.Render(info)
		}
	}
	// Right-column filter on detail screens — show filter text so the user can
	// see what they've typed or confirmed, mirroring the list-filter "/" display.
	if rs.kind == rsKindDetail && (rs.rightCol.IsFiltering() || rs.rightCol.HasFilter()) {
		return styles.FilterActive.Render("/" + rs.rightCol.FilterQuery())
	}
	if m.flash.active {
		text := m.flash.text
		// Truncate to prevent header wrapping (fixes #84).
		// Reserve ~40 chars for the left side (a9s + version + profile:region + padding).
		// Errors get more header width; reserve 6 chars minimum for the brand + gap.
		maxFlash := max(m.width-40, 20)
		if m.flash.isError {
			maxFlash = max(m.width-6, 20) // errors get wider display; 6 = innerPad(2)+minLeft(3)+gap(1)
		}
		if lipgloss.Width(text) > maxFlash {
			// Truncate by runes to handle Unicode safely.
			runes := []rune(text)
			if len(runes) > maxFlash-3 {
				text = string(runes[:maxFlash-3]) + "..."
			}
		}
		if m.flash.isError {
			return styles.FlashError.Render(text)
		}
		return styles.FlashSuccess.Render(text)
	}
	if rs.kind == rsKindReveal {
		return styles.FlashError.Render("Secret visible — press esc to close")
	}
	if m.showErrorHint && len(m.errorHistory) > 0 {
		return styles.FlashError.Render("! for errors")
	}
	return styles.DimText.Render("? for help")
}

// accountBadge returns the account alias (preferred) or account ID for the header.
func (m Model) accountBadge() string {
	id := m.core.Identity()
	if id == nil {
		return ""
	}
	if id.AccountAlias != "" {
		return id.AccountAlias
	}
	return id.AccountID
}

// identityRoleName returns the identity name (role or user) for the header.
func (m Model) identityRoleName() string {
	id := m.core.Identity()
	if id == nil {
		return ""
	}
	return id.IdentityName
}

// identityToViewData converts the session-cached caller identity (mirrored
// to the renderer-shaped *domain.CallerIdentity by m.core.Identity()) to a
// view-layer IdentityData. Used at identity rs construction time (the
// `i` key press) to seed the rs from current session state before the
// in-flight identity fetch returns. The post-h4-b SetIdentityIntent
// updates the identity rs via applyIntents using the same domain mirror —
// this helper covers the construction path that runs before any intent fires.
func (m Model) identityToViewData() views.IdentityData {
	id := m.core.Identity()
	if id == nil {
		return views.IdentityData{}
	}
	return views.IdentityData{
		AccountID:     id.AccountID,
		AccountAlias:  id.AccountAlias,
		ARN:           id.Arn,
		RoleName:      id.RoleName,
		UserName:      id.UserName,
		SessionName:   id.SessionName,
		IsAssumedRole: id.IsAssumedRole,
	}
}
