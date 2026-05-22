// app_view.go — PR-05a-h4-c (AS-963) tui.Model render-path helpers.
//
// Split out of app.go so the View() composition + header-right /
// account-badge / identity-role / identity-to-view-data helpers live in
// their own file and app.go stays inside the 300–400 LOC budget that the
// spec acceptance check enforces (`wc -l internal/tui/app.go`).
//
// All five functions here are pure renderer-side composition: they read
// view-stack state plus a small slice of session metadata (profile,
// region, identity) and produce strings or tea.View values for the
// renderer to display. No runtime / handler logic lives here.
package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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

	active := m.activeView()

	headerProfile := m.core.Profile()
	headerRegion := m.core.Region()
	if sel, ok := active.(*views.SelectorModel); ok {
		if sel.Title() == "aws-regions" {
			headerRegion = "..."
		} else if sel.Title() == "aws-profiles" {
			headerProfile = "..."
		}
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

	content := active.View()
	var lines []string
	if content != "" {
		lines = strings.Split(content, "\n")
	}
	frameHeight := max(m.height-1, 3)
	frameTitle := active.FrameTitle()
	if d, ok := active.(*views.DetailModel); ok {
		src := d.SourceResource()
		if src.ID != "" {
			if src.Name != "" {
				frameTitle = fmt.Sprintf("detail -- %s (%s)", src.ID, src.Name)
			} else {
				frameTitle = "detail -- " + src.ID
			}
		}
	}
	var hints []layout.KeyHint
	if h, ok := active.(views.Hintable); ok {
		hints = h.BottomHints()
	}
	frame := layout.RenderFrameWithHints(lines, frameTitle, hints, m.width, frameHeight)

	v := tea.NewView(header + "\n" + frame)
	v.AltScreen = true
	return v
}

// headerRight returns the pre-rendered right-side string for the header.
func (m Model) headerRight() string {
	switch m.inputMode {
	case modeFilter:
		return styles.FilterActive.Render("/" + m.cmdInput.Value())
	case modeCommand:
		return styles.FilterActive.Render(":" + m.cmdInput.Value())
	}
	// Show search state from active view.
	if s, ok := m.activeView().(views.Searchable); ok && s.IsSearchActive() {
		info := s.SearchInfo()
		if info != "" {
			return styles.FilterActive.Render(info)
		}
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
	if rv, ok := m.activeView().(*views.RevealModel); ok {
		return rv.HeaderWarning()
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
// view-layer IdentityData. Used at IdentityModel construction time (the
// `i` key press) to seed the view from current session state before the
// in-flight identity fetch returns. The post-h4-b SetIdentityIntent
// updates the IdentityModel via applyIntents using the same domain mirror —
// this helper covers the construction path that runs before any intent
// fires.
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
