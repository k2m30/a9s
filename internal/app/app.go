package app

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/atotto/clipboard"

	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/internal/navigation"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/styles"
	"github.com/k2m30/a9s/internal/ui"
	"github.com/k2m30/a9s/internal/views"
)

// ViewType represents the current application view.
type ViewType int

const (
	MainMenuView ViewType = iota
	ResourceListView
	DetailView
	JSONView
	RevealView
	ProfileSelectView
	RegionSelectView
)

const version = "0.1.0"

// AppState is the root Bubble Tea model for the a9s application.
type AppState struct {
	// Current view
	CurrentView ViewType

	// AWS context
	ActiveProfile string
	ActiveRegion  string

	// Navigation
	Breadcrumbs []string
	History     navigation.NavigationStack

	// UI state
	StatusMessage string
	StatusIsError bool
	Loading       bool
	Filter        string

	// Data
	CurrentResourceType string
	Resources           []resource.Resource
	FilteredResources   []resource.Resource
	SelectedIndex       int

	// AWS clients (can be nil until connected)
	Clients *awsclient.ServiceClients

	// Key map
	Keys KeyMap

	// Terminal dimensions
	Width  int
	Height int

	// Command mode
	CommandMode bool
	CommandText string

	// Filter mode
	FilterMode bool

	// Help overlay
	ShowHelp bool

	// S3 browsing state
	S3Bucket string
	S3Prefix string

	// Horizontal scroll offset for wide tables
	HScrollOffset int

	// Profile/Region selector models
	ProfileSelector views.ProfileSelectModel
	RegionSelector  views.RegionSelectModel

	// Detail/JSON/Reveal view models
	Detail   views.DetailModel
	JSONData views.JSONViewModel
	Reveal   views.RevealModel
}

// NewAppState creates a new AppState with sensible defaults.
// It reads the region from ~/.aws/config if not provided via flag or env var.
func NewAppState(profile, region string) AppState {
	return NewAppStateWithConfig(profile, region, awsclient.DefaultConfigPath())
}

// NewAppStateWithConfig creates a new AppState using a specific AWS config path.
// Region resolution order: explicit flag > AWS_REGION env > AWS_DEFAULT_REGION env > config file > us-east-1.
func NewAppStateWithConfig(profile, region, configPath string) AppState {
	if profile == "" {
		profile = os.Getenv("AWS_PROFILE")
		if profile == "" {
			profile = "default"
		}
	}
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = awsclient.GetDefaultRegion(configPath, profile)
	}

	return AppState{
		CurrentView:   MainMenuView,
		ActiveProfile: profile,
		ActiveRegion:  region,
		Breadcrumbs:   []string{"main"},
		Keys:          DefaultKeyMap(),
	}
}

// InitConnectMsg is sent by Init to trigger AWS client creation.
type InitConnectMsg struct {
	Profile string
	Region  string
}

// Init implements tea.Model. It sets up the initial application state.
// It sends an InitConnectMsg to trigger AWS client creation in Update.
func (m AppState) Init() tea.Cmd {
	profile := m.ActiveProfile
	region := m.ActiveRegion
	return func() tea.Msg {
		return InitConnectMsg{Profile: profile, Region: region}
	}
}

// Update implements tea.Model. It processes incoming messages and returns
// the updated model and any commands to execute.
func (m AppState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case InitConnectMsg:
		cfg, err := awsclient.NewAWSSession(msg.Profile, msg.Region)
		if err != nil {
			m.StatusMessage = fmt.Sprintf("AWS config error: %v. Try: aws configure or aws sso login", err)
			m.StatusIsError = true
			return m, nil
		}
		m.Clients = awsclient.CreateServiceClients(cfg)
		m.StatusMessage = fmt.Sprintf("Connected: %s / %s", msg.Profile, msg.Region)
		m.StatusIsError = false
		return m, nil

	case ResourcesLoadedMsg:
		// Discard stale responses from a previously-selected resource type
		if msg.ResourceType != m.CurrentResourceType {
			return m, nil
		}
		m.Resources = msg.Resources
		m.Loading = false
		m.applyFilter()
		m.updateBreadcrumbs()
		if len(msg.Resources) == 0 {
			m.StatusMessage = fmt.Sprintf("No %s found in %s", msg.ResourceType, m.ActiveRegion)
			m.StatusIsError = false
		}
		return m, nil

	case APIErrorMsg:
		errStr := msg.Err.Error()
		m.StatusIsError = true
		m.Loading = false
		if strings.Contains(errStr, "ExpiredToken") || strings.Contains(errStr, "ExpiredTokenException") {
			m.StatusMessage = fmt.Sprintf("Error fetching %s: credentials expired. Run: aws sso login", msg.ResourceType)
		} else {
			m.StatusMessage = fmt.Sprintf("Error fetching %s: %v", msg.ResourceType, msg.Err)
		}
		return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg {
			return ClearErrorMsg{}
		})

	case ClearErrorMsg:
		if m.StatusIsError {
			m.StatusMessage = ""
			m.StatusIsError = false
		}
		return m, nil

	case ProfileSwitchedMsg:
		m.ActiveProfile = msg.Profile
		m.ActiveRegion = msg.Region
		m.Clients = nil
		m = m.recreateClients()
		return m, nil

	case RegionSwitchedMsg:
		m.ActiveRegion = msg.Region
		m.Clients = nil
		m = m.recreateClients()
		return m, nil

	case StatusMsg:
		m.StatusMessage = msg.Text
		m.StatusIsError = msg.IsError
		return m, nil

	case SecretRevealedMsg:
		m.Loading = false
		if msg.Err != nil {
			m.StatusMessage = fmt.Sprintf("Error revealing secret: %v", msg.Err)
			m.StatusIsError = true
			return m, nil
		}
		m.Reveal = views.NewRevealView("Secret: "+msg.SecretName, msg.Value)
		m.Reveal.Width = m.Width
		m.Reveal.Height = m.Height
		m.CurrentView = RevealView
		m.updateBreadcrumbs()
		return m, nil

	case tea.KeyPressMsg:
		// Force quit always works
		if key.Matches(msg, m.Keys.ForceQuit) {
			return m, tea.Quit
		}

		if m.CommandMode {
			return m.handleCommandMode(msg)
		}
		if m.FilterMode {
			return m.handleFilterMode(msg)
		}
		return m.handleNormalMode(msg)
	}

	return m, nil
}

