// screens.go — Bubble Tea adapter's screen-builder registry. This is the
// renderer-side parallel of runtime.ScreenRegistry: the runtime emits
// PushScreen{ID, Context, Payload}; the adapter resolves ID through the
// builders map below and invokes the closure to construct the concrete
// views.View plus any follow-up tea.Cmd. The runtime never sees
// tea.Model or views.View; the adapter never invents ScreenIDs (they
// live in internal/runtime).
//
// Three builders ship in PR-05a-h4-a (AS-769) for the four ported
// view-stack handlers: profile selector, reveal, child list. Capability
// screens (logs, ct.scan, cost) remain out of scope until their handler
// PRs land.
package tui

import (
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// screenBuilder constructs a renderer-side view from a runtime-emitted
// screen payload. The adapter holds one builder per runtime.ScreenID
// and invokes it with the live *Model so the builder sees the current
// keymap, viewConfig, and innerSize() — not a snapshot from tui.New
// (the model is passed by value through Update so a builder closure
// that captured *Model at construction would see a stale, zero-sized
// copy after the first WindowSizeMsg).
//
// A builder may return a nil view to signal "skip the push" — used by
// the child-list builder when the runtime emitted an unknown ChildType
// (defensive: Core already validates, so this is belt-and-suspenders).
type screenBuilder func(m *Model, payload runtime.ScreenPayload) (views.View, tea.Cmd)

// builders maps runtime.ScreenID -> concrete builder closure. Populated
// once in tui.New(...) via defaultBuilders; tests may shadow individual
// entries by assigning to m.screens after construction.
type builders map[runtime.ScreenID]screenBuilder

// defaultBuilders returns the canonical builder set for the live TUI
// adapter. Each builder reads m.keys, m.viewConfig, and m.innerSize()
// from the *Model passed in at invocation time so successive
// PushScreens reflect the current keymap / viewConfig / terminal size.
func defaultBuilders() builders {
	return builders{
		runtime.ScreenProfileSelector: func(m *Model, p runtime.ScreenPayload) (views.View, tea.Cmd) {
			psp, ok := p.(runtime.ProfileSelectorPayload)
			if !ok {
				return nil, nil
			}
			m.ctrl.EnsureSelectorState(psp.Profiles, psp.Current, "aws-profiles")
			v := views.NewSelectorWithCtrl(m.ctrl, func(s string) tea.Msg {
				return messages.ProfileSelected{Profile: s}
			}, m.keys)
			v.SetSize(m.innerSize())
			return &v, nil
		},
		runtime.ScreenReveal: func(m *Model, p runtime.ScreenPayload) (views.View, tea.Cmd) {
			rp, ok := p.(runtime.RevealPayload)
			if !ok {
				return nil, nil
			}
			v := views.NewReveal(rp.ResourceID, rp.Value, m.keys)
			v.SetSize(m.innerSize())
			return &v, nil
		},
		runtime.ScreenChildList: func(m *Model, p runtime.ScreenPayload) (views.View, tea.Cmd) {
			clp, ok := p.(runtime.ChildListPayload)
			if !ok {
				return nil, nil
			}
			childTypeDef := resource.GetChildType(clp.ChildType)
			if childTypeDef == nil {
				return nil, nil
			}
			rl := views.NewChildResourceList(*childTypeDef, clp.ParentContext, clp.DisplayName, m.viewConfig, m.keys, m.ctrl)
			rl.SetSize(m.innerSize())
			rl, initCmd := rl.Init()
			return &rl, initCmd
		},
	}
}

// dispatchCoreScreenResult walks the intents/tasks returned by an h4-a
// Handle* method using the plural-semantics path (applyIntents +
// tasksToCmd). The key behavioural difference from dispatchHandlerResult
// (used by h3 handlers) is that FlashIntent is re-emitted as
// messages.Flash via cmd rather than direct-mutated. h3 handlers
// pre-bump m.flash.gen and pair FlashIntent with FlashTickPayload so
// direct-mutation works there; h4-a handlers do not own the flash gen
// dance (their flashes route back through handleFlash → HandleFlash to
// pick up the auto-clear tick), so re-emitting preserves the
// pre-port observable behaviour: the cmd produces a messages.Flash
// that downstream tests assert on.
func (m *Model) dispatchCoreScreenResult(intents []runtime.UIIntent, tasks []runtime.TaskRequest) tea.Cmd {
	cmds := m.applyIntents(intents)
	if tc := m.tasksToCmd(tasks); tc != nil {
		cmds = append(cmds, tc)
	}
	switch len(cmds) {
	case 0:
		return nil
	case 1:
		return cmds[0]
	default:
		return tea.Batch(cmds...)
	}
}

// readThemeFileCmd resolves the theme file path via config.ThemePath and
// reads the YAML bytes from disk, dispatching the result (or error) as
// messages.ThemeFileRead so HandleThemeFileRead can emit the
// apply/pop/flash/save sequence. Pure adapter-side I/O — the runtime
// owns no file-system handles.
func readThemeFileCmd(p runtime.ReadThemePayload) tea.Cmd {
	theme := p.Theme
	return func() tea.Msg {
		path, err := config.ThemePath(theme)
		if err != nil {
			return messages.ThemeFileRead{Theme: theme, Err: err}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return messages.ThemeFileRead{Theme: theme, Err: err}
		}
		return messages.ThemeFileRead{Theme: theme, Bytes: data}
	}
}

// saveThemeConfigCmd persists the chosen theme name to config.yaml via
// config.SaveTheme. On error the adapter surfaces a flash; on success
// no message fires (the user already saw the success flash emitted by
// HandleThemeFileRead's intent slice). Decoupled from the apply step so
// a save failure does not roll back the in-memory theme change —
// matching the documented Option B trade-off.
func saveThemeConfigCmd(p runtime.SaveThemeConfigPayload) tea.Cmd {
	theme := p.Theme
	return func() tea.Msg {
		if err := config.SaveTheme(theme); err != nil {
			return messages.Flash{
				Text:    "Cannot save theme config: " + err.Error(),
				IsError: true,
			}
		}
		return nil
	}
}
