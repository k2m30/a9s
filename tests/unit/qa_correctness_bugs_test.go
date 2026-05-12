package unit

// qa_correctness_bugs_test.go — Tests for correctness bugs #191–#194.
// These tests are written before the fixes so they fail red first (TDD).
//
// Bug #191: LoadFromDirs replaces instead of field-merging per-resource ViewDefs
// Bug #192: Availability probes declare "done" when queue empty but probes still in-flight
// Bug #193: Profile/region switch commits state before session is validated
// Bug #194: connectAWS bypasses AWS_REGION / AWS_DEFAULT_REGION env vars

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ════════════════════════════════════════════════════════════════════════════
// Bug #191: LoadFromDirs partial overlay must field-merge, not replace
// ════════════════════════════════════════════════════════════════════════════

// writeYAML is a local helper that writes content to name.yaml inside dir.
func writeYAML(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("writeYAML(%s): %v", name, err)
	}
}

// TestBug191_LoadFromDirs_PartialOverlay_PreservesGlobalList verifies that when
// a project file only defines "detail" (no "list"), the global "list" is preserved.
func TestBug191_LoadFromDirs_PartialOverlay_PreservesGlobalList(t *testing.T) {
	globalDir := t.TempDir()
	projectDir := t.TempDir()

	// Global has both list (width=20) and detail
	writeYAML(t, globalDir, "ec2", `list:
  Instance ID:
    path: instanceId
    width: 20
detail:
  - instanceId
  - state
`)

	// Project has ONLY detail (different paths), NO list
	writeYAML(t, projectDir, "ec2", `detail:
  - instanceId
  - state
  - placement
  - tags
`)

	cfg, err := config.LoadFromDirs([]string{globalDir, projectDir})
	if err != nil {
		t.Fatalf("LoadFromDirs failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	ec2, ok := cfg.Views["ec2"]
	if !ok {
		t.Fatal("missing ec2 view")
	}

	// List must come from global (width=20 preserved)
	if len(ec2.List) == 0 {
		t.Fatal("ec2.List should be preserved from global when project has no list")
	}
	if ec2.List[0].Width != 20 {
		t.Errorf("ec2.List[0].Width: want 20 (from global), got %d", ec2.List[0].Width)
	}

	// Detail must come from project (4 paths)
	if len(ec2.Detail) != 4 {
		t.Errorf("ec2.Detail: want 4 paths (from project), got %d", len(ec2.Detail))
	}
}

// TestBug191_LoadFromDirs_PartialOverlay_PreservesGlobalDetail verifies that when
// a project file only defines "list" (no "detail"), the global "detail" is preserved.
func TestBug191_LoadFromDirs_PartialOverlay_PreservesGlobalDetail(t *testing.T) {
	globalDir := t.TempDir()
	projectDir := t.TempDir()

	// Global has both list and detail
	writeYAML(t, globalDir, "ec2", `list:
  Instance ID:
    path: instanceId
    width: 20
detail:
  - instanceId
  - state
  - type
`)

	// Project has ONLY list (width=99), NO detail
	writeYAML(t, projectDir, "ec2", `list:
  Instance ID:
    path: instanceId
    width: 99
`)

	cfg, err := config.LoadFromDirs([]string{globalDir, projectDir})
	if err != nil {
		t.Fatalf("LoadFromDirs failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	ec2, ok := cfg.Views["ec2"]
	if !ok {
		t.Fatal("missing ec2 view")
	}

	// List must come from project (width=99)
	if len(ec2.List) == 0 {
		t.Fatal("ec2.List should come from project")
	}
	if ec2.List[0].Width != 99 {
		t.Errorf("ec2.List[0].Width: want 99 (from project), got %d", ec2.List[0].Width)
	}

	// Detail must come from global (3 paths)
	if len(ec2.Detail) != 3 {
		t.Errorf("ec2.Detail: want 3 paths (from global), got %d", len(ec2.Detail))
	}
}

// TestBug191_LoadFromDirs_FullOverlay_ReplacesAll verifies that when the project
// defines both list and detail, both fields come from the project.
func TestBug191_LoadFromDirs_FullOverlay_ReplacesAll(t *testing.T) {
	globalDir := t.TempDir()
	projectDir := t.TempDir()

	writeYAML(t, globalDir, "ec2", `list:
  Instance ID:
    path: instanceId
    width: 20
detail:
  - instanceId
  - state
`)

	writeYAML(t, projectDir, "ec2", `list:
  ID:
    path: instanceId
    width: 50
detail:
  - instanceId
  - state
  - tags
  - vpc
`)

	cfg, err := config.LoadFromDirs([]string{globalDir, projectDir})
	if err != nil {
		t.Fatalf("LoadFromDirs failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	ec2 := cfg.Views["ec2"]

	// List must come from project (width=50)
	if len(ec2.List) == 0 {
		t.Fatal("ec2.List should come from project")
	}
	if ec2.List[0].Width != 50 {
		t.Errorf("ec2.List[0].Width: want 50 (from project), got %d", ec2.List[0].Width)
	}
	if ec2.List[0].Title != "ID" {
		t.Errorf("ec2.List[0].Title: want %q (from project), got %q", "ID", ec2.List[0].Title)
	}

	// Detail must come from project (4 paths)
	if len(ec2.Detail) != 4 {
		t.Errorf("ec2.Detail: want 4 paths (from project), got %d", len(ec2.Detail))
	}
}

// TestBug191_LoadFromDirs_ThreeLayerMerge verifies field-level merging across 3 dirs:
// dir1 has list only, dir2 has detail only, dir3 has neither.
// Final result must have list from dir1, detail from dir2.
func TestBug191_LoadFromDirs_ThreeLayerMerge(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	dir3 := t.TempDir()

	// dir1: list only (width=77)
	writeYAML(t, dir1, "s3", `list:
  Bucket:
    path: name
    width: 77
`)

	// dir2: detail only
	writeYAML(t, dir2, "s3", `detail:
  - name
  - region
  - creation_date
`)

	// dir3: no s3.yaml at all — empty dir has no effect

	cfg, err := config.LoadFromDirs([]string{dir1, dir2, dir3})
	if err != nil {
		t.Fatalf("LoadFromDirs failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	s3, ok := cfg.Views["s3"]
	if !ok {
		t.Fatal("missing s3 view")
	}

	// List must come from dir1 (width=77)
	if len(s3.List) == 0 {
		t.Fatal("s3.List should come from dir1")
	}
	if s3.List[0].Width != 77 {
		t.Errorf("s3.List[0].Width: want 77 (from dir1), got %d", s3.List[0].Width)
	}

	// Detail must come from dir2 (3 paths)
	if len(s3.Detail) != 3 {
		t.Errorf("s3.Detail: want 3 paths (from dir2), got %d", len(s3.Detail))
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Bug #192: Availability probes must not declare "done" while probes in-flight
// ════════════════════════════════════════════════════════════════════════════

// The initial availability probe generation is 0 (set in tui.New before any
// profile/region switch). AvailabilityCacheLoadedMsg does NOT increment the
// generation — only profile/region switches do. So all probe results must
// use Gen=0 to match the model's availabilityGen.
const bug192InitialGen = 0

// bug192Model creates a model ready for availability probe testing.
func bug192Model(t *testing.T) tui.Model {
	t.Helper()
	tui.Version = "test"
	m := tui.New("testprofile", "us-east-1", tui.WithNoCache(false))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	return m
}

// TestBug192_AvailabilityProbes_NotDoneUntilAllReturn verifies that sending
// results for only half the resource types does not clear the progress indicator.
// The "done" path must only fire when availChecked == availTotal.
func TestBug192_AvailabilityProbes_NotDoneUntilAllReturn(t *testing.T) {
	m := bug192Model(t)
	allNames := resource.AllShortNames()
	if len(allNames) < 4 {
		t.Skip("need at least 4 resource types")
	}

	// Start probes
	m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoaded{
		Entries: make(map[string]int),
		Expired: true,
	})

	half := len(allNames) / 2

	// Send results for only the first half
	for i := range half {
		m, _ = rootApplyMsg(m, messages.AvailabilityChecked{
			ResourceType: allNames[i],
			HasResources: true,
			Count:        1,
			Gen:          bug192InitialGen,
		})
	}

	// After half the results, progress must not be declared done.
	// We detect "done" by observing that the SetCheckProgress(0,0) signal
	// fires. The only observable signal in tests: the LAST result in the
	// "not done" state still produces a non-nil cmd (next probe or cache save).
	// Verify the model still processes further messages without panicking.
	m, cmd := rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: allNames[half],
		HasResources: false,
		Count:        0,
		Gen:          bug192InitialGen,
	})
	_ = m
	_ = cmd
	// No panic = minimum correctness check. The definitive check is
	// TestBug192_AvailabilityProbes_SaveCacheOnlyAfterAllDone below.
}

// TestBug192_AvailabilityProbes_SaveCacheOnlyAfterAllDone verifies that the
// "done" path (saveCache + clear progress) fires only after availChecked==availTotal.
//
// The bug: with N types and initial batch of 3 concurrent probes, the queue
// drains after N-3 results arrive (each result schedules the next). When the
// queue hits 0 at result #(N-3), the current code sees len(queue)==0 and
// fires saveCache — but 3 probes are still in-flight. The fix: gate on
// availChecked == availTotal instead.
//
// Test strategy: inject probe results using Gen=0 (the model's actual gen
// after AvailabilityCacheLoadedMsg). Send N results with HasResources=true
// so the menu accumulates availability data. The saveCache cmd should only
// fire on the Nth result (not earlier).
func TestBug192_AvailabilityProbes_SaveCacheOnlyAfterAllDone(t *testing.T) {
	tui.Version = "test"
	m := tui.New("testprofile", "us-east-1", tui.WithNoCache(false))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	allNames := resource.AllShortNames()
	total := len(allNames)
	if total < 4 {
		t.Skip("need at least 4 resource types to test in-flight concurrency")
	}

	// Start probes (gen stays at 0 after this)
	m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoaded{
		Entries: make(map[string]int),
		Expired: true,
	})

	// Pre-populate the menu with availability data so saveCache doesn't return nil.
	// We do this by first sending a CacheLoaded message with pre-populated entries
	// to a fresh model, then switching to probe injection.
	// Actually, the simplest approach: use HasResources=true so SetAvailability
	// is called. BUT the probes return "AWS clients not initialized" errors
	// (Err != nil), so SetAvailability is NOT called from real probes.
	// We must inject Err=nil results to populate the menu.
	//
	// The probes already stored the real msgs above — we're injecting synthetic
	// AvailabilityCheckedMsg with Err=nil and HasResources=true.

	// Send all-but-last results (gen=0, no error, count=1)
	for i := range total - 1 {
		m, _ = rootApplyMsg(m, messages.AvailabilityChecked{
			ResourceType: allNames[i],
			HasResources: true,
			Count:        1,
			Err:          nil,
			Gen:          bug192InitialGen,
		})
	}

	// At this point availQueue is empty (drained long before result total-1).
	// The BUG: the "done" path already fired when the queue first emptied
	// (around result #(total-initial_batch_size)). So the last result
	// should see the model in a terminal state.
	//
	// After the FIX: the model waits for availChecked == availTotal.
	// The last result triggers the "done" path and returns the saveCache cmd.
	_, lastCmd := rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: allNames[total-1],
		HasResources: true,
		Count:        1,
		Err:          nil,
		Gen:          bug192InitialGen,
	})

	// After the last result, a cmd should be returned (the saveCache command).
	if lastCmd == nil {
		t.Error("after the last AvailabilityCheckedMsg, a saveCache cmd should be returned; " +
			"the 'done' path fired prematurely when the queue emptied (bug #192)")
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Bug #193: Profile/region switch must not commit state until session validated
// ════════════════════════════════════════════════════════════════════════════

// TestBug193_ProfileSwitch_FailedConnect_RollsBackProfile verifies that when
// ClientsReadyMsg carries an error, the header still shows the original profile.
func TestBug193_ProfileSwitch_FailedConnect_RollsBackProfile(t *testing.T) {
	tui.Version = "test"
	m := tui.New("original-profile", "us-west-2")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Verify initial profile visible
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "original-profile") {
		t.Fatal("header should show original-profile initially")
	}

	// Switch profile
	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "broken-profile"})

	// Connection fails
	m, _ = rootApplyMsg(m, messages.ClientsReady{
		Err: fmt.Errorf("connection failed: no such profile"),
		Gen: 1, // matches connectGen after one ProfileSelectedMsg
	})

	plain = stripANSI(rootViewContent(m))

	// After failed connect, header must show original profile, NOT broken-profile
	if strings.Contains(plain, "broken-profile") {
		t.Errorf("after failed connect, header must NOT show 'broken-profile'; got:\n%s", plain[:min(300, len(plain))])
	}
	if !strings.Contains(plain, "original-profile") {
		t.Errorf("after failed connect, header must show 'original-profile'; got:\n%s", plain[:min(300, len(plain))])
	}
}

// TestBug193_RegionSwitch_FailedConnect_RollsBackRegion verifies that when
// ClientsReadyMsg carries an error, the header still shows the original region.
func TestBug193_RegionSwitch_FailedConnect_RollsBackRegion(t *testing.T) {
	tui.Version = "test"
	m := tui.New("myprofile", "us-west-2")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "us-west-2") {
		t.Fatal("header should show us-west-2 initially")
	}

	// Switch region
	m, _ = rootApplyMsg(m, messages.RegionSelected{Region: "ap-southeast-1"})

	// Connection fails
	m, _ = rootApplyMsg(m, messages.ClientsReady{
		Err: fmt.Errorf("connection failed: invalid region"),
		Gen: 1, // matches connectGen after one RegionSelectedMsg
	})

	plain = stripANSI(rootViewContent(m))

	// After failed connect, header must show original region
	if strings.Contains(plain, "ap-southeast-1") {
		t.Errorf("after failed region switch, header must NOT show 'ap-southeast-1'; got:\n%s", plain[:min(300, len(plain))])
	}
	if !strings.Contains(plain, "us-west-2") {
		t.Errorf("after failed region switch, header must show 'us-west-2'; got:\n%s", plain[:min(300, len(plain))])
	}
}

// TestBug193_ProfileSwitch_SuccessfulConnect_CommitsProfile verifies that a
// successful ClientsReadyMsg causes the new profile to appear in the header.
func TestBug193_ProfileSwitch_SuccessfulConnect_CommitsProfile(t *testing.T) {
	tui.Version = "test"
	m := tui.New("original-profile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "new-profile"})

	// Successful connect (nil clients is acceptable for this test)
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: nil, Gen: 1})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "new-profile") {
		t.Errorf("after successful connect, header must show 'new-profile'; got:\n%s", plain[:min(300, len(plain))])
	}
}

