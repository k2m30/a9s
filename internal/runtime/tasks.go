package runtime

import (
	"context"
	"time"
)

// TaskKind names a family of background tasks ("enrich", "related",
// "logsource-discover", future query scans, …). Concrete kind constants
// land alongside the per-handler PRs that own each task family.
type TaskKind string

// TaskKey uniquely identifies one background task. Scope is kind-specific
// (resource-type, resource-id, query-id, …) and is opaque to the
// scheduler — equality on TaskKey is the dedup key.
type TaskKey struct {
	Kind  TaskKind
	Scope string
}

// TaskStatus tracks the lifecycle of a background task. Adapters render
// progress and final state from this value plus TaskState.Progress.
type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskRunning
	TaskPartial
	TaskComplete
	TaskFailed
	TaskCancelled
)

// CachePolicy controls how long a task's result remains valid before a
// fresh request must be issued. CacheUntilRotate ties freshness to
// session.Session.Rotate (profile / region switch), matching the
// generation-counter contract used elsewhere in the runtime.
type CachePolicy int

const (
	CacheNone CachePolicy = iota
	CacheSession
	CacheUntilRotate
)

// TaskState is the runtime-owned record for one in-flight or completed
// background task. Cancel is non-nil while Status is TaskPending,
// TaskRunning, or TaskPartial; nil after a terminal status.
type TaskState struct {
	Key       TaskKey
	Status    TaskStatus
	Progress  float64 // 0..1; -1 = indeterminate
	StartedAt time.Time
	Cache     CachePolicy
	Cancel    context.CancelFunc
	Err       error
}

// TaskRequest is what the runtime returns from HandleEvent to ask the
// adapter to start (or refresh) a background task. The adapter is
// responsible for translating it into platform-specific async work and
// emitting the corresponding events back into the runtime.
type TaskRequest struct {
	Key   TaskKey
	Cache CachePolicy
}
