// tui_help_identity_errorlog_parity_test.go — parity pins for convergence
// wave 4a (REDIRECTED scope per the coordinator's stale-scout correction).
//
// The original wave-4a dispatch assumed HelpBody/IdentityBody and the
// ScreenRegion selector mapping were still gaps. They are NOT: verified at
// HEAD (branch feat/cache), core/app/snapshot.go already has
// buildHelpBody() (L110-163), (*Controller).buildIdentityBody() (L169-193),
// and bodyKindForScreen already maps ScreenProfileSelector/ScreenRegion/
// ScreenTheme to BodyKindSelector (L92). Those three are pinned only as
// lightweight regression guards below (TestHelpBody / TestIdentityBody /
// TestRegionSelector cases in this file focus on the REAL remaining gaps
// instead of re-proving settled ground).
//
// The REAL wave-4a work (verified against HEAD before writing these tests):
//
//  1. Help-table parity: buildHelpBody() (core/app/snapshot.go) is a
//     hardcoded copy of views.HelpModel.mainMenuGroups() that ALWAYS returns
//     Context:"main-menu" content — it does not branch on what screen is
//     beneath the help screen the way HelpModel.buildGroups() branches on
//     HelpContext (mainMenuGroups vs resourceListGroups vs detailGroups
//     etc., internal/tui/views/help.go L173-196). TestHelpBody_ContentMatches
//     TUIMainMenuHelp pins that both renderers agree on a known set of
//     main-menu bindings (already true today — a regression guard).
//     TestHelpBody_UnderResourceList_CarriesListContextBindings is the RED
//     pin: it pushes a bare ScreenResourceList then opens help on top and
//     asserts the resulting HelpBody contains a list-only binding ("y",
//     "yaml" — present in resourceListGroups L267-268, absent from
//     mainMenuGroups L217-250). This fails today because buildHelpBody never
//     looks below the top of the stack.
//
//  2. Ctrl-backed overlays: today '?' (newHelpRS), 'i' (newIdentityRS), and
//     '!' (newErrorLogRS) push a rendererState WITHOUT ever calling
//     ctrl.Apply/ctrl.ApplyIntents (internal/tui/app_input.go L49-104) — the
//     controller's own screen stack is untouched. StackInSync() does not
//     catch this today because app_stack_invariant.go's rsIsCtrlBacked
//     explicitly EXCLUDES rsKindHelp/rsKindIdentity/error-log-text rs from
//     the comparison (the "nonCtrlBackedException" list, L16-28) — so
//     StackInSync() is vacuously true both before and after this wave's fix.
//     The tests below pin the two things that DO change observably: (a)
//     StackInSync() must stay true across the full push+esc round trip for
//     all three overlays (already true today via the exclusion path; stays
//     true post-fix only if the coder adds BOTH ctrlBacked:true AND the
//     matching screenIDMatchesRSKind case in the same change — a partial
//     landing flips this false, which is exactly the coupling this pin
//     exists to catch); (b) rendered View() content for each overlay is
//     unchanged by the refactor (regression guard against a broken wiring
//     swap).
//
//  3. Error-log dual-store: internal/tui/app.go's m.errorHistory
//     ([]errorEntry) and core/app/controller.go's c.errorHistory
//     ([]controllerErrorEntry) are two independently-maintained stores fed
//     by hand-synced switch cases (internal/tui/runtime_adapter.go L98-102
//     mirrors core/app/intents.go L231-234 per that file's own comment,
//     "mirror the headless applyIntents case"). TestErrorLog_TUIViewMatches
//     ControllerErrorHistoryExactCount drives messages.APIError through the
//     TUI's real Update() loop (which appends to m.errorHistory via
//     m.core.HandleAPIError -> an applied error FlashIntent) and asserts the '!'
//     overlay's rendered line count exactly matches the number of
//     error-flash events dispatched — a resurrected or
//     drifted dual-store (e.g. one store double-appending, or the collapse
//     wiring the '!' key to a stale/partial source) would produce a mismatch.
//
// Harness: mirrors tui_stack_sync_test.go / qa_help_context_test.go (TUI
// lane: rootApplyMsg/newRootSizedModel/rootViewContent/stripANSI) and
// tui_detail_parity_test.go (headless lane: directly-constructed
// *app.Controller + *runtime.Core, package unit importing core/app
// alongside the TUI helpers in the same file).
package unit

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// newParityHeadlessController builds a Controller matching newTestController's
// construction shape (tests/unit/app_controller_test.go), duplicated here
// because that helper lives in the external unit_test package and this file
// must stay in package unit to reuse the TUI rootApplyMsg/tuitest harness.
func newParityHeadlessController(t *testing.T, profile, region string) *app.Controller {
	t.Helper()
	s := session.New()
	s.Profile = profile
	s.Region = region
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	return c
}