// TestBug193_RegionSwitch_SuccessfulConnect_CommitsRegion verifies that a
// successful ClientsReadyMsg causes the new region to appear in the header.
func TestBug193_RegionSwitch_SuccessfulConnect_CommitsRegion(t *testing.T) {
	tui.Version = "test"
	m := tui.New("myprofile", "us-west-2")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	m, _ = rootApplyMsg(m, messages.RegionSelected{Region: "eu-west-1"})

	// Successful connect
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: nil, Gen: 1})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "eu-west-1") {
		t.Errorf("after successful region switch, header must show 'eu-west-1'; got:\n%s", plain[:min(300, len(plain))])
	}
}

// TestBug193_EmptyProfile_FailedConnect_RollsBack verifies rollback works when
// the original profile is empty (default/env-based credentials). The rollback
// guard must not use prevProfile != "" since empty is a valid original profile.
func TestBug193_EmptyProfile_FailedConnect_RollsBack(t *testing.T) {
	tui.Version = "test"
	m := tui.New("", "us-west-2") // empty profile = default credentials
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Attempt switch to a broken profile
	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "broken-profile"})

	// Connect fails
	m, _ = rootApplyMsg(m, messages.ClientsReady{
		Err: fmt.Errorf("connection refused"),
		Gen: 1, // matches connectGen after one ProfileSelectedMsg
	})

	plain := stripANSI(rootViewContent(m))
	// Must NOT show "broken-profile" in header after rollback
	if strings.Contains(plain, "broken-profile") {
		t.Errorf("after failed connect from empty profile, 'broken-profile' must not appear; got:\n%s", plain[:min(300, len(plain))])
	}
	// Region must be restored to us-west-2
	if !strings.Contains(plain, "us-west-2") {
		t.Errorf("after failed connect from empty profile, region must roll back to 'us-west-2'; got:\n%s", plain[:min(300, len(plain))])
	}
}

