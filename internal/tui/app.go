package tui

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// Version is set by cmd/a9s/main.go.
var Version string

// inputMode distinguishes the active header input state.
type inputMode int

const (
	modeNormal  inputMode = iota
	modeFilter            // "/" active
	modeCommand           // ":" active
)

// flashState holds transient header right-side message.
type flashState struct {
	text    string
	isError bool
	active  bool
	gen     domain.Gen // generation counter to avoid stale clears
}

// errorEntry records a single error for the session error log.
type errorEntry struct {
	time    time.Time
	message string
}

// Model is the root Bubble Tea model. It owns the view stack, header state,
// AWS clients, and routes all messages to the active child view.
//
// Session state is owned exclusively by core via core.Session(). Call
// m.core.Session() at each use site. See the internal/session package for
// the ownership contract. handleProfileSelected / handleRegionSelected call
// m.core.Session().Rotate() to invalidate in-flight async results on switch.
type Model struct {
	core    *runtime.Core    // platform-agnostic app core

	// --- UI shell state ---
	width  int
	height int

	appCtx    context.Context
	appCancel context.CancelFunc

	stack []views.View

	inputMode inputMode
	cmdInput  textinput.Model
	flash     flashState

	errorHistory  []errorEntry
	showErrorHint bool

	// Tab-completion cycle state for command mode. tabPrefix is the user's
	// original input at the start of a cycle; tabMatches are all candidates
	// matching that prefix; tabIndex is the currently shown match. Cleared
	// (tabPrefix="") on any non-Tab key so the next Tab starts a fresh cycle.
	tabPrefix  string
	tabMatches []string
	tabIndex   int

	keys        keys.Map
	viewConfig  *config.ViewsConfig
	configErr   error  // non-nil if views config was found but corrupt
	activeTheme string // current theme filename (for selector "(current)" indicator)

	// headerCache avoids re-computing the header string every render when
	// profile, region, version, and right-side content haven't changed.
	headerCache    string
	headerCacheKey string

	isDemo bool // true when running in --demo mode (synthetic clients); controls Wave 2 skip

	// screens is the renderer-side parallel of runtime.ScreenRegistry: it
	// resolves runtime.ScreenID -> builder closure for the four ported
	// view-stack handlers (HandleProfilesLoaded, HandleValueRevealed,
	// HandleEnterChildView, HandleThemeSelected). Populated once in New()
	// via defaultBuilders(&m); tests may shadow entries by assigning to
	// individual map keys.
	screens builders
}

// Option configures the root Model.
type Option func(*Model)