// handleCommandMode processes key events when in command mode (after typing ':').
func (m AppState) handleCommandMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		cmd := m.CommandText
		m.CommandMode = false
		m.CommandText = ""
		return m.executeCommand(cmd)
	case "esc", "escape":
		m.CommandMode = false
		m.CommandText = ""
		return m, nil
	case "backspace":
		if len(m.CommandText) > 0 {
			m.CommandText = m.CommandText[:len(m.CommandText)-1]
		}
		if len(m.CommandText) == 0 {
			m.CommandMode = false
		}
		return m, nil
	default:
		if msg.String() != "" && len(msg.String()) == 1 {
			m.CommandText += msg.String()
		}
		return m, nil
	}
}

// handleFilterMode processes key events when in filter mode (after typing '/').
func (m AppState) handleFilterMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.FilterMode = false
		return m, nil
	case "esc", "escape":
		m.FilterMode = false
		m.Filter = ""
		m.applyFilter()
		return m, nil
	case "backspace":
		if len(m.Filter) > 0 {
			m.Filter = m.Filter[:len(m.Filter)-1]
		}
		if len(m.Filter) == 0 {
			m.FilterMode = false
		}
		m.applyFilter()
		return m, nil
	default:
		if msg.String() != "" && len(msg.String()) == 1 {
			m.Filter += msg.String()
		}
		m.applyFilter()
		return m, nil
	}
}

// handleNormalMode processes key events in the default (normal) mode.
func (m AppState) handleNormalMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Toggle help
	if key.Matches(msg, m.Keys.Help) {
		m.ShowHelp = !m.ShowHelp
		return m, nil
	}

	// Close help if showing
	if m.ShowHelp {
		m.ShowHelp = false
		return m, nil
	}

	// Enter command mode
	if key.Matches(msg, m.Keys.Colon) {
		m.CommandMode = true
		m.CommandText = ""
		return m, nil
	}

	// Enter filter mode (only for resource list)
	if key.Matches(msg, m.Keys.Filter) && m.CurrentView == ResourceListView {
		m.FilterMode = true
		m.Filter = ""
		return m, nil
	}

	// Escape: go back
	if key.Matches(msg, m.Keys.Escape) {
		return m.goBack()
	}

	// Quit (only from main menu)
	if key.Matches(msg, m.Keys.Quit) {
		if m.CurrentView == MainMenuView {
			return m, tea.Quit
		}
		return m.goBack()
	}

	// Refresh (Ctrl-R): reload current resource list
	if key.Matches(msg, m.Keys.Refresh) && m.CurrentView != MainMenuView {
		if m.CurrentResourceType != "" {
			m.Loading = true
			return m, m.fetchResources()
		}
		return m, nil
	}

	// History navigation
	if key.Matches(msg, m.Keys.HistoryBack) {
		return m.historyBack()
	}
	if key.Matches(msg, m.Keys.HistoryForward) {
		return m.historyForward()
	}

	// View-specific key handling
	switch m.CurrentView {
	case MainMenuView:
		return m.handleMainMenuKeys(msg)
	case ResourceListView:
		return m.handleResourceListKeys(msg)
	case DetailView:
		return m.handleDetailKeys(msg)
	case JSONView:
		return m.handleJSONViewKeys(msg)
	case RevealView:
		return m.handleRevealKeys(msg)
	case ProfileSelectView:
		return m.handleProfileSelectKeys(msg)
	case RegionSelectView:
		return m.handleRegionSelectKeys(msg)
	default:
		return m, nil
	}
}

// handleMainMenuKeys handles keys specific to the main menu view.
func (m AppState) handleMainMenuKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) == 0 {
		return m, nil
	}

	if key.Matches(msg, m.Keys.Up) {
		if m.SelectedIndex > 0 {
			m.SelectedIndex--
		}
		return m, nil
	}
	if key.Matches(msg, m.Keys.Down) {
		if m.SelectedIndex < len(allTypes)-1 {
			m.SelectedIndex++
		}
		return m, nil
	}
	if key.Matches(msg, m.Keys.Top) {
		m.SelectedIndex = 0
		return m, nil
	}
	if key.Matches(msg, m.Keys.Bottom) {
		m.SelectedIndex = len(allTypes) - 1
		return m, nil
	}
	if key.Matches(msg, m.Keys.Enter) {
		rt := allTypes[m.SelectedIndex]
		m.pushCurrentView()
		m.CurrentResourceType = rt.ShortName
		m.CurrentView = ResourceListView
		m.Breadcrumbs = []string{"main", rt.Name}
		m.SelectedIndex = 0
		m.Filter = ""
		m.FilteredResources = nil
		m.Loading = true
		return m, m.fetchResources()
	}

	return m, nil
}

