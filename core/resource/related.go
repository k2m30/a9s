// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package resource

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/arn"

	"github.com/k2m30/a9s/v3/core/domain"
)

// RelatedDef defines one related resource class for a given resource type.
// Declaration lives in core/domain/contracts.go; this alias re-exports it.
type RelatedDef = domain.RelatedDef

// NavigableField associates a detail view field path with a target resource type.
// Declaration lives in core/domain/contracts.go; this alias re-exports it.
type NavigableField = domain.NavigableField

// RefRegion returns the Region an AWS reference names, and "" for one that
// names none: a bare name, or an ARN of a global service, which carries no
// Region at all. A reference naming another Region is read there — the
// session's own holds no row of it.
func RefRegion(ref string) string {
	a, err := arn.Parse(ref)
	if err != nil {
		return ""
	}
	return a.Region
}

// ResolveRef reads ref — any AWS reference to targetType — as the ID the
// target's rows are keyed by, through the target's own RefToID. ok is false
// when ref names no row of this account and region. A type without a
// resolver keys its rows on the reference itself.
func ResolveRef(targetType, ref string, rc domain.RefContext) (string, bool) {
	if td := TypeDef(targetType); td != nil && td.RefToID != nil {
		return td.RefToID(ref, rc)
	}
	return ref, ref != ""
}

// NavIDFromValue is the ID Enter on a navigable field opens: ResolveRef's
// reading of value, or "" when value names no row of targetType.
func NavIDFromValue(targetType, value string, rc domain.RefContext) string {
	id, ok := ResolveRef(targetType, value, rc)
	if !ok {
		return ""
	}
	return id
}

// RelatedCheckResult is returned by a RelatedChecker and carries all state
// needed by the right-column panel to display a row and navigate on Enter.
// Declaration lives in core/domain/contracts.go; this alias re-exports it.
//
// Semantics:
//
//   - State == RelatedResolved (zero value): Count (0..N) is authoritative.
//   - State == RelatedUnknown: the checker could not determine a count.
//   - State == RelatedLoading: no checker result has arrived yet — set only
//     by row-mirror producers, never returned by a checker itself.
//   - State == RelatedError: the checker (or a prerequisite lookup) failed.
//   - State == RelatedDeferred: navigation uses FetchFilter's server-side
//     filtered fetch instead of a local count.
//   - Truncated == true: Count was derived from a truncated cache page;
//     only meaningful when State == RelatedResolved.
//   - FetchFilter non-nil: navigation should use a server-side filtered fetcher.
type RelatedCheckResult = domain.RelatedCheckResult

// ResourceCacheEntry holds a snapshot of one resource type's list plus
// truncation state. Declaration lives in core/domain/contracts.go; this
// alias re-exports it.
type ResourceCacheEntry = domain.ResourceCacheEntry

// ResourceCache is a read-only snapshot of already-loaded resource lists,
// keyed by resource short name. Declaration lives in core/domain/contracts.go;
// this alias re-exports it.
type ResourceCache = domain.ResourceCache

// RelatedChecker returns a count of related resources of a specific type.
// Declaration lives in core/domain/contracts.go; this alias re-exports it.
type RelatedChecker = domain.RelatedChecker

// ValidateRelatedResult sanity-checks that a checker's result is internally
// consistent with its declared TargetType. Catches bugs where a checker
// scans the wrong cache (e.g., returning ecs-task IDs as TargetType "ecs").
//
// Returns the first violation as an error, or nil if the result is consistent.
// Currently checks:
//   - TargetType is non-empty
//   - When Count > 0, ResourceIDs is non-empty
//   - When State != RelatedResolved, Count must be 0 and ResourceIDs empty
//   - When Truncated is true, State must be RelatedResolved
//
// This is intended for test invariants and optional debug-mode runtime checks,
// not for production error returns.
//
// For cross-checking that returned IDs match the target type's canonical
// Resource.ID, use ValidateRelatedResultAgainstCacheForTest.
func ValidateRelatedResult(r RelatedCheckResult) error {
	if r.TargetType() == "" {
		return fmt.Errorf("RelatedCheckResult: empty TargetType")
	}
	if r.Count() > 0 && len(r.ResourceIDs()) == 0 {
		return fmt.Errorf("RelatedCheckResult[%s]: Count=%d but no ResourceIDs", r.TargetType(), r.Count())
	}
	if r.State() != domain.RelatedResolved {
		if r.Count() != 0 {
			return fmt.Errorf("RelatedCheckResult[%s]: State=%s but Count=%d (must be 0)", r.TargetType(), r.State(), r.Count())
		}
		if len(r.ResourceIDs()) > 0 {
			return fmt.Errorf("RelatedCheckResult[%s]: State=%s but %d ResourceIDs present", r.TargetType(), r.State(), len(r.ResourceIDs()))
		}
	}
	if r.Truncated() && r.State() != domain.RelatedResolved {
		return fmt.Errorf("RelatedCheckResult[%s]: Truncated=true but State=%s (must be RelatedResolved)", r.TargetType(), r.State())
	}
	return nil
}

