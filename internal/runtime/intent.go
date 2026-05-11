package runtime

import (
	"time"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
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
type ListEnrichmentPatch struct {
	Findings     map[string]resource.EnrichmentFinding
	TruncatedIDs map[string]bool
	FieldUpdates map[string]map[string]string
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
	// EnrichmentFindings carries the Wave-2 per-resource finding map used by
	// the adapter to look up the finding for a specific detail view's resource.
	// Keyed by resource.Resource.ID; nil means clear enrichment.
	EnrichmentFindings map[string]resource.EnrichmentFinding
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
type PushScreen struct {
	ID      ScreenID
	Context ScreenContext
}

func (PushScreen) isIntent() {}

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

// PatchMenuAvailability updates the resource-count display for one menu entry.
// Combines SetAvailability + SetTruncated into a single intent.
type PatchMenuAvailability struct {
	ResourceType string
	Count        int
	Truncated    bool
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
type ClearActiveListLoadingIntent struct{}

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
