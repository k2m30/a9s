package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
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
	gen     int // generation counter to avoid stale clears
}

// Model is the root Bubble Tea model. It owns the view stack, header state,
// AWS clients, and routes all messages to the active child view.
type Model struct {
	width  int
	height int

	profile string
	region  string
	clients *awsclient.ServiceClients

	stack []views.View

	inputMode inputMode
	cmdInput  textinput.Model
	flash     flashState

	keys           keys.Map
	viewConfig     *config.ViewsConfig
	pendingRefresh bool  // set after profile/region switch to refresh on ClientsReadyMsg
	configErr      error // non-nil if views config was found but corrupt

	identity         *awsclient.CallerIdentity
	identityFetching bool

	// headerCache avoids re-computing the header string every render when
	// profile, region, version, and right-side content haven't changed.
	headerCache    string
	headerCacheKey string

	demoMode bool

	noCache         bool
	availabilityGen int      // incremented on profile/region switch to cancel stale probes
	availQueue      []string // resource short names remaining to probe
	availChecked    int      // number probed so far in current gen
	availTotal      int      // total types to probe in current gen
}

// Option configures the root Model.
type Option func(*Model)

// WithDemo enables demo mode with synthetic fixture data.
func WithDemo(enabled bool) Option {
	return func(m *Model) {
		m.demoMode = enabled
	}
}

// WithNoCache disables resource availability caching and background checks.
func WithNoCache(disabled bool) Option {
	return func(m *Model) {
		m.noCache = disabled
	}
}