// ValidateRelatedResultAgainstCacheForTest enforces the canonical-target-identity
// contract: every ResourceID returned by a checker for a given
// TargetType MUST match the canonical Resource.ID that the TargetType's
// fetcher emits. We prove this by cross-checking the returned IDs against the
// target-type's cache entry.
//
// The check is deliberately opportunistic: it only runs when the cache has
// a non-truncated entry for the target type. A truncated cache could miss a
// legitimate ID, so we skip the check rather than produce false positives. If
// the target type has no cache entry at all, the check also skips (nothing to
// compare against). Shape invariants from ValidateRelatedResult are enforced
// regardless.
//
// This is the hard contract that catches bugs where a checker returns an ARN,
// name, or adjacent ID kind instead of the target type's canonical Resource.ID.
//
// Test-only. In production the shape invariants ValidateRelatedResult checks
// are enforced by construction — see the constructors in
// core/domain/related_result.go and the RelatedDef.TargetType guard in
// core/catalog/catalog.go.
func ValidateRelatedResultAgainstCacheForTest(r RelatedCheckResult, cache ResourceCache) error {
	if err := ValidateRelatedResult(r); err != nil {
		return err
	}
	if len(r.ResourceIDs()) == 0 {
		return nil
	}
	entry, ok := cache[r.TargetType()]
	if !ok {
		return nil
	}
	if entry.IsTruncated {
		return nil
	}
	known := make(map[string]struct{}, len(entry.Resources))
	for _, res := range entry.Resources {
		known[res.ID] = struct{}{}
	}
	for _, id := range r.ResourceIDs() {
		if _, seen := known[id]; !seen {
			return fmt.Errorf(
				"RelatedCheckResult[%s]: ResourceID %q is not a canonical Resource.ID for target type %q "+
					"(not found in target-type cache of %d resources); "+
					"checker likely returned an ARN/name/adjacent-ID kind instead of the target's canonical ID",
				r.TargetType(), id, r.TargetType(), len(entry.Resources),
			)
		}
	}
	return nil
}

// UnknownRelated returns a RelatedCheckResult representing "the checker
// could not determine the count because a prerequisite lookup failed". The
// most common case is a two-hop checker (snapshot → source DB instance →
// cluster) where the SOURCE was not found in a truncated intermediate cache,
// so the hop to the TARGET was never attempted. Renders as the fourth visible
// state — a blank, navigable row (no count, drill in) — never "(?)".
func UnknownRelated(targetType string) RelatedCheckResult {
	return domain.UnknownRelated(targetType)
}

// ErrorRelated returns a RelatedCheckResult representing "the checker (or a
// prerequisite AWS call) returned an error". Renders blank (no count — "(?)"
// is forbidden) AND dimmed: unlike UnknownRelated it is a dead end, not
// navigable, because drilling into data that never resolved is misleading.
// The failure is surfaced separately through a Flash{IsError:true} + the "!"
// error log; the user retries with Ctrl+R.
func ErrorRelated(targetType string, err error) RelatedCheckResult {
	return domain.ErrorRelated(targetType, err)
}

// DeferredRelated returns a RelatedCheckResult representing "the count is not
// resolved locally; Enter should drill in via a server-side FetchFilter fetch
// instead". Renders with a blank count badge and is always actionable.
func DeferredRelated(targetType string, filter map[string]string) RelatedCheckResult {
	return domain.DeferredRelated(targetType, filter)
}

