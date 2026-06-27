package app

import (
	"strings"

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
type Controller struct {
	core  *runtime.Core
	stack []Screen
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

// Apply translates a semantic Action into the matching Core command, applies
// the returned UIIntents to the screen stack, enqueues returned TaskRequests,
// and returns the updated ViewState plus newly-enqueued TaskRequests.
//
// USER-INTENT lane: each Action.Kind maps to a specific Core.HandleX method.
// PR-B wires the six navigate/session actions that need no selected-row state.
// PR-C-blocked actions (row-dependent) are kept as documented no-ops.
func (c *Controller) Apply(a Action) (ViewState, []runtime.TaskRequest) {
	switch a.Kind {

	// --- Navigate actions (PR-B) ---

	case ActionOpenHelp:
		res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetHelp})
		c.applyNavResult(res)
		return c.Snapshot(), tasks

	case ActionBack:
		// Pop a single screen, mirroring the TUI's m.popView() — NOT a full
		// collapse (root-collapse is the "root" Command). Per-view Esc semantics
		// (clear filter/search before popping) arrive with PR-C view state.
		c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
		return c.Snapshot(), nil

	case ActionOpenIdentity:
		// The runtime has no NavigateTargetIdentity: the TUI opens the identity
		// screen via direct key-handling (not HandleNavigate). The headless
		// controller pushes ScreenIdentity directly so tests can assert the stack
		// without standing up a full TUI.
		c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenIdentity}})
		c.core.SetIdentityFetching(true)
		// TODO PR-C: render IdentityBody.Loading from the session latch once body state is lifted here.
		fetchTask := runtime.TaskRequest{
			Key:     runtime.TaskKey{Kind: runtime.TaskKindFetchIdentity},
			Payload: runtime.FetchIdentityPayload{},
		}
		return c.Snapshot(), []runtime.TaskRequest{fetchTask}

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
		c.ApplyIntents(intents)
		return c.Snapshot(), tasks

	case ActionSelectRegion:
		intents, tasks := c.core.HandleRegionSelected(runtime.RegionSelectedEvent{
			Region: a.Arg,
			NewGen: c.core.ConnectGen(),
		})
		c.ApplyIntents(intents)
		return c.Snapshot(), tasks

	case ActionSelectTheme:
		intents, tasks := c.core.HandleThemeSelected(runtime.ThemeSelectedEvent{
			Theme: a.Arg,
		})
		c.ApplyIntents(intents)
		return c.Snapshot(), tasks

	// --- Command lane (PR-B) ---

	case ActionCommand:
		// Arg carries a colon-command token (mirrors executeCommand in app_input.go).
		// Only arg-driven tokens are dispatched here; tokens that need selected-row
		// or per-screen state are noted as PR-C TODOs below.
		switch a.Arg {
		case "root", "main":
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetMainMenu})
			c.applyNavResult(res)
			return c.Snapshot(), tasks

		case "profile", "ctx":
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetProfile})
			c.applyNavResult(res)
			return c.Snapshot(), tasks

		case "region":
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetRegion})
			c.applyNavResult(res)
			return c.Snapshot(), tasks

		case "theme":
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetTheme})
			c.applyNavResult(res)
			return c.Snapshot(), tasks

		case "help":
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetHelp})
			c.applyNavResult(res)
			return c.Snapshot(), tasks

		default:
			// Resource short-name or alias (e.g. "ec2", "s3", "dbi").
			if rt := resource.FindResourceType(a.Arg); rt != nil {
				res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
					Target:       runtime.NavigateTargetResourceList,
					ResourceType: a.Arg,
				})
				c.applyNavResult(res)
				return c.Snapshot(), tasks
			}
			// TODO PR-C: "q"/"quit" needs tea.Quit from the renderer, not the controller.
			// Unknown tokens are silently dropped at this layer; the renderer flashes.
		}
		return c.Snapshot(), nil

	// --- Menu screen actions (PR-C slice 1a) ---
	// These only take effect when the top screen is ScreenMenu.

	case ActionMoveUp:
		if ms := c.topMenuState(); ms != nil {
			all := resource.AllResourceTypes()
			visible := menuVisibleItems(ms, all)
			if ms.Cursor > 0 {
				ms.Cursor--
				menuSkipUnavailable(ms, visible, -1)
			}
		}
		return c.Snapshot(), nil

	case ActionMoveDown:
		if ms := c.topMenuState(); ms != nil {
			all := resource.AllResourceTypes()
			visible := menuVisibleItems(ms, all)
			if ms.Cursor < len(visible)-1 {
				ms.Cursor++
				menuSkipUnavailable(ms, visible, +1)
			}
		}
		return c.Snapshot(), nil

	case ActionMoveTop:
		if ms := c.topMenuState(); ms != nil {
			ms.Cursor = 0
			all := resource.AllResourceTypes()
			visible := menuVisibleItems(ms, all)
			menuSkipUnavailable(ms, visible, +1)
		}
		return c.Snapshot(), nil

	case ActionMoveBottom:
		if ms := c.topMenuState(); ms != nil {
			all := resource.AllResourceTypes()
			visible := menuVisibleItems(ms, all)
			if len(visible) > 0 {
				ms.Cursor = len(visible) - 1
			}
			menuSkipUnavailable(ms, visible, -1)
		}
		return c.Snapshot(), nil

	case ActionPageUp:
		if ms := c.topMenuState(); ms != nil {
			all := resource.AllResourceTypes()
			visible := menuVisibleItems(ms, all)
			ms.Cursor -= menuPageSize
			if ms.Cursor < 0 {
				ms.Cursor = 0
			}
			menuSkipUnavailable(ms, visible, -1)
		}
		return c.Snapshot(), nil

	case ActionPageDown:
		if ms := c.topMenuState(); ms != nil {
			all := resource.AllResourceTypes()
			visible := menuVisibleItems(ms, all)
			ms.Cursor += menuPageSize
			if n := len(visible); ms.Cursor >= n {
				ms.Cursor = max(n-1, 0)
			}
			menuSkipUnavailable(ms, visible, +1)
		}
		return c.Snapshot(), nil

	case ActionToggleAttention:
		if ms := c.topMenuState(); ms != nil {
			ms.AttentionOnly = !ms.AttentionOnly
			ms.Cursor = 0
		}
		return c.Snapshot(), nil

	case ActionSetFilter:
		if ms := c.topMenuState(); ms != nil {
			ms.Filter = a.Arg
			ms.Cursor = 0
			ms.ScrollOffset = 0
		}
		return c.Snapshot(), nil

	case ActionSelect:
		if ms := c.topMenuState(); ms != nil {
			all := resource.AllResourceTypes()
			visible := menuVisibleItems(ms, all)
			if len(visible) > 0 && ms.Cursor < len(visible) {
				selected := visible[ms.Cursor]
				// Block navigation to confirmed-empty types (count known, zero, not truncated).
				if ms.Availability != nil {
					isTruncated := ms.Truncated != nil && ms.Truncated[selected.ShortName]
					if count, known := ms.Availability[selected.ShortName]; known && count == 0 && !isTruncated {
						return c.Snapshot(), nil
					}
				}
				res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
					Target:       runtime.NavigateTargetResourceList,
					ResourceType: selected.ShortName,
				})
				c.applyNavResult(res)
				return c.Snapshot(), tasks
			}
		}
		return c.Snapshot(), nil

	// --- PR-C-blocked actions: need selected-row / per-screen view state ---

	case ActionOpenDetail,
		ActionOpenYAML, ActionOpenJSON,
		ActionReveal, ActionChildView,
		ActionToggleRelated,
		ActionLoadMore:
		// TODO PR-C: needs selected-row / view state (see plan PR-C)
		return c.Snapshot(), nil
	}

	// All remaining actions (sort, search, copy, refresh, quit, toggle-wrap,
	// command) either require per-screen state lifted in later PR-C slices or
	// are renderer-only (quit). Return current snapshot with no tasks.
	return c.Snapshot(), nil
}

