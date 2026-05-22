// screens_handlers_test.go — Core-direct unit tests for the five
// view-stack handler ports added in Phase-05 PR-05a-h4-a (AS-650 / AS-769):
//
//	HandleProfilesLoaded — emits PushScreen{ScreenProfileSelector,...}.
//	HandleValueRevealed  — emits PushScreen{ScreenReveal,...} or flash on Err.
//	HandleEnterChildView — emits PushScreen{ScreenChildList,...} + fetch task;
//	                       unknown ChildType flashes an error.
//	HandleThemeSelected  — emits TaskKindReadThemeFile; invalid name flashes.
//	HandleThemeFileRead  — emits Apply/Pop/Flash + Save task on success;
//	                       read failure flashes.
//
// Package runtime (not runtime_test) so the suite can access unexported
// fields when needed; the helpers in handlers_test.go (newCore, findFlashIntent,
// hasTaskKind, …) are reused here.
package runtime

import (
	"errors"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---- helpers ---------------------------------------------------------------

// findPushScreen returns the first PushScreen intent in xs and a bool flag.
func findPushScreen(xs []UIIntent) (PushScreen, bool) {
	for _, x := range xs {
		if ps, ok := x.(PushScreen); ok {
			return ps, true
		}
	}
	return PushScreen{}, false
}

// findApplyTheme returns the first ApplyThemeIntent and a bool flag.
func findApplyTheme(xs []UIIntent) (ApplyThemeIntent, bool) {
	for _, x := range xs {
		if at, ok := x.(ApplyThemeIntent); ok {
			return at, true
		}
	}
	return ApplyThemeIntent{}, false
}

// findFetchChildResources returns the first FetchChildResourcesPayload
// task and a bool flag.
func findFetchChildResources(tasks []TaskRequest) (FetchChildResourcesPayload, bool) {
	for _, t := range tasks {
		if p, ok := t.Payload.(FetchChildResourcesPayload); ok {
			return p, true
		}
	}
	return FetchChildResourcesPayload{}, false
}

func findReadTheme(tasks []TaskRequest) (ReadThemePayload, bool) {
	for _, t := range tasks {
		if p, ok := t.Payload.(ReadThemePayload); ok {
			return p, true
		}
	}
	return ReadThemePayload{}, false
}

func findSaveTheme(tasks []TaskRequest) (SaveThemeConfigPayload, bool) {
	for _, t := range tasks {
		if p, ok := t.Payload.(SaveThemeConfigPayload); ok {
			return p, true
		}
	}
	return SaveThemeConfigPayload{}, false
}

// withTempChildType registers a throwaway child type for the duration of
// the test so HandleEnterChildView's resource.GetChildType validation
// finds something. Cleans up via t.Cleanup.
func withTempChildType(t *testing.T, shortName string) {
	t.Helper()
	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		Name:      "Test " + shortName,
		ShortName: shortName,
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 8}},
	})
	t.Cleanup(func() {
		resource.CleanupChildTypeForTest(shortName)
	})
}

// ---- HandleProfilesLoaded ----------------------------------------------------

func TestCore_HandleProfilesLoaded_EmitsProfileSelectorPushScreen(t *testing.T) {
	c := newCore()
	c.session.Profile = "dev-account"

	intents, tasks := c.HandleProfilesLoaded(ProfilesLoadedEvent{
		Profiles: []string{"default", "dev-account", "prod-account"},
	})

	if len(tasks) != 0 {
		t.Errorf("expected zero tasks, got %d", len(tasks))
	}
	ps, ok := findPushScreen(intents)
	if !ok {
		t.Fatalf("expected PushScreen intent, got intents=%#v", intents)
	}
	if ps.ID != ScreenProfileSelector {
		t.Errorf("expected ScreenID=%s, got %s", ScreenProfileSelector, ps.ID)
	}
	pp, ok := ps.Payload.(ProfileSelectorPayload)
	if !ok {
		t.Fatalf("expected ProfileSelectorPayload, got %T", ps.Payload)
	}
	if len(pp.Profiles) != 3 || pp.Profiles[1] != "dev-account" {
		t.Errorf("unexpected Profiles slice: %v", pp.Profiles)
	}
	if pp.Current != "dev-account" {
		t.Errorf("expected Current=dev-account (from session), got %q", pp.Current)
	}
}

// ---- HandleValueRevealed -----------------------------------------------------