// KnownRelated returns a proven RelatedCheckResult: the checker's lookup
// completed and ids is either the exhaustive match set (truncated == false)
// or the best-effort subset found so far, with more possibly unseen
// (truncated == true). This is the ONLY way to construct a RelatedResolved
// result with a count — a bare struct literal reporting Count: 0 after a
// swallowed error does not compile outside core/domain, since
// RelatedCheckResult's fields are all unexported. See
// domain.KnownRelated for the truncated-as-partial-success semantics.
func KnownRelated(targetType string, ids []string, truncated bool) RelatedCheckResult {
	return domain.KnownRelated(targetType, ids, truncated)
}

// RelatedCoverage states how far a related lookup searched. Declaration lives
// in core/domain/contracts.go; this alias re-exports it.
type RelatedCoverage = domain.RelatedCoverage

// Coverage values; see domain.RelatedCoverage.
const (
	CoverageComplete  = domain.CoverageComplete
	CoveragePartial   = domain.CoveragePartial
	CoverageNoPath    = domain.CoverageNoPath
	CoverageHeuristic = domain.CoverageHeuristic
)

// ProvenZero is the only result that reports a complete zero; see
// domain.ProvenZero.
func ProvenZero(targetType, evidence string) RelatedCheckResult {
	return domain.ProvenZero(targetType, evidence)
}

// NoDiscoveryPath is the result of a pivot AWS records no link for; see
// domain.NoDiscoveryPath.
func NoDiscoveryPath(targetType string) RelatedCheckResult {
	return domain.NoDiscoveryPath(targetType)
}

// HeuristicRelated is the result of a pivot that matches by a shared
// property; see domain.HeuristicRelated.
func HeuristicRelated(targetType string, ids []string) RelatedCheckResult {
	return domain.HeuristicRelated(targetType, ids)
}

// IsRelatedActionable is the single source of truth for "can the user drill into
// this related-resource pivot". It is consumed by the TUI right column
// (isActionableRow), the headless controller (ActionRelatedSelect +
// RelatedBlock.Actionable), and — via that ViewState field — the web template,
// so the rule cannot drift between renderers.
//
// A related row's final disposition is one of (never "(?)", which is forbidden):
//  1. "(N)"        — an exact count                     (actionable)
//  2. "(N+)"/"(0+)"— a lower bound                      (actionable)
//  3. dimmed "(0)" — a proven zero                      (NOT actionable — dead end)
//  4. blank        — "we aren't counting this, drill in" (actionable: Unknown/Deferred)
//  5. blank dimmed — the checker errored                (NOT actionable — dead end)
//
// RelatedError is a dead end like a proven zero: an error is surfaced through a
// Flash{IsError:true} + the "!" error log, and the user
// retries with Ctrl+R rather than drilling into data that never resolved.
// RelatedLoading is the transient in-progress spinner and resolves into one of
// the above.
func IsRelatedActionable(state domain.RelatedRowState, count int, truncated bool) bool {
	switch state {
	case domain.RelatedLoading, domain.RelatedError:
		// Loading: transient spinner. Error: dead end — surfaced via flash + log,
		// recovered with Ctrl+R, never navigable.
		return false
	case domain.RelatedDeferred, domain.RelatedUnknown:
		// The blank-navigable state: no number, drill in to find out.
		return true
	default: // RelatedResolved
		// Only a PROVEN zero — a complete (non-truncated) scan that found
		// nothing — is a dead end (state 3). An truncated lower bound
		// ("N+"/"0+") stays actionable (state 2): the user drills in to see
		// the rest.
		if count == 0 && !truncated {
			return false
		}
		return true
	}
}

// RelatedEnterAction classifies what pressing Enter on a related row does. It is
// derived from the SAME (state, count, truncated) inputs as IsRelatedActionable /
// FormatRelatedCount, so every Enter path (live TUI app_stack, headless
// keyboard/click actions) and the Tab drillable-cursor predicate share one rule
// and can never drift. Critically, a truncated lower bound is ALWAYS
// RelatedResolved: KnownRelated is the only constructor that sets truncated,
// and it leaves state at its zero value RelatedResolved; the other three
// constructors (UnknownRelated, ErrorRelated, DeferredRelated) never set
// truncated, and WithTargetType/WithFetchFilter preserve whatever state and
// truncated a result already carries. So "(0+)" and "(N+)" produce the
// IDENTICAL action — there is no count-based special case for the zero lower
// bound.
type RelatedEnterAction int

