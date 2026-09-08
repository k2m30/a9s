// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"sync"
	"time"

	"github.com/k2m30/a9s/v3/core/session"

	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// Controller is the headless app controller. It wraps runtime.Core and
// owns the screen stack.
//
// Both renderers (TUI and web) hold a *Controller. The TUI calls Apply
// for key-driven actions and Handle for async Task results; the web
// renderer does the same over HTTP/SSE. Tests use DrainSync to drive the
// controller synchronously without a terminal or sleep.
//
// Task ownership: Apply and Handle RETURN []runtime.TaskRequest to the
// caller. The controller retains no task state — the host (TUI, web, or
// DrainSync) is responsible for executing and routing task results.
//
// Concurrency: mu guards stack and all map fields. Public mutating methods
// (Apply, Handle, ApplyIntents, ApplyEnrichmentState, RegisterFallbackTypeDef,
// etc.) acquire a write lock on entry. Public read-only methods (GetMenu*,
// GetList*) acquire a read lock. Snapshot acquires a WRITE lock — despite
// being read-only from the caller's perspective, it may populate a list
// screen's buildListBody memo cache (list_body.go), which is a mutation of
// ListState.bodyMemo/rowsVersion. Internal helpers called while a lock is
// already held must NOT lock — Go mutexes are not reentrant.
//
// Handle releases the lock BETWEEN its mutations and its snapshot, to build
// the list body for a freshly absorbed result outside it (captureTopListBodyBuild
// -> listBodyBuild.run -> installListBodyMemo, C4's absorb half). Everything
// that build reads is frozen under the lock first, and the memo it produces is
// installed only while it still describes the screen — so the lock is held for
// the swap, never for the row pass. That is why a row's contents are replaced
// rather than written into anywhere in this package: the build reads a shallow
// copy of a row set while other writers keep working.
type Controller struct {
	mu    sync.RWMutex
	core  *runtime.Core
	stack []Screen

	// enrichmentStore stores Wave-2 per-resource findings per resource type,
	// keyed by canonical short name then Resource.ID. A resource with more
	// than one independently-evaluated Wave-2 condition keeps every one of
	// them in its per-ID slice — never collapsed to a single worst-severity
	// representative. Populated by ApplyEnrichmentState.
	enrichmentStore map[string]map[string][]domain.Finding

	// enrichmentDetails stores Wave-2 per-resource AttentionDetail per
	// resource type, keyed by canonical short name then Resource.ID then the
	// owning Finding's Code — so a resource with more than one
	// independently-evaluated condition keeps every condition's own
	// supporting rows. Delivered alongside enrichmentStore's findings in the
	// same EnrichmentChecked payload; kept separate so a findings-only
	// re-apply (e.g. a fresh-fetch re-apply that only needs the findings for
	// row glyphs) can still recover the paired AttentionDetail without
	// re-deriving it.
	enrichmentDetails map[string]map[string]map[domain.FindingCode]domain.AttentionDetail

	// enrichmentTruncated stores the truncation flag per resource type from
	// ApplyEnrichmentState, parallel to enrichmentStore.
	enrichmentTruncated map[string]bool

	// enrichmentGen counts every mutation of enrichmentStore/enrichmentDetails/
	// enrichmentTruncated: applyEnrichmentState's write and the profile/
	// region-rotation reset in intents.go's MenuClearAvailabilityIntent case.
	// buildListBody's memo (list_body.go) includes this in its cache key
	// because resolveListRowSeverity and the S4 status-cell override read
	// these maps directly — a change invisible to a ListState's own
	// rowsVersion, since the maps are controller-level, not per-screen.
	enrichmentGen uint64

	// viewConfig is the per-session view configuration used by
	// resolveListColumnsForBuild to pick the correct column set for each
	// resource type. When nil, the built-in defaults are used. Set by
	// SetViewConfig after construction.
	viewConfig *config.ViewsConfig

	// fallbackTypeDefs stores ResourceTypeDef for resource types that are not
	// registered in the catalog (e.g. unit-test minimalTypeDef types). Both
	// buildListBody (for columns) and GetListIssueCount (for Color func) consult
	// this map when FindResourceType returns nil. Populated by RegisterFallbackTypeDef.
	fallbackTypeDefs map[string]resource.ResourceTypeDef

	// lastCostsNow is the clock most recently injected into a CostsState by
	// ensureCostsState, retained after the costs screen itself is popped —
	// ApplyCostsLoaded's fully-popped fallback (no CostsState survives to
	// own an injected Now) uses this instead of a raw wall-clock read, so a
	// late delivery's Store.Merge timestamp stays consistent with whatever
	// clock the rest of the session was using. Zero value (never seeded)
	// falls back to time.Now() at the one call site that reads it.
	lastCostsNow time.Time

	// costsDirtyStore is set by ApplyCostsLoaded (under c.mu) in place of
	// calling Store.Save() synchronously — the yaml.Marshal+os.WriteFile it
	// performs must not run while c.mu is held, since that would block every
	// other action against the controller for the duration of a disk write.
	// Handle() reads and clears this field under lock, then flushes it after
	// unlocking. nil means nothing is pending.
	costsDirtyStore *costs.Store

	// pendingCacheWrites is the one ordered queue of disk saves for this
	// controller: the rows-carrying per-type save and the counts-only
	// availability save both stage here, so the two writers of a type file
	// land in the order they were decided rather than racing (a restart would
	// otherwise read whichever finished last). Inputs are frozen under c.mu;
	// the write runs on the cache writer's goroutine (C4) — the same handoff
	// costsDirtyStore performs for the costs cache. Staged by stageCacheWrite
	// (both lanes), performed by runCacheWriteLoop.
	// Guarded by cacheWriteMu, not c.mu.
	pendingCacheWrites []cacheWrite

	// identityResult holds the resolved caller identity received via
	// messages.IdentityLoaded so snapshot can build IdentityBody without
	// importing core/aws or touching the TUI view stack.
	identityResult *domain.CallerIdentity

	// identityLoading is true from ActionOpenIdentity dispatch until either
	// messages.IdentityLoaded or messages.IdentityError arrives in Handle.
	identityLoading bool

	// identityErrMsg is non-empty when the identity fetch has failed.
	identityErrMsg string

	// flash holds the transient status-bar notification surfaced via FlashIntent
	// (e.g. an API error). snapshot() emits it as Header.Flash; it is cleared at
	// the start of each user Apply so it persists until the next action.
	flash Flash

	// errorHistory records one entry per error flash the controller applies.
	// The '!' / open-error-log action renders these newest-first as a text screen.
	errorHistory []controllerErrorEntry

	// showErrorHint is true after an error flash clears (SetErrorHintIntent{Show:true})
	// and cleared on any subsequent action. Surfaced as Header.ErrorHintVisible in snapshot().
	showErrorHint bool

	// uiMode is the renderer mode surfaced as Header.Mode ("" = TUI, "web").
	// Set once via SetUIMode after construction (e.g. core/web/construct.go
	// for web sessions). The TUI never calls SetUIMode, so it stays "". This is
	// independent of core.IsDemo() — a demo session is still a TUI or web
	// session and carries its own Header.Mode value ("demo") set elsewhere.
	uiMode string

	// menuSweepAcked tracks, per resource type, whether an AvailabilityChecked
	// result has landed for a type currently retained in
	// core.Session().ProbeResources. MenuBody.Refreshing (the menu-refreshing
	// signal) is true
	// while any type present in ProbeResources has not yet been acked here —
	// i.e. a background availability sweep is still confirming/replacing a
	// cache-seeded startup. Handle marks a type acked unconditionally on any
	// AvailabilityChecked arrival (even one the central gen-guard treats as
	// stale) because the sweep-in-flight signal tracks wall-clock probe
	// completion, not generation validity.
	menuSweepAcked map[string]bool

	// availSaveStop signals the cache writer to drain and exit; closed
	// exactly once by Close via availSaveCloseOnce.
	availSaveStop chan struct{}

	// availSaveCloseOnce guards closing availSaveStop so a Controller.Close
	// called more than once (or concurrently) never double-closes the channel.
	availSaveCloseOnce sync.Once

	// cacheWriteMu guards pendingCacheWrites — deliberately NOT c.mu. The
	// whole point of this lane is that persisting a result never touches the
	// lock a key press needs: a writer taking c.mu to pick up its next batch
	// makes every reader queue behind it (Go's RWMutex blocks new readers once
	// a writer is waiting), which is the latency this lane exists to avoid.
	// Never held while a write runs — a staged write calls back into the
	// Controller (SaveColumnsForType takes c.mu.RLock), so holding this lock
	// across one would invert the order the queue establishes.
	cacheWriteMu sync.Mutex

	// cacheWriteWake nudges the per-type cache writer that pendingCacheWrites
	// has something in it. Buffered to exactly 1 and sent to non-blockingly:
	// the queue is the slice, not this channel, so a nudge that arrives while
	// the writer is already draining costs nothing and loses nothing.
	cacheWriteWake chan struct{}

	// cacheWriteOnce starts the per-type cache writer goroutine on the first
	// write that needs it, so a Controller that never persists anything never
	// spawns it.
	cacheWriteOnce sync.Once

	// cacheWriteWG counts staged-but-unwritten cache writes: Add(1) when one
	// is queued, Done when it has run. WaitForCacheWrites (testing.go) is the
	// only reader — a test that reads the file a call it just made produces
	// needs a barrier, since the write no longer happens on its goroutine.
	cacheWriteWG sync.WaitGroup

	// availSaveWG is Add(1)-ed when the writer goroutine starts and Done on
	// its return; Close.Wait()s on it so the last queued write is guaranteed
	// to have run before Close returns.
	availSaveWG sync.WaitGroup
}

