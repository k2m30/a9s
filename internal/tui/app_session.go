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
// PR-05a-h4-a (AS-769) ports the remaining four view-stack handlers
// (handleProfilesLoaded, handleValueRevealed, handleEnterChildView,
// handleThemeSelected) to (c *Core) Handle* methods backed by the
// screen-builder registry in screens.go. handleThemeFileRead is a new
// thin adapter introduced by h4-a to complete the two-step theme flow
// (read task → HandleThemeFileRead → Apply/Pop/Flash + Save task).

import (
	tea "charm.land/bubbletea/v2"

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

// handleThemeSelected delegates to runtime.Core.HandleThemeSelected, which
// resolves the path via config.ThemePath and emits a TaskKindReadThemeFile.
// The adapter task closure performs the os.ReadFile and dispatches
// messages.ThemeFileRead → handleThemeFileRead for the apply/pop/save flow.
func (m Model) handleThemeSelected(msg messages.ThemeSelected) (tea.Model, tea.Cmd) {
	intents, tasks := m.core.HandleThemeSelected(runtime.ThemeSelectedEvent{Theme: msg.Theme})
	cmd := m.dispatchCoreScreenResult(intents, tasks)
	return m, cmd
}

// handleThemeFileRead delegates to runtime.Core.HandleThemeFileRead, which
// branches on read error → flash or success → ApplyThemeIntent (bytes) +
// PopSelectorIntent + success Flash + TaskKindSaveThemeConfig.
func (m Model) handleThemeFileRead(msg messages.ThemeFileRead) (tea.Model, tea.Cmd) {
	// Validate the theme YAML here in the adapter — styles is a renderer
	// package, so the runtime stays renderer-agnostic (SC-009) by branching on
	// the resulting ParseErr rather than parsing the bytes itself. Only parse
	// when the read succeeded; a read error short-circuits in the runtime.
	var parseErr error
	if msg.Err == nil {
		_, parseErr = styles.ThemeFromYAML(msg.Bytes)
	}
	intents, tasks := m.core.HandleThemeFileRead(runtime.ThemeFileReadEvent{
		Theme: msg.Theme, Bytes: msg.Bytes, Err: msg.Err, ParseErr: parseErr,
	})
	cmd := m.dispatchCoreScreenResult(intents, tasks)
	return m, cmd
}

// handleProfilesLoaded delegates to runtime.Core.HandleProfilesLoaded,
// which emits PushScreen{ScreenProfileSelector, ProfileSelectorPayload{...}}.
// The screens.go builder constructs the SelectorModel and pushes it.
func (m Model) handleProfilesLoaded(msg profilesLoadedMsg) (tea.Model, tea.Cmd) {
	intents, tasks := m.core.HandleProfilesLoaded(runtime.ProfilesLoadedEvent{
		Profiles: msg.profiles,
	})
	cmd := m.dispatchCoreScreenResult(intents, tasks)
	return m, cmd
}

