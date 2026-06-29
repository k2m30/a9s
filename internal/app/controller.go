package app

import (
	"maps"
	"strings"
	"sync"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
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
}

// New constructs a Controller backed by the given runtime Core.
// The root screen is always ScreenMenu so Snapshot() returns BodyKindMenu
// immediately; the QA agent updates the PR-A empty-stack tests accordingly.
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
// PR-B wires the six navigate/session actions that need no selected-row state.
// PR-C-blocked actions (row-dependent) are kept as documented no-ops.
func (c *Controller) Apply(a Action) (ViewState, []runtime.TaskRequest) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.applyLocked(a)
}

// selectedResourceForAction resolves the resource a row-dependent action targets:
// the top detail's resource when a detail is on top, otherwise the selected row
// of the top list. Returns (resource, resourceType, ok). Lock-free — callers
// must already hold c.mu (it is only called from applyLocked).
func (c *Controller) selectedResourceForAction() (resource.Resource, string, bool) {
	if ds := c.topDetailState(); ds != nil {
		return ds.Resource, ds.ResourceType, true
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

// applyLocked is the lock-free implementation of Apply. Callers must hold c.mu (write).
func (c *Controller) applyLocked(a Action) (ViewState, []runtime.TaskRequest) {
	switch a.Kind {

	// --- Navigate actions (PR-B) ---

	case ActionOpenHelp:
		res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetHelp})
		c.applyNavResult(res)
		return c.snapshot(), tasks

	case ActionBack:
		// Pop a single screen, mirroring the TUI's m.popView() — NOT a full
		// collapse (root-collapse is the "root" Command). Per-view Esc semantics
		// (clear filter/search before popping) arrive with PR-C view state.
		c.applyIntents([]runtime.UIIntent{runtime.PopScreen{}})
		return c.snapshot(), nil

	case ActionOpenIdentity:
		// The runtime has no NavigateTargetIdentity: the TUI opens the identity
		// screen via direct key-handling (not HandleNavigate). The headless
		// controller pushes ScreenIdentity directly so tests can assert the stack
		// without standing up a full TUI.
		c.applyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenIdentity}})
		c.core.SetIdentityFetching(true)
		// TODO PR-C: render IdentityBody.Loading from the session latch once body state is lifted here.
		fetchTask := runtime.TaskRequest{
			Key:     runtime.TaskKey{Kind: runtime.TaskKindFetchIdentity},
			Payload: runtime.FetchIdentityPayload{},
		}
		return c.snapshot(), []runtime.TaskRequest{fetchTask}

	// --- Session-selection actions (PR-B) ---

	case ActionSelectProfile:
		// ConnectGen is read pre-Rotate; HandleProfileSelected calls Rotate internally.
		// NewGen is passed as the bumped flash gen for the "Switching to …" tick.
		// The headless controller has no flash.gen to bump, so we pass the current
		// ConnectGen as a stable stand-in — the ClearFlash tick is adapter-owned.
		intents, tasks := c.core.HandleProfileSelected(runtime.ProfileSelectedEvent{
			Profile: a.Arg,
			NewGen:  c.core.ConnectGen(),
		})
		c.applyIntents(intents)
		return c.snapshot(), tasks

	case ActionSelectRegion:
		intents, tasks := c.core.HandleRegionSelected(runtime.RegionSelectedEvent{
			Region: a.Arg,
			NewGen: c.core.ConnectGen(),
		})
		c.applyIntents(intents)
		return c.snapshot(), tasks

	case ActionSelectTheme:
		intents, tasks := c.core.HandleThemeSelected(runtime.ThemeSelectedEvent{
			Theme: a.Arg,
		})
		c.applyIntents(intents)
		return c.snapshot(), tasks

	// --- Command lane (PR-B) ---

	case ActionCommand:
		// Arg carries a colon-command token (mirrors executeCommand in app_input.go).
		// Only arg-driven tokens are dispatched here; tokens that need selected-row
		// or per-screen state are noted as PR-C TODOs below.
		switch a.Arg {
		case "root", "main":
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetMainMenu})
			c.applyNavResult(res)
			return c.snapshot(), tasks

		case "profile", "ctx":
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetProfile})
			c.applyNavResult(res)
			return c.snapshot(), tasks

		case "region":
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetRegion})
			c.applyNavResult(res)
			return c.snapshot(), tasks

		case "theme":
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetTheme})
			c.applyNavResult(res)
			return c.snapshot(), tasks

		case "help":
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetHelp})
			c.applyNavResult(res)
			return c.snapshot(), tasks

		default:
			// Resource short-name or alias (e.g. "ec2", "s3", "dbi").
			if rt := resource.FindResourceType(a.Arg); rt != nil {
				res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
					Target:       runtime.NavigateTargetResourceList,
					ResourceType: a.Arg,
				})
				c.applyNavResult(res)
				return c.snapshot(), tasks
			}
			// TODO PR-C: "q"/"quit" needs tea.Quit from the renderer, not the controller.
			// Unknown tokens are silently dropped at this layer; the renderer flashes.
		}
		return c.snapshot(), nil

	// --- Shared navigation actions (PR-C): list screen takes priority, then menu ---

	case ActionMoveUp:
		if vs, tasks, handled := c.applyDetailActions(a); handled {
			return vs, tasks
		}
		if ts := c.topTextState(); ts != nil {
			if ts.ScrollY > 0 {
				ts.ScrollY--
			}
		} else if ls := c.topListState(); ls != nil {
			visible := c.listVisibleCount(ls)
			if ls.SelectedRow > 0 {
				ls.SelectedRow--
			}
			_ = visible
		} else if ms := c.topMenuState(); ms != nil {
			all := resource.AllResourceTypes()
			visible := menuVisibleItems(ms, all)
			if ms.Cursor > 0 {
				ms.Cursor--
			}
			menuSkipUnavailable(ms, visible, -1)
		} else if ss := c.topSelectorState(); ss != nil {
			visible := selectorVisibleItems(ss)
			if ss.Cursor > 0 {
				ss.Cursor--
			}
			_ = visible
		}
		return c.snapshot(), nil

	case ActionMoveDown:
		if vs, tasks, handled := c.applyDetailActions(a); handled {
			return vs, tasks
		}
		if ts := c.topTextState(); ts != nil {
			ts.ScrollY++
		} else if ls := c.topListState(); ls != nil {
			visible := c.listVisibleCount(ls)
			if ls.SelectedRow < visible-1 {
				ls.SelectedRow++
			}
		} else if ms := c.topMenuState(); ms != nil {
			all := resource.AllResourceTypes()
			visible := menuVisibleItems(ms, all)
			if ms.Cursor < len(visible)-1 {
				ms.Cursor++
			}
			menuSkipUnavailable(ms, visible, +1)
		} else if ss := c.topSelectorState(); ss != nil {
			visible := selectorVisibleItems(ss)
			if ss.Cursor < len(visible)-1 {
				ss.Cursor++
			}
		}
		return c.snapshot(), nil

	case ActionMoveTop:
		if vs, tasks, handled := c.applyDetailActions(a); handled {
			return vs, tasks
		}
		if ts := c.topTextState(); ts != nil {
			ts.ScrollY = 0
		} else if ls := c.topListState(); ls != nil {
			ls.SelectedRow = 0
		} else if ms := c.topMenuState(); ms != nil {
			ms.Cursor = 0
			all := resource.AllResourceTypes()
			visible := menuVisibleItems(ms, all)
			menuSkipUnavailable(ms, visible, +1)
		} else if ss := c.topSelectorState(); ss != nil {
			ss.Cursor = 0
		}
		return c.snapshot(), nil

	case ActionMoveBottom:
		if vs, tasks, handled := c.applyDetailActions(a); handled {
			return vs, tasks
		}
		if ts := c.topTextState(); ts != nil {
			if n := len(ts.Lines); n > 0 {
				ts.ScrollY = n - 1
			}
		} else if ls := c.topListState(); ls != nil {
			visible := c.listVisibleCount(ls)
			if visible > 0 {
				ls.SelectedRow = visible - 1
			}
		} else if ms := c.topMenuState(); ms != nil {
			all := resource.AllResourceTypes()
			visible := menuVisibleItems(ms, all)
			if len(visible) > 0 {
				ms.Cursor = len(visible) - 1
			}
			menuSkipUnavailable(ms, visible, -1)
		} else if ss := c.topSelectorState(); ss != nil {
			visible := selectorVisibleItems(ss)
			if len(visible) > 0 {
				ss.Cursor = len(visible) - 1
			}
		}
		return c.snapshot(), nil

	case ActionPageUp:
		if vs, tasks, handled := c.applyDetailActions(a); handled {
			return vs, tasks
		}
		if ts := c.topTextState(); ts != nil {
			ts.ScrollY -= textPageSizeFor(a)
			if ts.ScrollY < 0 {
				ts.ScrollY = 0
			}
		} else if ls := c.topListState(); ls != nil {
			pageSize := listPageSizeFor(a)
			ls.SelectedRow -= pageSize
			if ls.SelectedRow < 0 {
				ls.SelectedRow = 0
			}
		} else if ms := c.topMenuState(); ms != nil {
			all := resource.AllResourceTypes()
			visible := menuVisibleItems(ms, all)
			ms.Cursor -= menuPageSizeFor(a)
			if ms.Cursor < 0 {
				ms.Cursor = 0
			}
			menuSkipUnavailable(ms, visible, -1)
		} else if ss := c.topSelectorState(); ss != nil {
			pageSize := selectorPageSizeFor(a)
			ss.Cursor -= pageSize
			if ss.Cursor < 0 {
				ss.Cursor = 0
			}
		}
		return c.snapshot(), nil

	case ActionPageDown:
		if vs, tasks, handled := c.applyDetailActions(a); handled {
			return vs, tasks
		}
		if ts := c.topTextState(); ts != nil {
			ts.ScrollY += textPageSizeFor(a)
		} else if ls := c.topListState(); ls != nil {
			pageSize := listPageSizeFor(a)
			visible := c.listVisibleCount(ls)
			ls.SelectedRow += pageSize
			if n := visible; ls.SelectedRow >= n {
				ls.SelectedRow = max(n-1, 0)
			}
		} else if ms := c.topMenuState(); ms != nil {
			all := resource.AllResourceTypes()
			visible := menuVisibleItems(ms, all)
			ms.Cursor += menuPageSizeFor(a)
			if n := len(visible); ms.Cursor >= n {
				ms.Cursor = max(n-1, 0)
			}
			menuSkipUnavailable(ms, visible, +1)
		} else if ss := c.topSelectorState(); ss != nil {
			pageSize := selectorPageSizeFor(a)
			visible := selectorVisibleItems(ss)
			ss.Cursor += pageSize
			if n := len(visible); ss.Cursor >= n {
				ss.Cursor = max(n-1, 0)
			}
		}
		return c.snapshot(), nil

	case ActionToggleAttention:
		if ls := c.topListState(); ls != nil {
			ls.AttentionOnly = !ls.AttentionOnly
			ls.SelectedRow = 0
		} else if ms := c.topMenuState(); ms != nil {
			ms.AttentionOnly = !ms.AttentionOnly
			ms.Cursor = 0
		}
		return c.snapshot(), nil

	case ActionSetFilter:
		if vs, tasks, handled := c.applyDetailActions(a); handled {
			return vs, tasks
		}
		if ls := c.topListState(); ls != nil {
			ls.Filter = a.Arg
			ls.SelectedRow = 0
			ls.ScrollY = 0
		} else if ms := c.topMenuState(); ms != nil {
			ms.Filter = a.Arg
			ms.Cursor = 0
			ms.ScrollOffset = 0
		} else if ss := c.topSelectorState(); ss != nil {
			ss.Filter = a.Arg
			ss.Cursor = 0
		}
		return c.snapshot(), nil

	// --- List-only actions (PR-C) ---

	case ActionScrollLeft:
		if ls := c.topListState(); ls != nil {
			if ls.ScrollX > 0 {
				ls.ScrollX--
			}
		}
		return c.snapshot(), nil

	case ActionScrollRight:
		if ls := c.topListState(); ls != nil {
			ls.ScrollX++
		}
		return c.snapshot(), nil

	case ActionSort:
		if ls := c.topListState(); ls != nil && a.Arg != "" {
			if ls.SortCol == a.Arg {
				if ls.SortDir == "asc" {
					ls.SortDir = "desc"
				} else {
					ls.SortDir = "asc"
				}
			} else {
				ls.SortCol = a.Arg
				ls.SortDir = "asc"
			}
			ls.SelectedRow = 0
		}
		return c.snapshot(), nil

	case ActionSelect:
		// Related-panel Enter: when the top screen is a detail view and
		// RelatedFocus is active, navigate to the focused related row.
		if ds := c.topDetailState(); ds != nil && ds.RelatedFocus {
			// Find the row at RelatedCursor using the same filter logic as
			// detailRelatedVisibleCount.
			query := strings.TrimSpace(strings.ToLower(ds.RelatedFilter))
			var focusedRow *DetailRelatedRow
			idx := 0
			for i := range ds.RelatedRows {
				row := &ds.RelatedRows[i]
				if isSelfPivotZeroDetailRow(*row, ds.ResourceType) {
					continue
				}
				if query != "" && !strings.Contains(strings.ToLower(row.DisplayName), query) {
					continue
				}
				if idx == ds.RelatedCursor {
					focusedRow = row
					break
				}
				idx++
			}
			if focusedRow != nil && !focusedRow.Loading {
				// Derive the single target ID when there is exactly one related
				// resource (used by NavigationKindDetail cache-hit path).
				targetID := ""
				if len(focusedRow.ResourceIDs) == 1 {
					targetID = focusedRow.ResourceIDs[0]
				}
				// Look up the checker from the registered RelatedDef; DetailRelatedRow
				// is a serialisable value type (no funcs/checker field).
				var checker resource.RelatedChecker
				for _, def := range resource.GetRelated(ds.ResourceType) {
					if def.TargetType == focusedRow.TargetType {
						checker = def.Checker
						break
					}
				}
				ev := runtime.RelatedNavigateEvent{
					TargetType:     focusedRow.TargetType,
					SourceResource: ds.Resource,
					SourceType:     ds.ResourceType,
					TargetID:       targetID,
					RelatedIDs:     focusedRow.ResourceIDs,
					FetchFilter:    focusedRow.FetchFilter,
					Checker:        checker,
				}
				navRes, tasks := c.core.HandleRelatedNavigate(ev)
				extraTasks := c.applyRelatedNavResult(navRes)
				return c.snapshot(), append(tasks, extraTasks...)
			}
			return c.snapshot(), nil
		}

		if ms := c.topMenuState(); ms != nil {
			all := resource.AllResourceTypes()
			visible := menuVisibleItems(ms, all)
			if len(visible) > 0 && ms.Cursor < len(visible) {
				selected := visible[ms.Cursor]
				// Block navigation to confirmed-empty types (count known, zero, not
				// truncated). Availability may be stored under an alias key, so resolve
				// it via menuActiveKey — matching MenuSelected (the TUI Enter path).
				if ms.Availability != nil {
					activeKey := menuActiveKey(ms, selected)
					isTruncated := ms.Truncated != nil && ms.Truncated[activeKey]
					if count, known := ms.Availability[activeKey]; known && count == 0 && !isTruncated {
						return c.snapshot(), nil
					}
				}
				res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
					Target:       runtime.NavigateTargetResourceList,
					ResourceType: selected.ShortName,
				})
				c.applyNavResult(res)
				return c.snapshot(), tasks
			}
		}
		return c.snapshot(), nil

	// --- Text-screen and detail-screen actions ---

	case ActionToggleWrap:
		if vs, tasks, handled := c.applyDetailActions(a); handled {
			return vs, tasks
		}
		if ts := c.topTextState(); ts != nil {
			ts.Wrap = !ts.Wrap
		}
		return c.snapshot(), nil

	case ActionToggleFocus:
		// Detail-only: Tab toggles focus between the field and related columns.
		if vs, tasks, handled := c.applyDetailActions(a); handled {
			return vs, tasks
		}
		return c.snapshot(), nil

	case ActionSearch:
		if vs, tasks, handled := c.applyDetailActions(a); handled {
			return vs, tasks
		}
		if ts := c.topTextState(); ts != nil {
			ts.Search = a.Arg
			ts.SearchCursor = 0
		}
		return c.snapshot(), nil

	case ActionSearchNext:
		if vs, tasks, handled := c.applyDetailActions(a); handled {
			return vs, tasks
		}
		if ts := c.topTextState(); ts != nil && ts.Search != "" {
			matches := buildTextSearchMatches(ts.Lines, ts.Search)
			if len(matches) > 0 {
				ts.SearchCursor = (ts.SearchCursor + 1) % len(matches)
				if ts.SearchCursor < len(matches) {
					ts.ScrollY = matches[ts.SearchCursor].Line
				}
			}
		}
		return c.snapshot(), nil

	case ActionSearchPrev:
		if vs, tasks, handled := c.applyDetailActions(a); handled {
			return vs, tasks
		}
		if ts := c.topTextState(); ts != nil && ts.Search != "" {
			matches := buildTextSearchMatches(ts.Lines, ts.Search)
			if len(matches) > 0 {
				ts.SearchCursor = (ts.SearchCursor - 1 + len(matches)) % len(matches)
				if ts.SearchCursor < len(matches) {
					ts.ScrollY = matches[ts.SearchCursor].Line
				}
			}
		}
		return c.snapshot(), nil

	case ActionSearchClear:
		if vs, tasks, handled := c.applyDetailActions(a); handled {
			return vs, tasks
		}
		if ts := c.topTextState(); ts != nil {
			ts.Search = ""
			ts.SearchCursor = 0
		}
		return c.snapshot(), nil

	case ActionToggleRelated:
		if vs, tasks, handled := c.applyDetailActions(a); handled {
			return vs, tasks
		}
		return c.snapshot(), nil

	// --- Row-dependent actions: require selected row + per-screen state ---

	case ActionOpenDetail:
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
							entry.Result.Count, false, errMsg, entry.Result.Approximate, entry.Result.FetchFilter)
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

	case ActionOpenYAML:
		r, typeName, ok := c.selectedResourceForAction()
		if !ok {
			return c.snapshot(), nil
		}
		res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
			Target:       runtime.NavigateTargetYAML,
			ResourceType: typeName,
			Resource:     &r,
		})
		c.applyNavResult(res)
		return c.snapshot(), tasks

	case ActionOpenJSON:
		r, typeName, ok := c.selectedResourceForAction()
		if !ok {
			return c.snapshot(), nil
		}
		res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
			Target:       runtime.NavigateTargetJSON,
			ResourceType: typeName,
			Resource:     &r,
		})
		c.applyNavResult(res)
		return c.snapshot(), tasks

	case ActionReveal:
		// Resolve the resource from the active list or detail screen.
		var revealRes *resource.Resource
		var revealType string
		if ds := c.topDetailState(); ds != nil {
			r := ds.Resource
			revealRes = &r
			revealType = ds.ResourceType
		} else if ls := c.topListState(); ls != nil {
			r, ok := c.listSelected()
			if ok {
				revealRes = &r
				if top := c.stack[len(c.stack)-1]; len(c.stack) > 0 {
					revealType = top.Ctx.ResourceType
				}
			}
		}
		if revealRes == nil {
			return c.snapshot(), nil
		}
		res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
			Target:       runtime.NavigateTargetReveal,
			ResourceType: revealType,
			Resource:     revealRes,
		})
		// KindFetchReveal: no stack push yet — the push happens when
		// Handle receives messages.ValueRevealed and calls HandleValueRevealed.
		_ = res
		return c.snapshot(), tasks

	case ActionChildView:
		// Arg carries the trigger key (e, L, R, r, s, Enter, t …).
		triggerKey := a.Arg
		if triggerKey == "" {
			return c.snapshot(), nil
		}
		r, typeName, ok := c.selectedResourceForAction()
		if !ok {
			return c.snapshot(), nil
		}
		td := resource.FindResourceType(typeName)
		if td == nil {
			return c.snapshot(), nil
		}
		// Walk the type's children to find the one registered under this key.
		var matchedChild *resource.ChildViewDef
		for i := range td.Children {
			ch := &td.Children[i]
			if ch.Key != triggerKey {
				continue
			}
			if ch.DrillCondition != nil && !ch.DrillCondition(r) {
				continue
			}
			matchedChild = ch
			break
		}
		if matchedChild == nil {
			return c.snapshot(), nil
		}
		// Build the parent context from ContextKeys.
		ctx := make(map[string]string, len(matchedChild.ContextKeys))
		for param, source := range matchedChild.ContextKeys {
			switch source {
			case "ID":
				ctx[param] = r.ID
			case "Name":
				ctx[param] = r.Name
			default:
				ctx[param] = r.Fields[source]
			}
		}
		displayName := ctx[matchedChild.DisplayNameKey]
		ev := runtime.EnterChildViewEvent{
			ChildType:     matchedChild.ChildType,
			ParentContext: ctx,
			DisplayName:   displayName,
		}
		intents, tasks := c.core.HandleEnterChildView(ev)
		c.applyIntents(intents)
		// Seed the child list screen's context and state after PushScreen.
		if len(c.stack) > 0 {
			top := &c.stack[len(c.stack)-1]
			if top.ID == runtime.ScreenChildList {
				top.Ctx.ResourceType = matchedChild.ChildType
				if top.State.List == nil {
					top.State.List = &ListState{
						Loading:       true,
						ParentContext: ctx,
					}
				}
			}
		}
		return c.snapshot(), tasks

	case ActionLoadMore:
		ls := c.topListState()
		if ls == nil || !ls.HasPagination || ls.LoadingMore {
			return c.snapshot(), nil
		}
		ls.LoadingMore = true
		typeName := ""
		if top := c.stack[len(c.stack)-1]; len(c.stack) > 0 {
			typeName = top.Ctx.ResourceType
		}
		tasks := []runtime.TaskRequest{{
			Key: runtime.TaskKey{Kind: runtime.KindFetchMore, Scope: typeName},
			Payload: runtime.FetchMorePayload{
				ContinuationToken: ls.PaginationCursor,
			},
		}}
		return c.snapshot(), tasks

	case ActionRefresh:
		// Detail view: re-dispatch enrich + related.
		if ds := c.topDetailState(); ds != nil {
			rt := ds.ResourceType
			srcRes := ds.Resource
			var tasks []runtime.TaskRequest
			if resource.HasDetailEnricher(rt) {
				tasks = append(tasks, runtime.TaskRequest{
					Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: rt},
				})
			}
			// Emit the enrich detail task so the executor re-runs enrichment.
			// The related-check task is emitted separately via Handle(RelatedCheckStarted).
			_ = srcRes
			return c.snapshot(), tasks
		}
		// List view: delete cache and re-fetch.
		if ls := c.topListState(); ls != nil {
			typeName := ""
			if top := c.stack[len(c.stack)-1]; len(c.stack) > 0 {
				typeName = top.Ctx.ResourceType
			}
			if typeName == "" {
				return c.snapshot(), nil
			}
			c.core.DeleteResourceCache(typeName)
			ls.Loading = true
			ls.Rows = nil
			tasks := []runtime.TaskRequest{{
				Key:   runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: typeName},
				Cache: runtime.CacheNone,
			}}
			return c.snapshot(), tasks
		}
		return c.snapshot(), nil

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

