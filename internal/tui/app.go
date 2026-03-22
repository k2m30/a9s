package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/atotto/clipboard"

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
	configErr      error // non-nil if views.yaml was found but corrupt

	// headerCache avoids re-computing the header string every render when
	// profile, region, version, and right-side content haven't changed.
	headerCache    string
	headerCacheKey string

	demoMode bool
}

// Option configures the root Model.
type Option func(*Model)

// WithDemo enables demo mode with synthetic fixture data.
func WithDemo(enabled bool) Option {
	return func(m *Model) {
		m.demoMode = enabled
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
		// No AWS connection needed. Send ClientsReadyMsg with nil clients
		// so the app transitions to "ready" state without AWS credentials.
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
	case messages.S3EnterBucketMsg:
		return m.handleS3EnterBucket(msg)
	case messages.S3NavigatePrefixMsg:
		return m.handleS3NavigatePrefix(msg)
	case messages.R53EnterZoneMsg:
		return m.handleR53EnterZone(msg)
	case messages.LoadResourcesMsg:
		cmd := m.fetchResources(msg.ResourceType, msg.S3Bucket, msg.S3Prefix, "")
		return m, cmd
	case messages.APIErrorMsg:
		return m.handleAPIError(msg)
	case messages.ResourcesLoadedMsg:
		m.flash.active = false
		return m.updateActiveView(msg)
	}
	return m.updateActiveView(msg)
}

// handleKeyMsg processes all keyboard input: force-quit, input modes, global
// keys, then falls through to the active view.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys handled before delegation
	if key.Matches(msg, m.keys.ForceQuit) {
		return m, tea.Quit
	}

	// Handle input modes
	switch m.inputMode {
	case modeFilter:
		return m.updateFilterMode(msg)
	case modeCommand:
		return m.updateCommandMode(msg)
	}

	// Global keys in normal mode
	if key.Matches(msg, m.keys.Help) {
		// If already on help, let the help view handle it (closes help)
		if _, ok := m.activeView().(*views.HelpModel); ok {
			return m.updateActiveView(msg)
		}
		ctx := m.helpContext()
		help := views.NewHelp(m.keys, ctx)
		help.SetSize(m.innerSize())
		m.pushView(&help)
		return m, nil
	}
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}
	if key.Matches(msg, m.keys.Escape) {
		// If active view has a confirmed filter, clear it first
		if f, ok := m.activeView().(views.Filterable); ok && f.GetFilter() != "" {
			f.SetFilter("")
			return m, nil
		}
		// Otherwise pop view; no-op on main menu (never quit from Esc)
		m.popView()
		return m, nil
	}
	if key.Matches(msg, m.keys.Colon) {
		m.inputMode = modeCommand
		m.cmdInput.Reset()
		m.cmdInput.Focus()
		return m, nil
	}
	if key.Matches(msg, m.keys.Filter) {
		// Only activate filter mode on filterable views
		if _, ok := m.activeView().(views.Filterable); ok {
			m.inputMode = modeFilter
			m.cmdInput.Reset()
			m.cmdInput.Focus()
			return m, nil
		}
		// On static views (detail, yaml, help, reveal), ignore /
		// (help handles it via its own Update which sends PopViewMsg)
	}

	// Copy (c) — context-dependent clipboard copy
	if key.Matches(msg, m.keys.Copy) {
		return m.handleCopy()
	}

	// Refresh (ctrl+r) — re-fetch resources in resource list
	if key.Matches(msg, m.keys.Refresh) {
		return m.handleRefresh()
	}

	// Reveal (x) — reveal secret value (only for secrets)
	if key.Matches(msg, m.keys.Reveal) {
		return m.handleReveal()
	}

	return m.updateActiveView(msg)
}

// handleFlash sets the flash message and schedules its auto-clear.
func (m Model) handleFlash(msg messages.FlashMsg) (tea.Model, tea.Cmd) {
	newGen := m.flash.gen + 1
	m.flash = flashState{text: msg.Text, isError: msg.IsError, active: true, gen: newGen}
	gen := m.flash.gen
	return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return messages.ClearFlashMsg{Gen: gen}
	})
}

// handleClearFlash clears the flash if the generation matches (not stale).
func (m Model) handleClearFlash(msg messages.ClearFlashMsg) (tea.Model, tea.Cmd) {
	if msg.Gen == m.flash.gen {
		m.flash.active = false
	}
	return m, nil
}