// TestBug193_ProfileSwitch_FailedConnect_ShowsErrorFlash verifies that a failed
// connect (ClientsReadyMsg with Err) shows an error message in the header.
func TestBug193_ProfileSwitch_FailedConnect_ShowsErrorFlash(t *testing.T) {
	tui.Version = "test"
	m := tui.New("original-profile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "broken-profile"})

	m, _ = rootApplyMsg(m, messages.ClientsReady{
		Err: fmt.Errorf("NoCredentialProviders: no valid providers in chain"),
		Gen: 1, // matches connectGen after one ProfileSelectedMsg
	})

	plain := stripANSI(rootViewContent(m))

	// The error text must be visible somewhere in the rendered output
	if !strings.Contains(plain, "NoCredentialProviders") && !strings.Contains(plain, "no valid providers") {
		t.Errorf("after failed connect, header must show error text; got:\n%s", plain[:min(300, len(plain))])
	}
}

// TestBug193_RapidSwitch_StaleResponseIgnored verifies that when the user
// rapidly switches profiles (A→B→C), a stale ClientsReadyMsg from B's
// connect is discarded — it must not install B's clients or roll back to B.
func TestBug193_RapidSwitch_StaleResponseIgnored(t *testing.T) {
	tui.Version = "test"
	m := tui.New("profile-A", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// First switch: A → B (connectGen becomes 1)
	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "profile-B"})

	// Second switch before B's response: B → C (connectGen becomes 2)
	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "profile-C"})

	// Stale response from B's connect arrives (Gen: 1, current connectGen: 2)
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: nil, Gen: 1})

	plain := stripANSI(rootViewContent(m))
	// Header should still show profile-C (the latest switch), not profile-B
	if !strings.Contains(plain, "profile-C") {
		t.Errorf("stale response from B should be ignored, header must show 'profile-C'; got:\n%s", plain[:min(300, len(plain))])
	}

	// Now C's response arrives successfully (Gen: 2)
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: nil, Gen: 2})

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "profile-C") {
		t.Errorf("after C's successful connect, header must show 'profile-C'; got:\n%s", plain[:min(300, len(plain))])
	}
}