// Handle feeds an event through runtime.Core.HandleEvent, applies the returned
// UIIntents to the screen stack, enqueues returned TaskRequests, and returns
// the updated ViewState plus those TaskRequests.
//
// TASK-RESULT lane: completed background-task results arrive here. The caller
// is responsible for passing only values that implement runtime.Event
// (i.e. messages.Event) — unrecognised concrete types fall through to
// Core.HandleEvent's default nil, nil path.
//
// ResourcesLoaded events are also routed to applyResourcesLoaded so that
// DrainSync and the web renderer populate list rows without going through the
// TUI view stack. The target list screen is found by ResourceType in the
// controller stack, so a late async result for type X lands on X's screen even
// when it is not currently on top.
func (c *Controller) Handle(ev runtime.Event) (ViewState, []runtime.TaskRequest) {
	c.mu.Lock()
	defer c.mu.Unlock()

	intents, tasks := c.core.HandleEvent(ev)
	c.applyIntents(intents)

	// HandleEvent's central GenStamped guard drops stale events from the intent
	// path, but the row mutation below runs unconditionally. A host that passes
	// task results straight to Handle (headless/web) would otherwise let a late
	// fetch from a previous profile/region overwrite the current list rows, so
	// re-check staleness with the same predicate before mutating the controller.
	if msg, ok := ev.(messages.ResourcesLoaded); ok && !messages.IsStale(msg, c.core) {
		c.handleResourcesLoadedEvent(msg)
	}

	// messages.ValueRevealed is explicitly excluded from HandleEvent
	// (orchestrator.go routes it nowhere) — the controller must handle it
	// directly here. HandleValueRevealed emits PushScreen{ScreenReveal} on
	// success or a FlashIntent on error.
	//
	// Guard: only process when the stack contains a resource-bearing screen
	// (list or detail) — a reveal can only be initiated from those screens.
	// A menu-only stack receiving ValueRevealed is a spurious event (e.g.
	// late delivery after a profile switch) and is silently dropped.
	if msg, ok := ev.(messages.ValueRevealed); ok && !messages.IsStale(msg, c.core) && c.hasResourceScreen() {
		revealed := runtime.ValueRevealedEvent{
			ResourceID: msg.ResourceID,
			Value:      msg.Value,
			Err:        msg.Err,
		}
		revealIntents, revealTasks := c.core.HandleValueRevealed(revealed)
		c.applyIntents(revealIntents)
		tasks = append(tasks, revealTasks...)
	}

	// messages.RelatedCheckBatch is the headless executor's counterpart to the
	// per-def RelatedCheckResult messages the TUI fan-out emits. Route each
	// per-def result through the same Core handler and ApplyDetailRelatedResult
	// path the TUI uses, so DrainSync populates the detail's RelatedRows.
	if batch, ok := ev.(messages.RelatedCheckBatch); ok && !messages.IsStale(batch, c.core) {
		c.handleRelatedCheckBatch(batch)
	}

	return c.snapshot(), tasks
}