// handleResourceListKeys handles keys specific to the resource list view.
func (m AppState) handleResourceListKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	display := m.displayResources()
	listLen := len(display)

	if key.Matches(msg, m.Keys.Up) {
		if m.SelectedIndex > 0 {
			m.SelectedIndex--
		}
		return m, nil
	}
	if key.Matches(msg, m.Keys.Down) {
		if m.SelectedIndex < listLen-1 {
			m.SelectedIndex++
		}
		return m, nil
	}
	if key.Matches(msg, m.Keys.Top) {
		m.SelectedIndex = 0
		return m, nil
	}
	if key.Matches(msg, m.Keys.Bottom) {
		if listLen > 0 {
			m.SelectedIndex = listLen - 1
		}
		return m, nil
	}
	// Enter key: for S3 buckets, drill down into objects; for all others, open describe view
	if key.Matches(msg, m.Keys.Enter) {
		if listLen > 0 && m.SelectedIndex < listLen {
			selected := display[m.SelectedIndex]

			// S3 special case: drill into buckets/folders
			if m.CurrentResourceType == "s3" {
				if m.S3Bucket == "" {
					// We're viewing bucket list, drill into this bucket
					m.pushCurrentView()
					m.S3Bucket = selected.ID
					m.S3Prefix = ""
					m.SelectedIndex = 0
					m.Loading = true
					return m, m.fetchS3Objects()
				}
				// We're already inside a bucket, check if it's a folder
				if strings.HasSuffix(selected.ID, "/") {
					m.pushCurrentView()
					m.S3Prefix = selected.ID
					m.SelectedIndex = 0
					m.Loading = true
					return m, m.fetchS3Objects()
				}
				// S3 file: fall through to describe below
			}

			// Default: open describe view (same as 'd')
			if selected.DetailData != nil && len(selected.DetailData) > 0 {
				m.pushCurrentView()
				m.Detail = views.NewDetailModel(selected.Name+" - Detail", selected.DetailData)
				m.Detail.Width = m.Width
				m.Detail.Height = m.Height
				m.CurrentView = DetailView
				m.updateBreadcrumbs()
			}
		}
		return m, nil
	}

	// Describe (d)
	if key.Matches(msg, m.Keys.Describe) {
		if listLen > 0 && m.SelectedIndex < listLen {
			selected := display[m.SelectedIndex]
			if selected.DetailData != nil && len(selected.DetailData) > 0 {
				m.pushCurrentView()
				m.Detail = views.NewDetailModel(selected.Name+" - Detail", selected.DetailData)
				m.Detail.Width = m.Width
				m.Detail.Height = m.Height
				m.CurrentView = DetailView
				m.updateBreadcrumbs()
			} else {
				m.StatusMessage = "No detail data available for this resource"
				m.StatusIsError = false
			}
		}
		return m, nil
	}

	// JSON view (y)
	if key.Matches(msg, m.Keys.JSON) {
		if listLen > 0 && m.SelectedIndex < listLen {
			selected := display[m.SelectedIndex]
			if selected.RawJSON != "" {
				m.pushCurrentView()
				m.JSONData = views.NewJSONView(selected.Name+" - JSON", selected.RawJSON)
				m.JSONData.Width = m.Width
				m.JSONData.Height = m.Height
				m.CurrentView = JSONView
				m.updateBreadcrumbs()
			} else {
				m.StatusMessage = "No JSON data available for this resource"
				m.StatusIsError = false
			}
		}
		return m, nil
	}

	// Reveal secret (x) - only for secrets
	if key.Matches(msg, m.Keys.Reveal) {
		if m.CurrentResourceType == "secrets" && listLen > 0 && m.SelectedIndex < listLen {
			selected := display[m.SelectedIndex]
			if m.Clients == nil {
				m.StatusMessage = "No AWS connection; use :ctx to set profile"
				m.StatusIsError = true
				return m, nil
			}
			m.pushCurrentView()
			m.Loading = true
			secretName := selected.ID
			client := m.Clients.SecretsManager
			return m, func() tea.Msg {
				val, err := awsclient.RevealSecret(context.Background(), client, secretName)
				return SecretRevealedMsg{SecretName: secretName, Value: val, Err: err}
			}
		}
		return m, nil
	}

	// Copy ID (c)
	if key.Matches(msg, m.Keys.Copy) {
		if listLen > 0 && m.SelectedIndex < listLen {
			selected := display[m.SelectedIndex]
			err := clipboard.WriteAll(selected.ID)
			if err != nil {
				m.StatusMessage = fmt.Sprintf("Copy failed: %v", err)
				m.StatusIsError = true
			} else {
				m.StatusMessage = fmt.Sprintf("Copied: %s", selected.ID)
				m.StatusIsError = false
			}
		}
		return m, nil
	}

	// Horizontal scroll (h/l or left/right)
	if key.Matches(msg, m.Keys.ScrollLeft) {
		if m.HScrollOffset > 0 {
			m.HScrollOffset -= 4
			if m.HScrollOffset < 0 {
				m.HScrollOffset = 0
			}
		}
		return m, nil
	}
	if key.Matches(msg, m.Keys.ScrollRight) {
		m.HScrollOffset += 4
		return m, nil
	}

	// Sort by name (N)
	if key.Matches(msg, m.Keys.SortByName) {
		m.sortResources("name")
		m.SelectedIndex = 0
		m.StatusMessage = "Sorted by name"
		m.StatusIsError = false
		return m, nil
	}

	// Sort by status (S)
	if key.Matches(msg, m.Keys.SortByStatus) {
		m.sortResources("status")
		m.SelectedIndex = 0
		m.StatusMessage = "Sorted by status"
		m.StatusIsError = false
		return m, nil
	}

	// Sort by age (A)
	if key.Matches(msg, m.Keys.SortByAge) {
		m.sortResources("age")
		m.SelectedIndex = 0
		m.StatusMessage = "Sorted by age"
		m.StatusIsError = false
		return m, nil
	}

	return m, nil
}