const (
	// RelatedEnterDeadEnd — not actionable (proven "(0)", error, loading). No-op.
	RelatedEnterDeadEnd RelatedEnterAction = iota
	// RelatedEnterResolveInPlace — an actionable but scope-less row: the blank
	// RelatedUnknown state, which has no count, no IDs, and no server-side
	// filter. Enter re-dispatches the source's related checks so the row firms
	// up in place (it does NOT open the target type's plain unfiltered list).
	RelatedEnterResolveInPlace
	// RelatedEnterNavigate — open the scoped destination: a filtered list of the
	// found IDs (one ID → its detail), a Deferred server-side filtered fetch, or
	// — for a truncated scan with zero IDs found so far ("(0+)") — a scoped list
	// with zero rows and a "more" affordance, exactly like "(N+)".
	RelatedEnterNavigate
)

// RelatedEnter is the single source of truth for a related row's Enter action.
// "(0+)" and "(N+)" are both RelatedResolved+Truncated, so they map to the same
// RelatedEnterNavigate — no site may branch on the count to decide navigate vs
// resolve-in-place.
func RelatedEnter(state domain.RelatedRowState, count int, truncated bool) RelatedEnterAction {
	if !IsRelatedActionable(state, count, truncated) {
		return RelatedEnterDeadEnd
	}
	if state == domain.RelatedUnknown {
		// The only actionable row with nothing to scope by (no count, no IDs,
		// no filter). A truncated "(0+)" is RelatedResolved, not Unknown, so it
		// falls through to Navigate alongside "(N+)".
		return RelatedEnterResolveInPlace
	}
	return RelatedEnterNavigate
}

// FormatRelatedCount is the single source of truth for the count BADGE text on a
// related-resource row. It is consumed by the TUI right column and — via
// RelatedBlock.CountDisplay computed in the controller — the web template, so
// the displayed count cannot drift.
//
//   - RelatedResolved, exact       → "(N)"
//   - RelatedResolved, truncated → "(N+)" — a lower bound from a truncated
//     target scan; the real count is at least N and more may exist on later
//     pages. "(0+)" is the honest form of "scanned one page, found none yet".
//   - everything else (RelatedDeferred / RelatedUnknown / RelatedError /
//     RelatedLoading) → "" — NO number. RelatedDeferred/RelatedUnknown are the
//     "we aren't giving a count, drill in" rows; RelatedError is a blank dead
//     end (dimmed, not navigable); RelatedLoading shows a spinner. "(?)" is
//     FORBIDDEN and is never produced.
func FormatRelatedCount(state domain.RelatedRowState, count int, truncated bool) string {
	if state == domain.RelatedResolved {
		if truncated {
			return fmt.Sprintf("(%d+)", count)
		}
		return fmt.Sprintf("(%d)", count)
	}
	return ""
}

// NoopCheckerForTest is a stub RelatedChecker suitable for tests that exercise
// registry wiring (SetRelatedForTest / AppendRelated / GetRelated) without
// exercising real related-resource logic. Production code MUST NOT use it:
// SetRelatedForTest panics if any RelatedDef is registered with a nil Checker,
// but production tests using this explicit stub satisfy the guard while
// remaining free of test-specific behavior.
func NoopCheckerForTest(_ context.Context, _ any, _ Resource, _ ResourceCache) RelatedCheckResult {
	return RelatedCheckResult{}
}

// relatedTestOverridesMu guards relatedTestOverrides and relatedTestOverridesPrev. All
// reads and writes to those two maps must hold this mutex.
var relatedTestOverridesMu sync.RWMutex

// relatedTestOverrides is a TEST-ONLY override table: it maps a resource
// short name to a replacement set of RelatedDef, letting a test substitute
// fake checkers for a type without touching the catalog. It starts empty and
// is populated only by SetRelatedForTest / AppendRelated, both of which panic
// outside a test binary. The catalog (core/catalog, installed via aws.Install)
// is the only production source of RelatedDef registrations — GetRelated
// checks this table first purely so a test override can shadow it.
var relatedTestOverrides = map[string][]RelatedDef{}

// relatedTestOverridesPrev is a stack (per short name) of registration snapshots
// saved before each SetRelatedForTest / AppendRelated call. CleanupRelatedForTest pops
// the top entry to restore the previous state. Using a stack (instead of a single
// slot) keeps nested Register calls from losing the original registration past
// the second Unregister.
//
// A nil entry on the stack means "no previous registration existed" and Unregister
// should delete the active entry rather than restore.
var relatedTestOverridesPrev = map[string][][]RelatedDef{}