// New constructs the initial Model.
func New(profile, region string, opts ...Option) Model {
	ti := textinput.New()
	k := keys.Default()

	menu := views.NewMainMenu(k)

	// Load view config synchronously (fast local file read).
	cfg, cfgErr := config.Load()
	if cfg == nil {
		// Use the shared read-only default — tui.Model never mutates viewConfig.
		cfg = config.SharedDefaultConfig()
	}

	// Create the app-wide context first so it can be passed to AWS client
	// construction and threaded through all fetchers.
	ctx, cancel := context.WithCancel(context.Background())

	m := Model{
		core:        runtime.Bootstrap(profile, region, resource.AllResourceTypes()),
		keys:        k,
		stack:       []views.View{&menu},
		cmdInput:    ti,
		viewConfig:  cfg,
		configErr:   cfgErr,
		activeTheme: "tokyo-night.yaml",
		appCtx:      ctx,
		appCancel:   cancel,
	}
	m.screens = defaultBuilders()
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

// AppContext returns the app-wide context. It is cancelled when the app quits.
func (m Model) AppContext() context.Context {
	return m.appCtx
}

// Cancel invokes the app context cancel function to signal in-flight goroutines
// to abort. Safe to call multiple times and on a zero-value Model.
func (m Model) Cancel() {
	if m.appCancel != nil {
		m.appCancel()
	}
}

// Init implements tea.Model. Fires a command to establish the live AWS connection.
// When pre-supplied clients are present (demo mode or tests), emits a synthetic
// ClientsReadyMsg immediately. Otherwise initiates the live AWS connection flow.
func (m Model) Init() tea.Cmd {
	if m.core.Session().PreSuppliedClients != nil {
		preCmd := func() tea.Msg {
			return messages.ClientsReady{Clients: m.core.Session().PreSuppliedClients}
		}
		if m.configErr != nil {
			return tea.Batch(preCmd, func() tea.Msg {
				return messages.Flash{
					Text:    fmt.Sprintf("Config error: %v (using defaults)", m.configErr),
					IsError: true,
				}
			})
		}
		return preCmd
	}
	connectCmd := func() tea.Msg {
		return messages.InitConnect{
			Profile: m.core.Session().Profile,
			Region:  m.core.Session().Region,
		}
	}
	if m.configErr != nil {
		return tea.Batch(connectCmd, func() tea.Msg {
			return messages.Flash{
				Text:    fmt.Sprintf("Config error: %v (using defaults)", m.configErr),
				IsError: true,
			}
		})
	}
	return connectCmd
}

// Update implements tea.Model. Routes messages to global handlers or active view.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.QuitMsg:
		if m.appCancel != nil {
			m.appCancel()
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.propagateSize()
		if d, ok := m.activeView().(*views.DetailModel); ok && d.TakePendingRelatedDispatch() {
			src := d.SourceResource()
			rtype := d.ResourceType()
			return m, func() tea.Msg {
				return messages.RelatedCheckStarted{
					ResourceType:   rtype,
					SourceResource: src,
				}
			}
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case tea.PasteMsg:
		if m.inputMode == modeFilter || m.inputMode == modeCommand {
			var cmd tea.Cmd
			m.cmdInput, cmd = m.cmdInput.Update(msg)
			if m.inputMode == modeFilter {
				m.applyFilterToActiveView(m.cmdInput.Value())
			}
			return m, cmd
		}
		return m.updateActiveView(msg)
	case messages.Navigate:
		return m.handleNavigate(msg)
	case messages.PopView:
		m.popView()
		return m, nil
	case messages.Flash:
		return m.handleFlash(msg)
	case messages.ClearFlash:
		return m.handleClearFlash(msg)
	case messages.InitConnect:
		cmd := m.connectAWS(msg.Profile, msg.Region, m.core.Session().ConnectGen)
		return m, cmd
	case messages.ClientsReady:
		return m.handleClientsReady(msg)
	case messages.ProfileSelected:
		return m.handleProfileSelected(msg)
	case messages.RegionSelected:
		return m.handleRegionSelected(msg)
	case messages.ThemeSelected:
		return m.handleThemeSelected(msg)
	case messages.ThemeFileRead:
		return m.handleThemeFileRead(msg)
	case profilesLoadedMsg:
		return m.handleProfilesLoaded(msg)
	case messages.ValueRevealed:
		if messages.IsStale(msg, m.core.Session()) {
			return m, nil
		}
		return m.handleValueRevealed(msg)
	case messages.EnterChildView:
		return m.handleEnterChildView(msg)
	case messages.LoadResources:
		var cmd tea.Cmd
		if len(msg.ParentContext) > 0 {
			cmd = m.fetchChildResources(msg.ResourceType, msg.ParentContext)
		} else {
			cmd = m.fetchResources(msg.ResourceType, m.core.Session().AvailabilityGen)
		}
		return m, cmd
	case messages.LoadMore:
		cmd := m.fetchMoreResources(msg)
		return m, cmd
	case messages.APIError:
		if messages.IsStale(msg, m.core.Session()) {
			return m, nil
		}
		return m.handleAPIError(msg)
	case messages.ResourcesLoaded:
		return m.handleResourcesLoaded(msg)
	case messages.IdentityLoaded:
		return m.coreUpdate(msg)
	case messages.IdentityError:
		return m.handleIdentityError(msg)
	case messages.AvailabilityCacheLoaded:
		return m.coreUpdate(msg)
	case messages.AvailabilityPrefetched:
		return m.coreUpdate(msg)
	case messages.AvailabilityChecked:
		return m.coreUpdate(msg)
	case messages.EnrichmentChecked:
		return m.coreUpdate(msg)
	case messages.EnrichDetail:
		return m.handleEnrichDetail(msg)
	case messages.EnrichDetailResult:
		return m.handleEnrichDetailResult(msg)
	case messages.RelatedCheckStarted:
		return m.handleRelatedCheckStarted(msg)
	case messages.RelatedCheckResult:
		return m.handleRelatedCheckResult(msg)
	case messages.RelatedNavigate:
		return m.handleRelatedNavigate(msg)
	}
	// Route unmatched messages to cmdInput when in filter/command mode.
	// This handles textinput-internal clipboard messages (e.g., pasteMsg returned
	// by textinput.Paste) that need to reach m.cmdInput to complete the paste.
	if m.inputMode == modeFilter || m.inputMode == modeCommand {
		var cmd tea.Cmd
		m.cmdInput, cmd = m.cmdInput.Update(msg)
		if m.inputMode == modeFilter {
			m.applyFilterToActiveView(m.cmdInput.Value())
		}
		return m, cmd
	}
	return m.updateActiveView(msg)
}

