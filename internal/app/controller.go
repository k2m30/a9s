package app

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
	"sync"
	"time"

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

// applyLocked is the lock-free implementation of Apply. Callers must hold c.mu (write).
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
		c.identityLoading = true
		c.identityResult = nil
		c.identityErrMsg = ""
		fetchTask := runtime.TaskRequest{
			Key:     runtime.TaskKey{Kind: runtime.TaskKindFetchIdentity},
			Payload: runtime.FetchIdentityPayload{},
		}
		return c.snapshot(), []runtime.TaskRequest{fetchTask}

	case ActionOpenErrorLog:
		// Mirror the TUI's '!' key: flash when no errors recorded; otherwise push
		// a text screen with the log entries newest-first.
		c.showErrorHint = false
		if len(c.errorHistory) == 0 {
			intents, tasks := c.core.HandleFlash(runtime.FlashEvent{
				Text:    "No errors this session",
				IsError: false,
				NewGen:  c.core.ConnectGen(),
			})
			c.applyIntents(intents)
			return c.snapshot(), tasks
		}
		var sb strings.Builder
		for i := len(c.errorHistory) - 1; i >= 0; i-- {
			e := c.errorHistory[i]
			fmt.Fprintf(&sb, "[%s] %s\n", e.t.Format("15:04:05"), e.message)
		}
		lines := strings.Split(strings.TrimRight(sb.String(), "\n"), "\n")
		c.applyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenErrorLog}})
		c.ensureTextState(lines)
		return c.snapshot(), nil

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
		// Resource/child list: open the detail of the currently-selected row,
		// identical to ActionOpenDetail. Enter and row-clicks in the web UI both
		// send ActionSelect; the TUI uses ActionOpenDetail from its key handler.
		if ls := c.topListState(); ls != nil {
			return c.openSelectedListDetail()
		}

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
			if focusedRow != nil && isActionableDetailRow(*focusedRow) {
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
				return c.snapshot(), c.dispatchRelatedNavigate(ev)
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

	case ActionRelatedSelect:
		// Web UI click path: navigate to the related row at the visible index in
		// Arg. Sets RelatedFocus + RelatedCursor then delegates to the same
		// HandleRelatedNavigate path as the keyboard Enter in ActionSelect.
		ds := c.topDetailState()
		if ds == nil {
			return c.snapshot(), nil
		}
		clickIdx, err := strconv.Atoi(strings.TrimSpace(a.Arg))
		if err != nil || clickIdx < 0 {
			return c.snapshot(), nil
		}
		// Locate the row at clickIdx in the filtered visible list (mirrors
		// buildDetailRelatedBlocks / detailRelatedVisibleCount filter logic).
		query := strings.TrimSpace(strings.ToLower(ds.RelatedFilter))
		var targetRow *DetailRelatedRow
		visIdx := 0
		for i := range ds.RelatedRows {
			row := &ds.RelatedRows[i]
			if isSelfPivotZeroDetailRow(*row, ds.ResourceType) {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(row.DisplayName), query) {
				continue
			}
			if visIdx == clickIdx {
				targetRow = row
				break
			}
			visIdx++
		}
		if targetRow == nil || !isActionableDetailRow(*targetRow) {
			// Dead-end row: loading, error, count==-1 without FetchFilter, or
			// confirmed zero without FetchFilter/Approximate. No navigation.
			return c.snapshot(), nil
		}
		// Sync cursor state so the selection highlight is consistent with the
		// navigation that follows.
		ds.RelatedFocus = true
		ds.RelatedCursor = clickIdx
		// Navigate — identical to the ActionSelect related-Enter path.
		targetID := ""
		if len(targetRow.ResourceIDs) == 1 {
			targetID = targetRow.ResourceIDs[0]
		}
		var checker resource.RelatedChecker
		for _, def := range resource.GetRelated(ds.ResourceType) {
			if def.TargetType == targetRow.TargetType {
				checker = def.Checker
				break
			}
		}
		ev := runtime.RelatedNavigateEvent{
			TargetType:     targetRow.TargetType,
			SourceResource: ds.Resource,
			SourceType:     ds.ResourceType,
			TargetID:       targetID,
			RelatedIDs:     targetRow.ResourceIDs,
			FetchFilter:    targetRow.FetchFilter,
			Checker:        checker,
		}
		return c.snapshot(), c.dispatchRelatedNavigate(ev)

	case ActionFieldSelect:
		// Web UI click path: navigate to the resource linked by the navigable
		// detail field at the visible index in Arg. Mirrors the TUI Enter-on-
		// navigable-field path (TargetType + NavID/Value → HandleRelatedNavigate).
		ds := c.topDetailState()
		if ds == nil {
			return c.snapshot(), nil
		}
		fieldIdx, err := strconv.Atoi(strings.TrimSpace(a.Arg))
		if err != nil || fieldIdx < 0 {
			return c.snapshot(), nil
		}
		// Build the fields list using the same pipeline as buildDetailBody so
		// $i in the template aligns with the slice index here.
		fields := buildDetailBody(ds, c.viewConfig).Fields
		if fieldIdx >= len(fields) {
			return c.snapshot(), nil
		}
		field := fields[fieldIdx]
		if !field.IsNavigable || field.TargetType == "" {
			return c.snapshot(), nil
		}
		// Mirror TUI: NavID overrides Value when present.
		targetID := field.Value
		if field.NavID != "" {
			targetID = field.NavID
		}
		// No RelatedIDs / FetchFilter / Checker needed: HandleRelatedNavigate
		// routes a single-ID event to a cache-hit detail or a by-ID fetch.
		ev := runtime.RelatedNavigateEvent{
			TargetType:     field.TargetType,
			SourceResource: ds.Resource,
			SourceType:     ds.ResourceType,
			TargetID:       targetID,
		}
		return c.snapshot(), c.dispatchRelatedNavigate(ev)

	// --- Row-dependent actions: require selected row + per-screen state ---

	case ActionOpenDetail:
		return c.openSelectedListDetail()

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
				ParentContext:     ls.ParentContext,
				FetchFilter:       ls.FetchFilter,
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

	// messages.APIError: a failed fetch clears the list Loading flag and surfaces
	// an error flash. The TUI bumps flash.gen before calling HandleAPIError; the
	// headless controller has no flash.gen, so ConnectGen serves as stable stand-in
	// (same pattern as ActionSelectProfile/Region). The FlashTick task returned by
	// HandleAPIError is suppressed here — it is only meaningful in a running event
	// loop (TUI/web timer); the headless path has no loop to process it.
	// IsStale is not applicable here — APIError has AcceptZeroGen=true.
	if msg, ok := ev.(messages.APIError); ok {
		apiIntents, _ := c.core.HandleAPIError(runtime.APIErrorEvent{
			Err:    msg.Err,
			NewGen: c.core.ConnectGen(),
		})
		c.applyIntents(apiIntents)
	}

	// messages.IdentityError: the identity fetch failed. Core.HandleEvent routes
	// this through HandleIdentityError which clears IdentityFetching but does not
	// store the error string (it is view-layer state). Store it here so snapshot
	// can build IdentityBody.ErrorMsg. IsStale uses AspectConnect + Gen.
	if msg, ok := ev.(messages.IdentityError); ok && !messages.IsStale(msg, c.core) {
		c.identityLoading = false
		c.identityErrMsg = msg.Err
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
			result.Result.Count, false, errMsg, result.Result.Approximate, result.Result.ResourceIDs, result.Result.FetchFilter)
	}
}

