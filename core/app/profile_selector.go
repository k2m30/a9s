// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import "github.com/k2m30/a9s/v3/core/runtime"

// OpenProfileSelector is the blessed headless entry point for opening the
// profile selector (issue #464 part 2). runtime.NavigateKindFetchProfiles
// stays adapter-only — internal/tui's runtime adapter is the only consumer
// of that intent, since it owns the demo-mode guard and the tea.Cmd that
// reads the local AWS config file. A headless host (web, DrainSync, or any
// future non-TUI renderer) has no adapter to run that Navigate case, so this
// method reproduces the same three steps synchronously: the demo guard, the
// profiles fetch, and HandleProfilesLoaded's PushScreen — this is the
// supported pattern for hosts that need to open the profile selector outside
// the TUI.
func (c *Controller) OpenProfileSelector() (ViewState, []runtime.TaskRequest) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.core.PreSuppliedClients() != nil {
		c.applyIntents([]runtime.UIIntent{runtime.FlashIntent{
			Text:    "context switching is disabled in demo mode",
			IsError: true,
		}})
		return c.snapshot(), nil
	}

	profiles, err := c.core.FetchProfiles()
	if err != nil {
		c.applyIntents([]runtime.UIIntent{runtime.FlashIntent{
			Text:    err.Error(),
			IsError: true,
		}})
		return c.snapshot(), nil
	}

	intents, tasks := c.core.HandleProfilesLoaded(runtime.ProfilesLoadedEvent{Profiles: profiles})
	c.applyIntents(intents)
	// Mirrors the TUI's ScreenProfileSelector builder (internal/tui/screens.go),
	// which calls EnsureSelectorState right after the same PushScreen — the
	// lock-free variant, since c.mu is already held here.
	c.ensureSelectorState(profiles, c.core.Profile(), "aws-profiles")
	tasks = c.stampDispatchSnapshotLocked(tasks)
	return c.snapshot(), tasks
}