func TestCore_HandleValueRevealed_SuccessEmitsRevealPushScreen(t *testing.T) {
	c := newCore()

	intents, tasks := c.HandleValueRevealed(ValueRevealedEvent{
		ResourceID: "/prod/db/password",
		Value:      "hunter2",
	})

	if len(tasks) != 0 {
		t.Errorf("expected zero tasks on success, got %d", len(tasks))
	}
	ps, ok := findPushScreen(intents)
	if !ok {
		t.Fatalf("expected PushScreen intent, got intents=%#v", intents)
	}
	if ps.ID != ScreenReveal {
		t.Errorf("expected ScreenID=%s, got %s", ScreenReveal, ps.ID)
	}
	rp, ok := ps.Payload.(RevealPayload)
	if !ok {
		t.Fatalf("expected RevealPayload, got %T", ps.Payload)
	}
	if rp.ResourceID != "/prod/db/password" || rp.Value != "hunter2" {
		t.Errorf("payload mismatch: id=%q value=%q", rp.ResourceID, rp.Value)
	}
}

func TestCore_HandleValueRevealed_ErrorEmitsFlash(t *testing.T) {
	c := newCore()

	intents, tasks := c.HandleValueRevealed(ValueRevealedEvent{
		ResourceID: "/secrets/x",
		Err:        errors.New("permission denied"),
	})

	if len(tasks) != 0 {
		t.Errorf("expected zero tasks on error, got %d", len(tasks))
	}
	if _, ok := findPushScreen(intents); ok {
		t.Errorf("expected no PushScreen on error, got one")
	}
	fi, ok := findFlashIntent(intents)
	if !ok {
		t.Fatalf("expected FlashIntent, got intents=%#v", intents)
	}
	if !fi.IsError {
		t.Errorf("expected IsError=true, got false")
	}
	if got, want := fi.Text, "reveal failed: permission denied"; got != want {
		t.Errorf("Text=%q want %q", got, want)
	}
}

// ---- HandleEnterChildView ----------------------------------------------------

func TestCore_HandleEnterChildView_KnownTypeEmitsScreenAndTask(t *testing.T) {
	c := newCore()
	const childType = "h4a_test_child_known"
	withTempChildType(t, childType)

	intents, tasks := c.HandleEnterChildView(EnterChildViewEvent{
		ChildType:     childType,
		ParentContext: map[string]string{"bucket": "my-bucket"},
		DisplayName:   "my-bucket",
	})

	ps, ok := findPushScreen(intents)
	if !ok {
		t.Fatalf("expected PushScreen, got intents=%#v", intents)
	}
	if ps.ID != ScreenChildList {
		t.Errorf("expected ScreenChildList, got %s", ps.ID)
	}
	clp, ok := ps.Payload.(ChildListPayload)
	if !ok {
		t.Fatalf("expected ChildListPayload, got %T", ps.Payload)
	}
	if clp.ChildType != childType || clp.DisplayName != "my-bucket" {
		t.Errorf("payload mismatch: %#v", clp)
	}
	if clp.ParentContext["bucket"] != "my-bucket" {
		t.Errorf("ParentContext lost: %v", clp.ParentContext)
	}

	fp, ok := findFetchChildResources(tasks)
	if !ok {
		t.Fatalf("expected FetchChildResources task, got tasks=%#v", tasks)
	}
	if fp.ChildType != childType {
		t.Errorf("task ChildType=%q want %q", fp.ChildType, childType)
	}
	if fp.ParentContext["bucket"] != "my-bucket" {
		t.Errorf("task ParentContext lost: %v", fp.ParentContext)
	}
	if !hasTaskKind(tasks, TaskKindFetchChildResources) {
		t.Errorf("expected TaskKindFetchChildResources in tasks, got %v", taskKinds(tasks))
	}
}

func TestCore_HandleEnterChildView_UnknownTypeEmitsFlashError(t *testing.T) {
	c := newCore()

	intents, tasks := c.HandleEnterChildView(EnterChildViewEvent{
		ChildType:     "totally-not-registered-h4a",
		ParentContext: map[string]string{},
		DisplayName:   "X",
	})

	if len(tasks) != 0 {
		t.Errorf("expected zero tasks on unknown type, got %d", len(tasks))
	}
	if _, ok := findPushScreen(intents); ok {
		t.Errorf("expected no PushScreen on unknown type, got one")
	}
	fi, ok := findFlashIntent(intents)
	if !ok {
		t.Fatalf("expected FlashIntent, got intents=%#v", intents)
	}
	if !fi.IsError {
		t.Errorf("expected IsError=true")
	}
	if got, want := fi.Text, "unknown child type: totally-not-registered-h4a"; got != want {
		t.Errorf("Text=%q want %q", got, want)
	}
}