// handleResourcesLoadedEvent routes a ResourcesLoaded event to the matching
// list screen in the controller stack. It finds the screen by resolving the
// event's ResourceType (including aliases) against each screen's context,
// so a late result for type X lands on X's screen regardless of which screen
// is currently on top. Staleness is the caller's responsibility — Handle drops
// stale ResourcesLoaded via messages.IsStale before invoking this.
func (c *Controller) handleResourcesLoadedEvent(msg messages.ResourcesLoaded) {
	if msg.ResourceType == "" {
		return
	}
	// Resolve canonical short name (handles aliases like "rds" → "dbi").
	canon := msg.ResourceType
	if td := resource.FindResourceType(msg.ResourceType); td != nil {
		canon = td.ShortName
	}
	// A fetch result belongs to a single list — the active (topmost) one of its
	// type. Apply it to the FIRST matching list from the top and stop; fanning it
	// out to every same-type list would overwrite a stacked filtered/child list's
	// rows onto the list beneath it (and vice-versa).
	for i := len(c.stack) - 1; i >= 0; i-- {
		s := &c.stack[i]
		if s.ID != runtime.ScreenResourceList && s.ID != runtime.ScreenChildList {
			continue
		}
		screenType := s.Ctx.ResourceType
		if td := resource.FindResourceType(screenType); td != nil {
			screenType = td.ShortName
		}
		if screenType != canon {
			continue
		}
		c.applyResourcesLoaded(s.State.List, canon, msg.Resources, msg.Pagination, msg.Append)
		return
	}
}