// handleClientsReady stores the new AWS clients and optionally triggers a
// pending refresh (after profile/region switch).
func (m Model) handleClientsReady(msg messages.ClientsReadyMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.flash = flashState{text: msg.Err.Error(), isError: true, active: true}
		m.pendingRefresh = false
		return m, nil
	}
	if clients, ok := msg.Clients.(*awsclient.ServiceClients); ok {
		m.clients = clients
	}
	if m.profile == "" && !m.demoMode {
		m.profile = "default"
	}
	if m.region == "" && !m.demoMode {
		configPath := awsclient.DefaultConfigPath()
		m.region = awsclient.GetDefaultRegion(configPath, m.profile)
	}
	if m.pendingRefresh {
		m.pendingRefresh = false
		if rl, ok := m.activeView().(*views.ResourceListModel); ok {
			rt := rl.ResourceType()
			m.flash = flashState{text: "Connected. Refreshing...", active: true}
			cmd := m.fetchResources(rt, "", "", "")
			return m, cmd
		}
	}
	return m, nil
}

// handleProfileSelected switches the AWS profile, pops the profile selector,
// and reconnects.
func (m Model) handleProfileSelected(msg messages.ProfileSelectedMsg) (tea.Model, tea.Cmd) {
	if m.demoMode {
		return m, nil
	}
	m.profile = msg.Profile
	m.region = "" // clear so handleClientsReady resolves the new profile's default region
	m.pendingRefresh = true
	if _, ok := m.activeView().(*views.SelectorModel); ok {
		m.popView()
	}
	flashCmd := func() tea.Msg {
		return messages.FlashMsg{Text: "Switching to " + msg.Profile + "..."}
	}
	return m, tea.Batch(flashCmd, m.connectAWS(msg.Profile, ""))
}

// handleRegionSelected switches the AWS region, pops the region selector,
// and reconnects.
func (m Model) handleRegionSelected(msg messages.RegionSelectedMsg) (tea.Model, tea.Cmd) {
	if m.demoMode {
		return m, nil
	}
	m.region = msg.Region
	m.pendingRefresh = true
	if _, ok := m.activeView().(*views.SelectorModel); ok {
		m.popView()
	}
	flashCmd := func() tea.Msg {
		return messages.FlashMsg{Text: "Switching to " + msg.Region + "..."}
	}
	return m, tea.Batch(flashCmd, m.connectAWS(m.profile, msg.Region))
}

// handleProfilesLoaded pushes the profile selector view onto the stack.
func (m Model) handleProfilesLoaded(msg profilesLoadedMsg) (tea.Model, tea.Cmd) {
	p := views.NewProfile(msg.profiles, m.profile, m.keys)
	p.SetSize(m.innerSize())
	m.pushView(&p)
	return m, nil
}

// handleSecretRevealed pushes the secret reveal view or flashes an error.
func (m Model) handleSecretRevealed(msg messages.SecretRevealedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.flash = flashState{text: "reveal failed: " + msg.Err.Error(), isError: true, active: true}
		return m, nil
	}
	rv := views.NewReveal(msg.SecretName, msg.Value, m.keys)
	rv.SetSize(m.innerSize())
	m.pushView(&rv)
	return m, nil
}

// handleS3EnterBucket pushes an S3 objects list for the given bucket.
func (m Model) handleS3EnterBucket(msg messages.S3EnterBucketMsg) (tea.Model, tea.Cmd) {
	rl := views.NewS3ObjectsList(msg.BucketName, m.viewConfig, m.keys)
	rl.SetSize(m.innerSize())
	rl, initCmd := rl.Init()
	m.pushView(&rl)
	return m, tea.Batch(initCmd, m.fetchResources("s3", msg.BucketName, "", ""))
}

// handleS3NavigatePrefix pushes an S3 objects list for a prefix within a bucket.
func (m Model) handleS3NavigatePrefix(msg messages.S3NavigatePrefixMsg) (tea.Model, tea.Cmd) {
	rl := views.NewS3ObjectsList(msg.Bucket, m.viewConfig, m.keys, msg.Prefix)
	rl.SetSize(m.innerSize())
	rl, initCmd := rl.Init()
	m.pushView(&rl)
	return m, tea.Batch(initCmd, m.fetchResources("s3", msg.Bucket, msg.Prefix, ""))
}

// handleR53EnterZone pushes a DNS records list for the given hosted zone.
func (m Model) handleR53EnterZone(msg messages.R53EnterZoneMsg) (tea.Model, tea.Cmd) {
	rl := views.NewR53RecordsList(msg.ZoneId, msg.ZoneName, m.viewConfig, m.keys)
	rl.SetSize(m.innerSize())
	rl, initCmd := rl.Init()
	m.pushView(&rl)
	return m, tea.Batch(initCmd, m.fetchResources("r53_records", "", "", msg.ZoneId))
}