// handleDetailKeys handles keys in the detail view.
func (m AppState) handleDetailKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.Keys.Up) {
		m.Detail.ScrollUp()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Down) {
		m.Detail.ScrollDown()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Top) {
		m.Detail.GoTop()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Bottom) {
		m.Detail.GoBottom()
		return m, nil
	}
	return m, nil
}

// handleJSONViewKeys handles keys in the JSON view.
func (m AppState) handleJSONViewKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.Keys.Up) {
		m.JSONData.ScrollUp()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Down) {
		m.JSONData.ScrollDown()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Top) {
		m.JSONData.GoTop()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Bottom) {
		m.JSONData.GoBottom()
		return m, nil
	}
	return m, nil
}

// handleRevealKeys handles keys in the reveal view.
func (m AppState) handleRevealKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.Keys.Up) {
		m.Reveal.ScrollUp()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Down) {
		m.Reveal.ScrollDown()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Top) {
		m.Reveal.GoTop()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Bottom) {
		m.Reveal.GoBottom()
		return m, nil
	}
	// Copy secret content (c)
	if key.Matches(msg, m.Keys.Copy) {
		err := clipboard.WriteAll(m.Reveal.Content)
		if err != nil {
			m.StatusMessage = fmt.Sprintf("Copy failed: %v", err)
			m.StatusIsError = true
		} else {
			m.StatusMessage = "Secret copied to clipboard"
			m.StatusIsError = false
		}
		return m, nil
	}
	return m, nil
}

// handleProfileSelectKeys handles keys in the profile selector view.
func (m AppState) handleProfileSelectKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.Keys.Up) {
		m.ProfileSelector.MoveUp()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Down) {
		m.ProfileSelector.MoveDown()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Top) {
		m.ProfileSelector.GoTop()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Bottom) {
		m.ProfileSelector.GoBottom()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Enter) {
		selectedProfile := m.ProfileSelector.SelectedProfile()
		if selectedProfile != "" {
			region := awsclient.GetDefaultRegion(awsclient.DefaultConfigPath(), selectedProfile)
			m.CurrentView = MainMenuView
			m.Breadcrumbs = []string{"main"}
			return m, func() tea.Msg {
				return ProfileSwitchedMsg{Profile: selectedProfile, Region: region}
			}
		}
		return m, nil
	}
	return m, nil
}

// handleRegionSelectKeys handles keys in the region selector view.
func (m AppState) handleRegionSelectKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.Keys.Up) {
		m.RegionSelector.MoveUp()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Down) {
		m.RegionSelector.MoveDown()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Top) {
		m.RegionSelector.GoTop()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Bottom) {
		m.RegionSelector.GoBottom()
		return m, nil
	}
	if key.Matches(msg, m.Keys.Enter) {
		selectedRegion := m.RegionSelector.SelectedRegion()
		if selectedRegion.Code != "" {
			m.CurrentView = MainMenuView
			m.Breadcrumbs = []string{"main"}
			return m, func() tea.Msg {
				return RegionSwitchedMsg{Region: selectedRegion.Code}
			}
		}
		return m, nil
	}
	return m, nil
}

// goBack returns to the previous view.
func (m AppState) goBack() (tea.Model, tea.Cmd) {
	state, ok := m.History.Pop()
	if ok {
		prevS3Bucket := m.S3Bucket
		prevS3Prefix := m.S3Prefix

		m.CurrentView = ViewType(state.ViewType)
		m.CurrentResourceType = state.ResourceType
		m.SelectedIndex = state.CursorPos
		m.Filter = state.Filter
		m.S3Bucket = state.S3Bucket
		m.S3Prefix = state.S3Prefix
		if m.Filter != "" {
			m.FilteredResources = views.FilterResources(m.Filter, m.Resources)
		} else {
			m.FilteredResources = nil
		}
		m.updateBreadcrumbs()

		// If returning to a ResourceListView and the S3 context changed,
		// re-fetch data because Resources still holds stale data from
		// the child view.
		if m.CurrentView == ResourceListView &&
			m.CurrentResourceType == "s3" &&
			(m.S3Bucket != prevS3Bucket || m.S3Prefix != prevS3Prefix) {
			m.Loading = true
			m.Resources = nil
			m.FilteredResources = nil
			if m.S3Bucket == "" {
				return m, m.fetchResources()
			}
			return m, m.fetchS3Objects()
		}

		return m, nil
	}
	// Fallback: go to main menu
	if m.CurrentView != MainMenuView {
		m.CurrentView = MainMenuView
		m.Breadcrumbs = []string{"main"}
		m.SelectedIndex = 0
		m.S3Bucket = ""
		m.S3Prefix = ""
		m.Filter = ""
		m.FilteredResources = nil
		return m, nil
	}
	return m, nil
}

// historyBack navigates backward in the history stack.
func (m AppState) historyBack() (tea.Model, tea.Cmd) {
	return m.goBack()
}

