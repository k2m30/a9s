// A failed AWS connect in the headless lane (Handle's ClientsReady lane and
// BootstrapLive) rolls back and surfaces an error flash exactly like the TUI
// does, and Controller.OpenProfileSelector is the headless entry point for
// opening the profile selector outside a TUI adapter.
package unit_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// writeHeadlessAWSConfig writes a minimal AWS config file with [default] and
// [profile <name>] sections so awsclient.ListProfiles (via
// Core.FetchProfiles) resolves a known profile set. Local to this file —
// tests/unit/app_handlers_theme_profile_test.go's writeAWSConfig lives in
// the internal `unit` package and is not visible from `unit_test`.
func writeHeadlessAWSConfig(t *testing.T, profiles []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	var sb strings.Builder
	for _, name := range profiles {
		if name == "default" {
			sb.WriteString("[default]\nregion = us-east-1\n\n")
		} else {
			sb.WriteString("[profile " + name + "]\nregion = us-east-1\n\n")
		}
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// A failed connect result for a selected profile rolls the session back to the
// prior stable profile and surfaces an error flash, the same outcome the TUI's
// handleClientsReady produces (internal/tui/app_session.go).
func TestHeadless_SelectProfile_FailedConnect_RollsBackAndFlashes(t *testing.T) {
	core, ctrl := newHermeticLiveController(t, "stable-prof", "us-east-1")

	origProfile := ctrl.Snapshot().Header.Profile
	if origProfile != "stable-prof" {
		t.Fatalf("setup: Snapshot().Header.Profile = %q, want %q", origProfile, "stable-prof")
	}

	_, selectTasks := ctrl.Apply(app.Action{Kind: app.ActionSelectProfile, Arg: "broken-profile"})
	if len(selectTasks) == 0 {
		t.Fatal("Apply(ActionSelectProfile) returned 0 tasks, want a TaskKindConnect task")
	}

	// Gen must be fetched AFTER Apply (HandleProfileSelected rotates the
	// session and bumps ConnectGen) — a Gen captured before Apply would make
	// HandleClientsReady's staleness guard silently drop this event.
	gen := core.ConnectGen()
	vs, _ := ctrl.Handle(messages.ClientsReady{
		Err:    errors.New("NoCredentialProviders: no valid providers in chain"),
		Region: "us-east-1",
		Gen:    gen,
	})

	if vs.Header.Profile != origProfile {
		t.Errorf("Handle(ClientsReady error): return-value Header.Profile = %q, want rollback to %q", vs.Header.Profile, origProfile)
	}
	if !vs.Header.Flash.IsError {
		t.Error("Handle(ClientsReady error): return-value Header.Flash.IsError = false, want true")
	}

	snap := ctrl.Snapshot()
	if snap.Header.Profile != origProfile {
		t.Errorf("Handle(ClientsReady error): Snapshot().Header.Profile = %q after failed connect, want rollback to %q", snap.Header.Profile, origProfile)
	}
	if !snap.Header.Flash.IsError {
		t.Error("Handle(ClientsReady error): Snapshot().Header.Flash.IsError = false, want true")
	}
}

// A failed startup connect (the web/headless cold-boot seam) routes through
// Core.HandleClientsReady: the ViewState carries an error flash and the tasks
// include the FlashTick that clears it. "fake-profile-000000000000" is a
// nonexistent profile, so ConnectAWS's shared-config lookup fails synchronously
// with no network I/O.
func TestBootstrapLive_FailedConnect_SurfacesErrorFlash(t *testing.T) {
	profile := "fake-profile-000000000000"
	_, ctrl := newHermeticLiveController(t, profile, "us-east-1")

	tasks := ctrl.BootstrapLive(profile, "us-east-1")

	snap := ctrl.Snapshot()
	if !snap.Header.Flash.IsError {
		t.Errorf("BootstrapLive(%q): Snapshot().Header.Flash.IsError = false, want true", profile)
	}

	hasFlashTick := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.TaskKindFlashTick {
			hasFlashTick = true
			break
		}
	}
	if !hasFlashTick {
		t.Errorf("BootstrapLive(%q) tasks = %v, missing TaskKindFlashTick", profile, taskKindStrings(tasks))
	}
}

func TestOpenProfileSelector_PushesSelectorHeadless(t *testing.T) {
	cfgPath := writeHeadlessAWSConfig(t, []string{"default", "alpha"})
	t.Setenv("AWS_CONFIG_FILE", cfgPath)

	_, ctrl := newHermeticLiveController(t, "default", "us-east-1")

	vs, _ := ctrl.OpenProfileSelector()

	if vs.Body.Kind != app.BodyKindSelector {
		t.Fatalf("OpenProfileSelector: Body.Kind = %q, want %q", vs.Body.Kind, app.BodyKindSelector)
	}
	if vs.Body.Selector == nil {
		t.Fatal("OpenProfileSelector: Body.Selector is nil — profile selector must be visible in the returned ViewState")
	}
	if !slices.Contains(vs.Body.Selector.AllItems, "alpha") {
		t.Errorf("OpenProfileSelector: Body.Selector.AllItems = %v, missing profile %q", vs.Body.Selector.AllItems, "alpha")
	}

	_, tasks := ctrl.Apply(app.Action{Kind: app.ActionSelectProfile, Arg: "alpha"})
	hasConnect := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.TaskKindConnect {
			hasConnect = true
			break
		}
	}
	if !hasConnect {
		t.Errorf("Apply(ActionSelectProfile) after OpenProfileSelector: tasks = %v, missing TaskKindConnect", taskKindStrings(tasks))
	}
}

