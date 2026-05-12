package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Identity view model tests — construct views.IdentityModel directly
// ---------------------------------------------------------------------------

// TestIdentityView_LoadedContent_AccountSection verifies that the identity
// view renders account ID and alias after receiving identity data.
func TestIdentityView_LoadedContent_AccountSection(t *testing.T) {
	m := views.NewIdentity("testprofile", "us-east-1", keys.Default())
	m.SetSize(80, 24)
	m.SetIdentity(views.IdentityData{
		AccountID:     "123456789012",
		AccountAlias:  "acme-prod",
		ARN:           "arn:aws:sts::123456789012:assumed-role/admin-role/session",
		RoleName:      "admin-role",
		SessionName:   "session",
		IsAssumedRole: true,
	})

	plain := stripANSI(m.View())
	if !strings.Contains(plain, "123456789012") {
		t.Errorf("identity view should contain account ID, got:\n%s", plain)
	}
	if !strings.Contains(plain, "acme-prod") {
		t.Errorf("identity view should contain account alias, got:\n%s", plain)
	}
}

// TestIdentityView_LoadedContent_CallerSection_AssumedRole verifies that
// the identity view shows ARN, role name, and session name for assumed roles.
func TestIdentityView_LoadedContent_CallerSection_AssumedRole(t *testing.T) {
	m := views.NewIdentity("testprofile", "us-east-1", keys.Default())
	m.SetSize(80, 24)
	m.SetIdentity(views.IdentityData{
		AccountID:     "123456789012",
		AccountAlias:  "acme-prod",
		ARN:           "arn:aws:sts::123456789012:assumed-role/admin-role/session-name",
		RoleName:      "admin-role",
		SessionName:   "session-name",
		IsAssumedRole: true,
	})

	plain := stripANSI(m.View())
	if !strings.Contains(plain, "arn:aws:sts::123456789012:assumed-role/admin-role/session-name") {
		t.Errorf("identity view should contain full ARN, got:\n%s", plain)
	}
	if !strings.Contains(plain, "admin-role") {
		t.Errorf("identity view should contain role name, got:\n%s", plain)
	}
	if !strings.Contains(plain, "session-name") {
		t.Errorf("identity view should contain session name, got:\n%s", plain)
	}
}

// TestIdentityView_LoadedContent_CallerSection_IAMUser verifies that
// the identity view shows the user name for IAM users.
func TestIdentityView_LoadedContent_CallerSection_IAMUser(t *testing.T) {
	m := views.NewIdentity("testprofile", "us-east-1", keys.Default())
	m.SetSize(80, 24)
	m.SetIdentity(views.IdentityData{
		AccountID:     "111222333444",
		AccountAlias:  "",
		ARN:           "arn:aws:iam::111222333444:user/deploy-bot@example.com",
		UserName:      "deploy-bot@example.com",
		IsAssumedRole: false,
	})

	plain := stripANSI(m.View())
	if !strings.Contains(plain, "deploy-bot@example.com") {
		t.Errorf("identity view should contain user name, got:\n%s", plain)
	}
}

// TestIdentityView_LoadingState verifies the view shows a loading indicator
// before identity data has arrived.
func TestIdentityView_LoadingState(t *testing.T) {
	m := views.NewIdentity("testprofile", "us-east-1", keys.Default())
	m.SetSize(80, 24)

	plain := strings.ToLower(stripANSI(m.View()))
	if !strings.Contains(plain, "fetch") {
		t.Errorf("identity view before data should show fetching indicator, got:\n%s", plain)
	}
}

// TestIdentityView_ErrorState verifies the view shows an error message
// when identity fetch fails.
func TestIdentityView_ErrorState(t *testing.T) {
	m := views.NewIdentity("testprofile", "us-east-1", keys.Default())
	m.SetSize(80, 24)
	m.SetError("ExpiredToken: security token has expired")

	plain := stripANSI(m.View())
	if !strings.Contains(plain, "ExpiredToken") {
		t.Errorf("identity view should show error text, got:\n%s", plain)
	}
}

