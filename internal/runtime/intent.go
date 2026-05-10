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
	Findings map[string]resource.EnrichmentFinding
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