// historyForward navigates forward in the history stack.
func (m AppState) historyForward() (tea.Model, tea.Cmd) {
	state, ok := m.History.Forward()
	if ok {
		m.CurrentView = ViewType(state.ViewType)
		m.CurrentResourceType = state.ResourceType
		m.SelectedIndex = state.CursorPos
		m.Filter = state.Filter
		m.S3Prefix = state.S3Prefix
		if m.Filter != "" {
			m.FilteredResources = views.FilterResources(m.Filter, m.Resources)
		} else {
			m.FilteredResources = nil
		}
		m.updateBreadcrumbs()
	}
	return m, nil
}

// pushCurrentView saves the current view state to the history stack.
func (m *AppState) pushCurrentView() {
	m.History.Push(navigation.ViewState{
		ViewType:     navigation.ViewType(m.CurrentView),
		ResourceType: m.CurrentResourceType,
		CursorPos:    m.SelectedIndex,
		Filter:       m.Filter,
		S3Bucket:     m.S3Bucket,
		S3Prefix:     m.S3Prefix,
	})
}

// updateBreadcrumbs rebuilds breadcrumbs based on current view state.
func (m *AppState) updateBreadcrumbs() {
	switch m.CurrentView {
	case MainMenuView:
		m.Breadcrumbs = []string{"main"}
	case ResourceListView:
		rt := resource.FindResourceType(m.CurrentResourceType)
		name := m.CurrentResourceType
		if rt != nil {
			name = rt.Name
		}
		crumbs := []string{"main", name}
		if m.S3Bucket != "" {
			crumbs = append(crumbs, m.S3Bucket)
			if m.S3Prefix != "" {
				crumbs = append(crumbs, m.S3Prefix)
			}
		}
		m.Breadcrumbs = crumbs
	case DetailView:
		rt := resource.FindResourceType(m.CurrentResourceType)
		name := m.CurrentResourceType
		if rt != nil {
			name = rt.Name
		}
		m.Breadcrumbs = []string{"main", name, "detail"}
	case JSONView:
		rt := resource.FindResourceType(m.CurrentResourceType)
		name := m.CurrentResourceType
		if rt != nil {
			name = rt.Name
		}
		m.Breadcrumbs = []string{"main", name, "json"}
	case RevealView:
		rt := resource.FindResourceType(m.CurrentResourceType)
		name := m.CurrentResourceType
		if rt != nil {
			name = rt.Name
		}
		m.Breadcrumbs = []string{"main", name, "reveal"}
	default:
		m.Breadcrumbs = []string{"main"}
	}
}

// recreateClients creates new AWS service clients from the current profile/region.
func (m AppState) recreateClients() AppState {
	cfg, err := awsclient.NewAWSSession(m.ActiveProfile, m.ActiveRegion)
	if err != nil {
		m.StatusMessage = fmt.Sprintf("AWS config error: %v", err)
		m.StatusIsError = true
		return m
	}
	m.Clients = awsclient.CreateServiceClients(cfg)
	m.StatusMessage = fmt.Sprintf("Connected: %s / %s", m.ActiveProfile, m.ActiveRegion)
	m.StatusIsError = false
	return m
}

// executeCommand parses and executes a command string entered in command mode.
func (m AppState) executeCommand(cmd string) (tea.Model, tea.Cmd) {
	cmd = strings.TrimSpace(cmd)
	cmd = strings.ToLower(cmd)
	if cmd == "" {
		return m, nil
	}

	switch cmd {
	case "main", "root":
		m.CurrentView = MainMenuView
		m.Breadcrumbs = []string{"main"}
		m.SelectedIndex = 0
		m.StatusMessage = ""
		return m, nil

	case "q", "quit":
		return m, tea.Quit

	case "ctx":
		profiles, err := awsclient.ListProfiles(awsclient.DefaultConfigPath(), awsclient.DefaultCredentialsPath())
		if err != nil {
			m.StatusMessage = fmt.Sprintf("Error listing profiles: %v", err)
			m.StatusIsError = true
			return m, nil
		}
		if len(profiles) == 0 {
			m.StatusMessage = "No AWS profiles found. Configure with: aws configure"
			m.StatusIsError = true
			return m, nil
		}
		m.pushCurrentView()
		m.ProfileSelector = views.NewProfileSelect(profiles, m.ActiveProfile)
		m.CurrentView = ProfileSelectView
		m.SelectedIndex = 0
		m.Breadcrumbs = append(m.Breadcrumbs, "profile")
		return m, nil

	case "region":
		m.pushCurrentView()
		regions := awsclient.AllRegions()
		m.RegionSelector = views.NewRegionSelect(regions, m.ActiveRegion)
		m.CurrentView = RegionSelectView
		m.SelectedIndex = 0
		m.Breadcrumbs = append(m.Breadcrumbs, "region")
		return m, nil
	}

	// Check if it matches a resource type
	rt := resource.FindResourceType(cmd)
	if rt != nil {
		m.pushCurrentView()
		m.CurrentResourceType = rt.ShortName
		m.CurrentView = ResourceListView
		m.Breadcrumbs = []string{"main", rt.Name}
		m.SelectedIndex = 0
		m.Filter = ""
		m.FilteredResources = nil
		m.S3Bucket = ""
		m.S3Prefix = ""
		m.HScrollOffset = 0
		m.Loading = true
		return m, m.fetchResources()
	}

	m.StatusMessage = fmt.Sprintf("Unknown command: :%s", cmd)
	m.StatusIsError = true
	return m, nil
}