// ─────────────────────────────────────────────────────────────────────────
// 1. Help-table parity
// ─────────────────────────────────────────────────────────────────────────

// TestHelpBody_ContentMatchesTUIMainMenuHelp is a regression guard (already
// green at HEAD): the same well-known main-menu bindings must appear in both
// the controller's HelpBody sections and the TUI's rendered help View().
func TestHelpBody_ContentMatchesTUIMainMenuHelp(t *testing.T) {
	ctrl := newParityHeadlessController(t, "help-parity-prof", "us-east-1")
	vs, _ := ctrl.Apply(app.Action{Kind: app.ActionOpenHelp})

	if vs.Body.Kind != app.BodyKindHelp || vs.Body.Help == nil {
		t.Fatalf("Apply(OpenHelp) Body.Kind=%q Help=%v, want BodyKindHelp with non-nil Help", vs.Body.Kind, vs.Body.Help)
	}

	var bodyText strings.Builder
	for _, sec := range vs.Body.Help.Sections {
		bodyText.WriteString(sec.Title)
		bodyText.WriteString("\n")
		for _, h := range sec.Hints {
			bodyText.WriteString(h.Key)
			bodyText.WriteString(" ")
			bodyText.WriteString(h.Help)
			bodyText.WriteString("\n")
		}
	}
	headlessText := strings.ToLower(bodyText.String())

	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	tuiText := strings.ToLower(stripANSI(rootViewContent(m)))

	mustContainBoth := []string{"up/down", "top", "bottom", "select", "filter", "command", "quit"}
	for _, want := range mustContainBoth {
		if !strings.Contains(headlessText, want) {
			t.Errorf("HelpBody missing main-menu binding %q — headless body:\n%s", want, bodyText.String())
		}
		if !strings.Contains(tuiText, want) {
			t.Errorf("TUI help View() missing main-menu binding %q", want)
		}
	}
}

// TestHelpBody_UnderResourceList_CarriesListContextBindings is the RED pin
// for the wave-4a shared help-table gap: buildHelpBody() ignores what screen
// is beneath ScreenHelp and always returns main-menu content. "y"/"yaml" is
// present only in views.HelpModel.resourceListGroups (help.go L267-268),
// never in mainMenuGroups (help.go L217-250) — so this assertion
// distinguishes list-context help from main-menu-context help.
func TestHelpBody_UnderResourceList_CarriesListContextBindings(t *testing.T) {
	ctrl := newParityHeadlessController(t, "help-parity-list-prof", "us-east-1")

	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenResourceList,
		Context: runtime.ScreenContext{ResourceType: "ec2"},
	}})
	vs, _ := ctrl.Apply(app.Action{Kind: app.ActionOpenHelp})

	if vs.Body.Kind != app.BodyKindHelp || vs.Body.Help == nil {
		t.Fatalf("Apply(OpenHelp) over a resource-list screen: Body.Kind=%q Help=%v, want BodyKindHelp with non-nil Help", vs.Body.Kind, vs.Body.Help)
	}

	found := false
	for _, sec := range vs.Body.Help.Sections {
		for _, h := range sec.Hints {
			if h.Key == "y" && strings.Contains(strings.ToLower(h.Help), "yaml") {
				found = true
			}
		}
	}
	if !found {
		var got strings.Builder
		for _, sec := range vs.Body.Help.Sections {
			got.WriteString(sec.Title + ": ")
			for _, h := range sec.Hints {
				got.WriteString(h.Key + "=" + h.Help + " ")
			}
			got.WriteString("\n")
		}
		t.Errorf("HelpBody opened over ScreenResourceList should carry the list-context \"y\"/yaml binding (views.HelpModel.resourceListGroups), got context=%q sections:\n%s", vs.Body.Help.Context, got.String())
	}
	if vs.Body.Help.Context == "main-menu" {
		t.Errorf("HelpBody.Context = %q while a resource-list screen is beneath help — buildHelpBody must report the actual opening context, not the hardcoded default", vs.Body.Help.Context)
	}
}

