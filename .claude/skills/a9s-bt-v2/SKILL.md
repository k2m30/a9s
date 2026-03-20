---
name: a9s-bt-v2
description: Bubble Tea v2 / Lipgloss v2 / Bubbles v2 API patterns for TUI-touching agents
---

## Bubble Tea v2 (`charm.land/bubbletea/v2` v2.0.2)

- `Init() tea.Cmd` — NOT `(tea.Model, tea.Cmd)` (that was BT v1)
- Root `View() tea.View` via `tea.NewView(string)` — child views return `string`
- Root `Update() (tea.Model, tea.Cmd)` — child `Update() (ConcreteType, tea.Cmd)`
- ALL I/O in `tea.Cmd` — NEVER block in Update()
- `tea.Tick(duration, func(time.Time) tea.Msg) tea.Cmd` for timers

## Lipgloss v2 (`charm.land/lipgloss/v2` v2.0.2)

- `lipgloss.Width(s)` for ANSI-aware width — NEVER `len(s)`
- `lipgloss.NewStyle().Foreground(color).Bold(true).Render(text)`
- NO `lipgloss.Place` for frame construction — frames are manual per design spec

## Bubbles v2 (`charm.land/bubbles/v2` v2.0.0)

- `viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))` — NOT `viewport.New(w, h)`
- `vp.SetWidth(w)`, `vp.SetHeight(h)`, `vp.SetContent(s)` — NOT field assignment
- `key.NewBinding(key.WithKeys(...), key.WithHelp(...))` + `key.Matches(msg, binding)`
- ALL bindings in `internal/tui/keys/keys.go` — no inline `key.NewBinding`

## Go Module Cache

Read BT/Lipgloss/Bubbles source directly from:
- `/Users/k2m30/go/pkg/mod/charm.land/bubbletea/v2@v2.0.2/`
- `/Users/k2m30/go/pkg/mod/charm.land/lipgloss/v2@v2.0.2/`
- `/Users/k2m30/go/pkg/mod/charm.land/bubbles/v2@v2.0.0/`