// fetchResources returns a tea.Cmd that will fetch resources for the current type.
func (m AppState) fetchResources() tea.Cmd {
	resourceType := m.CurrentResourceType

	if m.Clients == nil {
		return func() tea.Msg {
			return APIErrorMsg{
				ResourceType: resourceType,
				Err:          fmt.Errorf("no AWS connection; use :ctx to set profile"),
			}
		}
	}

	switch resourceType {
	case "ec2":
		client := m.Clients.EC2
		return func() tea.Msg {
			resources, err := awsclient.FetchEC2Instances(context.Background(), client)
			if err != nil {
				return APIErrorMsg{ResourceType: resourceType, Err: err}
			}
			return ResourcesLoadedMsg{ResourceType: resourceType, Resources: resources}
		}
	case "s3":
		client := m.Clients.S3
		return func() tea.Msg {
			resources, err := awsclient.FetchS3Buckets(context.Background(), client)
			if err != nil {
				return APIErrorMsg{ResourceType: resourceType, Err: err}
			}
			return ResourcesLoadedMsg{ResourceType: resourceType, Resources: resources}
		}
	case "rds":
		client := m.Clients.RDS
		return func() tea.Msg {
			resources, err := awsclient.FetchRDSInstances(context.Background(), client)
			if err != nil {
				return APIErrorMsg{ResourceType: resourceType, Err: err}
			}
			return ResourcesLoadedMsg{ResourceType: resourceType, Resources: resources}
		}
	case "redis":
		client := m.Clients.ElastiCache
		return func() tea.Msg {
			resources, err := awsclient.FetchRedisClusters(context.Background(), client)
			if err != nil {
				return APIErrorMsg{ResourceType: resourceType, Err: err}
			}
			return ResourcesLoadedMsg{ResourceType: resourceType, Resources: resources}
		}
	case "docdb":
		client := m.Clients.DocDB
		return func() tea.Msg {
			resources, err := awsclient.FetchDocDBClusters(context.Background(), client)
			if err != nil {
				return APIErrorMsg{ResourceType: resourceType, Err: err}
			}
			return ResourcesLoadedMsg{ResourceType: resourceType, Resources: resources}
		}
	case "eks":
		listClient := m.Clients.EKS
		describeClient := m.Clients.EKS
		return func() tea.Msg {
			resources, err := awsclient.FetchEKSClusters(context.Background(), listClient, describeClient)
			if err != nil {
				return APIErrorMsg{ResourceType: resourceType, Err: err}
			}
			return ResourcesLoadedMsg{ResourceType: resourceType, Resources: resources}
		}
	case "secrets":
		client := m.Clients.SecretsManager
		return func() tea.Msg {
			resources, err := awsclient.FetchSecrets(context.Background(), client)
			if err != nil {
				return APIErrorMsg{ResourceType: resourceType, Err: err}
			}
			return ResourcesLoadedMsg{ResourceType: resourceType, Resources: resources}
		}
	default:
		return func() tea.Msg {
			return StatusMsg{Text: "Unknown resource type: " + resourceType, IsError: true}
		}
	}
}

// fetchS3Objects returns a tea.Cmd that fetches S3 objects for the current bucket/prefix.
func (m AppState) fetchS3Objects() tea.Cmd {
	if m.Clients == nil {
		return func() tea.Msg {
			return APIErrorMsg{
				ResourceType: "s3",
				Err:          fmt.Errorf("no AWS connection; use :ctx to set profile"),
			}
		}
	}
	client := m.Clients.S3
	bucket := m.S3Bucket
	prefix := m.S3Prefix
	return func() tea.Msg {
		resources, err := awsclient.FetchS3Objects(context.Background(), client, bucket, prefix)
		if err != nil {
			return APIErrorMsg{ResourceType: "s3", Err: err}
		}
		return ResourcesLoadedMsg{ResourceType: "s3", Resources: resources}
	}
}

// View implements tea.Model. It renders the full application UI.
func (m AppState) View() tea.View {
	var sections []string

	// Header
	header := m.renderHeader()
	sections = append(sections, header)

	// Breadcrumbs
	breadcrumbs := m.renderBreadcrumbs()
	sections = append(sections, breadcrumbs)

	// Content area
	content := m.renderContent()
	sections = append(sections, content)

	// Status bar
	statusBar := m.renderStatusBar()
	sections = append(sections, statusBar)

	output := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Cap output to terminal height, preserving the last line (status bar)
	if m.Height > 0 {
		lines := strings.Split(output, "\n")
		if len(lines) > m.Height {
			// Keep first lines (header, breadcrumbs) and last line (status bar)
			// Trim content lines from the middle
			lastLine := lines[len(lines)-1]
			lines = lines[:m.Height-1]
			lines = append(lines, lastLine)
		}
		output = strings.Join(lines, "\n")
	}

	v := tea.NewView(output)
	v.AltScreen = true
	return v
}

// renderHeader renders the top header line.
func (m AppState) renderHeader() string {
	headerText := fmt.Sprintf("a9s v%s | profile: %s | %s", version, m.ActiveProfile, m.ActiveRegion)
	if m.Loading {
		headerText += " [loading...]"
	}
	if m.Width > 0 {
		return styles.HeaderStyle.Width(m.Width).Render(headerText)
	}
	return styles.HeaderStyle.Render(headerText)
}

