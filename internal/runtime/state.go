package runtime

import (
	"github.com/k2m30/a9s/v3/internal/domain"
)

// RuntimeState is the view-ready snapshot that adapters render from when
// they need a one-shot read of runtime state (initial mount, debug
// overlays, future IPC bridges). It is a snapshot, not a live handle —
// adapters should still react to UIIntent for incremental updates.
type RuntimeState struct {
	// ResourceCache mirrors RowStore's retained full (non-Partial) entries
	// for the active session: per-resource-type cached list state.
	ResourceCache map[string]*domain.ListViewCacheEntry

	// MenuBadges is the current per-resource-type issue badge state used
	// to render the main menu.
	MenuBadges map[string]IssueBadgePatch

	// DetailFindings is the most-recent finding set for the focused
	// detail view, keyed by FindingCode for stable ordering.
	DetailFindings   []domain.Finding
	DetailAttention  map[domain.FindingCode]domain.AttentionDetail
	DetailResourceID string

	// Tasks is the runtime's view of every in-flight or recently-
	// completed background task. Keyed by TaskKey.
	Tasks map[TaskKey]TaskState

	// AvailabilityProgress / EnrichmentProgress mirror the queue
	// progress counters on session.Session for adapters that render a
	// progress UI.
	AvailabilityChecked int
	AvailabilityTotal   int
	EnrichmentChecked   int
	EnrichmentTotal     int
}