// handleAPIError shows a flash error and clears loading state on the resource list.
func (m Model) handleAPIError(msg messages.APIErrorMsg) (tea.Model, tea.Cmd) {
	code, message, _ := awsclient.ClassifyAWSError(msg.Err)
	var flashText string
	if code != "" && code != "Unknown" {
		flashText = fmt.Sprintf("[%s] %s", code, message)
	} else {
		flashText = msg.Err.Error()
	}
	m.flash = flashState{text: flashText, isError: true, active: true}
	if rl, ok := m.activeView().(*views.ResourceListModel); ok {
		rl.ClearLoading()
	}
	return m, nil
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
	cacheKey := headerProfile + ":" + headerRegion + ":" + Version + ":" + rightContent + ":" + fmt.Sprintf("%d", m.width)
	header := m.headerCache
	if cacheKey != m.headerCacheKey {
		header = layout.RenderHeader(headerProfile, headerRegion, Version, m.width, rightContent)
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

// handleNavigate pushes the appropriate view onto the stack.
func (m Model) handleNavigate(msg messages.NavigateMsg) (tea.Model, tea.Cmd) {
	switch msg.Target {
	case messages.TargetResourceList:
		rt := resource.FindResourceType(msg.ResourceType)
		if rt == nil {
			return m, func() tea.Msg {
				return messages.FlashMsg{Text: fmt.Sprintf("unknown resource type: %s", msg.ResourceType), IsError: true}
			}
		}
		rl := views.NewResourceList(*rt, m.viewConfig, m.keys)
		rl.SetSize(m.innerSize())
		rl, initCmd := rl.Init()
		m.pushView(&rl)
		return m, tea.Batch(initCmd, m.fetchResources(msg.ResourceType, "", "", ""))

	case messages.TargetDetail:
		if msg.Resource == nil {
			return m, nil
		}
		resType := msg.ResourceType
		if resType == "" {
			if rl, ok := m.activeView().(*views.ResourceListModel); ok {
				resType = rl.ResourceType()
			}
		}
		d := views.NewDetail(*msg.Resource, resType, m.viewConfig, m.keys)
		d.SetSize(m.innerSize())
		m.pushView(&d)
		return m, nil

	case messages.TargetYAML:
		if msg.Resource == nil {
			return m, nil
		}
		y := views.NewYAML(*msg.Resource, m.keys)
		y.SetSize(m.innerSize())
		m.pushView(&y)
		return m, nil

	case messages.TargetHelp:
		ctx := m.helpContext()
		h := views.NewHelp(m.keys, ctx)
		h.SetSize(m.innerSize())
		m.pushView(&h)
		return m, nil

	case messages.TargetProfile:
		if m.demoMode {
			return m, func() tea.Msg {
				return messages.FlashMsg{Text: "Profile switching disabled in demo mode", IsError: true}
			}
		}
		cmd := m.fetchProfiles()
		return m, cmd

	case messages.TargetRegion:
		if m.demoMode {
			return m, func() tea.Msg {
				return messages.FlashMsg{Text: "Region switching disabled in demo mode", IsError: true}
			}
		}
		regions := awsclient.AllRegions()
		regionCodes := make([]string, len(regions))
		for i, r := range regions {
			regionCodes[i] = r.Code
		}
		rg := views.NewRegion(regionCodes, m.region, m.keys)
		rg.SetSize(m.innerSize())
		m.pushView(&rg)
		return m, nil

	case messages.TargetReveal:
		if msg.Resource == nil {
			return m, nil
		}
		cmd := m.fetchSecretValue(msg.Resource.ID)
		return m, cmd
	}
	return m, nil
}

// updateActiveView delegates a message to the active view and merges the result.
// This is the one remaining type switch — unavoidable because each view's Update
// returns its own concrete type (standard Bubble Tea pattern).
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
	}
	return m, nil
}

// updateFilterMode handles keys while in filter input mode.
func (m Model) updateFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Escape) {
		m.inputMode = modeNormal
		m.cmdInput.Blur()
		m.applyFilterToActiveView("")
		return m, nil
	}
	if key.Matches(msg, m.keys.Enter) {
		m.inputMode = modeNormal
		m.cmdInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.cmdInput, cmd = m.cmdInput.Update(msg)
	m.applyFilterToActiveView(m.cmdInput.Value())
	return m, cmd
}

