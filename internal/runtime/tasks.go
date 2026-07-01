package runtime

import (
	"context"
	"time"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
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

	// TaskKindConnect asks the adapter to start an AWS connect attempt for the
	// profile/region carried by ConnectPayload, tagged with the current
	// session ConnectGen so a slow-arriving ClientsReadyMsg can be rejected
	// when the user switches again before this one returns.
	TaskKindConnect TaskKind = "connect"

	// TaskKindFetchIdentity asks the adapter to fetch the caller-identity
	// record for the current session and dispatch it back as an
	// IdentityLoadedMsg.
	TaskKindFetchIdentity TaskKind = "fetch-identity"

	// TaskKindLoadAvailCache asks the adapter to load the on-disk availability
	// cache for the current profile/region and dispatch the result back as
	// AvailabilityCacheLoadedMsg.
	TaskKindLoadAvailCache TaskKind = "load-avail-cache"

	// TaskKindDemoPrefetchCounts asks the adapter to run the demo-mode
	// synchronous prefetch that emits AvailabilityPrefetchedMsg. Used in
	// --no-cache and --demo paths to avoid the async probe pipeline.
	TaskKindDemoPrefetchCounts TaskKind = "demo-prefetch-counts"

	// TaskKindFlashTick asks the adapter to schedule a ClearFlashMsg dispatch
	// after FlashTickPayload.Duration. The payload's Gen is the flash gen the
	// scheduled dispatch should reference so a stale tick is rejected.
	TaskKindFlashTick TaskKind = "flash-tick"

	// TaskKindEmitNavigate asks the adapter to dispatch a one-shot
	// NavigateMsg derived from the -c CLI flag. Used by HandleClientsReady
	// on first connect when Command is set and the user is still on the
	// main menu.
	TaskKindEmitNavigate TaskKind = "emit-navigate"

	// TaskKindEmitAPIError asks the adapter to dispatch a messages.APIError
	// back through the input pipeline, used by HandleClientsReady's
	// impossible "wrong concrete type on ClientsReadyMsg.Clients" branch to
	// route the error through HandleAPIError's classification flow.
	TaskKindEmitAPIError TaskKind = "emit-api-error"

	// TaskKindFetchChildResources asks the adapter to run the paginated
	// child-resource fetcher for the (childType, parentContext) carried by
	// FetchChildResourcesPayload and dispatch the result as
	// messages.ResourcesLoaded. Emitted by HandleEnterChildView.
	TaskKindFetchChildResources TaskKind = "fetch-child-resources"

	// TaskKindReadThemeFile asks the adapter to resolve the theme file path
	// via config.ThemePath and read the YAML bytes from disk, then dispatch
	// the bytes back as messages.ThemeFileRead so HandleThemeFileRead can
	// emit the apply/pop/flash/save sequence. Emitted by HandleThemeSelected.
	TaskKindReadThemeFile TaskKind = "read-theme-file"

	// TaskKindSaveThemeConfig asks the adapter to persist the theme choice
	// to config.yaml via config.SaveTheme. Emitted by HandleThemeFileRead on
	// the success branch alongside the ApplyThemeIntent + PopSelectorIntent
	// + success FlashIntent. Save failures surface adapter-side as a
	// messages.Flash error; the in-memory theme remains applied.
	TaskKindSaveThemeConfig TaskKind = "save-theme-config"
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

// ConnectPayload carries the profile/region/gen the adapter must use when
// starting an AWS connect attempt. The gen is captured at dispatch time so
// the resulting ClientsReadyMsg can be rejected by the gen guard if the
// user switches again before the connect returns.
type ConnectPayload struct {
	Profile string
	Region  string
	Gen     domain.Gen
}

func (ConnectPayload) isTaskPayload() {}

// FetchIdentityPayload carries no fields — fetching the caller identity
// reads directly from the live session.Session via the adapter. The empty
// struct exists so the runtime can request the task via the typed
// TaskRequest contract instead of an opaque TaskKey alone.
type FetchIdentityPayload struct{}

func (FetchIdentityPayload) isTaskPayload() {}

// LoadAvailCachePayload carries no fields — the adapter resolves the
// on-disk cache path from the live session profile/region.
type LoadAvailCachePayload struct{}

func (LoadAvailCachePayload) isTaskPayload() {}

// DemoPrefetchCountsPayload carries no fields — the adapter runs the demo
// fixture prefetch using its retained PreSuppliedClients reference.
type DemoPrefetchCountsPayload struct{}

func (DemoPrefetchCountsPayload) isTaskPayload() {}

// FlashTickPayload carries the duration the adapter should sleep before
// emitting a ClearFlashMsg, and the flash gen that ClearFlashMsg should
// reference. The runtime owns the gen (computed at dispatch time) so the
// stale-clear guard works the same as it did before extraction.
type FlashTickPayload struct {
	Gen      domain.Gen
	Duration time.Duration
}

func (FlashTickPayload) isTaskPayload() {}

// EmitNavigatePayload carries the one-shot navigation the adapter must
// dispatch as messages.Navigate on first successful connect, derived
// from the -c / Command field on the session. Target is the runtime-owned
// NavigateTarget (translated to messages.ViewTarget by the adapter).
type EmitNavigatePayload struct {
	Target       NavigateTarget
	ResourceType string
}

func (EmitNavigatePayload) isTaskPayload() {}

// EmitAPIErrorPayload carries the error the adapter must dispatch as
// messages.APIError. Used by HandleClientsReady's impossible
// "wrong concrete type on Clients" branch.
type EmitAPIErrorPayload struct {
	Err error
}

func (EmitAPIErrorPayload) isTaskPayload() {}

// FetchChildResourcesPayload carries the child-type short name and the
// parent context map used by the adapter's paginated child fetcher.
// Emitted by HandleEnterChildView so the adapter's tasksToCmd can build
// the existing fetchChildResources closure without parsing TaskKey.Scope.
type FetchChildResourcesPayload struct {
	ChildType     string
	ParentContext map[string]string
}

func (FetchChildResourcesPayload) isTaskPayload() {}

// ReadThemePayload carries the theme filename whose YAML the adapter
// must read from disk. The adapter resolves the absolute path via
// config.ThemePath and reads via os.ReadFile, dispatching the result
// (or read error) back as messages.ThemeFileRead.
type ReadThemePayload struct {
	Theme string
}

func (ReadThemePayload) isTaskPayload() {}

// SaveThemeConfigPayload carries the theme filename the adapter must
// persist to config.yaml via config.SaveTheme. Emitted by
// HandleThemeFileRead's success branch after the apply/pop/flash
// intents. A save failure surfaces as a Flash; the in-memory theme
// remains applied (the theme is applied before the config save, so a save
// failure does not revert it).
type SaveThemeConfigPayload struct {
	Theme string
}

func (SaveThemeConfigPayload) isTaskPayload() {}

// RelatedCheckPayload carries the source resource the executor needs to
// invoke RelatedDef.Checker. The Scope field on the parent TaskKey
// ("type/id") is the dedup key; this struct carries the full resource so
// runRelatedCheckers can pass it to each checker without re-fetching.
type RelatedCheckPayload struct {
	ResourceType string
	Resource     resource.Resource
}

func (RelatedCheckPayload) isTaskPayload() {}