// navigableFieldMu guards navigableFieldRegistry and navigableFieldPrevious.
// All reads and writes to those two maps must hold this mutex.
var navigableFieldMu sync.RWMutex

// navigableFieldRegistry maps resource short names to their active navigable
// field definitions. This is the mutable "session" registry: it starts empty
// and is populated only by explicit SetNavigableFieldsForTest calls (from tests
// or from BootstrapActiveNavFields at app startup). This keeps unit tests that
// do not call SetNavigableFieldsForTest isolated from production init-time defaults.
var navigableFieldRegistry = map[string][]NavigableField{}

// navigableFieldPrevious is a stack (per short name) of registration snapshots
// saved before each SetNavigableFieldsForTest call. CleanupNavigableFieldsForTest pops
// the top entry to restore the previous state. Using a stack (instead of a single
// slot) prevents nested Register calls from losing the original default-registered
// state past the second Unregister.
var navigableFieldPrevious = map[string][][]NavigableField{}

// defaultNavFieldMu guards defaultNavFieldRegistry. Reads can happen from
// any goroutine after startup.
var defaultNavFieldMu sync.RWMutex

// defaultNavFieldRegistry is an immutable-by-convention registry, always
// empty (nothing writes to it). GetDefaultNavFields falls through to
// the catalog Navigable defaults below. NavFieldsProvider (used by
// projection.GenericWithConfig) reads from this registry. DetailModel reads
// from the mutable navigableFieldRegistry so that tests can construct
// models without any nav field registrations.
var defaultNavFieldRegistry = map[string][]NavigableField{}

// SetRelatedForTest stores related definitions for the given resource short
// name in the test-only override table. Panics outside a test binary — this
// is not a production registration path, the catalog is. Panics at
// init-time (of a test binary) if any RelatedDef has a nil Checker or empty
// TargetType — a nil Checker is a structural bug, not a supported stub state.
//
// The current value for shortName (which may be nil) is pushed onto a per-key
// stack in relatedTestOverridesPrev so that subsequent CleanupRelatedForTest calls
// restore the previous registration instead of destroying it.
func SetRelatedForTest(shortName string, defs []RelatedDef) {
	if !testing.Testing() {
		panic("SetRelatedForTest called outside a test binary — relatedTestOverrides is test-only; register production RelatedDefs in the catalog instead")
	}
	for _, d := range defs {
		if d.Checker == nil {
			panic(fmt.Sprintf("SetRelatedForTest(%q): nil Checker for target %q — every RelatedDef must have a real checker", shortName, d.TargetType))
		}
		if d.TargetType == "" {
			panic(fmt.Sprintf("SetRelatedForTest(%q): empty TargetType — every RelatedDef must name a target", shortName))
		}
	}
	relatedTestOverridesMu.Lock()
	defer relatedTestOverridesMu.Unlock()
	existing := relatedTestOverrides[shortName] // nil when not yet set
	relatedTestOverridesPrev[shortName] = append(relatedTestOverridesPrev[shortName], existing)
	relatedTestOverrides[shortName] = defs
}

// GetRelated returns the related definitions for the given resource short
// name. relatedTestOverrides is a test-only override table: SetRelatedForTest
// / AppendRelated let a test substitute fake checkers for shortName, and that
// override — when present — takes precedence here. Every production
// registration lives in the catalog (core/catalog, installed via
// aws.Install), which GetRelated falls back to whenever no test override is
// active.
func GetRelated(shortName string) []RelatedDef {
	relatedTestOverridesMu.RLock()
	if defs, ok := relatedTestOverrides[shortName]; ok {
		relatedTestOverridesMu.RUnlock()
		return defs
	}
	relatedTestOverridesMu.RUnlock()
	if ct := TypeDef(shortName); ct != nil && len(ct.Related) > 0 {
		return ct.Related
	}
	return nil
}

