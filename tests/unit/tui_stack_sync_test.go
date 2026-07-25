// tui_stack_sync_test.go — parity pins for the rendererState / controller
// stack-sync convergence.
//
// The TUI keeps its own rendererState stack (m.stack, unexported) alongside
// the headless app.Controller's own screen stack (m.ctrl, unexported). This
// file hardens the invariant that the two never diverge in DEPTH or SCREEN
// IDENTITY across every push/pop site (pushScreen, popRS/popRSOnly/
// popRSWithCtrlPop, and the applyIntents/applyIntent PushScreen/PopScreen/
// PopSelectorIntent cases).
//
// Oracle: internal/tui/app_stack_invariant.go's exported Model.StackInSync()
// (backed by core/app/snapshot.go's Controller.ScreenIDs()) is the
// debug-buildable invariant check — asserted after EVERY step below.
// StackInSync groups some ScreenIDs under one rsKind (list: ScreenResourceList
// or ScreenChildList; text: ScreenYAML or ScreenJSON; selector: any of the
// three selector flavors), so a kind-swap within one of those groups (e.g.
// YAML rendererState paired with a controller ScreenJSON) would NOT trip
// StackInSync alone. Each test therefore ALSO asserts on rendered View()
// content (frame title / body markers) to catch that narrower class of
// desync, matching the harness precedent in tui_detail_parity_test.go /
// qa_root_command_test.go.
package unit

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// assertStackInSync fails the test immediately if m.StackInSync() is false,
// naming the step for context.
func assertStackInSync(t *testing.T, m interface{ StackInSync() bool }, step string) {
	t.Helper()
	if !m.StackInSync() {
		t.Fatalf("%s: StackInSync() = false — rendererState stack diverged from controller screen stack", step)
	}
}

// stackSyncFirstNonEmptyContaining returns the first line of s containing
// substr, or "" if none matches. Used to isolate the frame-title line from
// footer-hint text that may contain the same words (e.g. detail's "y YAML"
// keybinding hint vs. an actual YAML-screen frame title).
func stackSyncFirstNonEmptyContaining(s, substr string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, substr) {
			return line
		}
	}
	return ""
}

// ─────────────────────────────────────────────────────────────────────────
// 1. StackSync_NavigationRoundTrip
// ─────────────────────────────────────────────────────────────────────────