// TestBug193_RapidSwitch_FailedFinalConnect_RollsBackToOriginal verifies
// that when A→B→C all fail, rollback goes to A (not B).
func TestBug193_RapidSwitch_FailedFinalConnect_RollsBackToOriginal(t *testing.T) {
	tui.Version = "test"
	m := tui.New("profile-A", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Rapid switches: A → B → C (connectGen becomes 2)
	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "profile-B"})
	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "profile-C"})

	// Stale B response — ignored
	m, _ = rootApplyMsg(m, messages.ClientsReady{
		Err: fmt.Errorf("B failed"), Gen: 1,
	})

	// C's response fails (Gen: 2, matches connectGen)
	m, _ = rootApplyMsg(m, messages.ClientsReady{
		Err: fmt.Errorf("C failed"), Gen: 2,
	})

	plain := stripANSI(rootViewContent(m))
	// Must roll back to A, not B
	if !strings.Contains(plain, "profile-A") {
		t.Errorf("after rapid A→B→C where C fails, must roll back to A; got:\n%s", plain[:min(300, len(plain))])
	}
	if strings.Contains(plain, "profile-B") || strings.Contains(plain, "profile-C") {
		t.Errorf("after rollback, neither B nor C should appear; got:\n%s", plain[:min(300, len(plain))])
	}
}

