package app

import (
	"sync"
	"time"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/costs"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
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
// etc.) acquire a write lock on entry. Public read-only methods (Snapshot,
// GetMenu*, GetList*) acquire a read lock. Internal helpers called while a
// lock is already held must NOT lock — Go mutexes are not reentrant.
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

	// viewConfig is the per-session view configuration used by resolveListColumns
	// to pick the correct column set for each resource type. When nil, the built-in
	// defaults are used. Set by SetViewConfig after construction.
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

	// identityResult holds the resolved caller identity received via
	// messages.IdentityLoaded so snapshot can build IdentityBody without
	// importing internal/aws or touching the TUI view stack.
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

	// errorHistory records each error entry appended via AppendErrorHistoryIntent.
	// The '!' / open-error-log action renders these newest-first as a text screen.
	errorHistory []controllerErrorEntry

	// showErrorHint is true after an error flash clears (SetErrorHintIntent{Show:true})
	// and cleared on any subsequent action. Surfaced as Header.ErrorHintVisible in snapshot().
	showErrorHint bool

	// uiMode is the renderer mode surfaced as Header.Mode ("" = TUI, "web").
	// Set once via SetUIMode after construction (e.g. internal/web/construct.go
	// for web sessions). The TUI never calls SetUIMode, so it stays "". This is
	// independent of core.IsDemo() — a demo session is still a TUI or web
	// session and carries its own Header.Mode value ("demo") set elsewhere.
	uiMode string

	// menuSweepAcked tracks, per resource type, whether an AvailabilityChecked
	// result has landed for a type currently retained in
	// core.Session().ProbeResources. MenuBody.Refreshing (Contract C) is true
	// while any type present in ProbeResources has not yet been acked here —
	// i.e. a background availability sweep is still confirming/replacing a
	// cache-seeded startup. Handle marks a type acked unconditionally on any
	// AvailabilityChecked arrival (even one the central gen-guard treats as
	// stale) because the sweep-in-flight signal tracks wall-clock probe
	// completion, not generation validity.
	menuSweepAcked map[string]bool

	// availSaveCh feeds persistMenuAvailabilityCache's snapshots to the
	// single writer goroutine started by availSaveOnce. Buffered to exactly
	// 1 so a burst of calls coalesces into a latest-wins queue of one pending
	// write instead of stacking a write per call (see menu.go).
	availSaveCh chan availabilitySavePayload

	// availSaveOnce starts the persistMenuAvailabilityCache writer goroutine
	// on the first send, so a Controller that never touches the availability
	// badge (e.g. most unit tests) never spawns it.
	availSaveOnce sync.Once

	// availSaveStop signals runAvailabilitySaveLoop to drain and exit; closed
	// exactly once by Close via availSaveCloseOnce.
	availSaveStop chan struct{}

	// availSaveCloseOnce guards closing availSaveStop so a Controller.Close
	// called more than once (or concurrently) never double-closes the channel.
	availSaveCloseOnce sync.Once

	// availSaveWG is Add(1)-ed when the writer goroutine starts and Done on
	// its return; Close.Wait()s on it so the last queued write is guaranteed
	// to have run before Close returns.
	availSaveWG sync.WaitGroup
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
		availSaveCh:   make(chan availabilitySavePayload, 1),
		availSaveStop: make(chan struct{}),
	}
	core.SetSaveColumns(c.resolveSaveColumns)
	return c
}

// resolveSaveColumns is the runtime.Core.saveColumns implementation
// registered by New. It mirrors resolveListColumnsForBuild's cascade (the
// same one buildListBody/materializeListFieldsForType use at render time)
// so a save persists exactly the columns the render path would show,
// including any per-session view-config override — then narrows the result
// to the config.ListColumn shape SaveTypeRows shares with the built-in-only
// fallback (resolveSaveColumns in internal/runtime/probes.go).
//
// Reads c.viewConfig/c.fallbackTypeDefs live (not a value captured at
// construction time) since SetViewConfig/RegisterFallbackTypeDef are called
// after New returns.
//
// Locking: Go's sync.RWMutex is not reentrant, and every production call to
// Core.SaveTypeRows (and thus this resolver) from the list-open lane
// (maybeSaveResourceListCache) currently runs while c.mu is ALREADY held for
// writing (Handle/Apply take c.mu.Lock() once at the top and every helper
// down to maybeSaveResourceListCache is lock-free by convention) — a
// c.mu.RLock() here would self-deadlock on that path. The executor's
// background sweep lane (saveProbeResourcesToTypeFiles) never touches
// Controller at all, so it cannot race this method's reads against
// SetViewConfig/RegisterFallbackTypeDef, which only ever run from a
// Controller-owned, c.mu-serialized call (SetViewConfig, RegisterFallbackTypeDef
// themselves take c.mu.Lock()). The only remaining hazard — a concurrent
// SetViewConfig/RegisterFallbackTypeDef call racing THIS method while it runs
// under the caller's already-held write lock — cannot happen either: both of
// those setters also require c.mu, so they cannot run concurrently with any
// other Controller method. No additional lock is taken here.
func (c *Controller) resolveSaveColumns(shortName string) []config.ListColumn {
	var tdVal resource.ResourceTypeDef
	var td *resource.ResourceTypeDef
	if ftd, ok := c.fallbackTypeDefs[shortName]; ok {
		tdVal = ftd
		td = &tdVal
	} else if catalogTD := resource.FindResourceType(shortName); catalogTD != nil {
		td = catalogTD
	}
	cols := resolveListColumnsForBuild(c.viewConfig, shortName, td)
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
// internal/web/construct.go's newSession calls SetUIMode("web"). The TUI
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
// resolveSaveColumns above).
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
	defer c.mu.Unlock()
	return c.applyLocked(a)
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