// cacheWrite is one staged disk save. availPair is set only on a counts-only
// availability save, which names the pair it will read the row store for:
// two of those queued back to back would write the same file twice from the
// same source, so the second coalesces into the first (queueAvailabilitySave).
// A rows-carrying save between them is a different write and stops the
// coalescing, which is what keeps the queue's order meaningful.
type cacheWrite struct {
	run       func()
	availPair session.Pair
}

// controllerErrorEntry is one session-error-log entry stored in Controller.
// Mirrors the tui.errorEntry private type but lives in the app package so
// the controller can build the error-log text body without importing tui.
type controllerErrorEntry struct {
	t       time.Time
	message string
}

// New constructs a Controller backed by the given runtime Core.
// The root screen is always ScreenMenu so Snapshot() returns BodyKindMenu
// immediately.
//
// Registers c's view-config-aware column resolver on core via
// SetSaveColumns (task #17 wave 1 stage 4) so Core.SaveTypeRows persists a
// user's per-session column overrides — not just the built-in defaults —
// from both the TUI and web renderer, which both construct their Controller
// through this single entry point.
func New(core *runtime.Core) *Controller {
	c := &Controller{
		core: core,
		stack: []Screen{
			{
				ID:    runtime.ScreenMenu,
				State: ScreenState{Menu: &MenuState{}},
			},
		},
		availSaveStop:  make(chan struct{}),
		cacheWriteWake: make(chan struct{}, 1),
	}
	core.SetSaveColumns(c.SaveColumnsForType)
	return c
}