// handleRelatedCheckBatch routes a RelatedCheckBatch (produced by the headless
// executor's runRelatedCheckers) into the stacked detail screen that matches
// the batch's (ResourceType, SourceResourceID). For each per-def result it
// calls HandleRelatedCheckResult on Core (to update the session RelatedCache
// and resource/lazy caches) and then ApplyDetailRelatedResult to merge the
// row into the matching detail's RelatedRows — mirroring the TUI's
// handleRelatedCheckResult path.
//
// The matching detail may not be the topmost screen (e.g. a YAML overlay is
// on top while the detail is stacked underneath). The search walks the stack
// from top to bottom and applies to the FIRST matching ScreenDetail.
//
// Callers must hold c.mu (write).
func (c *Controller) handleRelatedCheckBatch(batch messages.RelatedCheckBatch) {
	// Find the matching detail screen in the stack.
	var targetDetail *DetailState
	for i := len(c.stack) - 1; i >= 0; i-- {
		s := &c.stack[i]
		if s.ID != runtime.ScreenDetail {
			continue
		}
		ds := s.State.Detail
		if ds == nil {
			continue
		}
		if ds.ResourceType != batch.ResourceType || ds.Resource.ID != batch.SourceResourceID {
			continue
		}
		targetDetail = ds
		break
	}

	for _, result := range batch.Results {
		// Route through Core to update session caches (RelatedCache, ResourceCache,
		// LazyResourceCache) — mirrors handleRelatedCheckResult in the TUI adapter.
		intents, _ := c.core.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
			ResourceType:     result.ResourceType,
			SourceResourceID: result.SourceResourceID,
			DefDisplayName:   result.DefDisplayName,
			Result:           result.Result,
		})
		c.applyIntents(intents)

		// Merge the row into the matching detail's RelatedRows directly, since
		// ApplyDetailRelatedResult operates on the TOP detail screen but the
		// target may be stacked. Use the found targetDetail pointer directly.
		if targetDetail == nil {
			continue
		}
		errMsg := ""
		if result.Result.Err != nil {
			errMsg = result.Result.Err.Error()
		}
		mergeDetailRelatedRow(targetDetail, result.DefDisplayName, result.Result.TargetType,
			result.Result.Count, false, errMsg, result.Result.Approximate, result.Result.FetchFilter)
	}
}

