package runtime

import (
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