// CleanupRelatedForTest restores the previous registration for the given short name
// (or deletes the entry entirely if no previous registration existed). Used only
// in tests for cleanup.
//
// Pops the most recently pushed snapshot from the per-key stack in
// relatedTestOverridesPrev. If the popped snapshot is nil (the key had no entry
// before the most recent Register/Append call), the active-registry entry is
// deleted entirely. If the stack is empty (Unregister called without a matching
// Register/Append), the entry is deleted as a safe fallback — the case for
// test-only types like `test_append`, `srcType`, and `resizeTestType` that
// were never registered before the test.
func CleanupRelatedForTest(shortName string) {
	relatedTestOverridesMu.Lock()
	defer relatedTestOverridesMu.Unlock()
	stack := relatedTestOverridesPrev[shortName]
	if len(stack) == 0 {
		delete(relatedTestOverrides, shortName)
		return
	}
	prev := stack[len(stack)-1]
	relatedTestOverridesPrev[shortName] = stack[:len(stack)-1]
	if prev == nil {
		delete(relatedTestOverrides, shortName)
	} else {
		relatedTestOverrides[shortName] = prev
	}
}

// FetchByIDsFunc fetches specific resource instances by ID, bypassing any
// filter the top-level paginated fetcher applies.
// Declaration lives in core/domain/contracts.go; this alias re-exports it.
type FetchByIDsFunc = domain.FetchByIDsFunc

// fetchByIDsRegistry maps target resource short name to its FetchByIDs helper.
var fetchByIDsRegistry = map[string]FetchByIDsFunc{}

// SetFetchByIDsForTest stores the FetchByIDs helper for the given target short
// name. Replaces any existing entry. Safe to call from an init() alongside
// SetPaginatedForTest.
func SetFetchByIDsForTest(shortName string, fn FetchByIDsFunc) {
	fetchByIDsRegistry[shortName] = fn
}

// GetFetchByIDs returns the FetchByIDs helper for the target short name.
// Test overrides via SetFetchByIDsForTest take effect first; otherwise reads
// the catalog FetchByIDs field.
func GetFetchByIDs(shortName string) FetchByIDsFunc {
	if fn, ok := fetchByIDsRegistry[shortName]; ok {
		return fn
	}
	if ct := TypeDef(shortName); ct != nil && ct.FetchByIDs != nil {
		return func(ctx context.Context, clients any, ids []string) ([]Resource, error) {
			if err := ClientMissing(ct, clients); err != nil {
				return nil, err
			}
			return ct.FetchByIDs(ctx, clients, ids)
		}
	}
	return nil
}

// CleanupFetchByIDsForTest removes the FetchByIDs helper for the given short
// name. Parity with CleanupRelatedForTest — used only in tests for cleanup,
// never from production code.
func CleanupFetchByIDsForTest(shortName string) {
	delete(fetchByIDsRegistry, shortName)
}

// SetNavigableFieldsForTest stores navigable field definitions for the given
// resource short name. Replaces any existing entry.
//
// The current value for shortName (which may be nil) is pushed onto a per-key
// stack in navigableFieldPrevious so that nested Register calls can all be
// rolled back in order by successive CleanupNavigableFieldsForTest calls.
//
// Contract: every Register MUST be paired with an Unregister, otherwise the
// per-key snapshot stack grows unbounded for the lifetime of the process. In
// practice every test that registers also unregisters via t.Cleanup; production
// callers register once at init and never unregister.
func SetNavigableFieldsForTest(shortName string, fields []NavigableField) {
	navigableFieldMu.Lock()
	defer navigableFieldMu.Unlock()
	existing := navigableFieldRegistry[shortName] // nil when not yet set
	navigableFieldPrevious[shortName] = append(navigableFieldPrevious[shortName], existing)
	navigableFieldRegistry[shortName] = fields
}

// GetNavigableFields returns the navigable field definitions for the given
// resource short name from the active registry. If the active registry has no
// entry for shortName, it falls back to the default (init-time) registry,
// then to the catalog. Returns nil only when no entry exists anywhere.
//
// Catalog-backed: catalog is checked after the active and default registries.
func GetNavigableFields(shortName string) []NavigableField {
	navigableFieldMu.RLock()
	if fields := navigableFieldRegistry[shortName]; len(fields) > 0 {
		navigableFieldMu.RUnlock()
		return fields
	}
	if fields := defaultNavFieldRegistry[shortName]; len(fields) > 0 {
		navigableFieldMu.RUnlock()
		return fields
	}
	navigableFieldMu.RUnlock()
	if ct := TypeDef(shortName); ct != nil && len(ct.Navigable) > 0 {
		return ct.Navigable
	}
	return nil
}

