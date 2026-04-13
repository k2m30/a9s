package tui

import (
	"container/list"
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
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

// errorEntry records a single error for the session error log.
type errorEntry struct {
	time    time.Time
	message string
}

// resourceCacheEntry stores the state of a previously-viewed resource list.
// Used to restore the list when the user re-enters the same resource type
// from the main menu, avoiding redundant API calls.
type resourceCacheEntry struct {
	resources     []resource.Resource
	pagination    *resource.PaginationMeta
	filterText    string
	attentionOnly bool // §7.3: ctrl+z toggle persisted across view re-entry
	sortColIdx    int
	sortAsc       bool
	cursorPos     int
	hScrollOffset int
}

// Model is the root Bubble Tea model. It owns the view stack, header state,
// AWS clients, and routes all messages to the active child view.
type Model struct {
	width  int
	height int

	profile string
	region  string
	clients *awsclient.ServiceClients

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

	keys           keys.Map
	viewConfig     *config.ViewsConfig
	pendingRefresh bool   // set after profile/region switch to refresh on ClientsReadyMsg
	connectGen     int    // incremented on profile/region switch; stale ClientsReadyMsg ignored
	hasPrevState   bool   // true while prevProfile/prevRegion hold the rollback target
	prevProfile    string // last stable profile before any in-flight switch, restored on failure
	prevRegion     string // last stable region before any in-flight switch, restored on failure
	configErr      error  // non-nil if views config was found but corrupt
	activeTheme    string // current theme filename (for selector "(current)" indicator)
	command        string // initial resource short name to navigate to on first ClientsReadyMsg (from -c flag)

	identity         *awsclient.CallerIdentity
	identityFetching bool

	// headerCache avoids re-computing the header string every render when
	// profile, region, version, and right-side content haven't changed.
	headerCache    string
	headerCacheKey string

	preSuppliedClients *awsclient.ServiceClients

	noCache         bool
	availabilityGen int      // incremented on profile/region switch to cancel stale probes
	availQueue      []string // resource short names remaining to probe
	availChecked    int      // number probed so far in current gen
	availTotal      int      // total types to probe in current gen

	resourceCache map[string]*resourceCacheEntry
	relatedCache  *relatedCacheLRU
	relatedGen    uint64 // incremented on refresh/profile/region switch to discard stale results
	enrichGen     uint64 // incremented on refresh/profile/region switch to discard stale enrichment results
	enrichResKey  string // "resourceType:resourceID" of last enrichment dispatch; gen only bumps on change
}

// relatedCacheKey builds the map key for relatedCache lookups.
func relatedCacheKey(resourceType, resourceID string) string {
	return resourceType + ":" + resourceID
}

// relatedCacheLRU is a simple LRU cache for related-resource check results.
// It caps at maxRelatedCacheEntries entries; the least-recently-used entry
// is evicted when the cap is exceeded. Thread-safety is not required because
// all Model updates run on the single Bubble Tea goroutine.
const maxRelatedCacheEntries = 500

type relatedCacheLRU struct {
	cap   int
	index map[string]*list.Element
	order *list.List
}

type relatedCacheItem struct {
	key     string
	results []resource.RelatedCheckResult
}

func newRelatedCacheLRU(cap int) *relatedCacheLRU {
	return &relatedCacheLRU{
		cap:   cap,
		index: make(map[string]*list.Element),
		order: list.New(),
	}
}

func (c *relatedCacheLRU) get(key string) ([]resource.RelatedCheckResult, bool) {
	el, ok := c.index[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*relatedCacheItem).results, true
}

func (c *relatedCacheLRU) set(key string, results []resource.RelatedCheckResult) {
	if el, ok := c.index[key]; ok {
		c.order.MoveToFront(el)
		el.Value.(*relatedCacheItem).results = results
		return
	}
	el := c.order.PushFront(&relatedCacheItem{key: key, results: results})
	c.index[key] = el
	if c.order.Len() > c.cap {
		back := c.order.Back()
		if back != nil {
			c.order.Remove(back)
			delete(c.index, back.Value.(*relatedCacheItem).key)
		}
	}
}

func (c *relatedCacheLRU) delete(key string) {
	if el, ok := c.index[key]; ok {
		c.order.Remove(el)
		delete(c.index, key)
	}
}

func (c *relatedCacheLRU) clear() {
	c.index = make(map[string]*list.Element)
	c.order.Init()
}

func (c *relatedCacheLRU) len() int {
	return c.order.Len()
}

// Option configures the root Model.
type Option func(*Model)

// WithDemo is a compatibility shim for tests written before the 014-demo-transport-mock
// refactor. New code should use WithClients(demo.NewServiceClients()) + WithNoCache(true).
// Will be removed after test files are migrated in T045–T049.
//
//nolint:gocritic // intentional compat shim; removal tracked in T045–T049
func WithDemo(enabled bool) Option {
	return func(m *Model) {
		if !enabled {
			return
		}
		m.preSuppliedClients = demo.NewServiceClients()
		m.profile = demo.DemoProfile
		m.region = demo.DemoRegion
	}
}

// WithNoCache disables resource availability caching and background checks.
func WithNoCache(disabled bool) Option {
	return func(m *Model) {
		m.noCache = disabled
	}
}

// WithClients pre-supplies a set of service clients so that Init() emits a
// synthetic ClientsReadyMsg instead of initiating a live AWS connection.
func WithClients(clients *awsclient.ServiceClients) Option {
	return func(m *Model) {
		m.preSuppliedClients = clients
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
	return func(m *Model) { m.command = name }
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

	// Create the app-wide context first so it can be passed to AWS client
	// construction and threaded through all fetchers.
	ctx, cancel := context.WithCancel(context.Background())

	m := Model{
		profile:       profile,
		region:        region,
		keys:          k,
		stack:         []views.View{&menu},
		cmdInput:      ti,
		viewConfig:    cfg,
		configErr:     cfgErr,
		activeTheme:   "tokyo-night.yaml",
		resourceCache: make(map[string]*resourceCacheEntry),
		relatedCache:  newRelatedCacheLRU(maxRelatedCacheEntries),
		relatedGen:    1, // start at 1 so Generation=0 (unset) is always stale and rejected
		enrichGen:     1, // same convention as relatedGen
		appCtx:        ctx,
		appCancel:     cancel,
	}
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
	if m.preSuppliedClients != nil {
		preCmd := func() tea.Msg {
			return messages.ClientsReadyMsg{Clients: m.preSuppliedClients}
		}
		if m.configErr != nil {
			return tea.Batch(preCmd, func() tea.Msg {
				return messages.FlashMsg{
					Text:    fmt.Sprintf("Config error: %v (using defaults)", m.configErr),
					IsError: true,
				}
			})
		}
		return preCmd
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
				return messages.RelatedCheckStartedMsg{
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
		cmd := m.connectAWS(msg.Profile, msg.Region, m.connectGen)
		return m, cmd
	case messages.ClientsReadyMsg:
		return m.handleClientsReady(msg)
	case messages.ProfileSelectedMsg:
		return m.handleProfileSelected(msg)
	case messages.RegionSelectedMsg:
		return m.handleRegionSelected(msg)
	case messages.ThemeSelectedMsg:
		return m.handleThemeSelected(msg)
	case profilesLoadedMsg:
		return m.handleProfilesLoaded(msg)
	case messages.ValueRevealedMsg:
		return m.handleValueRevealed(msg)
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
		updated, cmd := m.updateActiveView(msg)
		// Write-through: update cache after view has processed the message.
		if updatedModel, ok := updated.(Model); ok {
			if rl, ok := updatedModel.activeView().(*views.ResourceListModel); ok {
				// Only cache top-level resource lists, not child views.
				if rl.ParentContext() == nil && !rl.EscPops() {
					rt := rl.ResourceType()
					sortColIdx, sortAsc := rl.SortState()
					updatedModel.resourceCache[rt] = &resourceCacheEntry{
						resources:     rl.AllResources(),
						pagination:    rl.PaginationState(),
						filterText:    rl.FilterText(),
						attentionOnly: rl.AttentionOnly(),
						sortColIdx:    sortColIdx,
						sortAsc:       sortAsc,
						cursorPos:     rl.CursorPosition(),
						hScrollOffset: rl.HScrollOffset(),
					}
					return updatedModel, cmd
				}
			} else if msg.ResourceType != "" && !msg.Append {
				// Active view is not a ResourceList for this type (e.g., detail or menu).
				// Cache resources directly from the message so handleRelatedNavigate
				// can find them when navigating to a related resource type.
				if _, alreadyCached := updatedModel.resourceCache[msg.ResourceType]; !alreadyCached {
					updatedModel.resourceCache[msg.ResourceType] = &resourceCacheEntry{
						resources: msg.Resources,
					}
				}
			}
			return updatedModel, cmd
		}
		return updated, cmd
	case messages.IdentityLoadedMsg:
		return m.handleIdentityLoaded(msg)
	case messages.IdentityErrorMsg:
		return m.handleIdentityError(msg)
	case messages.AvailabilityCacheLoadedMsg:
		return m.handleAvailabilityCacheLoaded(msg)
	case messages.AvailabilityPrefetchedMsg:
		return m.handleAvailabilityPrefetched(msg)
	case messages.AvailabilityCheckedMsg:
		return m.handleAvailabilityChecked(msg)
	case messages.EnrichDetailMsg:
		return m.handleEnrichDetail(msg)
	case messages.EnrichDetailResultMsg:
		// Discard stale enrichment results (same convention as relatedGen).
		if msg.Generation != 0 && msg.Generation != m.enrichGen {
			return m, nil
		}
		// Surface enrichment errors as a flash message.
		if msg.Err != nil {
			return m, func() tea.Msg {
				return messages.FlashMsg{Text: "enrich failed: " + msg.Err.Error(), IsError: true}
			}
		}
		return m.updateActiveView(msg)
	case messages.RelatedCheckStartedMsg:
		return m.handleRelatedCheckStarted(msg)
	case messages.RelatedCheckResultMsg:
		// Discard results from a previous check generation (e.g., after Ctrl+R or
		// profile/region switch). relatedGen starts at 1 so the live path never
		// stamps Generation=0 onto results. Generation=0 is therefore the safe
		// sentinel for test/manual injection (always accepted). Any non-zero
		// generation that doesn't match the current relatedGen is stale and dropped.
		if msg.Generation != 0 && msg.Generation != m.relatedGen {
			return m, nil
		}
		// Accumulate in relatedCache so re-entering the same detail skips re-dispatch.
		// Fall back to the active detail view's resource ID when SourceResourceID is unset
		// (e.g., manually-injected test messages or legacy callers).
		sourceID := msg.SourceResourceID
		if sourceID == "" {
			if d, ok := m.activeView().(*views.DetailModel); ok {
				sourceID = d.SourceResource().ID
			}
		}
		if sourceID != "" {
			ck := relatedCacheKey(msg.ResourceType, sourceID)
			existing, _ := m.relatedCache.get(ck)
			m.relatedCache.set(ck, append(existing, msg.Result))
		}
		// Write-back: persist pages fetched on cold miss so the next detail view
		// for any resource type gets a cache hit instead of re-fetching.
		for shortName, entry := range msg.CachedPages {
			if _, exists := m.resourceCache[shortName]; !exists {
				pagination := entry.Pagination
				// Backward compat: callers that set IsTruncated=true but leave Pagination nil
				// (e.g., test fixtures, demo mode) must still have truncation preserved so
				// that buildResourceCacheSnapshot can reconstruct IsTruncated correctly.
				if pagination == nil && entry.IsTruncated {
					pagination = &resource.PaginationMeta{IsTruncated: true}
				}
				m.resourceCache[shortName] = &resourceCacheEntry{
					resources:  entry.Resources,
					pagination: pagination,
				}
			}
		}
		return m.updateActiveView(msg)
	case messages.RelatedNavigateMsg:
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
	m.resourceCache[rt] = &resourceCacheEntry{
		resources:     rl.AllResources(),
		pagination:    rl.PaginationState(),
		filterText:    rl.FilterText(),
		attentionOnly: rl.AttentionOnly(),
		sortColIdx:    sortColIdx,
		sortAsc:       sortAsc,
		cursorPos:     rl.CursorPosition(),
		hScrollOffset: rl.HScrollOffset(),
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