// Handle feeds an event through runtime.Core.HandleEvent, applies the returned
// UIIntents to the screen stack, enqueues returned TaskRequests, and returns
// the updated ViewState plus those TaskRequests.
//
// TASK-RESULT lane: completed background-task results arrive here. The caller
// is responsible for passing only values that implement runtime.Event
// (i.e. messages.Event) — unrecognised concrete types fall through to
// Core.HandleEvent's default nil, nil path.
func (c *Controller) Handle(ev runtime.Event) (ViewState, []runtime.TaskRequest) {
	intents, tasks := c.core.HandleEvent(ev)
	c.ApplyIntents(intents)
	return c.Snapshot(), tasks
}

// ApplyIntents applies a slice of UIIntents to the controller's screen stack.
// Stack-navigation intents (PushScreen / PopScreen / ReplaceScreen) are fully
// implemented. All other intent variants are no-ops with a PR-C marker so
// tests can drive the stack directly without standing up a full renderer.
//
// ApplyIntents never panics on a PopScreen against an empty stack.
// It returns the post-apply ViewState snapshot.
func (c *Controller) ApplyIntents(intents []runtime.UIIntent) ViewState {
	for _, intent := range intents {
		switch v := intent.(type) {
		case runtime.PushScreen:
			c.stack = append(c.stack, Screen{
				ID:  v.ID,
				Ctx: v.Context,
			})

		case runtime.PopScreen:
			if len(c.stack) > 0 {
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
			if ms := c.rootMenuState(); ms != nil && len(v.Known) > 0 {
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

		// TODO PR-C: PatchResourceList mutates state lifted in PR-C
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
	return c.Snapshot()
}

// Snapshot builds a ViewState from the current controller state. In PR-A
// only the Header, FrameTitle, and BodyKind are populated; full body
// rendering is added in PR-C when per-screen state is lifted here.
//
// Snapshot never panics on an empty stack — it returns a ViewState with
// BodyKindUnknown.
func (c *Controller) Snapshot() ViewState {
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
		// Pop every screen until the stack is empty (return to main menu).
		for len(c.stack) > 0 {
			c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
		}

	case runtime.NavigateKindPushHelp:
		c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenHelp}})

	case runtime.NavigateKindPushRegion:
		c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenRegion}})

	case runtime.NavigateKindPushTheme:
		c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenTheme}})

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
			c.ApplyIntents([]runtime.UIIntent{runtime.ReplaceScreen{ID: intent.ID, Context: intent.Context}})
		} else {
			c.ApplyIntents([]runtime.UIIntent{intent})
		}
		// TODO PR-C: populate rows once ListState + the result lane land.

	// TODO PR-C: NavigateKindPushDetail / NavigateKindPushYAML / NavigateKindPushJSON
	//            need ScreenContext{ResourceType, ResourceID} from result.Resource.
	// TODO PR-C: NavigateKindFetchReveal needs result.Resource for the reveal payload.
	}
}