// SaveColumnsForType is the runtime.Core.saveColumns implementation
// registered by New: a projection of resolveColumnsLocked's answer — the one
// the render path shows, per-session view-config override included — onto the
// config.ListColumn shape SaveTypeRows shares with the built-in-only fallback
// (resolveSaveColumns in core/runtime/probes.go). A projection, not a second
// resolution, so a save can never persist a column set the render path would
// not show. Humanize is dropped deliberately: the render lane applies it from
// its own resolved column, so carrying it on the persisted shape would give a
// replayed cell a second, staler source for the same decision.
//
// Exported so the save lane's answer is readable beside the render lane's; it
// is lock-free by contract (below), so a caller outside the lock is safe.
//
// Reads c.viewConfig/c.fallbackTypeDefs live (not a value captured at
// construction time) since SetViewConfig/RegisterFallbackTypeDef are called
// after New returns.
//
// Locking: Go's sync.RWMutex is not reentrant, and every production call to
// It reads c.viewConfig and c.fallbackTypeDefs, and it takes the read lock to
// do it: neither caller of Core.SaveTypeRows holds c.mu. The list-open lane
// queues its save with frozen inputs and the cache writer performs it on its
// own goroutine (stageCacheWrite/runCacheWriteLoop), and the executor's sweep
// lane never touches Controller at all. Without the lock, a save running there
// reads those maps while RegisterFallbackTypeDef or SetViewConfig writes them —
// a concurrent map read that took the whole process down once the writer moved
// off the caller's goroutine.
func (c *Controller) SaveColumnsForType(shortName string) []config.ListColumn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cols := c.resolveColumnsLocked(shortName)
	out := make([]config.ListColumn, len(cols))
	for i, cd := range cols {
		out[i] = config.ListColumn{Key: cd.Key, Title: cd.Title, Width: cd.Width, Path: cd.Path}
	}
	return out
}