// ---- HandleThemeSelected -----------------------------------------------------

func TestCore_HandleThemeSelected_ValidNameEmitsReadTask(t *testing.T) {
	c := newCore()

	intents, tasks := c.HandleThemeSelected(ThemeSelectedEvent{Theme: "tokyo-night.yaml"})

	if len(intents) != 0 {
		t.Errorf("expected zero intents on valid name, got %#v", intents)
	}
	rp, ok := findReadTheme(tasks)
	if !ok {
		t.Fatalf("expected ReadThemePayload task, got tasks=%#v", tasks)
	}
	if rp.Theme != "tokyo-night.yaml" {
		t.Errorf("Theme=%q want tokyo-night.yaml", rp.Theme)
	}
	if !hasTaskKind(tasks, TaskKindReadThemeFile) {
		t.Errorf("expected TaskKindReadThemeFile in tasks, got %v", taskKinds(tasks))
	}
}

func TestCore_HandleThemeSelected_InvalidNameEmitsFlashError(t *testing.T) {
	c := newCore()

	// Empty theme name is rejected by config.ThemePath with a deterministic error.
	intents, tasks := c.HandleThemeSelected(ThemeSelectedEvent{Theme: ""})

	if len(tasks) != 0 {
		t.Errorf("expected zero tasks on invalid name, got %d", len(tasks))
	}
	fi, ok := findFlashIntent(intents)
	if !ok {
		t.Fatalf("expected FlashIntent, got intents=%#v", intents)
	}
	if !fi.IsError {
		t.Errorf("expected IsError=true")
	}
	const prefix = "Invalid theme: "
	if len(fi.Text) < len(prefix) || fi.Text[:len(prefix)] != prefix {
		t.Errorf("Text=%q want prefix %q", fi.Text, prefix)
	}
}

// ---- HandleThemeFileRead -----------------------------------------------------

func TestCore_HandleThemeFileRead_SuccessEmitsApplyPopFlashAndSaveTask(t *testing.T) {
	c := newCore()
	bytes := []byte("name: Custom\ncolors:\n  accent: \"#ffffff\"\n")

	intents, tasks := c.HandleThemeFileRead(ThemeFileReadEvent{
		Theme: "custom.yaml",
		Bytes: bytes,
	})

	at, ok := findApplyTheme(intents)
	if !ok {
		t.Fatalf("expected ApplyThemeIntent, got intents=%#v", intents)
	}
	if at.Name != "custom.yaml" {
		t.Errorf("ApplyTheme Name=%q want custom.yaml", at.Name)
	}
	if len(at.Bytes) != len(bytes) {
		t.Errorf("ApplyTheme carried %d bytes, expected %d", len(at.Bytes), len(bytes))
	}
	if !findPopSelector(intents) {
		t.Errorf("expected PopSelectorIntent in intents, got %#v", intents)
	}
	fi, ok := findFlashIntent(intents)
	if !ok {
		t.Fatalf("expected success FlashIntent, got intents=%#v", intents)
	}
	if fi.IsError {
		t.Errorf("success flash must not be IsError")
	}
	if got, want := fi.Text, "Theme: custom.yaml"; got != want {
		t.Errorf("Text=%q want %q", got, want)
	}

	sp, ok := findSaveTheme(tasks)
	if !ok {
		t.Fatalf("expected SaveThemeConfig task, got tasks=%#v", tasks)
	}
	if sp.Theme != "custom.yaml" {
		t.Errorf("Save Theme=%q want custom.yaml", sp.Theme)
	}
	if !hasTaskKind(tasks, TaskKindSaveThemeConfig) {
		t.Errorf("expected TaskKindSaveThemeConfig in tasks, got %v", taskKinds(tasks))
	}
}

func TestCore_HandleThemeFileRead_ReadErrorEmitsFlashOnly(t *testing.T) {
	c := newCore()

	intents, tasks := c.HandleThemeFileRead(ThemeFileReadEvent{
		Theme: "missing.yaml",
		Err:   errors.New("open ~/.a9s/themes/missing.yaml: no such file or directory"),
	})

	if len(tasks) != 0 {
		t.Errorf("expected zero tasks on read error, got %d", len(tasks))
	}
	if _, ok := findApplyTheme(intents); ok {
		t.Errorf("expected no ApplyThemeIntent on read error")
	}
	if findPopSelector(intents) {
		t.Errorf("expected no PopSelectorIntent on read error")
	}
	fi, ok := findFlashIntent(intents)
	if !ok {
		t.Fatalf("expected FlashIntent, got intents=%#v", intents)
	}
	if !fi.IsError {
		t.Errorf("expected IsError=true on read error")
	}
	const prefix = "Cannot read theme: "
	if len(fi.Text) < len(prefix) || fi.Text[:len(prefix)] != prefix {
		t.Errorf("Text=%q want prefix %q", fi.Text, prefix)
	}
}