// mergeDetailRelatedRow updates or appends one RelatedRow in ds, matching by
// DisplayName. Mirrors ApplyDetailRelatedResult but operates on a DetailState
// pointer directly rather than the top-of-stack screen.
func mergeDetailRelatedRow(ds *DetailState, displayName, targetType string, count int, loading bool, errMsg string, approximate bool, fetchFilter map[string]string) {
	for i := range ds.RelatedRows {
		if ds.RelatedRows[i].DisplayName == displayName {
			ds.RelatedRows[i].Count = count
			ds.RelatedRows[i].Loading = loading
			ds.RelatedRows[i].Err = errMsg
			ds.RelatedRows[i].Approximate = approximate
			ds.RelatedRows[i].FetchFilter = fetchFilter
			return
		}
	}
	ds.RelatedRows = append(ds.RelatedRows, DetailRelatedRow{
		TargetType:  targetType,
		DisplayName: displayName,
		Count:       count,
		Loading:     loading,
		Err:         errMsg,
		Approximate: approximate,
		FetchFilter: fetchFilter,
	})
}

// ApplyIntents applies a slice of UIIntents to the controller's screen stack.
// Stack-navigation intents (PushScreen / PopScreen / ReplaceScreen) are fully
// implemented. All other intent variants are no-ops with a PR-C marker so
// tests can drive the stack directly without standing up a full renderer.
//
// ApplyIntents never panics on a PopScreen against an empty stack.
// It returns the post-apply ViewState snapshot.
func (c *Controller) ApplyIntents(intents []runtime.UIIntent) ViewState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.applyIntents(intents)
}

// applyIntents is the lock-free implementation of ApplyIntents.
// Callers must hold c.mu (write).
func (c *Controller) applyIntents(intents []runtime.UIIntent) ViewState {
	for _, intent := range intents {
		switch v := intent.(type) {
		case runtime.PushScreen:
			c.stack = append(c.stack, Screen{
				ID:  v.ID,
				Ctx: v.Context,
			})

		case runtime.PopScreen:
			// Never pop the root screen (the menu) — mirrors the TUI's popView
			// (app_stack.go), which refuses to pop the last screen. Popping to an
			// empty stack would blank the app to BodyKindUnknown.
			if len(c.stack) > 1 {
				c.stack = c.stack[:len(c.stack)-1]
			}

		case runtime.ReplaceScreen:
			if len(c.stack) == 0 {
				c.stack = append(c.stack, Screen{ID: v.ID, Ctx: v.Context})
			} else {
				c.stack[len(c.stack)-1] = Screen{ID: v.ID, Ctx: v.Context}
			}

		case runtime.PopSelectorIntent:
			// Pop the top screen when it is a selector (profile/region/theme).
			// Emitted by HandleProfileSelected / HandleRegionSelected /
			// HandleThemeSelected so the selector dismisses after confirm.
			if len(c.stack) > 0 {
				top := c.stack[len(c.stack)-1]
				if top.ID == runtime.ScreenProfileSelector ||
					top.ID == runtime.ScreenRegion ||
					top.ID == runtime.ScreenTheme {
					c.stack = c.stack[:len(c.stack)-1]
				}
			}

		case runtime.PatchMenuAvailability:
			if ms := c.rootMenuState(); ms != nil {
				if ms.Availability == nil {
					ms.Availability = make(map[string]int)
				}
				if ms.Truncated == nil {
					ms.Truncated = make(map[string]bool)
				}
				// Store under the key as emitted by the runtime (may be an alias
				// such as "rds" for ShortName "dbi"). buildMenuBody resolves the
				// active key per item using menuActiveKey().
				ms.Availability[v.ResourceType] = v.Count
				ms.Truncated[v.ResourceType] = v.Truncated
			}

		case runtime.PatchMenu:
			if ms := c.rootMenuState(); ms != nil {
				if ms.IssueCounts == nil {
					ms.IssueCounts = make(map[string]int)
				}
				if ms.IssueKnown == nil {
					ms.IssueKnown = make(map[string]bool)
				}
				if ms.IssueTruncated == nil {
					ms.IssueTruncated = make(map[string]bool)
				}
				ms.IssueCounts[v.ResourceType] = v.Issues
				ms.IssueKnown[v.ResourceType] = true
				ms.IssueTruncated[v.ResourceType] = v.Truncated
			}

		case runtime.PatchMenuIssueBatch:
			if ms := c.rootMenuState(); ms != nil && v.Known != nil {
				if ms.IssueCounts == nil {
					ms.IssueCounts = make(map[string]int)
				}
				if ms.IssueKnown == nil {
					ms.IssueKnown = make(map[string]bool)
				}
				if ms.IssueTruncated == nil {
					ms.IssueTruncated = make(map[string]bool)
				}
				for name, k := range v.Known {
					if k {
						ms.IssueCounts[name] = v.Counts[name]
						ms.IssueKnown[name] = true
						ms.IssueTruncated[name] = v.Truncated[name]
					}
				}
			}

		case runtime.PatchMenuCheckProgress:
			if ms := c.rootMenuState(); ms != nil {
				ms.AvailChecked = v.Checked
				ms.AvailTotal = v.Total
			}

		case runtime.PatchMenuEnrichProgress:
			if ms := c.rootMenuState(); ms != nil {
				ms.EnrichChecked = v.Checked
				ms.EnrichTotal = v.Total
			}

		case runtime.MenuClearAvailabilityIntent:
			if ms := c.rootMenuState(); ms != nil {
				ms.Availability = nil
				ms.Truncated = nil
				ms.AvailChecked = 0
				ms.AvailTotal = 0
				ms.IssueCounts = nil
				ms.IssueKnown = nil
				ms.IssueTruncated = nil
				ms.EnrichChecked = 0
				ms.EnrichTotal = 0
			}

		case runtime.PatchResourceList:
			// Apply enrichment data (findings + issue badge) to the controller's
			// enrichment store. Resource rows themselves arrive via applyResourcesLoaded
			// (called from the task-result lane); this intent carries Wave-2 data only.
			if v.Enrichment != nil {
				c.applyEnrichmentState(v.ResourceType, 0, false, v.Enrichment.Findings)
			}

		// TODO PR-C: PatchDetail mutates state lifted in PR-C
		// TODO PR-C: FlashIntent mutates state lifted in PR-C
		// TODO PR-C: ClearFlash mutates state lifted in PR-C
		// TODO PR-C: SetErrorHintIntent mutates state lifted in PR-C
		// TODO PR-C: AppendErrorHistoryIntent mutates state lifted in PR-C
		// TODO PR-C: ClearActiveListLoadingIntent mutates state lifted in PR-C
		// TODO PR-C: RefreshActiveListIntent mutates state lifted in PR-C
		// TODO PR-C: PatchResourceCache mutates state lifted in PR-C
		// TODO PR-C: PatchRelatedCache mutates state lifted in PR-C
		// TODO PR-C: PatchLazyResourceCache mutates state lifted in PR-C
		// TODO PR-C: SetIdentityIntent mutates state lifted in PR-C
		// TODO PR-C: HeaderInvalidateIntent mutates state lifted in PR-C
		// TODO PR-C: ApplyThemeIntent mutates state lifted in PR-C
		default:
			_ = v
		}
	}
	return c.snapshot()
}

// Snapshot builds a ViewState from the current controller state. In PR-A
// only the Header, FrameTitle, and BodyKind are populated; full body
// rendering is added in PR-C when per-screen state is lifted here.
//
// Snapshot never panics on an empty stack — it returns a ViewState with
// BodyKindUnknown.
func (c *Controller) Snapshot() ViewState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snapshot()
}