// applyFilterToActiveView applies the given filter text to whichever navigable view is active.
func (m *Model) applyFilterToActiveView(text string) {
	if f, ok := m.activeView().(views.Filterable); ok {
		f.SetFilter(text)
	}
}

// updateCommandMode handles keys while in command input mode.
func (m Model) updateCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Escape) {
		m.inputMode = modeNormal
		m.cmdInput.Blur()
		return m, nil
	}
	if key.Matches(msg, m.keys.Enter) {
		m.inputMode = modeNormal
		cmd := m.cmdInput.Value()
		m.cmdInput.Blur()
		return m.executeCommand(cmd)
	}

	var teaCmd tea.Cmd
	m.cmdInput, teaCmd = m.cmdInput.Update(msg)
	return m, teaCmd
}

// executeCommand dispatches a colon-command string.
func (m Model) executeCommand(cmd string) (tea.Model, tea.Cmd) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return m, nil
	}

	switch cmd {
	case "q", "quit":
		return m, tea.Quit
	case "ctx", "profile":
		if m.demoMode {
			return m, func() tea.Msg {
				return messages.FlashMsg{Text: "Profile switching disabled in demo mode", IsError: true}
			}
		}
		return m, func() tea.Msg {
			return messages.NavigateMsg{Target: messages.TargetProfile}
		}
	case "region":
		if m.demoMode {
			return m, func() tea.Msg {
				return messages.FlashMsg{Text: "Region switching disabled in demo mode", IsError: true}
			}
		}
		return m, func() tea.Msg {
			return messages.NavigateMsg{Target: messages.TargetRegion}
		}
	case "help":
		return m, func() tea.Msg {
			return messages.NavigateMsg{Target: messages.TargetHelp}
		}
	}

	rt := resource.FindResourceType(cmd)
	if rt != nil {
		return m, func() tea.Msg {
			return messages.NavigateMsg{
				Target:       messages.TargetResourceList,
				ResourceType: rt.ShortName,
			}
		}
	}

	return m, func() tea.Msg {
		return messages.FlashMsg{
			Text:    fmt.Sprintf("unknown command: %s", cmd),
			IsError: true,
		}
	}
}

// fetchResources returns a tea.Cmd that calls the appropriate AWS fetcher.
// Uses the resource registry for standard fetchers; S3 objects and R53 records
// are special cases because they require additional parameters.
func (m *Model) fetchResources(resourceType, s3Bucket, s3Prefix, r53ZoneId string) tea.Cmd {
	if m.demoMode {
		return m.fetchDemoResources(resourceType, s3Bucket, r53ZoneId)
	}
	clients := m.clients
	return func() tea.Msg {
		if clients == nil {
			return messages.APIErrorMsg{
				ResourceType: resourceType,
				Err:          fmt.Errorf("AWS clients not initialized"),
			}
		}

		ctx := context.Background()
		var resources []resource.Resource
		var err error

		// S3 objects are a special case: they need bucket/prefix params
		// and don't map to a single registry entry.
		switch {
		case resourceType == "s3" && s3Bucket != "":
			resources, err = awsclient.FetchS3Objects(ctx, clients.S3, s3Bucket, s3Prefix)
		case resourceType == "r53_records" && r53ZoneId != "":
			resources, err = awsclient.FetchR53Records(ctx, clients.Route53, r53ZoneId)
		default:
			fetcher := resource.GetFetcher(resourceType)
			if fetcher == nil {
				return messages.APIErrorMsg{
					ResourceType: resourceType,
					Err:          fmt.Errorf("unsupported resource type: %s", resourceType),
				}
			}
			resources, err = fetcher(ctx, clients)
		}

		if err != nil {
			return messages.APIErrorMsg{ResourceType: resourceType, Err: err}
		}
		return messages.ResourcesLoadedMsg{ResourceType: resourceType, Resources: resources}
	}
}

// fetchDemoResources returns a tea.Cmd that provides synthetic fixture data
// instead of calling AWS APIs. Maintains the async message contract.
func (m *Model) fetchDemoResources(resourceType, s3Bucket, r53ZoneId string) tea.Cmd {
	return func() tea.Msg {
		// S3 object drill-down
		if s3Bucket != "" {
			resources, ok := demo.GetS3Objects(s3Bucket, "")
			if !ok {
				resources = nil
			}
			return messages.ResourcesLoadedMsg{
				ResourceType: resourceType,
				Resources:    resources,
			}
		}
		// R53 records drill-down
		if r53ZoneId != "" {
			resources, ok := demo.GetR53Records(r53ZoneId)
			if !ok {
				resources = nil
			}
			return messages.ResourcesLoadedMsg{
				ResourceType: resourceType,
				Resources:    resources,
			}
		}
		// Standard resource fetch — resolve alias to canonical short name
		canonicalType := resourceType
		rt := resource.FindResourceType(resourceType)
		if rt != nil {
			canonicalType = rt.ShortName
		}
		resources, _ := demo.GetResources(canonicalType)
		return messages.ResourcesLoadedMsg{
			ResourceType: resourceType,
			Resources:    resources,
		}
	}
}

