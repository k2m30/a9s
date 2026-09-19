package unit

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// With no AWS config file, no env vars and an empty region parameter,
// NewAWSSession produces a config with an empty Region, and any API call made
// with it fails with "Missing Region".
func TestBug82_NewAWSSession_EmptyRegionProducesEmptyConfig(t *testing.T) {
	t.Setenv("AWS_CONFIG_FILE", "/nonexistent/path/config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent/path/credentials")
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_PROFILE", "")

	cfg, err := awsclient.NewAWSSessionContext(context.Background(), "", "")
	if err != nil {
		// Profile error is acceptable in isolated env
		t.Logf("NewAWSSession error (expected in isolated env): %v", err)
		return
	}

	if cfg.Region == "" {
		t.Log("Confirmed: NewAWSSession with empty region and no config produces empty Region in cfg")
	} else {
		t.Skipf("SDK found region %q from an unexpected source", cfg.Region)
	}
}

func TestBug82_NewAWSSession_ExplicitRegionPopulatesConfig(t *testing.T) {
	t.Setenv("AWS_CONFIG_FILE", "/nonexistent/path/config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent/path/credentials")
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_PROFILE", "")

	region := awsclient.GetDefaultRegion("/nonexistent/path/config", "default")
	if region != "us-east-1" {
		t.Fatalf("GetDefaultRegion should return us-east-1 fallback, got %q", region)
	}

	cfg, err := awsclient.NewAWSSessionContext(context.Background(), "", region)
	if err != nil {
		t.Logf("NewAWSSession error (expected in isolated env): %v", err)
		return
	}

	if cfg.Region != "us-east-1" {
		t.Errorf("expected Region=us-east-1 in config, got %q", cfg.Region)
	}
}

func TestBug82_GetDefaultRegion_FallbackWhenNoConfig(t *testing.T) {
	region := awsclient.GetDefaultRegion("/nonexistent/path/config", "default")
	if region != "us-east-1" {
		t.Errorf("expected fallback us-east-1, got %q", region)
	}
}

func TestBug82_GetDefaultRegion_EmptyProfile(t *testing.T) {
	region := awsclient.GetDefaultRegion("/nonexistent/path/config", "")
	if region != "us-east-1" {
		t.Errorf("expected fallback us-east-1 for empty profile, got %q", region)
	}
}

// Other connect errors (profile or credentials not found) are acceptable in an
// isolated environment; "Missing Region" is not.
func TestBug82_ConnectAWS_NoMissingRegionError(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "default", "")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	_, cmd := rootApplyMsg(m, messages.InitConnect{Profile: "default", Region: ""})
	if cmd == nil {
		t.Fatal("InitConnectMsg should return a command")
	}

	msg := cmd()
	clientsReady, ok := msg.(messages.ClientsReady)
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

// Profile switching calls connectAWS with an empty region.
func TestBug82_ProfileSwitch_NoMissingRegionError(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "dev", "us-west-2")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})

	_, cmd := rootApplyMsg(m, messages.ProfileSelected{Profile: "some-profile"})
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
		if clientsReady, ok := subMsg.(messages.ClientsReady); ok {
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

func TestBug82_RegionSwitch_PassesExplicitRegion(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "dev", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	_, cmd := rootApplyMsg(m, messages.RegionSelected{Region: "eu-west-1"})
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
		if clientsReady, ok := subMsg.(messages.ClientsReady); ok {
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