// GetActiveNavigableFields returns the navigable field definitions for the
// given resource short name from the active registry ONLY. Unlike
// GetNavigableFields, this function does NOT fall back to the default registry.
// Returns nil when no explicit SetNavigableFieldsForTest call has been made for
// shortName.
//
// Used by projection.buildItems (core/semantics/projection/generic.go) so
// that navigable affordances in the detail view require an explicit registration (from tests or from
// BootstrapActiveNavFields at app startup). This prevents init-time default
// entries from being visible in test models that deliberately omit nav fields.
func GetActiveNavigableFields(shortName string) []NavigableField {
	navigableFieldMu.RLock()
	defer navigableFieldMu.RUnlock()
	return navigableFieldRegistry[shortName]
}

// IsFieldNavigableForTest returns the NavigableField for the given field
// path, or nil if not registered. Test-only: no production caller —
// production reads the full set via GetActiveNavigableFields instead of
// probing one field path at a time.
func IsFieldNavigableForTest(shortName, fieldPath string) *NavigableField {
	for _, f := range GetNavigableFields(shortName) {
		if f.FieldPath == fieldPath {
			return &f
		}
	}
	return nil
}

// CleanupNavigableFieldsForTest removes the navigable field registration for the
// given short name. Used only in tests for cleanup.
//
// Pops the most recently pushed snapshot from the per-key stack in
// navigableFieldPrevious. If the popped snapshot is nil (the key had no entry
// before the most recent Register call), the active-registry entry is deleted
// entirely. If the stack is empty (Unregister called without a matching
// Register), the entry is deleted as a safe fallback.
func CleanupNavigableFieldsForTest(shortName string) {
	navigableFieldMu.Lock()
	defer navigableFieldMu.Unlock()
	stack := navigableFieldPrevious[shortName]
	if len(stack) == 0 {
		delete(navigableFieldRegistry, shortName)
		return
	}
	prev := stack[len(stack)-1]
	navigableFieldPrevious[shortName] = stack[:len(stack)-1]
	if prev == nil {
		delete(navigableFieldRegistry, shortName)
	} else {
		navigableFieldRegistry[shortName] = prev
	}
}

// GetDefaultNavFields returns the default (init-time) navigable field definitions
// for the given resource short name. Returns nil if none were registered at init.
// Used by NavFieldsProvider so that projection.GenericWithConfig always sees
// the canonical nav fields regardless of the active-registry state.
func GetDefaultNavFields(shortName string) []NavigableField {
	defaultNavFieldMu.RLock()
	if fields, ok := defaultNavFieldRegistry[shortName]; ok && len(fields) > 0 {
		defaultNavFieldMu.RUnlock()
		return fields
	}
	defaultNavFieldMu.RUnlock()
	if ct := TypeDef(shortName); ct != nil && len(ct.Navigable) > 0 {
		return ct.Navigable
	}
	return nil
}

// BootstrapActiveNavFields copies all entries from the default nav field
// registry into the active registry. Called once at app startup (from
// cmd/a9s/main.go) so that DetailModel navigability works in production.
// Must be called after all init() functions have run (i.e. inside main()).
// Noop in test binaries that never call this function.
func BootstrapActiveNavFields() {
	defaultNavFieldMu.RLock()
	snapshot := make(map[string][]NavigableField, len(defaultNavFieldRegistry))
	maps.Copy(snapshot, defaultNavFieldRegistry)
	defaultNavFieldMu.RUnlock()

	navigableFieldMu.Lock()
	defer navigableFieldMu.Unlock()
	for k, v := range snapshot {
		// Only populate entries that have not already been explicitly set via
		// SetNavigableFieldsForTest. This preserves test-supplied overrides when
		// BootstrapActiveNavFields is called inside tui.New (e.g. by golden
		// scenario helpers that register custom nav fields before constructing
		// the TUI model).
		if _, exists := navigableFieldRegistry[k]; !exists {
			navigableFieldRegistry[k] = v
		}
	}
}