// snapshot is the lock-free implementation of Snapshot.
// Callers must hold c.mu (at least read).
func (c *Controller) snapshot() ViewState {
	vs := ViewState{
		Header: Header{
			Profile: c.core.Profile(),
			Region:  c.core.Region(),
		},
	}
	if len(c.stack) == 0 {
		vs.Body.Kind = BodyKindUnknown
		return vs
	}
	top := c.stack[len(c.stack)-1]
	vs.FrameTitle = string(top.ID)
	vs.Body.Kind = bodyKindForScreen(top)
	if top.State.Menu != nil {
		vs.Body.Menu = buildMenuBody(top.State.Menu)
		vs.FrameTitle = menuFrameTitle(top.State.Menu)
	}
	if top.State.List != nil {
		vs.Body.List = c.buildListBody(top.Ctx, top.State.List)
		vs.FrameTitle = c.buildListFrameTitle(top.Ctx, top.State.List)
	}
	if top.State.Selector != nil {
		vs.Body.Selector = buildSelectorBody(top.State.Selector)
		vs.FrameTitle = selectorFrameTitle(top.State.Selector)
	}
	if top.State.Text != nil {
		vs.Body.Text = buildTextBody(top.State.Text)
	}
	if top.State.Detail != nil {
		vs.Body.Detail = buildDetailBody(top.State.Detail, c.viewConfig)
		vs.FrameTitle = c.detailFrameTitleLocked()
	}
	return vs
}

// applyNavResult converts a NavigateResult into PushScreen/ReplaceScreen/PopScreen
// stack operations. Called by Apply after HandleNavigate returns.
//
// The adapter (not the runtime) decides which ScreenID to push for each kind;
// this method encodes that mapping for the headless controller.
//
// Kinds that require selected-row or resource data (PushDetail, PushYAML,
// PushJSON, PushResourceList/Cached, FetchReveal) are deferred to PR-C where
// per-screen state is lifted into the controller.
func (c *Controller) applyNavResult(res runtime.NavigateResult) {
	switch res.Kind {
	case runtime.NavigateKindPopAll:
		// Pop back to the root menu — leave exactly one screen, never empty.
		// (Popping via ApplyIntents would now stop at the len<=1 guard anyway;
		// pop directly to keep the intent clear.)
		if len(c.stack) > 1 {
			c.stack = c.stack[:1]
		}

	case runtime.NavigateKindPushHelp:
		c.applyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenHelp}})

	case runtime.NavigateKindPushRegion:
		c.applyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenRegion}})

	case runtime.NavigateKindPushTheme:
		c.applyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenTheme}})

	case runtime.NavigateKindFetchProfiles:
		// No stack change — the adapter starts the fetch task; when the result
		// arrives (ProfilesLoaded), HandleProfilesLoaded pushes ScreenProfileSelector.

	case runtime.NavigateKindFlash, runtime.NavigateKindNoop:
		// No stack change — flash is surfaced via FlashIntent in the intent stream.

	case runtime.NavigateKindPushResourceList, runtime.NavigateKindPushResourceListCached:
		intent := runtime.PushScreen{
			ID:      runtime.ScreenResourceList,
			Context: runtime.ScreenContext{ResourceType: res.ResolvedType},
		}
		if res.ReplaceCurrent {
			c.applyIntents([]runtime.UIIntent{runtime.ReplaceScreen{ID: intent.ID, Context: intent.Context}})
		} else {
			c.applyIntents([]runtime.UIIntent{intent})
		}
		c.ensureListState()
		// For the cached path, populate rows immediately from the cache entry so
		// headless/web callers see data without waiting for a fetch round-trip.
		if res.Kind == runtime.NavigateKindPushResourceListCached && res.CachedEntry != nil {
			top := &c.stack[len(c.stack)-1]
			c.applyResourcesLoaded(top.State.List, res.ResolvedType, res.CachedEntry.Resources, res.CachedEntry.Pagination, false)
		}

	case runtime.NavigateKindPushDetail:
		if res.Resource == nil {
			return
		}
		intent := runtime.PushScreen{
			ID: runtime.ScreenDetail,
			Context: runtime.ScreenContext{
				ResourceType: res.ResolvedType,
				ResourceID:   res.Resource.ID,
			},
		}
		c.applyIntents([]runtime.UIIntent{intent})
		// Use lock-free variants — applyNavResult is always called while c.mu
		// is already held by Apply or Handle.
		c.ensureDetailState(*res.Resource, res.ResolvedType)
		if res.DispatchRelated {
			c.initDetailRelatedRows(res.ResolvedType)
		}

	case runtime.NavigateKindPushYAML:
		if res.Resource == nil {
			return
		}
		lines := resourceYAMLLines(*res.Resource)
		intent := runtime.PushScreen{
			ID: runtime.ScreenYAML,
			Context: runtime.ScreenContext{
				ResourceType: res.ResolvedType,
				ResourceID:   res.Resource.ID,
			},
		}
		c.applyIntents([]runtime.UIIntent{intent})
		c.ensureTextState(lines)

	case runtime.NavigateKindPushJSON:
		if res.Resource == nil {
			return
		}
		lines := resourceJSONLines(*res.Resource)
		intent := runtime.PushScreen{
			ID: runtime.ScreenJSON,
			Context: runtime.ScreenContext{
				ResourceType: res.ResolvedType,
				ResourceID:   res.Resource.ID,
			},
		}
		c.applyIntents([]runtime.UIIntent{intent})
		c.ensureTextState(lines)

	case runtime.NavigateKindFetchReveal:
		// No stack push yet — the push happens when Handle receives
		// messages.ValueRevealed and routes it to HandleValueRevealed.
		// Tasks are returned by Apply's ActionReveal branch directly.
	}
}

// applyRelatedNavResult converts a NavigationResult into stack operations and
// returns any additional task requests the result spawns.
//
// NavigationResult carries NavigationKind plus TargetType, TargetID,
// RelatedIDs, FetchFilter, FilterText — no ScreenID; the controller maps
// kind → ScreenID.
func (c *Controller) applyRelatedNavResult(res runtime.NavigationResult) []runtime.TaskRequest {
	switch res.Kind {
	case runtime.NavigationKindResourceList:
		intent := runtime.PushScreen{
			ID:      runtime.ScreenResourceList,
			Context: runtime.ScreenContext{ResourceType: res.TargetType},
		}
		c.applyIntents([]runtime.UIIntent{intent})
		c.ensureListState()

	case runtime.NavigationKindFilteredList:
		intent := runtime.PushScreen{
			ID:      runtime.ScreenResourceList,
			Context: runtime.ScreenContext{ResourceType: res.TargetType},
		}
		c.applyIntents([]runtime.UIIntent{intent})
		c.ensureListState()
		if ls := c.topListState(); ls != nil && res.FilterText != "" {
			ls.Filter = res.FilterText
		}

	case runtime.NavigationKindDetail:
		// Navigate to a detail view for the single related resource.
		// The resource itself must be fetched — the task is returned by
		// HandleRelatedNavigate and starts the KindFetchByIDDetail pipeline.
		// We push a ScreenDetail placeholder so the stack reflects the intent
		// immediately; the detail state is seeded when the fetch result arrives.
		intent := runtime.PushScreen{
			ID:      runtime.ScreenDetail,
			Context: runtime.ScreenContext{ResourceType: res.TargetType, ResourceID: res.TargetID},
		}
		c.applyIntents([]runtime.UIIntent{intent})

	case runtime.NavigationKindEnterChildView:
		// Delegate to the same path used by ActionChildView but with the
		// target type already resolved. Build a minimal EnterChildViewEvent.
		ev := runtime.EnterChildViewEvent{
			ChildType: res.TargetType,
		}
		intents, tasks := c.core.HandleEnterChildView(ev)
		c.applyIntents(intents)
		if len(c.stack) > 0 {
			top := &c.stack[len(c.stack)-1]
			if top.ID == runtime.ScreenChildList {
				top.Ctx.ResourceType = res.TargetType
				if top.State.List == nil {
					top.State.List = &ListState{Loading: true}
				}
				if res.FetchFilter != nil {
					top.State.List.ParentContext = res.FetchFilter
				}
			}
		}
		return tasks

	case runtime.NavigationKindFlash:
		// Flash is surfaced as a FlashIntent by HandleRelatedNavigate — no
		// stack change needed here.

	default:
		// NavigationKindUnknown or future kinds: no-op.
	}
	return nil
}