// renderBreadcrumbs renders the breadcrumb navigation line.
func (m AppState) renderBreadcrumbs() string {
	crumbs := strings.Join(m.Breadcrumbs, " > ")
	return styles.BreadcrumbStyle.Render(crumbs)
}

// renderContent renders the main content area based on the current view.
func (m AppState) renderContent() string {
	if m.ShowHelp {
		return m.renderHelp()
	}

	switch m.CurrentView {
	case MainMenuView:
		return m.renderMainMenu()
	case ResourceListView:
		return m.renderResourceList()
	case DetailView:
		return m.Detail.View()
	case JSONView:
		return m.JSONData.View()
	case RevealView:
		return m.Reveal.View()
	case ProfileSelectView:
		return m.ProfileSelector.View()
	case RegionSelectView:
		return m.RegionSelector.View()
	default:
		return "View not yet implemented"
	}
}

// renderMainMenu renders the main menu as a list of resource types.
func (m AppState) renderMainMenu() string {
	allTypes := resource.AllResourceTypes()
	var b strings.Builder
	b.WriteString("\n  AWS Resources\n\n")

	for i, rt := range allTypes {
		cursor := "  "
		if i == m.SelectedIndex {
			cursor = "> "
		}
		line := fmt.Sprintf("  %s%s", cursor, rt.Name)
		if i == m.SelectedIndex {
			line = styles.TableCursorStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n  Press : for commands, ? for help\n")
	return b.String()
}

// applyFilter updates FilteredResources based on the current Filter and Resources.
// It also resets SelectedIndex to 0 to avoid out-of-bounds cursor positions.
func (m *AppState) applyFilter() {
	if m.Filter == "" {
		m.FilteredResources = nil
		m.SelectedIndex = 0
		return
	}
	m.FilteredResources = views.FilterResources(m.Filter, m.Resources)
	m.SelectedIndex = 0
}

// sortResources sorts both Resources and FilteredResources in place by the given
// sort type: "name", "status", or "age". It finds the appropriate field key from
// the resource type definition columns.
func (m *AppState) sortResources(sortType string) {
	rt := resource.FindResourceType(m.CurrentResourceType)
	if rt == nil {
		return
	}

	var sortKey string
	switch sortType {
	case "name":
		sortKey = findColumnKeyBySubstr(rt.Columns, "name")
	case "status":
		sortKey = findColumnKeyBySubstr(rt.Columns, "status")
		if sortKey == "" {
			sortKey = findColumnKeyBySubstr(rt.Columns, "state")
		}
	case "age":
		for _, suffix := range []string{"time", "date", "created", "launch", "accessed", "changed"} {
			sortKey = findColumnKeyBySubstr(rt.Columns, suffix)
			if sortKey != "" {
				break
			}
		}
	}

	if sortKey == "" {
		// Fall back to sorting by Name field (case-insensitive)
		sort.Slice(m.Resources, func(i, j int) bool {
			return strings.ToLower(m.Resources[i].Name) < strings.ToLower(m.Resources[j].Name)
		})
	} else {
		sort.Slice(m.Resources, func(i, j int) bool {
			return strings.ToLower(m.Resources[i].Fields[sortKey]) < strings.ToLower(m.Resources[j].Fields[sortKey])
		})
	}

	// Re-apply filter if active
	if m.Filter != "" {
		m.FilteredResources = views.FilterResources(m.Filter, m.Resources)
	}
}

// findColumnKeyBySubstr returns the key of the first column whose key contains the
// given substring (case-insensitive). Returns "" if no match is found.
func findColumnKeyBySubstr(columns []resource.Column, substr string) string {
	lower := strings.ToLower(substr)
	for _, col := range columns {
		if strings.Contains(strings.ToLower(col.Key), lower) {
			return col.Key
		}
	}
	return ""
}

// displayResources returns the resources to display: FilteredResources if a filter
// is active, otherwise all Resources.
func (m AppState) displayResources() []resource.Resource {
	if m.Filter != "" && m.FilteredResources != nil {
		return m.FilteredResources
	}
	return m.Resources
}

// padOrTruncate pads or truncates a string to fit exactly the given width.
func padOrTruncate(s string, width int) string {
	if len(s) > width {
		if width <= 1 {
			return s[:width]
		}
		return s[:width-1] + "…"
	}
	return s + strings.Repeat(" ", width-len(s))
}

// renderResourceList renders the resource list view.
func (m AppState) renderResourceList() string {
	if m.Loading {
		return "\n  Loading resources..."
	}
	if len(m.Resources) == 0 {
		return fmt.Sprintf("\n  No %s resources found.", m.CurrentResourceType)
	}

	display := m.displayResources()

	if len(display) == 0 && m.Filter != "" {
		return fmt.Sprintf("\n  No %s resources matching filter: %s", m.CurrentResourceType, m.Filter)
	}

	rt := resource.FindResourceType(m.CurrentResourceType)
	if rt == nil {
		return "\n  Unknown resource type"
	}

	// Use S3 object columns when inside a bucket
	columns := rt.Columns
	title := rt.Name
	if m.CurrentResourceType == "s3" && m.S3Bucket != "" {
		columns = resource.S3ObjectColumns()
		title = "S3: " + m.S3Bucket
		if m.S3Prefix != "" {
			title += "/" + m.S3Prefix
		}
	}

	var b strings.Builder

	// Title with count
	b.WriteString(fmt.Sprintf("  %s (%d)\n", title, len(display)))

	// Calculate column widths
	colWidths := make([]int, len(columns))
	for i, col := range columns {
		colWidths[i] = len(col.Title)
		if col.Width > 0 {
			colWidths[i] = col.Width
		}
	}
	// Expand widths based on data
	for _, r := range display {
		for i, col := range columns {
			val := r.Fields[col.Key]
			if len(val) > colWidths[i] {
				colWidths[i] = len(val)
			}
		}
	}
	// Cap individual column widths
	maxColWidth := 40
	for i := range colWidths {
		if colWidths[i] > maxColWidth {
			colWidths[i] = maxColWidth
		}
		if colWidths[i] < 5 {
			colWidths[i] = 5
		}
	}

	// Clamp horizontal scroll offset
	totalWidth := 0
	for _, w := range colWidths {
		totalWidth += w
	}
	totalWidth += (len(columns) - 1) * 2 // gaps
	maxHScroll := totalWidth - m.Width + 4
	if maxHScroll < 0 {
		maxHScroll = 0
	}
	if m.HScrollOffset > maxHScroll {
		m.HScrollOffset = maxHScroll
	}

	// Build a full-width row from column values
	buildRow := func(values []string) string {
		var row strings.Builder
		for i, val := range values {
			row.WriteString(padOrTruncate(val, colWidths[i]))
			if i < len(values)-1 {
				row.WriteString("  ")
			}
		}
		return row.String()
	}

	// Apply horizontal scroll: crop a line to the visible window
	hcrop := func(prefix, line string) string {
		// prefix (cursor "  " or "> ") is always visible
		if m.HScrollOffset >= len(line) {
			return prefix
		}
		cropped := line[m.HScrollOffset:]
		maxVisible := m.Width - len(prefix)
		if maxVisible <= 0 {
			return prefix
		}
		if len(cropped) > maxVisible {
			cropped = cropped[:maxVisible]
		}
		return prefix + cropped
	}

	// Header row
	headerVals := make([]string, len(columns))
	for i, col := range columns {
		headerVals[i] = col.Title
	}
	headerLine := buildRow(headerVals)
	b.WriteString(hcrop("  ", headerLine))
	b.WriteString("\n")

	// Separator
	sepVals := make([]string, len(columns))
	for i, w := range colWidths {
		sepVals[i] = strings.Repeat("─", w)
	}
	sepLine := buildRow(sepVals)
	b.WriteString(hcrop("  ", sepLine))
	b.WriteString("\n")

	// Viewport: calculate visible window
	contentHeight := m.Height - 6 // header(1) + breadcrumbs(1) + title(1) + col header(1) + separator(1) + status bar(1)
	if contentHeight < 3 {
		contentHeight = 3
	}

	startIdx := 0
	if m.SelectedIndex >= contentHeight {
		startIdx = m.SelectedIndex - contentHeight + 1
	}
	endIdx := startIdx + contentHeight
	if endIdx > len(display) {
		endIdx = len(display)
	}

	// Data rows (only visible window)
	for i := startIdx; i < endIdx; i++ {
		r := display[i]
		cursor := "  "
		if i == m.SelectedIndex {
			cursor = "> "
		}

		rowVals := make([]string, len(columns))
		for j, col := range columns {
			rowVals[j] = r.Fields[col.Key]
		}
		rowLine := buildRow(rowVals)
		row := hcrop(cursor, rowLine)
		if i == m.SelectedIndex {
			row = styles.TableCursorStyle.Render(row)
		}
		b.WriteString(row)
		b.WriteString("\n")
	}

	return b.String()
}

// renderHelp renders the help overlay using the ui.HelpModel.
func (m AppState) renderHelp() string {
	help := ui.NewHelpModel()
	help.Width = m.Width
	help.Height = m.Height
	return help.View()
}

// renderStatusBar renders the status bar at the bottom of the screen.
func (m AppState) renderStatusBar() string {
	if m.CommandMode {
		display := ":" + m.CommandText
		suggestion := findBestCommandMatch(m.CommandText)
		if suggestion != "" && suggestion != m.CommandText {
			display += suggestion[len(m.CommandText):]
		}
		return styles.StatusBarStyle.Render(display)
	}
	if m.FilterMode {
		filterDisplay := "/" + m.Filter
		if m.Filter == "" {
			filterDisplay = "/  (type to filter)"
		} else {
			matched := len(m.displayResources())
			total := len(m.Resources)
			filterDisplay = fmt.Sprintf("/%s (%d/%d)", m.Filter, matched, total)
		}
		return styles.HeaderStyle.Render(filterDisplay)
	}
	if m.StatusMessage != "" {
		if m.StatusIsError {
			return styles.ErrorStyle.Render(m.StatusMessage)
		}
		return styles.StatusBarStyle.Render(m.StatusMessage)
	}
	return styles.StatusBarStyle.Render("Ready")
}

// knownCommands is the set of built-in commands used for auto-suggestions.
var knownCommands = []string{
	"main", "root", "ctx", "region",
	"s3", "ec2", "rds", "redis", "docdb", "eks", "secrets",
	"q", "quit",
}

// findBestCommandMatch returns the first known command that starts with the
// given prefix (case-insensitive) and is longer than the prefix itself.
// This ensures typing "q" suggests "quit" rather than matching "q" exactly.
// Returns "" if no match or prefix is empty.
func findBestCommandMatch(prefix string) string {
	if prefix == "" {
		return ""
	}
	lower := strings.ToLower(prefix)
	for _, cmd := range knownCommands {
		if strings.HasPrefix(cmd, lower) && cmd != lower {
			return cmd
		}
	}
	return ""
}