// TestIdentityView_CopyContent_ReturnsARN verifies that CopyContent()
// returns the ARN when identity data is loaded.
func TestIdentityView_CopyContent_ReturnsARN(t *testing.T) {
	m := views.NewIdentity("testprofile", "us-east-1", keys.Default())
	m.SetSize(80, 24)
	m.SetIdentity(views.IdentityData{
		AccountID:     "123456789012",
		ARN:           "arn:aws:sts::123456789012:assumed-role/admin-role/session",
		RoleName:      "admin-role",
		IsAssumedRole: true,
	})

	content, label := m.CopyContent()
	if content != "arn:aws:sts::123456789012:assumed-role/admin-role/session" {
		t.Errorf("CopyContent() should return ARN, got %q", content)
	}
	if label == "" {
		t.Error("CopyContent() label should not be empty")
	}
}

// TestIdentityView_CopyContent_EmptyWhenLoading verifies CopyContent returns
// empty when no identity has been loaded.
func TestIdentityView_CopyContent_EmptyWhenLoading(t *testing.T) {
	m := views.NewIdentity("testprofile", "us-east-1", keys.Default())
	m.SetSize(80, 24)

	content, label := m.CopyContent()
	if content != "" {
		t.Errorf("CopyContent() should return empty in loading state, got %q", content)
	}
	if label != "" {
		t.Errorf("CopyContent() label should be empty in loading state, got %q", label)
	}
}

// TestIdentityView_FrameTitle verifies the frame title is "identity".
func TestIdentityView_FrameTitle(t *testing.T) {
	m := views.NewIdentity("testprofile", "us-east-1", keys.Default())
	if m.FrameTitle() != "identity" {
		t.Errorf("FrameTitle should be 'identity', got %q", m.FrameTitle())
	}
}

// TestIdentityView_AnyKeyDismisses verifies that any key press sends PopViewMsg.
func TestIdentityView_AnyKeyDismisses(t *testing.T) {
	m := views.NewIdentity("testprofile", "us-east-1", keys.Default())
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyPressMsg{Code: -1, Text: "q"})
	if cmd == nil {
		t.Fatal("Update(KeyMsg) should return a non-nil Cmd")
	}

	msg := cmd()
	if _, ok := msg.(messages.PopView); !ok {
		t.Errorf("Update(KeyMsg) should return PopViewMsg, got %T", msg)
	}
}

// TestIdentityView_Update_IdentityLoadedMsg verifies that IdentityLoadedMsg
// transitions the view to loaded state when passed IdentityData.
func TestIdentityView_Update_IdentityLoadedMsg(t *testing.T) {
	m := views.NewIdentity("testprofile", "us-east-1", keys.Default())
	m.SetSize(80, 24)

	data := views.IdentityData{
		AccountID:    "123456789012",
		AccountAlias: "acme-prod",
		ARN:          "arn:aws:sts::123456789012:assumed-role/admin/s",
	}
	m, _ = m.Update(messages.IdentityLoaded{Identity: data})

	plain := stripANSI(m.View())
	if !strings.Contains(plain, "123456789012") {
		t.Errorf("after IdentityLoadedMsg, view should show account ID, got:\n%s", plain)
	}
	if !strings.Contains(plain, "acme-prod") {
		t.Errorf("after IdentityLoadedMsg, view should show alias, got:\n%s", plain)
	}
}

// TestIdentityView_Update_IdentityErrorMsg verifies that IdentityErrorMsg
// transitions the view to error state.
func TestIdentityView_Update_IdentityErrorMsg(t *testing.T) {
	m := views.NewIdentity("testprofile", "us-east-1", keys.Default())
	m.SetSize(80, 24)

	m, _ = m.Update(messages.IdentityError{Err: "ExpiredToken: the security token has expired"})

	plain := stripANSI(m.View())
	if !strings.Contains(plain, "ExpiredToken") {
		t.Errorf("after IdentityErrorMsg, view should show error, got:\n%s", plain)
	}
}