// menuPageSize is the default cursor jump for PageUp/PageDown when the renderer
// does not supply its viewport page size. The controller is renderer-neutral and
// has no terminal height; 10 matches a typical visible window.
const menuPageSize = 10

// menuPageSizeFor returns the page size for a PageUp/PageDown action: the
// renderer-supplied viewport page size (Action.N) when given, else the default.
// The TUI passes max(height-1, 1) so page movement tracks the live viewport.
func menuPageSizeFor(a Action) int {
	if a.N > 0 {
		return a.N
	}
	return menuPageSize
}

// topMenuState returns the MenuState of the top-of-stack screen if it is
// ScreenMenu, or nil otherwise.
func (c *Controller) topMenuState() *MenuState {
	if len(c.stack) == 0 {
		return nil
	}
	top := c.stack[len(c.stack)-1]
	if top.ID != runtime.ScreenMenu {
		return nil
	}
	return c.stack[len(c.stack)-1].State.Menu
}

// rootMenuState returns the MenuState of the root (bottom) screen if it is
// ScreenMenu. Menu intents always target the root menu regardless of which
// screen is currently on top.
func (c *Controller) rootMenuState() *MenuState {
	if len(c.stack) == 0 {
		return nil
	}
	if c.stack[0].ID != runtime.ScreenMenu {
		return nil
	}
	return c.stack[0].State.Menu
}

// hasResourceScreen reports whether the stack contains at least one screen that
// can initiate a reveal (ScreenResourceList, ScreenChildList, or ScreenDetail).
// Used to guard spurious ValueRevealed events delivered when the only screen
// visible is the menu (e.g. late delivery after a profile/region switch).
// Caller must hold c.mu.
func (c *Controller) hasResourceScreen() bool {
	for _, s := range c.stack {
		if s.ID == runtime.ScreenResourceList ||
			s.ID == runtime.ScreenChildList ||
			s.ID == runtime.ScreenDetail {
			return true
		}
	}
	return false
}

// menuVisibleItems returns the resource types visible under the current
// MenuState filter + attention settings, mirroring mainmenu.go applyFilter.
//
// PR-C 1b: converge with mainmenu.go applyFilter + isVisibleUnderIssueFilter.
func menuVisibleItems(ms *MenuState, all []resource.ResourceTypeDef) []resource.ResourceTypeDef {
	var result []resource.ResourceTypeDef
	if len(ms.Filter) < 2 {
		result = all
	} else {
		q := strings.ToLower(ms.Filter)
		result = make([]resource.ResourceTypeDef, 0, len(all))
		for _, item := range all {
			if strings.Contains(strings.ToLower(item.Name), q) ||
				strings.Contains(strings.ToLower(item.ShortName), q) {
				result = append(result, item)
			}
		}
	}

	if ms.AttentionOnly {
		filtered := make([]resource.ResourceTypeDef, 0, len(result))
		for _, item := range result {
			if menuIsVisibleUnderIssueFilter(ms, item, menuActiveKey(ms, item)) {
				filtered = append(filtered, item)
			}
		}
		result = filtered
	}
	return result
}

// menuIsVisibleUnderIssueFilter mirrors mainmenu.go isVisibleUnderIssueFilter.
//
// item is the catalog entry; activeKey is the key under which intent data is
// stored for this item (from menuActiveKey — may be an alias like "rds" for
// the "dbi" type).
//
// PR-C 1b: converge with mainmenu.go isVisibleUnderIssueFilter.
func menuIsVisibleUnderIssueFilter(ms *MenuState, item resource.ResourceTypeDef, activeKey string) bool {
	known := ms.IssueKnown != nil && ms.IssueKnown[activeKey]
	// ExcludeFromIssueBadge types are never probed — hide them in attention mode,
	// even at cold-start, UNLESS issue data was explicitly recorded for them (a
	// real detected issue beats the exclusion). In production these types are
	// never probed, so this is equivalent to an absolute exclusion; the
	// conditional only matters for tests that inject issues directly.
	if item.ExcludeFromIssueBadge && !known {
		return false
	}
	// Unknown non-excluded type: visible only during true cold-start (no type
	// probed anywhere); once any probe lands, unknown types hide.
	if !known {
		return len(ms.IssueKnown) == 0
	}
	if ms.IssueCounts != nil && ms.IssueCounts[activeKey] > 0 {
		return true
	}
	return ms.IssueTruncated != nil && ms.IssueTruncated[activeKey]
}

// menuSkipUnavailable advances the cursor past confirmed-empty resource types,
// mirroring mainmenu.go skipUnavailable.
//
// PR-C 1b: converge with mainmenu.go skipUnavailable.
func menuSkipUnavailable(ms *MenuState, visible []resource.ResourceTypeDef, direction int) {
	if ms.Availability == nil || len(visible) == 0 {
		return
	}
	total := len(visible)
	start := ms.Cursor

	cur := start
	for cur >= 0 && cur < total {
		item := visible[cur]
		key := menuActiveKey(ms, item)
		isTruncated := ms.Truncated != nil && ms.Truncated[key]
		if count, known := ms.Availability[key]; !known || count > 0 || isTruncated {
			ms.Cursor = cur
			return
		}
		cur += direction
	}

	cur = start - direction
	for cur >= 0 && cur < total {
		item := visible[cur]
		key := menuActiveKey(ms, item)
		isTruncated := ms.Truncated != nil && ms.Truncated[key]
		if count, known := ms.Availability[key]; !known || count > 0 || isTruncated {
			ms.Cursor = cur
			return
		}
		cur -= direction
	}
}

// menuActiveKey returns the key under which intent data for the given
// ResourceTypeDef is stored in a MenuState map. Intents are stored under
// whatever key the runtime emits (e.g. "rds" for the "dbi" type); this
// function resolves that key by checking the item's ShortName and all its
// Aliases in order, returning the first one present in any of the three
// intent maps. Falls back to item.ShortName when nothing is found.
//
// This allows buildMenuBody to expose the same key as the intent used —
// so MenuEntry.ShortName matches what the runtime and tests expect.
func menuActiveKey(ms *MenuState, item resource.ResourceTypeDef) string {
	candidates := make([]string, 0, 1+len(item.Aliases))
	candidates = append(candidates, item.ShortName)
	candidates = append(candidates, item.Aliases...)
	for _, c := range candidates {
		if ms.Availability != nil {
			if _, ok := ms.Availability[c]; ok {
				return c
			}
		}
		if ms.IssueKnown != nil {
			if ms.IssueKnown[c] {
				return c
			}
		}
	}
	return item.ShortName
}