func (m *Model) fetchProfiles() tea.Cmd {
	return func() tea.Msg {
		configPath := awsclient.DefaultConfigPath()
		profiles, err := awsclient.ListProfiles(configPath)
		if err != nil {
			return messages.FlashMsg{Text: "failed to list profiles: " + err.Error(), IsError: true}
		}
		if len(profiles) == 0 {
			return messages.FlashMsg{Text: "no AWS profiles found", IsError: true}
		}
		return profilesLoadedMsg{profiles: profiles}
	}
}

type profilesLoadedMsg struct {
	profiles []string
}

func (m *Model) fetchSecretValue(secretName string) tea.Cmd {
	clients := m.clients
	return func() tea.Msg {
		if clients == nil {
			return messages.FlashMsg{Text: "AWS clients not initialized", IsError: true}
		}
		ctx := context.Background()
		value, err := awsclient.RevealSecret(ctx, clients.SecretsManager, secretName)
		if err != nil {
			return messages.FlashMsg{Text: "failed to reveal secret: " + err.Error(), IsError: true}
		}
		return messages.SecretRevealedMsg{SecretName: secretName, Value: value}
	}
}

func (m *Model) connectAWS(profile, region string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := awsclient.NewAWSSession(profile, region)
		if err != nil {
			return messages.ClientsReadyMsg{Err: err}
		}
		clients := awsclient.CreateServiceClients(cfg)
		return messages.ClientsReadyMsg{Clients: clients}
	}
}

// handleCopy performs context-dependent clipboard copy as a tea.Cmd.
// Each view implements CopyContent() to provide its own content and label.
func (m Model) handleCopy() (tea.Model, tea.Cmd) {
	content, label := m.activeView().CopyContent()
	if content == "" {
		return m, nil
	}
	return m, copyToClipboard(content, label)
}

// handleRefresh re-fetches resources when on a resource list view.
func (m Model) handleRefresh() (tea.Model, tea.Cmd) {
	rl, ok := m.activeView().(*views.ResourceListModel)
	if !ok {
		return m, nil
	}
	rt := rl.ResourceType()
	s3Bucket := rl.S3Bucket()
	s3Prefix := rl.S3Prefix()
	r53ZoneId := rl.R53ZoneId()
	if s3Bucket != "" && rt == "s3_objects" {
		rt = "s3"
	}
	m.flash = flashState{text: "Refreshing...", isError: false, active: true}
	cmd := m.fetchResources(rt, s3Bucket, s3Prefix, r53ZoneId)
	return m, cmd
}

// handleReveal fetches a secret value (only for secrets resource type).
func (m Model) handleReveal() (tea.Model, tea.Cmd) {
	if m.demoMode {
		return m, func() tea.Msg {
			return messages.FlashMsg{Text: "Secret reveal disabled in demo mode", IsError: true}
		}
	}
	rl, ok := m.activeView().(*views.ResourceListModel)
	if !ok {
		return m, nil
	}
	if rl.ResourceType() != "secrets" {
		return m, nil
	}
	r := rl.SelectedResource()
	if r == nil {
		return m, nil
	}
	cmd := m.fetchSecretValue(r.ID)
	return m, cmd
}

func copyToClipboard(content, successLabel string) tea.Cmd {
	return func() tea.Msg {
		err := clipboard.WriteAll(content)
		if err != nil {
			return messages.FlashMsg{Text: fmt.Sprintf("Copy failed: %v", err), IsError: true}
		}
		return messages.FlashMsg{Text: successLabel, IsError: false}
	}
}

// helpContext determines the HelpContext from the current active view.
// Each view implements GetHelpContext() to provide its own context.
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
		if m.flash.isError {
			return styles.FlashError.Render(m.flash.text)
		}
		return styles.FlashSuccess.Render(m.flash.text)
	}
	if rv, ok := m.activeView().(*views.RevealModel); ok {
		return rv.HeaderWarning()
	}
	return styles.DimText.Render("? for help")
}