// TestIdentityView_SessionSection verifies the session section shows
// profile and region.
func TestIdentityView_SessionSection(t *testing.T) {
	m := views.NewIdentity("prod-profile", "eu-west-1", keys.Default())
	m.SetSize(80, 24)
	m.SetIdentity(views.IdentityData{
		AccountID: "123456789012",
		ARN:       "arn:aws:sts::123456789012:assumed-role/admin/s",
	})

	plain := stripANSI(m.View())
	if !strings.Contains(plain, "prod-profile") {
		t.Errorf("identity view should show profile name, got:\n%s", plain)
	}
	if !strings.Contains(plain, "eu-west-1") {
		t.Errorf("identity view should show region, got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Root wiring tests — use root model helpers
// These tests require the tui package to compile. If app_handlers.go has
// a compile error (e.g. wrong NewIdentity arg count), these will fail to
// build until that is fixed.
// ---------------------------------------------------------------------------

// TestRoot_IKey_ShowsIdentityView verifies that pressing 'i' from the
// main menu opens the identity view.
func TestRoot_IKey_ShowsIdentityView(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("i"))

	plain := strings.ToLower(stripANSI(rootViewContent(m)))
	if !strings.Contains(plain, "identity") {
		t.Errorf("pressing 'i' should show identity view, got:\n%s", plain)
	}
}

// TestRoot_IdentityView_EscDismisses verifies that pressing Esc on the
// identity view returns to the previous view.
func TestRoot_IdentityView_EscDismisses(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Open identity view
	m, _ = rootApplyMsg(m, rootKeyPress("i"))
	identityPlain := strings.ToLower(stripANSI(rootViewContent(m)))
	if !strings.Contains(identityPlain, "identity") {
		t.Fatal("identity view should be visible before dismiss test")
	}

	// Dismiss with Esc
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	afterPlain := strings.ToLower(stripANSI(rootViewContent(m)))

	// Should be back on main menu, not identity
	if strings.Contains(afterPlain, "fetching identity") {
		t.Error("identity view should be dismissed after Esc")
	}
}

// TestRoot_IdentityLoaded_UpdatesHeader verifies that after receiving
// IdentityLoadedMsg, the header contains the account badge and role name.
func TestRoot_IdentityLoaded_UpdatesHeader(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	identity := &awsclient.CallerIdentity{
		AccountID:     "123456789012",
		AccountAlias:  "acme-prod",
		Arn:           "arn:aws:sts::123456789012:assumed-role/admin-role/session",
		RoleName:      "admin-role",
		IsAssumedRole: true,
		IdentityName:  "admin-role",
	}
	m, _ = rootApplyMsg(m, messages.IdentityLoaded{Identity: identity})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "acme-prod") {
		t.Errorf("header should contain account badge after IdentityLoadedMsg, got:\n%s", plain)
	}
	if !strings.Contains(plain, "admin-role") {
		t.Errorf("header should contain identity name after IdentityLoadedMsg, got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Header layout tests — identity badge in RenderHeader
// ---------------------------------------------------------------------------

// TestLayoutRenderHeader_WithIdentityBadge verifies the header renders
// account badge and identity name when provided.
func TestLayoutRenderHeader_WithIdentityBadge(t *testing.T) {
	got := layout.RenderHeader("prod", "us-east-1", "0.5.0", 120, "? for help", "acme-prod", "admin-role")
	plain := stripANSI(got)
	if !strings.Contains(plain, "(acme-prod)") {
		t.Errorf("header should contain '(acme-prod)', got %q", plain)
	}
	if !strings.Contains(plain, "admin-role") {
		t.Errorf("header should contain 'admin-role', got %q", plain)
	}
}

// TestLayoutRenderHeader_IdentityBadge_NarrowOmitted verifies the
// identity badge is omitted when the terminal is too narrow to fit.
func TestLayoutRenderHeader_IdentityBadge_NarrowOmitted(t *testing.T) {
	got := layout.RenderHeader("my-long-profile", "ap-southeast-2", "3.15.1", 50, "? for help", "999888777666", "MyLongAccessRole")
	plain := stripANSI(got)
	if strings.Contains(plain, "(999888777666)") {
		t.Errorf("narrow header should omit identity badge, got %q", plain)
	}
}

// TestLayoutRenderHeader_EmptyBadge verifies that empty badge/identity
// strings produce the same output as no badge.
func TestLayoutRenderHeader_EmptyBadge(t *testing.T) {
	withEmpty := layout.RenderHeader("prod", "us-east-1", "0.5.0", 80, "? for help", "", "")
	without := layout.RenderHeader("prod", "us-east-1", "0.5.0", 80, "? for help", "", "")
	if withEmpty != without {
		t.Errorf("empty badge should produce identical output:\nwith empty: %q\nwithout:    %q", stripANSI(withEmpty), stripANSI(without))
	}
}

// TestLayoutRenderHeader_WithIdentityBadge_Width verifies the header
// maintains the exact requested width when identity badge is present.
func TestLayoutRenderHeader_WithIdentityBadge_Width(t *testing.T) {
	got := layout.RenderHeader("prod", "us-east-1", "0.5.0", 120, "? for help", "acme-prod", "admin-role")
	vis := lipglossWidth(got)
	if vis != 120 {
		t.Errorf("header with badge should be exactly 120 columns, got %d", vis)
	}
}

// ---------------------------------------------------------------------------
// Help screen test — identity binding
// ---------------------------------------------------------------------------

// ════════════════════════════════════════════════════════════════════════════
// GetHelpContext — 0% hit: returns HelpFromMainMenu
// ════════════════════════════════════════════════════════════════════════════

// TestIdentityView_GetHelpContext verifies GetHelpContext returns HelpFromMainMenu.
func TestIdentityView_GetHelpContext(t *testing.T) {
	m := views.NewIdentity("testprofile", "us-east-1", keys.Default())
	got := m.GetHelpContext()
	if got != views.HelpFromMainMenu {
		t.Errorf("GetHelpContext() = %v, want HelpFromMainMenu (%v)", got, views.HelpFromMainMenu)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Update — unrecognized message type returns unchanged model
// ════════════════════════════════════════════════════════════════════════════

// TestIdentityView_Update_UnknownMsg verifies that an unrecognized message type
// leaves the model state unchanged and returns a nil command.
func TestIdentityView_Update_UnknownMsg(t *testing.T) {
	m := views.NewIdentity("testprofile", "us-east-1", keys.Default())
	m.SetSize(80, 24)

	type unknownMsg struct{ Value string }
	m2, cmd := m.Update(unknownMsg{"ignored"})
	if cmd != nil {
		t.Error("Update(unknown msg) should return nil Cmd")
	}
	// State must remain loading (no identity data received)
	plain := strings.ToLower(stripANSI(m2.View()))
	if !strings.Contains(plain, "fetch") {
		t.Errorf("after unknown msg, view should still show loading state, got:\n%s", plain)
	}
}

// TestQA_Help_ShowsIdentityBinding verifies that the help screen shows
// the 'i' / 'identity' key binding in all view contexts.
func TestQA_Help_ShowsIdentityBinding(t *testing.T) {
	contexts := []struct {
		name  string
		setup func() tui.Model
	}{
		{
			name:  "main_menu",
			setup: newRootSizedModel,
		},
		{
			name: "resource_list",
			setup: func() tui.Model {
				m := newRootSizedModel()
				m, _ = rootApplyMsg(m, messages.Navigate{
					Target:       messages.TargetResourceList,
					ResourceType: "ec2",
				})
				return m
			},
		},
		{
			name: "detail_view",
			setup: func() tui.Model {
				m := newRootSizedModel()
				m, _ = rootApplyMsg(m, messages.Navigate{
					Target:       messages.TargetResourceList,
					ResourceType: "ec2",
				})
				m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
					ResourceType: "ec2",
					Resources: []resource.Resource{
						{
							ID:     "i-1234567890abcdef0",
							Name:   "test-instance",
							Status: "running",
							Fields: map[string]string{"Type": "t3.micro", "AZ": "us-east-1a"},
						},
					},
				})
				m, _ = rootApplyMsg(m, messages.Navigate{
					Target: messages.TargetDetail,
				})
				return m
			},
		},
	}

	for _, tc := range contexts {
		t.Run(tc.name, func(t *testing.T) {
			tui.Version = "0.6.0"
			m := tc.setup()

			// Open help
			m, _ = rootApplyMsg(m, rootKeyPress("?"))
			plain := strings.ToLower(stripANSI(rootViewContent(m)))

			if !strings.Contains(plain, "identity") {
				t.Errorf("help in %s context should show 'identity' binding, got:\n%s", tc.name, plain)
			}
		})
	}
}