// buildMenuBody constructs a MenuBody from MenuState + the resource catalog.
// Applies the same filter + attention + skip-unavailable + badge logic as
// mainmenu.go View(), but produces renderer-agnostic data instead of styled
// strings.
//
// PR-C 1b: converge with mainmenu.go View() + FrameTitle().
func buildMenuBody(ms *MenuState) *MenuBody {
	all := resource.AllResourceTypes()
	visible := menuVisibleItems(ms, all)

	cursor := ms.Cursor
	if cursor >= len(visible) && len(visible) > 0 {
		cursor = len(visible) - 1
	}

	entries := make([]MenuEntry, 0, len(visible))
	for _, item := range visible {
		// Resolve the key under which intent data was stored for this type.
		// Intents may use an alias ("rds") rather than the canonical ShortName
		// ("dbi"); menuActiveKey finds whichever key has data, falling back to
		// item.ShortName when no intent has been received yet.
		activeKey := menuActiveKey(ms, item)

		alias := ":" + item.ShortName
		if len(item.Aliases) > 0 {
			alias = ":" + item.Aliases[0]
		}

		avail, availKnown := 0, false
		if ms.Availability != nil {
			avail, availKnown = ms.Availability[activeKey]
		}
		availTruncated := ms.Truncated != nil && ms.Truncated[activeKey]

		badge := IssueBadge{}
		if ms.IssueKnown != nil && ms.IssueKnown[activeKey] {
			cnt := 0
			if ms.IssueCounts != nil {
				cnt = ms.IssueCounts[activeKey]
			}
			trunc := ms.IssueTruncated != nil && ms.IssueTruncated[activeKey]
			badge = IssueBadge{Count: cnt, Truncated: trunc}
		}

		entries = append(entries, MenuEntry{
			ShortName:      activeKey,
			Display:        item.Name,
			Alias:          alias,
			Category:       item.Category,
			IssueBadge:     badge,
			Availability:   avail,
			AvailKnown:     availKnown,
			AvailTruncated: availTruncated,
		})
	}

	return &MenuBody{
		Entries:       entries,
		Selected:      cursor,
		Filter:        ms.Filter,
		AttentionOnly: ms.AttentionOnly,
		Progress:      menuProgressIndicator(ms),
	}
}

// menuFrameTitle mirrors mainmenu.go FrameTitle().
//
// PR-C 1b: converge with mainmenu.go FrameTitle().
func menuFrameTitle(ms *MenuState) string {
	all := resource.AllResourceTypes()
	total := len(all)
	visible := menuVisibleItems(ms, all)
	filtered := len(visible)

	var title string
	switch {
	case ms.Filter != "" || ms.AttentionOnly:
		title = "resource-types(" + itoa(filtered) + "/" + itoa(total) + ")"
	default:
		title = "resource-types(" + itoa(total) + ")"
	}
	if ms.AttentionOnly {
		title += " [!]"
	}
	if ms.EnrichTotal > 0 && ms.EnrichChecked < ms.EnrichTotal {
		title += " [enriching " + itoa(ms.EnrichChecked) + "/" + itoa(ms.EnrichTotal) + "]"
	}
	return title
}

// menuProgressIndicator returns the scan/enrichment progress suffix only —
// empty when no scan is active. This is what MenuBody.Progress carries;
// menuFrameTitle() carries the full frame title string (base + suffix).
func menuProgressIndicator(ms *MenuState) string {
	if ms.EnrichTotal > 0 && ms.EnrichChecked < ms.EnrichTotal {
		return "[enriching " + itoa(ms.EnrichChecked) + "/" + itoa(ms.EnrichTotal) + "]"
	}
	if ms.AvailTotal > 0 && ms.AvailChecked < ms.AvailTotal {
		return "[checking " + itoa(ms.AvailChecked) + "/" + itoa(ms.AvailTotal) + "]"
	}
	return ""
}

// itoa converts an int to its decimal string representation without importing
// strconv (mirrors the views.itoa helper kept in the same conceptual layer).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// MenuFrameTitle returns the frame-border title for the main-menu screen,
// delegating to menuFrameTitle with the root MenuState. Returns an empty
// string when the root screen is not a menu.
func (c *Controller) MenuFrameTitle() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ms := c.rootMenuState()
	if ms == nil {
		return ""
	}
	return menuFrameTitle(ms)
}

// MenuSelected returns the ResourceTypeDef at the current cursor and a bool
// that is true when navigation is permitted (i.e. the item is not confirmed
// empty). Mirrors the Enter-key guard in ActionSelect.
func (c *Controller) MenuSelected() (resource.ResourceTypeDef, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ms := c.rootMenuState()
	if ms == nil {
		return resource.ResourceTypeDef{}, false
	}
	all := resource.AllResourceTypes()
	visible := menuVisibleItems(ms, all)
	if len(visible) == 0 {
		return resource.ResourceTypeDef{}, false
	}
	// Background issue/availability intents can shrink the visible list while the
	// stored cursor still points past the end. Snapshot clamps the displayed
	// selection to the last visible row, so clamp here too — otherwise Enter on
	// the highlighted last row would hit a stale guard and become a no-op.
	cursor := ms.Cursor
	if cursor >= len(visible) {
		cursor = len(visible) - 1
	}
	selected := visible[cursor]
	if ms.Availability != nil {
		key := menuActiveKey(ms, selected)
		isTruncated := ms.Truncated != nil && ms.Truncated[key]
		if count, known := ms.Availability[key]; known && count == 0 && !isTruncated {
			return selected, false
		}
	}
	return selected, true
}

// GetMenuAvailability returns a copy of the root MenuState availability map.
// Returns nil when no availability data has been recorded.
func (c *Controller) GetMenuAvailability() map[string]int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ms := c.rootMenuState()
	if ms == nil || ms.Availability == nil {
		return nil
	}
	cp := make(map[string]int, len(ms.Availability))
	maps.Copy(cp, ms.Availability)
	return cp
}

// GetMenuTruncated returns a copy of the root MenuState truncated map.
// Returns nil when no truncation data has been recorded.
func (c *Controller) GetMenuTruncated() map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ms := c.rootMenuState()
	if ms == nil || ms.Truncated == nil {
		return nil
	}
	cp := make(map[string]bool, len(ms.Truncated))
	maps.Copy(cp, ms.Truncated)
	return cp
}

// GetMenuIssueCounts returns a copy of the root MenuState issue-count map.
// Returns nil when no issue data has been recorded.
func (c *Controller) GetMenuIssueCounts() map[string]int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ms := c.rootMenuState()
	if ms == nil || ms.IssueCounts == nil {
		return nil
	}
	cp := make(map[string]int, len(ms.IssueCounts))
	maps.Copy(cp, ms.IssueCounts)
	return cp
}

// GetMenuIssueKnown returns a copy of the root MenuState issue-known map.
// Returns nil when no issue data has been recorded.
func (c *Controller) GetMenuIssueKnown() map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ms := c.rootMenuState()
	if ms == nil || ms.IssueKnown == nil {
		return nil
	}
	cp := make(map[string]bool, len(ms.IssueKnown))
	maps.Copy(cp, ms.IssueKnown)
	return cp
}

// GetMenuIssueTruncated returns a copy of the root MenuState issue-truncated map.
// Returns nil when no issue-truncation data has been recorded.
func (c *Controller) GetMenuIssueTruncated() map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ms := c.rootMenuState()
	if ms == nil || ms.IssueTruncated == nil {
		return nil
	}
	cp := make(map[string]bool, len(ms.IssueTruncated))
	maps.Copy(cp, ms.IssueTruncated)
	return cp
}

// bodyKindForScreen maps a Screen to the BodyKind a renderer uses to
// select the correct template/view.
func bodyKindForScreen(s Screen) BodyKind {
	switch s.ID {
	case runtime.ScreenMenu:
		return BodyKindMenu
	case runtime.ScreenProfileSelector, runtime.ScreenRegion, runtime.ScreenTheme:
		return BodyKindSelector
	case runtime.ScreenReveal, runtime.ScreenDetail:
		return BodyKindDetail
	case runtime.ScreenChildList, runtime.ScreenResourceList:
		return BodyKindList
	case runtime.ScreenYAML, runtime.ScreenJSON:
		return BodyKindText
	case runtime.ScreenHelp:
		return BodyKindHelp
	case runtime.ScreenIdentity:
		return BodyKindIdentity
	default:
		// Capability screens and future IDs not yet enumerated here.
		return BodyKindUnknown
	}
}