// SetViewConfig stores the per-session view configuration so that
// resolveListColumns picks the correct column set for each resource type.
// Must be called before the first Snapshot() when a non-nil config is needed.
func (c *Controller) SetViewConfig(vc *config.ViewsConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.viewConfig = vc
}

// SetUIMode sets the renderer mode surfaced as Header.Mode ("" = TUI, "web").
// Called once after construction by hosts that render outside a terminal —
// core/web/construct.go's newSession calls SetUIMode("web"). The TUI
// never calls this, so its Header.Mode stays "".
func (c *Controller) SetUIMode(mode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.uiMode = mode
}

// UIMode returns the renderer mode previously set via SetUIMode.
func (c *Controller) UIMode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.uiMode
}

// captureDispatch snapshots the session generations and clients under c.mu
// (read lock — CaptureDispatch only reads session fields). The headless drain
// lanes (DrainSync and its variants in drainsync.go) call this once per task,
// immediately before that task's Core.ExecuteTaskAt, instead of once per
// batch: a bootstrap drain's TaskKindConnect task can swap session.Clients
// mid-queue, and a snapshot taken at batch start would hand every later task
// in that same batch the pre-connect (nil) clients. Per-task capture also
// closes the same dispatch-time-identity gap the TUI's synchronous Update
// closes for free — a web request handler can mutate session.Clients/
// generations through a locked Controller method concurrently with a drain
// loop's unlocked Core.ExecuteTaskAt call, so the snapshot for each task must
// be taken under lock rather than read live inside ExecuteTaskAt.
func (c *Controller) captureDispatch() runtime.DispatchSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.core.CaptureDispatch()
}

// stampDispatchSnapshotLocked fills Snap on every task in tasks that does
// not already carry one, from a SINGLE snapshot captured once for the whole
// batch — these tasks are all produced by one synchronous, already-locked
// Controller call, so (unlike DrainSync's per-task recapture, which exists
// specifically to cover a mid-batch TaskKindConnect swap) the session cannot
// mutate between them.
//
// Called at every locked public boundary that returns []runtime.TaskRequest
// (Apply, Handle, routeClientsReady, RestartAvailabilitySweep,
// EnsureCostsFetch, ForceRefreshCosts, ApplyEmitNavigate,
// OpenProfileSelector) so a task deferred by a web request handler and
// executed later by a background drain reads the session state AS OF THIS
// DISPATCH, not whatever the session has become by the time a background
// goroutine reaches it — the same race captureDispatch's own doc comment
// describes for the drain side, closed here at the producing end instead.
//
// A task that already carries a Snap (none do today, but a future producer
// might pre-stamp one — e.g. a follow-up re-queued from within a drain that
// already resolved its own snapshot) is left untouched.
//
// Callers must hold c.mu.
func (c *Controller) stampDispatchSnapshotLocked(tasks []runtime.TaskRequest) []runtime.TaskRequest {
	if len(tasks) == 0 {
		return tasks
	}
	var snap *runtime.DispatchSnapshot
	for i := range tasks {
		if tasks[i].Snap != nil {
			continue
		}
		if snap == nil {
			s := c.core.CaptureDispatch()
			snap = &s
		}
		tasks[i].Snap = snap
	}
	for i := range tasks {
		c.core.StampListFetchSeq(&tasks[i])
	}
	return tasks
}

// PendingDetailRefreshGet exposes Core.PendingDetailRefreshGet to callers
// outside package app — core/app/apptest's quiescence check needs to confirm
// that a refresh latch armed by beginDetailWorkloadLocked's "sticky refresh"
// path (session.PendingDetailRefresh, see that method's doc comment) was
// actually cleared by the run it drove, not just readable from inside the
// locked call that arms it.
func (c *Controller) PendingDetailRefreshGet(key string) (domain.Gen, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.core.PendingDetailRefreshGet(key)
}

// RegisterFallbackTypeDef stores a ResourceTypeDef so that buildListBody
// (columns) and GetListIssueCount (Color func) use the model's explicitly-
// supplied typeDef rather than the catalog's when they differ. This is critical
// for test typeDefs that share a ShortName with a catalog type but have a
// different column layout or nil Color (nil falls back to
// colorFallback(r.Fields["status"]) per ResolveColor contract in catalog/types.go).
func (c *Controller) RegisterFallbackTypeDef(td resource.ResourceTypeDef) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.registerFallbackTypeDefLocked(td)
}