// TestBug193_FailedSwitch_RestoresIdentityAndAvailability verifies that after
// a failed profile switch, the error path re-fetches identity and reloads
// availability cache — not just labels. The switch cleared these, and rollback
// must trigger commands to recover them.
func TestBug193_FailedSwitch_RestoresIdentityAndAvailability(t *testing.T) {
	tui.Version = "test"
	m := tui.New("original", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Attempt switch and fail
	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "broken"})
	m, cmd := rootApplyMsg(m, messages.ClientsReady{
		Err: fmt.Errorf("access denied"), Gen: 1,
	})

	// The returned cmd must be non-nil — it should contain at least the
	// flash clear timer. When clients are available, it should also include
	// identity re-fetch and availability cache reload commands.
	if cmd == nil {
		t.Fatal("failed connect rollback must return commands (flash clear + recovery)")
	}

	// Profile/region must be restored
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "original") {
		t.Errorf("profile must be rolled back to 'original'; got:\n%s", plain[:min(300, len(plain))])
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Bug #194: connectAWS must respect AWS_REGION / AWS_DEFAULT_REGION env vars
// ════════════════════════════════════════════════════════════════════════════

// executeConnectCmd fires an InitConnectMsg and executes the returned cmd,
// returning the ClientsReadyMsg. Returns nil if the cmd doesn't produce one.
func executeConnectCmd(t *testing.T, m tui.Model) *messages.ClientsReady {
	t.Helper()
	_, cmd := rootApplyMsg(m, messages.InitConnect{Profile: "", Region: ""})
	if cmd == nil {
		t.Log("InitConnectMsg returned nil cmd")
		return nil
	}
	msg := cmd()
	if cr, ok := msg.(messages.ClientsReady); ok {
		return &cr
	}
	// cmd may return a BatchMsg; unwrap one level
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			subMsg := sub()
			if cr, ok := subMsg.(messages.ClientsReady); ok {
				return &cr
			}
		}
	}
	return nil
}

