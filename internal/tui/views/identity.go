package views

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// IdentityData holds caller identity information for display.
// This is a view-layer struct to avoid importing the aws package.
type IdentityData struct {
	AccountID     string
	AccountAlias  string
	ARN           string
	RoleName      string
	UserName      string
	SessionName   string
	IsAssumedRole bool
}

// identityState tracks the lifecycle of the identity view.
type identityState int

const (
	identityLoading identityState = iota
	identityLoaded
	identityError
)

// IdentityModel renders the current IAM caller identity.
// Any key press closes the view (parent pops it).
type IdentityModel struct {
	keys     keys.Map
	profile  string
	region   string
	state    identityState
	data     IdentityData
	errorMsg string
	width    int
	height   int
}

// NewIdentity returns an IdentityModel in loading state.
func NewIdentity(profile, region string, k keys.Map) IdentityModel {
	return IdentityModel{
		keys:    k,
		profile: profile,
		region:  region,
		state:   identityLoading,
	}
}

// Init implements tea.Model.
func (m IdentityModel) Init() (IdentityModel, tea.Cmd) {
	return m, nil
}

// Update handles messages. IdentityLoadedMsg/IdentityErrorMsg update state;
// any KeyMsg sends PopViewMsg (dismisses the view).
func (m IdentityModel) Update(msg tea.Msg) (IdentityModel, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.IdentityLoadedMsg:
		m.state = identityLoaded
		if data, ok := msg.Identity.(IdentityData); ok {
			m.data = data
		}
		return m, nil
	case messages.IdentityErrorMsg:
		m.state = identityError
		m.errorMsg = msg.Err
		return m, nil
	case tea.KeyMsg:
		_ = msg
		return m, func() tea.Msg {
			return messages.PopViewMsg{}
		}
	}
	return m, nil
}

// SetIdentity transitions the view to loaded state with the given data.
func (m *IdentityModel) SetIdentity(data IdentityData) {
	m.data = data
	m.state = identityLoaded
}

// SetError transitions the view to error state.
func (m *IdentityModel) SetError(msg string) {
	m.errorMsg = msg
	m.state = identityError
}

// View renders the identity information.
func (m IdentityModel) View() string {
	switch m.state {
	case identityLoading:
		return m.renderLoading()
	case identityError:
		return m.renderError()
	default:
		return m.renderLoaded()
	}
}

func (m IdentityModel) renderLoading() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("  ")
	sb.WriteString(styles.DimText.Render("Fetching identity..."))
	sb.WriteString("\n")
	return sb.String()
}

func (m IdentityModel) renderError() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("  ")
	sb.WriteString(styles.FlashError.Render("Error: " + m.errorMsg))
	sb.WriteString("\n\n")
	sb.WriteString("  ")
	sb.WriteString(styles.DimText.Render("Press any key to close"))
	sb.WriteString("\n")
	return sb.String()
}

func (m IdentityModel) renderLoaded() string {
	secStyle := styles.IdentitySectionStyle
	lblStyle := styles.IdentityLabelStyle
	valStyle := styles.IdentityValueStyle

	labelW := 14 // width for label column

	line := func(label, value string) string {
		if value == "" {
			value = "--"
		}
		return "  " + lblStyle.Render(padRight(label, labelW)) + valStyle.Render(value)
	}

	var sb strings.Builder

	// Account section
	sb.WriteString("\n")
	sb.WriteString("  " + secStyle.Render("ACCOUNT"))
	sb.WriteString("\n\n")
	sb.WriteString(line("Alias", m.data.AccountAlias))
	sb.WriteString("\n")
	sb.WriteString(line("Account ID", m.data.AccountID))
	sb.WriteString("\n")

	// Caller section
	sb.WriteString("\n")
	sb.WriteString("  " + secStyle.Render("CALLER"))
	sb.WriteString("\n\n")
	sb.WriteString(line("ARN", m.data.ARN))
	sb.WriteString("\n")
	if m.data.IsAssumedRole {
		sb.WriteString(line("Role", m.data.RoleName))
		sb.WriteString("\n")
		sb.WriteString(line("Session", m.data.SessionName))
		sb.WriteString("\n")
	} else {
		sb.WriteString(line("User", m.data.UserName))
		sb.WriteString("\n")
	}

	// Session section
	sb.WriteString("\n")
	sb.WriteString("  " + secStyle.Render("SESSION"))
	sb.WriteString("\n\n")
	sb.WriteString(line("Profile", m.profile))
	sb.WriteString("\n")
	sb.WriteString(line("Region", m.region))
	sb.WriteString("\n")

	// Close hint
	sb.WriteString("\n")
	closeHint := styles.DimText.Render("Press any key to close  |  c to copy ARN")
	sb.WriteString(lipgloss.Place(m.width, 1, lipgloss.Center, lipgloss.Top, closeHint))
	sb.WriteString("\n")

	return sb.String()
}

// padRight pads a string to the given width using spaces.
func padRight(s string, w int) string {
	visW := lipgloss.Width(s)
	if visW >= w {
		return s
	}
	return s + strings.Repeat(" ", w-visW)
}

// CopyContent returns the ARN for clipboard copy.
func (m IdentityModel) CopyContent() (string, string) {
	if m.state == identityLoaded && m.data.ARN != "" {
		return m.data.ARN, "Copied!"
	}
	return "", ""
}

// GetHelpContext returns the help context for the identity view.
func (m IdentityModel) GetHelpContext() HelpContext {
	return HelpFromMainMenu
}

// SetSize updates layout dimensions.
func (m *IdentityModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// FrameTitle returns "identity".
func (m IdentityModel) FrameTitle() string {
	return "identity"
}