// TestCore_HandleThemeFileRead_ParseErrorEmitsFlashOnly pins the AS-784
// invariant: when read succeeds but the YAML cannot be parsed by
// styles.ThemeFromYAML, the handler must emit exactly one error flash and
// MUST NOT emit ApplyThemeIntent, PopSelectorIntent, or any
// TaskKindSaveThemeConfig task. Pre-AS-784 the handler emitted Save
// unconditionally on read success, persisting an invalid theme choice to
// disk even though the adapter rejected the apply.
func TestCore_HandleThemeFileRead_ParseErrorEmitsFlashOnly(t *testing.T) {
	c := newCore()

	// Malformed YAML — the leading `:` token is not a valid YAML node.
	// styles.ThemeFromYAML returns "theme YAML parse error: …".
	badBytes := []byte(":\n  not yaml at all\n: : :\n")

	intents, tasks := c.HandleThemeFileRead(ThemeFileReadEvent{
		Theme: "broken.yaml",
		Bytes: badBytes,
	})

	if len(tasks) != 0 {
		t.Errorf("expected zero tasks on parse error, got %d (%v)", len(tasks), taskKinds(tasks))
	}
	if hasTaskKind(tasks, TaskKindSaveThemeConfig) {
		t.Errorf("expected NO TaskKindSaveThemeConfig on parse error; invalid theme must not be persisted")
	}
	if _, ok := findApplyTheme(intents); ok {
		t.Errorf("expected no ApplyThemeIntent on parse error")
	}
	if findPopSelector(intents) {
		t.Errorf("expected no PopSelectorIntent on parse error — selector stays open so user can retry")
	}
	fi, ok := findFlashIntent(intents)
	if !ok {
		t.Fatalf("expected FlashIntent, got intents=%#v", intents)
	}
	if !fi.IsError {
		t.Errorf("expected IsError=true on parse error, got IsError=false")
	}
	const prefix = "Bad theme YAML: "
	if len(fi.Text) < len(prefix) || fi.Text[:len(prefix)] != prefix {
		t.Errorf("Text=%q want prefix %q", fi.Text, prefix)
	}
}

// TestCore_HandleThemeFileRead_InvalidHexColorEmitsFlashOnly exercises the
// second parse-fail branch styles.ThemeFromYAML guards: YAML that parses as
// a themeYAML struct but contains an invalid hex colour string. Same
// invariants as the malformed-YAML test above — Save must NOT fire.
func TestCore_HandleThemeFileRead_InvalidHexColorEmitsFlashOnly(t *testing.T) {
	c := newCore()

	// Valid YAML shape, invalid hex colour value (`notahex` fails
	// hexColorRe in styles/theme.go).
	badBytes := []byte("name: Custom\ncolors:\n  accent: \"notahex\"\n")

	intents, tasks := c.HandleThemeFileRead(ThemeFileReadEvent{
		Theme: "bad-hex.yaml",
		Bytes: badBytes,
	})

	if hasTaskKind(tasks, TaskKindSaveThemeConfig) {
		t.Errorf("expected NO TaskKindSaveThemeConfig on invalid hex colour")
	}
	if _, ok := findApplyTheme(intents); ok {
		t.Errorf("expected no ApplyThemeIntent on invalid hex colour")
	}
	if findPopSelector(intents) {
		t.Errorf("expected no PopSelectorIntent on invalid hex colour")
	}
	fi, ok := findFlashIntent(intents)
	if !ok {
		t.Fatalf("expected FlashIntent, got intents=%#v", intents)
	}
	if !fi.IsError {
		t.Errorf("expected IsError=true on invalid hex colour")
	}
}

// ---- shared helper ---------------------------------------------------------

// taskKinds extracts the TaskKind slice from tasks for diagnostic prints.
func taskKinds(tasks []TaskRequest) []TaskKind {
	out := make([]TaskKind, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, t.Key.Kind)
	}
	return out
}
