package runtime

import (
	"context"
	"time"
)

// TaskKind names a family of background tasks ("enrich", "related",
// "logsource-discover", future query scans, …). Concrete kind constants
// land alongside the per-handler PRs that own each task family.
type TaskKind string

const (
	// TaskKindProbeAvailability asks the adapter to run a Wave-1 availability
	// probe for the resource type named by TaskKey.Scope.
	TaskKindProbeAvailability TaskKind = "probe-availability"

	// TaskKindProbeEnrich asks the adapter to run a Wave-2 enrichment probe
	// for the resource type named by TaskKey.Scope.
	TaskKindProbeEnrich TaskKind = "probe-enrich"

	// TaskKindSaveCache asks the adapter to persist the current availability
	// state to disk. TaskKey.Scope is empty.
	TaskKindSaveCache TaskKind = "save-cache"
)

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

// TaskPayload is the marker interface for typed per-Kind task payloads.
// Each TaskKind defines its own struct that satisfies this interface so
// adapters can type-switch on Payload to recover every input the runtime
// captured at dispatch — without re-deriving any state from the opaque
// TaskKey.Scope or accepting parameters out-of-band.
//
// nil Payload is permitted for tasks whose Kind needs no extra data.
type TaskPayload interface {
	isTaskPayload()
}

// TaskRequest is what the runtime returns from HandleEvent to ask the
// adapter to start (or refresh) a background task. The adapter is
// responsible for translating it into platform-specific async work and
// emitting the corresponding events back into the runtime.
//
// Payload carries the structured per-Kind data the adapter needs to
// execute the task (resource pointers, cursors, target types, …). The
// adapter type-switches on Payload to recover the typed fields. This
// keeps Scope as a pure dedup key (opaque to the adapter) and lets
// sibling per-handler PRs (probes, related, fetchers, …) define their
// own payload variants without growing this struct or threading
// out-of-band parameters into the dispatch path.
type TaskRequest struct {
	Key     TaskKey
	Cache   CachePolicy
	Payload TaskPayload
}
