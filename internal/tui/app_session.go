package tui

// app_session.go — AWS-session lifecycle handlers: clients-ready bootstrap,
// profile/region/theme selectors, and the profile-list loader. Split out of
// internal/tui/app_handlers.go in Phase-05 PR-05a-h1 (AS-147). The three
// session-driven handlers (handleClientsReady, handleProfileSelected,
// handleRegionSelected) were ported to runtime.Core in PR-05a-h3 (AS-324)
// and are ≤12-line adapters here that pre-bump the flash gen, translate
// the messages.* into the runtime.*Event, call the Core method, apply
// returned intents, and translate returned tasks into tea.Cmds.
//
// handleThemeSelected and handleProfilesLoaded stay in the TUI adapter
// for now — they push concrete view structs onto the adapter view stack,
// which requires the screen-builder registry that lands in a successor PR.

import (
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// handleClientsReady defers to runtime.Core.HandleClientsReady for the
// connect-lifecycle decision and translates the returned intents/tasks
// back into Bubble Tea side effects. The flash gen bump is committed
// only when Core actually emits flash work (FlashIntent or
// FlashTickPayload) — non-flash result branches (stale gen → nil/nil;
// success-no-pending-refresh → FetchIdentity+LoadAvailCache only) must
// leave flash.gen alone so that any ClearFlashMsg already in flight for
// the current flash still matches and clears on schedule (CXR/Architect
// Stage 5 R3 finding on the prior `len(intents)>0||len(tasks)>0` gate).
func (m Model) handleClientsReady(msg messages.ClientsReady) (tea.Model, tea.Cmd) {
	_, hasRL := m.activeView().(*views.ResourceListModel)
	intents, tasks := m.core.HandleClientsReady(runtime.ClientsReadyEvent{
		Clients: msg.Clients, Err: msg.Err, Region: msg.Region, Gen: msg.Gen,
		StackDepth: len(m.stack), HasActiveRL: hasRL, NewGen: m.flash.gen + 1,
	})
	if hasFlashWork(intents, tasks) {
		m.flash.gen++
	}
	cmd := m.dispatchHandlerResult(intents, tasks)
	return m, cmd
}

// hasFlashWork returns true when Core's result emits flash work — i.e.
// a new flash (FlashIntent) is being set or an auto-clear tick
// (FlashTickPayload) is being scheduled. Used by handleClientsReady to
// gate the flash.gen bump so non-flash success paths do not invalidate
// any in-flight ClearFlashMsg for the current flash.
func hasFlashWork(intents []runtime.UIIntent, tasks []runtime.TaskRequest) bool {
	for _, in := range intents {
		if _, ok := in.(runtime.FlashIntent); ok {
			return true
		}
	}
	for _, t := range tasks {
		if _, ok := t.Payload.(runtime.FlashTickPayload); ok {
			return true
		}
	}
	return false
}

// handleProfileSelected defers to runtime.Core.HandleProfileSelected for
// the Session.Rotate + rollback-latch + reconnect-request sequence.
func (m Model) handleProfileSelected(msg messages.ProfileSelected) (tea.Model, tea.Cmd) {
	m.flash.gen++
	intents, tasks := m.core.HandleProfileSelected(runtime.ProfileSelectedEvent{
		Profile: msg.Profile, NewGen: m.flash.gen,
	})
	cmd := m.dispatchHandlerResult(intents, tasks)
	return m, cmd
}

// handleRegionSelected defers to runtime.Core.HandleRegionSelected for
// the Session.Rotate + rollback-latch + reconnect-request sequence.
func (m Model) handleRegionSelected(msg messages.RegionSelected) (tea.Model, tea.Cmd) {
	m.flash.gen++
	intents, tasks := m.core.HandleRegionSelected(runtime.RegionSelectedEvent{
		Region: msg.Region, NewGen: m.flash.gen,
	})
	cmd := m.dispatchHandlerResult(intents, tasks)
	return m, cmd
}

// handleThemeSelected applies the selected theme, invalidates style caches,
// pops the selector, and persists the choice to config.yaml. Stays in the
// TUI adapter pending the screen-builder registry that successor PRs need
// to push the selector view from a Core method.
func (m Model) handleThemeSelected(msg messages.ThemeSelected) (tea.Model, tea.Cmd) {
	path, err := config.ThemePath(msg.Theme)
	if err != nil {
		return m, func() tea.Msg {
			return messages.Flash{Text: "Invalid theme: " + err.Error(), IsError: true}
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return m, func() tea.Msg {
			return messages.Flash{Text: "Cannot read theme: " + err.Error(), IsError: true}
		}
	}
	t, err := styles.ThemeFromYAML(data)
	if err != nil {
		return m, func() tea.Msg {
			return messages.Flash{Text: "Bad theme YAML: " + err.Error(), IsError: true}
		}
	}

	// Persist BEFORE applying — if save fails, abort the change entirely.
	if saveErr := config.SaveTheme(msg.Theme); saveErr != nil {
		return m, func() tea.Msg {
			return messages.Flash{Text: "Cannot save theme config: " + saveErr.Error(), IsError: true}
		}
	}

	// Save succeeded — now apply the theme.
	styles.ApplyTheme(t)
	m.activeTheme = msg.Theme

	// Invalidate header cache.
	m.headerCacheKey = ""

	// Invalidate styledRowCache on all ResourceListModel views in the stack.
	for _, v := range m.stack {
		if rl, ok := v.(*views.ResourceListModel); ok {
			rl.InvalidateStyleCache()
		}
	}

	// Pop the selector view.
	if _, ok := m.activeView().(*views.SelectorModel); ok {
		m.popView()
	}

	return m, func() tea.Msg {
		return messages.Flash{Text: "Theme: " + msg.Theme}
	}
}

// handleProfilesLoaded pushes the profile selector view onto the stack.
func (m Model) handleProfilesLoaded(msg profilesLoadedMsg) (tea.Model, tea.Cmd) {
	p := views.NewProfile(msg.profiles, m.core.Session().Profile, m.keys)
	p.SetSize(m.innerSize())
	m.pushView(&p)
	return m, nil
}