// TestStackSync_NavigationRoundTrip drives menu -> list -> detail -> yaml ->
// esc -> esc -> esc back to menu, asserting StackInSync() AND rendered
// content correctness after EVERY step.
func TestStackSync_NavigationRoundTrip(t *testing.T) {
	withTuiVersion(t, "1.0.0")
	m := newRootSizedModel()

	// Step 0: main menu.
	assertStackInSync(t, m, "step 0 (menu)")
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Fatalf("step 0 (menu): expected 'resource-types' in view, got:\n%s", plain)
	}

	// Step 1: menu -> list (ec2).
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	assertStackInSync(t, m, "step 1 (list)")
	plain = stripANSI(rootViewContent(m))
	if strings.Contains(plain, "resource-types") {
		t.Errorf("step 1 (list): stale menu content 'resource-types' still present:\n%s", plain)
	}
	if !strings.Contains(plain, "ec2") {
		t.Errorf("step 1 (list): expected 'ec2' frame/content, got:\n%s", plain)
	}

	// Step 2: list -> detail.
	res := &resource.Resource{
		ID:   "i-stacksync01",
		Name: "stacksync-instance",
		Type: "ec2",
		Fields: map[string]string{
			"instance_id": "i-stacksync01",
			"state":       "running",
		},
	}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     res,
		ResourceType: "ec2",
	})
	assertStackInSync(t, m, "step 2 (detail)")
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "i-stacksync01") && !strings.Contains(plain, "stacksync-instance") {
		t.Errorf("step 2 (detail): expected resource id/name in view, got:\n%s", plain)
	}

	// Step 3: detail -> yaml.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetYAML,
		Resource:     res,
		ResourceType: "ec2",
	})
	assertStackInSync(t, m, "step 3 (yaml)")
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(strings.ToLower(plain), "yaml") {
		t.Errorf("step 3 (yaml): expected 'yaml' in frame title, got:\n%s", plain)
	}

	// Step 4: esc -> back to detail.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	assertStackInSync(t, m, "step 4 (esc->detail)")
	plain = stripANSI(rootViewContent(m))
	// Check the FRAME TITLE line specifically (not the whole view) — the
	// detail footer legitimately contains the "y YAML" keybinding hint, so a
	// substring match against the whole view would false-positive here.
	frameTitleLine := stackSyncFirstNonEmptyContaining(plain, "┌")
	if strings.Contains(strings.ToLower(frameTitleLine), "yaml") {
		t.Errorf("step 4 (esc->detail): stale 'yaml' frame title still present:\n%s", plain)
	}
	if !strings.Contains(plain, "i-stacksync01") && !strings.Contains(plain, "stacksync-instance") {
		t.Errorf("step 4 (esc->detail): expected resource id/name back in view, got:\n%s", plain)
	}

	// Step 5: esc -> back to list.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	assertStackInSync(t, m, "step 5 (esc->list)")
	plain = stripANSI(rootViewContent(m))
	if strings.Contains(plain, "i-stacksync01") || strings.Contains(plain, "stacksync-instance") {
		t.Errorf("step 5 (esc->list): stale detail content still present:\n%s", plain)
	}

	// Step 6: esc -> back to menu.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	assertStackInSync(t, m, "step 6 (esc->menu)")
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("step 6 (esc->menu): expected 'resource-types' back in view, got:\n%s", plain)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// 2. StackSync_SelectorFlow
// ─────────────────────────────────────────────────────────────────────────