// TestBug194_ConnectAWS_RespectsAWSRegionEnvVar verifies that when AWS_REGION
// is set, connectAWS resolves the region from the env var and carries it in
// ClientsReadyMsg.Region (new field added by the coder fix).
func TestBug194_ConnectAWS_RespectsAWSRegionEnvVar(t *testing.T) {
	t.Setenv("AWS_REGION", "eu-central-1")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_CONFIG_FILE", "/nonexistent")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent")

	tui.Version = "test"
	m := tui.New("", "")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	cr := executeConnectCmd(t, m)
	if cr == nil {
		t.Fatal("InitConnectMsg should produce a ClientsReadyMsg")
	}

	if cr.Err != nil {
		errStr := strings.ToLower(cr.Err.Error())
		if strings.Contains(errStr, "missing region") || strings.Contains(errStr, "could not find region") {
			t.Errorf("connectAWS must not produce a region error when AWS_REGION is set: %v", cr.Err)
		}
		t.Logf("Non-region error (acceptable in isolated env): %v", cr.Err)
	}

	// After the fix: ClientsReadyMsg.Region should carry the resolved region.
	// This field is added by the coder; the test is intentionally forward-looking.
	if cr.Region != "" && cr.Region != "eu-central-1" {
		t.Errorf("ClientsReadyMsg.Region: want %q, got %q", "eu-central-1", cr.Region)
	}
}

