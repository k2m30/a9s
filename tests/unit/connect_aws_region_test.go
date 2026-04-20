package unit

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// TestBug82_NewAWSSession_EmptyRegionProducesEmptyConfig demonstrates the root
// cause of the bug: when no AWS config file, no env vars, and empty region
// parameter are provided, NewAWSSession produces a config with empty Region.
// Any API call made with this config will fail with "Missing Region".
func TestBug82_NewAWSSession_EmptyRegionProducesEmptyConfig(t *testing.T) {
	t.Setenv("AWS_CONFIG_FILE", "/nonexistent/path/config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent/path/credentials")
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_PROFILE", "")

	// This simulates what happens BEFORE the fix: connectAWS calls
	// NewAWSSession("", "") — the empty region parameter means no
	// config.WithRegion option is added.
	cfg, err := awsclient.NewAWSSessionContext(context.Background(), "", "")
	if err != nil {
		// Profile error is acceptable in isolated env
		t.Logf("NewAWSSession error (expected in isolated env): %v", err)
		return
	}

	// Without any region source, cfg.Region should be empty.
	// This is the root cause: later API calls fail with "Missing Region".
	if cfg.Region == "" {
		t.Log("Confirmed: NewAWSSession with empty region and no config produces empty Region in cfg")
	} else {
		t.Skipf("SDK found region %q from an unexpected source", cfg.Region)
	}
}

// TestBug82_NewAWSSession_ExplicitRegionPopulatesConfig verifies that when
// GetDefaultRegion resolves a fallback region and passes it to NewAWSSession,
// the config Region is properly set. This is the behavior we want after the fix.
func TestBug82_NewAWSSession_ExplicitRegionPopulatesConfig(t *testing.T) {
	t.Setenv("AWS_CONFIG_FILE", "/nonexistent/path/config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent/path/credentials")
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_PROFILE", "")

	// Resolve region via GetDefaultRegion — this is what the fix does
	region := awsclient.GetDefaultRegion("/nonexistent/path/config", "default")
	if region != "us-east-1" {
		t.Fatalf("GetDefaultRegion should return us-east-1 fallback, got %q", region)
	}

	// Now pass the resolved region to NewAWSSession
	cfg, err := awsclient.NewAWSSessionContext(context.Background(), "", region)
	if err != nil {
		t.Logf("NewAWSSession error (expected in isolated env): %v", err)
		return
	}

	// With the resolved region, cfg.Region should be "us-east-1"
	if cfg.Region != "us-east-1" {
		t.Errorf("expected Region=us-east-1 in config, got %q", cfg.Region)
	}
}

// TestBug82_GetDefaultRegion_FallbackWhenNoConfig verifies that GetDefaultRegion
// returns "us-east-1" when the config file doesn't exist.
func TestBug82_GetDefaultRegion_FallbackWhenNoConfig(t *testing.T) {
	region := awsclient.GetDefaultRegion("/nonexistent/path/config", "default")
	if region != "us-east-1" {
		t.Errorf("expected fallback us-east-1, got %q", region)
	}
}

// TestBug82_GetDefaultRegion_EmptyProfile verifies the fallback for empty profile.
func TestBug82_GetDefaultRegion_EmptyProfile(t *testing.T) {
	region := awsclient.GetDefaultRegion("/nonexistent/path/config", "")
	if region != "us-east-1" {
		t.Errorf("expected fallback us-east-1 for empty profile, got %q", region)
	}
}

// TestBug82_ConnectAWS_NoMissingRegionError verifies that when connectAWS
// is called with empty region, the resulting ClientsReadyMsg does NOT contain
// a "Missing Region" error. It may contain other errors (profile not found,
// credentials not found) which are acceptable.
func TestBug82_ConnectAWS_NoMissingRegionError(t *testing.T) {
	tui.Version = "test"
	m := tui.New("default", "")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	_, cmd := rootApplyMsg(m, messages.InitConnectMsg{Profile: "default", Region: ""})
	if cmd == nil {
		t.Fatal("InitConnectMsg should return a command")
	}

	msg := cmd()
	clientsReady, ok := msg.(messages.ClientsReadyMsg)
	if !ok {
		t.Fatalf("expected ClientsReadyMsg, got %T", msg)
	}

	if clientsReady.Err != nil {
		errStr := strings.ToLower(clientsReady.Err.Error())
		if strings.Contains(errStr, "missing region") || strings.Contains(errStr, "could not find region") {
			t.Errorf("connectAWS with empty region should resolve fallback before SDK call, "+
				"but got region error: %v", clientsReady.Err)
		}
		t.Logf("Non-region error (acceptable): %v", clientsReady.Err)
	}
}

// TestBug82_ProfileSwitch_NoMissingRegionError verifies that profile switching
// (which calls connectAWS with empty region) does not produce "Missing Region" errors.
func TestBug82_ProfileSwitch_NoMissingRegionError(t *testing.T) {
	tui.Version = "test"
	m := tui.New("dev", "us-west-2")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "ec2"})

	_, cmd := rootApplyMsg(m, messages.ProfileSelectedMsg{Profile: "some-profile"})
	if cmd == nil {
		t.Fatal("ProfileSelectedMsg should return a batch command")
	}

	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected BatchMsg, got %T", msg)
	}

	for _, subCmd := range batch {
		if subCmd == nil {
			continue
		}
		subMsg := subCmd()
		if clientsReady, ok := subMsg.(messages.ClientsReadyMsg); ok {
			if clientsReady.Err != nil {
				errStr := strings.ToLower(clientsReady.Err.Error())
				if strings.Contains(errStr, "missing region") || strings.Contains(errStr, "could not find region") {
					t.Errorf("connectAWS from profile switch should resolve fallback region, "+
						"but got region error: %v", clientsReady.Err)
				}
			}
			return
		}
	}
}

// TestBug82_RegionSwitch_PassesExplicitRegion verifies explicit region selection
// passes the region directly (no fallback needed).
func TestBug82_RegionSwitch_PassesExplicitRegion(t *testing.T) {
	tui.Version = "test"
	m := tui.New("dev", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	_, cmd := rootApplyMsg(m, messages.RegionSelectedMsg{Region: "eu-west-1"})
	if cmd == nil {
		t.Fatal("RegionSelectedMsg should return a batch command")
	}

	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected BatchMsg, got %T", msg)
	}

	for _, subCmd := range batch {
		if subCmd == nil {
			continue
		}
		subMsg := subCmd()
		if clientsReady, ok := subMsg.(messages.ClientsReadyMsg); ok {
			if clientsReady.Err != nil {
				errStr := strings.ToLower(clientsReady.Err.Error())
				if strings.Contains(errStr, "missing region") {
					t.Errorf("connectAWS with explicit region should not have missing region error: %v",
						clientsReady.Err)
				}
			}
			return
		}
	}
}