// registerFallbackTypeDefLocked is the lock-free core of RegisterFallbackTypeDef.
// Callers must already hold c.mu (write) — e.g. applyLocked and everything it
// calls (handleActionChildView registers the web lane's child typeDef fallback
// this way; calling the locking RegisterFallbackTypeDef from there would
// self-deadlock on the already-held write lock, the same hazard documented on
// SaveColumnsForType above).
func (c *Controller) registerFallbackTypeDefLocked(td resource.ResourceTypeDef) {
	if c.fallbackTypeDefs == nil {
		c.fallbackTypeDefs = make(map[string]resource.ResourceTypeDef, 1)
	}
	c.fallbackTypeDefs[td.ShortName] = td
}

// Apply translates a semantic Action into the matching Core command, applies
// the returned UIIntents to the screen stack, enqueues returned TaskRequests,
// and returns the updated ViewState plus newly-enqueued TaskRequests.
//
// USER-INTENT lane: each Action.Kind maps to a specific Core.HandleX method.
// All navigate/session actions and row-dependent actions are fully wired.
func (c *Controller) Apply(a Action) (ViewState, []runtime.TaskRequest) {
	c.mu.Lock()
	vs, tasks := c.applyLocked(a)
	tasks = c.stampDispatchSnapshotLocked(tasks)
	c.mu.Unlock()
	// Whatever applyLocked staged is handed to the writer only now: a marshal
	// started while the lock was still held competes with the hold itself (C4).
	c.wakeCacheWriter()
	return vs, tasks
}

// selectedResourceForAction resolves the resource a row-dependent action targets:
// the top detail's resource when a detail is on top, the resource identified by
// the top text screen's ScreenContext when on a YAML/JSON screen, otherwise the
// selected row of the top list. Returns (resource, resourceType, ok). Lock-free
// — callers must already hold c.mu (it is only called from applyLocked).
func (c *Controller) selectedResourceForAction() (resource.Resource, string, bool) {
	if ds := c.topDetailState(); ds != nil {
		return ds.Resource, ds.ResourceType, true
	}
	// Text screens (YAML/JSON): resolve the resource from the cache using the
	// screen's ScreenContext so that resource-backed actions (CloudTrail, child
	// views, etc.) work identically to how they work on detail screens.
	if len(c.stack) > 0 {
		top := c.stack[len(c.stack)-1]
		if isTextScreen(top.ID) && top.Ctx.ResourceType != "" && top.Ctx.ResourceID != "" {
			if r, ok := c.findCachedResourceByID(top.Ctx.ResourceType, top.Ctx.ResourceID); ok {
				return r, top.Ctx.ResourceType, true
			}
		}
	}
	if r, ok := c.listSelected(); ok {
		typeName := ""
		if len(c.stack) > 0 {
			if top := c.stack[len(c.stack)-1]; top.ID == runtime.ScreenResourceList || top.ID == runtime.ScreenChildList {
				typeName = top.Ctx.ResourceType
			}
		}
		return r, typeName, true
	}
	return resource.Resource{}, "", false
}

// openSelectedListDetail opens the detail view for the currently-selected row
// of the top list screen (resource list or child list). Called by both
// ActionOpenDetail and the list branch of ActionSelect so the two entry points
// share identical logic. Callers must hold c.mu (write).
func (c *Controller) openSelectedListDetail() (ViewState, []runtime.TaskRequest) {
	r, ok := c.listSelected()
	if !ok {
		return c.snapshot(), nil
	}
	typeName := ""
	if top := c.stack[len(c.stack)-1]; top.ID == runtime.ScreenResourceList || top.ID == runtime.ScreenChildList {
		typeName = top.Ctx.ResourceType
	}
	res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
		Target:       runtime.NavigateTargetDetail,
		ResourceType: typeName,
		Resource:     &r,
	})
	// applyNavResult's NavigateKindPushDetail case owns the related-cache
	// replay (cache hit: rows merged directly into the stacked detail; cache
	// miss: a KindRelatedCheck task so DrainSync/the web renderer run the
	// checkers headlessly) — single source of truth shared with the TUI
	// adapter's handleNavigate.
	tasks = append(tasks, c.applyNavResult(res)...)
	return c.snapshot(), tasks
}

