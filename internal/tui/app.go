package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
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
// m.core.Session() at each use site. See internal/session/session.go for the
// ownership contract. handleProfileSelected / handleRegionSelected call
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

// WithProfile overrides the profile field on the session. Used in tests that need
// a specific profile string without going through the live AWS bootstrap path.
func WithProfile(profile string) Option {
	return func(m *Model) { m.core.Session().Profile = profile }
}

// WithRegion overrides the region field on the session. Used in tests that need
// a specific region string without going through the live AWS bootstrap path.
func WithRegion(region string) Option {
	return func(m *Model) { m.core.Session().Region = region }
}

// WithIsDemo marks the session as demo mode, which skips Wave 2 enrichment.
// Set by the --demo CLI bootstrap path. Distinct from WithNoCache which only
// disables disk persistence.
func WithIsDemo(demo bool) Option {
	return func(m *Model) {
		m.isDemo = demo
	}
}

// WithNoCache disables resource availability caching and background checks.
func WithNoCache(disabled bool) Option {
	return func(m *Model) {
		m.core.Session().NoCache = disabled
	}
}

// WithClients pre-supplies a set of service clients so that Init() emits a
// synthetic ClientsReadyMsg instead of initiating a live AWS connection.
func WithClients(clients *awsclient.ServiceClients) Option {
	return func(m *Model) {
		m.core.Session().PreSuppliedClients = clients
	}
}

// WithActiveTheme sets the initial active theme filename for the selector's
// "(current)" indicator. main.go passes the validated theme after loading it.
func WithActiveTheme(name string) Option {
	return func(m *Model) { m.activeTheme = name }
}

// WithCommand sets the initial resource short name to navigate to on the first
// ClientsReadyMsg. Used by the -c/--command CLI flag to open a resource list
// directly on startup instead of the main menu. The caller is responsible for
// resolving the input via resource.FindResourceType.
func WithCommand(name string) Option {
	return func(m *Model) { m.core.Session().Command = name }
}

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

	sess := session.New()
	sess.Profile = profile
	sess.Region = region
	m := Model{
		core:        runtime.New(sess, resource.AllResourceTypes()),
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

// Init implements tea.Model. Fires a command to establish the AWS session.
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

	headerProfile := m.core.Session().Profile
	headerRegion := m.core.Session().Region
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
	// Sync list count back to main menu when popping directly from list → menu.
	// Only depth 2 (menu → list) triggers this. Related lists (pushed from a detail
	// view at depth 3+) are marked escPops=true because they show filtered subsets
	// of a resource type, not the global population — syncing their count back to
	// the menu badge would overwrite the real global count with a filter result.
	// See app_related.go for related-list construction.
	if len(m.stack) == 2 {
		if rl, ok := m.stack[1].(*views.ResourceListModel); ok {
			if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
				shortName := rl.ShortName()
				if shortName != "" {
					newCount := rl.LoadedCount()
					newTrunc := rl.IsTruncated()
					curCount, known := menu.GetAvailability()[shortName]
					if !newTrunc || !known || newCount > curCount {
						menu.SetAvailability(shortName, newCount)
						menu.SetTruncated(shortName, newTrunc)
					}
					// Sync-back issue count with only-increase guard (T036, FR-022).
					// The list's Status-based issueCount may be lower than the menu's
					// enriched count. Never overwrite a higher enriched count.
					newIssues := rl.IssueCount()
					curIssues := menu.GetIssueCounts()[shortName]
					curIssueTrunc := menu.GetIssueTruncated()[shortName]
					switch {
					case newIssues > curIssues:
						// higher enriched count from rl: take it
						menu.SetIssues(shortName, newIssues, newTrunc)
					case newIssues == curIssues && curIssueTrunc && !newTrunc:
						// count confirmed but stale "+" must clear
						menu.SetIssues(shortName, newIssues, false)
					}
				}
			}
		}
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
		m.cacheTopLevelResourceList(updated)
		return m, cmd
	case *views.DetailModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	case *views.YAMLModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	case *views.JSONModel:
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

func (m *Model) cacheTopLevelResourceList(rl views.ResourceListModel) {
	if rl.ParentContext() != nil || rl.EscPops() {
		return
	}
	rt := rl.ResourceType()
	sortColIdx, sortAsc := rl.SortState()
	m.core.Session().ResourceCache[rt] = &session.ResourceCacheEntry{
		Resources:     rl.AllResources(),
		Pagination:    rl.PaginationState(),
		FilterText:    rl.FilterText(),
		AttentionOnly: rl.AttentionOnly(),
		SortColIdx:    sortColIdx,
		SortAsc:       sortAsc,
		CursorPos:     rl.CursorPosition(),
		HScrollOffset: rl.HScrollOffset(),
	}
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
	if m.core.Session().Identity == nil {
		return ""
	}
	if m.core.Session().Identity.AccountAlias != "" {
		return m.core.Session().Identity.AccountAlias
	}
	return m.core.Session().Identity.AccountID
}

// identityRoleName returns the identity name (role or user) for the header.
func (m Model) identityRoleName() string {
	if m.core.Session().Identity == nil {
		return ""
	}
	return m.core.Session().Identity.IdentityName
}

// identityToViewData converts the session-cached awsclient.CallerIdentity
// to a view-layer IdentityData. Used at IdentityModel construction time
// (the `i` key press) to seed the view from current session state
// before the in-flight identity fetch returns. The post-h4-b
// SetIdentityIntent updates the IdentityModel via applyIntents using
// the *domain.CallerIdentity mirror — this helper covers the construction
// path that runs before any intent fires.
func (m Model) identityToViewData() views.IdentityData {
	if m.core.Session().Identity == nil {
		return views.IdentityData{}
	}
	return views.IdentityData{
		AccountID:     m.core.Session().Identity.AccountID,
		AccountAlias:  m.core.Session().Identity.AccountAlias,
		ARN:           m.core.Session().Identity.Arn,
		RoleName:      m.core.Session().Identity.RoleName,
		UserName:      m.core.Session().Identity.UserName,
		SessionName:   m.core.Session().Identity.SessionName,
		IsAssumedRole: m.core.Session().Identity.IsAssumedRole,
	}
}