// mergeDetailRelatedRow updates or appends one RelatedRow in ds, matching by
// DisplayName. Mirrors ApplyDetailRelatedResult but operates on a DetailState
// pointer directly rather than the top-of-stack screen.
func mergeDetailRelatedRow(ds *DetailState, displayName, targetType string, count int, loading bool, errMsg string, approximate bool, resourceIDs []string, fetchFilter map[string]string) {
	for i := range ds.RelatedRows {
		if ds.RelatedRows[i].DisplayName == displayName {
			ds.RelatedRows[i].Count = count
			ds.RelatedRows[i].Loading = loading
			ds.RelatedRows[i].Err = errMsg
			ds.RelatedRows[i].Approximate = approximate
			ds.RelatedRows[i].ResourceIDs = resourceIDs
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
		ResourceIDs: resourceIDs,
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

		case runtime.SetIdentityIntent:
			// SetIdentityIntent is emitted by Core.HandleIdentityLoaded (via
			// HandleEvent) when the identity fetch succeeds. Store the resolved
			// domain mirror so snapshot can build IdentityBody without importing
			// internal/aws or inspecting the TUI view stack.
			if v.Identity != nil {
				c.identityResult = v.Identity
				c.identityLoading = false
				c.identityErrMsg = ""
			}

		case runtime.FlashIntent:
			// Surface the transient notification (e.g. the API-error flash from
			// HandleAPIError) as Header.Flash; cleared at the start of the next Apply.
			c.flash = Flash{Text: v.Text, IsError: v.IsError}

		case runtime.ClearFlash:
			c.flash = Flash{}

		case runtime.ClearActiveListLoadingIntent:
			// A failed AWS fetch must drop the spinner on the active list rather
			// than leaving it stuck Loading=true (emitted by HandleAPIError).
			if ls := c.topListState(); ls != nil {
				ls.Loading = false
			}

		case runtime.SetErrorHintIntent:
			c.showErrorHint = v.Show

		case runtime.AppendErrorHistoryIntent:
			c.errorHistory = append(c.errorHistory, controllerErrorEntry{
				t:       v.Time,
				message: v.Message,
			})

		// TODO PR-C: PatchDetail mutates state lifted in PR-C
		// TODO PR-C: RefreshActiveListIntent mutates state lifted in PR-C
		// TODO PR-C: PatchResourceCache mutates state lifted in PR-C
		// TODO PR-C: PatchRelatedCache mutates state lifted in PR-C
		// TODO PR-C: PatchLazyResourceCache mutates state lifted in PR-C
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
			Profile:          c.core.Profile(),
			Region:           c.core.Region(),
			Flash:            c.flash,
			ErrorHintVisible: c.showErrorHint && len(c.errorHistory) > 0,
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
		vs.Footer = MenuFooterHints()
	}
	if top.State.List != nil {
		vs.Body.List = c.buildListBody(top.Ctx, top.State.List)
		vs.FrameTitle = c.buildListFrameTitle(top.Ctx, top.State.List)
		vs.Footer = c.buildListFooterHints(top.Ctx, top.State.List)
	}
	if top.State.Selector != nil {
		vs.Body.Selector = buildSelectorBody(top.State.Selector)
		vs.FrameTitle = selectorFrameTitle(top.State.Selector)
	}
	if top.State.Text != nil {
		vs.Body.Text = buildTextBody(top.State.Text)
		vs.Footer = c.buildTextFooterHints(top.ID, top.Ctx)
	}
	if top.State.Detail != nil {
		vs.Body.Detail = buildDetailBody(top.State.Detail, c.viewConfig)
		vs.FrameTitle = c.detailFrameTitleLocked()
		vs.Footer = c.buildDetailFooterHints(top.State.Detail)
	}
	if top.ID == runtime.ScreenHelp {
		vs.Body.Help = buildHelpBody()
	}
	if top.ID == runtime.ScreenIdentity {
		vs.Body.Identity = c.buildIdentityBody()
	}
	return vs
}

// buildListFooterHints builds the footer key hints for a resource-list screen,
// ported faithfully from ResourceListModel.BottomHints() so the controller is
// the single source of truth for all renderers. Callers must hold c.mu.
func (c *Controller) buildListFooterHints(ctx runtime.ScreenContext, ls *ListState) []KeyHint {
	var hints []KeyHint

	if ls.EscPops {
		hints = append(hints, KeyHint{Key: "esc", Help: "Back"})
	}

	td := resource.FindResourceType(ctx.ResourceType)
	if td == nil {
		if fv, ok := c.fallbackTypeDefs[ctx.ResourceType]; ok {
			td = &fv
		}
	}
	if td != nil {
		var enterChild *resource.ChildViewDef
		for i := range td.Children {
			if td.Children[i].Key == "enter" {
				enterChild = &td.Children[i]
				break
			}
		}
		if enterChild != nil {
			showEnterChild := true
			if enterChild.DrillCondition != nil {
				sel, ok := c.listSelected()
				showEnterChild = ok && enterChild.DrillCondition(sel)
			}
			if showEnterChild {
				desc := enterChild.ChildType
				if ct := resource.GetChildType(enterChild.ChildType); ct != nil {
					desc = ct.Name
				}
				hints = append(hints, KeyHint{Key: "enter", Help: desc})
				hints = append(hints, KeyHint{Key: "d", Help: "Detail"})
			}
		}

		if resource.HasRevealFetcher(td.ShortName) {
			hints = append(hints, KeyHint{Key: "x", Help: "Reveal"})
		}
	}

	hints = append(hints, KeyHint{Key: "y", Help: "YAML"})
	hints = append(hints, KeyHint{Key: "J", Help: "JSON"})

	if td != nil {
		for _, child := range td.Children {
			if child.Key == "enter" {
				continue
			}
			desc := child.ChildType
			if ct := resource.GetChildType(child.ChildType); ct != nil {
				desc = ct.Name
			}
			hints = append(hints, KeyHint{Key: child.Key, Help: desc})
		}

		if td.CloudTrailKey != "" && ls.ParentContext == nil {
			hints = append(hints, KeyHint{Key: "t", Help: "CloudTrail"})
		}
	}

	hints = append(hints, KeyHint{Key: "ctrl+r", Help: "Refresh"})
	hints = append(hints, KeyHint{Key: "ctrl+z", Help: "Only !"})

	if ls.HasPagination {
		hints = append(hints, KeyHint{Key: "m", Help: "More"})
	}

	return hints
}

// buildTextFooterHints builds the footer key hints for a YAML or JSON text
// screen. The CloudTrail hint appears only when the resource type has a
// CloudTrailKey and the resource can be found in the cache. Callers must hold c.mu.
func (c *Controller) buildTextFooterHints(screenID runtime.ScreenID, ctx runtime.ScreenContext) []KeyHint {
	hints := []KeyHint{
		{Key: "w", Help: "Wrap"},
		{Key: "c", Help: "Copy"},
	}
	// CloudTrail hint: needs a resource that has a CloudTrailKey.
	// Skip when no resource type is set (raw-text / reveal YAML path).
	if ctx.ResourceType != "" && ctx.ResourceID != "" {
		// Find the resource in the cache to call BuildCloudTrailFilter.
		for _, r := range c.resourceCache[ctx.ResourceType] {
			if r.ID == ctx.ResourceID {
				if resource.BuildCloudTrailFilter(r, ctx.ResourceType) != nil {
					hints = append(hints, KeyHint{Key: "t", Help: "CloudTrail"})
				}
				break
			}
		}
	}
	return hints
}

// buildDetailFooterHints builds the footer key hints for a detail screen.
// Ported faithfully from DetailModel.BottomHints() so the controller is
// the single source of truth for all renderers. Uses the controller-owned
// FieldCursor and RelatedRows/RelatedFocus state in ds. Callers must hold c.mu.
func (c *Controller) buildDetailFooterHints(ds *DetailState) []KeyHint {
	var hints []KeyHint

	// Right column focused: show Enter-on-selected-type + tab + y + J + t + ctrl+r.
	if ds.RelatedFocus && ds.RelatedVisible {
		// Find the selected row's target type.
		query := strings.TrimSpace(strings.ToLower(ds.RelatedFilter))
		var visibleRows []DetailRelatedRow
		for _, row := range ds.RelatedRows {
			if isSelfPivotZeroDetailRow(row, ds.ResourceType) {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(row.DisplayName), query) {
				continue
			}
			visibleRows = append(visibleRows, row)
		}
		cursor := ds.RelatedCursor
		if cursor >= 0 && cursor < len(visibleRows) {
			selected := visibleRows[cursor]
			if isActionableDetailRow(selected) {
				displayName := selected.TargetType
				if rt := resource.FindResourceType(selected.TargetType); rt != nil {
					displayName = rt.Name
				} else if ct := resource.GetChildType(selected.TargetType); ct != nil {
					displayName = ct.Name
				}
				hints = append(hints, KeyHint{Key: "enter", Help: displayName})
			}
		}
		hints = append(hints, KeyHint{Key: "tab", Help: "Fields"})
		hints = append(hints, KeyHint{Key: "y", Help: "YAML"})
		hints = append(hints, KeyHint{Key: "J", Help: "JSON"})
		if resource.BuildCloudTrailFilter(ds.Resource, ds.ResourceType) != nil {
			hints = append(hints, KeyHint{Key: "t", Help: "CloudTrail"})
		}
		hints = append(hints, KeyHint{Key: "ctrl+r", Help: "Refresh"})
		return hints
	}

	// Left column focused: check navigable field under cursor.
	items := buildDetailFieldItems(ds, c.viewConfig)
	fc := ds.FieldCursor
	if fc >= 0 && fc < len(items) {
		item := items[fc]
		if item.IsNavigable && item.TargetType != "" {
			displayName := item.TargetType
			if rt := resource.FindResourceType(item.TargetType); rt != nil {
				displayName = rt.Name
			} else if ct := resource.GetChildType(item.TargetType); ct != nil {
				displayName = ct.Name
			}
			hints = append(hints, KeyHint{Key: "enter", Help: displayName})
		}
	}

	hints = append(hints, KeyHint{Key: "y", Help: "YAML"})
	hints = append(hints, KeyHint{Key: "J", Help: "JSON"})
	if resource.BuildCloudTrailFilter(ds.Resource, ds.ResourceType) != nil {
		hints = append(hints, KeyHint{Key: "t", Help: "CloudTrail"})
	}

	// Related panel hints.
	if related := resource.GetRelated(ds.ResourceType); len(related) > 0 {
		hints = append(hints, KeyHint{Key: "r", Help: "Related"})
		// "tab: Cols" only when user explicitly toggled the panel on —
		// mirrors DetailModel.BottomHints which checks m.rightColVisible (not auto-shown).
		if ds.RelatedUserVisible {
			hints = append(hints, KeyHint{Key: "tab", Help: "Cols"})
		}
	}

	hints = append(hints, KeyHint{Key: "ctrl+r", Help: "Refresh"})
	hints = append(hints, KeyHint{Key: "w", Help: "Wrap"})

	return hints
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

// dispatchRelatedNavigate calls HandleRelatedNavigate then applyRelatedNavResult
// and merges the two task slices, preferring extraTasks when the same Key
// appears in both (applyRelatedNavResult returns payload-bearing replacements
// for tasks HandleRelatedNavigate emits without payloads, e.g. KindFetchFiltered).
// All three related-nav callers — ActionSelect keyboard, ActionRelatedSelect
// click, and ActionFieldSelect click — use this shared tail.
func (c *Controller) dispatchRelatedNavigate(ev runtime.RelatedNavigateEvent) []runtime.TaskRequest {
	navRes, tasks := c.core.HandleRelatedNavigate(ev)
	extraTasks := c.applyRelatedNavResult(navRes)
	if len(extraTasks) == 0 {
		return tasks
	}
	extraKeys := make(map[runtime.TaskKey]struct{}, len(extraTasks))
	for _, t := range extraTasks {
		extraKeys[t.Key] = struct{}{}
	}
	merged := make([]runtime.TaskRequest, 0, len(tasks)+len(extraTasks))
	for _, t := range tasks {
		if _, replaced := extraKeys[t.Key]; !replaced {
			merged = append(merged, t)
		}
	}
	merged = append(merged, extraTasks...)
	return merged
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
		if ls := c.topListState(); ls != nil {
			if res.FilterText != "" {
				ls.Filter = res.FilterText
			}
			if len(res.FetchFilter) > 0 {
				ls.FetchFilter = res.FetchFilter
				// HandleRelatedNavigate returns a no-payload KindFetchFiltered task
				// (tested as-is by the QA suite). Replace it here with a payload-
				// bearing version so the executor can invoke the filtered fetcher.
				return []runtime.TaskRequest{{
					Key:     runtime.TaskKey{Kind: runtime.KindFetchFiltered, Scope: res.TargetType},
					Cache:   runtime.CacheNone,
					Payload: runtime.FetchFilteredPayload{Filter: res.FetchFilter},
				}}
			}
			if len(res.RelatedIDs) > 0 {
				// Multi-ID related nav with no server-side filter (e.g. an EC2
				// instance → its several security groups): prefilter the list to
				// just the related subset, mirroring the TUI related-list path.
				// Without this the list shows every resource of the target type
				// instead of the related ones. We're already under c.mu, so seed
				// ls directly — PatchListRelatedIDSet would re-lock and deadlock.
				set := make(map[string]struct{}, len(res.RelatedIDs))
				for _, id := range res.RelatedIDs {
					if id != "" {
						set[id] = struct{}{}
					}
				}
				ls.RelatedIDSet = set
			}
		}

	case runtime.NavigationKindDetail:
		// The target resource is already cached: NavigationKindDetail is only
		// returned on a cache hit (ResolveRelatedNavigate), and HandleRelatedNavigate
		// returns no fetch task for it ("the adapter serves these from cached
		// state"). Seed the detail synchronously from the cache — this works for
		// every type (most have no by-id fetcher, so a fetch would land on an empty
		// detail). Mirrors ActionOpenDetail's related-panel handling.
		id := res.TargetID
		if id == "" && len(res.RelatedIDs) == 1 {
			id = res.RelatedIDs[0]
		}
		cached, ok := c.core.RelatedCachedResource(res.TargetType, id)
		if !ok {
			// Defensive: cache unexpectedly missing. Land on a filtered list by id
			// instead of an empty detail so navigation still resolves somewhere.
			c.applyIntents([]runtime.UIIntent{runtime.PushScreen{
				ID:      runtime.ScreenResourceList,
				Context: runtime.ScreenContext{ResourceType: res.TargetType},
			}})
			c.ensureListState()
			if ls := c.topListState(); ls != nil && id != "" {
				ls.Filter = id
			}
			return nil
		}
		c.applyIntents([]runtime.UIIntent{runtime.PushScreen{
			ID:      runtime.ScreenDetail,
			Context: runtime.ScreenContext{ResourceType: res.TargetType, ResourceID: id},
		}})
		c.ensureDetailState(cached, res.TargetType)
		ds := c.topDetailState()
		if ds == nil || len(resource.GetRelated(res.TargetType)) == 0 {
			return nil
		}
		c.initDetailRelatedRows(res.TargetType)
		// Populate the related panel: replay the related cache if present, else
		// dispatch a KindRelatedCheck task — same as ActionOpenDetail.
		ck := runtime.RelatedCacheKey(res.TargetType, cached.ID)
		if cachedRows, hit := c.core.RelatedCacheGet(ck); hit && len(cachedRows) > 0 {
			for _, entry := range cachedRows {
				errMsg := ""
				if entry.Result.Err != nil {
					errMsg = entry.Result.Err.Error()
				}
				mergeDetailRelatedRow(ds, entry.DefDisplayName, entry.Result.TargetType,
					entry.Result.Count, false, errMsg, entry.Result.Approximate, entry.Result.ResourceIDs, entry.Result.FetchFilter)
			}
			return nil
		}
		return []runtime.TaskRequest{{
			Key:     runtime.TaskKey{Kind: runtime.KindRelatedCheck, Scope: res.TargetType + "/" + cached.ID},
			Cache:   runtime.CacheNone,
			Payload: runtime.RelatedCheckPayload{ResourceType: res.TargetType, Resource: cached},
		}}

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
	case runtime.ScreenYAML, runtime.ScreenJSON, runtime.ScreenErrorLog:
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

// buildHelpBody constructs the HelpBody that the web renderer uses to populate
// the ? help overlay. It mirrors the helpGroup structure from
// internal/tui/views/help.go, sourcing the same static keybinding strings.
// The context is "main-menu" (the default) since the controller does not track
// which view opened help; a richer context can be wired in PR-C when per-screen
// state is lifted.
func buildHelpBody() *HelpBody {
	nav := HelpSection{
		Title: "NAVIGATION",
		Hints: []KeyHint{
			{Key: "j/k", Help: "up/down"},
			{Key: "g", Help: "top"},
			{Key: "G", Help: "bottom"},
			{Key: "pgup", Help: "page up"},
			{Key: "pgdn", Help: "page down"},
		},
	}
	actions := HelpSection{
		Title: "ACTIONS",
		Hints: []KeyHint{
			{Key: "enter", Help: "select"},
			{Key: "/", Help: "filter"},
			{Key: ":", Help: "command"},
			{Key: "q", Help: "quit"},
			{Key: "ctrl+c", Help: "force quit"},
		},
	}
	other := HelpSection{
		Title: "OTHER",
		Hints: []KeyHint{
			{Key: "i", Help: "identity"},
			{Key: "!", Help: "error log"},
			{Key: "?", Help: "help"},
			{Key: "esc", Help: "back"},
		},
	}
	commands := HelpSection{
		Title: "COMMANDS",
		Hints: []KeyHint{
			{Key: ":q", Help: "exit"},
			{Key: ":ctx", Help: "switch profile"},
			{Key: ":profile", Help: "switch profile"},
			{Key: ":region", Help: "switch region"},
			{Key: ":theme", Help: "switch theme"},
			{Key: ":help", Help: "show help"},
			{Key: ":root", Help: "main menu"},
			{Key: ":main", Help: "main menu"},
			{Key: ":<res>", Help: "e.g. :ec2 :s3 :lambda"},
		},
	}
	return &HelpBody{
		Context:  "main-menu",
		Sections: []HelpSection{nav, actions, other, commands},
	}
}

// buildIdentityBody constructs the IdentityBody from the controller's
// in-memory identity state. It returns Loading=true while the fetch is in
// flight, ErrorMsg on failure, or the fully-populated fields on success.
// Callers must hold c.mu (at least read).
func (c *Controller) buildIdentityBody() *IdentityBody {
	body := &IdentityBody{
		Profile: c.core.Profile(),
		Region:  c.core.Region(),
	}
	if c.identityLoading {
		body.Loading = true
		return body
	}
	if c.identityErrMsg != "" {
		body.ErrorMsg = c.identityErrMsg
		return body
	}
	if c.identityResult != nil {
		id := c.identityResult
		body.AccountID = id.AccountID
		body.AccountAlias = id.AccountAlias
		body.ARN = id.Arn
		body.IsAssumedRole = id.IsAssumedRole
		body.RoleName = id.RoleName
		body.SessionName = id.SessionName
		body.UserName = id.UserName
	}
	return body
}