// TestStackSync_SelectorFlow drives: open profile selector, esc-dismiss (back
// to menu); open theme selector, confirm via the real
// ThemeSelected->ThemeFileRead round trip (PopSelectorIntent path). Asserts
// StackInSync() and rendered content after each step, catching a desync
// where PopSelectorIntent's rendererState-only pop leaves a stale selector rs
// behind or double-pops past the menu.
func TestStackSync_SelectorFlow(t *testing.T) {
	withTuiVersion(t, "test")
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	cfgPath := writeAWSConfig(t, []string{"default", "staging", "prod"})
	t.Setenv("AWS_CONFIG_FILE", cfgPath)

	// NavigateKindPushTheme (runtime_adapter_navigate.go) lists
	// <A9S_CONFIG_FOLDER>/themes/*.yaml from disk before pushing the selector
	// — an empty/missing dir flashes "No theme files found" instead of
	// pushing, so seed one real theme file.
	themesDir := tmp + "/themes"
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(themes): %v", err)
	}
	if err := os.WriteFile(themesDir+"/stacksync-theme.yaml", []byte("name: stacksync-theme\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(theme seed): %v", err)
	}

	m := newRootSizedModel()
	assertStackInSync(t, m, "baseline")
	baseline := stripANSI(rootViewContent(m))
	if !strings.Contains(baseline, "resource-types") {
		t.Fatalf("baseline: expected main menu, got:\n%s", baseline)
	}

	// --- Profile selector: open then esc-dismiss. ---
	var fetchCmd tea.Cmd
	m, fetchCmd = rootApplyMsg(m, messages.Navigate{Target: messages.TargetProfile})
	if fetchCmd == nil {
		t.Fatal("NavigateMsg{TargetProfile} should return a cmd (fetchProfiles)")
	}
	loadedMsg := fetchCmd()
	if _, isFlash := loadedMsg.(messages.Flash); isFlash {
		t.Fatalf("fetchProfiles returned Flash — config file malformed: %v", loadedMsg)
	}
	m, _ = rootApplyMsg(m, loadedMsg)
	assertStackInSync(t, m, "profile selector opened")

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "staging") && !strings.Contains(plain, "prod") {
		t.Fatalf("profile selector: expected profile names in view, got:\n%s", plain)
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	assertStackInSync(t, m, "profile selector esc-dismissed")
	plain = stripANSI(rootViewContent(m))
	if strings.Contains(plain, "staging") || strings.Contains(plain, "prod") {
		t.Errorf("after esc-dismiss of profile selector: stale profile list still present:\n%s", plain)
	}
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after esc-dismiss of profile selector: expected main menu, got:\n%s", plain)
	}

	// --- Theme selector: open, then confirm via ThemeSelected -> ThemeFileRead
	// (the real PopSelectorIntent-emitting success path). ---
	// ':theme' + Enter (like ':root' in qa_root_command_test.go) executes
	// command mode and returns a cmd producing messages.Navigate{TargetTheme}
	// — it does not push the selector synchronously in the same Update call,
	// so the returned cmd must be executed and fed back.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: ":"})
	for _, ch := range "theme" {
		m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: string(ch)})
	}
	var themeCommandCmd tea.Cmd
	m, themeCommandCmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if themeCommandCmd == nil {
		t.Fatal(":theme + Enter should return a cmd")
	}
	themeNavMsg := themeCommandCmd()
	m, _ = rootApplyMsg(m, themeNavMsg)
	assertStackInSync(t, m, "theme selector opened")

	plain = stripANSI(rootViewContent(m))
	if strings.Contains(plain, "resource-types") {
		t.Fatalf(":theme should have pushed the theme selector off the menu, still shows menu:\n%s", plain)
	}

	var themeCmd tea.Cmd
	m, themeCmd = rootApplyMsg(m, messages.ThemeSelected{Theme: "stacksync-theme.yaml"})
	if themeCmd == nil {
		t.Fatal("ThemeSelected should return a cmd (read task)")
	}
	readResult := themeCmd()
	tfr, ok := readResult.(messages.ThemeFileRead)
	if !ok {
		t.Fatalf("expected messages.ThemeFileRead from read task, got %T", readResult)
	}
	if tfr.Err != nil {
		t.Fatalf("unexpected ThemeFileRead.Err for synthetic theme name: %v", tfr.Err)
	}
	// Feed back with valid, minimal YAML (all Theme fields are optional
	// pointers) so the success path (ApplyThemeIntent + PopSelectorIntent +
	// FlashIntent + TaskKindSaveThemeConfig) fires.
	tfr.Bytes = []byte("name: stacksync-theme\n")
	m, _ = rootApplyMsg(m, tfr)
	assertStackInSync(t, m, "theme selector confirmed (PopSelectorIntent)")

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after successful theme apply (PopSelectorIntent), expected main menu, got:\n%s", plain)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// 3. StackSync_ChildListFlow
// ─────────────────────────────────────────────────────────────────────────