// AppendRelated adds a single RelatedDef to the test-only override table for
// shortName. If the target type is already present, it is a no-op (prevents
// duplicates). If no override exists yet, it creates one. Panics outside a
// test binary, and at init-time (of a test binary) if def.Checker is nil or
// def.TargetType is empty — a nil Checker is a structural bug, not a
// supported stub state.
//
// Like SetRelatedForTest, the pre-append value is pushed onto the per-key
// snapshot stack so that a subsequent CleanupRelatedForTest restores the previous
// state (or deletes the entry, if no prior value existed). A duplicate-target
// no-op does NOT push a snapshot — Unregister has nothing to undo.
func AppendRelated(shortName string, def RelatedDef) {
	if !testing.Testing() {
		panic("AppendRelated called outside a test binary — relatedTestOverrides is test-only; register production RelatedDefs in the catalog instead")
	}
	if def.Checker == nil {
		panic(fmt.Sprintf("AppendRelated(%q): nil Checker for target %q — every RelatedDef must have a real checker", shortName, def.TargetType))
	}
	if def.TargetType == "" {
		panic(fmt.Sprintf("AppendRelated(%q): empty TargetType — every RelatedDef must name a target", shortName))
	}
	relatedTestOverridesMu.Lock()
	defer relatedTestOverridesMu.Unlock()
	existing := relatedTestOverrides[shortName]
	for _, d := range existing {
		if d.TargetType == def.TargetType {
			return // already registered, skip duplicate
		}
	}
	relatedTestOverridesPrev[shortName] = append(relatedTestOverridesPrev[shortName], existing)
	relatedTestOverrides[shortName] = append(existing, def)
}

// CTRegionFilterKey carries the Region a CloudTrail filter must be looked up
// in. It is not a LookupAttribute: the CloudTrail fetchers strip it and send
// the request to that Region's endpoint instead.
const CTRegionFilterKey = "_region"

// CTAltNameFilterKey carries the other spelling of the resource the lookup
// names — the ARN when the key sends the bare name, the bare name when it
// sends the ARN. CloudTrail records a resource under one or the other per API
// call, so a lookup that comes back empty can be worth one retry under this
// value. Like CTRegionFilterKey it is not a LookupAttribute.
const CTAltNameFilterKey = "_altname"

// CTQualifierFilterKey carries the parent the row belongs to, and
// CTQualifierPathsKey the comma-separated event-JSON paths that name the
// parent of what an event acted on. A name AWS scopes to a parent is unique
// nowhere else, so an event naming a different parent is another row's; like
// the two keys above these are a9s's own and name no LookupAttribute.
const (
	CTQualifierFilterKey = "_qualifier"
	CTQualifierPathsKey  = "_qualifierpaths"
)

// BuildCloudTrailFilter returns the CloudTrail LookupEvents filter for a resource.
// The filter is determined by the resource type's CloudTrailKey field, not by heuristics.
// Returns nil when the resource type has no CloudTrail support (empty CloudTrailKey).
func BuildCloudTrailFilter(res Resource, resourceType string) map[string]string {
	rt := FindResourceType(resourceType)
	if rt == nil || rt.CloudTrailKey == "" {
		return nil
	}
	filter := buildFilterFromKey(res, rt.CloudTrailKey)
	if filter == nil {
		return nil
	}
	if rt.CloudTrailRegion != nil {
		if region := rt.CloudTrailRegion(res); region != "" {
			filter[CTRegionFilterKey] = region
		}
	}
	if alt := ctAltName(res, filter); alt != "" {
		filter[CTAltNameFilterKey] = alt
	}
	if q := rt.CloudTrailQualifier; q.ParentField != "" && len(q.EventPaths) > 0 {
		if parent := res.Fields[q.ParentField]; parent != "" {
			filter[CTQualifierFilterKey] = parent
			filter[CTQualifierPathsKey] = strings.Join(q.EventPaths, ",")
		}
	}
	return filter
}

// ctAltName returns the spelling of res that a ResourceName lookup is not
// already sending: its ARN when the filter carries the bare name, its ID when
// the filter carries the ARN. Empty when the row holds only one of the two, or
// when the lookup is not by resource name — a Username is a caller, and an ARN
// is not another spelling of one.
func ctAltName(res Resource, filter map[string]string) string {
	sent, byName := filter["ResourceName"]
	if !byName {
		return ""
	}
	arn := res.Fields["arn"]
	if sent != arn {
		return arn
	}
	if sent != res.ID {
		return res.ID
	}
	return ""
}

func buildFilterFromKey(res Resource, ctKey string) map[string]string {
	parts := strings.SplitN(ctKey, ":", 2)
	if len(parts) != 2 {
		return nil
	}
	attr, source := parts[0], parts[1]

	var val string
	switch source {
	case "ID":
		val = res.ID
	case "Name":
		val = res.Name
	default:
		if key, ok := strings.CutPrefix(source, "Fields."); ok {
			val = res.Fields[key]
		}
	}
	if val == "" {
		return nil
	}
	return map[string]string{attr: val}
}