// TestHelpBody_ListContext_ProducedByTUIAndControllerAgree cross-checks the
// TUI's own resource-list-context HelpModel output against the same list of
// list-only markers, so both halves of the shared-table gap are pinned
// against the SAME renderer that will eventually source from one table.
func TestHelpBody_ListContext_ProducedByTUIAndControllerAgree(t *testing.T) {
	hm := views.NewHelpWithResource(keys.Default(), views.HelpFromResourceList, "ec2")
	hm.SetSize(100, 40)
	tuiListHelp := strings.ToLower(stripANSI(hm.View()))

	if !strings.Contains(tuiListHelp, "yaml") {
		t.Fatalf("sanity check failed: views.HelpModel resourceListGroups should contain \"yaml\" — harness precondition broken, got:\n%s", tuiListHelp)
	}

	ctrl := newParityHeadlessController(t, "help-parity-list-agree-prof", "us-east-1")
	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenResourceList,
		Context: runtime.ScreenContext{ResourceType: "ec2"},
	}})
	vs, _ := ctrl.Apply(app.Action{Kind: app.ActionOpenHelp})
	if vs.Body.Help == nil {
		t.Fatal("HelpBody is nil after opening help over a resource-list screen")
	}
	sawYAML := false
	for _, sec := range vs.Body.Help.Sections {
		for _, h := range sec.Hints {
			if strings.Contains(strings.ToLower(h.Help), "yaml") {
				sawYAML = true
			}
		}
	}
	if !sawYAML {
		t.Error("HelpBody built over ScreenResourceList does not contain a \"yaml\" hint that views.HelpModel.resourceListGroups carries — the two renderers have drifted apart, exactly the bug a shared table prevents")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// 2. Ctrl-backed overlays: help / identity / error-log
// ─────────────────────────────────────────────────────────────────────────

// assertOverlayRoundTripStaysInSync opens an overlay via keyPress, asserts
// StackInSync() and the expected content marker, dismisses via esc, and
// re-asserts StackInSync(). This is a regression guard, not a RED pin — see
// the file header for why StackInSync() cannot discriminate pre/post-fix on
// its own (nonCtrlBackedException excludes these three rsKinds today).
func assertOverlayRoundTripStaysInSync(t *testing.T, key, contentMarker, label string) {
	t.Helper()
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(key))
	if !m.StackInSync() {
		t.Errorf("%s: StackInSync() = false immediately after opening the overlay", label)
	}
	plain := strings.ToLower(stripANSI(rootViewContent(m)))
	if !strings.Contains(plain, strings.ToLower(contentMarker)) {
		t.Errorf("%s: overlay View() missing expected marker %q, got:\n%s", label, contentMarker, plain)
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !m.StackInSync() {
		t.Errorf("%s: StackInSync() = false after esc-dismissing the overlay", label)
	}
}

func TestOverlayParity_Help_RoundTripStaysInSync(t *testing.T) {
	assertOverlayRoundTripStaysInSync(t, "?", "NAVIGATION", "help overlay")
}

func TestOverlayParity_Identity_RoundTripStaysInSync(t *testing.T) {
	// The identity overlay opens in a loading state (no synchronous
	// IdentityLoaded event is dispatched in this test), so the content
	// marker must be something present while loading — the frame title —
	// rather than a resolved-identity field like ARN.
	assertOverlayRoundTripStaysInSync(t, "i", "identity", "identity overlay")
}

func TestOverlayParity_ErrorLog_NoErrors_RoundTripStaysInSync(t *testing.T) {
	// With zero recorded errors, '!' flashes "No errors this session" instead
	// of pushing an overlay (core/app/actions_view.go handleActionOpenErrorLog
	// mirrors this: len(errorHistory)==0 -> flash, no push). StackInSync must
	// stay true since no rendererState was pushed at all.
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, rootKeyPress("!"))
	if !m.StackInSync() {
		t.Error("error-log overlay with zero errors: StackInSync() = false after pressing ! (expected a flash, no screen push)")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// 3. Error-log dual-store parity
// ─────────────────────────────────────────────────────────────────────────

// TestErrorLog_TUIViewMatchesControllerErrorHistoryExactCount drives N
// distinct messages.APIError events through the TUI's real Update() loop
// (each appends one error-flash-derived entry to BOTH
// m.errorHistory, the TUI-local store, and — via the same Core.HandleAPIError
// call — whatever store the post-refactor '!' overlay is wired to read from)
// then opens the '!' overlay and asserts the rendered line count is EXACTLY N,
// with every distinct error message present exactly once. A resurrected or
// drifted dual-store (double-append, dropped entry, or wiring the overlay to
// a stale/partial source) would produce a count mismatch here.
func TestErrorLog_TUIViewMatchesControllerErrorHistoryExactCount(t *testing.T) {
	m := newRootSizedModel()

	errs := []string{
		"errorlog-parity: connection refused",
		"errorlog-parity: access denied",
		"errorlog-parity: throttled",
	}
	for _, e := range errs {
		m, _ = rootApplyMsg(m, messages.APIError{
			ResourceType: "ec2",
			Err:          errParityError(e),
			Gen:          0,
		})
	}

	m, _ = rootApplyMsg(m, rootKeyPress("!"))
	if !m.StackInSync() {
		t.Fatal("error-log overlay: StackInSync() = false after opening with recorded errors")
	}
	plain := stripANSI(rootViewContent(m))

	// Only count occurrences on the error-log panel's own timestamped lines
	// ("[HH:MM:SS] <message>", written by handleActionOpenErrorLog /
	// app_input.go's '!' handler) — the header flash bar independently
	// echoes the MOST RECENT error's text ("errorlog-parity: throttled"),
	// which would otherwise double-count that one message. The panel is
	// boxed (each line prefixed by a "│" border rune), so match the
	// timestamp pattern anywhere in the line rather than requiring it at
	// the start.
	panelLineRe := regexp.MustCompile(`\[\d{2}:\d{2}:\d{2}\]\s*(.*)$`)
	panelLines := 0
	for _, line := range strings.Split(plain, "\n") {
		m := panelLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		panelLines++
		rest := strings.TrimSpace(m[1])
		matched := false
		for _, e := range errs {
			if strings.Contains(rest, e) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("error-log overlay: panel line %q does not match any dispatched error message", strings.TrimSpace(line))
		}
	}
	if panelLines != len(errs) {
		t.Errorf("error-log overlay: expected exactly %d timestamped panel lines (one per dispatched APIError), got %d — View():\n%s", len(errs), panelLines, plain)
	}

	for _, e := range errs {
		count := 0
		for _, line := range strings.Split(plain, "\n") {
			if panelLineRe.MatchString(line) && strings.Contains(line, e) {
				count++
			}
		}
		if count != 1 {
			t.Errorf("error-log overlay: expected exactly 1 panel occurrence of %q, got %d — View():\n%s", e, count, plain)
		}
	}
}

// errParityError is a minimal error constructor so the dispatched
// messages.APIError.Err.Error() text is EXACTLY the marker string with no
// AWS-classification prefix (core/runtime/handlers.go HandleAPIError
// falls back to ev.Err.Error() verbatim when ClassifyAWSError finds no
// recognised AWS error code).
type errParityErrorType string

func (e errParityErrorType) Error() string { return string(e) }

func errParityError(msg string) error { return errParityErrorType(msg) }