// TestStackSync_ChildListFlow drives an S3-objects-style EnterChildView
// (ChildListPayload resolution) and backs out via esc, pinning the wave-1
// ChildListPayload fix: the pushed child-list rendererState must show child
// resources (not vanish into a nil/empty ListState), StackInSync() must hold
// at every step, and esc must return to the parent bucket list with the
// parent's content restored, not skip a level.
func TestStackSync_ChildListFlow(t *testing.T) {
	t.Parallel()
	m := newDemoColdCacheApp(t)

	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReady{Clients: clients, Gen: 1})

	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})
	if navCmd == nil {
		t.Fatal("expected a cmd after NavigateMsg{s3}, got nil")
	}
	raw := extractMsg(t, navCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoaded)
		return ok
	})
	loaded := raw.(messages.ResourcesLoaded)
	if len(loaded.Resources) == 0 {
		t.Fatal("fixture data has zero S3 buckets; cannot drill into child view")
	}
	*m, _ = rootApplyMsg(*m, loaded)
	assertStackInSync(t, *m, "s3 bucket list loaded")

	parentPlain := stripANSI(rootViewContent(*m))
	bucketName := loaded.Resources[0].ID

	var childCmd tea.Cmd
	*m, childCmd = rootApplyMsg(*m, messages.EnterChildView{
		ChildType:     "s3_objects",
		ParentContext: map[string]string{"bucket": bucketName, "prefix": ""},
		DisplayName:   bucketName,
	})
	if childCmd == nil {
		t.Fatal("expected a cmd after EnterChildViewMsg{s3_objects}, got nil")
	}
	assertStackInSync(t, *m, "child list pushed (pre-fetch)")
	childRaw := extractMsg(t, childCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoaded)
		return ok
	})
	childLoaded, ok := childRaw.(messages.ResourcesLoaded)
	if !ok {
		t.Fatalf("expected ResourcesLoaded for s3_objects child fetch, got %T", childRaw)
	}
	*m, _ = rootApplyMsg(*m, childLoaded)
	assertStackInSync(t, *m, "child list loaded")

	childPlain := stripANSI(rootViewContent(*m))
	if childPlain == parentPlain {
		t.Fatalf("child-list view is identical to parent bucket-list view — ChildListPayload did not resolve to a distinct rendererState:\n%s", childPlain)
	}
	if !strings.Contains(childPlain, bucketName) {
		t.Errorf("child-list frame should reference the parent bucket %q, got:\n%s", bucketName, childPlain)
	}

	// Back out: esc must return to the PARENT bucket list, not skip past it.
	*m, _ = rootApplyMsg(*m, tea.KeyPressMsg{Code: tea.KeyEscape})
	assertStackInSync(t, *m, "esc back to parent list")
	afterEsc := stripANSI(rootViewContent(*m))
	if afterEsc != parentPlain {
		t.Errorf("esc from child list did not restore parent bucket-list view exactly.\nwant:\n%s\ngot:\n%s", parentPlain, afterEsc)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// 4. StackSync_DoublePopGuard
// ─────────────────────────────────────────────────────────────────────────

// TestStackSync_DoublePopGuard crafts the scenario the double-pop guard in
// internal/tui/runtime_adapter.go's applyIntent (singular) PopSelectorIntent
// case exists to protect: a selector rendererState sits ctrlBacked on top of
// a DEEPER stack (menu -> list -> detail -> theme-selector, 4 levels) when
// HandleThemeFileRead's success path emits PopSelectorIntent. Production
// code forwards the intent to the controller FIRST (m.ctrl.ApplyIntents),
// which pops exactly the top controller screen (the selector), THEN
// popRSOnly removes only the matching rendererState — deliberately NOT
// popRS(), which would additionally call ActionBack and pop the controller
// stack a SECOND time, landing the two stacks on different screens (detail
// popped too, stranding the rendererState stack one level ahead of the
// controller). StackInSync() is the direct assertion for this: a double-pop
// desyncs stack LENGTH (controller now 2 shorter than the renderer stack by
// one net level), which StackInSync's length-mismatch check catches
// immediately, before any per-entry kind comparison.
//
// Reproduced via the real production message path: ThemeSelected ->
// ThemeFileRead (success) is the one production event that emits
// PopSelectorIntent (core/runtime/handlers.go HandleThemeFileRead).
// Asserts the resulting screen is the DETAIL view underneath the selector —
// exactly one level up, not two (which would land on the list instead).
func TestStackSync_DoublePopGuard(t *testing.T) {
	withTuiVersion(t, "1.0.0")
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	themesDir := tmp + "/themes"
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(themes): %v", err)
	}
	if err := os.WriteFile(themesDir+"/doublepop-theme.yaml", []byte("name: doublepop-theme\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(theme seed): %v", err)
	}
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	assertStackInSync(t, m, "list pushed")

	res := &resource.Resource{
		ID:   "i-doublepop01",
		Name: "doublepop-instance",
		Type: "ec2",
		Fields: map[string]string{
			"instance_id": "i-doublepop01",
			"state":       "running",
		},
	}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     res,
		ResourceType: "ec2",
	})
	assertStackInSync(t, m, "detail pushed")
	detailPlain := stripANSI(rootViewContent(m))
	if !strings.Contains(detailPlain, "i-doublepop01") && !strings.Contains(detailPlain, "doublepop-instance") {
		t.Fatalf("detail push did not render the pushed resource, cannot test pop guard:\n%s", detailPlain)
	}

	// Open the theme selector ON TOP of detail (4 levels deep: menu, list,
	// detail, selector) via ':theme' + Enter, mirroring StackSync_SelectorFlow.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: ":"})
	for _, ch := range "theme" {
		m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: string(ch)})
	}
	var themeCommandCmd tea.Cmd
	m, themeCommandCmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if themeCommandCmd == nil {
		t.Fatal(":theme + Enter should return a cmd")
	}
	m, _ = rootApplyMsg(m, themeCommandCmd())
	assertStackInSync(t, m, "theme selector pushed over detail")

	selectorPlain := stripANSI(rootViewContent(m))
	if selectorPlain == detailPlain {
		t.Fatal("theme selector did not push a distinct rendererState over detail — cannot test pop guard")
	}

	// Confirm the theme (real ThemeSelected -> ThemeFileRead success path,
	// which is the ONE production trigger for PopSelectorIntent). This must
	// pop EXACTLY the selector, landing back on detail — not detail AND list.
	var themeCmd tea.Cmd
	m, themeCmd = rootApplyMsg(m, messages.ThemeSelected{Theme: "doublepop-theme.yaml"})
	if themeCmd == nil {
		t.Fatal("ThemeSelected should return a cmd (read task)")
	}
	readResult := themeCmd()
	tfr, ok := readResult.(messages.ThemeFileRead)
	if !ok {
		t.Fatalf("expected messages.ThemeFileRead from read task, got %T", readResult)
	}
	if tfr.Err != nil {
		t.Fatalf("unexpected ThemeFileRead.Err: %v", tfr.Err)
	}
	m, _ = rootApplyMsg(m, tfr)
	assertStackInSync(t, m, "theme confirmed (PopSelectorIntent) — the double-pop guard's own assertion point")

	afterPop := stripANSI(rootViewContent(m))
	if !strings.Contains(afterPop, "i-doublepop01") && !strings.Contains(afterPop, "doublepop-instance") {
		t.Fatalf("after PopSelectorIntent, expected to land back on the detail view underneath the selector — got:\n%s", afterPop)
	}
	if afterPop != detailPlain {
		t.Errorf("after PopSelectorIntent, view does not exactly match the pre-selector detail view (possible double-pop landed elsewhere).\nwant:\n%s\ngot:\n%s", detailPlain, afterPop)
	}

	// A single Escape from here must reach the LIST (depth exactly 3
	// remaining: menu, list, detail) — a prior double-pop would have already
	// stranded us at list or menu, and this esc would then reach one level
	// too far (menu directly, or attempt to pop past an empty stack).
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	assertStackInSync(t, m, "esc after theme confirm")
	afterEsc := stripANSI(rootViewContent(m))
	if strings.Contains(afterEsc, "resource-types") {
		t.Errorf("single esc from detail (post PopSelectorIntent) reached the main menu directly — stack was already short one level (double-pop regression):\n%s", afterEsc)
	}
	if strings.Contains(afterEsc, "i-doublepop01") || strings.Contains(afterEsc, "doublepop-instance") {
		t.Errorf("single esc from detail (post PopSelectorIntent) did not leave the detail view:\n%s", afterEsc)
	}
}