// applyLocked is the lock-free thin dispatcher of Apply. Callers must hold c.mu (write).
// Each case delegates to a handleActionX method in actions.go; 1-2 line cases stay inline.
func (c *Controller) applyLocked(a Action) (ViewState, []runtime.TaskRequest) {
	// A new user action supersedes any prior transient flash (e.g. a stale
	// API error). Clear it up front; a FlashIntent applied later in this action
	// or by its task results (via Handle) re-sets it, so an error still shows
	// until the next action.
	c.flash = Flash{}
	// Any user action dismisses the persistent error hint (mirrors the TUI's
	// m.showErrorHint = false at the top of handleKeyMsg). ActionOpenErrorLog
	// re-clears it explicitly after this point so the hint doesn't reappear.
	c.showErrorHint = false

	switch a.Kind {
	case ActionOpenHelp:
		return c.handleActionOpenHelp(a)
	case ActionBack:
		return c.handleActionBack(a)
	case ActionOpenIdentity:
		return c.handleActionOpenIdentity(a)
	case ActionOpenErrorLog:
		return c.handleActionOpenErrorLog(a)
	case ActionSelectProfile:
		return c.handleActionSelectProfile(a)
	case ActionSelectRegion:
		return c.handleActionSelectRegion(a)
	case ActionSelectTheme:
		return c.handleActionSelectTheme(a)
	case ActionCommand:
		return c.handleActionCommand(a)
	case ActionMoveUp:
		return c.handleActionMoveUp(a)
	case ActionMoveDown:
		return c.handleActionMoveDown(a)
	case ActionMoveTop:
		return c.handleActionMoveTop(a)
	case ActionMoveBottom:
		return c.handleActionMoveBottom(a)
	case ActionPageUp:
		return c.handleActionPageUp(a)
	case ActionPageDown:
		return c.handleActionPageDown(a)
	case ActionToggleAttention:
		return c.handleActionToggleAttention(a)
	case ActionSetFilter:
		return c.handleActionSetFilter(a)
	case ActionScrollLeft:
		return c.handleActionScrollLeft(a)
	case ActionScrollRight:
		return c.handleActionScrollRight(a)
	case ActionSort:
		return c.handleActionSort(a)
	case ActionSelect:
		return c.handleActionSelect(a)
	case ActionSelectIndex:
		return c.handleActionSelectIndex(a)
	case ActionToggleWrap:
		return c.handleActionToggleWrap(a)
	case ActionToggleFocus:
		return c.handleActionToggleFocus(a)
	case ActionSearch:
		return c.handleActionSearch(a)
	case ActionSearchNext:
		return c.handleActionSearchNext(a)
	case ActionSearchPrev:
		return c.handleActionSearchPrev(a)
	case ActionSearchClear:
		return c.handleActionSearchClear(a)
	case ActionToggleRelated:
		return c.handleActionToggleRelated(a)
	case ActionRelatedSelect:
		return c.handleActionRelatedSelect(a)
	case ActionFieldSelect:
		return c.handleActionFieldSelect(a)
	case ActionOpenDetail:
		return c.openSelectedListDetail()
	case ActionOpenYAML:
		return c.handleActionOpenYAML(a)
	case ActionOpenJSON:
		return c.handleActionOpenJSON(a)
	case ActionReveal:
		return c.handleActionReveal(a)
	case ActionChildView:
		return c.handleActionChildView(a)
	case ActionCloudTrail:
		return c.handleActionCloudTrail(a)
	case ActionLoadMore:
		return c.handleActionLoadMore(a)
	case ActionRefresh:
		return c.handleActionRefresh(a)
	case ActionCostZoomIn:
		return c.handleActionCostZoomIn(a)
	case ActionCostZoomOut:
		return c.handleActionCostZoomOut(a)
	case ActionCostMetric:
		return c.handleActionCostMetric(a)
	case ActionCostPivot:
		return c.handleActionCostPivot(a)
	case ActionCopy:
		// Copy is renderer-only (clipboard access is a renderer concern).
		// The controller has no clipboard; the web/TUI renderer handles this
		// directly without routing through Apply.
		return c.snapshot(), nil
	}

	// All remaining actions (sort, search, quit) are either renderer-only
	// or require state not yet lifted here.
	return c.snapshot(), nil
}
