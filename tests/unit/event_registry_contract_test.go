package unit

// event_registry_contract_test.go — exhaustiveness gate for the
// core/runtime/messages Event/Cmd registries.
//
// AllEventSamples() (core/runtime/messages/event.go) and AllCmdSamples()
// (core/runtime/messages/cmd.go) each claim to carry exactly one sample per
// concrete type implementing the Event/Cmd marker interfaces. This test
// verifies that claim by source-scanning the package for every isEvent()/
// isCmd() method declaration (a go/parser walk, not reflection over the
// registries alone — reflection can only see what the registries already
// claim to contain, never a type the registries forgot) and asserting the
// declared set and the sampled set are identical, in both directions, with
// no duplicate entries hiding a real gap behind a coincidentally-matching
// count.
//
// Boundary-sealing wave addition: the routing-classification half v2 had
// deliberately deferred is rebuilt below (TestEventRouting_*), NOT copied
// from attempt-1 — every citation was verified fresh against the CURRENT
// core/app/handle.go and core/runtime/orchestrator.go. v2's routing is a
// single Controller.Handle fold (calling Core.HandleEvent plus its own
// per-type side channels) rather than attempt-1's TUI-Update()-switch-shaped
// dispatch, so "neutral-handled" here means reachable from Controller.Handle
// and/or Core.HandleEvent — the surface web/headless/tests all share —
// regardless of whatever internal/tui's OWN separate Update() switch
// additionally does for its own rendering needs (internal/tui keeps a bare
// *runtime.Core plus, increasingly, an embedded *app.Controller — see
// messages.CostsLoaded's case in internal/tui/app.go — so a type can be both
// TUI-handled AND neutral-handled without contradiction).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// ercMessagesDir returns the repo-relative path to core/runtime/messages,
// following the same filepath.Join("..", "..") repo-root pattern
// testhelpers_forbidden_test.go uses to locate tests/unit.
func ercMessagesDir() string {
	root := filepath.Join("..", "..")
	return filepath.Join(root, "core", "runtime", "messages")
}

// ercScanMarkerReceivers parses every non-test .go file in dir and returns
// the set of receiver base type names for every method declaration named
// methodName (e.g. "isEvent" or "isCmd"). isEvent/isCmd are declared only as
// interface method specs on messages.Event/messages.Cmd (no receiver, not an
// *ast.FuncDecl at all) and as concrete-type method implementations (which
// do have a receiver) — so filtering on Recv != nil naturally selects only
// the concrete implementations, exactly the set AllEventSamples/
// AllCmdSamples are supposed to cover.
func ercScanMarkerReceivers(t *testing.T, dir, methodName string) map[string]bool {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}

	found := make(map[string]bool)
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", path, err)
		}
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || len(fd.Recv.List) != 1 || fd.Name.Name != methodName {
				continue
			}
			if base := ercReceiverBaseName(fd.Recv.List[0].Type); base != "" {
				found[base] = true
			}
		}
	}
	return found
}

// ercReceiverBaseName returns the bare type name of a receiver expression,
// unwrapping a pointer receiver's leading *.
func ercReceiverBaseName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// ercSampleTypeCounts returns, for a registry slice of interface values
// (messages.Event or messages.Cmd), how many times each concrete dynamic
// type name appears — reflect.TypeOf on an interface value yields the
// dynamic (concrete) type, exactly the name ercScanMarkerReceivers collects.
func ercSampleTypeCounts[T any](samples []T) map[string]int {
	counts := make(map[string]int, len(samples))
	for _, s := range samples {
		counts[reflect.TypeOf(s).Name()]++
	}
	return counts
}