// OpenProfileSelector applies the TUI's demo-mode guard
// (internal/tui/runtime_adapter_navigate.go's NavigateKindFetchProfiles case)
// with the same error-flash text.
func TestOpenProfileSelector_DemoMode_Blocked(t *testing.T) {
	core, ctrl := newHermeticLiveController(t, "demo", "us-east-1")
	core.SetPreSuppliedClients(demo.NewServiceClients())

	vs, tasks := ctrl.OpenProfileSelector()

	if vs.Body.Kind != app.BodyKindMenu {
		t.Errorf("OpenProfileSelector in demo mode: Body.Kind = %q, want %q (selector must not be pushed)", vs.Body.Kind, app.BodyKindMenu)
	}
	if !vs.Header.Flash.IsError {
		t.Error("OpenProfileSelector in demo mode: Header.Flash.IsError = false, want true")
	}
	wantText := "context switching is disabled in demo mode"
	if vs.Header.Flash.Text != wantText {
		t.Errorf("OpenProfileSelector in demo mode: Header.Flash.Text = %q, want %q", vs.Header.Flash.Text, wantText)
	}
	if tasks != nil {
		t.Errorf("OpenProfileSelector in demo mode: tasks = %v, want nil", tasks)
	}
}

// OpenProfileSelector follows the TUI's fetchProfiles error path
// (internal/tui/fetch_adapter.go).
func TestOpenProfileSelector_FetchError_Flashes(t *testing.T) {
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "missing"))

	_, ctrl := newHermeticLiveController(t, "default", "us-east-1")

	vs, tasks := ctrl.OpenProfileSelector()

	if vs.Body.Kind != app.BodyKindMenu {
		t.Errorf("OpenProfileSelector with unreadable config: Body.Kind = %q, want %q (selector must not be pushed)", vs.Body.Kind, app.BodyKindMenu)
	}
	if !vs.Header.Flash.IsError {
		t.Error("OpenProfileSelector with unreadable config: Header.Flash.IsError = false, want true")
	}
	if tasks != nil {
		t.Errorf("OpenProfileSelector with unreadable config: tasks = %v, want nil", tasks)
	}
}
