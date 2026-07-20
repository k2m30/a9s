// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

import (
	"time"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// UIIntent is the contract by which the runtime tells an adapter to
// update UI state. Every intent is a closed type — adapters apply them
// by type-switching on the concrete variants below.
//
// The runtime returns []UIIntent + []TaskRequest from HandleEvent; the
// adapter walks its view tree applying matching intents and turns each
// TaskRequest into platform-specific async work. This preserves today's
// stack-walking semantics (every matching view in the stack receives the
// update) without exposing renderer types to the shared core.
type UIIntent interface {
	isIntent()
}

// IssueBadgePatch describes the issue-badge state for a single resource
// type. Used by both PatchMenu (top-level menu badge) and embedded inside
// PatchResourceList (list-view badge).
type IssueBadgePatch struct {
	Count     int
	Truncated bool
}

// ListEnrichmentPatch carries Wave 2 enrichment data for the rows of a
// resource-list view. Findings is keyed by Resource.ID; nil means clear.
// AttentionDetails is keyed by Resource.ID then by the owning Finding's
// Code (mirrors messages.EnrichmentChecked.AttentionDetails) — the adapter
// folds both directly onto cached rows (applyRowFindings/
// runtime.ApplyWave2ToRow) so a multi-condition resource keeps every
// independently-evaluated Finding, each with its own AttentionDetail rows.
type ListEnrichmentPatch struct {
	Findings         map[string][]domain.Finding
	AttentionDetails map[string]map[domain.FindingCode]domain.AttentionDetail
	TruncatedIDs     map[string]bool
	FieldUpdates     map[string]map[string]string
}

// PatchResourceList instructs the adapter to apply the contained patches
// to every resource-list view for ResourceType. Nil sub-patches are
// no-ops; non-nil sub-patches replace the corresponding state.
type PatchResourceList struct {
	ResourceType string
	Issues       *IssueBadgePatch
	Enrichment   *ListEnrichmentPatch
}

func (PatchResourceList) isIntent() {}

// PatchDetail instructs the adapter to apply the contained patches to
// every detail view matching ResourceType (and ResourceID, if non-empty).
// Empty ResourceID targets all detail views of the given type.
type PatchDetail struct {
	ResourceType string
	ResourceID   string
	Findings     []domain.Finding
	Attention    map[domain.FindingCode]domain.AttentionDetail
	FieldUpdates map[string]string
	// EnrichmentFindings carries EVERY independently-evaluated Wave-2 Finding
	// per resource used by the adapter to look up the findings for a specific
	// detail view's resource. Keyed by resource.Resource.ID; nil/empty means
	// clear enrichment. An already-open detail must show every
	// independently-evaluated condition, not just the worst-severity one, so
	// this carries the full per-resource slice (mirrors
	// messages.EnrichmentChecked.Findings) rather than a single
	// representative.
	EnrichmentFindings map[string][]domain.Finding
	// EnrichmentAttentionDetails carries the supporting rows for each per-
	// resource Wave-2 finding, keyed by Resource.ID then by the owning
	// Finding's Code at the message-emission boundary. Paired with
	// EnrichmentFindings (same Resource.ID set) — each finding in that slice
	// looks up its own AttentionDetail here by its Code.
	EnrichmentAttentionDetails map[string]map[domain.FindingCode]domain.AttentionDetail
}

func (PatchDetail) isIntent() {}

// PatchMenu instructs the adapter to update the top-level menu badge for
// ResourceType.
type PatchMenu struct {
	ResourceType string
	Issues       int
	Truncated    bool
}

func (PatchMenu) isIntent() {}

// PushScreen asks the adapter to materialize and push the named screen
// onto its view stack with the given context.
//
// Payload carries per-screen typed data (selectors, reveal results,
// child-list parameters). It is nil for capability screens whose
// adapter-side builder resolves everything from ScreenContext alone.
// Callers that emit PushScreen with Context only continue to work because
// the builder closures type-switch on Payload and tolerate the zero value
// when their ScreenID does not require it.
type PushScreen struct {
	ID      ScreenID
	Context ScreenContext
	Payload ScreenPayload
}

func (PushScreen) isIntent() {}

// ApplyThemeIntent asks the adapter to swap the active theme and
// invalidate any caches that materialised colour-dependent state
// (header text, per-row styled caches in ResourceListModel views, …).
//
// The runtime carries the parsed YAML *bytes* and the theme filename, and
// the adapter re-parses via
// styles.ThemeFromYAML before applying. This keeps the runtime free of
// any lipgloss / Bubble Tea coupling that hosting a *styles.Theme
// would force. Option A (extracting a domain.Theme value type) was
// considered cleaner but is a larger refactor properly scoped to a
// follow-on PR; B preserves the boundary invariant today.
//
// On parse failure the adapter MUST emit a flash describing the
// failure and skip the apply; the persist task that this intent ships
// with continues to fire because the runtime already committed to the
// new theme name. (Save-fail surfaces a separate adapter-side flash
// from the SaveThemeConfig task.)
type ApplyThemeIntent struct {
	Bytes []byte
	Name  string
}

func (ApplyThemeIntent) isIntent() {}

// PopScreen asks the adapter to pop the topmost screen off its view
// stack. The runtime owns no view stack itself; this is purely a
// renderer-shaped instruction.
type PopScreen struct{}

func (PopScreen) isIntent() {}

// ReplaceScreen asks the adapter to replace the topmost screen with the
// named screen and the given context.
type ReplaceScreen struct {
	ID      ScreenID
	Context ScreenContext
}

func (ReplaceScreen) isIntent() {}

// OriginCache and OriginVerified are the two valid values for
// PatchMenuAvailability.Origin — a cross-layer contract shared by every
// Core handler that sets Origin and every adapter that reads it (C3).
// Defined as constants (rather than inline string literals) so a typo in
// either producer or consumer fails to compile instead of silently landing
// on the "leave origin unchanged" empty-string branch.
const (
	OriginCache    = "cache"
	OriginVerified = "verified"
)

// PatchMenuAvailability updates the resource-count display for one menu entry.
// Combines SetAvailability + SetTruncated into a single intent.
type PatchMenuAvailability struct {
	ResourceType string
	Count        int
	Truncated    bool
	// Origin distinguishes a disk-cache-seeded entry ("cache", not yet
	// re-verified this session) from one confirmed by a live probe this
	// session ("verified") — C3. Empty means "leave the stored origin
	// unchanged" (used by callers that only ever touch Count/Truncated).
	Origin string
}

func (PatchMenuAvailability) isIntent() {}

// PatchMenuIssueBatch applies a batch of cached issue counts to the main menu.
// Used on cache load to apply all stored issue counts atomically.
type PatchMenuIssueBatch struct {
	Counts    map[string]int
	Truncated map[string]bool
	Known     map[string]bool
}

func (PatchMenuIssueBatch) isIntent() {}

// PatchMenuCheckProgress updates the Wave-1 availability-scan progress indicator.
// Total=0 signals "scan complete" (clear the indicator).
type PatchMenuCheckProgress struct {
	Checked int
	Total   int
}

func (PatchMenuCheckProgress) isIntent() {}

// PatchMenuEnrichProgress updates the Wave-2 enrichment progress indicator.
// Total=0 signals "enrichment complete" (clear the indicator).
type PatchMenuEnrichProgress struct {
	Checked int
	Total   int
}

func (PatchMenuEnrichProgress) isIntent() {}

// FlashIntent emits a transient notification to the adapter's status bar.
type FlashIntent struct {
	Text    string
	IsError bool
}

func (FlashIntent) isIntent() {}

// ClearFlash clears the active flash message from the adapter's status bar.
type ClearFlash struct{}

func (ClearFlash) isIntent() {}

// SetErrorHintIntent toggles the persistent "errors visible — press ! to view"
// hint shown after an error flash auto-clears (the adapter's showErrorHint
// flag). HandleClearFlash emits this when the cleared flash carried an error.
type SetErrorHintIntent struct {
	Show bool
}

func (SetErrorHintIntent) isIntent() {}

// AppendErrorHistoryIntent appends one entry to the adapter's per-session
// error log used by the `!` overlay. HandleFlash / HandleAPIError /
// HandleClientsReady emit this in error paths so the history matches the
// flash text the user saw.
type AppendErrorHistoryIntent struct {
	Time    time.Time
	Message string
}

func (AppendErrorHistoryIntent) isIntent() {}

// ClearActiveListLoadingIntent tells the adapter to clear the loading
// indicator on the currently-active resource-list view (if any). Emitted by
// HandleAPIError so a failed AWS call removes the spinner immediately rather
// than waiting for the next render.
//
// Err carries the error-marker text (C4): when non-empty, the adapter
// must also clear the active list's Refreshing flag and set its
// LastFetchError to Err, so cached content stays on screen with the marker
// swapped from "refreshing" to "error" instead of going blank. Empty when
// the failure is not list-scoped (matches the pre-existing spinner-only
// behavior for other HandleAPIError call sites).
type ClearActiveListLoadingIntent struct {
	Err string
}

func (ClearActiveListLoadingIntent) isIntent() {}

// MenuClearAvailabilityIntent tells the adapter to reset the main-menu
// availability counts before a profile/region switch reconnect. Emitted by
// HandleProfileSelected / HandleRegionSelected so the menu shows the
// "checking…" state until the new session reports back.
type MenuClearAvailabilityIntent struct{}

func (MenuClearAvailabilityIntent) isIntent() {}

// PopSelectorIntent tells the adapter to pop the top view from its stack if
// (and only if) that view is the profile/region/theme selector. Emitted by
// the selector-confirm handlers so they do not need to inspect renderer
// state to know whether the selector is on screen.
type PopSelectorIntent struct{}

func (PopSelectorIntent) isIntent() {}

// RefreshActiveListIntent tells the adapter to re-fetch the currently-active
// resource-list view, if there is one. Emitted by HandleClientsReady when
// PendingRefresh is set so a successful post-switch connect refreshes the
// list the user was viewing.
type RefreshActiveListIntent struct{}

func (RefreshActiveListIntent) isIntent() {}

// PatchResourceCache writes a single top-level resource-list entry into
// RowStore (via Core.SetResourceCache). Emitted by HandleResourcesLoaded
// when the loaded slice should be cached without the view-side
// write-through path (i.e. the active view is not the ResourceListModel for
// this type, so cacheTopLevelResourceList will not fire). Entry may be nil
// to signal a clear (no current emitter exercises that branch — kept for
// symmetry).
//
// Entry is *domain.ListViewCacheEntry; adapters apply the intent via
// Core.SetResourceCache (RowStore-backed — task #17 wave 1 stage 3).
type PatchResourceCache struct {
	ResourceType string
	Entry        *domain.ListViewCacheEntry
}

func (PatchResourceCache) isIntent() {}

// PatchRelatedCache appends one resolved related-check result to the
// session-owned RelatedCache. Emitted by HandleRelatedCheckResult after
// resolving the source resource ID; the adapter applies by appending to
// any existing slice under the runtime.RelatedCacheKey.
type PatchRelatedCache struct {
	ResourceType   string
	SourceID       string
	DefDisplayName string
	Result         resource.RelatedCheckResult
}

func (PatchRelatedCache) isIntent() {}

// PatchLazyResourceCache merges sparse lazy-added resources into the
// session-owned LazyResourceCache. Keys in Adds may be aliases — Core
// canonicalises before emitting, so adapters store the canonical
// ShortName verbatim. De-dup by Resource.ID is performed by Core; the
// adapter overwrites the slice for each key.
type PatchLazyResourceCache struct {
	Adds map[string][]resource.Resource
}

func (PatchLazyResourceCache) isIntent() {}

// SetIdentityIntent carries the resolved caller-identity mirror to the
// adapter. The runtime writes session.Identity (still typed as
// *awsclient.CallerIdentity) before emitting; this
// intent gives the renderer a renderer-shaped value to apply to active
// views (today: IdentityModel.SetIdentity) without importing core/aws.
// nil Identity is permitted and signals a no-op render-side update
// (the session field is already cleared by Core in that path).
type SetIdentityIntent struct {
	Identity *domain.CallerIdentity
}

func (SetIdentityIntent) isIntent() {}

// ClearIdentityIntent tells the adapter to discard any resolved identity for
// the outgoing profile/region pair. Emitted unconditionally by
// HandleProfileSelected / HandleRegionSelected (never by a same-pair refresh,
// e.g. menu Ctrl+R's MenuClearAvailabilityIntent) so a stale ARN from the
// previous pair can never be copied or displayed before the new pair's
// identity fetch (if any) lands.
type ClearIdentityIntent struct{}

func (ClearIdentityIntent) isIntent() {}

// HeaderInvalidateIntent asks the adapter to bust the cached header so
// the next View() pass recomputes the badge / role / right-side string.
// Emitted alongside SetIdentityIntent on successful identity resolution
// because account alias + identity name are inputs to the header cache
// key — clearing the cache key forces a re-render with the fresh data.
type HeaderInvalidateIntent struct{}

func (HeaderInvalidateIntent) isIntent() {}