// ercAssertExhaustive asserts declared and sampled name the exact same set
// of concrete types, and that no sampled type appears more than once (a
// duplicate could otherwise mask a missing type behind a matching count).
func ercAssertExhaustive(t *testing.T, label string, declared map[string]bool, sampled map[string]int) {
	t.Helper()

	var missingSample []string
	for name := range declared {
		if _, ok := sampled[name]; !ok {
			missingSample = append(missingSample, name)
		}
	}
	sort.Strings(missingSample)
	if len(missingSample) > 0 {
		t.Errorf("%s: declared but missing a sample: %v", label, missingSample)
	}

	var extraSample []string
	for name := range sampled {
		if !declared[name] {
			extraSample = append(extraSample, name)
		}
	}
	sort.Strings(extraSample)
	if len(extraSample) > 0 {
		t.Errorf("%s: sampled but not declared: %v", label, extraSample)
	}

	var duplicates []string
	for name, count := range sampled {
		if count > 1 {
			duplicates = append(duplicates, name)
		}
	}
	sort.Strings(duplicates)
	if len(duplicates) > 0 {
		t.Errorf("%s: duplicate sample types: %v", label, duplicates)
	}
}

// TestEventRegistry_Events_ExhaustiveEnumeration is the exhaustiveness gate
// for AllEventSamples(): every concrete type declaring isEvent() in
// core/runtime/messages must have exactly one sample there, and every
// sampled type must be a real isEvent() declaration.
func TestEventRegistry_Events_ExhaustiveEnumeration(t *testing.T) {
	declared := ercScanMarkerReceivers(t, ercMessagesDir(), "isEvent")
	sampled := ercSampleTypeCounts(messages.AllEventSamples())
	ercAssertExhaustive(t, "Event", declared, sampled)
}

// TestEventRegistry_Cmds_ExhaustiveEnumeration is the exhaustiveness gate
// for AllCmdSamples(): every concrete type declaring isCmd() in
// core/runtime/messages must have exactly one sample there, and every
// sampled type must be a real isCmd() declaration.
func TestEventRegistry_Cmds_ExhaustiveEnumeration(t *testing.T) {
	declared := ercScanMarkerReceivers(t, ercMessagesDir(), "isCmd")
	sampled := ercSampleTypeCounts(messages.AllCmdSamples())
	ercAssertExhaustive(t, "Cmd", declared, sampled)
}