// applyRelatedNavResult converts a NavigationResult into stack operations.
//
// NavigationResult carries NavigationKind plus TargetType, TargetID,
// RelatedIDs, FetchFilter, FilterText — no ScreenID; the controller maps
// kind → ScreenID.
//
// All NavigationKinds (ResourceList, FilteredList, Detail, EnterChildView)
// need selected-row and per-screen state that PR-C lifts into the controller.
//
//nolint:unused // wired in PR-C
func (c *Controller) applyRelatedNavResult(_ runtime.NavigationResult) {
	// TODO PR-C: needs selected-row / view state (see plan PR-C)
}

// menuPageSize is the cursor jump for PageUp/PageDown on the menu screen.
// The menu has no terminal height here; 10 matches the typical visible window.
const menuPageSize = 10

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
//
// Note on cold-start ordering: the cold-start check (len(IssueKnown)==0) is
// evaluated before ExcludeFromIssueBadge so that the attention-only toggle
// shows ALL types when no probe has landed yet. Once any probe reports,
// ExcludeFromIssueBadge types drop out because they can never be known.
func menuIsVisibleUnderIssueFilter(ms *MenuState, item resource.ResourceTypeDef, activeKey string) bool {
	// Cold-start: no probe has reported anywhere — show everything so the
	// user sees the full catalog rather than an empty menu.
	if len(ms.IssueKnown) == 0 {
		return true
	}
	if item.ExcludeFromIssueBadge {
		return false
	}
	if ms.IssueKnown == nil || !ms.IssueKnown[activeKey] {
		return false
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

		avail := 0
		if ms.Availability != nil {
			avail = ms.Availability[activeKey]
		}

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
			ShortName:    activeKey,
			Display:      item.Name,
			Alias:        alias,
			IssueBadge:   badge,
			Availability: avail,
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

// bodyKindForScreen maps a Screen to the BodyKind a renderer uses to
// select the correct template/view.
func bodyKindForScreen(s Screen) BodyKind {
	switch s.ID {
	case runtime.ScreenMenu:
		return BodyKindMenu
	case runtime.ScreenProfileSelector, runtime.ScreenRegion, runtime.ScreenTheme:
		return BodyKindSelector
	case runtime.ScreenReveal:
		return BodyKindDetail
	case runtime.ScreenChildList, runtime.ScreenResourceList:
		return BodyKindList
	case runtime.ScreenHelp:
		return BodyKindHelp
	case runtime.ScreenIdentity:
		return BodyKindIdentity
	default:
		// Capability screens and future IDs not yet enumerated here.
		return BodyKindUnknown
	}
}