// New constructs the initial Model.
func New(profile, region string, opts ...Option) Model {
	ti := textinput.New()
	k := keys.Default()

	menu := views.NewMainMenu(k)

	// Load view config synchronously (fast local file read).
	cfg, cfgErr := config.Load()
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	m := Model{
		profile:    profile,
		region:     region,
		keys:       k,
		stack:      []views.View{&menu},
		cmdInput:   ti,
		viewConfig: cfg,
		configErr:  cfgErr,
	}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

// Init implements tea.Model. Fires a command to establish the AWS session.
func (m Model) Init() tea.Cmd {
	if m.demoMode {
		demoCmd := func() tea.Msg {
			return messages.ClientsReadyMsg{}
		}
		if m.configErr != nil {
			return tea.Batch(demoCmd, func() tea.Msg {
				return messages.FlashMsg{
					Text:    fmt.Sprintf("Config error: %v (using defaults)", m.configErr),
					IsError: true,
				}
			})
		}
		return demoCmd
	}
	connectCmd := func() tea.Msg {
		return messages.InitConnectMsg{
			Profile: m.profile,
			Region:  m.region,
		}
	}
	if m.configErr != nil {
		return tea.Batch(connectCmd, func() tea.Msg {
			return messages.FlashMsg{
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.propagateSize()
		return m, nil
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case messages.NavigateMsg:
		return m.handleNavigate(msg)
	case messages.PopViewMsg:
		m.popView()
		return m, nil
	case messages.FlashMsg:
		return m.handleFlash(msg)
	case messages.ClearFlashMsg:
		return m.handleClearFlash(msg)
	case messages.InitConnectMsg:
		if m.demoMode {
			return m, nil
		}
		cmd := m.connectAWS(msg.Profile, msg.Region)
		return m, cmd
	case messages.ClientsReadyMsg:
		return m.handleClientsReady(msg)
	case messages.ProfileSelectedMsg:
		return m.handleProfileSelected(msg)
	case messages.RegionSelectedMsg:
		return m.handleRegionSelected(msg)
	case profilesLoadedMsg:
		return m.handleProfilesLoaded(msg)
	case messages.SecretRevealedMsg:
		return m.handleSecretRevealed(msg)
	case messages.EnterChildViewMsg:
		return m.handleEnterChildView(msg)
	case messages.LoadResourcesMsg:
		var cmd tea.Cmd
		if len(msg.ParentContext) > 0 {
			cmd = m.fetchChildResources(msg.ResourceType, msg.ParentContext)
		} else {
			cmd = m.fetchResources(msg.ResourceType)
		}
		return m, cmd
	case messages.LoadMoreMsg:
		cmd := m.fetchMoreResources(msg)
		return m, cmd
	case messages.APIErrorMsg:
		return m.handleAPIError(msg)
	case messages.ResourcesLoadedMsg:
		m.flash.active = false
		return m.updateActiveView(msg)
	case messages.IdentityLoadedMsg:
		return m.handleIdentityLoaded(msg)
	case messages.IdentityErrorMsg:
		return m.handleIdentityError(msg)
	case messages.AvailabilityCacheLoadedMsg:
		return m.handleAvailabilityCacheLoaded(msg)
	case messages.AvailabilityCheckedMsg:
		return m.handleAvailabilityChecked(msg)
	}
	return m.updateActiveView(msg)
}

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
	if m.width < 60 {
		return alt("Terminal too narrow (min 60 columns)")
	}
	if m.height < 7 {
		return alt("Terminal too short (min 7 lines)")
	}

	active := m.activeView()

	headerProfile := m.profile
	headerRegion := m.region
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
	cacheKey := headerProfile + ":" + headerRegion + ":" + Version + ":" + rightContent + ":" + badge + ":" + role + ":" + fmt.Sprintf("%d", m.width)
	header := m.headerCache
	if cacheKey != m.headerCacheKey {
		header = layout.RenderHeader(headerProfile, headerRegion, Version, m.width, rightContent, badge, role)
		m.headerCache = header
		m.headerCacheKey = cacheKey
	}

	content := active.View()
	var lines []string
	if content != "" {
		lines = strings.Split(content, "\n")
	}
	frameHeight := m.height - 1
	if frameHeight < 3 {
		frameHeight = 3
	}
	frame := layout.RenderFrame(lines, active.FrameTitle(), m.width, frameHeight)

	v := tea.NewView(header + "\n" + frame)
	v.AltScreen = true
	return v
}

// activeView returns the top of the view stack.
func (m *Model) activeView() views.View {
	return m.stack[len(m.stack)-1]
}

// pushView adds a new view to the stack.
func (m *Model) pushView(v views.View) {
	m.stack = append(m.stack, v)
}

// popView removes the top view. Returns false if only one entry remains.
func (m *Model) popView() bool {
	if len(m.stack) <= 1 {
		return false
	}
	m.stack = m.stack[:len(m.stack)-1]
	return true
}

// innerSize returns the content area dimensions inside the frame.
func (m *Model) innerSize() (int, int) {
	w := m.width - 2
	h := m.height - 3
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

// propagateSize calls SetSize on every view in the stack with inner dimensions.
func (m *Model) propagateSize() {
	w, h := m.innerSize()
	for _, v := range m.stack {
		v.SetSize(w, h)
	}
}

// updateActiveView delegates a message to the active view and merges the result.
func (m Model) updateActiveView(msg tea.Msg) (tea.Model, tea.Cmd) {
	active := m.activeView()
	switch v := active.(type) {
	case *views.MainMenuModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	case *views.ResourceListModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	case *views.DetailModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	case *views.YAMLModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	case *views.RevealModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	case *views.SelectorModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	case *views.HelpModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	case *views.IdentityModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	}
	return m, nil
}

// helpContext determines the HelpContext from the current active view.
func (m *Model) helpContext() views.HelpContext {
	return m.activeView().GetHelpContext()
}

// headerRight returns the pre-rendered right-side string for the header.
func (m Model) headerRight() string {
	switch m.inputMode {
	case modeFilter:
		return styles.FilterActive.Render("/" + m.cmdInput.Value())
	case modeCommand:
		return styles.FilterActive.Render(":" + m.cmdInput.Value())
	}
	if m.flash.active {
		text := m.flash.text
		// Truncate to prevent header wrapping (fixes #84).
		// Reserve ~40 chars for the left side (a9s + version + profile:region + padding).
		maxFlash := m.width - 40
		if maxFlash < 20 {
			maxFlash = 20
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
	return styles.DimText.Render("? for help")
}

// accountBadge returns the account alias (preferred) or account ID for the header.
func (m Model) accountBadge() string {
	if m.identity == nil {
		return ""
	}
	if m.identity.AccountAlias != "" {
		return m.identity.AccountAlias
	}
	return m.identity.AccountID
}

// identityRoleName returns the identity name (role or user) for the header.
func (m Model) identityRoleName() string {
	if m.identity == nil {
		return ""
	}
	return m.identity.IdentityName
}

// identityToViewData converts the cached CallerIdentity to a view-layer IdentityData.
func (m Model) identityToViewData() views.IdentityData {
	if m.identity == nil {
		return views.IdentityData{}
	}
	return views.IdentityData{
		AccountID:     m.identity.AccountID,
		AccountAlias:  m.identity.AccountAlias,
		ARN:           m.identity.Arn,
		RoleName:      m.identity.RoleName,
		UserName:      m.identity.UserName,
		SessionName:   m.identity.SessionName,
		IsAssumedRole: m.identity.IsAssumedRole,
	}
}
