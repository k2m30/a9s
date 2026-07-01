package app

import (
	"sync"
	"time"

	"github.com/k2m30/a9s/v3/internal/config"
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
	mu   sync.RWMutex
	core *runtime.Core
	stack []Screen

	// resourceCache stores the latest fetched resource pages per resource type,
	// keyed by canonical short name. Populated by applyResourcesLoaded.
	resourceCache map[string][]resource.Resource

	// enrichmentStore stores Wave-2 per-resource findings per resource type,
	// keyed by canonical short name. Populated by ApplyEnrichmentState.
	enrichmentStore map[string]map[string]domain.Finding

	// enrichmentTruncated stores the truncation flag per resource type from
	// ApplyEnrichmentState, parallel to enrichmentStore.
	enrichmentTruncated map[string]bool

	// reapplyCheckers stores per-type reapply checker + source resource for
	// approximate-pivot navigations. Populated by PatchListReapplyChecker.
	reapplyCheckers map[string]reapplyCheckerEntry

	// viewConfig is the per-session view configuration used by resolveListColumns
	// to pick the correct column set for each resource type. When nil, the built-in
	// defaults are used. Set by SetViewConfig after construction.
	viewConfig *config.ViewsConfig

	// fallbackTypeDefs stores ResourceTypeDef for resource types that are not
	// registered in the catalog (e.g. unit-test minimalTypeDef types). Both
	// buildListBody (for columns) and GetListIssueCount (for Color func) consult
	// this map when FindResourceType returns nil. Populated by RegisterFallbackTypeDef.
	fallbackTypeDefs map[string]resource.ResourceTypeDef

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
func New(core *runtime.Core) *Controller {
	return &Controller{
		core: core,
		stack: []Screen{
			{
				ID:    runtime.ScreenMenu,
				State: ScreenState{Menu: &MenuState{}},
			},
		},
	}
}

// SetViewConfig stores the per-session view configuration so that
// resolveListColumns picks the correct column set for each resource type.
// Must be called before the first Snapshot() when a non-nil config is needed.
func (c *Controller) SetViewConfig(vc *config.ViewsConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.viewConfig = vc
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
			for _, r := range c.resourceCache[top.Ctx.ResourceType] {
				if r.ID == top.Ctx.ResourceID {
					return r, top.Ctx.ResourceType, true
				}
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
	c.applyNavResult(res)
	// When HandleNavigate signals DispatchRelated, emit a KindRelatedCheck task
	// so DrainSync (and the web renderer) run the checkers headlessly. The TUI
	// adapter handles this separately via messages.RelatedCheckStarted; the
	// headless path uses the executor's runRelatedCheckers instead. Check the
	// related cache first — if results are already cached, replay them
	// synchronously into the stacked detail's RelatedRows (mirrors the TUI's
	// RelatedCacheGet/Replay path in runtime_adapter_navigate.go).
	if res.DispatchRelated && res.Resource != nil && len(resource.GetRelated(res.ResolvedType)) > 0 {
		ck := runtime.RelatedCacheKey(res.ResolvedType, res.Resource.ID)
		if cached, hit := c.core.RelatedCacheGet(ck); hit && len(cached) > 0 {
			// Cache hit: replay rows directly into the stacked detail.
			if ds := c.topDetailState(); ds != nil {
				for _, entry := range cached {
					errMsg := ""
					if entry.Result.Err != nil {
						errMsg = entry.Result.Err.Error()
					}
					mergeDetailRelatedRow(ds, entry.DefDisplayName, entry.Result.TargetType,
						entry.Result.Count, false, errMsg, entry.Result.Approximate, entry.Result.ResourceIDs, entry.Result.FetchFilter)
				}
			}
		} else {
			// Cache miss: dispatch a KindRelatedCheck task with the source resource
			// so the headless executor can invoke the checkers via runRelatedCheckers.
			src := *res.Resource
			tasks = append(tasks, runtime.TaskRequest{
				Key:     runtime.TaskKey{Kind: runtime.KindRelatedCheck, Scope: res.ResolvedType + "/" + src.ID},
				Cache:   runtime.CacheNone,
				Payload: runtime.RelatedCheckPayload{ResourceType: res.ResolvedType, Resource: src},
			})
		}
	}
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
