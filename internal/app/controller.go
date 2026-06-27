package app

import (
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
func New(core *runtime.Core) *Controller {
	return &Controller{core: core}
}

// Apply translates a semantic Action into the matching Core command, applies
// the returned UIIntents to the screen stack, enqueues returned TaskRequests,
// and returns the updated ViewState plus newly-enqueued TaskRequests.
//
// USER-INTENT lane: each Action.Kind maps to a specific Core.HandleX method
// with its own event type and return type. The event construction is added
// in PR-B once the Action→Event translation contract is finalized.
func (c *Controller) Apply(a Action) (ViewState, []runtime.TaskRequest) {
	switch a.Kind {
	case ActionOpenDetail, ActionSelect, ActionCommand,
		ActionOpenYAML, ActionOpenJSON, ActionOpenHelp,
		ActionBack, ActionMoveUp, ActionMoveDown,
		ActionMoveTop, ActionMoveBottom,
		ActionSetFilter, ActionSort,
		ActionSearch, ActionSearchNext, ActionSearchPrev, ActionSearchClear,
		ActionCopy, ActionToggleRelated, ActionToggleWrap, ActionToggleAttention,
		ActionLoadMore, ActionRefresh, ActionOpenIdentity,
		ActionSelectProfile, ActionSelectRegion, ActionSelectTheme,
		ActionReveal, ActionChildView,
		ActionQuit:
		// Dispatch table — each case names the Core command method it will call.
		//
		//   ActionOpenDetail / ActionSelect / ActionOpenYAML / ActionOpenJSON /
		//   ActionOpenHelp / ActionBack:
		//     → c.core.HandleNavigate(NavigateEvent{...}) → (NavigateResult, []TaskRequest)
		//     TODO PR-B: build NavigateEvent from action and call c.core.HandleNavigate
		//
		//   ActionReveal:
		//     → c.core.HandleNavigate(NavigateEvent{Target: NavigateTargetReveal, ...})
		//     TODO PR-B: build NavigateEvent from action and call c.core.HandleNavigate
		//
		//   ActionOpenIdentity (open the caller-identity screen):
		//     → c.core.HandleNavigate(NavigateEvent{Target: NavigateTargetIdentity, ...})
		//     TODO PR-B: build NavigateEvent from action and call c.core.HandleNavigate
		//
		//   ActionSelectProfile:
		//     → c.core.HandleProfileSelected(ProfileSelectedEvent{...}) → ([]UIIntent, []TaskRequest)
		//     TODO PR-B: build ProfileSelectedEvent from action and call c.core.HandleProfileSelected
		//
		//   ActionSelectRegion:
		//     → c.core.HandleRegionSelected(RegionSelectedEvent{...}) → ([]UIIntent, []TaskRequest)
		//     TODO PR-B: build RegionSelectedEvent from action and call c.core.HandleRegionSelected
		//
		//   ActionSelectTheme (theme selector confirm):
		//     → c.core.HandleThemeSelected(ThemeSelectedEvent{...}) → ([]UIIntent, []TaskRequest)
		//     TODO PR-B: build ThemeSelectedEvent from action and call c.core.HandleThemeSelected
		//
		//   ActionChildView:
		//     → c.core.HandleEnterChildView(EnterChildViewEvent{...}) → ([]UIIntent, []TaskRequest)
		//     TODO PR-B: build EnterChildViewEvent from action and call c.core.HandleEnterChildView
		//
		//   ActionToggleRelated (related-panel navigation row selected):
		//     → c.core.HandleRelatedNavigate(RelatedNavigateEvent{...}) → (NavigationResult, []TaskRequest)
		//     TODO PR-B: build RelatedNavigateEvent from action and call c.core.HandleRelatedNavigate
	}
	// TODO PR-B: replace this fall-through with the per-case Core call results above.
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

		// TODO PR-C: PatchResourceList mutates state lifted in PR-C
		// TODO PR-C: PatchDetail mutates state lifted in PR-C
		// TODO PR-C: PatchMenu mutates state lifted in PR-C
		// TODO PR-C: PatchMenuAvailability mutates state lifted in PR-C
		// TODO PR-C: PatchMenuIssueBatch mutates state lifted in PR-C
		// TODO PR-C: PatchMenuCheckProgress mutates state lifted in PR-C
		// TODO PR-C: PatchMenuEnrichProgress mutates state lifted in PR-C
		// TODO PR-C: FlashIntent mutates state lifted in PR-C
		// TODO PR-C: ClearFlash mutates state lifted in PR-C
		// TODO PR-C: SetErrorHintIntent mutates state lifted in PR-C
		// TODO PR-C: AppendErrorHistoryIntent mutates state lifted in PR-C
		// TODO PR-C: ClearActiveListLoadingIntent mutates state lifted in PR-C
		// TODO PR-C: MenuClearAvailabilityIntent mutates state lifted in PR-C
		// TODO PR-C: PopSelectorIntent mutates state lifted in PR-C
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
	// TODO PR-C: populate vs.Body.{List,Detail,Text,Menu,Selector} from top.State.
	return vs
}

// applyNavResult converts a NavigateResult into PushScreen/ReplaceScreen/PopScreen
// stack operations. Called by Apply after HandleNavigate returns.
//
// NavigateResult carries NavigateKind plus ResolvedType, DisplayAlias, Resource,
// CachedEntry, ReplaceCurrent, DispatchEnrich, DispatchRelated — but no ScreenID.
// The adapter (not the runtime) decides which ScreenID to push; this method
// encodes that policy for the headless controller.
//
// TODO PR-B: implement — build ScreenContext from result fields and call
// c.ApplyIntents with the appropriate PushScreen / ReplaceScreen / PopScreen:
//
//	NavigateKindPopAll       → PopScreen until stack empty
//	NavigateKindPushResourceList / NavigateKindPushResourceListCached
//	                         → PushScreen or ReplaceScreen (when ReplaceCurrent)
//	                           with ScreenContext{ResourceType: result.ResolvedType}
//	NavigateKindPushDetail   → PushScreen with ScreenContext{ResourceType, ResourceID}
//	NavigateKindPushYAML / NavigateKindPushJSON
//	                         → PushScreen with ScreenContext{ResourceType, ResourceID}
//	NavigateKindPushHelp / NavigateKindPushRegion / NavigateKindPushTheme
//	                         → PushScreen with appropriate ScreenID
//	NavigateKindFetchProfiles / NavigateKindFetchReveal → no stack change
//	NavigateKindFlash / NavigateKindNoop               → no stack change
func (c *Controller) applyNavResult(_ runtime.NavigateResult) { //nolint:unused // wired in PR-B
	// TODO PR-B: applyNavResult
}

// applyRelatedNavResult converts a NavigationResult into stack operations.
//
// NavigationResult carries NavigationKind plus TargetType, TargetID,
// RelatedIDs, FetchFilter, FilterText — no ScreenID; the controller maps
// kind → ScreenID.
//
// TODO PR-B: implement — build ScreenContext{ResourceType: result.TargetType}
// and emit the appropriate PushScreen based on NavigationKind:
//
//	NavigationKindResourceList / NavigationKindFilteredList → PushScreen (list)
//	NavigationKindDetail                                    → PushScreen (detail)
//	NavigationKindEnterChildView → delegate to HandleEnterChildView path
//	NavigationKindFlash / NavigationKindUnknown            → no stack change
func (c *Controller) applyRelatedNavResult(_ runtime.NavigationResult) { //nolint:unused // wired in PR-B
	// TODO PR-B: applyRelatedNavResult
}

// bodyKindForScreen maps a Screen to the BodyKind a renderer uses to
// select the correct template/view.
func bodyKindForScreen(s Screen) BodyKind {
	switch s.ID {
	case runtime.ScreenProfileSelector:
		return BodyKindSelector
	case runtime.ScreenReveal:
		return BodyKindDetail
	case runtime.ScreenChildList:
		return BodyKindList
	default:
		// Capability screens and future IDs not yet enumerated here.
		// PR-C will extend this switch as new ScreenIDs are registered.
		return BodyKindUnknown
	}
}