// TestEventRegistry_ScannerDetectsMarkerType_InIsolatedTempDir is the
// executable detection proof: it writes a throwaway package messages file
// containing a canary type with an isEvent() method into a fresh t.TempDir(),
// points ercScanMarkerReceivers (the exact same helper the gate tests above
// use) at that directory, and asserts the canary is found — proving the
// scanner actually parses and matches declarations rather than vacuously
// passing on an empty or misconfigured directory.
func TestEventRegistry_ScannerDetectsMarkerType_InIsolatedTempDir(t *testing.T) {
	dir := t.TempDir()
	src := `package messages

type zzzCanaryEvent struct{}

func (zzzCanaryEvent) isEvent() {}
`
	if err := os.WriteFile(filepath.Join(dir, "canary.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	found := ercScanMarkerReceivers(t, dir, "isEvent")
	if !found["zzzCanaryEvent"] {
		t.Errorf("scanner did not detect canary type zzzCanaryEvent in isolated temp dir; found=%v", found)
	}
}

// ---------------------------------------------------------------------------
// Routing classification (boundary-sealing wave).
// ---------------------------------------------------------------------------

// ercRouteKind classifies where an Event/Cmd type is actually consumed.
type ercRouteKind int

const (
	// ercNeutralHandled: reachable via core/app.Controller.Handle and/or
	// core/runtime.Core.HandleEvent — the renderer-neutral surface web,
	// headless (DrainSync), and tests all share. internal/tui may ALSO
	// handle the same type in its own Update() switch; that does not change
	// this classification.
	ercNeutralHandled ercRouteKind = iota
	// ercRendererOnly: consumed exclusively by internal/tui's Update()
	// switch (or, for Cmd types, that switch is the ONLY consumer by
	// architecture — Cmd is the runtime-to-TUI-adapter directive channel).
	// Never reachable via Controller.Handle/Core.HandleEvent, for a
	// structural reason (adapter-owned state the call needs, or a
	// TUI-only/GPL-only package dependency), cited per entry.
	ercRendererOnly
	// ercUnrouted: declared and sampled (AllEventSamples/AllCmdSamples), but
	// consumed NOWHERE — neither Controller.Handle/Core.HandleEvent nor
	// internal/tui's Update() switch has a case for it. A real gap, not a
	// third kind of legitimate design — see
	// TestEventRouting_UnroutedEventTypes_AreExplicitlyFlagged.
	ercUnrouted
)

// ercRoute is one Event (or Cmd) type's routing classification: which kind,
// and the citation justifying it (a Controller.Handle/Core.HandleEvent case,
// an internal/tui/app.go case, or — for ercUnrouted — the absence found in
// all three).
type ercRoute struct {
	Kind   ercRouteKind
	Reason string
}

// ercEventRoutes classifies every concrete messages.Event type. Verified
// fresh against core/app/handle.go and core/runtime/orchestrator.go (not
// copied from attempt-1's table, which predates the single-Controller-fold
// shape) — see each Reason for the exact citation.
var ercEventRoutes = map[string]ercRoute{
	"ResourcesLoaded": {ercNeutralHandled,
		"Controller.Handle (handle.go): handleResourcesLoadedEvent + reapplyCheckerAgainst + autoOpenSingleDetail; " +
			"Core.HandleEvent (orchestrator.go): RowStore dual-write + list-open Wave-2 dispatch, forwarding tasks only " +
			"(never HandleResourcesLoaded's intents, to avoid double-applying them against Controller.Handle's own pipeline)"},
	"APIError": {ercNeutralHandled,
		"Core.HandleEvent (orchestrator.go) case: classifies via Core.HandleAPIError using session.ConnectGen as the " +
			"headless stand-in for the TUI's adapter-owned flash.gen"},
	"Flash": {ercRendererOnly,
		"internal/tui/app.go case messages.Flash -> m.handleFlash; no Controller.Handle/Core.HandleEvent case exists. " +
			"Structural: Flash is the TUI's own async tea.Cmd-completion notification (e.g. runtime_adapter_navigate.go's " +
			"copyToClipboard returns one) needing the adapter-owned flash generation counter bumped before dispatch — " +
			"web/headless callers never produce a bare Flash event; they apply FlashIntent (already computed by the " +
			"runtime) directly instead"},
	"ByIDFetchFailed": {ercNeutralHandled,
		"Controller.Handle (handle.go): popAutoOpenSinglePlaceholderOnNotFound. Core.HandleEvent's default nil,nil path " +
			"applies (handle.go's own comment: 'this typed outcome is Core.HandleEvent's default nil,nil path')"},
	"ClearFlash": {ercRendererOnly,
		"internal/tui/app.go case messages.ClearFlash -> m.handleClearFlash; no Controller.Handle/Core.HandleEvent case. " +
			"Structural: same adapter-owned flash.gen/flash.isError dependency as Flash above (orchestrator.go's own " +
			"'Messages NOT wired here' doc list: 'TUI shim handleClearFlash passes flash.gen and flash.isError ... into Core')"},
	"ValueRevealed": {ercNeutralHandled,
		"Controller.Handle (handle.go) explicit block, gated on a resource-bearing screen being on the stack; " +
			"explicitly excluded from Core.HandleEvent per orchestrator.go's doc list (adapter needs its own flash.gen for " +
			"the staleness surface, but Controller.Handle computes an equivalent guard via hasResourceScreen()/IsStale)"},
	"ClientsReady": {ercNeutralHandled,
		"Controller.Handle (handle.go) explicit block: computes StackDepth/HasActiveRL/HasActiveCosts itself (the " +
			"renderer-shape inputs the TUI shim would otherwise supply) before calling Core.HandleClientsReady — " +
			"explicitly excluded from Core.HandleEvent per orchestrator.go's doc list for that same reason"},
	"RelatedCheckResult": {ercNeutralHandled,
		"Controller.Handle (handle.go): foldRelatedCheckResultLocked, gated by messages.IsStale against the active " +
			"DetailOperation; Core.HandleEvent (orchestrator.go) case ALSO runs (RowStore dual-write only, returns nil,nil " +
			"— Controller.Handle owns the intents to avoid double-applying PatchRelatedCache et al.)"},
	"RelatedCheckBatch": {ercNeutralHandled,
		"Controller.Handle (handle.go): handleRelatedCheckBatch folds each per-def result through the same " +
			"foldRelatedCheckResultLocked RelatedCheckResult uses. No Core.HandleEvent case and no internal/tui/app.go " +
			"case — by design, not a gap: this is the headless executor's bulk-fan-out result " +
			"(core/runtime/executor.go's runRelatedCheckers), and the TUI's own per-def tea.Cmd fan-out " +
			"(runtime_adapter_related.go) never produces a batch at all, only individual RelatedCheckResult messages"},
	"AvailabilityCacheLoaded": {ercNeutralHandled,
		"Core.HandleEvent (orchestrator.go) case -> handleAvailabilityCacheLoaded"},
	"AvailabilityPrefetched": {ercNeutralHandled,
		"Core.HandleEvent (orchestrator.go) case -> handleAvailabilityPrefetched"},
	"AvailabilityChecked": {ercNeutralHandled,
		"Core.HandleEvent (orchestrator.go) case -> handleAvailabilityChecked; Controller.Handle (handle.go) also runs " +
			"a side-channel (markMenuSweepAcked) unconditionally, independent of HandleEvent's own gen-guard verdict"},
	"EnrichmentChecked": {ercNeutralHandled,
		"Core.HandleEvent (orchestrator.go) case -> handleEnrichmentChecked"},
	"IdentityLoaded": {ercNeutralHandled,
		"Core.HandleEvent (orchestrator.go) case -> HandleIdentityLoaded"},
	"IdentityError": {ercNeutralHandled,
		"Core.HandleEvent (orchestrator.go) case -> HandleIdentityError; Controller.Handle (handle.go) also stores " +
			"identityErrMsg (view-layer state Core does not own) as a side channel"},
	"EnrichDetailResult": {ercNeutralHandled,
		"Controller.Handle (handle.go) explicit block: foldEnrichDetailResultLocked, gated by messages.IsStale against " +
			"the active DetailOperation; explicitly excluded from Core.HandleEvent per orchestrator.go's doc list (the TUI " +
			"shim's reason given there — adapter-side staleness drop and derive — does not apply to Controller.Handle, " +
			"which does its own IsStale check inline instead)"},
	"CostsLoaded": {ercNeutralHandled,
		"Controller.Handle (handle.go) explicit block: ApplyCostsLoaded. Explicitly 'not wired into runtime.Core." +
			"HandleEvent' per handle.go's own comment — CostsState lives on the Controller's screen stack, not session " +
			"state Core owns"},
	"ThemeFileRead": {ercRendererOnly,
		"internal/tui/app.go case messages.ThemeFileRead -> m.handleThemeFileRead; no Controller.Handle/Core.HandleEvent " +
			"case. Structural: parsing the YAML into a Theme requires internal/tui/styles.ThemeFromYAML, a TUI-only " +
			"(GPL-3.0-or-later-only) renderer package core/runtime cannot depend on (core/ is dual-licensed, importable " +
			"by external modules)"},
}

// ercCmdRoutes classifies every concrete messages.Cmd type. Unlike Event,
// every Cmd type is legitimately TUI-only by architecture — Cmd is the
// runtime-to-adapter directive channel (Navigate, PopView, fetch triggers,
// …); there is no "neutral" bucket to check against, only "does
// internal/tui/app.go's Update() switch have a case for it". Verified fresh
// against internal/tui/app.go's switch (item (d)'s "if tractable" — it is:
// all 10 map cleanly to one file's switch).
var ercCmdRoutes = map[string]string{
	"Navigate":        "internal/tui/app.go case messages.Navigate -> m.handleNavigate",
	"PopView":         "internal/tui/app.go case messages.PopView -> m.popRS",
	"LoadMore":        "internal/tui/app.go case messages.LoadMore -> m.fetchMoreResources",
	"ProfileSelected": "internal/tui/app.go case messages.ProfileSelected -> m.handleProfileSelected",
	"RegionSelected":  "internal/tui/app.go case messages.RegionSelected -> m.handleRegionSelected",
	"ThemeSelected":   "internal/tui/app.go case messages.ThemeSelected -> m.handleThemeSelected",
	"InitConnect":     "internal/tui/app.go case messages.InitConnect -> m.connectAWS",
	"EnterChildView":  "internal/tui/app.go case messages.EnterChildView -> m.handleEnterChildView",
	"LoadResources":   "internal/tui/app.go case messages.LoadResources -> m.fetchResources / m.fetchChildResources",
	"RelatedNavigate": "internal/tui/app.go case messages.RelatedNavigate -> m.handleRelatedNavigate",
}

// TestEventRouting_Classification_SetEqualsDeclaredEvents asserts
// ercEventRoutes classifies EVERY concrete Event type the AST scan finds —
// reusing ercScanMarkerReceivers/ercMessagesDir, the same declared-set source
// the exhaustiveness gate above uses — and nothing extra. A new Event type
// with no entry here fails the suite instead of silently passing.
func TestEventRouting_Classification_SetEqualsDeclaredEvents(t *testing.T) {
	declared := ercScanMarkerReceivers(t, ercMessagesDir(), "isEvent")

	var missing []string
	for name := range declared {
		if _, ok := ercEventRoutes[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("declared Event types with no routing classification: %v", missing)
	}

	var extra []string
	for name := range ercEventRoutes {
		if !declared[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	if len(extra) > 0 {
		t.Errorf("classified but not a declared Event type (stale entry?): %v", extra)
	}
}

// TestEventRouting_EveryClassificationHasReason guards against a hollow
// entry (a bare Kind with no citation) slipping past the set-equality check
// above — every classification must carry a non-empty Reason and a valid
// Kind.
func TestEventRouting_EveryClassificationHasReason(t *testing.T) {
	for name, route := range ercEventRoutes {
		if strings.TrimSpace(route.Reason) == "" {
			t.Errorf("%s: classification has an empty Reason", name)
		}
		if route.Kind != ercNeutralHandled && route.Kind != ercRendererOnly && route.Kind != ercUnrouted {
			t.Errorf("%s: invalid route Kind %d", name, route.Kind)
		}
	}
}

// TestEventRouting_UnroutedEventTypes_AreExplicitlyFlagged pins the exact
// set of Event types classified ercUnrouted (a genuine gap — consumed
// nowhere) to empty: every declared event is routed or structurally
// renderer-only. A new unrouted type appearing means investigate before
// accepting (has a real routing case been missed?) — a deliberate edit to
// `want` is required, never a silent pass. (The last occupant, Copied, was
// dead on arrival — never emitted, never consumed — and was deleted.)
func TestEventRouting_UnroutedEventTypes_AreExplicitlyFlagged(t *testing.T) {
	var unrouted []string
	for name, route := range ercEventRoutes {
		if route.Kind == ercUnrouted {
			unrouted = append(unrouted, name)
		}
	}
	sort.Strings(unrouted)

	var want []string
	if !reflect.DeepEqual(unrouted, want) {
		t.Errorf("ercUnrouted Event types = %v, want %v — see this test's doc comment before changing `want`", unrouted, want)
	}
}

// TestEventRouting_CmdClassification_SetEqualsDeclaredCmds is the Cmd-side
// twin of TestEventRouting_Classification_SetEqualsDeclaredEvents.
func TestEventRouting_CmdClassification_SetEqualsDeclaredCmds(t *testing.T) {
	declared := ercScanMarkerReceivers(t, ercMessagesDir(), "isCmd")

	var missing []string
	for name := range declared {
		if _, ok := ercCmdRoutes[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("declared Cmd types with no consumption-site classification: %v", missing)
	}

	var extra []string
	for name := range ercCmdRoutes {
		if !declared[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	if len(extra) > 0 {
		t.Errorf("classified but not a declared Cmd type (stale entry?): %v", extra)
	}
}

// TestEventRouting_EveryCmdClassificationHasReason is the Cmd-side twin of
// TestEventRouting_EveryClassificationHasReason.
func TestEventRouting_EveryCmdClassificationHasReason(t *testing.T) {
	for name, reason := range ercCmdRoutes {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("%s: Cmd classification has an empty citation", name)
		}
	}
}