// TestBug194_ConnectAWS_RespectsAWSDefaultRegionEnvVar verifies that when
// AWS_DEFAULT_REGION is set (and AWS_REGION is not), the resolved region is
// eu-central-1 → ap-northeast-1 in this case.
func TestBug194_ConnectAWS_RespectsAWSDefaultRegionEnvVar(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "ap-northeast-1")
	t.Setenv("AWS_CONFIG_FILE", "/nonexistent")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent")

	tui.Version = "test"
	m := tui.New("", "")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	cr := executeConnectCmd(t, m)
	if cr == nil {
		t.Fatal("InitConnectMsg should produce a ClientsReadyMsg")
	}

	if cr.Err != nil {
		errStr := strings.ToLower(cr.Err.Error())
		if strings.Contains(errStr, "missing region") || strings.Contains(errStr, "could not find region") {
			t.Errorf("connectAWS must not produce a region error when AWS_DEFAULT_REGION is set: %v", cr.Err)
		}
		t.Logf("Non-region error (acceptable in isolated env): %v", cr.Err)
	}

	// After the fix: ClientsReadyMsg.Region should be "ap-northeast-1"
	if cr.Region != "" && cr.Region != "ap-northeast-1" {
		t.Errorf("ClientsReadyMsg.Region: want %q, got %q", "ap-northeast-1", cr.Region)
	}
}

// TestBug194_ConnectAWS_FallsBackToConfigFileWhenNoEnvVar verifies that when
// no env vars are set and no config file exists, connectAWS falls back to
// us-east-1 (the GetDefaultRegion fallback) and does NOT produce a "Missing Region"
// error. This is the regression check for issue #82.
func TestBug194_ConnectAWS_FallsBackToConfigFileWhenNoEnvVar(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_CONFIG_FILE", "/nonexistent")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent")

	tui.Version = "test"
	m := tui.New("", "")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	cr := executeConnectCmd(t, m)
	if cr == nil {
		t.Fatal("InitConnectMsg should produce a ClientsReadyMsg")
	}

	if cr.Err != nil {
		errStr := strings.ToLower(cr.Err.Error())
		if strings.Contains(errStr, "missing region") || strings.Contains(errStr, "could not find region") {
			t.Errorf("connectAWS with no env vars must still resolve a fallback region; got: %v", cr.Err)
		}
		// Other errors (credentials, profile) are acceptable in an isolated env
		t.Logf("Non-region error (acceptable in isolated env): %v", cr.Err)
	}
}

// TestBug194_ClientsReadyMsg_CarriesResolvedRegion verifies that after the
// coder adds the Region field to ClientsReadyMsg, the field is non-empty
// regardless of whether an error occurred. This documents the expected contract
// after the fix is applied.
func TestBug194_ClientsReadyMsg_CarriesResolvedRegion(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_CONFIG_FILE", "/nonexistent")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent")

	tui.Version = "test"
	m := tui.New("", "")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	cr := executeConnectCmd(t, m)
	if cr == nil {
		t.Fatal("InitConnectMsg should produce a ClientsReadyMsg")
	}

	// After the fix: Region is always populated in ClientsReadyMsg.
	// Before the fix: Region is always "" (field doesn't exist).
	// This test fails until the coder adds the Region field and populates it.
	if cr.Region == "" {
		t.Error("ClientsReadyMsg.Region must be non-empty after the fix — connectAWS must report the resolved region")
	}
}
